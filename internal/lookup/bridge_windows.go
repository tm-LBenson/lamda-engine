//go:build windows

package lookup

import (
	"context"
	"image"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var gdi32 = syscall.NewLazyDLL("gdi32.dll")
var foreground = user32.NewProc("GetForegroundWindow")
var windowProcess = user32.NewProc("GetWindowThreadProcessId")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var openProcess = kernel32.NewProc("OpenProcess")
var closeHandle = kernel32.NewProc("CloseHandle")
var processImage = kernel32.NewProc("QueryFullProcessImageNameW")
var getDC = user32.NewProc("GetDC")
var releaseDC = user32.NewProc("ReleaseDC")
var clientRect = user32.NewProc("GetClientRect")
var clientToScreen = user32.NewProc("ClientToScreen")
var compatibleDC = gdi32.NewProc("CreateCompatibleDC")
var compatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
var selectObject = gdi32.NewProc("SelectObject")
var deleteObject = gdi32.NewProc("DeleteObject")
var deleteDC = gdi32.NewProc("DeleteDC")
var bitBlt = gdi32.NewProc("BitBlt")
var dibits = gdi32.NewProc("GetDIBits")
var setDPI = user32.NewProc("SetThreadDpiAwarenessContext")

func gameExecutable(path, retail string, clients ...string) bool {
	if len(clients) > 0 && clients[0] == "forever" {
		executable := "WowB.exe"
		if strings.EqualFold(filepath.Base(filepath.Clean(retail)), "_forever_") {
			executable = "Wow.exe"
		}
		return retail != "" && strings.EqualFold(filepath.Clean(path), filepath.Join(retail, executable))
	}
	if retail == "" || path == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(path), filepath.Join(retail, "Wow.exe")) ||
		strings.EqualFold(filepath.Clean(path), filepath.Join(retail, "WowT.exe"))
}

// Check the owning executable in the configured installation. WoW 12.1 uses
// "waApplication Window" here; class names are not a stable game identity.
// PROCESS_QUERY_LIMITED_INFORMATION reads process metadata, never game memory.
func windowExecutable(hwnd uintptr) string {
	var pid uint32
	windowProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	handle, _, _ := openProcess.Call(0x1000, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer closeHandle.Call(handle)
	var path [32768]uint16
	size := uint32(len(path))
	ok, _, _ := processImage.Call(handle, 0, uintptr(unsafe.Pointer(&path[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(path[:size])
}

func captureBridge(position BridgePosition, retail string, clients ...string) BridgeSample {
	return captureBridgeMode(position, retail, false, clients...)
}

func captureBridgeMode(position BridgePosition, retail string, sim bool, clients ...string) BridgeSample {
	return captureBridgeLayout(position, retail, sim, 0, clients...)
}
func (capture *captureSession) captureBridgeLayout(position BridgePosition, retail string, sim bool, overlayWidth int, clients ...string) (sample BridgeSample) {
	hwnd, reason := capture.gameWindow(retail, clients...)
	if reason != "" {
		return BridgeSample{Error: reason}
	}
	bounds, valid := windowClientBounds(hwnd)
	if !valid {
		return BridgeSample{Error: "bridge_capture_failed", Window: hwnd}
	}
	capture.target, capture.targetBounds, capture.captureError = hwnd, bounds, ""
	defer func() {
		if sample.Error != "" && capture.captureError != "" {
			sample.Error = capture.captureError
		}
		capture.target = 0
	}()
	var rect struct{ Left, Top, Right, Bottom int32 }
	ok, _, _ := clientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return BridgeSample{Error: "bridge_capture_failed"}
	}
	layoutPosition := position
	widePosition, wideFits := position.clampSize(int(rect.Right), int(rect.Bottom), SimBridgeWidth, SimBridgeHeight)
	position, fits := position.Clamp(int(rect.Right), int(rect.Bottom))
	if !fits {
		return BridgeSample{Error: "bridge_window_too_small"}
	}
	if overlayWidth != 0 {
		if !ValidOverlayWidth(overlayWidth) {
			return BridgeSample{Error: "overlays_shape_invalid"}
		}
		widePosition, wideFits = layoutPosition.clampSize(int(rect.Right), int(rect.Bottom), overlayWidth, 18432/overlayWidth)
		if !wideFits {
			return BridgeSample{Error: "bridge_window_too_small"}
		}
	}
	var origin struct{ X, Y int32 }
	ok, _, _ = clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin)))
	if ok == 0 {
		return BridgeSample{Error: "bridge_capture_failed"}
	}
	// Read the visible strip at its desktop coordinates. A DirectX window DC
	// can expose an old redirected surface instead of the currently presented frame.
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return BridgeSample{Error: "bridge_capture_failed"}
	}
	defer releaseDC.Call(0, dc)
	position = BridgePosition{X: position.X + int(origin.X), Y: position.Y + int(origin.Y)}
	if sim {
		widePosition = BridgePosition{X: widePosition.X + int(origin.X), Y: widePosition.Y + int(origin.Y)}
		if overlayWidth != 0 {
			p, ok := capture.capturePixels(dc, widePosition, overlayWidth, 18432/overlayWidth)
			if !ok {
				return BridgeSample{Error: "overlays_capture_failed"}
			}
			sample = DecodeOverlayShape(p)
		} else {
			sample = capture.captureSimBridgeDC(dc, widePosition, position, wideFits)
		}
	} else {
		sample = capture.captureBridgeDC(dc, position)
		if sample.Error == "" && sample.Observations != nil && sample.Observations.Auras != nil && sample.Observations.Screen == nil {
			a := sample.Observations.Auras
			top := min(0, a.Offset)
			if position.Y+top >= int(origin.Y) && position.Y+top+24 <= int(origin.Y+rect.Bottom) {
				if confirmed := capture.captureAuraBridgeDC(dc, position, a.Offset); confirmed.Error == "" {
					sample = confirmed
				}
			}
		}
		if sample.Error == "" && sample.Observations != nil && sample.Observations.Screen != nil {
			offset := sample.Observations.Screen.Offset
			auraOffset := 0
			if a := sample.Observations.Auras; a != nil {
				auraOffset = a.Offset
			}
			panelHeight := sample.Observations.Screen.Height()
			top, bottom := screenBounds(offset, auraOffset, panelHeight)
			if position.Y+top >= int(origin.Y) && position.Y+bottom <= int(origin.Y+rect.Bottom) {
				if confirmed := capture.captureScreenBridgeDC(dc, position, offset, auraOffset, panelHeight); confirmed.Error == "" {
					sample = confirmed
				} else {
					sample.Observations.Screen.Error = confirmed.Error
				}
			} else {
				sample.Observations.Screen.Error = "screen_panel_outside_window"
			}
		}
	}

	if sample.Error == "" && sample.NativeMeter != nil {
		n := sample.NativeMeter
		top, bottom := min(0, n.Offset), max(32, n.Offset+n.Height())
		if widePosition.Y+top < int(origin.Y) || widePosition.Y+bottom > int(origin.Y+rect.Bottom) {
			n.Error = "native_meter_panel_outside_window"
		} else {
			sample = capture.captureNativeMeterDC(dc, widePosition, n)
		}
	}
	if sample.Error == "" && sample.Overlays != nil {
		a := sample.Overlays
		if a.Width != int(rect.Right) || a.Height != int(rect.Bottom) {
			return BridgeSample{Error: "overlays_window_size_changed"}
		}
		if overlayWidth == 0 {
			top, bottom := min(0, a.Offset), max(32, a.Offset+40)
			if widePosition.Y+top < int(origin.Y) || widePosition.Y+bottom > int(origin.Y+rect.Bottom) {
				return BridgeSample{Error: "overlays_panel_outside_window"}
			}
			pixels, valid := capture.capturePixels(dc, BridgePosition{X: widePosition.X, Y: widePosition.Y + top}, 256, bottom-top)
			if !valid {
				return BridgeSample{Error: "overlays_capture_failed"}
			}
			sample = DecodeOverlayPixels(pixels, -top, a.Offset-top)
		}
	}
	currentBounds, currentValid := windowClientBounds(hwnd)
	if !currentValid || currentBounds != bounds || !windowVisible(hwnd) || !gameExecutable(windowExecutable(hwnd), retail, clients...) {
		return BridgeSample{Error: "bridge_window_changed"}
	}
	sample.Window = hwnd
	windowProcess.Call(hwnd, uintptr(unsafe.Pointer(&sample.WindowPID)))
	// Geometry is only meaningful for the exact client size that produced it.
	if sample.Arena != nil && (sample.Arena.Width != int(rect.Right) || sample.Arena.Height != int(rect.Bottom)) {
		return BridgeSample{Error: "arena_window_size_changed"}
	}
	if sample.Frames != nil && (sample.Frames.Width != int(rect.Right) || sample.Frames.Height != int(rect.Bottom)) {
		return BridgeSample{Error: "frame_window_size_changed"}
	}
	if sample.Kick != nil && (sample.Kick.Width != int(rect.Right) || sample.Kick.Height != int(rect.Bottom)) {
		return BridgeSample{Error: "kick_window_size_changed"}
	}
	return sample
}

func screenBounds(offset, auraOffset int, heights ...int) (int, int) {
	height := ScreenCastHeight
	if len(heights) > 0 {
		height = heights[0]
	}
	top, bottom := min(0, offset), max(16, offset+height)
	if auraOffset != 0 {
		top = min(top, auraOffset)
		bottom = max(bottom, auraOffset+8)
	}
	return top, bottom
}

func (capture *captureSession) captureScreenBridgeDC(dc uintptr, position BridgePosition, offset, auraOffset int, heights ...int) BridgeSample {
	height := ScreenCastHeight
	if len(heights) > 0 {
		height = heights[0]
	}
	if (height != ScreenCastHeight && height != ColorCastHeight && height != MeterCastHeight) || (offset != 24 && offset != -(height+8)) {
		return BridgeSample{Error: "screen_capture_offset"}
	}
	top, bottom := screenBounds(offset, auraOffset, height)
	p, ok := capture.capturePixels(dc, BridgePosition{position.X, position.Y + top}, 128, bottom-top)
	if !ok {
		return BridgeSample{Error: "screen_capture_failed"}
	}
	s := DecodeSample(p.Cells(image.Rect(0, -top, 128, -top+16)))
	if s.Error != "" {
		return s
	}
	if s.Observations == nil || s.Observations.Screen == nil || s.Observations.Screen.Offset != offset || s.Observations.Screen.Height() != height {
		return BridgeSample{Error: "screen_mapping_changed"}
	}
	DecodeScreenCasts(s.Observations.Screen, p, offset-top)
	if a := s.Observations.Auras; a != nil {
		DecodeAuraWatch(a, p.Cells(image.Rect(0, a.Offset-top, 128, a.Offset-top+8)))
	}
	return s
}

func (capture *captureSession) captureBridgeDC(dc uintptr, position BridgePosition) BridgeSample {
	return capture.captureBridgeDCSize(dc, position, BridgeWidth, BridgeHeight)
}

func (capture *captureSession) captureNativeMeterDC(dc uintptr, position BridgePosition, n *NativeMeter) BridgeSample {
	top, bottom := min(0, n.Offset), max(32, n.Offset+n.Height())
	p, valid := capture.capturePixels(dc, BridgePosition{X: position.X, Y: position.Y + top}, 256, bottom-top)
	if !valid {
		n.Error = "native_meter_capture_failed"
		return BridgeSample{Source: "augmentation", NativeMeter: n}
	}
	confirmed := DecodeSample(p.Cells(image.Rect(0, -top, 256, -top+32)))
	if confirmed.Error != "" || !nativeMeterMappingEqual(n, confirmed.NativeMeter) {
		n.Error = "native_meter_mapping_changed"
		return BridgeSample{Source: "augmentation", NativeMeter: n}
	}
	DecodeNativeMeter(confirmed.NativeMeter, p, n.Offset-top)
	return confirmed
}

func (capture *captureSession) captureAuraBridgeDC(dc uintptr, position BridgePosition, offset int) BridgeSample {
	if offset != 16 && offset != -8 {
		return BridgeSample{Error: "aura_capture_offset"}
	}
	top := min(0, offset)
	cells, ok := capture.captureRGBCells(dc, BridgePosition{position.X, position.Y + top}, 128, 24)
	if !ok {
		return BridgeSample{Error: "aura_capture_failed"}
	}
	base := -top / 2 * 64 * 3
	confirmed := DecodeSample(cells[base : base+BridgeBytes])
	// Metadata and native pixels must come from the same screen copy.
	if confirmed.Error != "" {
		return confirmed
	}
	if confirmed.Observations == nil || confirmed.Observations.Auras == nil || confirmed.Observations.Auras.Offset != offset {
		return BridgeSample{Error: "aura_mapping_changed"}
	}
	start := (offset - top) / 2 * 64 * 3
	DecodeAuraWatch(confirmed.Observations.Auras, cells[start:start+768])
	return confirmed
}

func (capture *captureSession) captureSimBridgeDC(dc uintptr, wide, narrow BridgePosition, wideFits bool) BridgeSample {
	result := BridgeSample{Error: "bridge_not_visible"}
	if wideFits {
		result = capture.captureBridgeDCSize(dc, wide, SimBridgeWidth, SimBridgeHeight)
		if result.Error == "" && (len(result.Sim) > 0 || len(result.Reload) > 0 || len(result.KBB) > 0) {
			return result
		}
	}
	legacy := capture.captureBridgeDC(dc, narrow)
	if legacy.Error == "" && len(legacy.Sim) > 0 {
		return legacy
	}
	return result
}

func (capture *captureSession) captureBridgeDCSize(dc uintptr, position BridgePosition, width, height int) BridgeSample {
	data, ok := capture.captureRGBCells(dc, position, width, height)
	if !ok {
		return BridgeSample{Error: "bridge_capture_failed"}
	}
	return DecodeSample(data)
}

func (capture *captureSession) captureRGBCells(dc uintptr, position BridgePosition, width, height int) ([]byte, bool) {
	p, ok := capture.capturePixels(dc, position, width, height)
	if !ok {
		return nil, false
	}
	return p.Cells(image.Rect(0, 0, width, height)), true
}

func (capture *captureSession) capturePixels(dc uintptr, position BridgePosition, width, height int) (Pixels, bool) {
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 || width*height > 65536 {
		return Pixels{}, false
	}
	area := image.Rect(position.X, position.Y, position.X+width, position.Y+height)
	if !capture.captureVisible(area) {
		return Pixels{}, false
	}
	key := image.Pt(width, height)
	if capture.surfaces == nil {
		capture.surfaces = map[image.Point]*captureSurface{}
	}
	surface := capture.surfaces[key]
	if surface == nil {
		// Shapes and native bands can change. Bound retained GDI resources per reader.
		limit := capture.surfaceLimit
		if limit == 0 {
			limit = 4
		}
		if len(capture.surfaces) >= limit {
			capture.Close()
			capture.surfaces = map[image.Point]*captureSurface{}
		}
		mem, _, _ := compatibleDC.Call(dc)
		if mem == 0 {
			return Pixels{}, false
		}
		bmp, _, _ := compatibleBitmap.Call(dc, uintptr(width), uintptr(height))
		if bmp == 0 {
			deleteDC.Call(mem)
			return Pixels{}, false
		}
		surface = &captureSurface{mem: mem, bmp: bmp, pixels: make([]byte, width*height*4)}
		capture.surfaces[key] = surface
	}
	mem, bmp := surface.mem, surface.bmp
	old, _, _ := selectObject.Call(mem, bmp)
	if old == 0 || old == ^uintptr(0) {
		capture.Close()
		return Pixels{}, false
	}
	ok, _, _ := bitBlt.Call(mem, 0, 0, uintptr(width), uintptr(height), dc, uintptr(position.X), uintptr(position.Y), 0x00CC0020)
	selectObject.Call(mem, old) // GetDIBits requires the bitmap to be unselected.
	if ok == 0 {
		return Pixels{}, false
	}
	info := struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, SizeImage uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, Bits: 32}
	pixels := surface.pixels
	rows, _, _ := dibits.Call(mem, bmp, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), 0)
	if rows != uintptr(height) {
		capture.Close()
		return Pixels{}, false
	}
	if !capture.captureVisible(area) {
		return Pixels{}, false
	}
	return Pixels{Width: width, Height: height, BGRA: pixels}, true
}

// Capture is bounded to the addon strip and runs off the log-ingestion loop.
// Never saves screenshots or examines game memory. Unfocused WoW remains
// readable when the requested strip is visible on the desktop.
func ReadBridge(ctx context.Context, position BridgePosition, retail string, clients ...string) <-chan BridgeSample {
	return readBridge(ctx, position, retail, false, clients...)
}

// ReadSimBridge checks the larger v2 sim strip, then a legacy v1 strip at the
// same preferred origin. It never changes the CD/Kick capture path.
func ReadSimBridge(ctx context.Context, position BridgePosition, retail string, clients ...string) <-chan BridgeSample {
	return readBridge(ctx, position, retail, true, clients...)
}

func ReadOverlayBridge(ctx context.Context, position BridgePosition, width int, retail string, clients ...string) <-chan BridgeSample {
	return readBridgeLayout(ctx, position, retail, true, width, clients...)
}
func readBridge(ctx context.Context, position BridgePosition, retail string, sim bool, clients ...string) <-chan BridgeSample {
	return readBridgeLayout(ctx, position, retail, sim, 0, clients...)
}
func readBridgeLayout(ctx context.Context, position BridgePosition, retail string, sim bool, overlayWidth int, clients ...string) <-chan BridgeSample {
	return readBridgeLayoutTimed(ctx, position, retail, sim, overlayWidth, 0, clients...)
}
func readTargetBridge(ctx context.Context, target BridgeTarget, game, client string) <-chan BridgeSample {
	if target.Source == "augmentation" {
		return readMeterBorderBridge(ctx, target, game, client)
	}
	width := 0
	if target.Source == "overlays" {
		width = target.OverlayWidth
	}
	return readBridgeLayoutTimed(ctx, target.Position, game, target.Wide, width, target.IdleInterval, client)
}
func readBridgeLayoutTimed(ctx context.Context, position BridgePosition, retail string, sim bool, overlayWidth int, idle time.Duration, clients ...string) <-chan BridgeSample {
	ch := make(chan BridgeSample, 1)
	go func() {
		defer close(ch)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if setDPI.Find() == nil {
			old, _, _ := setDPI.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4).
			if old != 0 {
				defer setDPI.Call(old)
			}
		}
		var capture captureSession
		defer capture.Close()
		cadence := captureCadence{idle: idle}
		interval := activeCaptureInterval
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				value := capture.captureBridgeLayout(position, retail, sim, overlayWidth, clients...)
				if next := cadence.next(value.Error == ""); next != interval {
					interval = next
					ticker.Reset(interval)
				}
				value.Position = position
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

// Each reader owns its session on one locked OS thread. Pixel buffers are
// decoded before reuse; decoded samples never retain a BGRA buffer.
type captureSurface struct {
	mem, bmp uintptr
	pixels   []byte
}
type captureSession struct {
	surfaceLimit              int
	surfaces                  map[image.Point]*captureSurface
	game, target              uintptr
	searched                  time.Time
	searchError, captureError string
	targetBounds              image.Rectangle
}

func (capture *captureSession) Close() {
	for _, surface := range capture.surfaces {
		deleteObject.Call(surface.bmp)
		deleteDC.Call(surface.mem)
	}
	capture.surfaces = nil
}

func captureBridgeLayout(position BridgePosition, retail string, sim bool, overlayWidth int, clients ...string) BridgeSample {
	var capture captureSession
	defer capture.Close()
	return capture.captureBridgeLayout(position, retail, sim, overlayWidth, clients...)
}

func captureScreenBridgeDC(dc uintptr, position BridgePosition, offset, auraOffset int, heights ...int) BridgeSample {
	var capture captureSession
	defer capture.Close()
	return capture.captureScreenBridgeDC(dc, position, offset, auraOffset, heights...)
}

func captureBridgeDC(dc uintptr, position BridgePosition) BridgeSample {
	var capture captureSession
	defer capture.Close()
	return capture.captureBridgeDC(dc, position)
}

func captureAuraBridgeDC(dc uintptr, position BridgePosition, offset int) BridgeSample {
	var capture captureSession
	defer capture.Close()
	return capture.captureAuraBridgeDC(dc, position, offset)
}

func captureSimBridgeDC(dc uintptr, wide, narrow BridgePosition, wideFits bool) BridgeSample {
	var capture captureSession
	defer capture.Close()
	return capture.captureSimBridgeDC(dc, wide, narrow, wideFits)
}

func captureBridgeDCSize(dc uintptr, position BridgePosition, width, height int) BridgeSample {
	var capture captureSession
	defer capture.Close()
	return capture.captureBridgeDCSize(dc, position, width, height)
}

func captureRGBCells(dc uintptr, position BridgePosition, width, height int) ([]byte, bool) {
	var capture captureSession
	defer capture.Close()
	return capture.captureRGBCells(dc, position, width, height)
}

func capturePixels(dc uintptr, position BridgePosition, width, height int) (Pixels, bool) {
	var capture captureSession
	defer capture.Close()
	return capture.capturePixels(dc, position, width, height)
}
