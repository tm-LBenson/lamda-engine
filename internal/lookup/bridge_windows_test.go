//go:build windows

package lookup

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"
)

// Exercises real Windows BitBlt/GetDIBits against a private memory bitmap. No
// window is shown, no focus is changed, and no live WoW settings are touched.
func TestWindowsBridgeBitmapRoundTrip(t *testing.T) {
	body := []byte("boot:1|1789400000|key_start|1789400010\nPlayer-1-ALICE|us|Area 52|Alice")
	packet := make([]byte, BridgeBytes)
	copy(packet, "LUI1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
	copy(packet[10:], body)
	if path := os.Getenv("LAMDA_LOOKUP_TEST_PACKET"); path != "" {
		var err error
		packet, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected := DecodeSample(packet)
	if expected.Error != "" {
		t.Fatal(expected.Error)
	}
	for _, preferred := range []BridgePosition{{}, {271, 139}, {16000, 16000}} {
		position, _ := preferred.Clamp(640, 480)
		t.Run(fmt.Sprintf("%d_%d", position.X, position.Y), func(t *testing.T) {
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
			}{Size: 40, Width: 640, Height: -480, Planes: 1, Bits: 32}
			var bits uintptr
			bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
			if bmp == 0 || bits == 0 {
				t.Fatal("CreateDIBSection failed")
			}
			defer deleteObject.Call(bmp)
			old, _, _ := selectObject.Call(dc, bmp)
			defer selectObject.Call(dc, old)
			setPixel := gdi32.NewProc("SetPixelV")
			for cell := 0; cell < 512; cell++ {
				value := uintptr(packet[cell*3]) | uintptr(packet[cell*3+1])<<8 | uintptr(packet[cell*3+2])<<16
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						setPixel.Call(dc, uintptr(position.X+(cell%64)*2+dx), uintptr(position.Y+(cell/64)*2+dy), value)
					}
				}
			}
			var capture captureSession
			defer capture.Close()
			result := capture.captureBridgeDC(dc, position)
			for range 3 {
				if again := capture.captureBridgeDC(dc, position); !reflect.DeepEqual(again, result) {
					t.Fatal("reused capture changed data")
				}
			}
			if !reflect.DeepEqual(result, expected) {
				t.Fatal(result)
			}
			if want := DecodeSample(packet); !reflect.DeepEqual(result.Frames, want.Frames) {
				t.Fatal("frame geometry changed in Windows capture", result.Frames, want.Frames)
			}
			if want := DecodeSample(packet); !reflect.DeepEqual(result.Kick, want.Kick) || !reflect.DeepEqual(result.Observations, want.Observations) {
				t.Fatal("kick/observation data changed in Windows capture")
			}
			if position != (BridgePosition{}) && captureBridgeDC(dc, BridgePosition{}).Error == "" {
				t.Fatal("moved packet found at the old origin")
			}
			// A bad cell must fail CRC instead of producing a different character.
			setPixel.Call(dc, uintptr(position.X+11), uintptr(position.Y+1), 0)
			if result = capture.captureBridgeDC(dc, position); result.Error == "" {
				t.Fatal("corrupt bitmap accepted")
			}
		})
	}
}

func TestGameExecutableIdentity(t *testing.T) {
	production := `D:\World of Warcraft\_forever_`
	if !gameExecutable(production+`\Wow.exe`, production, "forever") {
		t.Fatal("Forever production executable rejected")
	}
	for _, path := range []string{production + `\WowB.exe`, production + `\WowT.exe`, `D:\World of Warcraft\_retail_\Wow.exe`} {
		if gameExecutable(path, production, "forever") {
			t.Fatal("other client captured as Forever production", path)
		}
	}
	beta := `D:\World of Warcraft\_classic_beta_`
	if !gameExecutable(beta+`\WowB.exe`, beta, "forever") {
		t.Fatal("Forever executable rejected")
	}
	for _, path := range []string{beta + `\Wow.exe`, beta + `\WowT.exe`, `D:\World of Warcraft\_retail_\WowB.exe`} {
		if gameExecutable(path, beta, "forever") {
			t.Fatal("other client captured", path)
		}
	}
	retail := `D:\World of Warcraft\_retail_`
	for _, path := range []string{retail + `\Wow.exe`, `d:\WORLD OF WARCRAFT\_retail_\wow.EXE`, retail + `\WowT.exe`} {
		if !gameExecutable(path, retail) {
			t.Errorf("game rejected: %s", path)
		}
	}
	for _, path := range []string{"", `D:\Other\Wow.exe`, retail + `\Launcher.exe`, retail + `\Wow.exe.bak`} {
		if gameExecutable(path, retail) {
			t.Errorf("unrelated executable accepted: %s", path)
		}
	}
}

// Opt-in read-only live probe: normal tests never capture any application.
func TestLiveGameBridgeProbe(t *testing.T) {
	retail := os.Getenv("LAMDA_LIVE_RETAIL")
	if retail == "" {
		t.Skip("live game probe not requested")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	last := ""
	lastFrameID := ""
	validFrames := 0
	for sample := range ReadBridge(ctx, BridgePosition{}, retail) {
		if sample.Error != last {
			t.Logf("bridge=%q window=%d", sample.Error, sample.Window)
			last = sample.Error
		}
		if sample.Error == "" {
			frames := 0
			if sample.Frames != nil {
				frames = len(sample.Frames.Frames)
			}
			if sample.Frames != nil {
				if sample.Frames.ID == lastFrameID {
					continue
				}
				t.Logf("frameSection=true frames=%d eventAge=%.2fs", frames, time.Since(sample.Event.At).Seconds())
				t.Logf("frameAge=%.2fs frameID=%s", time.Since(sample.Frames.At).Seconds(), sample.Frames.ID)
				lastFrameID = sample.Frames.ID
				if age := time.Since(sample.Frames.At); age >= -5*time.Second && age <= 5*time.Second {
					validFrames++
				}
				if validFrames < 3 {
					continue
				}
			} else {
				continue
			}
			return
		}
	}
	t.Fatalf("no valid packet; final status=%q", last)
}

// Exercise both physical strip sizes and automatic fallback entirely in private
// memory bitmaps. No visible windows, game capture, focus, or input is involved.
func TestWindowsSimBridgeWideAndLegacyFallback(t *testing.T) {
	for _, tc := range []struct {
		name, magic   string
		width, height int
		version       byte
	}{{"legacy", "LUS1", BridgeWidth, BridgeHeight, '1'}, {"wide", "LUS2", SimBridgeWidth, SimBridgeHeight, '2'}, {"reload", "LUR2", SimBridgeWidth, SimBridgeHeight, '2'}} {
		for _, preferred := range []BridgePosition{{0, 48}, {271, 139}, {16000, 16000}} {
			position, _ := preferred.clampSize(640, 480, tc.width, tc.height)
			wide, _ := preferred.clampSize(640, 480, SimBridgeWidth, SimBridgeHeight)
			narrow, _ := preferred.Clamp(640, 480)
			t.Run(fmt.Sprintf("%s_%d_%d", tc.name, position.X, position.Y), func(t *testing.T) {
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
				}{Size: 40, Width: 640, Height: -480, Planes: 1, Bits: 32}
				var bits uintptr
				bmp, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
				if bmp == 0 || bits == 0 {
					t.Fatal("CreateDIBSection failed")
				}
				defer deleteObject.Call(bmp)
				old, _, _ := selectObject.Call(dc, bmp)
				defer selectObject.Call(dc, old)
				bodySize := (tc.width/2)*(tc.height/2)*3 - 10
				if tc.magic == "LUR2" {
					bodySize = 512
				}
				body := make([]byte, bodySize)
				body[0], body[1] = tc.version, '|'
				for i := 2; i < len(body); i++ {
					body[i] = byte((i * 71) % 251)
				}
				packet := simPacketFixture(tc.magic, (tc.width/2)*(tc.height/2)*3, body)
				setPixel := gdi32.NewProc("SetPixelV")
				for cell := 0; cell < len(packet)/3; cell++ {
					value := uintptr(packet[cell*3]) | uintptr(packet[cell*3+1])<<8 | uintptr(packet[cell*3+2])<<16
					for dy := 0; dy < 2; dy++ {
						for dx := 0; dx < 2; dx++ {
							setPixel.Call(dc, uintptr(position.X+(cell%(tc.width/2))*2+dx), uintptr(position.Y+(cell/(tc.width/2))*2+dy), value)
						}
					}
				}
				expected := DecodeSample(packet)
				if result := captureSimBridgeDC(dc, wide, narrow, true); !reflect.DeepEqual(result, expected) {
					t.Fatalf("sim size/fallback mismatch: %s", result.Error)
				}
				if tc.magic == "LUS1" {
					if result := captureSimBridgeDC(dc, wide, narrow, false); !reflect.DeepEqual(result, expected) {
						t.Fatal("small-window legacy fallback failed", result.Error)
					}
				} else if result := captureBridgeDC(dc, position); result.Error == "" {
					t.Fatal("wide sim accepted by narrow tracking reader")
				}
				setPixel.Call(dc, uintptr(position.X+21), uintptr(position.Y+1), 0)
				if result := captureSimBridgeDC(dc, wide, narrow, true); result.Error == "" {
					t.Fatal("corrupt sim bitmap accepted")
				}
			})
		}
	}
}

func TestWindowsCaptureSessionReusesAndReleasesSurfaces(t *testing.T) {
	dc, _, _ := compatibleDC.Call(0)
	if dc == 0 {
		t.Fatal("memory DC unavailable")
	}
	defer deleteDC.Call(dc)
	bmp, _, _ := compatibleBitmap.Call(dc, 256, 64)
	if bmp == 0 {
		t.Fatal("memory bitmap unavailable")
	}
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(dc, bmp)
	defer selectObject.Call(dc, old)
	var capture captureSession
	defer capture.Close()
	first, ok := capture.capturePixels(dc, BridgePosition{}, 128, 16)
	if !ok {
		t.Fatal("initial capture failed")
	}
	second, ok := capture.capturePixels(dc, BridgePosition{1, 1}, 128, 16)
	if !ok {
		t.Fatal("repeat capture failed")
	}
	if &first.BGRA[0] != &second.BGRA[0] || len(capture.surfaces) != 1 {
		t.Fatal("capture allocations not reused")
	}
	for i := 1; i < 8; i++ {
		if _, ok := capture.capturePixels(dc, BridgePosition{}, 128, 16+i); !ok {
			t.Fatal("resize failed")
		}
		if len(capture.surfaces) > 4 {
			t.Fatal("unbounded capture resources")
		}
	}
	capture.Close()
	capture.Close()
	if len(capture.surfaces) != 0 {
		t.Fatal("resources not released")
	}
	if _, ok := capture.capturePixels(dc, BridgePosition{}, 128, 16); !ok {
		t.Fatal("capture did not recover after close")
	}
}
