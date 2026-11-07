package engine

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func inspected(m *Model, at time.Time, spells map[int]int) BuildSnapshot {
	b := BuildSnapshot{GUID: "Player-1-BOB", Player: "Bob", Source: "inspect", At: at, SpecID: 264, GameBuild: "12.1.0", SpellRanks: spells, Talents: [][3]int{{1, 101, 1}}}
	m.ObserveInspectedBuild(b, at)
	return m.InspectedBuilds[b.GUID]
}
func TestInspectedTalentsAdjustRechargeAndDoNotInventReadiness(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	m := NewModel()
	b := inspected(m, now.Add(-time.Second), map[int]int{108271: 1, 381647: 1})
	m.ObservePublicBuild(lookup.Profile{Character: lookup.Character{GUID: b.GUID, Name: "Bob", Realm: "Realm", Region: "us"}, SpecID: 264, Source: "raider_io", FetchedAt: now, Talents: []lookup.Talent{{SpellID: 98008, Rank: 1}}})
	known := knownCooldowns(m.Snapshot(now), now)
	if len(known) != 1 || known[0].Spell != 108271 || known[0].Source != "inspect" || known[0].Status != "unknown" || known[0].EstimatedEnd != 0 {
		t.Fatal(known)
	}
	if len(candidateCooldowns(m.Snapshot(now), now, "Player-1-OWN")) != 0 {
		t.Fatal("unselected candidates returned")
	}
	m.Observe(Cast{At: now, GUID: b.GUID, Name: "Bob", Spell: 108271, Source: "unit_spellcast"}, now.Add(200*time.Millisecond))
	e := m.Snapshot(now.Add(time.Second)).Cooldowns[0]
	if e.BaselineSeconds != 120 || e.CooldownSeconds != 90 || e.EstimatedEnd != float64(now.UnixMilli())/1000+90 || e.Source != "unit_spellcast" || e.AvailableCharges != nil {
		t.Fatal(e)
	}
	if len(e.AppliedTalents) != 1 || e.AppliedTalents[0] != 381647 {
		t.Fatal(e)
	}
	// A changed specialization in a newer log invalidates the inspection cache.
	m.Builds[b.GUID] = BuildSnapshot{GUID: b.GUID, At: now.Add(time.Second), SpecID: 263}
	if current, _ := m.buildFor(b.GUID, now.Add(2*time.Second)); current.Source == "inspect" {
		t.Fatal("old spec retained")
	}
}
func TestRuleExamplesRanksVersionAndFreshness(t *testing.T) {
	cases := []struct {
		spell   int
		talents map[int]int
		want    float64
	}{
		{108416, map[int]int{386686: 1}, 45}, {104773, map[int]int{386659: 1}, 135},
		{871, map[int]int{397103: 1, 391271: 1}, 108}, {642, map[int]int{114154: 1}, 210},
		{45438, map[int]int{382424: 2}, 180}, {357170, map[int]int{376204: 1}, 50},
	}
	now := time.Now().Truncate(time.Millisecond)
	for _, c := range cases {
		m := NewModel()
		b := inspected(m, now.Add(-time.Second), c.talents)
		m.Observe(Cast{At: now, GUID: b.GUID, Spell: c.spell}, now)
		e := m.Snapshot(now).Cooldowns[0]
		if math.Abs(e.CooldownSeconds-c.want) > 0.0001 {
			t.Fatal(c, e)
		}
	}
	for _, mutate := range []func(*BuildSnapshot){func(b *BuildSnapshot) { b.GameBuild = "12.2.0" }, func(b *BuildSnapshot) { b.At = now.Add(-6 * time.Minute) }, func(b *BuildSnapshot) { b.At = now.Add(time.Second) }} {
		m := NewModel()
		b := inspected(m, now.Add(-time.Second), map[int]int{381647: 1})
		mutate(&b)
		m.InspectedBuilds[b.GUID] = b
		m.Observe(Cast{At: now, GUID: b.GUID, Spell: 108271}, now)
		if m.Snapshot(now).Cooldowns[0].CooldownSeconds != 120 {
			t.Fatal("invalid build modified a cast")
		}
	}
}
func TestChargesQueueReconciliationAndLateSessionBoundary(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	m := NewModel()
	b := inspected(m, now.Add(-time.Second), map[int]int{363916: 1, 375406: 1})
	for _, offset := range []time.Duration{0, 5 * time.Second} {
		m.Observe(Cast{At: now.Add(offset), GUID: b.GUID, Spell: 363916, Source: "unit_spellcast"}, now.Add(offset))
	}
	e := m.Snapshot(now.Add(10 * time.Second)).Cooldowns[0]
	if e.MaxCharges == nil || *e.MaxCharges != 2 || len(e.RechargeEnds) != 2 || e.RechargeEnds[1]-e.RechargeEnds[0] != 90 || e.AvailableCharges != nil {
		t.Fatal(e)
	}
	if e = m.Snapshot(now.Add(95 * time.Second)).Cooldowns[0]; e.EstimatedEnd != float64(now.UnixMilli())/1000+180 {
		t.Fatal("wrong next recharge", e)
	}
	m.ResetLog(now.Add(-time.Second))
	if len(m.Evidence) != 1 {
		t.Fatal("delayed marker erased live casts")
	}
	m.ResetLog(now.Add(6 * time.Second))
	if len(m.Evidence) != 0 {
		t.Fatal("old zone evidence retained")
	}
	// Either source can arrive first; matching log lines never spend another charge.
	for _, liveFirst := range []bool{true, false} {
		m = NewModel()
		inspected(m, now.Add(-time.Second), map[int]int{363916: 1, 375406: 1})
		live := Cast{At: now.Add(200 * time.Millisecond), GUID: b.GUID, Spell: 363916, Source: "unit_spellcast"}
		logCast := Cast{At: now, GUID: b.GUID, Spell: 363916}
		if liveFirst {
			m.Observe(live, live.At)
			m.Observe(logCast, now.Add(154*time.Second))
		} else {
			m.Observe(logCast, now)
			m.Observe(live, live.At)
		}
		e = m.Snapshot(now.Add(time.Second)).Cooldowns[0]
		if len(e.RechargeEnds) != 1 || m.Diagnostics.AcceptedCasts != 1 {
			t.Fatal("duplicate charge spent", e, m.Diagnostics)
		}
	}
}
func TestSelectedBuildChangesAndResetDoNotReapplyOldTalentRules(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	m := NewModel()
	b := inspected(m, now.Add(-time.Second), map[int]int{381647: 1})
	m.Observe(Cast{At: now, GUID: b.GUID, Spell: 108271}, now)
	b.At = now.Add(time.Second)
	b.SpellRanks = map[int]int{108271: 1}
	m.ObserveInspectedBuild(b, b.At)
	if e := m.Snapshot(b.At).Cooldowns[0]; e.CooldownSeconds != 90 {
		t.Fatal("retroactive timer rewrite", e)
	}
	m.Observe(Cast{At: now.Add(100 * time.Second), GUID: b.GUID, Spell: 108271}, now.Add(100*time.Second))
	if e := m.Snapshot(now.Add(101 * time.Second)).Cooldowns[0]; e.CooldownSeconds != 120 {
		t.Fatal("removed modifier applied", e)
	}
	m = NewModel()
	inspected(m, now.Add(-time.Second), map[int]int{382424: 2, 1244110: 1})
	m.Observe(Cast{At: now, GUID: b.GUID, Spell: 45438}, now)
	m.Observe(Cast{At: now.Add(time.Second), GUID: b.GUID, Spell: 235219}, now.Add(time.Second))
	e := m.Snapshot(now.Add(2 * time.Second)).Cooldowns[0]
	if e.Kind != "reset" || len(e.RechargeEnds) != 0 || e.Status != "estimated_ready" {
		t.Fatal(fmt.Sprint(e))
	}
}

func TestInspectedBuildKeepsBaselineDefensesWithoutResurrectingOptionalTalents(t *testing.T) {
	now := time.Now()
	m := NewModel()
	b := inspected(m, now.Add(-time.Second), map[int]int{61336: 1})
	b.SpecID = 104
	m.InspectedBuilds[b.GUID] = b
	candidates := candidateCooldowns(m.Snapshot(now), now, "Player-own")
	if len(candidates) != 1 || candidates[0].Spell != 22812 || candidates[0].Status != "unknown" {
		t.Fatal(candidates)
	}
}

func TestUninspectedSpellRanksDoNotInventAChargeCapacity(t *testing.T) {
	now := time.Now()
	m := NewModel()
	b := inspected(m, now.Add(-time.Second), map[int]int{2565: 1})
	b.SpecID = 73
	m.InspectedBuilds[b.GUID] = b
	m.Observe(Cast{At: now, GUID: b.GUID, Spell: 2565}, now)
	e := m.Snapshot(now).Cooldowns[0]
	if e.MaxCharges != nil || e.AvailableCharges != nil || !e.MayHaveCharges {
		t.Fatal(e)
	}
}
