package lookup

import (
	"testing"
	"time"
)

func TestLocalInspectionArrivingDuringGraceAvoidsWebRequest(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	bob := Character{"Player-1-BOB", "Bob", "Area 52", "us"}
	known := map[string]bool{alice.GUID: true}
	q.LocalBuild = func(guid string, _ time.Time) bool { return known[guid] }
	q.RosterGrace = 15 * time.Second
	q.Handle(Event{ID: "join", Kind: "roster", At: now, Members: []Character{alice, bob}}, now)
	q.Advance(now)
	if len(q.pending) != 1 || len(f.Calls) != 0 || len(q.Roster()) != 2 {
		t.Fatal("lost roster or queried own character", q.Status())
	}
	known[bob.GUID] = true
	q.Advance(now.Add(16 * time.Second))
	if len(q.pending) != 0 || len(f.Calls) != 0 {
		t.Fatal("queried newly inspected player", q.Status())
	}
}

func TestInspectionCancelsInflightFallback(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	known := false
	q.LocalBuild = func(string, time.Time) bool { return known }
	q.Handle(Event{ID: "join", Kind: "roster", At: now, Members: []Character{alice}}, now)
	q.Advance(now)
	call := takeCall(t, f)
	known = true
	q.Advance(now)
	finish(t, q, call, nil)
	if len(q.Advance(time.Now())) != 0 || len(q.pending) != 0 {
		t.Fatal("superseded web result accepted")
	}
}
