package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestScreenKickNeedsStableReadingsAndNeverRestartsFromCopies(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m := engine.NewModel()
	in := observationInput{}
	sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", At: now, Kind: "roster", Members: []lookup.Character{{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}}}, Observations: &lookup.Observations{Screen: &lookup.ScreenCasts{ID: "1:1", Captured: true, Casts: []lookup.LiveCast{{ID: "Screen:1:1:5", GUID: "Player-1-BOB", Spell: 47528, AgeMillis: 100}}}}}
	in.handle(m, sample, now)
	if m.Diagnostics.AcceptedCasts != 0 {
		t.Fatal("single image accepted")
	}
	sample.Observations.Screen.Casts[0].AgeMillis = 300
	in.handle(m, sample, now.Add(200*time.Millisecond))
	if m.Diagnostics.ScreenConfirmed != 1 || m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal(m.Diagnostics)
	}
	e := m.Snapshot(now).Cooldowns[0]
	if e.Spell != 47528 || e.EventAt != now.Add(-100*time.Millisecond) {
		t.Fatal(e)
	}
	sample.Observations.Screen.Casts[0].AgeMillis = 500
	in.handle(m, sample, now.Add(400*time.Millisecond))
	if m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("replayed")
	}
	m.Observe(engine.Cast{GUID: "Player-1-BOB", Spell: 47528, At: e.EventAt, Source: "combat_log"}, now.Add(66*time.Second))
	if len(m.Snapshot(now).Cooldowns) != 1 || m.Snapshot(now).Cooldowns[0].EventAt != e.EventAt {
		t.Fatal("late log restarted cast")
	}
}

func TestScreenKickRejectsAmbiguityFrozenClockAndFocusGap(t *testing.T) {
	for _, kind := range []string{"conflict", "frozen", "gap", "uncaptured", "rotation"} {
		t.Run(kind, func(t *testing.T) {
			in := screenInput{}
			m := engine.NewModel()
			now := time.Now()
			s := &lookup.ScreenCasts{ID: "1:1", Captured: true, Casts: []lookup.LiveCast{{ID: "Screen:1:1:5", GUID: "Player-1-BOB", Spell: 47528, AgeMillis: 100}}}
			in.casts(m, s, now)
			later := now.Add(200 * time.Millisecond)
			s.Casts[0].AgeMillis = 300
			switch kind {
			case "conflict":
				s.Casts[0].Spell = 2139
			case "frozen":
				s.Casts[0].AgeMillis = 100
			case "gap":
				later = now.Add(time.Second)
				s.Casts[0].AgeMillis = 1100
			case "uncaptured":
				s.Captured = false
			case "rotation":
				s.Casts[0].Spell = 133
			}
			if len(in.casts(m, s, later)) != 0 {
				t.Fatal("invalid cast accepted")
			}
		})
	}
}

func TestScreenColorCaptureDiagnosticClearsOnRecovery(t *testing.T) {
	m := engine.NewModel()
	in := screenInput{}
	now := time.Now()
	s := &lookup.ScreenCasts{Version: 2, Error: "color_calibration_mismatch"}
	in.casts(m, s, now)
	if m.Diagnostics.ScreenFormat != 2 || m.Diagnostics.ScreenCaptureError != s.Error || m.Diagnostics.ScreenCaptureFailures != 1 {
		t.Fatal(m.Diagnostics)
	}
	in.casts(m, nil, now)
	if m.Diagnostics.ScreenCaptureError != s.Error {
		t.Fatal("lost last capture failure during focus gap")
	}
	s.Error = ""
	s.Captured = true
	in.casts(m, s, now)
	if m.Diagnostics.ScreenCaptureError != "" || m.Diagnostics.ScreenSamples != 1 {
		t.Fatal(m.Diagnostics)
	}
}

func TestPublicAndScreenKickShareOneAttempt(t *testing.T) {
	for _, publicFirst := range []bool{false, true} {
		m := engine.NewModel()
		in := observationInput{}
		now := time.Now().Truncate(time.Second)
		guid := "Player-1-BOB"
		public := lookup.LiveCast{ID: "Cast-1", GUID: guid, Spell: 351338, AgeMillis: 30}
		screen := lookup.LiveCast{ID: "Screen:1:1:5", GUID: guid, Spell: 351338, AgeMillis: 100}
		sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", At: now, Kind: "roster", Members: []lookup.Character{{GUID: guid, Name: "Bob", Realm: "Draenor", Region: "us"}}}, Observations: &lookup.Observations{Screen: &lookup.ScreenCasts{ID: "1:1", Captured: true, Casts: []lookup.LiveCast{screen}}}}
		if publicFirst {
			sample.Observations.Casts = []lookup.LiveCast{public}
		}
		in.handle(m, sample, now)
		sample.Observations.Screen.Casts[0].AgeMillis += 200
		in.handle(m, sample, now.Add(200*time.Millisecond))
		public.AgeMillis += 400
		sample.Observations.Casts = []lookup.LiveCast{public}
		sample.Observations.Screen.Casts[0].AgeMillis += 200
		in.handle(m, sample, now.Add(400*time.Millisecond))
		if m.Snapshot(now).InterruptAttempts[guid] != 1 || m.Diagnostics.ScreenConfirmed != 1 {
			t.Fatal("duplicate live paths", publicFirst, m.Snapshot(now).InterruptAttempts, m.Diagnostics)
		}
	}
}
