package lookup

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakeCall struct {
	Character Character
	Reply     chan error
}
type fakeFetcher struct{ Calls chan fakeCall }

func (f *fakeFetcher) Fetch(ctx context.Context, c Character) (Profile, error) {
	call := fakeCall{c, make(chan error, 1)}
	select {
	case f.Calls <- call:
	case <-ctx.Done():
		return Profile{}, ctx.Err()
	}
	select {
	case err := <-call.Reply:
		return Profile{Character: c, FetchedAt: time.Now(), SpecID: 1468, Talents: []Talent{{1, 2, 3, 1}}}, err
	case <-ctx.Done():
		return Profile{}, ctx.Err()
	}
}
func testQueue(t *testing.T) (*Queue, *fakeFetcher) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f := &fakeFetcher{make(chan fakeCall, 10)}
	q := NewQueue(ctx, f)
	t.Cleanup(q.Clear)
	return q, f
}
func takeCall(t *testing.T, f *fakeFetcher) fakeCall {
	t.Helper()
	select {
	case c := <-f.Calls:
		return c
	case <-time.After(time.Second):
		t.Fatal("request not started")
		return fakeCall{}
	}
}
func finish(t *testing.T, q *Queue, call fakeCall, err error) {
	t.Helper()
	call.Reply <- err
	end := time.Now().Add(time.Second)
	for len(q.done) == 0 && time.Now().Before(end) {
		time.Sleep(time.Millisecond)
	}
	if len(q.done) == 0 {
		t.Fatal("worker did not return")
	}
}
func TestJoinQueueOneSecondSpacingAndNoCatchupBurst(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	members := []Character{alice, {"Player-1-BOB", "Bob", "Area 52", "us"}, {"Player-1-CAT", "Cat", "Area 52", "us"}}
	q.Handle(Event{ID: "join", Kind: "roster", At: now, Members: members}, now)
	q.Advance(now)
	call := takeCall(t, f)
	q.Advance(now.Add(2 * time.Second))
	if len(f.Calls) != 0 {
		t.Fatal("parallel lookup started")
	}
	finish(t, q, call, nil)
	results := q.Advance(time.Now())
	if len(results) != 1 {
		t.Fatal("result lost")
	}
	q.Advance(q.next.Add(-time.Nanosecond))
	if len(f.Calls) != 0 {
		t.Fatal("one-second gap skipped")
	}
	q.Advance(q.next)
	call = takeCall(t, f)
	if q.stats.Requests != 2 || len(q.pending) != 1 {
		t.Fatal("catch-up burst", q.Status())
	}
	finish(t, q, call, nil)
	q.Advance(time.Now())
	if q.next.Before(time.Now().Add(900 * time.Millisecond)) {
		t.Fatal("gap measured from start instead of finish")
	}
}
func TestDuplicateRosterPrepRefreshAndDepartedResults(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	e := Event{ID: "join", Kind: "roster", At: now, Members: []Character{alice}}
	q.Handle(e, now)
	e.ID = "heartbeat"
	q.Handle(e, now)
	if len(q.pending) != 1 {
		t.Fatal("heartbeat duplicated request")
	}
	q.Advance(now)
	call := takeCall(t, f)
	e.ID = "key"
	e.Kind = "key_start"
	e.Deadline = now.Add(10 * time.Second)
	q.Handle(e, now)
	q.Handle(e, now)
	if len(q.pending) != 0 {
		t.Fatal("prep duplicated an in-flight request")
	}
	e.ID = "left"
	e.Kind = "roster"
	e.Members = nil
	q.Handle(e, now)
	finish(t, q, call, nil)
	if len(q.Advance(time.Now())) != 0 || len(q.pending) != 0 || q.stats.Dropped == 0 {
		t.Fatal("departed player retained", q.Status())
	}
}
func TestLeaveRejoinDoesNotAcceptOldInflightBuild(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	q.Handle(Event{ID: "1", Kind: "roster", At: now, Members: []Character{alice}}, now)
	q.Advance(now)
	call := takeCall(t, f)
	q.Handle(Event{ID: "2", Kind: "roster", At: now}, now)
	q.Handle(Event{ID: "3", Kind: "roster", At: now, Members: []Character{alice}}, now)
	finish(t, q, call, nil)
	if len(q.Advance(time.Now())) != 0 {
		t.Fatal("old membership result accepted")
	}
}
func TestRateLimitAndExpiredPrepDoNotBurstAfterWindow(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	e := Event{ID: "key", Kind: "key_start", At: now, Deadline: now.Add(10 * time.Second), Members: []Character{alice, {"Player-1-BOB", "Bob", "Area 52", "us"}}}
	q.Handle(e, now)
	q.Advance(now)
	call := takeCall(t, f)
	finish(t, q, call, &Failure{Code: "rate_limited", RetryAfter: 20 * time.Second})
	q.Advance(time.Now())
	q.Advance(now.Add(11 * time.Second))
	if len(f.Calls) != 0 || len(q.pending) != 0 || q.stats.Failures["refresh_deadline_missed"] != 1 || q.stats.Failures["rate_limited"] != 1 {
		t.Fatal(q.Status())
	}
	if q.blocked.Before(now.Add(19 * time.Second)) {
		t.Fatal("Retry-After ignored")
	}
}
func TestRosterAndPrepReuseRecentCache(t *testing.T) {
	q, _ := testQueue(t)
	now := time.Now()
	q.cache[alice.Key()] = Profile{Character: alice, FetchedAt: now}
	e := Event{ID: "join", Kind: "roster", At: now, Members: []Character{alice}}
	if len(q.Handle(e, now)) != 1 || len(q.pending) != 0 {
		t.Fatal("join cache missed")
	}
	e.ID = "arena"
	e.Kind = "arena_ready"
	e.Deadline = now.Add(10 * time.Second)
	if len(q.Handle(e, now)) != 1 || len(q.pending) != 0 {
		t.Fatal("prep bypassed recent cache")
	}
}
func TestInvalidEventsAndLateTriggersAreRecorded(t *testing.T) {
	q, _ := testQueue(t)
	now := time.Now()
	for i, e := range []Event{
		{Kind: "roster", At: now, Members: []Character{alice}},
		{Kind: "roster", At: now.Add(-time.Minute), Members: []Character{alice}},
		{Kind: "arena_ready", At: now, Deadline: now.Add(time.Hour), Members: []Character{alice}},
		{Kind: "roster", At: now, Members: []Character{{GUID: "Player-1-UNKNOWN", Name: "Unknown"}}},
	} {
		if i > 0 {
			e.ID = fmt.Sprint(i)
		}
		q.Handle(e, now)
	}
	if len(q.pending) != 0 || len(q.stats.Failures) < 3 {
		t.Fatal(q.Status())
	}
	e := Event{ID: "late", Kind: "arena_ready", At: now.Add(-12 * time.Second), Deadline: now.Add(-2 * time.Second), Members: []Character{alice}}
	q.Handle(e, now)
	q.Advance(now)
	if q.stats.Failures["refresh_deadline_missed"] != 1 || q.stats.Requests != 0 {
		t.Fatal("late countdown became a new ten-second window", q.Status())
	}
}

func TestDisabledQueueCancelsRequestsAndRejectsNewWork(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	e := Event{ID: "joined", Kind: "roster", At: now, Members: []Character{alice, {"Player-1-BOB", "Bob", "Area 52", "us"}}}
	q.Handle(e, now)
	q.Advance(now)
	takeCall(t, f)
	q.SetEnabled(false)
	deadline := time.Now().Add(time.Second)
	for len(q.done) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(q.done) == 0 {
		t.Fatal("active request was not canceled")
	}
	if got := q.Advance(now); len(got) != 0 || q.active != nil || len(q.pending) != 0 || len(q.Roster()) != 0 {
		t.Fatal("disabled work retained", q.Status())
	}
	e.ID = "disabled"
	q.Handle(e, now)
	q.Advance(now.Add(time.Minute))
	if len(f.Calls) != 0 || len(q.pending) != 0 || len(q.Roster()) != 0 {
		t.Fatal("disabled caller started lookup")
	}
	q.SetEnabled(true)
	e.ID = "enabled"
	e.At = time.Now()
	q.Handle(e, e.At)
	q.Advance(e.At.Add(2 * time.Second))
	call := takeCall(t, f)
	finish(t, q, call, nil)
	if results := q.Advance(time.Now()); len(results) != 1 {
		t.Fatal("lookup did not resume", q.Status())
	}
}
