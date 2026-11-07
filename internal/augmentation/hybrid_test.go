package augmentation

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func nativeFrame(serial int, combat bool, own, ally int64) *lookup.NativeMeter {
	return &lookup.NativeMeter{ID: "123:45:1", Serial: serial, SessionID: 68, Player: aug, Spec: 1473, Combat: combat, DurationMillis: 10000, Captured: true,
		Members: []lookup.NativeMeterMember{{AuraMode: 15, AuraMask: 1, Slot: 1, GUID: aug, Class: 13, Name: "Aug", Total: own}, {AuraMode: 15, AuraMask: 1, Slot: 2, GUID: other, Class: 8, Name: "Ally", Total: ally}}}
}

func TestSoloAugNeedsNoSupportEstimateOrRecordedTail(t *testing.T) {
	s := ready()
	now := time.Now()
	initial := nativeFrame(1, true, 0, 0)
	initial.Members = initial.Members[:1]
	initial.DurationMillis = 0
	s.ObserveNative(initial, now.Add(-10*time.Second), "")
	n := nativeFrame(2, false, 869200, 0)
	n.Members = n.Members[:1]
	s.ObserveNative(n, now, "")
	m := s.HybridView(now)
	if m == nil || !m.Valid || m.Provisional || !m.HasAug || m.InputSource != "native" || m.RawTotal != 869200 || m.AdjustedTotal != 869200 || m.RecordedSupport != 0 || m.EstimatedSupport != 0 {
		t.Fatalf("solo damage waited for nonexistent group support: %+v", m)
	}
	// An ally in the reconciled roster restores the usual attribution checks.
	n = nativeFrame(3, false, 869200, 0)
	s.ObserveNative(n, now.Add(time.Millisecond), "")
	if m = s.HybridView(now.Add(time.Millisecond)); m == nil || !m.Provisional {
		t.Fatalf("group attribution checks bypassed: %+v", m)
	}
	// A completed one-member capture after attachment does not prove that the
	// original party had no Aug. Require the observed start, not today's roster.
	s = ready()
	n = nativeFrame(1, false, 869200, 0)
	n.Members = n.Members[:1]
	s.ObserveNative(n, now, "")
	if m = s.HybridView(now); m == nil || !m.Provisional {
		t.Fatalf("unobserved history was assumed to be solo: %+v", m)
	}
}

func TestOverallKeyMatchUsesExactRosterRatherThanDifferentTimers(t *testing.T) {
	s := ready()
	now := time.Now()
	s.damageMeters = []DamageMeter{{ID: "key/1", Kind: "key", Name: "Finished key", Owner: aug, Valid: true, Seconds: 180, AttributionMode: "all", RawTotal: 300, AdjustedTotal: 300,
		Rows: []DamageRow{{GUID: aug, Raw: 100, Added: 30, Adjusted: 130}, {GUID: other, Raw: 200, Removed: 30, Adjusted: 170}}}}
	n := nativeFrame(1, false, 100, 200)
	n.DurationMillis = 120000
	n.Frame = &lookup.NativeMeterFrame{Scope: "overall"}
	s.ObserveNative(n, now, "")
	m := s.HybridView(now)
	if m == nil || m.Provisional || m.InputSource != "native+recorded" || m.Seconds != 120 || findRow(t, m, aug).Adjusted != 130 {
		t.Fatal(m)
	}
	// Extra damage means this Overall includes something outside that key.
	n = nativeFrame(2, false, 101, 200)
	n.DurationMillis = 120000
	n.Frame = &lookup.NativeMeterFrame{Scope: "overall"}
	s.ObserveNative(n, now.Add(time.Millisecond), "")
	if m = s.HybridView(now.Add(time.Millisecond)); m == nil || !m.Provisional {
		t.Fatalf("different Overall was substituted with a key: %+v", m)
	}
}

func TestPreviousDoesNotConsumeTheNextPullsSupportOrBecomeLive(t *testing.T) {
	s := ready()
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 200, 2, "")
	damageRecord(s, 2, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 30, 395152, aug)
	n := nativeFrame(1, true, 100, 200)
	n.Frame = &lookup.NativeMeterFrame{Scope: "previous"}
	now := time.Date(2026, 10, 26, 16, 0, 10, 0, time.UTC)
	s.ObserveNative(n, now, "")
	m := s.HybridView(now)
	if m == nil || !m.Historical || m.Live || m.RecordedSupport != 0 || m.EstimatedSupport != 0 || !m.Provisional || findRow(t, m, aug).Adjusted != 100 {
		t.Fatal(m)
	}
}
func TestHybridLiveTotalUsesMeasuredPriorAndConservesGroup(t *testing.T) {
	s := ready()
	now := time.Date(2026, 10, 26, 16, 0, 10, 0, time.UTC)
	s.hybridState().calibrations[calibrationKey(aug, other, 1)] = &Calibration{Owner: aug, GUID: other, Mask: 1, Raw: 100, Support: 20}
	initial := nativeFrame(1, true, 0, 0)
	initial.DurationMillis = 0
	s.ObserveNative(initial, now.Add(-10*time.Second), "")
	s.ObserveNative(nativeFrame(2, true, 100, 200), now, "")
	v := s.HybridView(now)
	if !v.Provisional || v.CalibrationMissing || v.EstimatedSupport != 40 || v.RawTotal != 300 || v.AdjustedTotal != 300 {
		t.Fatal(v)
	}
	if findRow(t, v, aug).Adjusted != 140 || findRow(t, v, other).Adjusted != 160 {
		t.Fatal(v.Rows)
	}
	old := v
	s.ObserveNative(nativeFrame(3, true, 90, 190), now.Add(time.Second), "")
	if findRow(t, s.HybridView(now), aug).Raw != 100 || findRow(t, old, aug).Raw != 100 {
		t.Fatal("regression mutated good readings")
	}
	if s.HybridView(now.Add(2 * time.Second)).Valid {
		t.Fatal("stale live feed remained valid")
	}
}
func TestHybridReplacesEstimatedTailAsSupportArrivesIncludingTrash(t *testing.T) {
	s := ready()
	now := time.Date(2026, 10, 26, 16, 0, 10, 0, time.UTC)
	initial := nativeFrame(1, true, 0, 0)
	initial.DurationMillis = 0
	s.ObserveNative(initial, now.Add(-10*time.Second), "")
	s.ObserveNative(nativeFrame(2, true, 100, 200), now, "")
	if !s.HybridView(now).CalibrationMissing {
		t.Fatal("cold start silently invents calibration")
	}
	// No ENCOUNTER_START: normal trash damage must feed the hybrid too.
	auraEvent(s, 1, "SPELL_AURA_APPLIED", aug, other, 395152)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 100, 2, "")
	damageRecord(s, 2, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 25, 2, aug)
	v := s.HybridView(now)
	if v.RecordedSupport != 25 || v.EstimatedSupport != 25 || findRow(t, v, aug).Adjusted != 150 {
		t.Fatal(v)
	}
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 100, 2, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 5, 2, aug)
	v = s.HybridView(now)
	if v.RecordedSupport != 30 || v.EstimatedSupport != 0 || findRow(t, v, aug).Adjusted != 130 || v.RawTotal != v.AdjustedTotal {
		t.Fatal(v)
	}
}
func TestHybridCompletedMatchUsesRecordedSplitAfterRestart(t *testing.T) {
	s := ready()
	now := time.Date(2026, 10, 26, 16, 5, 0, 0, time.UTC)
	s.damageMeters = []DamageMeter{{ID: "boss/1", Name: "Boss", Owner: aug, Valid: true, Seconds: 10, RawTotal: 300, AdjustedTotal: 300,
		Rows: []DamageRow{{GUID: aug, Name: "Aug", Raw: 100, Added: 30, Adjusted: 130}, {GUID: other, Name: "Ally", Raw: 200, Removed: 30, Adjusted: 170}}}}
	s.ObserveNative(nativeFrame(1, false, 100, 200), now, "")
	v := s.HybridView(now)
	if v.Provisional || v.InputSource != "native+recorded" || findRow(t, v, aug).Adjusted != 130 {
		t.Fatal(v)
	}
	if s.HybridView(now.Add(2 * time.Second)).Valid {
		t.Fatal("cached match bypassed stale capture")
	}
}
func TestHybridForeignAugSelfSupportAndInvalidPacketsCannotCreateDamage(t *testing.T) {
	s := ready()
	now := time.Date(2026, 10, 26, 16, 0, 10, 0, time.UTC)
	s.ObserveNative(nativeFrame(1, true, 100, 200), now, "")
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 200, 2, "")
	damageRecord(s, 2, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 80, 2, "Player-1-Foreign")
	damageRecord(s, 2, "SPELL_DAMAGE_SUPPORT", aug, "Creature-1", 50, 2, aug)
	v := s.HybridView(now)
	if v.RecordedSupport != 0 || v.RawTotal != v.AdjustedTotal {
		t.Fatal(v)
	}
	bad := nativeFrame(2, true, 9000, 9000)
	bad.Captured = false
	s.ObserveNative(bad, now.Add(time.Second), "corrupt")
	if findRow(t, s.HybridView(now), aug).Raw != 100 {
		t.Fatal("corruption accepted")
	}
}

func TestHybridDoesNotProjectSupportOntoUnbuffedDamage(t *testing.T) {
	s := ready()
	now := time.Date(2026, 10, 26, 16, 0, 0, 0, time.UTC)
	s.hybridState().calibrations[calibrationKey(aug, other, 1)] = &Calibration{Owner: aug, GUID: other, Mask: 1, Raw: 100, Support: 20}
	initial := nativeFrame(1, true, 0, 0)
	initial.DurationMillis = 0
	for i := range initial.Members {
		initial.Members[i].AuraMask = 0
	}
	s.ObserveNative(initial, now, "")
	n := nativeFrame(2, true, 100, 200)
	for i := range n.Members {
		n.Members[i].AuraMask = 0
	}
	s.ObserveNative(n, now.Add(10*time.Second), "")
	v := s.HybridView(now.Add(10 * time.Second))
	if v.EstimatedSupport != 0 || findRow(t, v, aug).Adjusted != 100 {
		t.Fatal(v)
	}
	n = nativeFrame(3, true, 100, 300)
	n.DurationMillis = 11000
	s.ObserveNative(n, now.Add(11*time.Second), "")
	v = s.HybridView(now.Add(11 * time.Second))
	if v.EstimatedSupport != 20 || v.RawTotal != v.AdjustedTotal {
		t.Fatal(v)
	}
}

func TestPublicSharingControlDoesNotBecomeDamageOrChangeCollectorIdentity(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNative(nativeFrame(1, true, 100, 200), now, "")
	s.ObserveNative(&lookup.NativeMeter{ControlOnly: true, Player: "Player-Other-Character", ID: "share:preview"}, now.Add(time.Second), "")
	v := s.HybridView(now.Add(time.Second))
	if v == nil || v.RawTotal != 300 || v.Owner != aug || s.own != aug || v.NativeSessionID != 68 {
		t.Fatal(v, s.own)
	}
	if s.hybridState().error != "native_meter_capture_paused" {
		t.Fatal(s.hybridState().error)
	}
}
