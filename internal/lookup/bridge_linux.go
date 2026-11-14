//go:build linux

package lookup

import (
	"bufio"
	"context"
	"encoding/json"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// A separate Python/Xlib connection belongs to each reader. Pixel decoding and
// all freshness, checksum, roster and client gates stay in the shared Go core.
type linuxCapture struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     *bufio.Reader
	context linuxWindow
	error   string
}

type linuxWindow struct {
	Window     uintptr `json:"window"`
	PID        uint32  `json:"pid"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Foreground bool    `json:"foreground"`
	Pixels     []byte  `json:"pixels,omitempty"`
	Error      string  `json:"error,omitempty"`
}

func newLinuxCapture(ctx context.Context, game, client string) (*linuxCapture, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	python := os.Getenv("LAMDA_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmd := exec.CommandContext(ctx, python, filepath.Join(filepath.Dir(executable), "linux", "capture.py"), "--wow", game, "--client", client)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		in.Close()
		return nil, err
	}
	return &linuxCapture{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 512*1024)}, nil
}

func (c *linuxCapture) Close() {
	c.in.Close()
	_ = c.cmd.Process.Kill()
	_ = c.cmd.Wait()
}

func (c *linuxCapture) request(request any) linuxWindow {
	// A blocked X server must expire capture, rather than retain old evidence.
	deadline := time.AfterFunc(2*time.Second, func() { _ = c.cmd.Process.Kill() })
	defer deadline.Stop()
	if err := json.NewEncoder(c.in).Encode(request); err != nil {
		return linuxWindow{Error: "bridge_linux_helper_failed"}
	}
	line, err := c.out.ReadBytes('\n')
	if err != nil || len(line) > 512*1024 {
		return linuxWindow{Error: "bridge_linux_helper_failed"}
	}
	var result linuxWindow
	if json.Unmarshal(line, &result) != nil {
		result.Error = "bridge_linux_helper_failed"
	}
	return result
}

func (c *linuxCapture) capturePixels(_ uintptr, p BridgePosition, width, height int) (Pixels, bool) {
	if width < 1 || height < 1 || width*height > 65536 || p.X < 0 || p.Y < 0 || p.X+width > c.context.Width || p.Y+height > c.context.Height {
		c.error = "bridge_capture_bounds"
		return Pixels{}, false
	}
	r := c.request(map[string]any{"op": "capture", "expected": c.context, "x": p.X, "y": p.Y, "width": width, "height": height})
	c.error = r.Error
	if r.Error != "" || len(r.Pixels) != width*height*4 {
		return Pixels{}, false
	}
	return Pixels{Width: width, Height: height, BGRA: r.Pixels}, true
}

func (c *linuxCapture) captureBridgeLayout(position BridgePosition, wide bool, overlayWidth int) BridgeSample {
	c.context = c.request(map[string]string{"op": "context"})
	c.error = c.context.Error
	if c.error != "" {
		return BridgeSample{Error: c.error}
	}
	w, h := c.context.Width, c.context.Height
	p, fits := position.Clamp(w, h)
	if !fits {
		return BridgeSample{Error: "bridge_window_too_small"}
	}
	var sample BridgeSample
	if overlayWidth != 0 {
		if !ValidOverlayWidth(overlayWidth) {
			return BridgeSample{Error: "overlays_shape_invalid"}
		}
		p, fits = position.clampSize(w, h, overlayWidth, 18432/overlayWidth)
		if !fits {
			return BridgeSample{Error: "bridge_window_too_small"}
		}
		pixels, ok := c.capturePixels(0, p, overlayWidth, 18432/overlayWidth)
		if !ok {
			return BridgeSample{Error: "overlays_capture_failed"}
		}
		sample = decodeLinuxOverlayShape(pixels)
	} else if wide {
		wp, wf := position.clampSize(w, h, SimBridgeWidth, SimBridgeHeight)
		sample = c.captureSimBridgeDC(0, wp, p, wf)
		p = wp
	} else {
		sample = c.captureBridgeDC(0, p)
		if sample.Error == "" && sample.Observations != nil {
			auraOffset := 0
			if a := sample.Observations.Auras; a != nil {
				auraOffset = a.Offset
			}
			if s := sample.Observations.Screen; s != nil {
				sample = c.captureScreenBridgeDC(0, p, s.Offset, auraOffset, s.Height())
			} else if auraOffset != 0 {
				sample = c.captureAuraBridgeDC(0, p, auraOffset)
			}
		}
	}
	if sample.Error == "" && sample.NativeMeter != nil {
		sample = c.captureNativeMeterDC(0, p, sample.NativeMeter)
	}
	if sample.Error == "" && sample.Overlays != nil {
		a := sample.Overlays
		if a.Width != w || a.Height != h {
			return BridgeSample{Error: "overlays_window_size_changed"}
		}
		if overlayWidth == 0 {
			top, bottom := min(0, a.Offset), max(32, a.Offset+40)
			pixels, ok := c.capturePixels(0, BridgePosition{p.X, p.Y + top}, 256, bottom-top)
			if !ok {
				return BridgeSample{Error: "overlays_capture_failed"}
			}
			sample = DecodeOverlayPixels(pixels, -top, a.Offset-top)
		}
	}
	if sample.Error == "" && ((sample.Frames != nil && (sample.Frames.Width != w || sample.Frames.Height != h)) || (sample.Kick != nil && (sample.Kick.Width != w || sample.Kick.Height != h)) || (sample.Arena != nil && (sample.Arena.Width != w || sample.Arena.Height != h))) {
		return BridgeSample{Error: "bridge_window_size_changed"}
	}
	if sample.Error != "" && c.error != "" {
		sample.Error = c.error
	}
	sample.Window, sample.WindowPID = c.context.Window, c.context.PID
	return sample
}

func ReadBridge(ctx context.Context, position BridgePosition, game string, clients ...string) <-chan BridgeSample {
	return linuxRead(ctx, BridgeTarget{Position: position}, game, linuxClient(clients))
}
func ReadSimBridge(ctx context.Context, position BridgePosition, game string, clients ...string) <-chan BridgeSample {
	return linuxRead(ctx, BridgeTarget{Position: position, Wide: true}, game, linuxClient(clients))
}
func ReadOverlayBridge(ctx context.Context, position BridgePosition, width int, game string, clients ...string) <-chan BridgeSample {
	return linuxRead(ctx, BridgeTarget{Position: position, Wide: true, OverlayWidth: width}, game, linuxClient(clients))
}
func linuxClient(clients []string) string {
	if len(clients) > 0 {
		return clients[0]
	}
	return "retail"
}
func readTargetBridge(ctx context.Context, target BridgeTarget, game, client string) <-chan BridgeSample {
	return linuxRead(ctx, target, game, client)
}

func linuxRead(ctx context.Context, target BridgeTarget, game, client string) <-chan BridgeSample {
	out := make(chan BridgeSample, 1)
	go func() {
		defer close(out)
		c, err := newLinuxCapture(ctx, game, client)
		if err != nil {
			out <- BridgeSample{Error: "bridge_linux_helper_unavailable"}
			return
		}
		defer c.Close()
		cadence := captureCadence{idle: target.IdleInterval}
		timer := time.NewTimer(activeCaptureInterval)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			var value BridgeSample
			found := false
			if target.Source == "augmentation" {
				value, found = c.captureMeterBorderSlot(target.LocatorBase, target.MeterSlot, game, client)
			}
			if !found {
				value = c.captureBridgeLayout(target.Position, target.Wide, target.OverlayWidth)
				value.Position = target.Position
			}
			value.LocatorBase = target.LocatorBase
			select {
			case out <- value:
			default:
				select {
				case <-out:
				default:
				}
				select {
				case out <- value:
				default:
				}
			}
			timer.Reset(cadence.next(value.Error == ""))
		}
	}()
	return out
}

type WindowTracker struct {
	mu           sync.Mutex
	at           time.Time
	game, client string
	value        linuxWindow
}

func (t *WindowTracker) Context(game, client string) (WindowContext, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.at) > 500*time.Millisecond || game != t.game || client != t.client {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c, err := newLinuxCapture(ctx, game, client)
		t.value = linuxWindow{Error: "bridge_linux_helper_unavailable"}
		if err == nil {
			t.value = c.request(map[string]string{"op": "context"})
			c.Close()
		}
		t.at, t.game, t.client = time.Now(), game, client
	}
	v := t.value
	return WindowContext{Window: v.Window, PID: v.PID, Width: v.Width, Height: v.Height, Foreground: v.Foreground}, v.Error == "" && v.Window != 0
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

func (capture *linuxCapture) captureScreenBridgeDC(dc uintptr, position BridgePosition, offset, auraOffset int, heights ...int) BridgeSample {
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

func (capture *linuxCapture) captureBridgeDC(dc uintptr, position BridgePosition) BridgeSample {
	return capture.captureBridgeDCSize(dc, position, BridgeWidth, BridgeHeight)
}

func (capture *linuxCapture) captureNativeMeterDC(dc uintptr, position BridgePosition, n *NativeMeter) BridgeSample {
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

func (capture *linuxCapture) captureAuraBridgeDC(dc uintptr, position BridgePosition, offset int) BridgeSample {
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

func (capture *linuxCapture) captureSimBridgeDC(dc uintptr, wide, narrow BridgePosition, wideFits bool) BridgeSample {
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

func (capture *linuxCapture) captureBridgeDCSize(dc uintptr, position BridgePosition, width, height int) BridgeSample {
	data, ok := capture.captureRGBCells(dc, position, width, height)
	if !ok {
		return BridgeSample{Error: "bridge_capture_failed"}
	}
	return DecodeSample(data)
}

func (capture *linuxCapture) captureRGBCells(dc uintptr, position BridgePosition, width, height int) ([]byte, bool) {
	p, ok := capture.capturePixels(dc, position, width, height)
	if !ok {
		return nil, false
	}
	return p.Cells(image.Rect(0, 0, width, height)), true
}
