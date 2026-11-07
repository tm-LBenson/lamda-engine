package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestCastRosterFreshnessIdentityAndLeaving(t *testing.T) {
	now := time.Date(2026, 9, 15, 23, 33, 10, 0, time.UTC)
	member := lookup.Character{GUID: "Player-1-00000001", Name: "Shaman", Realm: "BurningBlade", Region: "us"}
	valid := lookup.Event{ID: "1:1", At: now, Kind: "roster", Members: []lookup.Character{member}}
	if !rosterForCasts(&valid, now)[member.GUID] {
		t.Fatal("fresh explicit roster rejected")
	}
	if !rosterForCasts(&valid, now.Add(-3*time.Second))[member.GUID] {
		t.Fatal("measured WoW server clock difference rejected")
	}
	for _, age := range []time.Duration{31 * time.Second, -6 * time.Second} {
		if rosterForCasts(&valid, now.Add(age)) != nil {
			t.Fatal("expired/future roster accepted", age)
		}
	}
	if rosterForCasts(nil, now) != nil {
		t.Fatal("missing bridge became a known party")
	}
	for _, mutate := range []func(*lookup.Event){
		func(e *lookup.Event) { e.ID = "" },
		func(e *lookup.Event) { e.Kind = "unknown" },
		func(e *lookup.Event) { e.Members = append(e.Members, member) },
		func(e *lookup.Event) { e.Members = []lookup.Character{{GUID: member.GUID}} },
		func(e *lookup.Event) { e.Kind = "arena_ready" },
		func(e *lookup.Event) { e.Kind = "key_start"; e.Deadline = now.Add(16 * time.Second) },
	} {
		event := valid
		mutate(&event)
		if rosterForCasts(&event, now) != nil {
			t.Fatal("invalid snapshot accepted", event)
		}
	}
	left := valid
	left.Members = nil
	if r := rosterForCasts(&left, now); r == nil || len(r) != 0 {
		t.Fatal("leaving group did not clear membership", r)
	}
	// This pipeline needs no public lookup queue or HTTP response.
	line := fmt.Sprintf(`%s  SPELL_CAST_SUCCESS,%s,"Shaman",0x10548,0,0000000000000000,nil,0,0,108271,"Astral Shift",1`, now.Format("1/2/2006 15:04:05.000-07:00"), member.GUID)
	for name, event := range map[string]*lookup.Event{"joined": &valid, "left": &left, "unavailable": nil} {
		m := engine.NewModel()
		accepted := m.IngestWithRoster(line, now, false, rosterForCasts(event, now))
		if accepted != (name == "joined") {
			t.Fatal(name, accepted)
		}
	}
}
