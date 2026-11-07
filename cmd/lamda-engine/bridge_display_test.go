package main

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func displaySamples(now time.Time) (lookup.BridgeSample, lookup.BridgeSample) {
	frames := &framewire.Snapshot{ID: "cd:1", At: now, Width: 1600, Height: 1000, Player: "Player-1-SELF",
		Self: true, Party: true, Unknown: true, Frames: []framewire.Rect{{GUID: "Player-1-FRIEND", X: 100, Y: 500, Width: 200, Height: 40}}}
	kick := &framewire.KickSnapshot{ID: "kick:1", At: now, Width: 1600, Height: 1000, Player: "Player-1-SELF", Mode: 2,
		BarWidth: 240, BarHeight: 28, Size: 26, Opacity: 95, Members: []framewire.KickMember{{GUID: "Player-1-FRIEND", Name: "Friend", Class: 7, Spec: 264}}}
	return lookup.BridgeSample{Source: "cd", Window: 123, Frames: frames}, lookup.BridgeSample{Source: "kick", Window: 123, Kick: kick}
}

func TestBridgeDisplayRetainsIndependentLayoutsAcrossTransientFailures(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	cd, kick := displaySamples(now)
	var displays bridgeDisplayCache
	if displays.observe(cd, false, now) != displaySampleAccepted || displays.observe(kick, false, now) != displaySampleAccepted {
		t.Fatal("valid display samples were rejected")
	}
	frames, kicks := displays.geometry, displays.kick
	for _, source := range []string{"cd", "kick"} {
		if displays.observe(lookup.BridgeSample{Source: source, Error: "bridge_checksum"}, false, now.Add(200*time.Millisecond)) != displaySampleTransientFailure {
			t.Fatal("source failure was not transient", source)
		}
		if displays.geometry != frames || displays.kick != kicks || displays.geometrySeen != now || displays.kickSeen != now {
			t.Fatal("transient source failure discarded or refreshed a valid layout", source)
		}
	}
	// A valid shared/event packet can temporarily omit one module section. It
	// must not erase or refresh that module's last independently valid layout.
	if displays.observe(lookup.BridgeSample{Source: "kick", Window: 123}, false, now.Add(400*time.Millisecond)) != displaySampleAccepted ||
		displays.kick != kicks || displays.kickSeen != now {
		t.Fatal("missing optional Kick section discarded its display lease")
	}

	cfg := engine.DefaultConfig()
	cfg.FrameEstimates, cfg.KickEnabled = true, true
	if view := engine.FrameView(engine.InferenceSnapshot{}, cfg, displays.geometry, displays.geometrySeen, now.Add(time.Second), displays.geometryWindow); !view.Enabled {
		t.Fatal("CD layout did not survive a short reader gap", view)
	}
	if view := engine.KickView(engine.InferenceSnapshot{}, cfg, displays.kick, displays.kickSeen, now.Add(time.Second), displays.kickWindow); !view.Enabled || view.KickPanel == nil {
		t.Fatal("Kick layout did not survive a short reader gap", view)
	}
	later := now.Add(engine.FrameLifetime + time.Millisecond)
	if engine.FrameView(engine.InferenceSnapshot{}, cfg, displays.geometry, displays.geometrySeen, later, displays.geometryWindow).Enabled ||
		engine.KickView(engine.InferenceSnapshot{}, cfg, displays.kick, displays.kickSeen, later, displays.kickWindow).Enabled {
		t.Fatal("retained layouts outlived the safety lease")
	}
}

func TestBridgeDisplayClearsGlobalAndWrongClientFailures(t *testing.T) {
	now := time.Now()
	cd, kick := displaySamples(now)
	for _, test := range []struct {
		name   string
		sample lookup.BridgeSample
		want   bridgeDisplayStatus
	}{
		{"game unavailable", lookup.BridgeSample{Source: "cd", Error: "wow_minimized"}, displaySampleGlobalFailure},
		{"shared failure", lookup.BridgeSample{Error: "bridge_closed"}, displaySampleGlobalFailure},
		{"wrong client", lookup.BridgeSample{Source: "cd", Forever: true, Frames: cd.Frames}, displaySampleWrongClient},
	} {
		t.Run(test.name, func(t *testing.T) {
			var displays bridgeDisplayCache
			displays.observe(cd, false, now)
			displays.observe(kick, false, now)
			if got := displays.observe(test.sample, false, now.Add(time.Millisecond)); got != test.want {
				t.Fatal(got)
			}
			if displays.geometry != nil || displays.kick != nil {
				t.Fatal("unsafe display layout retained")
			}
		})
	}
}
