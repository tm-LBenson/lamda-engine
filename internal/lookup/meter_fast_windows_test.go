//go:build windows

package lookup

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// Render only an off-screen DIB with our own font. Never locates a game,
// creates a visible window, changes focus or sends input.
func TestWindowsFastMeterStripFont(t *testing.T) {
	path := os.Getenv("LAMDA_COMPACT_METER_FONT")
	if path == "" {
		t.Skip("font fixture not requested")
	}
	file, _ := syscall.UTF16PtrFromString(path)
	loaded, _, _ := gdi32.NewProc("AddFontResourceExW").Call(uintptr(unsafe.Pointer(file)), 0x10, 0)
	if loaded == 0 {
		t.Fatal("font load")
	}
	defer gdi32.NewProc("RemoveFontResourceExW").Call(uintptr(unsafe.Pointer(file)), 0x10, 0)
	for _, quality := range []uintptr{3, 4} {
		t.Run(fmt.Sprint(quality), func(t *testing.T) {
			p, h := fastFixture(40)
			dc, _, _ := compatibleDC.Call(0)
			defer deleteDC.Call(dc)
			info := struct {
				Size                   uint32
				Width, Height          int32
				Planes, Bits           uint16
				Compression, SizeImage uint32
				X, Y                   int32
				Used, Important        uint32
			}{Size: 40, Width: int32(p.Width), Height: -int32(p.Height), Planes: 1, Bits: 32}
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
			gdi32.NewProc("SetTextCharacterExtra").Call(dc, 1)
			// Remove synthetic glyphs, leaving only CRC-bound metadata, row tags and auras.
			for block := 0; block < 42; block++ {
				at := fastMeterOrigin(h.Layout, h.BodyBytes, block)
				for _, x := range []int{8, 106} {
					for y := 0; y < 8; y++ {
						limit := 96
						if block == 0 {
							limit = 102
						}
						for xx := 0; xx < limit; xx++ {
							off := ((at.Y+y)*p.Width + at.X + x + xx) * 4
							clear(p.BGRA[off : off+4])
						}
					}
				}
			}
			pixels := unsafe.Slice((*byte)(unsafe.Pointer(bits)), len(p.BGRA))
			copy(pixels, p.BGRA)
			draw := func(x, y int, text string) {
				v, _ := syscall.UTF16FromString(text)
				gdi32.NewProc("TextOutW").Call(dc, uintptr(x), uintptr(y+2), uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)-1))
			}
			at := fastMeterOrigin(h.Layout, h.BodyBytes, 0)
			draw(at.X+8, at.Y, "0123456789abcdefX")

			for row := 0; row <= 40; row++ {
				at := fastMeterOrigin(h.Layout, h.BodyBytes, row+1)
				n := int64(12)
				if row > 0 {
					n = 4294967296 + int64(row)
				}
				s := fmt.Sprintf("%015dX", n)
				draw(at.X+8, at.Y, s)
				draw(at.X+106, at.Y, s)
			}
			gdi32.NewProc("GdiFlush").Call()
			p.BGRA = append([]byte(nil), pixels...)
			s := DecodeMeterBorder(p)
			if s.NativeMeter == nil || !s.NativeMeter.Captured {
				t.Fatalf("native font decode: %s %+v", s.Error, s.NativeMeter)
			}
			if len(s.NativeMeter.Members) != 40 || s.NativeMeter.Members[39].Total != 4294967336 {
				t.Fatal(s.NativeMeter)
			}
		})
	}
}
