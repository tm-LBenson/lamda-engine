package augmentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestPlacementPersistsNormalizedCoordinatesAndLocksWithoutSavedVariables(t *testing.T) {
	dir := t.TempDir()
	s := ready()
	s.path = filepath.Join(dir, "reports.json")
	at := time.Date(2026, 10, 7, 20, 0, 0, 0, time.Local)
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "display.json"), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"schema":1,"x":0.5,"y":0.25,"width":500,"height":250,"locked":true,"scope":"boss"}`)
	v := s.Merge(engine.FrameDisplay{}, c, w, at)
	if v.Meter == nil || v.Meter.X != 960 || v.Meter.Y != 270 || v.Meter.Width != 500 || !v.Meter.Locked {
		t.Fatal(v.Meter)
	}
	write(`{"schema":1,"x":0.1,"y":0.1,"width":400,"height":200,"locked":false,"scope":"boss"}`)
	v = s.Merge(engine.FrameDisplay{}, c, w, at)
	if v.Meter.X != 192 || v.Meter.Y != 108 || v.Meter.Locked {
		t.Fatal(v.Meter)
	}
	for _, bad := range []string{`{`, `{"schema":2}`, `{"schema":1,"x":-1,"y":0,"width":500,"height":200,"scope":"boss"}`, `{"schema":1,"x":0,"y":0,"width":500,"height":200,"scope":"bad"}`} {
		write(bad)
		v = s.Merge(engine.FrameDisplay{}, c, w, at)
		if v.Meter == nil || v.Meter.X != 20 || v.Meter.Y != 300 {
			t.Fatal("invalid placement did not fall back", bad, v.Meter)
		}
	}
}

func TestMeterIsVisibleWhileWaitingAndHasNoInstructionalFooter(t *testing.T) {
	s := New("")
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}
	v := s.Merge(engine.FrameDisplay{}, c, w, time.Now())
	if !v.Enabled || v.Meter == nil || len(v.Overlays) != 2 {
		t.Fatal(v)
	}
	if !strings.HasPrefix(v.Overlays[0].Text, "LamdaMeters") || v.Overlays[1].Text != "No combat data" {
		t.Fatal(v.Overlays)
	}
}

func TestDetachedMeterKeepsVerifiedBackgroundDataAndIndependentSessionChoice(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "augmentation"), 0700); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(dir, "augmentation", "reports.json"))
	if err := os.WriteFile(filepath.Join(dir, "overlay-windows.json"), []byte(`{"schema":1,"windows":{"meters":{"detached":true}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "augmentation", "display.json"), []byte(`{"schema":1,"x":0.1,"y":0.1,"width":500,"height":300,"locked":false,"scope":"overall"}`), 0600); err != nil {
		t.Fatal(err)
	}
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	c.AugLiveHybrid = false
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080}
	d := s.Merge(engine.FrameDisplay{}, c, w, time.Now())
	if !d.Enabled || d.Meter == nil || d.Meter.NativeHost || d.Meter.Scope != "overall" || d.KeepUIWhenUnfocused {
		t.Fatal(d)
	}
	w.Window = 0
	if got := s.Merge(engine.FrameDisplay{}, c, w, time.Now()); got.Enabled {
		t.Fatal("detachment accepted absent game")
	}
}

func TestPausedCaptureKeepsWindowWithRecordedDataAndExplicitHideStillHides(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNative(nativeFrame(1, false, 100, 200), now, "")
	s.own = "Player-New-Character"
	s.damageMeters = []DamageMeter{{ID: "boss/99/1", Kind: "boss", Owner: s.own, Valid: true, RawTotal: 900, AdjustedTotal: 900, Rows: []DamageRow{{GUID: s.own, Name: "Current", Raw: 900, Adjusted: 900}}}}
	s.ObserveNative(nil, now, "native_meter_capture_paused")
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	c.AugLiveHybrid = true
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}
	d := s.Merge(engine.FrameDisplay{}, c, w, now.Add(time.Minute))
	if d.Meter == nil || len(d.Overlays) < 4 || d.Meter.NativeHost {
		t.Fatal("Paused capture hid recorded data", d)
	}
	for _, v := range d.Overlays {
		if strings.Contains(v.Text, "Ally") {
			t.Fatal("previous character leaked", v)
		}
	}
	s.ObserveNative(nil, now, "native_meter_inactive_spec")
	if s.Merge(engine.FrameDisplay{}, c, w, now).Meter != nil {
		t.Fatal("Explicit hide ignored")
	}
}

func TestBackgroundVisibilityPreferenceAlsoAppliesToMeter(t *testing.T) {
	s := ready()
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080}
	d := s.Merge(engine.FrameDisplay{KeepUIWhenUnfocused: true}, c, w, time.Now())
	if d.Meter == nil {
		t.Fatal("meter ignored the background visibility preference")
	}
	w.Window = 0
	if s.Merge(engine.FrameDisplay{KeepUIWhenUnfocused: true}, c, w, time.Now()).Meter != nil {
		t.Fatal("absent game accepted")
	}
}

func TestUnreadableHoverCannotUndoLastExplicitVisibility(t *testing.T) {
	s := ready()
	now := time.Now()
	s.ObserveNative(nil, now, "native_meter_inactive_spec")
	s.ObserveNative(nil, now, "bridge_bad_checksum")
	if s.nativeAugEligible() {
		t.Fatal("hover/unreadable marker undid explicit Hide")
	}
	s.ObserveNative(nil, now, "native_meter_capture_paused")
	s.ObserveNative(nil, now, "bridge_bad_checksum")
	if !s.nativeAugEligible() {
		t.Fatal("missed read hid a paused meter")
	}
}
