package main

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestOrdinaryRosterRetainsObservedReadyUntilMemberLeaves(t *testing.T) {
	start := time.Unix(1789500000, 0)
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", Kind: "roster", At: start, Members: []lookup.Character{member}}, Observations: &lookup.Observations{}}
	m := engine.NewModel()
	input := observationInput{}
	input.handle(m, sample, start)
	if m.FreshKey != nil || len(m.Snapshot(start).Cooldowns) != 0 {
		t.Fatal("roster invented a key or initial cooldown state")
	}
	sample.Event.At = start.Add(time.Second)
	sample.Observations.Casts = []lookup.LiveCast{{ID: "Cast-first", GUID: member.GUID, Spell: 57994, AgeMillis: 100}}
	input.handle(m, sample, sample.Event.At)
	if m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("first use not tracked")
	}
	now := start.Add(10 * time.Minute)
	sample.Event.At = now
	sample.Observations.Casts = nil
	input.handle(m, sample, now)
	if e := m.Snapshot(now).Cooldowns[0]; e.Status != "estimated_ready" {
		t.Fatal("ordinary party lost readiness", e)
	}
	sample.Event.Members = nil
	input.handle(m, sample, now)
	if e := m.Snapshot(now).Cooldowns[0]; e.Status != "unknown" {
		t.Fatal("departed member stayed ready", e)
	}
}

func TestOrdinaryRosterAndInspectionBecomeReadyWithoutCastInput(t *testing.T) {
	start := time.Unix(1789500000, 0)
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", Kind: "roster", At: start, Members: []lookup.Character{member}}, Observations: &lookup.Observations{}}
	m := engine.NewModel()
	input := observationInput{}
	input.handle(m, sample, start)
	m.ObserveInspectedBuild(engine.BuildSnapshot{GUID: member.GUID, Player: member.Name, Source: "inspect", SpecID: 262, GameBuild: "12.1.0", At: start, SpellRanks: map[int]int{57994: 1}}, start)
	sample.Event.At = start.Add(12 * time.Second)
	input.handle(m, sample, sample.Event.At)
	found := false
	for _, e := range m.Snapshot(sample.Event.At).Cooldowns {
		if e.Spell == 57994 && e.GUID == member.GUID {
			found = e.Status == "estimated_ready" && e.Kind == "party_ready"
		}
	}
	if !found || m.Diagnostics.AcceptedCasts != 0 || m.FreshKey != nil {
		t.Fatal("ordinary party warm-up failed")
	}
}
