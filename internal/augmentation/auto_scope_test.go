package augmentation

import (
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAutoScopePersistsEventsAndPreservesManualChoice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "display.json")
	initial := `{"schema":1,"x":0.31,"y":0.47,"width":600,"height":300,"locked":true,"scope":"previous","custom":"preserve"}`
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(dir, "reports.json"))
	defer s.Close()
	n := &lookup.NativeMeter{Player: aug, Auto: &lookup.MeterAutoScope{Token: "1800000000:1:1", Scope: "overall", AfterKey: true, OnCombat: true}}
	s.syncAutoScope(n)
	read := func() map[string]any {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]any
		if e = json.Unmarshal(b, &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	m := read()
	if m["scope"] != "overall" || m["width"] != float64(600) || m["height"] != float64(300) || m["locked"] != true || m["x"] != .31 || m["y"] != .47 || m["custom"] != "preserve" {
		t.Fatal(m)
	}
	// A desktop menu selection preserves the token and overrides that event.
	m["scope"] = "previous"
	b, _ := json.Marshal(m)
	_ = os.WriteFile(path, b, 0600)
	s.syncAutoScope(n)
	if read()["scope"] != "previous" {
		t.Fatal("repeated frame undid manual selection")
	}
	restarted := New(filepath.Join(dir, "reports.json"))
	defer restarted.Close()
	restarted.syncAutoScope(n)
	if read()["scope"] != "previous" {
		t.Fatal("Engine restart replayed old event")
	}
	n.Auto = &lookup.MeterAutoScope{Token: "1800000001:2:2", Scope: "boss", OnCombat: true, AfterKey: true}
	s.syncAutoScope(n)
	if read()["scope"] != "boss" {
		t.Fatal("next combat did not select Current")
	}
}
func TestAutoScopeDisabledAndCorruptPlacementStayUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "display.json")
	s := New(filepath.Join(dir, "reports.json"))
	defer s.Close()
	n := &lookup.NativeMeter{Player: aug, Auto: &lookup.MeterAutoScope{Token: "1800000000:1:1", Scope: "overall"}}
	s.syncAutoScope(n)
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("disabled option wrote placement")
	}
	_ = os.WriteFile(path, []byte("{invalid"), 0600)
	n.Auto.Token = "1800000000:1:2"
	n.Auto.AfterKey = true
	s.syncAutoScope(n)
	b, _ := os.ReadFile(path)
	if string(b) != "{invalid" {
		t.Fatal("corrupt data overwritten")
	}
}

func TestAutoScopeOnlyFollowsValidFreshNativeObservations(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	now := time.Now()
	n := nativeFrame(1, true, 100, 200)
	n.Auto = &lookup.MeterAutoScope{Token: "1800000000:1:1", Scope: "boss", OnCombat: true}
	n.Captured = false
	s.ObserveNative(n, now, "")
	if s.placement() != nil {
		t.Fatal("invalid capture changed selection")
	}
	n.Captured = true
	s.ObserveNative(n, now, "")
	if p := s.placement(); p == nil || !p.ScopeOnly || p.Scope != "boss" {
		t.Fatal(p)
	}
	n.Serial = 2
	n.Combat = false
	n.Auto = &lookup.MeterAutoScope{Token: "1800000001:2:2", Scope: "overall", AfterKey: true}
	s.ObserveNative(n, now.Add(time.Second), "")
	if p := s.placement(); p == nil || p.Scope != "overall" {
		t.Fatal(p)
	}
}

func TestFirstAutoEventDoesNotInventOrMoveWindowGeometry(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	now := time.Now()
	n := nativeFrame(1, true, 100, 200)
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	c.AugLiveHybrid = true
	c.AugX = 127
	c.AugY = 319
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}
	s.ObserveNative(n, now, "")
	before := s.Merge(engine.FrameDisplay{}, c, w, now)
	n.Serial = 2
	n.Auto = &lookup.MeterAutoScope{Token: "1800000000:1:1", Scope: "boss", OnCombat: true}
	s.ObserveNative(n, now, "")
	after := s.Merge(engine.FrameDisplay{}, c, w, now)
	if before.Meter == nil || after.Meter == nil || before.Meter.X != after.Meter.X || before.Meter.Y != after.Meter.Y || before.Meter.Width != after.Meter.Width || before.Meter.Height != after.Meter.Height {
		t.Fatal(before.Meter, after.Meter)
	}
}
