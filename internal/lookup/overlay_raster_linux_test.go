//go:build linux

package lookup

import "testing"

func protonRaster(t *testing.T) Pixels {
	a := denseInput(false)
	a.StripWidth = 256
	p := densePixels(t, a)
	// A neighbour's differently colored glyph bleeds into the right column.
	for y := 32; y < 72; y++ {
		for x := 0; x < 256; x += 2 {
			i := (y*256 + x + 1) * 4
			p.BGRA[i], p.BGRA[i+1], p.BGRA[i+2] = 71, 29, 93
		}
	}
	return p
}

func TestLinuxProtonClipKeepsPairedMeasurements(t *testing.T) {
	p := protonRaster(t)
	if DecodeOverlayShape(p).Error != "overlays_calibration" {
		t.Fatal("fixture did not reproduce compact clipping")
	}
	s := decodeLinuxOverlayShape(p)
	if s.Error != "" || s.Overlays == nil || s.Overlays.Health == nil || *s.Overlays.Health != 90 || s.Overlays.Cooldowns[0].Remaining == nil || *s.Overlays.Cooldowns[0].Remaining != 5 {
		t.Fatalf("decode: %#v", s)
	}
	// Independent copies still need to agree. No damaged value becomes ready.
	x, y := overlayTimerPoint(0, 0, 1, 2)
	i := ((y+32)*p.Width + x) * 4
	p.BGRA[i] ^= 1
	s = decodeLinuxOverlayShape(p)
	if s.Error != "" || s.Overlays.Cooldowns[0].Remaining != nil {
		t.Fatal("disagreeing copies accepted")
	}
}

func TestLinuxProtonClipRejectsFailedCalibrationAndMetadata(t *testing.T) {
	for _, index := range []int{0, 256 * 4} {
		p := protonRaster(t)
		p.BGRA[32*256*4+index] ^= 1
		if decodeLinuxOverlayShape(p).Error != "overlays_calibration" {
			t.Fatal("bad calibration accepted")
		}
	}
	p := protonRaster(t)
	p.BGRA[(p.Width+1)*4+2] ^= 42
	if decodeLinuxOverlayShape(p).Error == "" {
		t.Fatal("corrupt metadata accepted")
	}
}
