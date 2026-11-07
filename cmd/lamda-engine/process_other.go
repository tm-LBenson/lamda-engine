//go:build !windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func hideWindow(cmd *exec.Cmd) {}

func openSimBrowser(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "xdg-open", address).Run()
}

func notifyUpdate(ctx context.Context, tag string) error { return nil }

func startFrameDisplay(context.Context, string) (<-chan error, error) {
	return nil, fmt.Errorf("frame display requires Windows")
}
