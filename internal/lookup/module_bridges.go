package lookup

import (
	"context"
	"strings"
	"sync"
	"time"
)

// A target names an independent strip. Empty Source retains the old shared
// packet transport until the upgraded addon has saved its layout on reload.
type BridgeTarget struct {
	MeterSlot    int
	IdleInterval time.Duration
	Source       string
	Position     BridgePosition
	LocatorBase  BridgePosition
	Wide         bool
	OverlayWidth int
}

func ReadModuleBridges(ctx context.Context, targets []BridgeTarget, wow, client string) <-chan BridgeSample {
	return readModuleBridges(ctx, targets, func(ctx context.Context, target BridgeTarget) <-chan BridgeSample {
		return readTargetBridge(ctx, target, wow, client)
	})
}

func readModuleBridges(ctx context.Context, targets []BridgeTarget, read func(context.Context, BridgeTarget) <-chan BridgeSample) <-chan BridgeSample {
	out := make(chan BridgeSample, len(targets))
	var frameLock sync.Mutex
	frames := map[int]nativeWindowRevision{}
	for _, target := range targets {
		go func(target BridgeTarget) {
			input := read(ctx, target)
			for {
				select {
				case <-ctx.Done():
					return
				case sample, ok := <-input:
					if !ok {
						return
					}
					if sample.Error == "" && target.Source != "" && sample.Source != target.Source {
						sample = BridgeSample{Error: "bridge_wrong_module", Window: sample.Window}
					}
					// Legacy readers still receive arena and wide packets under
					// their original dispatch; only new layouts isolate failures.
					if target.Source != "" {
						sample.Source = target.Source
					}
					sample.MeterSlot = target.MeterSlot
					if sample.NativeMeter == nil || (sample.NativeMeter.Border == nil && !sample.NativeMeter.HeaderOnly && !sample.NativeMeter.ControlOnly) {
						sample.Position = target.Position
					}
					for _, value := range expandMeterAliases(sample) {
						frameLock.Lock()
						if !acceptNativeWindowRevision(frames, value) {
							frameLock.Unlock()
							continue
						}
						select {
						case out <- value:
						case <-ctx.Done():
							frameLock.Unlock()
							return
						}
						frameLock.Unlock()
					}
				}
			}
		}(target)
	}
	return out
}

type nativeWindowRevision struct {
	owner, epoch string
	serial       int
}

// A window can move between its own reader and a shared source. Preserve its
// monotonic serial across that move, so a late source cannot restore old menus
// or metrics. A new Lua epoch (reload/login) starts a new revision sequence.
func acceptNativeWindowRevision(last map[int]nativeWindowRevision, s BridgeSample) bool {
	n := s.NativeMeter
	if n == nil || !n.Captured || n.Frame == nil || n.Frame.WindowID == 0 || !strings.HasPrefix(n.ID, "overlay:") {
		return true
	}
	epoch := n.ID[:strings.LastIndex(n.ID, ":")]
	id := n.Frame.WindowID
	if p, ok := last[id]; ok && p.owner == n.Player && p.epoch == epoch {
		delta := (n.Serial - p.serial + 16777215) % 16777215
		if delta == 0 || delta > 16777215/2 {
			return false
		}
	}
	last[id] = nativeWindowRevision{n.Player, epoch, n.Serial}
	return true
}
