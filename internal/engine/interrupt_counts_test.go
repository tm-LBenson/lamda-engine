package engine

import (
	"fmt"
	"testing"
	"time"
)

func interruptLine(at time.Time, event, guid, target string, spell int) string {
	return fmt.Sprintf("%s  %s,%s,\"Teammate\",0x512,0x0,%s,\"Enemy\",0xa48,0x0,%d,\"Kick\",1,12345,\"Enemy cast\",4", at.Format("1/2/2006 15:04:05.000-07:00"), event, guid, target, spell)
}

func requireInterruptFraction(t *testing.T, m *Model, at time.Time, guid string, hits, attempts int) {
	t.Helper()
	s := m.Snapshot(at)
	if s.InterruptCounts[guid] != hits || s.InterruptAttempts[guid] != attempts {
		t.Fatalf("got %d/%d, want %d/%d", s.InterruptCounts[guid], s.InterruptAttempts[guid], hits, attempts)
	}
}

// These exact cast/effect pairs appeared in the September 28 Kings' Rest log.
// Duplicate aliases must neither hide a hit nor inflate the denominator.
func TestRetailSkullBashAndSilenceEffectIDs(t *testing.T) {
	at := time.Unix(1790628420, 0)
	for _, tc := range []struct {
		main, effect int
		dualCast     bool
	}{
		{106839, 93985, true}, {15487, 220543, false},
	} {
		m := NewModel()
		guid := "Player-1-TEST"
		for i := 0; i < 2; i++ {
			castAt := at.Add(time.Duration(i) * 35 * time.Second)
			m.Ingest(interruptLine(castAt, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", tc.main), castAt.Add(50*time.Second), false)
			if tc.dualCast {
				m.Ingest(interruptLine(castAt.Add(time.Millisecond), "SPELL_CAST_SUCCESS", guid, "Creature-1-A", tc.effect), castAt.Add(50*time.Second), false)
			}
			if i == 0 {
				m.Ingest(interruptLine(castAt.Add(2*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", tc.effect), castAt.Add(50*time.Second), false)
				m.Ingest(interruptLine(castAt.Add(2*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", tc.effect), castAt.Add(51*time.Second), false)
			}
		}
		requireInterruptFraction(t, m, at.Add(90*time.Second), guid, 1, 2)
		if len(m.Evidence) != 1 {
			t.Fatal("alias made separate cooldown", m.Evidence)
		}
	}
}

func TestTrackingAuditExplainsRejectedIDAndSurvivesReset(t *testing.T) {
	m := NewModel()
	var events []TrackingEvent
	m.TrackingAudit = func(e TrackingEvent) { events = append(events, e) }
	at := time.Unix(1790628420, 0)
	guid := "Player-1-TEST"
	m.Ingest(interruptLine(at, "SPELL_INTERRUPT", guid, "Creature-1-A", 9999999), at, false)
	if len(events) != 1 || events[0].Outcome != "unsupported_interrupt_id" || events[0].Applied {
		t.Fatal(events)
	}
	m.Reset()
	m.Ingest(interruptLine(at, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 15487), at.Add(50*time.Second), false)
	m.Ingest(interruptLine(at, "SPELL_INTERRUPT", guid, "Creature-1-A", 220543), at.Add(50*time.Second), false)
	last := events[len(events)-1]
	if last.Kind != "interrupt_landed" || !last.Applied || last.Landed != 1 || last.Attempts != 1 || last.Canonical != 15487 {
		t.Fatal(events)
	}
	if events[len(events)-2].Evidence == nil || events[len(events)-2].Evidence.DeliverySeconds != 50 {
		t.Fatal("missing delayed cooldown evidence")
	}
	m.ResetInterruptCounts(at.Add(time.Second))
	if events[len(events)-1].Kind != "interrupt_count_reset" {
		t.Fatal("silent count reset")
	}
}

func TestInterruptCountsUsesDeduplicatedAndIndependentOfCooldown(t *testing.T) {
	at := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, at)
	inspectedKeyMember(m, guid, at, 264, map[int]int{57994: 1})
	m.Snapshot(at.Add(12 * time.Second))
	if len(m.interruptCounts.Totals) != 0 || len(m.interruptCounts.Attempts) != 0 {
		t.Fatal("Ready incremented counter")
	}
	cast := interruptLine(at, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 57994)
	m.Ingest(cast, at, false)
	requireInterruptFraction(t, m, at, guid, 0, 1)
	hit := interruptLine(at, "SPELL_INTERRUPT", guid, "Creature-1-A", 57994)
	if !m.Ingest(hit, at, false) || m.Ingest(hit, at.Add(time.Second), false) {
		t.Fatal("hit acceptance or dedup failed")
	}
	requireInterruptFraction(t, m, at, guid, 1, 1)
	// A late copy cannot create a second cooldown or move the original timestamp.
	if e := keyEvidence(m.Snapshot(at.Add(8*time.Second)), guid, 57994); !e.EventAt.Equal(at) {
		t.Fatal(e)
	}
	m.Ingest(interruptLine(at, "SPELL_INTERRUPT", guid, "Creature-1-B", 57994), at, false)
	requireInterruptFraction(t, m, at, guid, 1, 1)
	s := m.Snapshot(at)
	s.InterruptCounts[guid] = 99
	s.InterruptAttempts[guid] = 100
	requireInterruptFraction(t, m, at, guid, 1, 1)
	m.DisabledInterrupts = true
	m.Ingest(interruptLine(at.Add(time.Second), "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), at.Add(time.Second), false)
	m.Observe(Cast{GUID: guid, Spell: 57994, At: at.Add(2 * time.Second)}, at.Add(2*time.Second))
	requireInterruptFraction(t, m, at, guid, 1, 1)
}

func TestInterruptCountBoundaryPreservesNewerHitsAndLogRotation(t *testing.T) {
	at := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	m := NewModel()
	for _, seconds := range []int{0, 2} {
		now := at.Add(time.Duration(seconds) * time.Second)
		m.Ingest(interruptLine(now, "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), now, false)
	}
	m.Ingest(interruptLine(at.Add(4*time.Second), "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 57994), at.Add(4*time.Second), false)
	m.ResetLog(at)
	requireInterruptFraction(t, m, at.Add(4*time.Second), guid, 2, 3)
	m.ResetInterruptCounts(at.Add(time.Second))
	requireInterruptFraction(t, m, at.Add(4*time.Second), guid, 1, 2)
	m.ResetInterruptCounts(at)
	m.Ingest(interruptLine(at, "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), at.Add(4*time.Second), false)
	requireInterruptFraction(t, m, at.Add(4*time.Second), guid, 1, 2)
	m.BeginFreshKey(at.Add(6*time.Second), at.Add(6*time.Second), map[string]bool{guid: true})
	if len(m.interruptCounts.Totals) != 0 || len(m.interruptCounts.Attempts) != 0 {
		t.Fatal("key start did not reset count")
	}
}

func TestInterruptCountsPetOwnershipAndOptionalBarColumn(t *testing.T) {
	at := time.Unix(1789500000, 0)
	k := kickFixture(at)
	m := NewModel()
	guid := k.Members[1].GUID
	k.Members[1].Class = 9
	k.Members[1].Spec = 265
	k.Members[1].Pet = "Pet-1-DEMON"
	roster := map[string]bool{guid: true}
	line := interruptLine(at, "SPELL_INTERRUPT", "Pet-1-DEMON", "Creature-1-A", 19647)
	if !m.IngestTeam(line, at, false, roster, k, at) || m.IngestTeam(line, at, false, roster, k, at) {
		t.Fatal("owned pet count or dedup")
	}
	if m.interruptCounts.Totals[guid] != 1 || m.interruptCounts.Totals["Pet-1-DEMON"] != 0 {
		t.Fatal("pet not credited to owner")
	}
	// The owner's command and pet cast are aliases for this same use. Repeated
	// deliveries must not add attempts, even when the timestamps differ.
	for repeat := 0; repeat < 2; repeat++ {
		m.IngestTeam(interruptLine(at, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 119910), at, false, roster, k, at)
		m.IngestTeam(interruptLine(at.Add(10*time.Millisecond), "SPELL_CAST_SUCCESS", "Pet-1-DEMON", "Creature-1-A", 19647), at.Add(time.Second), false, roster, k, at)
	}
	requireInterruptFraction(t, m, at.Add(time.Second), guid, 1, 1)
	m.IngestTeam(interruptLine(at, "SPELL_CAST_SUCCESS", "Pet-1-UNVERIFIED", "Creature-1-A", 19647), at, false, roster, k, at)
	if len(m.interruptCounts.Attempts) != 1 {
		t.Fatal("unverified pet counted")
	}
	c := DefaultConfig()
	c.KickEnabled = true
	for _, enabled := range []bool{false, true} {
		k.Counts = enabled
		k.X = 0
		d := KickView(m.Snapshot(at), c, k, at, at, 123)
		for _, bar := range d.KickBars {
			if (bar.Count != nil) != enabled || (bar.Attempts != nil) != enabled {
				t.Fatal("optional counter", bar)
			}
			if enabled && (bar.X < 28 || (bar.GUID == guid && (*bar.Count != 1 || *bar.Attempts != 1))) {
				t.Fatal("count placement/value", bar)
			}
		}
	}
	k.Preview = true
	if d := KickView(m.Snapshot(at), c, k, at, at, 123); d.KickBars[0].Count == nil || *d.KickBars[0].Count == 0 || *d.KickBars[0].Attempts <= *d.KickBars[0].Count {
		t.Fatal("preview missing count")
	}
	requireInterruptFraction(t, m, at, guid, 1, 1)
}

func TestInterruptFractionThreeOfFourAndLateResult(t *testing.T) {
	at := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	m := NewModel()
	for i := 0; i < 4; i++ {
		castAt := at.Add(time.Duration(i) * 30 * time.Second)
		// First learn the use live, then receive its buffered log copy.
		m.Observe(Cast{GUID: guid, Spell: 57994, At: castAt.Add(400 * time.Millisecond), Source: "unit_spellcast"}, castAt.Add(time.Second))
		m.Ingest(interruptLine(castAt, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 57994), castAt.Add(time.Second), false)
		if i < 3 {
			m.Ingest(interruptLine(castAt.Add(20*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), castAt.Add(time.Second), false)
		}
	}
	now := at.Add(2 * time.Minute)
	requireInterruptFraction(t, m, now, guid, 3, 4)
	m.Ingest(interruptLine(at.Add(90*time.Second), "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), now, false)
	requireInterruptFraction(t, m, now, guid, 4, 4)
}

func TestInterruptFractionDeliveryOrder(t *testing.T) {
	for _, order := range []string{"LCH", "LHC", "CLH", "CHL", "HLC", "HCL"} {
		t.Run(order, func(t *testing.T) {
			at := time.Unix(1789500000, 0)
			guid := "Player-1-SHAMAN"
			m := NewModel()
			// A newer cast has already advanced the timer. Older records must
			// still be counted, independently of timer duplicate rejection.
			m.Observe(Cast{GUID: guid, Spell: 57994, At: at.Add(time.Minute)}, at.Add(time.Minute))
			for repeat := 0; repeat < 2; repeat++ {
				for _, source := range order {
					switch source {
					case 'L':
						m.Observe(Cast{GUID: guid, Spell: 57994, At: at.Add(500 * time.Millisecond), Source: "unit_spellcast"}, at.Add(time.Minute))
					case 'C':
						m.Ingest(interruptLine(at, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 57994), at.Add(time.Minute), false)
					case 'H':
						m.Ingest(interruptLine(at.Add(10*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), at.Add(time.Minute), false)
					}
				}
			}
			requireInterruptFraction(t, m, at.Add(time.Minute), guid, 1, 2)
			if e := keyEvidence(m.Snapshot(at.Add(time.Minute)), guid, 57994); !e.EventAt.Equal(at.Add(time.Minute)) {
				t.Fatal("older count moved current timer", e)
			}
		})
	}
}

func TestInterruptFractionDistinctFastCasts(t *testing.T) {
	at := time.Unix(1789500000, 0)
	guid := "Player-1-SHAMAN"
	m := NewModel()
	for _, castAt := range []time.Time{at, at.Add(time.Second)} {
		m.Ingest(interruptLine(castAt, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", 57994), at.Add(2*time.Second), false)
	}
	m.Ingest(interruptLine(at.Add(1010*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", 57994), at.Add(2*time.Second), false)
	m.Ingest(interruptLine(at.Add(1010*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-B", 57994), at.Add(2*time.Second), false)
	requireInterruptFraction(t, m, at.Add(2*time.Second), guid, 1, 2)
}

func TestInterruptFractionRejectsNonUses(t *testing.T) {
	at := time.Unix(1789500000, 0)
	m := NewModel()
	for _, cast := range []Cast{
		{GUID: "Player-1-A", Spell: 57994, At: at, Source: "aura_activation"},
		{GUID: "Player-1-A", Spell: 57994, At: at, Companion: true},
		{GUID: "Player-1-A", Spell: 363916, At: at},
		{GUID: "Player-1-A", Spell: 57994, At: at.Add(3 * time.Second)},
		{GUID: "Player-1-A", Spell: 57994, At: at.Add(-observationRetention)},
		{Spell: 57994, At: at},
	} {
		m.Observe(cast, at)
	}
	if len(m.interruptCounts.Attempts) != 0 || len(m.interruptCounts.Totals) != 0 {
		t.Fatal("unobserved use counted", m.interruptCounts)
	}
}
