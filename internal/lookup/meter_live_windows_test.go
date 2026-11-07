//go:build windows

package lookup

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"
)

// Explicit, read-only real-client diagnostic. Never opens a window or sends input.
func TestLiveMeterControlReadOnly(t *testing.T) {
	wow := os.Getenv("LAMDA_LIVE_METER_READ")
	if wow == "" {
		t.Skip("real client diagnostic not requested")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if setDPI.Find() == nil {
		old, _, _ := setDPI.Call(^uintptr(3))
		if old != 0 {
			defer setDPI.Call(old)
		}
	}
	c := captureSession{surfaceLimit: 8}
	defer c.Close()
	last := ""
	for i := 0; i < 50; i++ {
		s, found := c.captureMeterBorder(BridgePosition{}, wow, "retail")
		b, _ := json.Marshal(struct {
			Found  bool
			Sample BridgeSample
		}{found, s})
		if string(b) != last {
			last = string(b)
			t.Log(string(b))
		}
		time.Sleep(100 * time.Millisecond)
	}
}
