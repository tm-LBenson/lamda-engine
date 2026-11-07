package lookup

import (
	"image/png"
	"os"
	"testing"
)

// Explicit screenshot fixture: reads pixels only, never locates or controls WoW.
func TestNativeMonoCaptureImage(t *testing.T) {
	path := os.Getenv("LAMDA_MONO_IMAGE")
	if path == "" {
		t.Skip("native mono fixture not supplied")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	im, e := png.Decode(f)
	if e != nil {
		t.Fatal(e)
	}
	b := im.Bounds()
	p := Pixels{Width: b.Dx(), Height: b.Dy(), BGRA: make([]byte, b.Dx()*b.Dy()*4)}
	for y := 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x++ {
			r, g, bl, a := im.At(x, y).RGBA()
			i := (y*p.Width + x) * 4
			p.BGRA[i], p.BGRA[i+1], p.BGRA[i+2], p.BGRA[i+3] = byte(bl>>8), byte(g>>8), byte(r>>8), byte(a>>8)
		}
	}
	cal, ok := monoDecimalCalibration(p, 608, 704)
	if !ok {
		t.Fatal("public native Roboto decimal calibration")
	}
	for i := 0; i < 17; i++ {
		var bits uint64
		for yy := 0; yy < 8; yy++ {
			for xx := 0; xx < 5; xx++ {
				if p.ink(608+i*5+xx, 704+yy) {
					bits |= 1 << uint(yy*5+xx)
				}
			}
		}
		t.Logf("glyph%d=0x%010x", i, bits)
	}
	for _, value := range []string{"000000000000000", "000004294967296", "123456789012345", "999999999999999"} {
		gs := make([]compactGlyph, 16)
		for i, c := range value {
			gs[i] = cal[int(c-'0')]
		}
		gs[15] = cal[16]
		if _, yes := monoDecimalValue(gs, cal); !yes {
			t.Fatal(value)
		}
	}
}
