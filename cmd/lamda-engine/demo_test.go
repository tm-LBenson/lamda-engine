package main

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

func TestDemoUsesSeparateFictionalEvidence(t *testing.T) {
	now := time.Now()
	live := engine.NewModel()
	demo := demoModel(now)
	if len(live.Evidence) != 0 || len(demo.Evidence) != 3 {
		t.Fatal("demo contaminated live model")
	}
	for _, e := range demo.Evidence {
		if e.Source != "demo" {
			t.Fatal(e)
		}
	}
	later := demo.Snapshot(now.Add(45 * time.Second))
	if len(later.Cooldowns) != 3 || later.Diagnostics.AcceptedCasts != 3 {
		t.Fatal("ticking a preview created observations")
	}
}
