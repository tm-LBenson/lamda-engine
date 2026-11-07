//go:build windows

package lookup

import (
	"testing"
	"unsafe"
)

func TestWindowsReloadInputLayoutAndInvalidSequence(t *testing.T) {
	var input reloadInput
	expected := uintptr(28)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		expected = 40
	}
	if unsafe.Sizeof(input) != expected {
		t.Fatalf("INPUT size=%d want%d", unsafe.Sizeof(input), expected)
	}
	offset := uintptr(4)
	if expected == 40 {
		offset = 8
	}
	if unsafe.Offsetof(input.Keyboard) != offset {
		t.Fatal("keyboard union offset mismatch")
	}
	// Invalid inputs fail before any OS call. Valid input is only tested using
	// injected wrappers; this suite never sends keys to any application.
	for _, events := range [][]simKeyEvent{nil, {{Key: 65}}, {{Key: simReloadKey}}, {{Key: 65}, {Key: 65, Up: true}}, append([]simKeyEvent(nil), simReloadSequence[:5]...)} {
		if n, err := nativeSimKeys(events); err == nil || n != 0 {
			t.Fatal("arbitrary key sequence accepted")
		}
	}
}
