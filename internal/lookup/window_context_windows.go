//go:build windows

package lookup

import "unsafe"

type WindowTracker struct{ session captureSession }

func (w *WindowTracker) Context(game, client string) (WindowContext, bool) {
	hwnd, problem := w.session.gameWindow(game, client)
	if problem != "" || hwnd == 0 {
		return WindowContext{}, false
	}
	bounds, ok := windowClientBounds(hwnd)
	if !ok || !windowVisible(hwnd) {
		return WindowContext{}, false
	}
	var pid uint32
	windowProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	front, _, _ := foreground.Call()
	return WindowContext{Window: hwnd, PID: pid, Width: bounds.Dx(), Height: bounds.Dy(), Foreground: front == hwnd}, true
}
