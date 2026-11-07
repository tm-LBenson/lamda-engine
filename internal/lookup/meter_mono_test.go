package lookup

import "testing"

// Original public alphabet captured from WoW's bundled Roboto Mono Bold at
// 8px, on 2026-10-29. Only these known glyph masks are retained, not game UI.
var nativeMonoMasks = []uint64{0x07294a5100, 0x0210843000, 0x0708884100, 0x0721084100, 0x04394a6000, 0x0721061380, 0x0721461000, 0x0110804380, 0x07214a5100, 0x0321c85100, 0x07290a0000, 0x07294e1080, 0x07080e0000, 0x07290e4200, 0x07094e0000, 0x0210862300, 0x0528845000}

func paintNativeMono(p Pixels, x, y int, text string) {
	for i, c := range text {
		id := 16
		if c >= '0' && c <= '9' {
			id = int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			id = 10 + int(c-'a')
		}
		for yy := 0; yy < 8; yy++ {
			for xx := 0; xx < 5; xx++ {
				if nativeMonoMasks[id]&(1<<uint(yy*5+xx)) != 0 {
					paintCastColor(p, x+i*5+xx, y+yy, 1, 0xffffff)
				}
			}
		}
	}
}

func TestNativeMonospaceViewUsesActualGlyphsWithoutCustomFont(t *testing.T) {
	for _, primary := range []bool{false, true} {
		for _, rows := range []int{1, 5, 25, 40} {
			p, _ := meterViewFixtureFont(rows, primary, true)
			n := DecodeMeterBorder(p).NativeMeter
			if n == nil || !n.Captured || n.Pitch != 5 || n.View.Rows[0].Total != 418240 {
				t.Fatalf("rows%d primary%v: %+v", rows, primary, n)
			}
		}
	}
}

func TestNativeMonospaceRejectsChangedCopyAndAmbiguousCalibration(t *testing.T) {
	p, h := meterViewFixtureFont(5, true, true)
	points, _, _ := meterViewPositions(h.Layout, h.BodyBytes, 5, true, 0)
	at := points[4]
	// Alter only the first reading. The second copy must still veto the frame.
	color := 0xffffff
	if p.ink(at.X+8, at.Y+2) {
		color = 0
	}
	paintCastColor(p, at.X+8, at.Y+2, 1, color)
	n := DecodeMeterBorder(p).NativeMeter
	if n == nil || n.Captured {
		t.Fatal("changed duplicate accepted")
	}
	p, h = meterViewFixtureFont(5, true, true)
	at = fastMeterOrigin(h.Layout, h.BodyBytes, 0)
	for yy := 0; yy < 8; yy++ {
		for xx := 0; xx < 5; xx++ {
			color := 0
			if p.ink(at.X+8+xx, at.Y+yy) {
				color = 0xffffff
			}
			paintCastColor(p, at.X+8+5+xx, at.Y+yy, 1, color)
		}
	}
	n = DecodeMeterBorder(p).NativeMeter
	if n == nil || n.Captured {
		t.Fatal("ambiguous alphabet accepted")
	}
}

func TestNativeMonospaceExactDecimalLimitsAndSentinel(t *testing.T) {
	p := Pixels{Width: 208, Height: 24, BGRA: make([]byte, 208*24*4)}
	paintNativeMono(p, 8, 0, "0123456789abcdefX")
	cal, ok := monoDecimalCalibration(p, 8, 0)
	if !ok {
		t.Fatal("native alphabet")
	}
	for _, text := range []string{"000000000000000X", "000004294967296X", "123456789012345X", "999999999999999X"} {
		gs := make([]compactGlyph, 16)
		var want int64
		for i, c := range text {
			if i == 15 {
				gs[i] = cal[16]
			} else {
				gs[i] = cal[int(c-'0')]
				want = want*10 + int64(c-'0')
			}
		}
		got, yes := monoDecimalValue(gs, cal)
		if !yes || got != want {
			t.Fatal(text, got, yes)
		}
		gs[15] = cal[0]
		if _, yes = monoDecimalValue(gs, cal); yes {
			t.Fatal("missing sentinel")
		}
	}
}
