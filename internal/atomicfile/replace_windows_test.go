//go:build windows

package atomicfile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReplaceSurvivesRealWindowsReadLock(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")
	source := filepath.Join(dir, ".state-new")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(80 * time.Millisecond)
		_ = syscall.CloseHandle(handle)
		close(released)
	}()
	if err = Replace(source, target); err != nil {
		t.Fatal(err)
	}
	<-released
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement failed: %q %v", data, err)
	}
}
