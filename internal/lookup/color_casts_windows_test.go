//go:build windows

package lookup

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

// Actual Lua metadata and headers plus synthetic solid color patches exercise
// the shipping Windows capture path. This does not emulate WoW's formatter.
func TestWindowsColorCastPacket(t *testing.T) {
	root := os.Getenv("LAMDA_COLOR_PACKETS")
	if root == "" {
		t.Skip("Lua color fixtures not requested")
	}
	packet, e := os.ReadFile(filepath.Join(root, "packet.bin"))
	if e != nil {
		t.Fatal(e)
	}
	headers, e := os.ReadFile(filepath.Join(root, "headers.bin"))
	if e != nil || len(headers) != 240 {
		t.Fatal("headers", e)
	}
	for _, version := range []int{2, 3} {
		height := ColorCastHeight
		if version == 3 {
			height = MeterCastHeight
		}
		for _, offset := range []int{24, -(height + 8)} {
			t.Run(fmt.Sprintf("v%d-%d", version, offset), func(t *testing.T) {
				dc, _, _ := compatibleDC.Call(0)
				if dc == 0 {
					t.Fatal("DC")
				}
				defer deleteDC.Call(dc)
				info := struct {
					Size                   uint32
					Width, Height          int32
					Planes, Bits           uint16
					Compression, SizeImage uint32
					X, Y                   int32
					Used, Important        uint32
				}{Size: 40, Width: 256, Height: -480, Planes: 1, Bits: 32}
				var bits uintptr
				bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
				if bmp == 0 {
					t.Fatal("bitmap")
				}
				defer deleteObject.Call(bmp)
				old, _, _ := selectObject.Call(dc, bmp)
				defer selectObject.Call(dc, old)
				position := BridgePosition{20, 50}
				panelY := position.Y + offset
				setPixel := gdi32.NewProc("SetPixelV")
				paint := func(x, y, size, c int) {
					for yy := y; yy < y+size; yy++ {
						for xx := x; xx < x+size; xx++ {
							setPixel.Call(dc, uintptr(xx), uintptr(yy), uintptr(c>>16&255)|uintptr(c>>8&255)<<8|uintptr(c&255)<<16)
						}
					}
				}
				p := append([]byte{}, packet...)
				size := int(binary.BigEndian.Uint16(p[4:6]))
				body := string(p[10 : 10+size])
				decoded := DecodeSample(p)
				if decoded.Error != "" || decoded.Observations == nil || decoded.Observations.Screen == nil {
					t.Fatal(decoded)
				}
				body = strings.Replace(body, "T|2|"+decoded.Observations.Screen.ID+"|24|", fmt.Sprintf("T|%d|%s|%d|", version, decoded.Observations.Screen.ID, offset), 1)
				copy(p[10:], body)
				binary.BigEndian.PutUint16(p[4:6], uint16(len(body)))
				binary.BigEndian.PutUint32(p[6:10], crc32.ChecksumIEEE([]byte(body)))
				for cell := 0; cell < len(p)/3; cell++ {
					i := cell * 3
					paint(position.X+cell%64*2, position.Y+cell/64*2, 2, int(p[i])<<16|int(p[i+1])<<8|int(p[i+2]))
				}
				for i, c := range castCalibrationColors {
					paint(position.X+i*4, panelY, 4, c)
				}
				for index := 0; index < 20; index++ {
					x, y := position.X+index%10*12, panelY+4+index/10*4
					for cell := 0; cell < 4; cell++ {
						i := index*12 + cell*3
						paint(x+cell%2*2, y+cell/2*2, 2, int(headers[i])<<16|int(headers[i+1])<<8|int(headers[i+2]))
					}
					paint(x+4, y, 4, 47528)
					paint(x+8, y, 4, 47528)
				}
				if version == 3 {
					fixture, _ := meterFixture()
					for y := 12; y < MeterCastHeight; y++ {
						for x := 0; x < 128; x++ {
							r, g, b := fixture.rgb(x, y)
							paint(position.X+x, panelY+y, 1, int(r)<<16|int(g)<<8|int(b))
						}
					}
				}
				result := captureScreenBridgeDC(dc, position, offset, 0, height)
				if result.Error != "" || !result.Observations.Screen.Captured || len(result.Observations.Screen.Casts) != 1 {
					t.Fatalf("%+v", result)
				}
				if version == 3 && len(result.Observations.Screen.Meter) != len(result.Observations.Screen.Members) {
					t.Fatal("missing meter owners", result.Observations.Screen)
				}
				c := result.Observations.Screen.Casts[0]
				if c.Spell != 47528 || c.AgeMillis != 250 || c.GUID != "Player-1-BOB" {
					t.Fatal(c)
				}
				if offset == 24 {
					wrong := captureScreenBridgeDC(dc, position, offset, 0, ScreenCastHeight)
					if wrong.Error != "screen_mapping_changed" {
						t.Fatal("format race accepted", wrong)
					}
				}
				paint(position.X+1, panelY+1, 1, 0)
				result = captureScreenBridgeDC(dc, position, offset, 0, height)
				if result.Error != "" || result.Observations.Screen.Captured || result.Observations.Screen.Error != "color_calibration_mismatch" {
					t.Fatal("calibration occlusion accepted", result)
				}
			})
		}
	}
}
