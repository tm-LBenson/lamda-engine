package main

import (
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func savedConfigPath(explicit, wow string) string {
	if explicit != "" {
		return explicit
	}
	matches, _ := filepath.Glob(filepath.Join(wow, "WTF", "Account", "*", "SavedVariables", "lamdaUI.lua"))
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func trackingStrips(c engine.Config) []lookup.BridgeTarget {
	var targets []lookup.BridgeTarget
	cd := c.DefensiveTracking && c.LookupBridge
	arena := c.ArenaTracking && !engine.Forever
	// The legacy origin carries shared packets, but only while a consumer needs them.
	if cd || c.ColorStripVersion != 2 && (c.KickEnabled || arena) {
		target := lookup.BridgeTarget{Position: lookup.BridgePosition{X: c.LookupBridgeX, Y: c.LookupBridgeY}}
		if c.ColorStripVersion == 2 {
			target.Source = "cd"
		}
		targets = append(targets, target)
	}
	if c.KickEnabled {
		targets = append(targets, lookup.BridgeTarget{Source: "kick", Position: lookup.BridgePosition{X: c.KickBridgeX, Y: c.KickBridgeY}})
	}
	if arena {
		targets = append(targets, lookup.BridgeTarget{Source: "gladius", Position: lookup.BridgePosition{X: c.GladiusBridgeX, Y: c.GladiusBridgeY}})
	}
	if c.AugEnabled && (c.AugLiveHybrid || c.MeterOverlay) && !engine.Forever {
		targets = append(targets, lookup.BridgeTarget{Source: "augmentation", Wide: true, Position: lookup.BridgePosition{X: c.AugmentationBridgeX, Y: c.AugmentationBridgeY}, LocatorBase: lookup.BridgePosition{X: c.LookupBridgeX, Y: c.LookupBridgeY}})
		if c.MeterOverlay {
			for slot := 1; slot < 8; slot++ {
				v := targets[len(targets)-1]
				v.MeterSlot = slot
				targets = append(targets, v)
			}
		}
	}
	return capturePolicy(targets, c)
}

func transferStrips(c engine.Config) []lookup.BridgeTarget {
	var targets []lookup.BridgeTarget
	if c.OverlaysEnabled {
		targets = append(targets, lookup.BridgeTarget{Source: "overlays", OverlayWidth: c.OverlaysBridgeWidth, Position: lookup.BridgePosition{X: c.OverlaysBridgeX, Y: c.OverlaysBridgeY}, Wide: true})
	}
	if c.SimEnabled {
		targets = append(targets, lookup.BridgeTarget{Source: "sim", Position: lookup.BridgePosition{X: c.SimBridgeX, Y: c.SimBridgeY}, Wide: true})
	}
	if c.KBBEnabled {
		targets = append(targets, lookup.BridgeTarget{Source: "kbb", Position: lookup.BridgePosition{X: c.KBBBridgeX, Y: c.KBBBridgeY}, Wide: true})
	}
	if c.ColorStripVersion != 2 && (c.SimEnabled || c.KBBEnabled) {
		targets = append(targets, lookup.BridgeTarget{Position: lookup.BridgePosition{X: 0, Y: 48}, Wide: true})
	}
	return capturePolicy(targets, c)
}

func capturePolicy(targets []lookup.BridgeTarget, c engine.Config) []lookup.BridgeTarget {
	if !c.OptionalWork(c.ResourceFastIdle) {
		for i := range targets {
			targets[i].IdleInterval = time.Second
		}
	}
	return targets
}

func sameStrips(a, b []lookup.BridgeTarget) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Keep engine-drawn icons clear of every module's capture rectangle.
type capturePanel struct {
	native   *lookup.NativeMeter
	position lookup.BridgePosition
	screen   *lookup.ScreenCasts
	auras    *lookup.AuraWatch
	seen     time.Time
}

func reserveModuleStrips(d *engine.FrameDisplay, c engine.Config, panels map[string]capturePanel, now time.Time, holding bool) {
	targets := append(trackingStrips(c), transferStrips(c)...)
	for _, target := range targets {
		if target.Source == "augmentation" {
			if p, ok := lookup.MeterLocatorPosition(target.LocatorBase, d.Width, d.Height); ok {
				engine.ReserveCaptureArea(d, p.X+target.MeterSlot*6, p.Y, 6, 4)
			}
			key := target.Source
			if target.MeterSlot > 0 {
				key += strconv.Itoa(target.MeterSlot)
			}
			if panel, ok := panels[key]; ok && panel.native != nil && (panel.native.Border != nil || panel.native.HeaderOnly || panel.native.ControlOnly) {
				continue
			}
			if target.MeterSlot > 0 {
				continue
			}
		}
		if target.Source == "overlays" && !c.OverlaysEnabled && !d.OverlaysActive {
			continue
		}
		width, height := 128, 16
		if target.Wide {
			width, height = 256, 32
		}
		if target.Source == "overlays" && lookup.ValidOverlayWidth(target.OverlayWidth) {
			width, height = target.OverlayWidth, 18432/target.OverlayWidth
		}
		x, y := max(0, min(target.Position.X, d.Width-width)), max(0, min(target.Position.Y, d.Height-height))
		// Reserve the native aura band before the first valid reading, so
		// our own cooldown icons cannot prevent initial calibration.
		if target.Source == "overlays" && target.OverlayWidth == 0 {
			if y+72 > d.Height {
				y -= 40
			}
			height = 72
		}
		engine.ReserveCaptureArea(d, x, y, width, height)
	}
	// Stable order keeps the renderer's geometry cache from rebuilding a
	// native window region just because Go changed map iteration order.
	sources := make([]string, 0, len(panels))
	for source := range panels {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	for _, source := range sources {
		panel := panels[source]
		if !holding && now.Sub(panel.seen) > engine.FrameLifetime {
			continue
		}
		if n := panel.native; n != nil {
			if n.ControlOnly {
				if n.ControlWidth > 0 && n.ControlHeight > 0 {
					engine.ReserveCaptureArea(d, panel.position.X, panel.position.Y, n.ControlWidth, n.ControlHeight)
				}
				continue
			}
			if n.Frame != nil && n.Frame.Menu != nil {
				m := n.Frame.Menu
				engine.ReserveCaptureArea(d, m.X, m.Y, m.Width, m.Height)
			}
			if n.HeaderOnly {
				engine.ReserveCaptureArea(d, panel.position.X, panel.position.Y, 6, 8)
				continue
			}
			if n.Border != nil {
				for _, band := range n.Border.Bands() {
					engine.ReserveCaptureArea(d, panel.position.X+band.Min.X, panel.position.Y+band.Min.Y, band.Dx(), band.Dy())
				}
				continue
			}
			if d.Width < 256 || d.Height < 32 {
				continue
			}
			x := max(0, min(panel.position.X, d.Width-256))
			y := max(0, min(panel.position.Y, d.Height-32))
			top, bottom := min(0, n.Offset), max(32, n.Offset+n.Height())
			engine.ReserveCaptureArea(d, x, y+top, 256, bottom-top)
			continue
		}
		p, ok := panel.position.Clamp(d.Width, d.Height)
		if !ok {
			continue
		}
		// The reader takes one atomic copy of the packet and native bands.
		// Include the gap between them in the overlay's native window hole.
		top, bottom := 0, 16
		if panel.screen != nil {
			top = min(top, panel.screen.Offset)
			bottom = max(bottom, panel.screen.Offset+panel.screen.Height())
		}
		if panel.auras != nil {
			top = min(top, panel.auras.Offset)
			bottom = max(bottom, panel.auras.Offset+8)
		}
		engine.ReserveCaptureArea(d, p.X, p.Y+top, 128, bottom-top)
	}
}

// Queue capture is independent of tracking/transfer configuration. Keep its
// fixed packet clear even before the first invitation appears.
func reserveQueueStrip(d *engine.FrameDisplay) {
	if d.Width < lookup.BridgeWidth || d.Height < lookup.QueueStripBottomInset+lookup.BridgeHeight {
		return
	}
	engine.ReserveCaptureArea(d, 0, d.Height-lookup.QueueStripBottomInset-lookup.BridgeHeight, lookup.BridgeWidth, lookup.BridgeHeight)
}
