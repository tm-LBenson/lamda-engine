// Package arena isolates enemy observations from friendly-party inference.
package arena

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type Spell struct {
	Name     string
	Cooldown float64
	Category int
	Charges  bool
	Lockout  float64
}

//go:embed catalog.json
var catalogData []byte
var Catalog = func() map[int]Spell {
	var m map[int]Spell
	if err := json.Unmarshal(catalogData, &m); err != nil {
		panic(err)
	}
	return m
}()

func normalizeSpell(id int) int {
	switch id {
	case 33697, 33702:
		return 20572 // One Blood Fury racial, with class variants.
	case 25046, 50613, 69179, 80483, 129597, 155145, 202719, 232633:
		return 28730
	case 59543, 59544, 59545, 59547, 59548, 121093, 28880:
		return 59542
	case 93985:
		return 106839 // Skull Bash interrupt effect.
	}
	return engine.InterruptSpell(id)
}

type Evidence struct {
	Slot, Spell int
	At          time.Time
	Source      string
}
type candidate struct {
	cast               lookup.LiveCast
	at, first, last    time.Time
	rejected, accepted bool
}
type Status struct {
	Epoch                           string `json:"epoch"`
	State                           string `json:"state"`
	Observed, Rejected, CaptureGaps uint64
	Cooldowns                       []Evidence `json:"cooldowns"`
	LogActors                       int        `json:"logActors"`
}
type Model struct {
	snapshot     *lookup.ArenaSnapshot
	received     time.Time
	window       uintptr
	pid          uint32
	background   bool
	pending      map[string]candidate
	spells       map[int]map[int]Evidence
	lastSequence int
	panel        string
	status       Status
	started      time.Time
	lockouts     map[int]lockout
}
type lockout struct {
	School   int
	At       time.Time
	Duration float64
}

func (m *Model) Reset() { *m = Model{status: Status{State: "waiting_for_arena"}} }
func (m *Model) Pause() { m.pending = nil; m.received = time.Time{}; m.status.State = "capture_paused" }
func (m *Model) Active() bool {
	return m.snapshot != nil && m.snapshot.Flags != 0 && len(m.snapshot.Actors) > 0
}
func (m *Model) CanHold(now time.Time) bool {
	return m.snapshot != nil && m.background && !m.received.IsZero() && !now.Before(m.received) && now.Sub(m.received) <= engine.FrameLifetime
}
func (m *Model) Handle(s lookup.BridgeSample, now time.Time) {
	a := s.Arena
	if a == nil {
		return
	}
	if now.Sub(a.At) > 3*time.Second || a.At.Sub(now) > 3*time.Second {
		m.Pause()
		return
	}
	if m.snapshot == nil || m.snapshot.Epoch != a.Epoch || m.window != s.Window || m.pid != s.WindowPID {
		m.Reset()
		m.spells = map[int]map[int]Evidence{}
		m.lockouts = map[int]lockout{}
		m.started = a.At
		m.status.Epoch = a.Epoch
	}
	if a.Sequence <= m.lastSequence {
		return
	} // frozen pixels cannot renew a display lease
	if m.snapshot != nil {
		present := map[int]bool{}
		for _, actor := range a.Actors {
			present[actor.Slot] = true
		}
		for _, old := range m.snapshot.Actors {
			if !present[old.Slot] {
				delete(m.spells, old.Slot)
				delete(m.lockouts, old.Slot)
				m.pending = nil
			}
			for _, newActor := range a.Actors {
				if old.Slot == newActor.Slot && (old.Spec != newActor.Spec || (old.GUID != "" && newActor.GUID != "" && old.GUID != newActor.GUID)) {
					delete(m.spells, old.Slot)
					delete(m.lockouts, old.Slot)
					m.pending = nil
				}
			}
		}
	}
	m.snapshot = a
	m.lastSequence = a.Sequence
	m.received = now
	m.window = s.Window
	m.pid = s.WindowPID
	m.background = s.Event.KeepUIWhenUnfocused
	m.status.State = "observing"
	m.status.LogActors = 0
	for _, actor := range a.Actors {
		if actor.GUID != "" {
			m.status.LogActors++
		}
	}
	if a.Flags == 0 || len(a.Actors) == 0 {
		m.spells = map[int]map[int]Evidence{}
		m.lockouts = map[int]lockout{}
		m.pending = nil
		m.status.State = "disabled"
		return
	}
	if s.Observations == nil || s.Observations.Screen == nil || !s.Observations.Screen.Captured {
		m.pending = nil
		m.status.CaptureGaps++
		m.status.State = "waiting_for_cast_pixels"
		return
	}
	screen := s.Observations.Screen
	if m.panel != screen.ID {
		m.pending = nil
		m.panel = screen.ID
	}
	if m.pending == nil {
		m.pending = map[string]candidate{}
	}
	m.status.Rejected += uint64(screen.Rejected)
	slots := map[string]int{}
	for _, actor := range a.Actors {
		slots[a.Identity(actor.Slot)] = actor.Slot
	}
	present := map[string]bool{}
	for _, c := range screen.Casts {
		slot := slots[c.GUID]
		spellID := normalizeSpell(c.Spell)
		if spellID == 0 {
			spellID = c.Spell
		}
		spell, known := Catalog[spellID]
		if slot == 0 || !known || spell.Category&a.Flags == 0 || c.AgeMillis < 0 || c.AgeMillis > 30000 {
			continue
		}
		key := c.GUID + "/" + c.ID
		present[key] = true
		at := now.Add(-time.Duration(c.AgeMillis) * time.Millisecond)
		p, exists := m.pending[key]
		if !exists {
			m.pending[key] = candidate{cast: c, at: at, first: now, last: now}
			continue
		}
		if p.cast.Spell != c.Spell || at.Sub(p.at) > 350*time.Millisecond || p.at.Sub(at) > 350*time.Millisecond {
			if !p.rejected {
				m.status.Rejected++
			}
			p.rejected = true
		}
		if now.Sub(p.last) > 1200*time.Millisecond || now.Before(p.last) {
			p.first = now
			p.at = at
			p.cast.AgeMillis = c.AgeMillis
		}
		p.last = now
		if !p.rejected && !p.accepted && now.Sub(p.first) >= 150*time.Millisecond && c.AgeMillis > p.cast.AgeMillis {
			m.observe(slot, spellID, p.at, "arena_screen_cast")
			p.accepted = true
		}
		m.pending[key] = p
	}
	for key := range m.pending {
		if !present[key] {
			delete(m.pending, key)
		}
	}
}
func (m *Model) observe(slot, spell int, at time.Time, source string) {
	if m.spells[slot] == nil {
		m.spells[slot] = map[int]Evidence{}
	}
	old, exists := m.spells[slot][spell]
	// A buffered screen cast and its delayed log must not restart one cooldown.
	if exists && !at.After(old.At.Add(time.Second)) {
		return
	}
	m.spells[slot][spell] = Evidence{slot, spell, at, source}
	m.status.Observed++
}
func (m *Model) Status() Status {
	s := m.status
	s.Cooldowns = nil
	for _, spells := range m.spells {
		for _, e := range spells {
			s.Cooldowns = append(s.Cooldowns, e)
		}
	}
	sort.Slice(s.Cooldowns, func(i, j int) bool {
		a, b := s.Cooldowns[i], s.Cooldowns[j]
		if a.Slot != b.Slot {
			return a.Slot < b.Slot
		}
		return a.Spell < b.Spell
	})
	return s
}
func (m *Model) View(now time.Time, holding bool) engine.FrameDisplay {
	d := engine.FrameDisplay{Status: "arena_not_visible", At: float64(now.UnixMilli()) / 1000, Icons: []engine.FrameIcon{}}
	a := m.snapshot
	if a == nil || a.Flags == 0 || a.Hidden || len(a.Actors) == 0 || m.received.IsZero() || now.Before(m.received) {
		return d
	}
	if !(holding && m.background) && now.Sub(m.received) > engine.FrameLifetime {
		return d
	}
	d.Enabled = true
	d.Status = "arena_observations"
	d.Window = strconv.FormatUint(uint64(m.window), 10)
	d.WindowPID = m.pid
	d.Width = a.Width
	d.Height = a.Height
	d.Expires = float64(now.Add(engine.FrameLifetime).UnixMilli()) / 1000
	d.KeepUIWhenUnfocused = m.background
	d.Background = holding
	d.GeometryAge = now.Sub(m.received).Seconds()
	for _, actor := range a.Actors {
		var evidence []Evidence
		var latest Evidence
		for _, e := range m.spells[actor.Slot] {
			if Catalog[e.Spell].Category&a.Flags != 0 {
				evidence = append(evidence, e)
				if e.At.After(latest.At) {
					latest = e
				}
			}
		}
		text := ""
		ends := float64(latest.At.Add(3*time.Second).UnixMilli()) / 1000
		countdown := false
		if a.Flags&8 != 0 && ends > d.At {
			text = Catalog[latest.Spell].Name
		}
		if lock, ok := m.lockouts[actor.Slot]; ok && a.Flags&16 != 0 {
			lockEnds := float64(lock.At.UnixMilli())/1000 + lock.Duration
			if lockEnds > d.At {
				text = schoolName(lock.School) + " locked ~"
				ends = lockEnds
				countdown = true
			}
		}
		if text != "" {
			x, y := actor.X, actor.Y+actor.Size+2
			w := min(240, a.Width-x)
			if x >= 0 && w >= 60 && y >= 0 && y+18 <= a.Height {
				d.Notices = append(d.Notices, engine.FrameNotice{Text: text, X: x, Y: y, Width: w, Opacity: a.Opacity, Ends: ends, Countdown: countdown})
			}
		}
		sort.Slice(evidence, func(i, j int) bool {
			x, y := Catalog[evidence[i].Spell], Catalog[evidence[j].Spell]
			if x.Category != y.Category {
				return x.Category < y.Category
			}
			return evidence[i].Spell < evidence[j].Spell
		})
		for i, e := range evidence {
			if i >= a.Maximum {
				d.Omitted++
				continue
			}
			spell := Catalog[e.Spell]
			end := float64(e.At.UnixMilli())/1000 + spell.Cooldown
			status := "estimated_cooldown"
			if end <= d.At {
				status = "estimated_ready"
			}
			x := actor.X + i*(actor.Size+2)
			y := actor.Y
			if x < 0 || y < 0 || x+actor.Size > a.Width || y+actor.Size > a.Height {
				d.Omitted++
				continue
			}
			d.Icons = append(d.Icons, engine.FrameIcon{GUID: a.Identity(actor.Slot), Spell: e.Spell, Name: spell.Name,
				Label: fmt.Sprintf("Arena %d", actor.Slot), X: x, Y: y, Size: actor.Size, Opacity: a.Opacity, Status: status,
				Ends: end, Duration: spell.Cooldown, Source: e.Source, ChargesUnknown: spell.Charges,
				Reasons: []string{"observed_opponent_cast", "baseline_cooldown", "opponent_talents_and_resets_unverified"}})
		}
	}
	return d
}
func Merge(base, opponents engine.FrameDisplay) engine.FrameDisplay {
	if !opponents.Enabled {
		return base
	}
	if !base.Enabled {
		return opponents
	}
	if base.Window != opponents.Window || base.Width != opponents.Width || base.Height != opponents.Height {
		return base
	}
	base.Expires = min(base.Expires, opponents.Expires)
	for _, icon := range opponents.Icons {
		icon.Module = "arena"
		base.Icons = append(base.Icons, icon)
	}
	base.Notices = append(base.Notices, opponents.Notices...)
	base.Omitted += opponents.Omitted
	return base
}
func schoolName(mask int) string {
	names := map[int]string{1: "Physical", 2: "Holy", 4: "Fire", 8: "Nature", 16: "Frost", 32: "Shadow", 64: "Arcane"}
	if name := names[mask]; name != "" {
		return name
	}
	return "Spell school"
}

// The optional log path requires a fresh native arena mapping. Names, class,
// spec and hostile flags are never used to assign a log actor to an arena row.
func (m *Model) IngestLog(line string, now time.Time) {
	if m.snapshot == nil || m.snapshot.Flags == 0 || m.received.IsZero() || now.Sub(m.received) > engine.FrameLifetime {
		return
	}
	at, kind, err := engine.RecordHeader(line)
	if err != nil || (kind != "SPELL_CAST_SUCCESS" && kind != "SPELL_INTERRUPT") || at.Before(m.started) || now.Sub(at) > 30*time.Second || at.Sub(now) > time.Second {
		return
	}
	_, f, err := engine.RecordFields(line)
	if err != nil || len(f) < 12 {
		return
	}
	id, err := strconv.Atoi(f[9])
	if err != nil {
		return
	}
	id = normalizeSpell(id)
	spell, known := Catalog[id]
	if !known {
		return
	}
	source, target := 0, 0
	for _, actor := range m.snapshot.Actors {
		if actor.GUID == "" {
			continue
		}
		if actor.GUID == f[1] {
			source = actor.Slot
		}
		if actor.GUID == f[5] {
			target = actor.Slot
		}
	}
	if source != 0 && spell.Category&m.snapshot.Flags != 0 {
		m.observe(source, id, at, "arena_combat_log")
	}
	if kind == "SPELL_INTERRUPT" && target != 0 && m.snapshot.Flags&16 != 0 && spell.Lockout > 0 && len(f) >= 15 {
		school, err := strconv.Atoi(f[14])
		if err != nil || school < 1 || school > 127 {
			return
		}
		if old, ok := m.lockouts[target]; !ok || at.After(old.At) {
			m.lockouts[target] = lockout{school, at, spell.Lockout}
		}
	}
}
