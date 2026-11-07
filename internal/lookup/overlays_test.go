package lookup

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"
)

func overlayPixels(errors ...string) Pixels {
	message := ""
	if len(errors) > 0 {
		message = errors[0]
	}
	b, _ := json.Marshal(OverlayInput{CaptureError: message, Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, Watches: []OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true}}})
	packet := make([]byte, SimBridgeBytes)
	copy(packet, "LUO1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(b)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(b))
	copy(packet[10:], b)
	p := Pixels{Width: 256, Height: 72, BGRA: make([]byte, 256*72*4)}
	for i := 0; i < len(packet)/3; i++ {
		x := (i % 128) * 2
		y := (i / 128) * 2
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				j := ((y+dy)*256 + x + dx) * 4
				p.BGRA[j] = packet[i*3+2]
				p.BGRA[j+1] = packet[i*3+1]
				p.BGRA[j+2] = packet[i*3]
			}
		}
	}
	for i, v := range OverlayCalibration {
		paintOverlay(&p, i*4, 32, v)
	}
	for _, x := range []int{0, 16} {
		paintOverlay(&p, x, 40, 0x37d6a0)
		paintOverlay(&p, x+4, 40, 3)
		paintOverlay(&p, x+8, 40, 49)
	}
	paintOverlay(&p, 40, 32, 90)
	paintOverlay(&p, 44, 32, 90)
	return p
}

func TestOverlaySetupFailureIsUnknownEvenWithInactiveOrStalePixels(t *testing.T) {
	for _, presence := range []int{0x19354b, 0x37d6a0} {
		p := overlayPixels("Native binding failed")
		paintOverlay(&p, 0, 40, presence)
		paintOverlay(&p, 16, 40, presence)
		s := DecodeOverlayPixels(p, 0, 32)
		if s.Error != "" || s.Overlays == nil || s.Overlays.CaptureError != "Native binding failed" {
			t.Fatal(s)
		}
		w := s.Overlays.Watches[0]
		if w.Active != nil || w.Stacks != nil || w.Remaining != nil || w.Permanent {
			t.Fatal("failed setup produced a native aura reading", w)
		}
		if s.Overlays.Health == nil || *s.Overlays.Health != 90 {
			t.Fatal("aura setup error discarded healthy capture")
		}
	}
	if s := DecodeOverlayPixels(overlayPixels(strings.Repeat("x", 241)), 0, 32); s.Error == "" {
		t.Fatal("oversized setup error accepted")
	}
}
func paintOverlay(p *Pixels, x, y, v int) {
	for dy := 0; dy < 4; dy++ {
		for dx := 0; dx < 4; dx++ {
			j := ((y+dy)*p.Width + x + dx) * 4
			p.BGRA[j] = byte(v)
			p.BGRA[j+1] = byte(v >> 8)
			p.BGRA[j+2] = byte(v >> 16)
		}
	}
}
func TestOverlayNativeCaptureRejectsPartialAndUnknown(t *testing.T) {
	p := overlayPixels()
	s := DecodeOverlayPixels(p, 0, 32)
	if s.Error != "" || s.Overlays.Health == nil || *s.Overlays.Health != 90 || s.Overlays.Watches[0].Remaining == nil || *s.Overlays.Watches[0].Remaining != 4.9 {
		t.Fatal(s)
	}
	paintOverlay(&p, 20, 40, 4)
	s = DecodeOverlayPixels(p, 0, 32)
	if s.Overlays.Watches[0].Stacks != nil {
		t.Fatal("different copies accepted")
	}
	paintOverlay(&p, 0, 32, 42)
	if DecodeOverlayPixels(p, 0, 32).Error == "" {
		t.Fatal("bad calibration accepted")
	}
	p = overlayPixels()
	paintOverlay(&p, 0, 40, 0)
	s = DecodeOverlayPixels(p, 0, 32)
	if s.Overlays.Watches[0].Active != nil {
		t.Fatal("occluded presence became absence")
	}
	p = overlayPixels()
	p.BGRA[(256+1)*4+2] ^= 42
	if DecodeOverlayPixels(p, 0, 32).Error == "" {
		t.Fatal("torn packet accepted")
	}
}

func TestOverlayResourcesPairedCapacityAndOldAddon(t *testing.T) {
	p := overlayPixels()
	// Update metadata while keeping native samples; resource readings in JSON are ignored.
	update := func(version int) {
		mods := map[string]float64{}
		for i := 0; i < 30; i++ {
			mods[fmt.Sprint(i)] = 1
		}
		mods["7"] = 10
		b, _ := json.Marshal(OverlayInput{Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, ResourceMods: mods, ResourcesVersion: version, PrimaryPower: 19, Resources: map[string]OverlayResource{"0": {0, 100}}})
		packet := make([]byte, SimBridgeBytes)
		copy(packet, "LUO1")
		binary.BigEndian.PutUint16(packet[4:6], uint16(len(b)))
		binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(b))
		copy(packet[10:], b)
		for i := 0; i < len(packet)/3; i++ {
			x, y := (i%128)*2, (i/128)*2
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					j := ((y+dy)*256 + x + dx) * 4
					p.BGRA[j] = packet[i*3+2]
					p.BGRA[j+1] = packet[i*3+1]
					p.BGRA[j+2] = packet[i*3]
				}
			}
		}
	}
	update(1)
	for power := 0; power < 30; power++ {
		x, y := (power%16)*16, 44+(power/16)*16
		for i, v := range []int{3, 3, 6, 6} {
			paintOverlay(&p, x+i*4, y, v)
		}
	}
	s := DecodeOverlayPixels(p, 0, 32)
	if s.Error != "" || len(s.Overlays.Resources) != 30 || s.Overlays.Resources["7"].Current != .3 || s.Overlays.Resources["19"].Current != 3 {
		t.Fatal(s)
	}
	for _, tc := range []struct {
		name   string
		values []int
	}{{"no pool", []int{0, 0, 0, 0}}, {"mismatched current", []int{3, 4, 6, 6}}, {"mismatched max", []int{3, 3, 6, 5}}, {"unknown", []int{0xffffff, 0xffffff, 0xffffff, 0xffffff}}, {"over max", []int{7, 7, 6, 6}}} {
		t.Run(tc.name, func(t *testing.T) {
			for i, v := range tc.values {
				paintOverlay(&p, i*4, 44, v)
			}
			s = DecodeOverlayPixels(p, 0, 32)
			if _, ok := s.Overlays.Resources["0"]; ok {
				t.Fatal("invalid resource accepted")
			}
		})
	}
	update(0)
	if len(DecodeOverlayPixels(p, 0, 32).Overlays.Resources) != 0 {
		t.Fatal("old addon/JSON invented resource")
	}
}

func overlayShapePixels(width int) Pixels {
	p := overlayPixels()
	a := DecodeOverlayPixels(p, 0, 32).Overlays
	a.StripWidth = width
	a.ResourcesVersion = 1
	a.ResourceMods = map[string]float64{"0": 1}
	b, _ := json.Marshal(a)
	packet := make([]byte, SimBridgeBytes)
	copy(packet, "LUO1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(b)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(b))
	copy(packet[10:], b)
	for i := 0; i < len(packet)/3; i++ {
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				j := (((i/128)*2+dy)*256 + (i%128)*2 + dx) * 4
				p.BGRA[j], p.BGRA[j+1], p.BGRA[j+2] = packet[i*3+2], packet[i*3+1], packet[i*3]
			}
		}
	}
	for i, v := range []int{15, 15, 100, 100} {
		paintOverlay(&p, i*4, 44, v)
	}
	out := Pixels{Width: width, Height: 18432 / width, BGRA: make([]byte, 18432*4)}
	// Place whole tiles, as the addon does, independently of the decoder's pixel walk.
	for tile := 0; tile < 1152; tile++ {
		for y := 0; y < 4; y++ {
			from := ((tile/64*4+y)*256 + tile%64*4) * 4
			to := ((tile/(width/4)*4+y)*width + tile%(width/4)*4) * 4
			copy(out.BGRA[to:to+16], p.BGRA[from:from+16])
		}
	}
	return out
}

func TestOverlayEveryShapePreservesNativeReadings(t *testing.T) {
	for width := 16; width <= 1152; width += 4 {
		if !ValidOverlayWidth(width) {
			continue
		}
		p := overlayShapePixels(width)
		s := DecodeOverlayShape(p)
		if s.Error != "" || s.Overlays.Health == nil || *s.Overlays.Health != 90 || *s.Overlays.Watches[0].Remaining != 4.9 || s.Overlays.Resources["0"].Current != 15 {
			t.Fatalf("width %d: %#v", width, s)
		}
		p.BGRA[(width+1)*4+2] ^= 42
		if DecodeOverlayShape(p).Error == "" {
			t.Fatal("moving/torn header accepted", width)
		}
	}
	if DecodeOverlayShape(overlayPixels()).Error == "" {
		t.Fatal("legacy metadata accepted as a new shape")
	}
	for _, width := range []int{0, 1, 20, 100, 255, 1024, 4096} {
		if ValidOverlayWidth(width) {
			t.Fatal("invalid shape", width)
		}
	}
}
