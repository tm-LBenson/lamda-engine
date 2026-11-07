package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

func demoModel(now time.Time) *engine.Model {
	m := engine.NewModel()
	for _, example := range []struct {
		guid, name string
		spell      int
		remaining  float64
		self       bool
	}{
		{"Player-DEMO-SELF", "Demo player", 55342, 40, true},
		{"Player-DEMO-SELF", "Demo player", 45438, 70, true},
		{"Player-DEMO-FRIEND", "Demo teammate", 102342, 22, false},
	} {
		at := now.Add(-time.Duration(engine.Catalog[example.spell].Cooldown-example.remaining) * time.Second)
		m.Observe(engine.Cast{GUID: example.guid, Name: example.name, Spell: example.spell, At: at, Self: example.self}, at.Add(time.Second))
	}
	// Provenance is explicitly fictional, even in the isolated demo state file.
	for key, e := range m.Evidence {
		e.Source = "demo"
		m.Evidence[key] = e
	}
	return m
}

func runDemo(dir string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	path := filepath.Join(dir, "demo-state.json")
	defer os.Remove(path)
	cfg := engine.DefaultConfig()
	cfg.Enabled = false
	cfg.EstimatePanel = false
	cfg.EstimateReady = true
	now := time.Now()
	cycle := now
	m := demoModel(now)
	write := func(now time.Time) error {
		snapshot := m.Snapshot(now)
		current := buildState(os.Getpid(), cfg, nil, "")
		current.Inference = &snapshot
		current.InputStatus = "demo"
		current.Estimates = engine.PanelFor(snapshot, cfg, "demo", time.Time{}, nil, now)
		current.Estimates.Demo = true
		current.Estimates.Heading = "Demo · cooldown estimates"
		current.Estimates.Status = "Fictional casts. No game data or public lookups."
		return engine.WriteJSON(path, current)
	}
	if err := write(now); err != nil {
		return err
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if now.Sub(cycle) > 90*time.Second {
				cycle = now
				m = demoModel(now)
			}
			if err := write(now); err != nil {
				return err
			}
		}
	}
}
