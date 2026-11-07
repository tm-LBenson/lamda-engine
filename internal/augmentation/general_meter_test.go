package augmentation

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestHealerClientCreditsMultiplePartyAugsAndConservesNativeDamage(t *testing.T) {
	s := ready()
	now := time.Date(2026, 10, 26, 16, 0, 10, 0, time.UTC)
	a, b := "Player-1-AugA", "Player-1-AugB"
	spec(s, 1, aug, 1468)
	spec(s, 1, a, 1473)
	spec(s, 1, b, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 300, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", a, "Creature-1", 50, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", b, "Creature-1", 60, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 40, 395152, a)
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 30, 395152, b)
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", aug, "Creature-1", 20, 395152, a)
	n := nativeFrame(1, true, 100, 300)
	n.Spec = 1468
	n.Members = append(n.Members, lookup.NativeMeterMember{Slot: 3, GUID: a, Class: 13, Name: "Aug A", Total: 50}, lookup.NativeMeterMember{Slot: 4, GUID: b, Class: 13, Name: "Aug B", Total: 60})
	s.ObserveNative(n, now, "")
	v := s.HybridView(now)
	if v == nil || v.RawTotal != 510 || v.AdjustedTotal != 510 || findRow(t, v, aug).Adjusted != 80 || findRow(t, v, other).Adjusted != 230 || findRow(t, v, a).Adjusted != 110 || findRow(t, v, b).Adjusted != 90 {
		t.Fatal(v)
	}
}

func TestHealingNativeMetricDoesNotReuseDamageSupportOrRegressAfterSwitch(t *testing.T) {
	s := ready()
	now := time.Now()
	n := nativeFrame(1, true, 10000, 20000)
	n.Spec = 1468
	s.ObserveNative(n, now, "")
	healing := nativeFrame(2, true, 500, 100)
	healing.Spec = 1468
	healing.Metric = 2
	healing.Frame = &lookup.NativeMeterFrame{Width: 380, Height: 220}
	healing.Border = &lookup.MeterBorderLayout{InnerWidth: 380, InnerHeight: 220}
	healing.Captured = false
	s.ObserveNative(healing, now.Add(time.Second), "")
	pending, _ := s.selectedMeter("boss", true, now.Add(time.Second))
	if pending == nil || pending.Metric != 2 || pending.Valid || len(pending.Rows) != 0 {
		t.Fatal("old damage leaked into a pending healing view", pending)
	}
	healing.Captured = true
	s.ObserveNative(healing, now.Add(time.Second), "")
	v := s.HybridView(now.Add(time.Second))
	if v == nil || v.Metric != 2 || v.RawTotal != 600 || v.AdjustedTotal != 600 || v.Provisional || findRow(t, v, aug).Added != 0 || findRow(t, v, aug).DPS != 50 {
		t.Fatal(v)
	}
	current, _ := s.selectedMeter("boss", true, now.Add(time.Second))
	if current.Metric != 2 {
		t.Fatal(current)
	}
	n.Serial = 3
	s.ObserveNative(n, now.Add(2*time.Second), "")
	if s.HybridView(now.Add(2*time.Second)).Metric != 0 {
		t.Fatal("damage switch lost")
	}
}

func TestNonAugKeyIsPersistedAndExposedAsActiveDuringCollection(t *testing.T) {
	s := ready()
	spec(s, 1, aug, 1468)
	ingest(s, 2, "CHALLENGE_MODE_START", "Fixture", "123", "1", "10")
	damageRecord(s, 3, "SPELL_DAMAGE", aug, "Creature-1", 200, 1, "")
	state := s.Snapshot()
	if len(state.ActiveSegments) != 1 || state.ActiveSegments[0].Kind != "key" {
		t.Fatal(state.ActiveSegments)
	}
	ingest(s, 10, "CHALLENGE_MODE_END", "123", "1")
	if len(s.damageMeters) != 1 || s.damageMeters[0].RawTotal != 200 || !s.damageMeters[0].Valid || s.damageMeters[0].AttributionMode != "all" {
		t.Fatal(s.damageMeters)
	}
}
