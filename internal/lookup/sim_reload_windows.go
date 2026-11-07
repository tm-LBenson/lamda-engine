//go:build windows

package lookup

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"time"
	"unsafe"
)

type reloadKeyboardInput struct {
	Key, Scan   uint16
	Flags, Time uint32
	Extra       uintptr
}
type reloadInput struct {
	Type     uint32
	Keyboard reloadKeyboardInput
	Padding  [8]byte // INPUT union is MOUSEINPUT-sized (32/24 bytes on64/32-bit).
}

func nativeSimKeys(events []simKeyEvent) (int, error) {
	// The only caller creates the fixed reload chord or its release-only cleanup.
	if !slices.Equal(events, simReloadSequence) && !slices.Equal(events, simReloadCleanup) {
		return 0, fmt.Errorf("invalid reload key sequence")
	}
	inputs := make([]reloadInput, len(events))
	for i, event := range events {
		inputs[i].Type = 1
		inputs[i].Keyboard.Key = event.Key
		if event.Up {
			inputs[i].Keyboard.Flags = 2
		}
	}
	result, _, err := user32.NewProc("SendInput").Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if int(result) != len(inputs) {
		return int(result), err
	}
	return int(result), nil
}

// SendSimReload never activates a window and never sends arbitrary input. Its
// final capture must still show the exact temporary opt-in lease from this WoW.
func SendSimReload(ctx context.Context, window uintptr, wow, client string, expectedBody []byte, positions ...BridgePosition) error {
	gameInputMu.Lock()
	defer gameInputMu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if setDPI.Find() == nil {
		old, _, _ := setDPI.Call(^uintptr(3))
		if old != 0 {
			defer setDPI.Call(old)
		}
	}
	position := BridgePosition{X: 0, Y: 48}
	if len(positions) > 0 {
		position = positions[0]
	}
	platform := simReloadPlatform{
		now:         time.Now,
		foreground:  func() uintptr { value, _, _ := foreground.Call(); return value },
		executable:  windowExecutable,
		matchesGame: func(exe, path, client string) bool { return gameExecutable(exe, path, client) },
		capture:     func() BridgeSample { return captureBridgeMode(position, wow, true, client) },
		keyDown: func(key uint16) bool {
			value, _, _ := user32.NewProc("GetAsyncKeyState").Call(uintptr(key))
			return value&0x8000 != 0
		},
		send: nativeSimKeys,
	}
	return sendSimReload(ctx, window, wow, client, expectedBody, platform)
}
