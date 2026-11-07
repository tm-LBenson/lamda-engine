package lookup

import (
	"image/png"
	"os"
	"testing"
)

func TestNativeColorCaptureImage(t *testing.T) {
	path := os.Getenv("LAMDA_COLOR_IMAGE")
	if path == "" {
		t.Skip("public native color fixture not supplied")
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
	cal, ok := meterColorCalibration(p, 400, 707, 6)
	if !ok {
		t.Fatal("native V3 4-row color alphabet")
	}
	for i, g := range cal {
		var bits uint64
		for j, on := range g {
			if on {
				bits |= 1 << j
			}
		}
		if bits != nativeColorMasks[i] {
			t.Fatalf("native glyph%d: %012x", i, bits)
		}
	}
}
