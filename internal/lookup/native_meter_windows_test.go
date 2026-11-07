//go:build windows

package lookup

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// Private off-screen GDI fixture; it neither finds nor controls a WoW window.
func TestWindowsNativeMeterRealFont(t *testing.T) {
	fontPath := os.Getenv("LAMDA_NATIVE_METER_FONT")
	if fontPath == "" {
		t.Skip("native font fixture not requested")
	}
	path, _ := syscall.UTF16PtrFromString(fontPath)
	loaded, _, _ := gdi32.NewProc("AddFontResourceExW").Call(uintptr(unsafe.Pointer(path)), 0x10, 0)
	if loaded == 0 {
		t.Fatal("font load")
	}
	defer gdi32.NewProc("RemoveFontResourceExW").Call(uintptr(unsafe.Pointer(path)), 0x10, 0)
	for _, fontSize := range []int{11, 12} {
		for _, quality := range []uintptr{3, 4} {
			for _, offset := range []int{32, -72} {
				t.Run(fmt.Sprintf("font%d_quality%d_offset%d", fontSize, quality, offset), func(t *testing.T) {
					dc, _, _ := compatibleDC.Call(0)
					defer deleteDC.Call(dc)
					info := struct {
						Size                   uint32
						Width, Height          int32
						Planes, Bits           uint16
						Compression, SizeImage uint32
						X, Y                   int32
						Used, Important        uint32
					}{Size: 40, Width: 320, Height: -240, Planes: 1, Bits: 32}
					var bits uintptr
					bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
					if bmp == 0 {
						t.Fatal("bitmap")
					}
					defer deleteObject.Call(bmp)
					old, _, _ := selectObject.Call(dc, bmp)
					defer selectObject.Call(dc, old)
					face, _ := syscall.UTF16PtrFromString("Roboto Mono")
					font, _, _ := gdi32.NewProc("CreateFontW").Call(uintptr(-fontSize), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, quality, 0, uintptr(unsafe.Pointer(face)))
					if font == 0 {
						t.Fatal("font")
					}
					defer deleteObject.Call(font)
					oldFont, _, _ := selectObject.Call(dc, font)
					defer selectObject.Call(dc, oldFont)
					gdi32.NewProc("SetTextColor").Call(dc, 0xffffff)
					gdi32.NewProc("SetBkColor").Call(dc, 0)
					text := func(x, y int, s string) {
						v, _ := syscall.UTF16FromString(s)
						gdi32.NewProc("TextOutW").Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)-1))
					}
					v, _ := syscall.UTF16FromString("0123456789")
					var size struct{ X, Y int32 }
					gdi32.NewProc("GetTextExtentPoint32W").Call(dc, uintptr(unsafe.Pointer(&v[0])), 10, uintptr(unsafe.Pointer(&size)))
					pitch := int(size.X) * 100
					body := fmt.Sprintf("1|123:45:1|7|68|Player-1-Aug|1473|1|%d|%d|12345\n1|Player-1-Aug|13|Aug|0\n2|Player-1-Ally|8|Ally|0", pitch, offset)
					packet := nativePacket(body)
					sample := DecodeSample(packet)
					if sample.Error != "" {
						t.Fatal(sample.Error)
					}
					position := BridgePosition{X: 20, Y: 100}
					panelY := position.Y + offset
					setPixel := gdi32.NewProc("SetPixelV")
					paint := func(x, y int, b []byte) {
						for yy := 0; yy < 2; yy++ {
							for xx := 0; xx < 2; xx++ {
								setPixel.Call(dc, uintptr(x+xx), uintptr(y+yy), uintptr(b[0])|uintptr(b[1])<<8|uintptr(b[2])<<16)
							}
						}
					}
					for i := 0; i < 2048; i++ {
						paint(position.X+(i%128)*2, position.Y+(i/128)*2, packet[i*3:i*3+3])
					}
					text(position.X+12, panelY+2, "0123456789X")
					for index, value := range []int64{12, 4294967296, 999999999999999} {
						y := panelY + 16 + index*16
						h := []byte{173, byte(index), 118, 0, 0, 7, 59, 0, 93, 205, 127, 61}
						for i := 0; i < 7; i++ {
							h[7] += h[i] * byte(i+1)
						}
						for c := 0; c < 4; c++ {
							paint(position.X+2+c%2*2, y+6+c/2*2, h[c*3:c*3+3])
						}
						text(position.X+12, y+2, fmt.Sprintf("%015dX", value))
						text(position.X+140, y+2, fmt.Sprintf("%015dX", value))
					}
					capture := &captureSession{}
					defer capture.Close()
					result := capture.captureNativeMeterDC(dc, position, sample.NativeMeter)
					if result.NativeMeter == nil || !result.NativeMeter.Captured || result.NativeMeter.Members[0].Total != 4294967296 || result.NativeMeter.Members[1].Total != 999999999999999 {
						t.Fatalf("capture: %+v", result.NativeMeter)
					}
				})
			}
		}
	}
}
