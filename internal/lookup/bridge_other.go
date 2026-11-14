//go:build !windows && !linux

package lookup

import "context"

func readTargetBridge(ctx context.Context, target BridgeTarget, game, client string) <-chan BridgeSample {
	return ReadBridge(ctx, target.Position, game, client)
}

func ReadBridge(context.Context, BridgePosition, string, ...string) <-chan BridgeSample {
	c := make(chan BridgeSample, 1)
	c <- BridgeSample{Error: "bridge_requires_windows"}
	close(c)
	return c
}

func ReadSimBridge(ctx context.Context, position BridgePosition, game string, clients ...string) <-chan BridgeSample {
	return ReadBridge(ctx, position, game, clients...)
}

func ReadOverlayBridge(ctx context.Context, position BridgePosition, width int, game string, clients ...string) <-chan BridgeSample {
	return ReadBridge(ctx, position, game, clients...)
}
