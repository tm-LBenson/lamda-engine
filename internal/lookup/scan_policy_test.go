package lookup

import (
	"testing"
	"time"
)

func TestWebFallbackFollowsSharedScanPolicy(t *testing.T) {
	for _, mode := range []int{1, 2, 3, 4} {
		q, _ := testQueue(t)
		q.SetScanPolicy(mode, 3)
		now := time.Now()
		q.Handle(Event{ID: "join", Kind: "roster", At: now, Members: []Character{alice}}, now)
		want := 1
		if mode == 3 {
			want = 0
		}
		if len(q.pending) != want {
			t.Fatal("join", mode, len(q.pending))
		}
		q.pending = nil
		q.Handle(Event{ID: "insert", Kind: "key_start", At: now.Add(time.Second), Deadline: now.Add(10 * time.Second), Members: []Character{alice}}, now.Add(time.Second))
		if len(q.pending) != 0 {
			t.Fatal("insertion triggered extra scan", mode)
		}
		key := now.Add(5 * time.Second)
		q.Handle(Event{ID: "key", Kind: "challenge_start", At: key, Deadline: key, Members: []Character{alice}}, key)
		want = 1
		if mode == 1 {
			want = 0
		}
		if len(q.pending) != want {
			t.Fatal("key", mode, len(q.pending))
		}
		q.pending = nil
		q.Handle(Event{ID: "held-key", Kind: "challenge_start", At: key.Add(time.Second), Deadline: key, Members: []Character{alice}}, key.Add(time.Second))
		if len(q.pending) != 0 {
			t.Fatal("held key repeated scan", mode)
		}
		later := now.Add(4 * time.Minute)
		q.Handle(Event{ID: "later", Kind: "roster", At: later, Members: []Character{alice}}, later)
		want = 0
		if mode == 4 {
			want = 1
		}
		if len(q.pending) != want {
			t.Fatal("periodic", mode, len(q.pending))
		}
	}
}
