package atomicfile

import (
	"errors"
	"io/fs"
	"syscall"
	"testing"
	"time"
)

func TestWindowsSharingFailureRetriesWithoutRemovingDestination(t *testing.T) {
	for name, transient := range map[string]error{
		"access denied":     fs.ErrPermission,
		"sharing violation": syscall.Errno(32),
	} {
		t.Run(name, func(t *testing.T) {
			attempts, pauses := 0, 0
			err := replace(func() error {
				attempts++
				if attempts < 3 {
					return transient
				}
				return nil
			}, true, func(time.Duration) { pauses++ })
			if err != nil || attempts != 3 || pauses != 2 {
				t.Fatalf("retry failed: error=%v attempts=%d pauses=%d", err, attempts, pauses)
			}
		})
	}
}

func TestReplaceDoesNotRetryPermanentOrNonWindowsFailures(t *testing.T) {
	permanent := errors.New("permanent")
	for name, failure := range map[string]struct {
		windows bool
		failure error
	}{
		"permanent":   {true, permanent},
		"non-windows": {false, fs.ErrPermission},
	} {
		t.Run(name, func(t *testing.T) {
			attempts := 0
			err := replace(func() error { attempts++; return failure.failure }, failure.windows, func(time.Duration) {})
			if !errors.Is(err, failure.failure) || attempts != 1 {
				t.Fatalf("unexpected retry: error=%v attempts=%d", err, attempts)
			}
		})
	}
}

func TestReplaceReturnsLastFailureAfterBoundedRetries(t *testing.T) {
	attempts, pauses := 0, 0
	err := replace(func() error { attempts++; return fs.ErrPermission }, true, func(time.Duration) { pauses++ })
	if !errors.Is(err, fs.ErrPermission) || attempts != replaceAttempts || pauses != replaceAttempts-1 {
		t.Fatalf("retry bound failed: error=%v attempts=%d pauses=%d", err, attempts, pauses)
	}
}
