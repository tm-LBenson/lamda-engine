//go:build windows

package lookup

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// Render the bundled real font and Lua metadata into a private bitmap. Tests
// capture it through the shipping GDI code without touching WoW or its focus.
func TestWindowsScreenCastFontAndPacket(t *testing.T) {
	root := os.Getenv("LAMDA_SCREEN_PACKETS")
	fontPath := os.Getenv("LAMDA_SCREEN_FONT")
	if root == "" || fontPath == "" {
		t.Skip("native font fixtures not requested")
	}
	fontSize := 12
	expectRejected := os.Getenv("LAMDA_SCREEN_EXPECT_REJECTION") == "1"
	if configured := os.Getenv("LAMDA_SCREEN_FONT_SIZE"); configured != "" {
		var err error
		fontSize, err = strconv.Atoi(configured)
		if err != nil || fontSize < 9 || fontSize > 14 {
			t.Fatal("invalid native test font size", configured)
		}
	}
	packet, e := os.ReadFile(filepath.Join(root, "packet.bin"))
	if e != nil {
		t.Fatal(e)
	}
	headers, e := os.ReadFile(filepath.Join(root, "headers.bin"))
	if e != nil || len(headers) != 240 {
		t.Fatal("headers", e)
	}
	path, _ := syscall.UTF16PtrFromString(fontPath)
	loaded, _, _ := gdi32.NewProc("AddFontResourceExW").Call(uintptr(unsafe.Pointer(path)), 0x10, 0)
	if loaded == 0 {
		t.Fatal("font load")
	}
	defer gdi32.NewProc("RemoveFontResourceExW").Call(uintptr(unsafe.Pointer(path)), 0x10, 0)
	for _, quality := range []uintptr{3, 4} {
		for _, offset := range []int{24, -184} {
			t.Run(fmt.Sprintf("quality%d_offset%d", quality, offset), func(t *testing.T) {
				dc, _, _ := compatibleDC.Call(0)
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
				faceName := os.Getenv("LAMDA_SCREEN_FONT_FACE")
				if faceName == "" {
					faceName = "Roboto Mono"
				}
				face, _ := syscall.UTF16PtrFromString(faceName)
				weight := uintptr(700)
				if faceName == "Arial" {
					weight = 400
				}
				font, _, _ := gdi32.NewProc("CreateFontW").Call(^uintptr(fontSize-1), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, quality, 0, uintptr(unsafe.Pointer(face)))
				if font == 0 {
					t.Fatal("font")
				}
				defer deleteObject.Call(font)
				oldFont, _, _ := selectObject.Call(dc, font)
				defer selectObject.Call(dc, oldFont)
				gdi32.NewProc("SetTextColor").Call(dc, 0xffffff)
				gdi32.NewProc("SetBkColor").Call(dc, 0)
				position := BridgePosition{20, 220}
				panelY := position.Y + offset
				text := func(x, y int, s string) {
					v, _ := syscall.UTF16FromString(s)
					gdi32.NewProc("TextOutW").Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)-1))
				}
				text(position.X+10, panelY+2, "0123456789")
				setPixel := gdi32.NewProc("SetPixelV")
				paint := func(x, y int, b []byte) {
					for dy := 0; dy < 2; dy++ {
						for dx := 0; dx < 2; dx++ {
							setPixel.Call(dc, uintptr(x+dx), uintptr(y+dy), uintptr(b[0])|uintptr(b[1])<<8|uintptr(b[2])<<16)
						}
					}
				}
				p := append([]byte{}, packet...)
				size := int(binary.BigEndian.Uint16(p[4:6]))
				body := string(p[10 : 10+size])
				decoded := DecodeSample(p)
				if decoded.Error != "" || decoded.Observations.Screen == nil {
					t.Fatal(decoded)
				}
				if offset < 0 {
					body = strings.Replace(body, "T|1|"+decoded.Observations.Screen.ID+"|24|", "T|1|"+decoded.Observations.Screen.ID+"|-184|", 1)
				}
				var extent struct{ X, Y int32 }
				calibration, _ := syscall.UTF16PtrFromString("0123456789")
				ok, _, _ := gdi32.NewProc("GetTextExtentPoint32W").Call(dc, uintptr(unsafe.Pointer(calibration)), 10, uintptr(unsafe.Pointer(&extent)))
				if ok == 0 || extent.X <= 40 || extent.X >= 100 {
					t.Fatal("calibration size", extent)
				}
				// The addon measures its actual public calibration font; fallback
				// fonts must use that pitch rather than the mock fixture's 7px.
				body = strings.Replace(body, "|7000\n", "|"+strconv.Itoa(int(extent.X)*100)+"\n", 1)
				if strings.HasSuffix(body, "|7000") {
					body = strings.TrimSuffix(body, "|7000") + "|" + strconv.Itoa(int(extent.X)*100)
				}
				copy(p[10:], body)
				binary.BigEndian.PutUint16(p[4:6], uint16(len(body)))
				binary.BigEndian.PutUint32(p[6:10], crc32.ChecksumIEEE([]byte(body)))
				for cell := 0; cell < len(p)/3; cell++ {
					paint(position.X+cell%64*2, position.Y+cell/64*2, p[cell*3:cell*3+3])
				}
				for index := 0; index < 20; index++ {
					for cell := 0; cell < 4; cell++ {
						start := index*12 + cell*3
						paint(position.X+index%2*64+2+cell%2*2, panelY+16+index/2*16+6+cell/2*2, headers[start:start+3])
					}
				}
				for _, spell := range []int{1766, 2139, 6552, 15487, 19647, 47528, 57994, 78675, 89766, 96231, 106839, 116705, 147362, 183752, 187707, 351338, 119910, 119914, 347008, 195439, 642, 20594, 59752, 363916, 436344} {
					text(position.X+10, panelY+16+2*16+2, fmt.Sprintf("%07d", spell))
					result := captureScreenBridgeDC(dc, position, offset, 0)
					if expectRejected {
						if result.Error != "" || result.Observations.Screen.Captured || len(result.Observations.Screen.Casts) != 0 {
							t.Fatalf("ambiguous small font accepted: %+v", result)
						}
						continue
					}
					if result.Error != "" || !result.Observations.Screen.Captured || len(result.Observations.Screen.Casts) != 1 {
						t.Fatalf("%d: error=%s %+v", spell, result.Error, result.Observations.Screen)
					}
					c := result.Observations.Screen.Casts[0]
					owner := "Player-1-BOB"
					if decoded.Arena != nil {
						owner = decoded.Arena.Identity(2)
					}
					if c.Spell != spell || c.GUID != owner || c.AgeMillis != 250 {
						t.Fatal(c)
					}
				}
				// Header occlusion cannot be mistaken for a different owner or cast.
				setPixel.Call(dc, uintptr(position.X+3), uintptr(panelY+16+2*16+7), 0)
				result := captureScreenBridgeDC(dc, position, offset, 0)
				if result.Error != "" || len(result.Observations.Screen.Casts) != 0 {
					t.Fatal("occlusion accepted")
				}
			})
		}
	}
}
