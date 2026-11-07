//go:build windows

package lookup

import (
	"encoding/json"
	"image"
	"os"
	"runtime"
	"testing"
	"time"
)

// Opt-in observation only: this probe never changes focus or submits input.
func TestQueueLiveReadOnly(t *testing.T) {
	game, output := os.Getenv("LAMDA_QUEUE_GAME"), os.Getenv("LAMDA_QUEUE_DIAGNOSTIC")
	if game == "" || output == "" {
		t.Skip("live queue observation not requested")
	}
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if setDPI.Find() == nil {
		old, _, _ := setDPI.Call(^uintptr(3))
		if old != 0 {
			defer setDPI.Call(old)
		}
	}
	var capture captureSession
	defer capture.Close()
	var gate QueueGate
	lastProblem := "initial"
	lastWrite := time.Time{}
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		now := time.Now()
		record := map[string]any{"at": now.Format(time.RFC3339Nano)}
		hwnd, problem := capture.gameWindow(game, "retail")
		if problem == "" {
			bounds, ok := windowClientBounds(hwnd)
			front, _, _ := foreground.Call()
			record["foreground"] = front == hwnd
			if !ok {
				problem = "bounds_unavailable"
			} else {
				sample := capture.captureBridgeLayout(queueBridgePosition(bounds.Dy()), game, false, 0, "retail")
				problem = sample.Error
				record["source"] = sample.Source
				record["width"], record["height"] = bounds.Dx(), bounds.Dy()
				if len(sample.Queue) > 0 {
					r, parseErr := ParseQueueRequest(sample.Queue, "retail", now)
					if parseErr != nil {
						problem = parseErr.Error()
					} else {
						record["kind"] = r.Kind
						record["remainingMs"] = r.Remaining
						record["due"] = r.Due
						record["tick"] = r.Tick
						record["x"], record["y"], record["buttonWidth"], record["buttonHeight"] = r.X, r.Y, r.ButtonWidth, r.ButtonHeight
						area := image.Rect(r.X, r.Y, r.X+r.ButtonWidth, r.Y+r.ButtonHeight).Add(bounds.Min)
						record["markerVisible"] = capture.queueMarkerVisible(hwnd, area, r.Nonce)
						record["gateReady"] = gate.Observe(sample, "retail", now) != nil
					}
				} else if problem == "" {
					problem = "no_queue_lease"
				}
			}
		}
		record["problem"] = problem
		if problem != lastProblem || (problem == "" && now.Sub(lastWrite) >= 500*time.Millisecond) || now.Sub(lastWrite) >= 10*time.Second {
			if err := encoder.Encode(record); err != nil {
				t.Fatal(err)
			}
			file.Sync()
			t.Logf("queue observation: %v", record)
			lastProblem, lastWrite = problem, now
		}
	}
}
