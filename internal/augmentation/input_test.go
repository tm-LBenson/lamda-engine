package augmentation

import (
	"testing"
	"time"
)

func TestReadProgressDistinguishesBacklogAndSnapshotsStayImmutable(t *testing.T) {
	s := New("")
	at := time.Now()
	s.ObserveLogRead(10000, 1000, 20, at)
	before := s.Snapshot()
	if before.LogInput.BacklogBytes != 9000 {
		t.Fatal(before.LogInput)
	}
	s.ObserveLogRead(10000, 10000, 200, at.Add(time.Second))
	after := s.Snapshot()
	if after.LogInput.BacklogBytes != 0 || before.LogInput.BacklogBytes != 9000 {
		t.Fatal(before.LogInput, after.LogInput)
	}
	s.ObserveLogRead(10000, 10010, 1, at.Add(2*time.Second))
	if s.Snapshot().LogInput.BacklogBytes != 0 {
		t.Fatal("a concurrent append produced negative backlog")
	}
}
