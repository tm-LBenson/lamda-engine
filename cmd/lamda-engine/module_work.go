package main

import (
	"context"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// No reader or receiver channel exists when there are no enabled consumers.
// The settings watcher remains available to start a newly enabled module.
type moduleCapture struct {
	targets []lookup.BridgeTarget
	samples <-chan lookup.BridgeSample
	cancel  context.CancelFunc
	open    func(context.Context, []lookup.BridgeTarget, string, string) <-chan lookup.BridgeSample
}

func (c *moduleCapture) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	c.cancel = nil
	c.samples = nil
	c.targets = nil
}
func (c *moduleCapture) Configure(ctx context.Context, targets []lookup.BridgeTarget, wow, client string) bool {
	if sameStrips(c.targets, targets) && (len(targets) == 0 || c.samples != nil) {
		return false
	}
	c.Close()
	c.targets = append([]lookup.BridgeTarget(nil), targets...)
	if len(targets) > 0 {
		read := c.open
		if read == nil {
			read = lookup.ReadModuleBridges
		}
		var input context.Context
		input, c.cancel = context.WithCancel(ctx)
		c.samples = read(input, targets, wow, client)
	}
	return true
}

// Resume at EOF after an idle period, so disabling every consumer cannot build
// a backlog that is replayed when one is enabled again.
type combatInput struct {
	engine.Tail
	active bool
}

func (t *combatInput) Poll(dir string, needed bool) ([]string, error) {
	if !needed {
		if t.active {
			*t = combatInput{}
		}
		return nil, nil
	}
	t.active = true
	return t.Tail.Poll(dir)
}
