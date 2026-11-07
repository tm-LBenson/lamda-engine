//go:build windows

package lookup

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Opt-in desktop integration. Run this test binary as Wow.exe in a private test
// directory, never the installed game directory. Only small nonactivating test
// windows are shown; no input, focus changes, or real game access is performed.
func TestVisibleBackgroundCapture(t *testing.T) {
	if os.Getenv("LAMDA_VISIBLE_CAPTURE_TEST") != "1" {
		t.Skip("visible fixture not requested")
	}
	exe, err := os.Executable()
	if err != nil || !strings.EqualFold(filepath.Base(exe), "Wow.exe") {
		t.Fatal("run a private copy of the test executable named Wow.exe", err)
	}
	game := filepath.Dir(exe)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previousDPI, _, _ := setDPI.Call(^uintptr(3))
	defer setDPI.Call(previousDPI)
	front, _, _ := foreground.Call()
	defer func() {
		if after, _, _ := foreground.Call(); after != front {
			t.Error("foreground changed during the nonactivating fixture")
		}
	}()
	create := user32.NewProc("CreateWindowExW")
	destroy := user32.NewProc("DestroyWindow")
	position := user32.NewProc("SetWindowPos")
	show := user32.NewProc("ShowWindow")
	update := user32.NewProc("UpdateWindow")
	flush := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmFlush")
	class, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("Lamda capture test")
	newWindow := func(width, height int) uintptr {
		w, _, e := create.Call(0x08000080, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x80000004, 0, 0, uintptr(width), uintptr(height), 0, 0, 0, 0)
		if w == 0 {
			t.Fatal("fixture window", e)
		}
		t.Cleanup(func() { destroy.Call(w) })
		return w
	}
	screenWidth, _, _ := getSystemMetrics.Call(0)
	x, y := int(screenWidth)-450, 120
	hwnd := newWindow(400, 220)
	move := func(w uintptr, left, top, width, height int) {
		ok, _, e := position.Call(w, ^uintptr(0), uintptr(left), uintptr(top), uintptr(width), uintptr(height), 0x50) // TOPMOST, NOACTIVATE, SHOWWINDOW
		if ok == 0 {
			t.Fatal("position fixture", e)
		}
		update.Call(w)
		flush.Call()
	}
	move(hwnd, x, y, 400, 220)
	if front == hwnd {
		t.Fatal("fixture must be in the background")
	}
	base, other := BridgePosition{20, 20}, BridgePosition{20, 100}
	body := fmt.Sprintf("boot:1|%d|roster|0\nPlayer-1-BOB|us|Draenor|Bob\n\n\n\nT|3|123:1|24|0,1|0", time.Now().Unix())
	packet := simPacketFixture("LUI3", BridgeBytes, []byte(body))
	if sample := DecodeSample(packet); sample.Error != "" || sample.Observations == nil || sample.Observations.Screen == nil {
		t.Fatal("invalid fixture packet", sample)
	}
	native, _ := meterFixture()
	paint := func() {
		dc, _, _ := getDC.Call(hwnd)
		defer releaseDC.Call(hwnd, dc)
		pixel := gdi32.NewProc("SetPixelV")
		for _, p := range []BridgePosition{base, other} {
			for cell := 0; cell < len(packet)/3; cell++ {
				value := uintptr(packet[cell*3]) | uintptr(packet[cell*3+1])<<8 | uintptr(packet[cell*3+2])<<16
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						pixel.Call(dc, uintptr(p.X+cell%64*2+dx), uintptr(p.Y+cell/64*2+dy), value)
					}
				}
			}
			for yy := 0; yy < native.Height; yy++ {
				for xx := 0; xx < native.Width; xx++ {
					r, g, b := native.rgb(xx, yy)
					pixel.Call(dc, uintptr(p.X+xx), uintptr(p.Y+24+yy), uintptr(r)|uintptr(g)<<8|uintptr(b)<<16)
				}
			}
		}
		gdi32.NewProc("GdiFlush").Call()
		flush.Call()
	}
	paint()
	var capture captureSession
	defer capture.Close()
	read := func(p BridgePosition) BridgeSample { return capture.captureBridgeLayout(p, game, false, 0, "retail") }
	valid := func(p BridgePosition) {
		t.Helper()
		sample := read(p)
		if sample.Error != "" || sample.Window != hwnd || sample.Observations == nil || sample.Observations.Screen == nil {
			t.Fatalf("background read failed: %+v", sample)
		}
		s := sample.Observations.Screen
		if !s.Captured || s.Error != "" || len(s.Meter) != 1 || s.Meter[0].Amount != 7 || len(s.Casts) != 4 {
			t.Fatalf("background native observations: %+v", s)
		}
	}
	valid(base) // Discovery works even if this process never had focus.
	valid(other)
	t.Log("background discovery and native kick readings passed")
	cover := newWindow(64, 60)
	move(cover, x+30, y+20, 64, 60)
	if sample := read(base); sample.Error != "bridge_strip_occluded" {
		t.Fatalf("covered strip accepted: %+v", sample)
	}
	valid(other)
	t.Log("covered strip pauses independently")
	// Only the native band is covered. Preserve base metadata so the renderer
	// can reserve a capture hole on its next frame, without accepting values.
	move(cover, x+30, y+44, 64, 16)
	paint()
	partial := read(base)
	if partial.Error != "" || partial.Observations == nil || partial.Observations.Screen == nil || partial.Observations.Screen.Error == "" || partial.Observations.Screen.Captured || len(partial.Observations.Screen.Meter) != 0 {
		t.Fatalf("covered native band accepted or mapping lost: %+v", partial)
	}
	t.Log("covered native band rejected while preserving its mapping")
	// A native region hole is how the WPF overlay leaves strips unobstructed.
	move(cover, x, y, 400, 220)
	holes := func(areas ...image.Rectangle) {
		region, _, _ := createRectRgn.Call(0, 0, 400, 220)
		for _, area := range areas {
			cut, _, _ := createRectRgn.Call(uintptr(area.Min.X), uintptr(area.Min.Y), uintptr(area.Max.X), uintptr(area.Max.Y))
			gdi32.NewProc("CombineRgn").Call(region, region, cut, 4)
			deleteObject.Call(cut)
		}
		if ok, _, _ := user32.NewProc("SetWindowRgn").Call(cover, region, 1); ok == 0 {
			deleteObject.Call(region)
			t.Fatal("fixture region")
		}
		paint()
	}
	holes(image.Rect(20, 20, 148, 60), image.Rect(20, 100, 148, 140))
	valid(base)
	valid(other)
	// Separate holes without their eight-pixel gap cannot support one atomic
	// packet/native read. This catches accidental incomplete reservations.
	holes(image.Rect(20, 20, 148, 36), image.Rect(20, 44, 148, 60))
	partial = read(base)
	if partial.Error != "" || partial.Observations.Screen.Error == "" || partial.Observations.Screen.Captured {
		t.Fatal("incomplete capture hole accepted", partial)
	}
	holes(image.Rect(20, 20, 148, 60), image.Rect(20, 100, 148, 140))
	valid(base)
	t.Log("transparent overlay region and whole-snapshot holes passed")
	// Reset its region then move it outside both strips.
	user32.NewProc("SetWindowRgn").Call(cover, 0, 1)
	move(cover, x+250, y+100, 64, 60)
	paint()
	valid(base)
	t.Log("uncovered strips resume without refocusing")
	x -= 40
	move(hwnd, x, y, 400, 220)
	paint()
	valid(base)
	if visibleCaptureRect(hwnd, image.Rect(-100000, -100000, -99999, -99999)) {
		t.Fatal("off-screen strip accepted")
	}
	show.Call(hwnd, 7) // SW_SHOWMINNOACTIVE
	if sample := read(base); sample.Error != "wow_not_visible" {
		t.Fatal("minimized window accepted", sample.Error)
	}
	show.Call(hwnd, 4) // SW_SHOWNOACTIVATE
	move(hwnd, x, y, 400, 220)
	paint()
	valid(base)
	var wrong captureSession
	defer wrong.Close()
	if sample := wrong.captureBridgeLayout(base, filepath.Join(game, "other-client"), false, 0, "retail"); sample.Error != "wow_not_found" {
		t.Fatal("wrong installation accepted", sample.Error)
	}
	show.Call(hwnd, 0)
	if sample := read(base); sample.Error != "wow_not_visible" {
		t.Fatal("hidden window accepted", sample.Error)
	}
	destroy.Call(hwnd)
	capture.searched = time.Time{}
	if sample := read(base); sample.Error != "wow_not_found" {
		t.Fatal("closed window accepted", sample.Error)
	}
	t.Log("move, minimize/restore, hide, close, and executable identity passed")
}
