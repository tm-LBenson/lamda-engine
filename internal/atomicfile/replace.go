// Package atomicfile provides the final replace step for durable temporary-file writes.
package atomicfile

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
	"syscall"
	"time"
)

const replaceAttempts = 16

// Replace never removes the destination first. Windows readers and security
// scanners can briefly omit delete sharing, so retry only those sharing/access
// failures while the old complete file remains in place.
func Replace(source, destination string) error {
	return replace(func() error { return os.Rename(source, destination) }, runtime.GOOS == "windows", time.Sleep)
}

func replace(run func() error, windows bool, pause func(time.Duration)) error {
	var err error
	for attempt := 0; attempt < replaceAttempts; attempt++ {
		if err = run(); err == nil {
			return nil
		}
		if !windows || (!errors.Is(err, fs.ErrPermission) && !errors.Is(err, syscall.Errno(5)) && !errors.Is(err, syscall.Errno(32))) {
			return err
		}
		if attempt+1 < replaceAttempts {
			delay := 10 * time.Millisecond << min(attempt, 3)
			pause(delay)
		}
	}
	return err
}
