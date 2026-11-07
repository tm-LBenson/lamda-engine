package augmentation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Merge supplies meter bars to a separate interactive window. Other overlays
// remain input-free. It only draws over the verified foreground game HWND.
func (s *Store) Merge(d engine.FrameDisplay, c engine.Config, w lookup.WindowContext, now time.Time) engine.FrameDisplay {
	if c.MeterOverlay {
		return s.mergeWindows(d, c, w, now)
	}
	desktop := s.desktopMeter()
	if !c.AugEnabled || !c.AugShowMeter || (!w.Foreground && !desktop && !d.KeepUIWhenUnfocused) || w.Window == 0 || w.Width < 400 || w.Height < 200 {
		return d
	}
	if c.AugLiveHybrid && !s.nativeAugEligible() {
		return d
	}
	window := strconv.FormatUint(uint64(w.Window), 10)
	if d.Enabled && (d.Window != window || d.Width != w.Width || d.Height != w.Height) {
		return d
	}
	scope := "boss"
	if c.AugMeterScope == 2 {
		scope = "key"
	}
	p := s.placement()
	if p != nil {
		scope = p.Scope
	}
	var native *lookup.NativeMeterFrame
	if c.AugLiveHybrid {
		native = s.nativeFrame(now)
		if native != nil && !native.Standalone && !desktop {
			scope = native.Scope
			p = nil
		}
	}
	meter, choices := s.selectedMeter(scope, c.AugLiveHybrid, now)
	if meter == nil {
		meter = &DamageMeter{Rows: []DamageRow{}}
	}
	if meter.ID != "" {
		s.debugDisplay(*meter, now)
	}
	state := s.Snapshot()
	d.Enabled = true
	d.OverlaysActive = true
	d.Window = window
	d.WindowPID = w.PID
	d.Width = w.Width
	d.Height = w.Height
	d.At = float64(now.UnixMilli()) / 1000
	d.Expires = d.At + 1.2
	width := 380.0
	x := float64(c.AugX) * float64(w.Width) / float64(max(1, c.AugUIWidth))
	y := float64(c.AugY) * float64(w.Height) / float64(max(1, c.AugUIHeight))
	rows := min(40, len(meter.Rows))
	height := float64(max(150, 76+rows*25))
	locked := false
	if p != nil && !p.ScopeOnly {
		x, y = p.X*float64(w.Width), p.Y*float64(w.Height)
		width, height, locked = float64(p.Width), float64(p.Height), p.Locked
		rows = min(rows, max(1, (p.Height-76)/25))
	}
	if native != nil && !native.Standalone && !desktop {
		x, y, width, height = float64(native.X), float64(native.Y), float64(native.Width), float64(native.Height)
		locked = native.Locked
		rows = min(rows, max(1, (native.Height-76)/25))
	}
	x = max(0, min(x, float64(w.Width)-width))
	y = max(0, min(y, float64(w.Height)-height))
	d.Meter = &engine.MeterWindow{Metric: "Damage", X: x, Y: y, Width: width, Height: height, Locked: locked, Scope: scope, Choices: choices, NativeHost: native != nil && !native.Standalone && !desktop}
	if meter.Metric == 2 {
		d.Meter.Metric = "Healing"
	}
	if native != nil && native.Menu != nil {
		m := native.Menu
		d.Meter.Menu = &engine.MeterNativeMenu{X: m.X, Y: m.Y, Width: m.Width, Height: m.Height}
	}
	label := func(id, text string, top, h, font float64, color string, fill *float64) {
		kind := "text"
		if fill != nil {
			kind = "bar"
		}
		px, py := x, top
		value := text
		if fill != nil {
			value = ""
		}
		d.Overlays = append(d.Overlays, engine.OverlayDisplay{ID: "lamdaaug/" + id, Rule: "lamdaaug", Kind: kind, Text: value, PixelX: &px, PixelY: &py,
			Size: width, Font: font, Opacity: 100, Color: color, Persistent: true, Ends: d.Expires, Duration: 1.2, Fraction: fill,
			Visual: &engine.OverlayVisual{Width: width, Height: h, TextAlign: "left", TrackColor: "#101B21", TextEffect: "outline"}})
		if fill != nil {
			d.Overlays = append(d.Overlays, engine.OverlayDisplay{ID: "lamdaaug/" + id + "/text", Rule: "lamdaaug", Kind: "text", Text: text, PixelX: &px, PixelY: &py,
				Size: width, Font: font, Opacity: 100, Color: "#F2F1FF", Persistent: true, Ends: d.Expires, Duration: 1.2, Transparent: true,
				Visual: &engine.OverlayVisual{Width: width, Height: h, TextAlign: "left", TextEffect: "outline", Layer: 1}})
		}
	}
	if native == nil || native.Standalone || desktop {
		title := "LamdaMeters · Damage"
		if meter.HasAug {
			title = "LamdaMeters · Aug-adjusted"
		}
		if meter.Metric == 2 {
			title = "LamdaMeters · Healing"
		}
		label("title", title, y, 24, 15, "#D9E5FF", nil)
	}
	delay := max(0, now.Sub(state.LastEventAt).Seconds())
	status := fmt.Sprintf("%s · %.0fs log age", meter.Kind, delay)
	if !meter.Live {
		status = fmt.Sprintf("%s · finished · %.0fs", meter.Kind, meter.Seconds)
	}
	if !meter.Valid {
		status = "Pending"
	} else if meter.Partial {
		status = fmt.Sprintf("Partial · finished · %d hooks excluded", meter.RejectedHooks)
		if meter.Live {
			status = fmt.Sprintf("Partial · %.0fs log age · %d hooks excluded", delay, meter.RejectedHooks)
		}
	}
	if meter.ID == "" {
		status = "No combat data"
	}
	if meter.InputSource != "" {
		status = "Recorded"
		if meter.InputSource == "native" && meter.Live {
			status = "Live"
		}
		if meter.Provisional {
			status = "Attribution pending"
			if meter.Live {
				status = "Live · provisional"
			}
		}
		if meter.CalibrationMissing && meter.Live {
			status = "Live · calibrating"
		}
		if meter.Partial {
			status = "Partial · provisional"
		}
		if !meter.Valid {
			status = "Waiting for live feed"
		}
	}
	label("status", status, y+23, 23, 11, "#D4C6A0", nil)
	selected := "Current"
	for _, choice := range choices {
		if choice.Scope == scope {
			selected = choice.Label
			break
		}
	}
	d.Meter.Body = s.presentMeterView(*meter, defaultMeterAppearance(), state.NativeClasses, int(height), selected, status)
	maximum := int64(1)
	for _, row := range meter.Rows {
		maximum = max(maximum, row.Adjusted)
	}
	for i, row := range meter.Rows[:rows] {
		name := strings.Split(row.Name, "-")[0]
		color := "#4786A8"
		if row.GUID == meter.Owner {
			color = "#7560C9"
		}
		rateLabel := "DPS"
		if meter.Metric == 2 {
			rateLabel = "HPS"
		}
		text := fmt.Sprintf("%d. %s    %s  |  %s %s", i+1, name, compact(row.Adjusted), compact(int64(row.DPS)), rateLabel)
		fraction := float64(max(int64(0), row.Adjusted)) / float64(maximum)
		if !meter.Valid {
			text = fmt.Sprintf("%d. %s    pending", i+1, name)
			fraction = 0
		}
		label(fmt.Sprintf("row%d", i), text, y+47+float64(i*25), 23, 12, color, &fraction)
	}
	return d
}
func compact(n int64) string {
	if n >= 1e9 {
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
	if n >= 1e6 {
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return strconv.FormatInt(n, 10)
}
