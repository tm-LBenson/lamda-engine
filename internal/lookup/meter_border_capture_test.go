package lookup

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
)

// Opt-in read-only diagnostic for an explicitly captured game-window fixture.
func TestMeterBorderCapturedPNG(t *testing.T) {
	path := os.Getenv("LAMDA_METER_BORDER_PNG")
	if path == "" {
		t.Skip("no captured border fixture")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	pixels := func(rect image.Rectangle) Pixels {
		p := Pixels{Width: rect.Dx(), Height: rect.Dy(), BGRA: make([]byte, rect.Dx()*rect.Dy()*4)}
		for y := 0; y < p.Height; y++ {
			for x := 0; x < p.Width; x++ {
				r, g, b, a := src.At(rect.Min.X+x, rect.Min.Y+y).RGBA()
				i := (y*p.Width + x) * 4
				p.BGRA[i] = byte(b >> 8)
				p.BGRA[i+1] = byte(g >> 8)
				p.BGRA[i+2] = byte(r >> 8)
				p.BGRA[i+3] = byte(a >> 8)
			}
		}
		return p
	}
	const x, y = 288, 320
	first := pixels(image.Rect(x, y, x+6, y+8))
	h, err := ParseMeterBorderHeader(first.Cells(image.Rect(0, 0, 6, 8)))
	if err != nil {
		t.Fatal(err)
	}
	p := pixels(image.Rect(x, y, x+h.Layout.Width(), y+h.Layout.Height()))
	s := DecodeMeterBorder(p)
	t.Logf("error=%s native=%+v", s.Error, s.NativeMeter)
	slots := h.Layout.Slots()
	start := 1 + (h.BodyBytes+35)/36
	gs := make([]compactGlyph, 17)
	for i := range gs {
		point := slots[start+i]
		gs[i], _ = compactGlyphAt(p, point.X, point.Y, 6)
		rows := ""
		for yy := 0; yy < 8; yy++ {
			for xx := 0; xx < 6; xx++ {
				if gs[i][yy*6+xx] {
					rows += "#"
				} else {
					rows += "."
				}
			}
			rows += "/"
		}
		t.Logf("cal %d at %v: %s", i, point, rows)
		for j := 0; j < i; j++ {
			if d := compactDistance(gs[i], gs[j]); d < .06 {
				t.Logf("ambiguous %d / %d distance=%v", i, j, d)
			}
		}
	}
	if s.NativeMeter == nil || !s.NativeMeter.Captured {
		for row := 0; row < 2; row++ {
			at := start + 17 + row*33 + 1
			for copy := 0; copy < 2; copy++ {
				line := ""
				for char := 0; char < 16; char++ {
					point := slots[at+copy*16+char]
					g, _ := compactGlyphAt(p, point.X, point.Y, 6)
					best, id := 1.0, -1
					for j, cal := range gs {
						if d := compactDistance(g, cal); d < best {
							best, id = d, j
						}
					}
					line += fmt.Sprintf(" %d:%.3f", id, best)
				}
				t.Logf("row%d copy%d:%s", row, copy, line)
			}
		}
		t.Fatal("captured fixture did not decode")
	}
}
