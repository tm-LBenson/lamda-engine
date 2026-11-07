//go:build windows

package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"testing"
	"unsafe"
)

// Reproduce the live failure: the bottom edge already contains an LUO1 packet.
// Queue.lua renders 48 physical pixels above that edge, independently of it.
func TestQueueWindowsCoexistsWithOverlayFooter(t *testing.T) {
	const width, height = 320, 200
	dc, _, _ := compatibleDC.Call(0)
	if dc == 0 {
		t.Fatal("CreateCompatibleDC failed")
	}
	defer deleteDC.Call(dc)
	info := struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, SizeImage uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, Width: width, Height: -height, Planes: 1, Bits: 32}
	var bits uintptr
	bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		t.Fatal("CreateDIBSection failed")
	}
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(dc, bmp)
	defer selectObject.Call(dc, old)
	setPixel := gdi32.NewProc("SetPixelV")
	paint := func(data []byte, position BridgePosition) {
		for cell := 0; cell < 512; cell++ {
			color := uintptr(data[cell*3]) | uintptr(data[cell*3+1])<<8 | uintptr(data[cell*3+2])<<16
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					setPixel.Call(dc, uintptr(position.X+(cell%64)*2+dx), uintptr(position.Y+(cell/64)*2+dy), color)
				}
			}
		}
	}
	footer := make([]byte, BridgeBytes)
	copy(footer, "LUO1")
	paint(footer, BridgePosition{Y: height - 16})
	body := queueBody(1)
	packet := make([]byte, BridgeBytes)
	copy(packet, "LUQ1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
	copy(packet[10:], body)
	paint(packet, BridgePosition{Y: height - 48 - 16})
	var capture captureSession
	defer capture.Close()
	sample := capture.captureBridgeDC(dc, queueBridgePosition(height))
	if sample.Error != "" || string(sample.Queue) != string(body) {
		t.Fatal("queue reader collided with the overlay footer", sample.Error)
	}
	data, ok := capture.captureRGBCells(dc, BridgePosition{Y: height - 16}, BridgeWidth, BridgeHeight)
	if !ok || string(data[:4]) != "LUO1" {
		t.Fatal("queue changed the overlay footer")
	}
	if sample = capture.captureBridgeDC(dc, BridgePosition{Y: height - 16}); sample.Error != "bridge_not_visible" {
		t.Fatal("old position no longer reproduces the failure", sample.Error)
	}
}

// Layout verification only. This test never submits mouse or keyboard input.
func TestQueueWindowsInputLayout(t *testing.T) {
	var input queueInput
	want, offset := uintptr(28), uintptr(4)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want, offset = 40, 8
	}
	if unsafe.Sizeof(input) != want || unsafe.Offsetof(input.Mouse) != offset {
		t.Fatalf("INPUT layout: size=%d mouse offset=%d", unsafe.Sizeof(input), unsafe.Offsetof(input.Mouse))
	}
}

// Read only a private GDI bitmap. No desktop window, cursor or input is used.
func TestQueueWindowsMarkerBitmap(t *testing.T) {
	dc, _, _ := compatibleDC.Call(0)
	if dc == 0 {
		t.Fatal("CreateCompatibleDC failed")
	}
	defer deleteDC.Call(dc)
	info := struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, SizeImage uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, Width: 8, Height: -2, Planes: 1, Bits: 32}
	var bits uintptr
	bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		t.Fatal("CreateDIBSection failed")
	}
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(dc, bmp)
	defer selectObject.Call(dc, old)
	marker := queueMarker("invitation_1")
	set := gdi32.NewProc("SetPixelV")
	for cell := 0; cell < 4; cell++ {
		offset := cell * 3
		color := uintptr(marker[offset]) | uintptr(marker[offset+1])<<8 | uintptr(marker[offset+2])<<16
		for x := 0; x < 2; x++ {
			for y := 0; y < 2; y++ {
				set.Call(dc, uintptr(cell*2+x), uintptr(y), color)
			}
		}
	}
	var capture captureSession
	defer capture.Close()
	if !capture.captureQueueMarkerDC(dc, BridgePosition{}, "invitation_1") {
		t.Fatal("valid rendered marker rejected")
	}
	if capture.captureQueueMarkerDC(dc, BridgePosition{}, "other_invitation") {
		t.Fatal("different invitation marker accepted")
	}
	set.Call(dc, 1, 1, 0)
	if capture.captureQueueMarkerDC(dc, BridgePosition{}, "invitation_1") {
		t.Fatal("covered or corrupted marker accepted")
	}
}
