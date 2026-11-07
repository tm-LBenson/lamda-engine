package augmentation

import (
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestSelectedWindowShareUsesItsOwnCountersWithoutBossBoundary(t *testing.T) {
	s := ready()
	now := time.Now()
	for id := 1; id <= 2; id++ {
		n := windowFrame(id, 0, 1, int64(id*100), int64(id*200))
		n.Spec = 1467
		n.Frame.Scope = "overall"
		n.Frame.Label = "Overall"
		s.ObserveNativeSlot(n, id-1, now, "")
	}
	for _, test := range []struct{ scope, amount string }{{"window:1", "20 DPS"}, {"window:2", "40 DPS"}} {
		v, err := s.PreparedSummaryForPlayer(test.scope, aug, 4)
		if err != nil || !strings.Contains(v.Text, test.amount) || strings.Contains(v.Text, "Aug-adjusted") || strings.Contains(v.Text, "MVP") {
			t.Fatal(test, v, err)
		}
	}
	if _, err := s.PreparedSummaryForPlayer("window:1", aug, 0); err == nil {
		t.Fatal("window scope was accepted for awards")
	}
	if _, err := s.PreparedSummaryForPlayer("window:1", "Player-Different", 4); err == nil {
		t.Fatal("wrong owner was accepted")
	}
}

func windowFrame(id, metric, serial int, own, ally int64) *lookup.NativeMeter {
	n := nativeFrame(serial, false, own, ally)
	n.Metric = metric
	n.Frame = &lookup.NativeMeterFrame{WindowID: id, X: 100 + id*320, Y: 400, Width: 323, Height: 200, Scope: "boss", Label: "Current", HeaderHeight: 45, RowHeight: 18, FontSize: 9, Opacity: 750, Values: "total", ClassColors: true}
	n.Border = &lookup.MeterBorderLayout{Fast: true, InnerWidth: 832, Edges: [4]int{24, 0, 0, 0}}
	return n
}
func TestOverlayWindowsKeepIndependentMetricsCountersStylesAndVisibility(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNativeSlot(windowFrame(1, 2, 1, 100, 200), 0, now, "")
	s.ObserveNativeSlot(windowFrame(2, 0, 1, 400, 500), 1, now, "")
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	c.MeterOverlay = true
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}
	d := s.Merge(engine.FrameDisplay{}, c, w, now)
	if !d.Enabled || len(d.Meters) != 2 || d.Meter != nil || d.Meters[0].Body.Metric != "Healing" || d.Meters[1].Body.Metric != "Damage" {
		t.Fatal(d)
	}
	for _, m := range d.Meters {
		if !m.NativeHost || m.HeaderHeight != 45 || m.Body.RowHeight != 18 || m.Body.FontSize != 9 || m.Body.Opacity != .75 || m.Body.WindowID != m.ID {
			t.Fatal(m)
		}
	}
	if d.Meters[0].Body.Rows[0].Value != "200" {
		t.Fatal("window2 replaced window1's counters", d.Meters[0])
	}
	s.ObserveNativeSlot(&lookup.NativeMeter{ControlOnly: true, ControlHidden: true, Frame: &lookup.NativeMeterFrame{WindowID: 2}}, 1, now, "")
	d = s.Merge(engine.FrameDisplay{}, c, w, now)
	if len(d.Meters) != 1 || d.Meters[0].ID != 1 {
		t.Fatal("hiding one window hid another", d)
	}
	s.ObserveNativeSlot(nil, 0, now, "native_meter_inactive_spec")
	if d := s.Merge(engine.FrameDisplay{}, c, w, now); len(d.Meters) != 0 {
		t.Fatal("global Hidden did not hide every meter", d)
	}
	// Showing a nonprimary window must work with primary still hidden.
	s.ObserveNativeSlot(windowFrame(2, 2, 2, 30, 40), 1, now.Add(time.Second), "")
	d = s.Merge(engine.FrameDisplay{}, c, w, now.Add(time.Second))
	if len(d.Meters) != 2 {
		t.Fatal(d)
	}
}

func TestOverlayPreviewControlDoesNotReplaceWindowCounters(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNativeSlot(windowFrame(1, 2, 1, 100, 200), 0, now, "")
	s.ObserveNativeSlot(&lookup.NativeMeter{ControlOnly: true, ShareCapable: true}, 0, now, "")
	views := s.windowMeters(now)
	if len(views) != 1 || views[0].meter.RawTotal != 300 {
		t.Fatal(views)
	}
	// A mismatched index may not overwrite a different profile window.
	s.ObserveNativeSlot(windowFrame(2, 2, 2, 999, 999), 0, now, "")
	if views := s.windowMeters(now); len(views) != 1 || views[0].meter.RawTotal != 300 {
		t.Fatal(views)
	}
}
