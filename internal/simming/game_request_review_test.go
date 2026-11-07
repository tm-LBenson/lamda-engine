package simming

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type gameReviewProvider struct{ runs atomic.Int32 }

func (p *gameReviewProvider) Capability(*Snapshot, time.Time) Capability {
	return Capability{Available: true, Name: "Review fixture"}
}
func (p *gameReviewProvider) Run(ctx context.Context, _ *Snapshot, _ Scenario, _ []string, _ string) (*Result, error) {
	p.runs.Add(1)
	<-ctx.Done()
	return nil, ctx.Err()
}

func gameReviewRequest(id, task string) *Snapshot {
	s := gameFixture()
	s.ID, s.GameRequest.ID, s.GameRequest.Task = id, id, task
	return s
}

func gameReviewOutcome(t *testing.T, w *Workspace, id string) Job {
	t.Helper()
	for _, j := range w.State().Jobs {
		if j.RequestID == id {
			return j
		}
	}
	t.Fatalf("no visible outcome for request %s", id)
	return Job{}
}

func TestGameReviewOrphanRequestBecomesInterruptedWithoutRerunning(t *testing.T) {
	dir := t.TempDir()
	provider := &gameReviewProvider{}
	w, err := NewWorkspace(context.Background(), dir, "retail", provider)
	if err != nil {
		t.Fatal(err)
	}
	s := gameReviewRequest("interrupted_request", "baseline")
	// Simulate the engine stopping after snapshot persistence and before it
	// could create the job or record a rejection. Never rerun this implicitly.
	if fresh, err := w.Receive(s); !fresh || err != nil {
		t.Fatal(fresh, err)
	}
	w.Close()
	w, err = NewWorkspace(context.Background(), dir, "retail", provider)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	j := gameReviewOutcome(t, w, s.ID)
	if j.State != "interrupted" || j.Error == "" || j.GameTask != "baseline" || j.SnapshotID != s.ID {
		t.Fatalf("orphan request has no useful interrupted outcome: %+v", j)
	}
	if fresh, err := w.ReceiveGame(s); fresh || err != nil || provider.runs.Load() != 0 {
		t.Fatalf("replayed orphan started work: %v %v %d", fresh, err, provider.runs.Load())
	}
	if len(w.State().Jobs) != 1 {
		t.Fatal("orphan replay duplicated the outcome")
	}
}

func TestGameReviewBusyAndCrossSpecControlsHaveIndependentVisibleOutcomes(t *testing.T) {
	provider := &gameReviewProvider{}
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", provider)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	first := gameReviewRequest("active_baseline", "baseline")
	if _, err = w.ReceiveGame(first); err != nil {
		t.Fatal(err)
	}
	active := gameReviewOutcome(t, w, first.ID)
	busy := gameReviewRequest("busy_baseline", "baseline")
	if fresh, err := w.ReceiveGame(busy); !fresh || err == nil {
		t.Fatal("busy request accepted", fresh, err)
	}
	if j := gameReviewOutcome(t, w, busy.ID); j.State != "failed" || !strings.Contains(j.Error, "already running") {
		t.Fatal(j)
	}
	for _, task := range []string{"cancel", "plan"} {
		r := gameReviewRequest("foreign_spec_"+task, task)
		r.Character.Spec = 63
		r.GameRequest.JobID = active.ID
		if task == "plan" {
			r.GameRequest.PlanOptions = &PlanOptions{Mode: "farm", View: "drop", Chance: .4, Cost: 30}
		}
		if fresh, err := w.ReceiveGame(r); !fresh || err == nil {
			t.Fatal("cross-spec control accepted", fresh, err)
		}
		if j := gameReviewOutcome(t, w, r.ID); j.State != "failed" || j.Plan != nil || !strings.Contains(j.Error, "specialization") {
			t.Fatal(j)
		}
	}
	if j := gameReviewOutcome(t, w, first.ID); j.State != "running" {
		t.Fatal("foreign request changed active run", j)
	}
	cancel := gameReviewRequest("own_cancel", "cancel")
	cancel.GameRequest.JobID = active.ID
	if _, err = w.ReceiveGame(cancel); err != nil {
		t.Fatal(err)
	}
	if j := gameReviewOutcome(t, w, cancel.ID); j.State != "complete" || j.Result != nil || j.Error != "" {
		t.Fatal(j)
	}
	if j := waitJob(t, w, active.ID); j.State != "cancelled" {
		t.Fatal(j)
	}
}

func TestGameReviewPlanRetainsRewardDenominatorAndOriginalRun(t *testing.T) {
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", &gameReviewProvider{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := fixture(time.Now())
	if _, err = w.Receive(s); err != nil {
		t.Fatal(err)
	}
	original := &Job{ID: "completed_loot", SnapshotID: s.ID, State: "complete", CreatedAt: time.Now(), Result: &Result{
		Provider: "Questionably Epic", Baseline: Metric{Mean: 100}, Upgrades: []Upgrade{
			{ID: 1, SourceID: "raid:1", Source: "Boss", Kind: "Raid", Type: "drop", Gain: 10, PoolComplete: true, PoolSize: 2},
			{ID: 2, SourceID: "raid:1", Source: "Boss", Kind: "Raid", Type: "drop", Gain: -20, PoolComplete: true, PoolSize: 2},
			{ID: 3, SourceID: "raid:2", Source: "Unresolved boss", Kind: "Raid", Type: "drop", Gain: 100, PoolComplete: false, PoolSize: 2},
		}}}
	w.jobs[original.ID] = original
	r := gameReviewRequest("plan_request", "plan")
	r.GameRequest.JobID = original.ID
	r.GameRequest.PlanOptions = &PlanOptions{Mode: "farm", View: "drop", Chance: .4, Cost: 30}
	if fresh, err := w.ReceiveGame(r); !fresh || err != nil {
		t.Fatal(fresh, err)
	}
	j := gameReviewOutcome(t, w, r.ID)
	if j.State != "complete" || j.Result != nil || j.Plan == nil || j.Plan.JobID != original.ID || len(j.Plan.Sources) != 1 {
		t.Fatalf("plan not linked to the original eligible run: %+v", j)
	}
	p := j.Plan.Sources[0]
	if p.ID != "raid:1" || p.Pool != 2 || p.Expected != 2 || p.UpgradeChance != .2 {
		t.Fatalf("non-upgrades vanished or incomplete pool ranked: %+v", p)
	}
	if original.Plan != nil || original.GameTask != "" || original.RequestID != "" || original.Result.Upgrades[1].Gain != -20 {
		t.Fatal("planning altered the original calculation")
	}
	if fresh, err := w.ReceiveGame(r); fresh || err != nil || len(w.State().Jobs) != 2 {
		t.Fatal("plan replay repeated", fresh, err)
	}
}
