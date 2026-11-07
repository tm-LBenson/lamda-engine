//go:build windows

package lookup

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// Native off-screen font rendering only: never locates or controls WoW.
func TestWindowsCompactMeterDataFont(t *testing.T) {
	path := os.Getenv("LAMDA_COMPACT_METER_FONT")
	if path == "" {
		t.Skip("compact font fixture not requested")
	}
	file, _ := syscall.UTF16PtrFromString(path)
	loaded, _, _ := gdi32.NewProc("AddFontResourceExW").Call(uintptr(unsafe.Pointer(file)), 0x10, 0)
	if loaded == 0 {
		t.Fatal("font load")
	}
	defer gdi32.NewProc("RemoveFontResourceExW").Call(uintptr(unsafe.Pointer(file)), 0x10, 0)
	for _, quality := range []uintptr{3, 4} {
		t.Run(fmt.Sprintf("quality%d", quality), func(t *testing.T) {
			dc, _, _ := compatibleDC.Call(0)
			defer deleteDC.Call(dc)
			info := struct {
				Size                   uint32
				Width, Height          int32
				Planes, Bits           uint16
				Compression, SizeImage uint32
				X, Y                   int32
				Used, Important        uint32
			}{Size: 40, Width: 256, Height: -64, Planes: 1, Bits: 32}
			var bits uintptr
			bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
			if bmp == 0 {
				t.Fatal("bitmap")
			}
			defer deleteObject.Call(bmp)
			old, _, _ := selectObject.Call(dc, bmp)
			defer selectObject.Call(dc, old)
			face, _ := syscall.UTF16PtrFromString("Lamda Meter Data")
			font, _, _ := gdi32.NewProc("CreateFontW").Call(^uintptr(3), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, quality, 0, uintptr(unsafe.Pointer(face)))
			if font == 0 {
				t.Fatal("font")
			}
			defer deleteObject.Call(font)
			previous, _, _ := selectObject.Call(dc, font)
			defer selectObject.Call(dc, previous)
			gdi32.NewProc("SetTextColor").Call(dc, 0xffffff)
			gdi32.NewProc("SetBkColor").Call(dc, 0)
			// Match the extra 1px glyph padding measured in WoW's FontString.
			gdi32.NewProc("SetTextCharacterExtra").Call(dc, 1)
			text := func(x, y int, s string) {
				v, _ := syscall.UTF16FromString(s)
				gdi32.NewProc("TextOutW").Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)-1))
			}
			calText := "0123456789abcdefX"
			v, _ := syscall.UTF16FromString(calText)
			var size struct{ X, Y int32 }
			gdi32.NewProc("GetTextExtentPoint32W").Call(dc, uintptr(unsafe.Pointer(&v[0])), 17, uintptr(unsafe.Pointer(&size)))
			pitch := float64(size.X) / 17
			if pitch != 6 {
				t.Fatalf("compact pitch=%v expected 6", pitch)
			}
			text(8, 2, calText)
			values := []int64{0, 16777215, 16777216, 4294967296, 999999999999999}
			for i, n := range values {
				text(8, 18+i*8, fmt.Sprintf("%015dX", n))
				text(128, 18+i*8, fmt.Sprintf("%015dX", n))
			}
			capture := &captureSession{}
			defer capture.Close()
			pixels, ok := capture.capturePixels(dc, BridgePosition{}, 256, 64)
			if !ok {
				t.Fatal("capture")
			}
			cal, ok := compactCalibration(pixels, 8, 0, pitch)
			if !ok {
				for row := 0; row < 10; row++ {
					line := ""
					for x := 8; x < 110; x++ {
						if pixels.ink(x, row) {
							line += "#"
						} else {
							line += "."
						}
					}
					t.Log(line)
				}
				for i := 0; i < 17; i++ {
					g, _ := compactGlyphAt(pixels, 8+i*6, 0, pitch)
					count := 0
					for _, on := range g {
						if on {
							count++
						}
					}
					t.Logf("glyph %d ink %d", i, count)
				}
				t.Fatal("calibration")
			}
			decimal := func(x, y int) (int64, bool) {
				gs := make([]compactGlyph, 16)
				for i := range gs {
					g, valid := compactGlyphAt(pixels, x+i*6, y, pitch)
					if !valid {
						return 0, false
					}
					gs[i] = g
				}
				return compactValue(gs, 10, cal)
			}
			for i, want := range values {
				first, a := decimal(8, 16+i*8)
				second, b := decimal(128, 16+i*8)
				if !a || !b || first != want || second != want {
					t.Fatalf("value %d: %d/%d ok %v/%v", want, first, second, a, b)
				}
			}
		})
	}
}
