package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestKickConfirmationSurvivesCDSamplesAndCaptureFailure(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	model := engine.NewModel()
	input := observationInput{}
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	event := lookup.Event{ID: "1:1", At: now, Kind: "roster", Members: []lookup.Character{member}}
	screen := &lookup.ScreenCasts{ID: "kick:1", Captured: true, Casts: []lookup.LiveCast{{ID: "Screen:kick:1", GUID: member.GUID, Spell: 47528, AgeMillis: 100}}}
	kick := lookup.BridgeSample{Source: "kick", Event: event, Observations: &lookup.Observations{Screen: screen}}
	input.handle(model, kick, now)
	input.handle(model, lookup.BridgeSample{Source: "cd", Event: event, Observations: &lookup.Observations{}}, now.Add(100*time.Millisecond))
	input.handle(model, lookup.BridgeSample{Source: "cd", Error: "bridge_not_visible"}, now.Add(150*time.Millisecond))
	screen.Casts[0].AgeMillis = 300
	input.handle(model, kick, now.Add(200*time.Millisecond))
	if model.Diagnostics.ScreenConfirmed != 1 || model.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("CD interrupted Kick confirmation", model.Diagnostics)
	}
	input.handle(model, kick, now.Add(300*time.Millisecond))
	if model.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("separate delivery replayed a cast")
	}
}

func TestBuffOnsetSurvivesInterleavedKickSamples(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	model := engine.NewModel()
	input := observationInput{}
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Realm", Region: "us"}
	aura := &lookup.AuraWatch{ID: "cd:1", Captured: true, Members: []lookup.AuraWatchMember{{Slot: 1, Member: 1, Class: 6, Mask: 1, GUID: member.GUID}}}
	event := lookup.Event{ID: "1:1", At: now, Kind: "roster", Members: []lookup.Character{member}}
	for i := 0; i < 4; i++ {
		at := now.Add(time.Duration(i) * 200 * time.Millisecond)
		aura.Active[8] = i >= 2
		input.handle(model, lookup.BridgeSample{Source: "cd", Window: 123, Event: event, Observations: &lookup.Observations{Auras: aura}}, at)
		input.handle(model, lookup.BridgeSample{Source: "kick", Event: event, Observations: &lookup.Observations{}}, at.Add(50*time.Millisecond))
		input.handle(model, lookup.BridgeSample{Source: "kick", Error: "bridge_checksum"}, at.Add(100*time.Millisecond))
	}
	if len(model.Snapshot(now.Add(time.Second)).Cooldowns) != 1 {
		t.Fatal("Kick interrupted buff onset", model.Diagnostics)
	}
}
