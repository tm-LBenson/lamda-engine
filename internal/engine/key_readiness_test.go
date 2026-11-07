package engine

import (
	"slices"
	"testing"
	"time"
)

func TestSameKeyReadinessSurvivesInactivityAndLateDelivery(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	for _, tc := range []struct {
		name, source string
		spell        int
		delay        time.Duration
	}{
		{"live kick", "unit_spellcast", 57994, 0},
		{"late kick", "", 57994, 49 * time.Second},
		{"defensive", "", 108271, 0},
		{"buff onset", "aura_activation", 48707, 0},
		{"unknown charge count", "aura_activation", 363916, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel()
			m.BeginFreshKey(start, start, map[string]bool{guid: true})
			cast := Cast{GUID: guid, Spell: tc.spell, At: start.Add(time.Second), Source: tc.source}
			m.Observe(cast, cast.At.Add(tc.delay))
			end := keyEvidence(m.Snapshot(cast.At.Add(tc.delay)), guid, tc.spell).EstimatedEnd
			for _, elapsed := range []time.Duration{5 * time.Minute, 35 * time.Minute} {
				e := keyEvidence(m.Snapshot(start.Add(elapsed)), guid, tc.spell)
				if e.Status != "estimated_ready" || e.EstimatedEnd != end || !slices.Contains(e.Reasons, "same_key_ready_estimate") || slices.Contains(e.Reasons, "observation_stale") {
					t.Fatal("same-key estimate expired or changed its clock", e)
				}
				if e.AvailableCharges != nil {
					t.Fatal("estimated readiness invented a charge count", e)
				}
			}
			// An old buffered copy cannot erase the retained timestamp. A new
			// observed use must immediately replace Ready with its real timer.
			m.Observe(cast, start.Add(35*time.Minute))
			cast.At = start.Add(36 * time.Minute)
			m.Observe(cast, cast.At)
			e := keyEvidence(m.Snapshot(cast.At), guid, tc.spell)
			if e.Status != "estimated_cooldown" && e.Status != "estimated_recharge" {
				t.Fatal("new use did not replace retained readiness", e)
			}
			if !e.EventAt.Equal(cast.At) {
				t.Fatal("new use kept the old clock", e)
			}
		})
	}
}

func TestSameKeyReadinessBoundaries(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	for _, boundary := range []string{"end", "reset", "new key", "left party", "changed build", "log boundary"} {
		t.Run(boundary, func(t *testing.T) {
			m := NewModel()
			inspectedKeyMember(m, guid, start, 264, map[int]int{108271: 1})
			m.BeginFreshKey(start, start, map[string]bool{guid: true, "Player-1-OTHER": true})
			m.Observe(Cast{GUID: guid, Spell: 108271, At: start.Add(time.Second)}, start.Add(time.Second))
			now := start.Add(10 * time.Minute)
			if keyEvidence(m.Snapshot(now), guid, 108271).Status != "estimated_ready" {
				t.Fatal("test did not establish retained readiness")
			}
			switch boundary {
			case "end":
				m.EndFreshKey()
			case "reset":
				m.Reset()
			case "new key":
				m.BeginFreshKey(now, now, map[string]bool{guid: true})
			case "left party":
				m.KeepKeyRoster(map[string]bool{"Player-1-OTHER": true})
				m.KeepKeyRoster(map[string]bool{guid: true, "Player-1-OTHER": true})
			case "changed build":
				inspectedKeyMember(m, guid, now, 264, map[int]int{})
			case "log boundary":
				m.ResetLog(start.Add(5 * time.Minute))
			}
			if e := keyEvidence(m.Snapshot(now), guid, 108271); e.Status == "estimated_ready" {
				t.Fatal("boundary retained the previous run's ready assumption", e)
			}
			if boundary == "changed build" {
				later := now.Add(6 * time.Minute)
				if e := keyEvidence(m.Snapshot(later), guid, 108271); e.Status != "unknown" || !e.ReadyInvalidated {
					t.Fatal("inspection expiry revived invalidated readiness", e)
				}
				m.Observe(Cast{GUID: guid, Spell: 108271, At: later}, later)
				if e := keyEvidence(m.Snapshot(later.Add(3*time.Minute)), guid, 108271); e.Status != "estimated_ready" || e.ReadyInvalidated {
					t.Fatal("new use did not recover after build change", e)
				}
			}
		})
	}
}

func TestSameKeyRetainsSeedsAndBufferedBoundaryButNotUnseenAbilities(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	m := NewModel()
	inspectedKeyMember(m, guid, start, 264, map[int]int{108271: 1})
	m.BeginFreshKey(start, start, map[string]bool{guid: true})
	m.Snapshot(start)
	castAt := start.Add(time.Second)
	m.Observe(Cast{GUID: guid, Spell: 57994, At: castAt}, castAt)
	now := start.Add(35 * time.Minute)
	m.ResetLog(start.Add(-time.Second))
	s := m.Snapshot(now)
	for _, spell := range []int{108271, 57994} {
		if e := keyEvidence(s, guid, spell); e.Status != "estimated_ready" {
			t.Fatal("old buffered boundary erased same-key knowledge", e)
		}
	}
	if e := keyEvidence(s, guid, 642); e.Spell != 0 {
		t.Fatal("unobserved ability invented", e)
	}
	k := kickFixture(now)
	c := DefaultConfig()
	c.KickEnabled = true
	d := KickView(s, c, k, now, now, 123)
	if d.KickBars[1].Status != "estimated_ready" || d.Icons[1].Status != "estimated_ready" {
		t.Fatal("retained readiness did not reach teammate bars/icons", d)
	}
}

func TestSameKeyKeepsExistingRechargeQueue(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-EVOKER"
	m := NewModel()
	m.BeginFreshKey(start, start, map[string]bool{guid: true})
	m.Observe(Cast{GUID: guid, Spell: 363916, At: start}, start)
	e := m.Evidence[guid+"/363916"]
	e.RechargeEnds = []float64{float64(start.Unix() + 90), float64(start.Unix() + 180)}
	m.Evidence[guid+"/363916"] = e
	if got := keyEvidence(m.Snapshot(start.Add(100*time.Second)), guid, 363916); got.Status != "estimated_recharge" || got.EstimatedEnd != e.RechargeEnds[1] {
		t.Fatal("remaining recharge was replaced by Ready", got)
	}
	if got := keyEvidence(m.Snapshot(start.Add(181*time.Second)), guid, 363916); got.Status != "estimated_ready" || got.AvailableCharges != nil {
		t.Fatal("finished recharge did not remain an estimate", got)
	}
}

func TestSameKeyCarriesOnlyAnActivePreKeyTimer(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	for _, age := range []time.Duration{time.Second, 5 * time.Minute} {
		m := NewModel()
		castAt := start.Add(-age)
		m.Observe(Cast{GUID: guid, Spell: 108271, At: castAt}, castAt)
		m.BeginFreshKey(start, start, map[string]bool{guid: true})
		e := keyEvidence(m.Snapshot(start.Add(10*time.Minute)), guid, 108271)
		if (e.Status == "estimated_ready") != (age == time.Second) {
			t.Fatal("pre-key evidence boundary ignored", age, e)
		}
	}
}
