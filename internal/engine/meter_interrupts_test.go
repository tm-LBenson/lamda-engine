package engine

import (
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"testing"
	"time"
)

func TestMeterLandedUseReconcilesDelayedLogWithoutResettingTimerOrCount(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	guid := "Player-1-A"
	m := NewModel()
	c := Cast{GUID: guid, Spell: 2139, At: now, Source: "damage_meter", RosterMatched: true}
	if !m.ObserveMeterInterrupt(c, now.Add(250*time.Millisecond)) {
		t.Fatal("not accepted")
	}
	requireInterruptFraction(t, m, now, guid, 1, 1)
	before := m.Snapshot(now).Cooldowns[0]
	// Native meter refresh can lag the cast timestamp by over a second.
	castAt := now.Add(-1800 * time.Millisecond)
	for _, event := range []string{"SPELL_CAST_SUCCESS", "SPELL_INTERRUPT"} {
		m.Ingest(interruptLine(castAt, event, guid, "Creature-1-A", 2139), now.Add(66*time.Second), false)
	}
	requireInterruptFraction(t, m, now.Add(66*time.Second), guid, 1, 1)
	after := m.Snapshot(now).Cooldowns[0]
	if after.EventAt != before.EventAt || after.EstimatedEnd != before.EstimatedEnd {
		t.Fatal("late log restarted meter cooldown", before, after)
	}
	m.ResetLog(now.Add(-time.Second))
	if len(m.Evidence) != 1 {
		t.Fatal("delayed log boundary lost live meter evidence")
	}
}

func TestMeterAbilityInferenceRejectsAlternateAndPersistentInterrupts(t *testing.T) {
	now := time.Now()
	m := NewModel()
	for _, member := range []framewire.KickMember{
		{GUID: "Player-1-P", Class: 2, Spec: 66}, {GUID: "Player-1-D", Class: 11, Spec: 102},
		{GUID: "Player-1-W", Class: 1, Spec: 73}, {GUID: "Player-1-H", Class: 12, Spec: 581},
	} {
		if m.MeterInterruptSpell(member, now) != 0 {
			t.Fatal("ambiguous ability guessed", member)
		}
	}
	for _, member := range []framewire.KickMember{
		{GUID: "Player-1-M", Class: 8, Spec: 64}, {GUID: "Player-1-R", Class: 4, Spec: 260},
		{GUID: "Player-1-D", Class: 11, Spec: 104}, {GUID: "Player-1-W", Class: 1, Spec: 72},
	} {
		if !IsInterrupt(m.MeterInterruptSpell(member, now)) {
			t.Fatal("known standard kick missing", member)
		}
	}
}
