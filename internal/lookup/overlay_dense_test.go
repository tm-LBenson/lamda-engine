package lookup

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"testing"
)

func denseInput(casts bool) OverlayInput {
	a := OverlayInput{NativeVersion: 2, Epoch: "dense", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, ResourcesVersion: 1, ResourceMods: map[string]float64{}, AuraDurationsVersion: 1, CooldownsVersion: 1, CooldownHash: "1234567812345678", CastsVersion: 1, CastHash: "1234567812345678", MeasurementsVersion: 1, MeasurementHash: "1234567812345678"}
	for i := 0; i < 64; i++ {
		a.Watches = append(a.Watches, OverlayAura{Spell: 1000 + i, Kind: "buff", Unit: "player", Available: true, DurationAvailable: true})
		if !casts {
			a.Cooldowns = append(a.Cooldowns, OverlayCooldown{Kind: "spell", ID: 2000 + i, Available: true})
		}
		kind, resource := "health", ""
		if i >= 32 {
			kind, resource = "resource", "0"
		}
		a.Measurements = append(a.Measurements, OverlayMeasurement{Unit: fmt.Sprintf("raid%d", i%32+1), Kind: kind, Resource: resource, Power: 0, Mod: 1, Available: true, Generation: 1})
	}
	for i := 0; i < 32; i++ {
		a.ResourceMods[fmt.Sprint(i)] = 1
		if casts {
			a.Casts = append(a.Casts, OverlayCast{Unit: fmt.Sprintf("raid%d", i+1), Generation: 3, Available: true, Seq: 1, Events: []OverlayCastEvent{{Seq: 1, Kind: "caststart", Bar: 0, Age: 0}}})
		}
	}
	return a
}
func paintDense(p *Pixels, x, y, value int) {
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			i := ((y+dy)*p.Width + x + dx) * 4
			p.BGRA[i] = byte(value)
			p.BGRA[i+1] = byte(value >> 8)
			p.BGRA[i+2] = byte(value >> 16)
		}
	}
}
func densePixels(t *testing.T, a OverlayInput) Pixels {
	t.Helper()
	p := Pixels{Width: 256, Height: 72, BGRA: make([]byte, 18432*4)}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 6134 {
		b = compressedMetadata(t, b)
	}
	if len(b) > 6134 {
		t.Fatal("dense metadata exceeds packet", len(b))
	}
	packet := make([]byte, SimBridgeBytes)
	copy(packet, "LUO1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(b)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(b))
	copy(packet[10:], b)
	for i := 0; i < len(packet)/3; i++ {
		paintDense(&p, (i%128)*2, (i/128)*2, int(packet[i*3])<<16|int(packet[i*3+1])<<8|int(packet[i*3+2]))
	}
	for i, v := range OverlayCalibration {
		x, y := overlayCalibrationPoint(i, a.NativeVersion)
		paintDense(&p, x, 32+y, v)
	}
	for copy := 0; copy < 2; copy++ {
		x, y := overlayHealthPoint(copy, a.NativeVersion)
		paintDense(&p, x, 32+y, 90)
	}
	for power := 0; power < 32; power++ {
		for field, v := range []int{10, 10, 100, 100} {
			x, y := overlayResourcePoint(power, field, a.NativeVersion)
			paintDense(&p, x, 32+y, v)
		}
	}
	for i := range a.Watches {
		for copy := 0; copy < 2; copy++ {
			for field, v := range []int{0x37d6a0, i%3 + 1, 40, 0x100000 + 120} {
				x, y := overlayAuraPoint(i, field, copy, a.NativeVersion)
				paintDense(&p, x, 32+y, v)
			}
		}
	}
	for i := range a.Cooldowns {
		for field, v := range []int{50, 200, 1, 2, 50, 200, 1, 1} {
			for copy := 0; copy < 2; copy++ {
				x, y := overlayTimerPoint(i, field, copy, a.NativeVersion)
				paintDense(&p, x, 32+y, 0x100000+v)
			}
		}
	}
	for i := range a.Casts {
		for field, v := range []int{0x100000 + 50, 0x100000 + 200, 2000 + i, 0x100001, 0x100001, 3, 0, 0, 2000 + i, 1, 0, 0, 0, 0, 0, 0} {
			for copy := 0; copy < 2; copy++ {
				x, y := overlayTimerPoint(len(a.Cooldowns)+i*2, field, copy, a.NativeVersion)
				paintDense(&p, x, 32+y, v)
			}
		}
	}
	for i := range a.Measurements {
		for copy := 0; copy < 2; copy++ {
			for field, v := range []int{0x28c6a1, 30, 100} {
				x, y := overlayMeasurementPoint(i, field, copy, a.NativeVersion)
				paintDense(&p, x, 32+y, v)
			}
		}
	}
	return p
}
func TestDenseAllSourcesWithinExistingFootprint(t *testing.T) {
	for mode := 0; mode < 5; mode++ {
		casts := mode == 1 || mode == 3 || mode == 4
		a := denseInput(casts)
		if mode >= 2 {
			a.NativeVersion = 3
		}
		if mode == 2 || mode == 4 {
			for i := 0; i < 26; i++ {
				a.Cooldowns = append(a.Cooldowns, OverlayCooldown{Kind: "spell", ID: 3000 + i, Available: true})
			}
		}
		if mode == 3 {
			for _, u := range append(OverlayScopeTokens("party"), OverlayScopeTokens("boss")...) {
				a.Casts = append(a.Casts, OverlayCast{Unit: u, Generation: 3, Available: true, Seq: 1, Events: []OverlayCastEvent{{Seq: 1, Kind: "caststart", Bar: 0, Age: 0}}})
			}
		}
		p := densePixels(t, a)
		s := DecodeOverlayPixels(p, 0, 32)
		if s.Error != "" || s.Overlays == nil {
			t.Fatal(casts, s.Error)
		}
		got := s.Overlays
		if got.Health == nil || *got.Health != 90 || len(got.Resources) != 32 {
			t.Fatal("player readings lost")
		}
		for i, w := range got.Watches {
			if w.Active == nil || !*w.Active || w.Stacks == nil || *w.Stacks != float64(i%3+1) || w.Remaining == nil || *w.Remaining != 4 || w.Duration == nil || *w.Duration != 12 {
				t.Fatal("aura", i, w)
			}
		}
		for i, c := range got.Cooldowns {
			if c.Charges == nil || *c.Charges != 1 || c.Remaining == nil || *c.Remaining != 5 || c.Enabled == nil || !*c.Enabled {
				t.Fatal("cooldown", i, c)
			}
		}
		for i, c := range got.Casts {
			if c.Mode == nil || *c.Mode != 1 || c.Spell == nil || *c.Spell != 2000+i || c.Remaining == nil || *c.Remaining != 5 || c.Events[0].Spell == nil || *c.Events[0].Spell != 2000+i {
				t.Fatal("cast", i, c)
			}
		}
		for i, m := range got.Measurements {
			if m.Current == nil || *m.Current != 30 || m.Max == nil || *m.Max != 100 {
				t.Fatal("measurement", i, m)
			}
		}
		for width := 16; width <= 1152; width += 4 {
			if !ValidOverlayWidth(width) {
				continue
			}
			a.StripWidth = width
			p = densePixels(t, a)
			out := Pixels{Width: width, Height: 18432 / width, BGRA: make([]byte, 18432*4)}
			for tile := 0; tile < 1152; tile++ {
				for y := 0; y < 4; y++ {
					from := ((tile/64*4+y)*256 + tile%64*4) * 4
					to := ((tile/(width/4)*4+y)*width + tile%(width/4)*4) * 4
					copy(out.BGRA[to:to+16], p.BGRA[from:from+16])
				}
			}
			s := DecodeOverlayShape(out)
			if s.Error != "" || s.Overlays.Watches[63].Stacks == nil || *s.Overlays.Watches[63].Stacks != 1 || s.Overlays.Measurements[63].Current == nil {
				t.Fatal("reshape", width, s.Error)
			}
		}
	}
}
func TestDenseRejectsPartialPairsOcclusionAndUnknownFormats(t *testing.T) {
	a := denseInput(false)
	for _, pixel := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		p := densePixels(t, a)
		x, y := overlayAuraPoint(63, 1, 1, 2)
		p.BGRA[((32+y+pixel[1])*p.Width+x+pixel[0])*4] ^= 1
		s := DecodeOverlayPixels(p, 0, 32)
		if s.Error != "" || s.Overlays.Watches[63].Stacks != nil || s.Overlays.Watches[0].Stacks == nil {
			t.Fatal("partial tile trusted", pixel)
		}
	}
	p := densePixels(t, a)
	x, y := overlayTimerPoint(63, 0, 1, 2)
	paintDense(&p, x, y+32, 0x100000)
	s := DecodeOverlayPixels(p, 0, 32)
	if s.Overlays.Cooldowns[63].Remaining != nil {
		t.Fatal("disagreeing pair trusted")
	}
	p = densePixels(t, a)
	for i := 256 * 32 * 4; i < len(p.BGRA); i++ {
		p.BGRA[i] = 0
	}
	if s = DecodeOverlayPixels(p, 0, 32); s.Error != "overlays_calibration" {
		t.Fatal("black occlusion trusted", s.Error)
	}
	for _, version := range []int{1, 4, -1} {
		bad := a
		bad.NativeVersion = version
		b, _ := json.Marshal(bad)
		if parseOverlays(compressedMetadata(t, b)).Error == "" {
			t.Fatal("unknown format", version)
		}
	}
	a.NativeVersion = 0
	b, _ := json.Marshal(a)
	if parseOverlays(compressedMetadata(t, b)).Error == "" {
		t.Fatal("large legacy metadata accepted")
	}
	a.NativeVersion = 2
	a.Watches = append(a.Watches, OverlayAura{Spell: 9999, Kind: "buff", Unit: "player"})
	b, _ = json.Marshal(a)
	if parseOverlays(compressedMetadata(t, b)).Error == "" {
		t.Fatal("source overflow accepted")
	}
}
