package augmentation

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Window captures share log inputs and calibration, but never their selected
// native counters, clocks, geometry, errors or scroll position.
func (s *Store) ObserveNativeSlot(n *lookup.NativeMeter, slot int, at time.Time, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slot < 0 || slot > 7 {
		return
	}
	if n != nil && n.Frame != nil && n.Frame.WindowID > 0 {
		if n.Frame.WindowID != slot+1 {
			return
		}
		if s.nativeWindows == nil {
			s.nativeWindows = map[int]*hybridState{}
		}
		root := s.hybridState()
		state := s.nativeWindows[slot+1]
		if state == nil {
			state = &hybridState{classes: root.classes, buffs: root.buffs, calibrations: root.calibrations, nativeBuckets: map[string]map[int]int64{}}
			s.nativeWindows[slot+1] = state
		}
		if n.ControlOnly {
			state.hidden = n.ControlHidden
			state.controlAt = at
			return
		}
		state.hidden = false
		s.meterOverlayHidden = false
		state.events = root.events
		state.classes = root.classes
		state.buffs = root.buffs
		state.calibrations = root.calibrations
		if n.Captured && n.Error == "" && reason == "" && n.View != nil && !n.View.Primary {
			state.view = n
			state.viewAt = at
			return
		}
		if n.Captured && n.Error == "" && reason == "" {
			state.view = nil
		}
		s.hybrid = state
		s.observeNative(n, at, reason)
		s.hybrid = root
		return
	}
	if len(s.nativeWindows) > 0 {
		if slot == 0 && (reason == "native_meter_inactive_spec" || n != nil && n.ControlOnly && n.ControlHidden) {
			s.meterOverlayHidden = true
			return
		}
		if n != nil && n.ControlOnly {
			for _, h := range s.nativeWindows {
				h.controlAt = at
			}
			return
		} // preview control cannot erase meter data
		if state := s.nativeWindows[slot+1]; state != nil {
			state.error = reason
		}
		return
	}
	s.observeNative(n, at, reason)
}

type windowMeter struct {
	frame      lookup.NativeMeterFrame
	meter      DamageMeter
	classes    map[string]int
	serial     int
	epoch      string
	navigation bool
	native     *lookup.NativeMeter
}

func (s *Store) windowMeters(now time.Time) []windowMeter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.meterOverlayHidden {
		return nil
	}
	root := s.hybridState()
	out := []windowMeter{}
	for id := 1; id <= 8; id++ {
		h := s.nativeWindows[id]
		if h == nil || h.hidden {
			continue
		}
		if h.view != nil && h.view.Frame != nil && h.view.Player == s.own && (now.Sub(h.viewAt) <= 1500*time.Millisecond || !h.view.Combat && now.Sub(h.controlAt) < 7*time.Second) {
			var parent DamageMeter
			if h.frame != nil && h.frame.SessionID == h.view.SessionID && h.frame.Metric == h.view.Metric && h.frame.Frame.Scope == h.view.Frame.Scope && math.Abs(float64(h.frame.DurationMillis-h.view.DurationMillis)) < 1000 {
				s.hybrid = h
				if m := s.hybridView(h.seen); m != nil {
					parent = *m
				}
				s.hybrid = root
			}
			n := h.view
			out = append(out, windowMeter{frame: *n.Frame, meter: parent, serial: n.Serial, epoch: n.ID, navigation: n.NavigationCapable, native: n})
			continue
		}
		held := h.frame != nil && !h.frame.Combat && now.Sub(h.controlAt) < 7*time.Second
		if h.frame == nil || h.frame.Frame == nil || h.frame.Player != s.own || now.Sub(h.seen) > 1500*time.Millisecond && !held {
			continue
		}
		h.events = root.events
		h.classes = root.classes
		h.buffs = root.buffs
		h.calibrations = root.calibrations
		s.hybrid = h
		viewAt := now
		if held {
			viewAt = h.seen
		}
		m := s.hybridView(viewAt)
		s.hybrid = root
		if m == nil {
			continue
		}
		rows := make([]DamageRow, 0, len(m.Rows))
		for _, r := range m.Rows {
			if r.Raw != 0 || r.Adjusted != 0 {
				rows = append(rows, r)
			}
		}
		m.Rows = rows
		classes := map[string]int{}
		for guid, class := range root.classes {
			classes[guid] = class
		}
		out = append(out, windowMeter{frame: *h.frame.Frame, meter: *m, classes: classes, serial: h.frame.Serial, epoch: h.frame.ID, navigation: h.frame.NavigationCapable, native: h.frame})
	}
	return out
}

func (s *Store) mergeWindows(d engine.FrameDisplay, c engine.Config, w lookup.WindowContext, now time.Time) engine.FrameDisplay {
	if !c.AugEnabled || !c.AugShowMeter || w.Window == 0 || (!w.Foreground && !d.KeepUIWhenUnfocused && !s.desktopMeter()) {
		return d
	}
	window := strconv.FormatUint(uint64(w.Window), 10)
	if d.Enabled && (d.Window != window || d.Width != w.Width || d.Height != w.Height) {
		return d
	}
	meters := s.windowMeters(now)
	d.Meters = []*engine.MeterWindow{}
	for _, v := range meters {
		f, m := v.frame, v.meter
		if !c.AugLiveHybrid && m.Provisional {
			m.Valid = false
		}
		if f.Minimized {
			continue
		}
		appearance := meterAppearance{RowHeight: f.RowHeight, FontSize: f.FontSize, Opacity: float64(f.Opacity) / 1000, Values: f.Values, ClassColors: f.ClassColors, HeaderHeight: f.HeaderHeight, FooterHeight: max(10, f.RowHeight), PixelScaled: true}
		status := "Recorded"
		if m.Live {
			status = "Live"
		}
		if m.Provisional {
			status = "Attribution pending"
			if m.Live {
				status = "Live · provisional"
			}
		}
		if !m.Valid {
			status = "Waiting for live feed"
		}
		nativeNav := strings.HasPrefix(m.ID, "native/") || !s.hasMeterDetails(m)
		if nav := s.meterNavigation(m, f.WindowID); nav.Schema == 1 && nav.Actor == "" {
			appearance.Offset = nav.Offset
		}
		body := presentMeter(m, appearance, v.classes, f.Height, f.Label, status)
		if !nativeNav {
			body = s.presentMeterView(m, appearance, v.classes, f.Height, f.Label, status, f.WindowID)
		}
		if v.navigation {
			body.Navigation = &engine.MeterNavigation{WindowID: f.WindowID, Serial: v.serial, Epoch: v.epoch, Native: nativeNav}
		}
		if v.native != nil && v.native.View != nil {
			if !v.native.View.Primary {
				body = s.presentNativeView(v.native, m, appearance)
			}
			s.decorateNativeBody(body, v.native)
			if v.navigation {
				body.Navigation = &engine.MeterNavigation{WindowID: f.WindowID, Serial: v.serial, Epoch: v.epoch, Native: nativeNav || !v.native.View.Primary}
			}
		}
		body.WindowID = f.WindowID
		view := &engine.MeterWindow{ID: f.WindowID, HeaderHeight: f.HeaderHeight, Body: body, NativeHost: true, Metric: body.Metric, Scope: f.Scope, X: float64(f.X), Y: float64(f.Y), Width: float64(f.Width), Height: float64(f.Height), Locked: f.Locked}
		if v.native != nil && v.native.View != nil {
			for _, choice := range v.native.View.Choices {
				view.Choices = append(view.Choices, engine.MeterChoice{Scope: choice.Scope, Label: choice.Label})
			}
		}
		if f.Menu != nil {
			view.Menu = &engine.MeterNativeMenu{X: f.Menu.X, Y: f.Menu.Y, Width: f.Menu.Width, Height: f.Menu.Height}
		}
		d.Meters = append(d.Meters, view)
	}
	if len(d.Meters) > 0 {
		d.Enabled = true
		d.OverlaysActive = true
		d.Window = window
		d.WindowPID = w.PID
		d.Width = w.Width
		d.Height = w.Height
		d.At = float64(now.UnixMilli()) / 1000
		d.Expires = d.At + 1.2
	}
	return d
}
