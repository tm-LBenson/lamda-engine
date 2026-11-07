package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"reflect"
	"testing"
	"time"
)

func TestQueueCaptureClearBeforeFirstInvitation(t *testing.T) {
	d := engine.FrameDisplay{Width: 1920, Height: 1080}
	reserveQueueStrip(&d)
	expected := engine.CaptureArea{X: 0, Y: 1016, Width: 128, Height: 16}
	if len(d.CaptureAreas) != 1 || d.CaptureAreas[0] != expected {
		t.Fatal("queue packet must have a native window hole before detection", d.CaptureAreas)
	}
	reserveQueueStrip(&d)
	if len(d.CaptureAreas) != 1 {
		t.Fatal("duplicate capture hole", d.CaptureAreas)
	}
	small := engine.FrameDisplay{Width: 64, Height: 32}
	reserveQueueStrip(&small)
	if len(small.CaptureAreas) != 0 {
		t.Fatal("reserved an unreadable queue packet", small.CaptureAreas)
	}
}

func TestResourcePolicyKeepsEveryReaderAndRestartsOnIdleChange(t *testing.T) {
	c := allModulesConfig()
	c.ColorStripVersion = 2
	normal := append(trackingStrips(c), transferStrips(c)...)
	c.ResourceLow = true
	reduced := append(trackingStrips(c), transferStrips(c)...)
	if len(normal) != len(reduced) || sameStrips(normal, reduced) {
		t.Fatal("policy did not reach readers")
	}
	for i, target := range reduced {
		if target.IdleInterval != time.Second {
			t.Fatal("missing idle policy", target)
		}
		target.IdleInterval = 0
		if target != normal[i] {
			t.Fatal("required source or layout changed")
		}
	}
	c.ResourceAdvanced = true
	if !sameStrips(normal, append(trackingStrips(c), transferStrips(c)...)) {
		t.Fatal("advanced choices not restored")
	}
}

func TestSeparateAndFirstReloadCaptureOrigins(t *testing.T) {
	c := allModulesConfig()
	transfer := transferStrips(c)
	if len(transfer) != 4 || transfer[3].Position != (lookup.BridgePosition{X: 0, Y: 48}) {
		t.Fatal("first reload lost legacy wide reader", transfer)
	}
	c.ColorStripVersion = 2
	c.LookupBridgeX = 23
	c.KickBridgeY = 170
	c.GladiusBridgeX = 700
	c.KBBBridgeY = 800
	c.SimBridgeX = 1300
	targets := trackingStrips(c)
	if targets[0].Source != "cd" || targets[0].Position.X != 23 || targets[1].Source != "kick" || targets[1].Position.Y != 170 {
		t.Fatal(targets)
	}
	if !engine.Forever && (len(targets) != 3 || targets[2].Source != "gladius" || targets[2].Position.X != 700) {
		t.Fatal(targets)
	}
	transfer = transferStrips(c)
	if len(transfer) != 3 || transfer[1].Position.X != 1300 || transfer[2].Position.Y != 800 {
		t.Fatal(transfer)
	}
	old := c
	c.SimBridgeY = 600
	if sameStrips(transferStrips(old), transferStrips(c)) || !sameStrips(trackingStrips(old), trackingStrips(c)) {
		t.Fatal("moving Sim must restart only transfer readers")
	}
}

func TestDefaultStripsLeaveRoomForTextFallbackOnSmallWindow(t *testing.T) {
	c := allModulesConfig()
	c.ColorStripVersion = 2
	targets := append(trackingStrips(c), transferStrips(c)...)
	type box struct{ x, y, w, h int }
	boxes := []box{}
	for _, target := range targets {
		b := box{target.Position.X, target.Position.Y, 128, 16}
		switch target.Source {
		case "cd":
			b.w = 208
			b.h = 24
		case "kick", "gladius":
			b.h = 200
		case "overlays":
			b.w = 256
			b.h = 72
		case "kbb", "sim":
			b.w = 256
			b.h = 32
		}
		if b.x+b.w > 640 || b.y+b.h > 480 {
			t.Fatal("default strip exceeds 640x480", target)
		}
		for _, other := range boxes {
			if b.x < other.x+other.w && b.x+b.w > other.x && b.y < other.y+other.h && b.y+b.h > other.y {
				t.Fatal("default capture panels overlap", target)
			}
		}
		boxes = append(boxes, b)
	}
}

func TestOverlayBandReservedBeforeFirstReading(t *testing.T) {
	for _, origin := range []int{320, 448} {
		c := engine.DefaultConfig()
		c.ColorStripVersion = 2
		c.OverlaysBridgeY = origin
		c.OverlaysEnabled = true
		y := origin + 36
		if origin+72 > 480 {
			y = origin - 36
		}
		d := engine.FrameDisplay{Width: 640, Height: 480, Icons: []engine.FrameIcon{{X: 12, Y: y, Size: 16}}}
		reserveModuleStrips(&d, c, nil, time.Now(), false)
		if len(d.Icons) != 0 {
			t.Fatal("An existing cooldown can occlude the native aura band before calibration")
		}
	}
}

func TestDisabledOverlaysDoNotReserveScreenSpace(t *testing.T) {
	c := engine.DefaultConfig()
	c.ColorStripVersion = 2
	d := engine.FrameDisplay{Width: 640, Height: 480, Icons: []engine.FrameIcon{{X: 12, Y: 324, Size: 16}, {X: 12, Y: 356, Size: 16}}}
	reserveModuleStrips(&d, c, nil, time.Now(), false)
	if len(d.Icons) != 2 {
		t.Fatal("Disabled module hid another module's icons")
	}
}

func TestReshapeRestartsOnlyTransferReadersAndReservesFullShape(t *testing.T) {
	c := engine.DefaultConfig()
	c.ColorStripVersion, c.OverlaysEnabled = 2, true
	before := c
	for _, width := range []int{16, 64, 128, 256, 512, 1152} {
		c.OverlaysBridgeWidth = width
		c.OverlaysBridgeX, c.OverlaysBridgeY = 15900, 15900
		if sameStrips(transferStrips(before), transferStrips(c)) || !sameStrips(trackingStrips(before), trackingStrips(c)) {
			t.Fatal("shape must restart only transfer readers")
		}
		d := engine.FrameDisplay{Width: 1920, Height: 1200, Icons: []engine.FrameIcon{{X: 1910, Y: 1190, Size: 8}}}
		reserveModuleStrips(&d, c, nil, time.Now(), false)
		if len(d.Icons) != 0 {
			t.Fatal("shaped capture area was occluded", width)
		}
	}
}

func TestManaAlertSurvivesSmallCaptureIntersection(t *testing.T) {
	c := engine.DefaultConfig()
	c.ColorStripVersion, c.OverlaysEnabled = 2, true
	c.OverlaysBridgeWidth, c.OverlaysBridgeY = 1152, 1064
	c.KBBBridgeX, c.KBBBridgeY = 1155, 1047
	// This alert's empty upper-left corner touches Sim's reserved rectangle.
	// A capture reservation must not discard the whole visible reminder.
	v := engine.OverlayDisplay{Kind: "text", Text: "NEED TO DRINK!!!!!", Size: 46, Font: 59,
		X: 25.663325471698112, Y: 23.045987060674943, Opacity: 100, Transparent: true}
	d := engine.FrameDisplay{Width: 1920, Height: 1080, Overlays: []engine.OverlayDisplay{v}}
	reserveModuleStrips(&d, c, nil, time.Now(), false)
	if len(d.Overlays) != 1 || d.Overlays[0] != v || d.Omitted != 0 {
		t.Fatalf("capture intersection hid or moved the alert: %+v", d)
	}
}

func TestCaptureHolesCoverWholeNativeSnapshot(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name      string
		screen    *lookup.ScreenCasts
		auras     *lookup.AuraWatch
		y, height int
	}{
		{"kick_below", &lookup.ScreenCasts{Version: 3, Offset: 24}, nil, 200, 40},
		{"kick_above", &lookup.ScreenCasts{Version: 3, Offset: -24}, nil, 176, 40},
		{"text_below", &lookup.ScreenCasts{Version: 1, Offset: 24}, nil, 200, 200},
		{"cd_below", nil, &lookup.AuraWatch{Offset: 16}, 200, 24},
		{"cd_above", nil, &lookup.AuraWatch{Offset: -8}, 192, 24},
		{"legacy_both", &lookup.ScreenCasts{Version: 3, Offset: 24}, &lookup.AuraWatch{Offset: -8}, 192, 48},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := engine.DefaultClientConfig()
			d := engine.FrameDisplay{Width: 640, Height: 480}
			panels := map[string]capturePanel{"fixture": {position: lookup.BridgePosition{X: 20, Y: 200}, screen: tc.screen, auras: tc.auras, seen: now}}
			reserveModuleStrips(&d, c, panels, now, false)
			if len(d.CaptureAreas) != 1 || d.CaptureAreas[0] != (engine.CaptureArea{X: 20, Y: tc.y, Width: 128, Height: tc.height}) {
				t.Fatal("native window hole leaves part of the read covered", d.CaptureAreas)
			}
			d.CaptureAreas = nil
			reserveModuleStrips(&d, c, panels, now.Add(2*time.Second), false)
			if len(d.CaptureAreas) != 0 {
				t.Fatal("expired capture area retained")
			}
		})
	}
}

func TestCaptureHoleOrderDoesNotInvalidateRendererCache(t *testing.T) {
	now := time.Now()
	panels := map[string]capturePanel{}
	for i, source := range []string{"kick", "cd", "gladius", ""} {
		panels[source] = capturePanel{position: lookup.BridgePosition{X: i * 128, Y: 200}, screen: &lookup.ScreenCasts{Version: 3, Offset: 24}, seen: now}
	}
	var first []engine.CaptureArea
	for range 64 {
		d := engine.FrameDisplay{Width: 640, Height: 480}
		reserveModuleStrips(&d, engine.DefaultClientConfig(), panels, now, false)
		if first == nil {
			first = d.CaptureAreas
		} else if !reflect.DeepEqual(first, d.CaptureAreas) {
			t.Fatal("unchanged regions invalidate renderer cache")
		}
	}
}

func allModulesConfig() engine.Config {
	c := engine.DefaultConfig()
	c.DefensiveTracking, c.FrameEstimates, c.LookupBridge = true, true, true
	c.KickEnabled, c.ArenaTracking, c.OverlaysEnabled, c.SimEnabled, c.KBBEnabled = true, true, true, true, true
	return c
}

func TestOnlyEnabledModulesHaveCaptureReaders(t *testing.T) {
	for mask := 0; mask < 64; mask++ {
		c := engine.DefaultClientConfig()
		c.ColorStripVersion = 2
		c.DefensiveTracking, c.LookupBridge = mask&1 != 0, mask&1 != 0
		c.KickEnabled, c.ArenaTracking, c.OverlaysEnabled, c.SimEnabled, c.KBBEnabled = mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0, mask&32 != 0
		want := map[string]bool{"cd": mask&1 != 0, "kick": mask&2 != 0, "gladius": mask&4 != 0 && !engine.Forever, "overlays": mask&8 != 0, "sim": mask&16 != 0, "kbb": mask&32 != 0}
		got := map[string]bool{}
		for _, r := range append(trackingStrips(c), transferStrips(c)...) {
			if got[r.Source] {
				t.Fatal("duplicate", mask, r)
			}
			got[r.Source] = true
		}
		for name, enabled := range want {
			if got[name] != enabled {
				t.Fatal("module demand", mask, name, got)
			}
		}
	}
	c := engine.DefaultClientConfig()
	if len(trackingStrips(c))+len(transferStrips(c)) != 0 {
		t.Fatal("missing settings started readers")
	}
	c.KickEnabled = true
	r := trackingStrips(c)
	if len(r) != 2 || r[0].Source != "" || r[1].Source != "kick" {
		t.Fatal("legacy shared input lost", r)
	}
	c.KickEnabled = false
	c.SimEnabled = true
	r = transferStrips(c)
	if len(r) != 2 || r[0].Source != "sim" || r[1].Source != "" {
		t.Fatal("legacy transfer fallback lost", r)
	}
}

func TestShareControlReservesSmallPacketWithoutNumericCaptureHole(t *testing.T) {
	c := engine.DefaultConfig()
	c.ColorStripVersion = 2
	c.AugEnabled = true
	now := time.Now()
	for _, pending := range []bool{false, true} {
		n := &lookup.NativeMeter{ControlOnly: true, ShareCapable: !pending}
		if pending {
			n.ControlWidth = 64
			n.ControlHeight = 8
		}
		d := engine.FrameDisplay{Width: 1920, Height: 1080}
		reserveModuleStrips(&d, c, map[string]capturePanel{"augmentation": {position: lookup.BridgePosition{X: 130, Y: 24}, native: n, seen: now}}, now, false)
		found := false
		for _, area := range d.CaptureAreas {
			if area.X == 130 && area.Y == 24 {
				if !pending || area.Width != 64 || area.Height != 8 {
					t.Fatal(area)
				}
				found = true
			}
			if area.Width == 256 && area.Height == 32 {
				t.Fatal("reserved disabled numeric capture strip", area)
			}
		}
		if found != pending {
			t.Fatal(d.CaptureAreas)
		}
	}
}
