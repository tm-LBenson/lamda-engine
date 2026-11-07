//go:build windows

package lookup

import (
	"fmt"
	"testing"
	"unsafe"
)

// Exercise actual GDI copies at a moved origin, without interacting with WoW.
func TestWindowsReshapedOverlayCapture(t *testing.T) {
	for _, format := range []int{0, 2, 3} {
		dense := format >= 2
		for width := 16; width <= 1152; width += 4 {
			if !ValidOverlayWidth(width) {
				continue
			}
			t.Run(fmt.Sprintf("format-%v-width-%d", format, width), func(t *testing.T) {
				p := overlayShapePixels(width)
				if dense {
					a := denseInput(false)
					a.NativeVersion = format
					if format == 3 {
						for i := 0; i < 26; i++ {
							a.Cooldowns = append(a.Cooldowns, OverlayCooldown{Kind: "spell", ID: 3000 + i, Available: true})
						}
					}
					a.StripWidth = width
					raw := densePixels(t, a)
					p = Pixels{Width: width, Height: 18432 / width, BGRA: make([]byte, 18432*4)}
					for tile := 0; tile < 1152; tile++ {
						for y := 0; y < 4; y++ {
							from := ((tile/64*4+y)*256 + tile%64*4) * 4
							to := ((tile/(width/4)*4+y)*width + tile%(width/4)*4) * 4
							copy(p.BGRA[to:to+16], raw.BGRA[from:from+16])
						}
					}
				}
				dc, _, _ := compatibleDC.Call(0)
				if dc == 0 {
					t.Fatal("DC")
				}
				func() {
					defer deleteDC.Call(dc)
					info := struct {
						Size                   uint32
						Width, Height          int32
						Planes, Bits           uint16
						Compression, SizeImage uint32
						X, Y                   int32
						Used, Important        uint32
					}{Size: 40, Width: int32(width + 64), Height: -int32(p.Height + 64), Planes: 1, Bits: 32}
					var bits uintptr
					bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
					if bmp == 0 {
						t.Fatal("bitmap")
					}
					defer deleteObject.Call(bmp)
					old, _, _ := selectObject.Call(dc, bmp)
					defer selectObject.Call(dc, old)
					data := unsafe.Slice((*byte)(unsafe.Pointer(bits)), int(info.Width)*int(-info.Height)*4)
					for y := 0; y < p.Height; y++ {
						i := ((y+29)*int(info.Width) + 17) * 4
						copy(data[i:i+width*4], p.BGRA[y*width*4:(y+1)*width*4])
					}
					captured, ok := capturePixels(dc, BridgePosition{X: 17, Y: 29}, width, p.Height)
					if !ok {
						t.Fatal("capture", width)
					}
					s := DecodeOverlayShape(captured)
					if s.Error != "" || s.Overlays.Resources["0"].Current != map[bool]float64{false: 15, true: 10}[dense] || *s.Overlays.Health != 90 {
						t.Fatal(width, s)
					}

					if dense && (len(s.Overlays.Watches) != 64 || s.Overlays.Watches[63].Stacks == nil || len(s.Overlays.Measurements) != 64) {
						t.Fatal("Dense GDI capture lost data")
					}
				}()
			})
		}
	}
}
