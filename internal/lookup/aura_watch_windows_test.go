//go:build windows

package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

// Real GDI capture of the Lua-produced packet and native tile colors. A private
// memory bitmap keeps the test away from WoW, desktop focus and user input.
func TestWindowsAuraBandBitmap(t *testing.T) {
	root := os.Getenv("LAMDA_AURA_PACKETS")
	if root == "" {
		t.Skip("Lua fixture not requested")
	}
	packet, err := os.ReadFile(filepath.Join(root, "packet.bin"))
	if err != nil {
		t.Fatal(err)
	}
	colors, err := os.ReadFile(filepath.Join(root, "on.bin"))
	if err != nil {
		t.Fatal(err)
	}
	decoded := DecodeSample(packet)
	if decoded.Error != "" || decoded.Observations.Auras == nil {
		t.Fatal(decoded)
	}
	for _, offset := range []int{16, -8} {
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
		}{Size: 40, Width: 384, Height: -200, Planes: 1, Bits: 32}
		var bits uintptr
		bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
		if bmp == 0 || bits == 0 {
			t.Fatal("bitmap")
		}
		defer deleteObject.Call(bmp)
		old, _, _ := selectObject.Call(dc, bmp)
		defer selectObject.Call(dc, old)
		p := append([]byte{}, packet...)
		size := int(binary.BigEndian.Uint16(p[4:6]))
		body := string(p[10 : 10+size])
		if offset < 0 {
			body = strings.Replace(body, "A|1|"+decoded.Observations.Auras.ID+"|16|", "A|1|"+decoded.Observations.Auras.ID+"|-8|", 1)
		}
		copy(p[10:], body)
		binary.BigEndian.PutUint16(p[4:6], uint16(len(body)))
		binary.BigEndian.PutUint32(p[6:10], crc32.ChecksumIEEE([]byte(body)))
		position := BridgePosition{99, 100}
		setPixel := gdi32.NewProc("SetPixelV")
		paint := func(data []byte, y int) {
			for cell := 0; cell < len(data)/3; cell++ {
				value := uintptr(data[cell*3]) | uintptr(data[cell*3+1])<<8 | uintptr(data[cell*3+2])<<16
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						setPixel.Call(dc, uintptr(position.X+(cell%64)*2+dx), uintptr(y+(cell/64)*2+dy), value)
					}
				}
			}
		}
		paint(p, position.Y)
		paint(colors, position.Y+offset)
		result := captureAuraBridgeDC(dc, position, offset)
		if result.Error != "" || !result.Observations.Auras.Captured {
			t.Fatal("band capture", offset, result)
		}
		for _, m := range result.Observations.Auras.Members {
			for i := range AuraWatchSpells[m.Class] {
				if m.Mask&(1<<i) != 0 && !result.Observations.Auras.Active[m.Slot*8+i] {
					t.Fatal("lost onset", offset)
				}
			}
		}
		// Occluded native pixels invalidate only the fast signal, keeping the
		// independently validated cooldown and kick packet usable.
		m := result.Observations.Auras.Members[0]
		slot := m.Slot * 8
		setPixel.Call(dc, uintptr(position.X+(slot%32)*4+1), uintptr(position.Y+offset+(slot/32)*4+1), 0)
		result = captureAuraBridgeDC(dc, position, offset)
		if result.Error != "" || result.Observations.Auras.Captured {
			t.Fatal("occlusion handling", result)
		}
	}
}
