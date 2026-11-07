package lookup

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"testing"
)

func cooldownPixels(input OverlayInput) Pixels {
	p := overlayPixels()
	b, _ := json.Marshal(input)
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
				copy(p.BGRA[j:j+3], []byte{packet[i*3+2], packet[i*3+1], packet[i*3]})
			}
		}
	}
	return p
}

func TestCooldownPixelsRequirePairedMarkedMeasurements(t *testing.T) {
	input := OverlayInput{Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, CooldownsVersion: 1, CooldownHash: "1234567812345678"}
	for i := 0; i < 16; i++ {
		input.Cooldowns = append(input.Cooldowns, OverlayCooldown{Kind: "spell", ID: 100 + i, Available: true})
	}
	p := cooldownPixels(input)
	values := []int{11, 600, 1, 2, 125, 300, 1, 1}
	for i := 0; i < 16; i++ {
		for f, v := range values {
			for c := 0; c < 2; c++ {
				x, y := OverlayCooldownPoint(i, f, c)
				paintOverlay(&p, x, y+32, 0x100000+v)
			}
		}
	}
	s := DecodeOverlayPixels(p, 0, 32)
	if s.Error != "" {
		t.Fatal(s.Error)
	}
	for _, c := range s.Overlays.Cooldowns {
		if c.Remaining == nil || *c.Remaining != 1.1 || c.Charges == nil || *c.Charges != 1 || c.Recharge == nil || *c.Recharge != 12.5 || c.Enabled == nil || !*c.Enabled {
			t.Fatalf("bad measurement: %+v", c)
		}
	}
	if s.Overlays.Health == nil || *s.Overlays.Health != 90 {
		t.Fatal("cooldown tiles overlap existing health")
	}
	for _, bad := range []int{0, 0xffffff, 0x1fffff, 0x100000 + 12} {
		x, y := OverlayCooldownPoint(0, 0, 1)
		paintOverlay(&p, x, y+32, bad)
		if DecodeOverlayPixels(p, 0, 32).Overlays.Cooldowns[0].Remaining != nil {
			t.Fatal("mismatched/occluded sample accepted", bad)
		}
	}
	for c := 0; c < 2; c++ {
		x, y := OverlayCooldownPoint(0, 0, c)
		paintOverlay(&p, x, y+32, 0)
	}
	if DecodeOverlayPixels(p, 0, 32).Overlays.Cooldowns[0].Remaining != nil {
		t.Fatal("black obstruction became ready")
	}
	for c := 0; c < 2; c++ {
		x, y := OverlayCooldownPoint(0, 0, c)
		paintOverlay(&p, x, y+32, 0x100000)
	}
	if v := DecodeOverlayPixels(p, 0, 32).Overlays.Cooldowns[0].Remaining; v == nil || *v != 0 {
		t.Fatal("marked zero not ready")
	}
	// JSON cannot provide its own trusted cooldown or readiness state.
	z := 0.0
	yes := true
	input.Native = false
	input.Cooldowns[0].Remaining = &z
	input.Cooldowns[0].Enabled = &yes
	s = DecodeOverlayPixels(cooldownPixels(input), 0, 32)
	if s.Overlays.Cooldowns[0].Remaining != nil || s.Overlays.Cooldowns[0].Enabled != nil {
		t.Fatal("forged metadata accepted")
	}
}

func TestCooldownMappingBounds(t *testing.T) {
	occupied := map[[2]int]bool{}
	for i := 0; i < 16; i++ {
		for f := 0; f < 8; f++ {
			for c := 0; c < 2; c++ {
				x, y := OverlayCooldownPoint(i, f, c)
				key := [2]int{x, y}
				if occupied[key] || x < 0 || x > 252 || y < 0 || y > 36 || y%4 != 0 || x%4 != 0 {
					t.Fatal("invalid geometry", key)
				}
				occupied[key] = true
			}
		}
	}
	for _, w := range []struct {
		k  string
		id int
	}{{"slot", 20}, {"spell", 0}, {"item", 10000000}, {"gcd", 1}, {"invalid", 123}} {
		if ValidCooldownWatch(w.k, w.id) {
			t.Fatal("invalid source accepted", w)
		}
	}
}
