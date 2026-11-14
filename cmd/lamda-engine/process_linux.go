//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func hideWindow(cmd *exec.Cmd) {}

func openSimBrowser(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "xdg-open", address).Run()
}

func notifyUpdate(context.Context, string) error { return nil }

func startFrameDisplay(ctx context.Context, state string) (<-chan error, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	python := os.Getenv("LAMDA_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmd := exec.CommandContext(ctx, python, filepath.Join(filepath.Dir(executable), "linux", "frame_estimates.py"), "--state", state, "--engine-pid", fmt.Sprint(os.Getpid()))
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); close(done) }()
	return done, nil
}
