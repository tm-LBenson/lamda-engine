package main

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestChallengeBoundaryThroughObservationInput(t *testing.T) {
	now := time.Unix(1789500000, 0)
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", Kind: "key_start", At: now, Deadline: now.Add(10 * time.Second), Members: []lookup.Character{member}},
		Observations: &lookup.Observations{Builds: []lookup.BuildChunk{{ID: "1:1", GUID: member.GUID, Game: "12.1.0", At: now, Spec: 264, Part: 1, Total: 1, Data: "1,101,1,108271;2,102,1,57994"}}}}
	m := engine.NewModel()
	input := observationInput{}
	input.handle(m, sample, now)
	if m.FreshKey != nil || len(m.Snapshot(now).Cooldowns) != 0 {
		t.Fatal("stone insertion marked ready")
	}
	sample.Event.Kind = "challenge_start"
	sample.Event.Deadline = now
	input.handle(m, sample, now)
	if len(m.Snapshot(now).Cooldowns) != 2 {
		t.Fatal("complete inspected abilities not seeded")
	}
	sample.Event.At = now.Add(2 * time.Second)
	sample.Event.ID = "1:2"
	input.handle(m, sample, sample.Event.At)
	if !m.FreshKey.At.Equal(now) {
		t.Fatal("heartbeat slid initial boundary")
	}
	sample.Observations.Casts = []lookup.LiveCast{{ID: "Cast-first", GUID: member.GUID, Spell: 57994, AgeMillis: 200}}
	input.handle(m, sample, sample.Event.At)
	if m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("first use not tracked")
	}
	sample.Event.Kind = "challenge_end"
	sample.Event.Deadline = time.Time{}
	input.handle(m, sample, sample.Event.At)
	if m.FreshKey != nil || len(m.Snapshot(sample.Event.At).Cooldowns) != 1 {
		t.Fatal("end failed to remove only initial estimates")
	}
	sample.Event.Kind = "challenge_start"
	sample.Event.Deadline = now
	input.handle(m, sample, sample.Event.At)
	if m.FreshKey != nil {
		t.Fatal("ended challenge revived by buffered strip")
	}
}
