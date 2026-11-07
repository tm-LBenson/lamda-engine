package simming

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestGamePublicationReadsStateInsideSerializationLock(t *testing.T) {
	addons, path := seedGameDestination(t, "retail")
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", &Provider{})
	if err != nil {
		t.Fatal(err)
	}
	state := gameStateFixture()
	s, job := state.Snapshots[0].Snapshot, state.Jobs[0]
	job.State, job.RequestID = "running", s.ID
	w.snapshots[s.ID], w.jobs[job.ID] = &s, &job
	w.publicationMu.Lock()
	started, done := make(chan struct{}), make(chan error, 1)
	go func() { close(started); _, err := w.PublishResults(addons, s.ID); done <- err }()
	<-started
	select {
	case err := <-done:
		w.publicationMu.Unlock()
		t.Fatalf("publication bypassed lock: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	w.mu.Lock()
	job.State = "complete"
	job.Error = "latest state after publisher began waiting"
	w.mu.Unlock()
	w.publicationMu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`["state"]="complete"`)) || !bytes.Contains(data, []byte(job.Error)) {
		t.Fatal("publisher wrote a state captured before acquiring its lock")
	}
	// Periodic and reload-triggered publication use the same method. Concurrent
	// callers must all complete without the destination-change CAS rejecting one.
	var wg sync.WaitGroup
	errors := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			required := ""
			if i%2 == 0 {
				required = s.ID
			}
			_, err := w.PublishResults(addons, required)
			errors <- err
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestReloadPublicationRequiresRequestToSurviveHistoryBounds(t *testing.T) {
	addons, path := seedGameDestination(t, "retail")
	now := time.Unix(1800000010, 0)
	state := gameStateFixture()
	original := state.Snapshots[0]
	request := original.Snapshot.ID
	state.Jobs[0].RequestID = request
	if err := exportGameResults(addons, state, now, request); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	for i := 0; i < MaxGameGroups; i++ {
		s := original
		s.Snapshot.ID = fmt.Sprintf("newer_%d", i)
		s.Snapshot.Character.GUID = fmt.Sprintf("Player-other-%d", i)
		s.Snapshot.CapturedAt += int64(1 + i)
		state.Snapshots = append(state.Snapshots, s)
	}
	if err := exportGameResults(addons, state, now, request); err == nil {
		t.Fatal("reload publication accepted an omitted request")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed required-request publication replaced the current file")
	}
	if _, err := BuildGameResults(state, now); err != nil {
		t.Fatal("ordinary bounded history export should remain available", err)
	}
}

type publicationProvider struct {
	Provider
	calls int
}

func (p *publicationProvider) Capability(s *Snapshot, at time.Time) Capability {
	p.calls++
	return p.Provider.Capability(s, at)
}
func TestUnchangedPublicationReusesSummaryAndRepairsReplacedSeed(t *testing.T) {
	addons, path := seedGameDestination(t, "retail")
	seed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := &publicationProvider{}
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", provider)
	if err != nil {
		t.Fatal(err)
	}
	state := gameStateFixture()
	s := state.Snapshots[0].Snapshot
	j := state.Jobs[0]
	w.snapshots[s.ID], w.jobs[j.ID] = &s, &j
	if _, err = w.PublishResults(addons, ""); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	calls := provider.calls
	if _, err = w.PublishResults(addons, ""); err != nil || provider.calls != calls {
		t.Fatal("unchanged publication rebuilt summaries", err)
	}
	if err = os.WriteFile(path, seed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = w.PublishResults(addons, ""); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(path)
	if !bytes.Equal(restored, first) {
		t.Fatal("cached output did not recover replaced seed")
	}
	w.mu.Lock()
	j.Error = "new result"
	w.revision++
	w.mu.Unlock()
	if _, err = w.PublishResults(addons, ""); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(path)
	if !bytes.Contains(updated, []byte("new result")) {
		t.Fatal("job change did not publish")
	}
}
