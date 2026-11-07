package lookup

import (
	"context"
	"errors"
	"sort"
	"time"
)

type Event struct {
	KeepUIWhenUnfocused bool        `json:"keepUIWhenUnfocused,omitempty"`
	ID                  string      `json:"id"`
	Kind                string      `json:"kind"`
	At                  time.Time   `json:"at"`
	Deadline            time.Time   `json:"deadline"`
	Members             []Character `json:"members"`
}
type Stats struct {
	Queued    uint64            `json:"queued"`
	Requests  uint64            `json:"requests"`
	Completed uint64            `json:"completed"`
	CacheHits uint64            `json:"cacheHits"`
	Dropped   uint64            `json:"dropped"`
	Failures  map[string]uint64 `json:"failures"`
}
type Status struct {
	Pending       int       `json:"pending"`
	InFlight      bool      `json:"inFlight"`
	NextRequestAt time.Time `json:"nextRequestAt"`
	Stats         Stats     `json:"stats"`
}
type membership struct {
	Character
	Generation uint64
}
type job struct {
	Member                membership
	Reason                string
	RequestedAt, Deadline time.Time
	Attempts              int
}
type outcome struct {
	Job      job
	Profile  Profile
	Error    error
	Finished time.Time
}

// Queue is owned by the engine loop. Only the fetch runs in a worker; it returns
// immutable results through done. No HTTP wait or Sleep runs in the game thread
// or the engine's log-ingestion loop.
type Queue struct {
	scanTiming   int
	scanInterval time.Duration
	lastScan     map[string]time.Time
	lastKey      time.Time
	// Called only by the owning loop; local/inspected builds need no web fallback.
	LocalBuild    func(string, time.Time) bool
	RosterGrace   time.Duration
	disabled      bool
	ctx           context.Context
	fetcher       Fetcher
	done          chan outcome
	active        *job
	cancel        context.CancelFunc
	pending       []job
	members       map[string]membership
	cache         map[string]Profile
	events        map[string]time.Time
	next, blocked time.Time
	generation    uint64
	interval      time.Duration
	stats         Stats
	rosterKnown   bool
}

func NewQueue(ctx context.Context, f Fetcher) *Queue {
	return &Queue{ctx: ctx, fetcher: f, done: make(chan outcome, 1), members: map[string]membership{}, cache: map[string]Profile{}, events: map[string]time.Time{}, interval: time.Second, stats: Stats{Failures: map[string]uint64{}}}
}
func (q *Queue) SetScanPolicy(mode, minutes int) {
	q.scanTiming = max(1, min(4, mode))
	q.scanInterval = time.Duration(max(1, min(5, minutes))) * time.Minute
	if q.lastScan == nil {
		q.lastScan = map[string]time.Time{}
	}
}
func (q *Queue) SetInterval(interval time.Duration) {
	q.interval = max(time.Second, min(5*time.Second, interval))
}
func (q *Queue) Fail(code string) {
	if len(q.stats.Failures) >= 32 {
		if _, ok := q.stats.Failures[code]; !ok {
			code = "other"
		}
	}
	q.stats.Failures[code]++
}
func (q *Queue) Clear() {
	if q.cancel != nil {
		q.cancel()
	}
	q.members = map[string]membership{}
	q.lastScan = map[string]time.Time{}
	q.lastKey = time.Time{}
	q.rosterKnown = false
	q.pending = nil
	q.events = map[string]time.Time{}
	q.generation++
}
func (q *Queue) SetEnabled(enabled bool) {
	if q.disabled == !enabled {
		return
	}
	q.disabled = !enabled
	if !enabled {
		q.Clear()
	}
}
func (q *Queue) Handle(e Event, now time.Time) []Profile {
	if q.disabled {
		return nil
	}
	keyStart := e.Kind == "challenge_start" && !e.Deadline.IsZero() && !e.Deadline.Equal(q.lastKey)
	keyAt := e.Deadline
	// Challenge lifecycle also refreshes the live roster. Web prewarming is
	// handled earlier by the separate key_start countdown event.
	if e.Kind == "challenge_start" || e.Kind == "challenge_end" {
		e.Kind, e.Deadline = "roster", time.Time{}
	}
	if e.ID == "" || len(e.ID) > 128 || len(e.Members) > 5 || e.At.After(now.Add(2*time.Second)) || now.Sub(e.At) > 30*time.Second {
		q.Fail("invalid_or_stale_trigger")
		return nil
	}
	if e.Kind != "roster" && e.Kind != "key_start" && e.Kind != "arena_ready" {
		q.Fail("unsupported_trigger")
		return nil
	}
	if e.Kind != "roster" && (e.Deadline.IsZero() || e.Deadline.Sub(e.At) > 15*time.Second || !e.Deadline.After(e.At)) {
		q.Fail("invalid_deadline")
		return nil
	}
	if _, exists := q.events[e.ID]; exists {
		return nil
	}
	for id, at := range q.events {
		if now.Sub(at) > time.Minute {
			delete(q.events, id)
		}
	}
	if len(q.events) >= 128 {
		q.Fail("trigger_overflow")
		return nil
	}
	newMembers := map[string]membership{}
	for _, c := range e.Members {
		if !c.Valid() {
			q.Fail("missing_identity")
			return nil
		}
		if _, exists := newMembers[c.GUID]; exists {
			q.Fail("duplicate_identity")
			return nil
		}
		m, exists := q.members[c.GUID]
		if !exists || m.Key() != c.Key() {
			q.generation++
			m = membership{c, q.generation}
		}
		newMembers[c.GUID] = m
	}
	q.events[e.ID] = now
	if keyStart {
		q.lastKey = keyAt
	}
	for guid := range q.lastScan {
		if _, present := newMembers[guid]; !present {
			delete(q.lastScan, guid)
		}
	}
	oldMembers := q.members
	q.members = newMembers
	q.rosterKnown = true
	filtered := q.pending[:0]
	for _, j := range q.pending {
		if m, exists := q.members[j.Member.GUID]; exists && m.Generation == j.Member.Generation {
			filtered = append(filtered, j)
		} else {
			q.stats.Dropped++
		}
	}
	q.pending = filtered
	if q.active != nil {
		if m, exists := q.members[q.active.Member.GUID]; !exists || m.Generation != q.active.Member.Generation {
			q.cancel()
		}
	}
	var cached []Profile
	for _, c := range e.Members {
		m := q.members[c.GUID]
		if q.hasLocal(c.GUID, now) {
			continue
		}
		old, existed := oldMembers[c.GUID]
		if q.scanTiming != 0 {
			joined := !existed || old.Generation != m.Generation
			wanted := (joined && q.scanTiming != 3) || (keyStart && q.scanTiming != 1)
			if q.scanTiming == 4 && now.Sub(q.lastScan[c.GUID]) >= q.scanInterval {
				wanted = true
			}
			if !wanted {
				continue
			}
			q.lastScan[c.GUID] = now
		} else if e.Kind == "roster" && existed && old.Generation == m.Generation {
			continue
		}
		{
			if p, ok := q.cache[c.Key()]; ok && now.Sub(p.FetchedAt) < 5*time.Minute {
				p.Character = c
				cached = append(cached, p)
				q.stats.CacheHits++
				continue
			}
		}
		j := job{Member: m, Reason: e.Kind, RequestedAt: e.At, Deadline: e.Deadline}
		if q.scanTiming != 0 {
			j.Reason, j.RequestedAt, j.Deadline = "roster", now, now.Add(30*time.Second)
		}
		if j.Deadline.IsZero() {
			j.Deadline = now.Add(30 * time.Second)
		}
		q.enqueue(j)
	}
	return cached
}

// A nil roster means unavailable; an empty, non-nil roster means ungrouped.
func (q *Queue) Roster() map[string]bool {
	if !q.rosterKnown {
		return nil
	}
	r := make(map[string]bool, len(q.members))
	for guid := range q.members {
		r[guid] = true
	}
	return r
}
func (q *Queue) enqueue(j job) {
	if q.active != nil && q.active.Member.GUID == j.Member.GUID && q.active.Member.Generation == j.Member.Generation {
		return
	}
	for i, pending := range q.pending {
		if pending.Member.GUID == j.Member.GUID {
			// Repeated prep/roster events coalesce; prep gets the tighter deadline.
			if j.Reason != "roster" || pending.Reason == "roster" {
				q.pending[i] = j
			}
			return
		}
	}
	q.pending = append(q.pending, j)
	q.stats.Queued++
}

func (q *Queue) Advance(now time.Time) []Profile {
	if q.active != nil && q.hasLocal(q.active.Member.GUID, now) && q.cancel != nil {
		q.cancel()
	}
	var accepted []Profile
	select {
	case result := <-q.done:
		q.active = nil
		q.cancel = nil
		q.next = result.Finished.Add(q.interval)
		j := result.Job
		// Provider backoff applies even when the requesting player has left.
		var limited *Failure
		if errors.As(result.Error, &limited) && limited.RetryAfter > 0 {
			q.blocked = maxTime(q.blocked, result.Finished.Add(limited.RetryAfter))
		}
		m, present := q.members[j.Member.GUID]
		if !present || m.Generation != j.Member.Generation || q.hasLocal(j.Member.GUID, now) {
			q.stats.Dropped++
		} else if result.Error != nil {
			failure := &Failure{Code: "network_error"}
			var known *Failure
			if errors.As(result.Error, &known) {
				failure = known
			}
			q.Fail(failure.Code)
			if !result.Finished.Before(j.Deadline) {
				q.Fail("refresh_deadline_missed")
			}
			if j.Attempts < 2 && failure.RetryAfter > 0 && q.blocked.Before(j.Deadline) {
				q.enqueue(j)
			}
		} else {
			q.stats.Completed++
			if result.Finished.After(j.Deadline) {
				q.Fail("refresh_finished_late")
			}
			p := result.Profile
			p.Character = m.Character
			q.cache[m.Key()] = p
			accepted = append(accepted, p)
		}
	default:
	}
	for key, p := range q.cache {
		if now.Sub(p.FetchedAt) > 24*time.Hour {
			delete(q.cache, key)
		}
	}
	if len(q.cache) > 128 {
		var oldest string
		for key, p := range q.cache {
			if oldest == "" || p.FetchedAt.Before(q.cache[oldest].FetchedAt) {
				oldest = key
			}
		}
		delete(q.cache, oldest)
	}
	// Never start a catch-up burst, even after a delayed engine tick.
	filtered := q.pending[:0]
	for _, j := range q.pending {
		if q.hasLocal(j.Member.GUID, now) {
			q.stats.Dropped++
		} else if !now.Before(j.Deadline) {
			q.Fail("refresh_deadline_missed")
			q.stats.Dropped++
		} else {
			filtered = append(filtered, j)
		}
	}
	q.pending = filtered
	if q.disabled || q.active != nil || len(q.pending) == 0 || now.Before(q.next) || now.Before(q.blocked) || q.ctx.Err() != nil {
		return accepted
	}
	sort.SliceStable(q.pending, func(i, j int) bool { return q.pending[i].Deadline.Before(q.pending[j].Deadline) })
	j := q.pending[0]
	if j.Reason == "roster" && now.Before(j.RequestedAt.Add(q.RosterGrace)) {
		return accepted
	}
	q.pending = q.pending[1:]
	j.Attempts++
	q.active = &j
	q.stats.Requests++
	ctx, cancel := context.WithDeadline(q.ctx, minTime(j.Deadline, now.Add(3*time.Second)))
	q.cancel = cancel
	go func() {
		p, err := q.fetcher.Fetch(ctx, j.Member.Character)
		cancel()
		q.done <- outcome{j, p, err, time.Now()}
	}()
	return accepted
}
func (q *Queue) hasLocal(guid string, now time.Time) bool {
	return q.LocalBuild != nil && q.LocalBuild(guid, now)
}
func (q *Queue) Status() Status {
	s := q.stats
	s.Failures = map[string]uint64{}
	for k, v := range q.stats.Failures {
		s.Failures[k] = v
	}
	return Status{len(q.pending), q.active != nil, maxTime(q.next, q.blocked), s}
}
func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
