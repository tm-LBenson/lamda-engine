package engine

import (
	"reflect"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

func TestSamePartyLogBoundariesKeepTimersAndWarmup(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-FRIEND"
	for _, boundary := range []time.Time{{}, start.Add(5 * time.Minute), start.Add(10 * time.Minute)} {
		m := NewModel()
		m.TrackPartyRoster(map[string]bool{guid: true}, start)
		inspectedKeyMember(m, guid, start, 1467, map[int]int{363916: 1, 374227: 1})
		now := start.Add(10 * time.Minute)
		m.Snapshot(now) // Establish ready without a first use.
		m.Observe(Cast{GUID: guid, Spell: 363916, At: now.Add(-time.Second), Source: "unit_spellcast"}, now)
		before := m.Snapshot(now)
		m.ResetLog(boundary)
		after := m.Snapshot(now)
		if !reflect.DeepEqual(before.Cooldowns, after.Cooldowns) || !reflect.DeepEqual(before.PartySince, after.PartySince) {
			t.Fatalf("log boundary restarted the same party: %v\nbefore=%+v\nafter=%+v", boundary, before.Cooldowns, after.Cooldowns)
		}
		// A delayed log copy after zoning must not spend a second charge.
		m.Observe(Cast{GUID: guid, Spell: 363916, At: now.Add(-time.Second)}, now.Add(time.Second))
		if got := keyEvidence(m.Snapshot(now.Add(time.Second)), guid, 363916); got.EstimatedEnd != keyEvidence(before, guid, 363916).EstimatedEnd {
			t.Fatal("delayed log copy moved the timer", got)
		}
	}
}

func TestRetainedPartyBuildReachesFramesAndKickAfterCacheExpiry(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-FRIEND"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	// A different game version leaves baseline abilities uncertain. They must
	// remain visible as unknown instead of disappearing with the scan cache.
	b := BuildSnapshot{GUID: guid, Source: "inspect", At: start, SpecID: 250, GameBuild: "12.1.5", SpellRanks: map[int]int{48792: 1}}
	m.ObserveInspectedBuild(b, start)
	for _, age := range []time.Duration{6 * time.Minute, time.Hour} {
		now := start.Add(age)
		s := m.Snapshot(now)
		if len(s.Builds) != 1 || !s.Builds[0].At.Equal(start) {
			t.Fatal("cached build missing or inspection timestamp rewritten", s.Builds)
		}
		c := DefaultConfig()
		c.FrameEstimates, c.FrameSpells = true, nil
		g := frameFixture(now)
		g.Persistent, g.Unknown, g.Maximum = true, true, 10
		d := FrameView(s, c, g, now, now, 123)
		found := map[int]FrameIcon{}
		for _, icon := range d.Icons {
			if icon.GUID == guid {
				found[icon.Spell] = icon
			}
		}
		if found[48792].Spell == 0 || found[48707].Status != "unknown" || found[49039].Status != "unknown" {
			t.Fatal("cached abilities disappeared or invented readiness", found)
		}
		s.Cooldowns = nil
		if known := knownCooldowns(s, now); len(known) != 1 || known[0].Spell != 48792 || known[0].Status != "unknown" {
			t.Fatal("retained selection lost before cooldown was observed", known)
		}
	}
	// A retained complete inspection must still suppress an unselected kick.
	inspectedKeyMember(m, guid, start.Add(time.Hour), 253, map[int]int{})
	now := start.Add(2 * time.Hour)
	e := kickEvidence(m.Snapshot(now), framewire.KickMember{GUID: guid, Class: 3, Spec: 253}, now)
	if e.Status != "unavailable" {
		t.Fatal("expired scan invented an unselected kick", e)
	}
	m.TrackPartyRoster(map[string]bool{}, now)
	if len(m.Snapshot(now).Builds) != 0 {
		t.Fatal("cached build survived departure")
	}
}

func TestRetainedPartyBuildStillAppliesCooldownModifiers(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-FRIEND"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 264, map[int]int{108271: 1, 381647: 1})
	now := start.Add(time.Hour)
	m.Snapshot(now) // Expire the short inspection cache first.
	m.Observe(Cast{GUID: guid, Spell: 108271, At: now}, now)
	if e := keyEvidence(m.Snapshot(now), guid, 108271); e.CooldownSeconds != 90 || e.BuildRevision != m.partyBuilds[guid].Build.Revision {
		t.Fatal("retained talent modifier was ignored", e)
	}
	inspectedKeyMember(m, guid, now.Add(time.Second), 264, map[int]int{108271: 1})
	if e := keyEvidence(m.Snapshot(now.Add(91*time.Second)), guid, 108271); e.Status != "unknown" || !e.ReadyInvalidated {
		t.Fatal("changed build reused old retained cooldown", e)
	}
	if e := keyEvidence(m.Snapshot(now.Add(121*time.Second)), guid, 108271); e.Status != "estimated_ready" || e.CooldownSeconds != 120 || e.BuildRevision != m.partyBuilds[guid].Build.Revision {
		t.Fatal("changed build did not complete its own warm-up", e)
	}
}
