package augmentation

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionSelectionConservesAndDoesNotDuplicateCurrent(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNative(nativeFrame(1, false, 100, 200), now, "")
	v, choices := s.selectedMeter("overall", true, now)
	if v == nil || v.RawTotal != 300 || v.AdjustedTotal != 300 || len(choices) < 5 {
		t.Fatal(v, choices)
	}
	n := nativeFrame(2, true, 50, 70)
	n.SessionID = 69
	s.ObserveNative(n, now.Add(20*time.Second), "")
	v, _ = s.selectedMeter("overall", true, now.Add(20*time.Second))
	if v.RawTotal != 420 || v.AdjustedTotal != 420 || !v.Provisional || !v.Live {
		t.Fatal(v)
	}
	previous, _ := s.selectedMeter("previous", true, now.Add(20*time.Second))
	if previous.NativeSessionID != 68 || previous.RawTotal != 300 || previous.Live {
		t.Fatal(previous)
	}
	chosen, _ := s.selectedMeter("native:68", true, now.Add(20*time.Second))
	if chosen.RawTotal != previous.RawTotal {
		t.Fatal(chosen)
	}
}

func TestOverallCountsMatchedWholeKeyOnceWhileRetainingPullHistory(t *testing.T) {
	at := time.Date(2026, 10, 28, 18, 0, 0, 0, time.UTC)
	key := DamageMeter{Owner: aug, ID: "key/1", Kind: "key", Valid: true, StartedAt: at, EndedAt: at.Add(30 * time.Minute), Seconds: 1800,
		RawTotal: 3000, AdjustedTotal: 3000, Rows: []DamageRow{{GUID: aug, Raw: 1000, Adjusted: 1500}, {GUID: other, Raw: 2000, Adjusted: 1500}}}
	boss := key
	boss.ID = "boss/1"
	boss.Kind = "boss"
	boss.StartedAt = at.Add(time.Minute)
	boss.EndedAt = at.Add(3 * time.Minute)
	boss.RawTotal = 300
	boss.AdjustedTotal = 300
	boss.Seconds = 120
	boss.Rows = []DamageRow{{GUID: aug, Raw: 100, Adjusted: 150}, {GUID: other, Raw: 200, Adjusted: 150}}
	trash := boss
	trash.ID = "native/1"
	trash.Kind = "combat"
	trash.StartedAt = at.Add(5 * time.Minute)
	trash.EndedAt = at.Add(7 * time.Minute)
	before := boss
	before.ID = "boss/previous"
	before.StartedAt = at.Add(-10 * time.Minute)
	before.EndedAt = at.Add(-8 * time.Minute)
	inputs := []DamageMeter{boss, trash, key, before}
	filtered := nonoverlappingMeterSessions(inputs)
	view := sumMeterSessions(aug, filtered)
	if len(inputs) != 4 || len(filtered) != 2 || view.RawTotal != 3300 || view.AdjustedTotal != 3300 || view.Seconds != 1920 {
		t.Fatal(filtered, view)
	}
	// A second retained copy cannot remove both copies or count either twice.
	filtered = nonoverlappingMeterSessions(append(inputs, key))
	if len(filtered) != 2 || sumMeterSessions(aug, filtered).RawTotal != 3300 {
		t.Fatal(filtered)
	}
	// Unknown history and a conflicting source cannot be assigned to a key.
	unknown := trash
	unknown.StartedAt = time.Time{}
	unknown.EndedAt = time.Time{}
	conflict := trash
	conflict.Rows = []DamageRow{{GUID: "Player-1-Unrelated", Raw: 300, Adjusted: 300}}
	if len(nonoverlappingMeterSessions([]DamageMeter{key, unknown, conflict})) != 3 {
		t.Fatal("inferred an unsupported key boundary")
	}
}

func TestNativeHistoryPersistsAndLaterRecordedSplitRepairsSelection(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	now := time.Now()
	s.ObserveNative(nativeFrame(1, false, 100, 200), now, "")
	if err := s.Flush(""); err != nil {
		t.Fatal(err)
	}
	reloaded := New(s.path)
	if len(reloaded.nativeMeters) != 1 || reloaded.nativeMeters[0].RawTotal != 300 {
		t.Fatal(reloaded.nativeMeters)
	}
	n := nativeFrame(2, true, 50, 70)
	n.SessionID = 69
	s.ObserveNative(n, now.Add(20*time.Second), "")
	s.damageMeters = []DamageMeter{{ID: "boss/1", Name: "Recorded boss", Owner: aug, Valid: true, Seconds: 10, RawTotal: 300, AdjustedTotal: 300,
		Rows: []DamageRow{{GUID: aug, Raw: 100, Added: 30, Adjusted: 130}, {GUID: other, Raw: 200, Removed: 30, Adjusted: 170}}}}
	v, _ := s.selectedMeter("previous", true, now.Add(20*time.Second))
	if v.Provisional || findRow(t, v, aug).Adjusted != 130 {
		t.Fatal(v)
	}
}

func TestInitialCaptureFailureIsVisibleInAPI(t *testing.T) {
	s := ready()
	s.ObserveNative(nil, time.Now(), "native_meter_addon: font layout")
	status := s.Snapshot().Hybrid
	if status == nil || status.Error != "native_meter_addon: font layout" || status.Meter != nil {
		t.Fatal(status)
	}
}

func TestIdleZeroSessionDoesNotDiluteOverallRate(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNative(nativeFrame(1, false, 100, 200), now, "")
	n := nativeFrame(2, false, 0, 0)
	n.SessionID = 69
	n.DurationMillis = 9999000
	s.ObserveNative(n, now.Add(time.Second), "")
	v, _ := s.selectedMeter("overall", true, now.Add(time.Second))
	if v == nil || v.RawTotal != 300 || v.Seconds != 10 || len(s.nativeMeters) != 1 {
		t.Fatal(v, s.nativeMeters)
	}
}

func TestHistoricalUnknownSpecDoesNotOverwriteCurrentSpecOrClaimExactSupport(t *testing.T) {
	s := ready()
	now := time.Now()
	s.specs[aug] = 1473
	n := nativeFrame(1, false, 100, 200)
	n.Spec = 0
	s.ObserveNative(n, now, "")
	v := s.HybridView(now)
	if s.specs[aug] != 1473 || v == nil || !v.Provisional || !v.CalibrationMissing {
		t.Fatal(s.specs, v)
	}
}

func TestManualNativeHistoryHasUnknownDatesAndCannotDoubleCountOverall(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNative(nativeFrame(1, false, 100, 200), now, "")
	h := nativeFrame(2, false, 1000, 2000)
	h.SessionID = 69
	h.Frame = &lookup.NativeMeterFrame{Scope: "native:69"}
	s.ObserveNative(h, now.Add(time.Second), "")
	v := s.HybridView(now.Add(time.Second))
	if !v.Historical || !v.StartedAt.IsZero() || !v.EndedAt.IsZero() || v.EstimatedSupport != 0 {
		t.Fatal(v)
	}
	n := nativeFrame(3, false, 0, 0)
	n.SessionID = 70
	s.ObserveNative(n, now.Add(2*time.Second), "")
	overall, _ := s.selectedMeter("overall", true, now.Add(2*time.Second))
	if overall == nil || overall.RawTotal != 300 || overall.Seconds != 10 {
		t.Fatal(overall)
	}
	for _, m := range s.nativeMeters {
		if m.Historical && (!m.StartedAt.IsZero() || !m.EndedAt.IsZero()) {
			t.Fatal("invented historical dates", m)
		}
	}
}
