package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func openSimBrowser(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", address)
	hideWindow(cmd)
	return cmd.Run()
}

func startFrameDisplay(ctx context.Context, statePath string) (<-chan error, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	script := filepath.Join(filepath.Dir(executable), "frame-estimates.ps1")
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("frame-estimates.ps1 is missing: %w", err)
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-STA", "-File", script, "-StatePath", statePath, "-EnginePID", strconv.Itoa(os.Getpid()))
	cmd.Stderr = log.Writer()
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); close(done) }()
	return done, nil
}

// A local Windows update notice contains only a validated public release tag.
func notifyUpdate(ctx context.Context, tag string) error {
	if _, _, ok := parseReleaseVersion(tag); !ok {
		return fmt.Errorf("invalid release tag")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-STA", "-File", filepath.Join(filepath.Dir(executable), "notify-update.ps1"), "-Version", tag)
	hideWindow(cmd)
	return cmd.Run()
}
