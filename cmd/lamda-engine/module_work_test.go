package main

import (
	"context"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureStopsAndResumesWithModuleDemand(t *testing.T) {
	var contexts []context.Context
	c := moduleCapture{open: func(ctx context.Context, targets []lookup.BridgeTarget, wow, client string) <-chan lookup.BridgeSample {
		if len(targets) == 0 {
			t.Fatal("empty capture")
		}
		contexts = append(contexts, ctx)
		return make(chan lookup.BridgeSample)
	}}
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if c.Configure(ctx, nil, "wow", "retail") || c.samples != nil || len(contexts) != 0 {
		t.Fatal("disabled work started")
	}
	targets := []lookup.BridgeTarget{{Source: "kick"}}
	if !c.Configure(ctx, targets, "wow", "retail") || c.samples == nil || len(contexts) != 1 {
		t.Fatal("reader not started")
	}
	if c.Configure(ctx, targets, "wow", "retail") || contexts[0].Err() != nil {
		t.Fatal("unchanged reader restarted")
	}
	targets = append(targets, lookup.BridgeTarget{Source: "cd"})
	if !c.Configure(ctx, targets, "wow", "retail") || contexts[0].Err() == nil || contexts[1].Err() != nil {
		t.Fatal("changed readers not replaced")
	}
	if !c.Configure(ctx, nil, "wow", "retail") || c.samples != nil || contexts[1].Err() == nil {
		t.Fatal("reader still active")
	}
	c.Configure(ctx, targets[:1], "wow", "retail")
	if len(contexts) != 3 || c.samples == nil {
		t.Fatal("re-enable failed")
	}
	cancel()
	if contexts[2].Err() == nil {
		t.Fatal("shutdown did not propagate")
	}
}

func TestCombatInputSleepsWithoutConsumersAndDoesNotReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WoWCombatLog-test.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var tail combatInput
	if lines, err := tail.Poll(path+"[", false); err != nil || len(lines) != 0 || tail.active {
		t.Fatal("disabled input touched filesystem", err)
	}
	if lines, err := tail.Poll(dir, true); err != nil || len(lines) != 0 {
		t.Fatal("initial history replayed", err, lines)
	}
	appendLine := func(s string) {
		t.Helper()
		f, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if _, e = f.WriteString(s + "\n"); e != nil {
			t.Fatal(e)
		}
	}
	appendLine("active")
	if lines, err := tail.Poll(dir, true); err != nil || len(lines) != 1 || lines[0] != "active" {
		t.Fatal("required input lost", err, lines)
	}
	tail.Poll(dir, false)
	appendLine("disabled")
	if tail.Path != "" || tail.active {
		t.Fatal("idle tail retained")
	}
	if lines, err := tail.Poll(dir, true); err != nil || len(lines) != 0 {
		t.Fatal("disabled backlog replayed", err, lines)
	}
	appendLine("resumed")
	if lines, err := tail.Poll(dir, true); err != nil || len(lines) != 1 || lines[0] != "resumed" {
		t.Fatal("resume failed", err, lines)
	}
}
