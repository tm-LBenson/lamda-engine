package main

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestFastInterruptInputsSurviveMeasuredServerClockSkew(t *testing.T) {
	for _, path := range []string{"public", "screen", "meter"} {
		t.Run(path, func(t *testing.T) {
			start := time.Now().Add(time.Second).Truncate(time.Second)
			guid := "Player-1-BOB"
			m := engine.NewModel()
			var input observationInput
			sample := lookup.BridgeSample{Source: "kick", Event: lookup.Event{ID: "1:1", Kind: "roster", Members: []lookup.Character{{GUID: guid, Name: "Bob", Realm: "Draenor", Region: "us"}}},
				Kick: &framewire.KickSnapshot{Player: "Player-1-SELF", Members: []framewire.KickMember{{GUID: guid, Class: 8, Spec: 64}}}, Observations: &lookup.Observations{}}
			screen := &lookup.ScreenCasts{ID: "1:1", Version: 3, Captured: true}
			for i := 0; i < 4; i++ {
				now := start.Add(time.Duration(i) * 200 * time.Millisecond)
				// This is the live measured clock range. Cast times still come
				// from monotonic ages and local receipt, never this server clock.
				sample.Event.At = now.Add(2900 * time.Millisecond).Truncate(time.Second)
				sample.Kick.At = sample.Event.At
				cast := lookup.LiveCast{ID: "Cast-clock", GUID: guid, Spell: 2139, AgeMillis: i * 200}
				switch path {
				case "public":
					sample.Observations.Casts = []lookup.LiveCast{cast}
				case "screen":
					cast.ID = "Screen:clock:1:5"
					screen.Casts = []lookup.LiveCast{cast}
					sample.Observations.Screen = screen
				case "meter":
					amount := 7
					if i > 0 {
						amount++
					}
					screen.Meter = []lookup.MeterOwner{{GUID: guid, Serial: i + 1, Amount: amount}}
					sample.Observations.Screen = screen
				}
				input.handle(m, sample, now)
				if path != "public" && i == 0 && len(m.Snapshot(now).Cooldowns) != 0 {
					t.Fatal("history or single native read became a cast")
				}
			}
			state := m.Snapshot(start.Add(time.Second))
			if len(state.Cooldowns) != 1 || state.InterruptAttempts[guid] != 1 {
				t.Fatalf("fast %s input lost or duplicated: %+v", path, state)
			}
			e := state.Cooldowns[0]
			want := start
			if path == "meter" {
				want = start.Add(200 * time.Millisecond)
			}
			if !e.EventAt.Equal(want) {
				t.Fatalf("server skew moved cast clock: %v != %v", e.EventAt, want)
			}
			if path == "meter" && (m.Diagnostics.MeterConfirmed != 1 || state.InterruptCounts[guid] != 1) {
				t.Fatal("meter confirmation lost", m.Diagnostics)
			}
			if path == "screen" && m.Diagnostics.ScreenConfirmed != 1 {
				t.Fatal("screen confirmation lost", m.Diagnostics)
			}
		})
	}
}

func TestClockToleranceStillRejectsFrozenNativeAndInvalidFutureMetadata(t *testing.T) {
	start := time.Now().Add(time.Second).Truncate(time.Second)
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	for _, kind := range []string{"frozen", "future", "expired", "error"} {
		t.Run(kind, func(t *testing.T) {
			m := engine.NewModel()
			var input observationInput
			sample := lookup.BridgeSample{Source: "kick", Event: lookup.Event{ID: "1:1", Kind: "roster", At: start.Add(2 * time.Second), Members: []lookup.Character{member}}, Observations: &lookup.Observations{Screen: &lookup.ScreenCasts{ID: "1:1", Version: 3, Captured: true, Casts: []lookup.LiveCast{{ID: "Screen:frozen:1:5", GUID: member.GUID, Spell: 2139, AgeMillis: 0}}}}}
			switch kind {
			case "future":
				sample.Event.At = start.Add(addonClockTolerance + time.Second)
			case "expired":
				sample.Event.At = start.Add(-4 * time.Second)
			case "error":
				sample.Error = "bridge_checksum"
			}
			for i := 0; i < 5; i++ {
				input.handle(m, sample, start.Add(time.Duration(i)*200*time.Millisecond))
			}
			if len(m.Snapshot(start.Add(time.Second)).Cooldowns) != 0 || m.Diagnostics.ScreenConfirmed != 0 {
				t.Fatal("non-advancing or invalid data created a kick", kind)
			}
		})
	}
}
