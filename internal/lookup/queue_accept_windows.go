//go:build windows

package lookup

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"runtime"
	"time"
	"unsafe"
)

func (c *captureSession) queueMarkerVisible(window uintptr, area image.Rectangle, nonce string) bool {
	if !visibleCaptureRect(window, area) {
		return false
	}
	bounds, ok := windowClientBounds(window)
	if !ok {
		return false
	}
	c.target, c.targetBounds = window, bounds
	defer func() { c.target = 0 }()
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return false
	}
	defer releaseDC.Call(0, dc)
	center := area.Min.Add(image.Pt(area.Dx()/2, area.Dy()/2))
	return c.captureQueueMarkerDC(dc, BridgePosition{X: center.X - 4, Y: center.Y - 1}, nonce)
}

func (c *captureSession) captureQueueMarkerDC(dc uintptr, position BridgePosition, nonce string) bool {
	data, valid := c.captureRGBCells(dc, position, 8, 2)
	expected := queueMarker(nonce)
	return valid && bytes.Equal(data, expected[:])
}

type queueMouseInput struct {
	X, Y              int32
	Data, Flags, Time uint32
	Extra             uintptr
}
type queueInput struct {
	Type  uint32
	Mouse queueMouseInput
}

func queueNativeInputs(inputs []queueInput) (int, error) {
	if len(inputs) == 0 {
		return 0, fmt.Errorf("empty queue input")
	}
	n, _, err := user32.NewProc("SendInput").Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if int(n) != len(inputs) {
		return int(n), err
	}
	return int(n), nil
}

func queueAbsolute(point image.Point) (queueInput, error) {
	x, _, _ := getSystemMetrics.Call(76)
	y, _, _ := getSystemMetrics.Call(77)
	w, _, _ := getSystemMetrics.Call(78)
	h, _, _ := getSystemMetrics.Call(79)
	left, top := int(int32(x)), int(int32(y))
	if w < 2 || h < 2 || point.X < left || point.Y < top || point.X >= left+int(w) || point.Y >= top+int(h) {
		return queueInput{}, fmt.Errorf("queue target outside desktop")
	}
	return queueInput{Mouse: queueMouseInput{X: int32(((point.X-left)*65536 + 32768) / int(w)), Y: int32(((point.Y-top)*65536 + 32768) / int(h)), Flags: 0xC001}}, nil // MOVE | ABSOLUTE | VIRTUALDESK; target pixel centers.
}

func queueCursor() (image.Point, bool) {
	var point struct{ X, Y int32 }
	ok, _, _ := user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
	return image.Pt(int(point.X), int(point.Y)), ok != 0
}

func queueHeld() bool {
	for _, key := range []uint16{1, 2, 4, 5, 6, 0x10, 0x11, 0x12, 0x5B, 0x5C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5} {
		v, _, _ := user32.NewProc("GetAsyncKeyState").Call(uintptr(key))
		if v&0x8000 != 0 {
			return true
		}
	}
	return false
}

func (c *captureSession) captureQueue(wow, client string) BridgeSample {
	hwnd, reason := c.gameWindow(wow, client)
	if reason != "" {
		return BridgeSample{Error: reason}
	}
	bounds, ok := windowClientBounds(hwnd)
	front, _, _ := foreground.Call()
	if !ok || front != hwnd || bounds.Dx() < 320 || bounds.Dy() < 200 {
		return BridgeSample{Error: "queue_game_not_foreground"}
	}
	position := queueBridgePosition(bounds.Dy())
	sample := c.captureBridgeLayout(position, wow, false, 0, client)
	if sample.Window != hwnd || len(sample.Queue) == 0 {
		return BridgeSample{Error: "queue_bridge_not_visible"}
	}
	return sample
}

// RunQueueAccept reads only the module's fixed inset data strip. An
// absent/disabled module cannot authorize clicks. This never activates WoW.
func RunQueueAccept(ctx context.Context, wow, client string, report func(string, ...any)) {
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
	interval := time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	problem := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		sample := capture.captureQueue(wow, client)
		next := 100 * time.Millisecond
		if sample.Error != "" {
			next = time.Second
		}
		if next != interval {
			interval = next
			ticker.Reset(interval)
		}
		r := gate.Observe(sample, client, time.Now())
		if r == nil {
			continue
		}
		p := queuePlatform{
			now: time.Now, foreground: func() uintptr { v, _, _ := foreground.Call(); return v }, executable: windowExecutable,
			matchesGame: func(exe, path, client string) bool { return gameExecutable(exe, path, client) },
			bounds:      windowClientBounds, visible: func(window uintptr, area image.Rectangle) bool {
				return capture.queueMarkerVisible(window, area, r.Nonce)
			}, capture: func() BridgeSample { return capture.captureQueue(wow, client) }, held: queueHeld, cursor: queueCursor,
			click: func(point image.Point) (int, error) {
				move, err := queueAbsolute(point)
				if err != nil {
					return 0, err
				}
				return queueNativeInputs([]queueInput{move, {Mouse: queueMouseInput{Flags: 2}}, {Mouse: queueMouseInput{Flags: 4}}})
			},
			release: func() { _, _ = queueNativeInputs([]queueInput{{Mouse: queueMouseInput{Flags: 4}}}) },
			restore: func(original, target image.Point) {
				front, _, _ := foreground.Call()
				position, ok := queueCursor()
				if front != sample.Window || !ok || position != target || queueHeld() {
					return
				}
				if move, err := queueAbsolute(original); err == nil {
					_, _ = queueNativeInputs([]queueInput{move})
				}
			},
		}
		attempted, err := sendQueueAccept(ctx, sample.Window, wow, client, *r, &gate, p)
		if attempted {
			if err != nil {
				report("LamdaQueue %s click failed: %v; invitation will not be retried", r.Kind, err)
			} else {
				report("LamdaQueue %s acceptance click submitted to Windows; game acceptance is not confirmed", r.Kind)
			}
			problem = ""
		} else if err != nil && err.Error() != problem {
			problem = err.Error()
			report("LamdaQueue deferred: %s", problem)
		}
	}
}
