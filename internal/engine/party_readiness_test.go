package engine

import (
	"slices"
	"testing"
	"time"
)

func TestObservedPartyCooldownsStayEstimatedReadyWithoutAKey(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	for _, tc := range []struct {
		spell  int
		source string
		delay  time.Duration
	}{{6552, "", 24 * time.Second}, {109304, "", 25 * time.Second}, {22812, "aura_activation", 0}, {363916, "unit_spellcast", 0}} {
		m := NewModel()
		m.TrackPartyRoster(map[string]bool{guid: true}, start)
		cast := Cast{GUID: guid, Spell: tc.spell, At: start.Add(time.Second), Source: tc.source}
		m.Observe(cast, cast.At.Add(tc.delay))
		end := keyEvidence(m.Snapshot(cast.At.Add(tc.delay)), guid, tc.spell).EstimatedEnd
		for _, elapsed := range []time.Duration{10 * time.Minute, 35 * time.Minute} {
			now := start.Add(elapsed)
			m.TrackPartyRoster(map[string]bool{guid: true}, now)
			e := keyEvidence(m.Snapshot(now), guid, tc.spell)
			if e.Status != "estimated_ready" || e.EstimatedEnd != end || e.AvailableCharges != nil || !slices.Contains(e.Reasons, "same_party_ready_estimate") || slices.Contains(e.Reasons, "observation_stale") {
				t.Fatal("party observation expired or invented charge information", e)
			}
		}
		cast.At = start.Add(36 * time.Minute)
		m.Observe(cast, cast.At)
		e := keyEvidence(m.Snapshot(cast.At), guid, tc.spell)
		if e.Status != "estimated_cooldown" && e.Status != "estimated_recharge" {
			t.Fatal("new cast did not replace Ready", e)
		}
	}
}

func TestPartyKnownAbilitiesWarmUpWithoutAFirstUse(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	m := NewModel()
	inspectedKeyMember(m, guid, start, 262, map[int]int{108271: 1, 57994: 1})
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	if len(m.Snapshot(start.Add(11*time.Second)).Cooldowns) != 0 {
		t.Fatal("seed before a full cooldown")
	}
	now := start.Add(12 * time.Second)
	e := keyEvidence(m.Snapshot(now), guid, 57994)
	if e.Status != "estimated_ready" || e.Kind != "party_ready" || !slices.Contains(e.Reasons, "full_cooldown_in_party") {
		t.Fatal(e)
	}
	if len(m.Seen) != 0 || m.Diagnostics.AcceptedCasts != 0 {
		t.Fatal("warm-up fabricated a cast")
	}
	if keyEvidence(m.Snapshot(now), guid, 108271).Spell != 0 {
		t.Fatal("defensive used kick duration")
	}
	// Delayed first inspection can use time already spent observing this party.
	other := "Player-1-LATE"
	m.TrackPartyRoster(map[string]bool{guid: true, other: true}, start)
	inspectedKeyMember(m, other, start.Add(20*time.Second), 262, map[int]int{57994: 1})
	if keyEvidence(m.Snapshot(start.Add(20*time.Second)), other, 57994).Status != "estimated_ready" {
		t.Fatal("late lookup reset party clock")
	}
	m.Observe(Cast{GUID: guid, Spell: 57994, At: now.Add(200 * time.Millisecond)}, now.Add(200*time.Millisecond))
	if e := keyEvidence(m.Snapshot(now.Add(time.Second)), guid, 57994); e.Kind != "cast" || e.Status != "estimated_cooldown" {
		t.Fatal("first cast did not replace seed", e)
	}
	if e := keyEvidence(m.Snapshot(start.Add(35*time.Minute)), guid, 108271); e.Status != "estimated_ready" || e.AvailableCharges != nil {
		t.Fatal("known ability expired", e)
	}
}

func TestPartyWarmupOwnershipBuildChangesAndLongCooldowns(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 262, map[int]int{57994: 1, 108271: 1})
	// A repeated identical inspection must not keep restarting the clock.
	inspectedKeyMember(m, guid, start.Add(10*time.Second), 262, map[int]int{57994: 1, 108271: 1})
	if keyEvidence(m.Snapshot(start.Add(12*time.Second)), guid, 57994).Status != "estimated_ready" {
		t.Fatal("unchanged inspection slid clock")
	}
	changed := start.Add(20 * time.Second)
	inspectedKeyMember(m, guid, changed, 262, map[int]int{57994: 1})
	if keyEvidence(m.Snapshot(changed), guid, 57994).Status == "estimated_ready" {
		t.Fatal("build change inherited Ready")
	}
	if e := keyEvidence(m.Snapshot(changed.Add(12*time.Second)), guid, 57994); e.Status != "estimated_ready" || !e.EventAt.Equal(changed) {
		t.Fatal(e)
	}
	if keyEvidence(m.Snapshot(start.Add(5*time.Minute)), guid, 108271).Spell != 0 {
		t.Fatal("unselected defensive seeded")
	}
	// Long cooldowns can finish after the ordinary five-minute inspect cache TTL.
	m = NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 70, map[int]int{642: 1})
	duration := Catalog[642].Cooldown
	m.Snapshot(start.Add(6 * time.Minute))
	if e := keyEvidence(m.Snapshot(start.Add(time.Duration(duration+360)*time.Second)), guid, 642); e.Status != "estimated_ready" {
		t.Fatal("inspection expiry erased knowledge", e)
	}
}

func TestPartyWarmupNeverOverwritesActiveEvidenceAndRespectsTalentDuration(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 262, map[int]int{57994: 1, 108271: 1})
	castAt := start.Add(10 * time.Second)
	m.Observe(Cast{GUID: guid, Spell: 57994, At: castAt}, castAt)
	e := keyEvidence(m.Snapshot(start.Add(12*time.Second)), guid, 57994)
	if e.Kind != "cast" || e.Status != "estimated_cooldown" {
		t.Fatal("warm-up erased use", e)
	}
	// A pre-party expired use does not prevent a new warm-up reaching Ready.
	m.Observe(Cast{GUID: guid, Spell: 108271, At: start.Add(-time.Second)}, start)
	if e := keyEvidence(m.Snapshot(start.Add(121*time.Second)), guid, 108271); e.Kind != "party_ready" || e.Status != "estimated_ready" {
		t.Fatal(e)
	}
	// Each modeled passive modifier participates in warm-up timing.
	for _, rule := range referenceRules.Rules {
		if rule.Mode != "seconds" && rule.Mode != "percent" {
			continue
		}
		spell, known := trackedSpell(rule.Spell)
		if !known || spell.Cooldown <= 0 {
			continue
		}
		m = NewModel()
		m.TrackPartyRoster(map[string]bool{guid: true}, start)
		inspectedKeyMember(m, guid, start, 262, map[int]int{rule.Spell: 1, rule.Talent: 1})
		expected := CooldownEvidence{Spell: rule.Spell, EventAt: start, BaselineSeconds: spell.Cooldown, CooldownSeconds: spell.Cooldown}
		applyBuildRules(&expected, m.partyBuilds[guid].Build)
		now := time.UnixMilli(int64(expected.EstimatedEnd * 1000))
		if e := keyEvidence(m.Snapshot(now.Add(-time.Millisecond)), guid, rule.Spell); e.Status == "estimated_ready" {
			t.Fatal("early modified readiness", rule, e)
		}
		if e := keyEvidence(m.Snapshot(now), guid, rule.Spell); e.Status != "estimated_ready" {
			t.Fatal("modified warm-up missed", rule, e)
		}
	}
}

func TestPartyReadinessEndsOnDepartureBuildChangeAndReset(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	for _, boundary := range []string{"departure", "disband", "build", "reset"} {
		t.Run(boundary, func(t *testing.T) {
			m := NewModel()
			inspectedKeyMember(m, guid, start, 264, map[int]int{108271: 1})
			m.TrackPartyRoster(map[string]bool{guid: true, "Player-1-OTHER": true}, start)
			m.Observe(Cast{GUID: guid, Spell: 108271, At: start.Add(time.Second)}, start.Add(time.Second))
			now := start.Add(10 * time.Minute)
			if keyEvidence(m.Snapshot(now), guid, 108271).Status != "estimated_ready" {
				t.Fatal("test did not establish readiness")
			}
			switch boundary {
			case "departure":
				m.TrackPartyRoster(map[string]bool{"Player-1-OTHER": true}, now)
				m.TrackPartyRoster(map[string]bool{guid: true, "Player-1-OTHER": true}, now.Add(time.Second))
			case "disband":
				m.TrackPartyRoster(map[string]bool{}, now)
			case "build":
				inspectedKeyMember(m, guid, now, 264, map[int]int{})
				if e := keyEvidence(m.Snapshot(now), guid, 108271); e.Status != "unknown" || !e.ReadyInvalidated {
					t.Fatal("changed build did not invalidate readiness", e)
				}
			case "reset":
				m.Reset()
			}
			if e := keyEvidence(m.Snapshot(now.Add(time.Second)), guid, 108271); e.Status == "estimated_ready" {
				t.Fatal("old readiness survived boundary", e)
			}
		})
	}
}

func TestBufferedLogBoundaryPreservesNewerPartyUse(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	for _, source := range []string{"unit_spellcast", "aura_activation", ""} {
		m := NewModel()
		m.TrackPartyRoster(map[string]bool{guid: true}, start)
		m.Observe(Cast{GUID: guid, Spell: 108271, At: start.Add(2 * time.Second), Source: source}, start.Add(3*time.Second))
		m.ResetLog(start.Add(time.Second))
		if e := keyEvidence(m.Snapshot(start.Add(10*time.Minute)), guid, 108271); e.Status != "estimated_ready" {
			t.Fatal("delayed boundary erased a later use", source, e)
		}
	}
}

func TestPartyReadinessDoesNotChangeForever(t *testing.T) {
	before := Forever
	Forever = true
	defer func() { Forever = before }()
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{"Player-1-PARTY": true}, time.Now())
	if len(m.partyMembers) != 0 {
		t.Fatal("Retail party policy enabled for Forever")
	}
}

func TestPartyWarmupChargesBaselineAndRemoval(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-PARTY"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 1473, map[int]int{363916: 1})
	now := start.Add(10 * time.Minute)
	if e := keyEvidence(m.Snapshot(now), guid, 363916); e.Status != "estimated_ready" || e.AvailableCharges != nil {
		t.Fatal("charge readiness/capacity", e)
	}
	m.TrackPartyRoster(map[string]bool{}, now)
	if len(m.partyBuilds) != 0 || len(m.Snapshot(now).Cooldowns) != 0 {
		t.Fatal("synthetic readiness leaked after departure")
	}
	m.TrackPartyRoster(map[string]bool{guid: true}, now.Add(time.Second))
	if len(m.Snapshot(now.Add(2*time.Second)).Cooldowns) != 0 {
		t.Fatal("old inspection seeded returning member")
	}
	inspectedKeyMember(m, guid, now.Add(2*time.Second), 264, map[int]int{57994: 1})
	if keyEvidence(m.Snapshot(now.Add(30*time.Second)), guid, 57994).Spell != 0 {
		t.Fatal("restoration spec cooldown ignored")
	}
	if keyEvidence(m.Snapshot(now.Add(31*time.Second)), guid, 57994).Status != "estimated_ready" {
		t.Fatal("restoration spec warm-up failed")
	}
	// Baseline ownership is enough, while optional abilities need a talent rank.
	m = NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 73, map[int]int{})
	if e := keyEvidence(m.Snapshot(start.Add(15*time.Second)), guid, 6552); e.Status != "estimated_ready" {
		t.Fatal("baseline interrupt missing", e)
	}
}
