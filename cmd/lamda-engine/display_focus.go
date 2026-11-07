package main

import (
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Background display retains layout only. It never turns cached screen pixels
// into new observations, roster updates or lookup events.
type displayFocus struct {
	allow, holding bool
	lastVisible    time.Time
	windowPID      uint32
}

func (f *displayFocus) observe(sample lookup.BridgeSample) {
	f.allow = sample.Event.KeepUIWhenUnfocused
	f.windowPID = sample.WindowPID
	f.holding = false
}

func (f *displayFocus) pause(reason string, now time.Time) bool {
	f.holding = reason == "wow_not_foreground" && f.allow &&
		(f.holding || (!f.lastVisible.IsZero() && !now.Before(f.lastVisible) && now.Sub(f.lastVisible) <= engine.FrameLifetime))
	if !f.holding {
		f.lastVisible = time.Time{}
	}
	return f.holding
}

func (f *displayFocus) view(s engine.InferenceSnapshot, c engine.Config,
	g *framewire.Snapshot, gs time.Time, gw uintptr,
	k *framewire.KickSnapshot, ks time.Time, kw uintptr, now time.Time) engine.FrameDisplay {
	// Only copies used for drawing receive a new display lease. Original capture
	// timestamps still gate cast matching and every live-input consumer.
	originalSeen := gs
	if g == nil || originalSeen.IsZero() {
		originalSeen = ks
	}
	if f.holding {
		if g != nil {
			copy := *g
			copy.At = now
			g = &copy
			gs = now
		}
		if k != nil {
			copy := *k
			copy.At = now
			k = &copy
			ks = now
		}
	}
	d := engine.MergeKickDisplay(engine.FrameView(s, c, g, gs, now, gw), engine.KickView(s, c, k, ks, now, kw))
	d.KeepUIWhenUnfocused, d.Background, d.WindowPID = f.allow, f.holding, f.windowPID
	if f.holding && d.Enabled {
		d.Status = "background_layout"
		d.GeometryAge = now.Sub(originalSeen).Seconds()
	} else if d.Enabled {
		f.lastVisible = now
	}
	return d
}
