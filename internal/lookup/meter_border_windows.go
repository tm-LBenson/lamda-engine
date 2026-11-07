//go:build windows

package lookup

import (
	"bytes"
	"context"
	"image"
	"runtime"
	"time"
	"unsafe"
)

// Capture only the six-cell position marker and the verified client's border.
// No full-screen search, input, game-memory access or saved screenshots.
func readMeterBorderBridge(ctx context.Context, target BridgeTarget, game, client string) <-chan BridgeSample {
	ch := make(chan BridgeSample, 1)
	go func() {
		defer close(ch)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if setDPI.Find() == nil {
			old, _, _ := setDPI.Call(^uintptr(3))
			if old != 0 {
				defer setDPI.Call(old)
			}
		}
		capture := captureSession{surfaceLimit: 8}
		defer capture.Close()
		ticker := time.NewTicker(activeCaptureInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				value, found := capture.captureMeterBorderSlot(target.LocatorBase, target.MeterSlot, game, client)
				if !found {
					if target.MeterSlot > 0 {
						continue
					}
					value = capture.captureBridgeLayout(target.Position, game, true, 0, client)
					value.Position = target.Position
				}
				value.LocatorBase = target.LocatorBase
				select {
				case ch <- value:
				default:
					select {
					case <-ch:
					default:
					}
					select {
					case ch <- value:
					default:
					}
				}
			}
		}
	}()
	return ch
}

func (c *captureSession) captureMeterBorder(base BridgePosition, game, client string) (result BridgeSample, found bool) {
	return c.captureMeterBorderSlot(base, 0, game, client)
}
func (c *captureSession) captureMeterBorderSlot(base BridgePosition, slot int, game, client string) (result BridgeSample, found bool) {
	result.Source = "augmentation"
	result.MeterSlot = slot
	hwnd, reason := c.gameWindow(game, client)
	if reason != "" {
		result.Error = reason
		return result, true
	}
	bounds, ok := windowClientBounds(hwnd)
	if !ok {
		result.Error = "bridge_capture_failed"
		return result, true
	}
	c.target, c.targetBounds, c.captureError = hwnd, bounds, ""
	defer func() { c.target = 0 }()
	result.Window = hwnd
	windowProcess.Call(hwnd, uintptr(unsafe.Pointer(&result.WindowPID)))
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		result.Error = "bridge_capture_failed"
		return result, true
	}
	defer releaseDC.Call(0, dc)
	loc, ok := MeterLocatorPosition(base, bounds.Dx(), bounds.Dy())
	if !ok {
		return result, false
	}
	if slot < 0 || slot > 7 {
		return result, false
	}
	loc.X += slot * 6
	mark, ok := c.capturePixels(dc, BridgePosition{X: bounds.Min.X + loc.X, Y: bounds.Min.Y + loc.Y}, 6, 4)
	if !ok {
		result.Error = c.captureError
		return result, true
	}
	pointer, err := ParseMeterBorderPointer(mark.Cells(image.Rect(0, 0, 6, 4)))
	if err != nil {
		return result, false
	}
	if pointer.Inactive {
		if slot > 0 {
			return result, false
		}
		result.Error = "native_meter_inactive_spec"
		return result, true
	}
	if pointer.WindowHidden || pointer.WindowHeld {
		result.NativeMeter = &NativeMeter{ControlOnly: true, ControlHidden: pointer.WindowHidden, Frame: &NativeMeterFrame{WindowID: slot + 1}}
		return result, true
	}
	if pointer.Paused {
		result.Error = "native_meter_capture_paused"
		return result, true
	}
	if pointer.ShareCapable {
		result.NativeMeter = &NativeMeter{ControlOnly: true, ShareCapable: true, ShareOpenVersion: pointer.ShareOpenVersion}
		return result, true
	}
	position := BridgePosition{X: pointer.X, Y: pointer.Y}
	result.Position = position
	if pointer.X+pointer.Width > bounds.Dx() || pointer.Y+pointer.Height > bounds.Dy() {
		result.Error = "meter_border_outside_window"
		return result, true
	}
	// The CRC-bound public pointer may reserve a header hole, never damage.
	result.NativeMeter = &NativeMeter{HeaderOnly: true, Error: "meter_border_header_pending"}
	origin := BridgePosition{X: bounds.Min.X + position.X, Y: bounds.Min.Y + position.Y}
	if pointer.ControlOnly {
		result.NativeMeter = &NativeMeter{ControlOnly: true, ControlWidth: 64, ControlHeight: 8}
		pixels, ok := c.capturePixels(dc, origin, 64, 8)
		if !ok {
			result.Error = c.captureError
			return result, true
		}
		control, err := parseMeterControl(pixels.Cells(image.Rect(0, 0, 64, 8)))
		if err != nil {
			result.Error = err.Error()
			return result, true
		}
		last, ok := c.capturePixels(dc, BridgePosition{X: bounds.Min.X + loc.X, Y: bounds.Min.Y + loc.Y}, 6, 4)
		if !ok || !bytes.Equal(mark.Cells(image.Rect(0, 0, 6, 4)), last.Cells(image.Rect(0, 0, 6, 4))) {
			result.Error = "meter_control_changed"
			return result, true
		}
		result.NativeMeter = control
		return result, true
	}
	first, ok := c.capturePixels(dc, origin, 6, 8)
	if !ok {
		result.Error = c.captureError
		return result, true
	}
	headerBytes := first.Cells(image.Rect(0, 0, 6, 8))
	h, err := ParseMeterBorderHeader(headerBytes)
	if err != nil {
		result.Error = "meter_border_header_unreadable"
		return result, true
	}
	if h.Layout.Width() != pointer.Width || h.Layout.Height() != pointer.Height {
		result.Error = "meter_border_size_pending"
		return result, true
	}
	result.NativeMeter = &NativeMeter{Border: &h.Layout, Frame: &NativeMeterFrame{Standalone: h.Layout.Standalone(), X: position.X + h.Layout.Edges[3], Y: position.Y + h.Layout.Edges[0], Width: h.Layout.InnerWidth, Height: h.Layout.InnerHeight}, Error: "meter_border_bands_pending"}
	p := Pixels{Width: h.Layout.Width(), Height: h.Layout.Height(), BGRA: make([]byte, h.Layout.Width()*h.Layout.Height()*4)}
	for _, band := range h.Layout.Bands() {
		if band.Empty() {
			continue
		}
		for y := band.Min.Y; y < band.Max.Y; {
			height := min(band.Max.Y-y, 65536/band.Dx())
			if height < 1 {
				result.Error = "meter_border_band_too_large"
				return result, true
			}
			pixels, good := c.capturePixels(dc, BridgePosition{X: origin.X + band.Min.X, Y: origin.Y + y}, band.Dx(), height)
			if !good {
				result.Error = c.captureError
				return result, true
			}
			for row := 0; row < height; row++ {
				to := ((y+row)*p.Width + band.Min.X) * 4
				from := row * band.Dx() * 4
				copy(p.BGRA[to:to+band.Dx()*4], pixels.BGRA[from:from+band.Dx()*4])
			}
			y += height
		}
	}
	last, ok := c.capturePixels(dc, origin, 6, 8)
	if !ok {
		result.Error = c.captureError
		return result, true
	}
	if !bytes.Equal(headerBytes, last.Cells(image.Rect(0, 0, 6, 8))) {
		result.Error = "meter_border_changed_during_capture"
		return result, true
	}
	current, valid := windowClientBounds(hwnd)
	if !valid || current != bounds || !windowVisible(hwnd) || !gameExecutable(windowExecutable(hwnd), game, client) {
		result.Error = "bridge_window_changed"
		return result, true
	}
	decoded := DecodeMeterBorder(p)
	if decoded.NativeMeter == nil {
		// The CRC-checked header still identifies capture holes when body
		// bytes are occluded. Preserve geometry, never trust numeric values.
		decoded.NativeMeter = result.NativeMeter
	}
	decoded.Window = result.Window
	decoded.WindowPID = result.WindowPID
	decoded.Position = position
	if decoded.NativeMeter != nil && decoded.NativeMeter.Frame != nil {
		f := decoded.NativeMeter.Frame
		if f.WindowID == 0 && (f.X != position.X+h.Layout.Edges[3] || f.Y != position.Y+h.Layout.Edges[0]) {
			decoded.NativeMeter.Captured = false
			decoded.NativeMeter.Error = "meter_border_position_pending"
		}
	}
	return decoded, true
}
