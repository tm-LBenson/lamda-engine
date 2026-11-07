//go:build windows

package addonstore

import (
	"fmt"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var createMutex = kernel32.NewProc("CreateMutexW")
var waitForSingleObject = kernel32.NewProc("WaitForSingleObject")
var releaseMutex = kernel32.NewProc("ReleaseMutex")
var closeHandle = kernel32.NewProc("CloseHandle")

func acquireControl() (func(), error) {
	name, _ := syscall.UTF16PtrFromString(`Local\LamdaUI.EngineControl`)
	handle, _, callErr := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, fmt.Errorf("open LamdaUI update lock: %v", callErr)
	}
	result, _, _ := waitForSingleObject.Call(handle, 0)
	if result != 0 && result != 0x80 {
		closeHandle.Call(handle)
		return nil, fmt.Errorf("LamdaUI is updating; try again when it finishes")
	}
	return func() {
		releaseMutex.Call(handle)
		closeHandle.Call(handle)
	}, nil
}
