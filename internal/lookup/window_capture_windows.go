//go:build windows

package lookup

import (
	"image"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var enumWindows = user32.NewProc("EnumWindows")
var isWindowVisible = user32.NewProc("IsWindowVisible")
var isIconic = user32.NewProc("IsIconic")
var getWindow = user32.NewProc("GetWindow")
var getWindowRect = user32.NewProc("GetWindowRect")
var getWindowRgn = user32.NewProc("GetWindowRgn")
var getSystemMetrics = user32.NewProc("GetSystemMetrics")
var createRectRgn = gdi32.NewProc("CreateRectRgn")
var rectInRegion = gdi32.NewProc("RectInRegion")
var dwmAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmGetWindowAttribute")

type nativeRect struct{ Left, Top, Right, Bottom int32 }

type gameWindowSearch struct {
	game    string
	clients []string
	found   uintptr
}

var gameWindowSearches sync.Map
var gameWindowSearchID atomic.Uintptr

// Windows callback thunks cannot be freed. Allocate one for all synchronous
// searches, not a fresh callback every time WoW is absent.
var gameWindowCallback = syscall.NewCallback(func(hwnd, data uintptr) uintptr {
	value, ok := gameWindowSearches.Load(data)
	if !ok {
		return 0
	}
	search := value.(*gameWindowSearch)
	if !windowVisible(hwnd) {
		return 1
	}
	if bounds, ok := windowClientBounds(hwnd); !ok || bounds.Dx() < BridgeWidth || bounds.Dy() < BridgeHeight {
		return 1
	}
	if gameExecutable(windowExecutable(hwnd), search.game, search.clients...) {
		search.found = hwnd
		return 0
	}
	return 1
})

func (r nativeRect) rectangle() image.Rectangle {
	return image.Rect(int(r.Left), int(r.Top), int(r.Right), int(r.Bottom))
}

func windowVisible(hwnd uintptr) bool {
	visible, _, _ := isWindowVisible.Call(hwnd)
	minimized, _, _ := isIconic.Call(hwnd)
	if visible == 0 || minimized != 0 {
		return false
	}
	var cloaked uint32
	if dwmAttribute.Find() == nil {
		dwmAttribute.Call(hwnd, 14, uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	}
	return cloaked == 0
}

func windowClientBounds(hwnd uintptr) (image.Rectangle, bool) {
	var rect nativeRect
	var point struct{ X, Y int32 }
	ok, _, _ := clientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return image.Rectangle{}, false
	}
	ok, _, _ = clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&point)))
	return rect.rectangle().Add(image.Pt(int(point.X), int(point.Y))), ok != 0
}

// Retain the verified game HWND when another application receives focus. Only
// enumerate when it disappears; six strip readers must not repeatedly scan all
// process metadata while WoW is closed. No activation or input is performed.
func (c *captureSession) gameWindow(game string, clients ...string) (uintptr, string) {
	front, _, _ := foreground.Call()
	if front != 0 && gameExecutable(windowExecutable(front), game, clients...) {
		c.game = front
	}
	if c.game != 0 && gameExecutable(windowExecutable(c.game), game, clients...) {
		if !windowVisible(c.game) {
			return 0, "wow_not_visible"
		}
		return c.game, ""
	}
	c.game = 0
	if time.Since(c.searched) < time.Second {
		return 0, c.searchError
	}
	c.searched = time.Now()
	c.searchError = "wow_not_found"
	search := gameWindowSearch{game: game, clients: clients}
	// Pass an opaque ID through Win32, not a Go pointer kept across callbacks.
	// This also keeps simultaneous readers' searches isolated.
	id := gameWindowSearchID.Add(1)
	gameWindowSearches.Store(id, &search)
	enumWindows.Call(gameWindowCallback, id)
	gameWindowSearches.Delete(id)
	c.game = search.found
	if c.game != 0 {
		return c.game, ""
	}
	return 0, c.searchError
}

// Examine window geometry, not other applications' pixels. An opaque window
// above any part of the requested strip prevents that read. Native window
// regions include holes in LamdaUI's transparent overlay.
func visibleCaptureRect(hwnd uintptr, area image.Rectangle) bool {
	if area.Empty() || !windowVisible(hwnd) {
		return false
	}
	x, _, _ := getSystemMetrics.Call(76) // SM_XVIRTUALSCREEN, including negative monitor positions.
	y, _, _ := getSystemMetrics.Call(77)
	w, _, _ := getSystemMetrics.Call(78)
	h, _, _ := getSystemMetrics.Call(79)
	desktop := image.Rect(int(int32(x)), int(int32(y)), int(int32(x))+int(w), int(int32(y))+int(h))
	if !area.In(desktop) {
		return false
	}
	region, _, _ := createRectRgn.Call(0, 0, 0, 0)
	if region == 0 {
		return false
	}
	defer deleteObject.Call(region)
	seen := map[uintptr]bool{}
	for current, count := hwnd, 0; count < 512; count++ {
		above, _, _ := getWindow.Call(current, 3) // GW_HWNDPREV: walk toward the top of Z order.
		if above == 0 {
			return true
		}
		if seen[above] {
			return false
		}
		seen[above], current = true, above
		if !windowVisible(above) {
			continue
		}
		var rect nativeRect
		ok, _, _ := getWindowRect.Call(above, uintptr(unsafe.Pointer(&rect)))
		if ok == 0 {
			return false
		}
		intersection := area.Intersect(rect.rectangle())
		if intersection.Empty() {
			continue
		}
		kind, _, _ := getWindowRgn.Call(above, region)
		if kind == 0 {
			return false
		} // Ordinary rectangular window (or an unavailable region).
		if kind == 1 {
			continue
		} // NULLREGION draws nothing.
		local := intersection.Sub(rect.rectangle().Min)
		part := nativeRect{int32(local.Min.X), int32(local.Min.Y), int32(local.Max.X), int32(local.Max.Y)}
		overlaps, _, _ := rectInRegion.Call(region, uintptr(unsafe.Pointer(&part)))
		if overlaps != 0 {
			return false
		}
	}
	return false
}

func (c *captureSession) captureVisible(area image.Rectangle) bool {
	if c.target == 0 {
		return true
	} // Private bitmap tests have no desktop target.
	if !area.In(c.targetBounds) || !visibleCaptureRect(c.target, area) {
		c.captureError = "bridge_strip_occluded"
		return false
	}
	return true
}
