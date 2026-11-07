package engine

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

func inspectedKeyMember(m *Model, guid string, at time.Time, spec int, ranks map[int]int) {
	m.ObserveInspectedBuild(BuildSnapshot{GUID: guid, Source: "inspect", At: at,
		SpecID: spec, GameBuild: referenceRules.Game, SpellRanks: ranks}, at)
}

func keyEvidence(s InferenceSnapshot, guid string, spell int) CooldownEvidence {
	for _, e := range s.Cooldowns {
		if e.GUID == guid && e.Spell == spell {
			return e
		}
	}
	return CooldownEvidence{}
}

func TestFreshKeyReadyReachesBothViewsAndFirstCastReplacesIt(t *testing.T) {
	now := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	m := NewModel()
	inspectedKeyMember(m, guid, now.Add(-time.Second), 264, map[int]int{108271: 1, 57994: 1})
	if len(m.Snapshot(now).Cooldowns) != 0 {
		t.Fatal("inspection alone invented availability")
	}
	if !m.BeginFreshKey(now, now, map[string]bool{guid: true}) {
		t.Fatal("start rejected")
	}
	s := m.Snapshot(now)
	for _, id := range []int{108271, 57994} {
		e := keyEvidence(s, guid, id)
		if e.Status != "estimated_ready" || e.Source != "fresh_key" {
			t.Fatal(e)
		}
	}
	k := kickFixture(now)
	c := DefaultConfig()
	c.KickEnabled = true
	c.FrameEstimates = true
	kick := KickView(s, c, k, now, now, 123)
	if kick.KickBars[1].Status != "estimated_ready" {
		t.Fatal(kick)
	}
	g := &framewire.Snapshot{At: now, Width: 1600, Height: 1000, Party: true, Unknown: true, Ready: true,
		Size: 24, PerRow: 4, Maximum: 8, Opacity: 95,
		Frames: []framewire.Rect{*k.Members[1].Frame}}
	g.Frames[0].GUID = guid
	d := FrameView(s, c, g, now, now, 123)
	found := false
	for _, icon := range d.Icons {
		if icon.Spell == 108271 && icon.Status == "estimated_ready" {
			found = true
		}
	}
	if !found {
		t.Fatal("LamdaCD ready missing", d)
	}
	// A real interrupt within the duplicate window of the synthetic seed must win.
	castAt := now.Add(200 * time.Millisecond)
	if !m.Observe(Cast{GUID: guid, Spell: 57994, At: castAt, Source: "unit_spellcast"}, castAt) {
		t.Fatal("first kick dropped")
	}
	if e := keyEvidence(m.Snapshot(castAt), guid, 57994); e.Status != "estimated_cooldown" || e.Kind != "cast" {
		t.Fatal(e)
	}
	if m.BeginFreshKey(now.Add(time.Second), castAt, map[string]bool{guid: true}) {
		t.Fatal("log/strip duplicate restarted key")
	}
	if e := keyEvidence(m.Snapshot(now.Add(61*time.Second)), guid, 108271); e.Status != "estimated_ready" {
		t.Fatal("same-key ready estimate faded without a new use", e)
	}
}

func TestFreshKeyConfirmationAndRosterBoundaries(t *testing.T) {
	now := time.Unix(1789500000, 0)
	for _, tc := range []struct {
		name   string
		age    time.Duration
		spec   int
		ranks  map[int]int
		member bool
		want   bool
	}{
		{"selected", time.Second, 264, map[int]int{108271: 1}, true, true},
		{"baseline", time.Second, 73, map[int]int{}, true, true},
		{"optional absent", time.Second, 264, map[int]int{}, true, false},
		{"old inspection", 61 * time.Second, 264, map[int]int{108271: 1}, true, false},
		{"incomplete", time.Second, 264, nil, true, false},
		{"late join", time.Second, 264, map[int]int{108271: 1}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel()
			guid := "Player-1-X"
			inspectedKeyMember(m, guid, now.Add(-tc.age), tc.spec, tc.ranks)
			roster := map[string]bool{"Player-1-OTHER": true}
			if tc.member {
				roster[guid] = true
			}
			m.BeginFreshKey(now, now, roster)
			id := 108271
			if tc.spec == 73 {
				id = 6552
			}
			if e := keyEvidence(m.Snapshot(now), guid, id); (e.Status == "estimated_ready") != tc.want {
				t.Fatal(e)
			}
		})
	}
	m := NewModel()
	guid := "Player-1-X"
	m.BeginFreshKey(now, now, map[string]bool{guid: true})
	inspectedKeyMember(m, guid, now.Add(5*time.Second), 264, map[int]int{108271: 1})
	if e := keyEvidence(m.Snapshot(now.Add(5*time.Second)), guid, 108271); e.Status != "estimated_ready" {
		t.Fatal("query completing after start lost", e)
	}
	inspectedKeyMember(m, guid, now.Add(6*time.Second), 264, map[int]int{})
	if e := keyEvidence(m.Snapshot(now.Add(6*time.Second)), guid, 108271); e.Status != "unknown" {
		t.Fatal("talent removal retained ready", e)
	}
}

func TestFreshKeyPreservesObservedCooldownAndCannotReviveAfterEnd(t *testing.T) {
	now := time.Unix(1789500000, 0)
	guid := "Player-1-X"
	m := NewModel()
	inspectedKeyMember(m, guid, now.Add(-2*time.Second), 264, map[int]int{108271: 1, 57994: 1})
	m.Observe(Cast{GUID: guid, Spell: 108271, At: now.Add(-time.Second), Source: "unit_spellcast"}, now)
	roster := map[string]bool{guid: true}
	m.BeginFreshKey(now, now, roster)
	if e := keyEvidence(m.Snapshot(now), guid, 108271); e.Status != "estimated_cooldown" || e.Kind != "cast" {
		t.Fatal("pre-key use erased", e)
	}
	m.ResetLog(now.Add(-2 * time.Second))
	if e := keyEvidence(m.Snapshot(now), guid, 57994); e.Status != "estimated_ready" {
		t.Fatal("delayed log boundary erased start", e)
	}
	m.EndFreshKey()
	if e := keyEvidence(m.Snapshot(now), guid, 57994); e.Spell != 0 {
		t.Fatal("end retained seed", e)
	}
	m.ResetLog(now)
	if m.BeginFreshKey(now, now.Add(time.Second), roster) {
		t.Fatal("old strip revived ended key")
	}
	for _, at := range []time.Time{now.Add(-31 * time.Second), now.Add(3 * time.Second)} {
		if NewModel().BeginFreshKey(at, now, roster) {
			t.Fatal("stale/future start accepted")
		}
	}
	if m.BeginFreshKey(now.Add(time.Minute), now.Add(time.Minute), roster) == false {
		t.Fatal("next key blocked")
	}
}

func TestFreshKeyRespectsModuleSwitchesAndQueryWindow(t *testing.T) {
	now := time.Unix(1789500000, 0)
	guid := "Player-1-X"
	for _, cdOff := range []bool{false, true} {
		m := NewModel()
		m.DisabledDefensives = cdOff
		m.DisabledInterrupts = !cdOff
		inspectedKeyMember(m, guid, now, 264, map[int]int{108271: 1, 57994: 1})
		m.BeginFreshKey(now, now, map[string]bool{guid: true})
		s := m.Snapshot(now)
		if (keyEvidence(s, guid, 108271).Spell != 0) == cdOff || (keyEvidence(s, guid, 57994).Spell != 0) != cdOff {
			t.Fatal(s.Cooldowns)
		}
	}
	m := NewModel()
	m.BeginFreshKey(now, now, map[string]bool{guid: true})
	inspectedKeyMember(m, guid, now.Add(46*time.Second), 264, map[int]int{108271: 1})
	if len(m.Snapshot(now.Add(46*time.Second)).Cooldowns) != 0 {
		t.Fatal("late inspection invented initial state")
	}
}

func TestKickConfirmedAbsenceIsNotUnknown(t *testing.T) {
	now := time.Unix(1789500000, 0)
	c := DefaultConfig()
	c.KickEnabled = true
	for _, spec := range []struct{ class, id int }{{2, 65}, {5, 256}, {5, 257}, {10, 270}, {11, 105}, {13, 1468}} {
		k := kickFixture(now)
		k.Members = k.Members[1:]
		k.Members[0].Class = spec.class
		k.Members[0].Spec = spec.id
		m := NewModel()
		d := KickView(m.Snapshot(now), c, k, now, now, 123)
		if len(d.KickBars) != 0 || len(d.Icons) != 0 {
			t.Fatal("phantom healer interrupt", spec, d)
		}
		k.Members[0].Spec = 0
		if len(KickView(m.Snapshot(now), c, k, now, now, 123).KickBars) != 1 {
			t.Fatal("uninspected member hidden")
		}
	}
	k := kickFixture(now)
	k.Members = k.Members[1:]
	if len(KickView(NewModel().Snapshot(now), c, k, now, now, 123).KickBars) != 1 {
		t.Fatal("Restoration Shaman hidden")
	}
	// An observed interrupt after inspection still overrides a no-kick suggestion.
	m := NewModel()
	k.Members[0].Class = 10
	k.Members[0].Spec = 270
	m.Observe(Cast{GUID: k.Members[0].GUID, Spell: 116705, At: now}, now)
	if len(KickView(m.Snapshot(now), c, k, now, now, 123).KickBars) != 1 {
		t.Fatal("actual cast discarded")
	}
	// Optional talent absence requires a complete query, not a failed query.
	k.Members[0].Class = 7
	k.Members[0].Spec = 264
	m = NewModel()
	inspectedKeyMember(m, k.Members[0].GUID, now, 264, map[int]int{})
	if len(KickView(m.Snapshot(now), c, k, now, now, 123).KickBars) != 0 {
		t.Fatal("unselected talent shown")
	}
}

func TestForeverDoesNotInheritRetailNoKickOrReadyAssumptions(t *testing.T) {
	before := Forever
	Forever = true
	t.Cleanup(func() { Forever = before })
	now := time.Unix(1789500000, 0)
	k := kickFixture(now)
	k.Members = k.Members[1:]
	k.Members[0].Class = 5
	k.Members[0].Spec = 257
	m := NewModel()
	guid := k.Members[0].GUID
	inspectedKeyMember(m, guid, now, 257, map[int]int{108271: 1})
	m.BeginFreshKey(now, now, map[string]bool{guid: true})
	s := m.Snapshot(now)
	if len(s.Cooldowns) != 0 || len(m.InspectedBuilds) != 0 {
		t.Fatal("Retail query assumptions entered beta")
	}
	c := DefaultConfig()
	c.KickEnabled = true
	d := KickView(s, c, k, now, now, 123)
	if len(d.KickBars) != 1 || d.KickBars[0].Status != "unknown" {
		t.Fatal("beta priest inherited Retail absence", d)
	}
}

func TestConfirmedRestorationWindShearUsesItsOwnCooldown(t *testing.T) {
	now := time.Unix(1789500000, 0)
	guid := "Player-1-X"
	for _, spec := range []int{262, 263, 264} {
		m := NewModel()
		inspectedKeyMember(m, guid, now.Add(-time.Second), spec, map[int]int{57994: 1})
		m.Observe(Cast{GUID: guid, Spell: 57994, At: now}, now)
		e := keyEvidence(m.Snapshot(now), guid, 57994)
		want := 12.0
		if spec == 264 {
			want = 30
		}
		if e.CooldownSeconds != want || e.EstimatedEnd != float64(now.Unix())+want {
			t.Fatal(spec, e)
		}
	}
}
