package lookup

import (
	"context"
	"testing"
	"time"
)

func TestRosterSnapshotDistinguishesMissingFromUngrouped(t *testing.T) {
	q := NewQueue(context.Background(), nil)
	now := time.Now()
	if q.Roster() != nil {
		t.Fatal("unknown roster became ungrouped")
	}
	q.Handle(Event{ID: "joined", Kind: "roster", At: now, Members: []Character{alice}}, now)
	r := q.Roster()
	if !r[alice.GUID] {
		t.Fatal(r)
	}
	delete(r, alice.GUID)
	if !q.Roster()[alice.GUID] {
		t.Fatal("caller changed membership")
	}
	q.Handle(Event{ID: "invalid", Kind: "roster", At: now.Add(-time.Minute)}, now)
	if !q.Roster()[alice.GUID] {
		t.Fatal("stale packet cleared membership")
	}
	q.Handle(Event{ID: "left", Kind: "roster", At: now}, now)
	if q.Roster() == nil || len(q.Roster()) != 0 {
		t.Fatal("leaving party not reflected")
	}
	q.Clear()
	if q.Roster() != nil {
		t.Fatal("cleared stream still claims roster knowledge")
	}
}
