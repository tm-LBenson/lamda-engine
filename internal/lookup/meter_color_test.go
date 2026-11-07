package lookup

import (
	"fmt"
	"hash/crc32"
	"image"
	"strings"
	"testing"
)

// Exact public V3 glyphs from the running client, 2026-10-29. Cyan, four
// physical rows. These retain no player data or complete UI screenshots.
var nativeColorMasks = []uint64{0x0c30c3, 0x0c33cf, 0x0c3cf3, 0x0c3fff, 0x3cf0c3, 0x3cf3cf, 0x3cfcf3, 0x3cffff, 0xcf30c3, 0xcf33cf, 0xcf3cf3, 0xcf3fff, 0xfff0c3, 0xfff3cf, 0xfffcf3, 0xffffff, 0xc3030c}

func paintNativeColor(p Pixels, x, y int, text string) {
	for i, c := range text {
		id := strings.IndexRune("0123456789abcdefX", c)
		if id < 0 {
			panic("fixture alphabet")
		}
		for yy := 0; yy < 4; yy++ {
			for xx := 0; xx < 6; xx++ {
				color := 0
				if nativeColorMasks[id]&(1<<uint(yy*6+xx)) != 0 {
					color = 0x20e0ff
				}
				paintCastColor(p, x+i*6+xx, y+yy, 1, color)
			}
		}
	}
}
func meterColorFixture(rows int, primary bool, aura ...int) (Pixels, MeterBorderHeader) {
	old, h := meterViewFixture(rows, primary)
	var data []byte
	slots := h.Layout.Slots()
	for i := 1; i <= (h.BodyBytes+35)/36; i++ {
		at := slots[i]
		data = append(data, old.Cells(image.Rect(at.X, at.Y, at.X+6, at.Y+8))...)
	}
	data = data[:h.BodyBytes]
	data[0] = '9'
	if len(aura) > 0 {
		lines := strings.Split(string(data), "\n")
		lines[1] = strings.TrimSuffix(lines[1], "|0") + "|15"
		data = []byte(strings.Join(lines, "\n"))
		h.BodyBytes = len(data)
	}
	masks := 0
	if !primary {
		masks = 1
	}
	layout := planMeterViewDensity(len(data), rows, primary, masks, 832, 6)
	h.Layout = layout
	h.Tiles = layout.capacity()
	h.BodyCRC = crc32.ChecksumIEEE(data)
	p := Pixels{Width: layout.Width(), Height: layout.Height(), BGRA: make([]byte, layout.Width()*layout.Height()*4)}
	cell := func(at image.Point, b []byte, columns, count int) {
		for i := 0; i < count; i++ {
			rgb := 0
			for j := 0; j < 3; j++ {
				rgb <<= 8
				if i*3+j < len(b) {
					rgb |= int(b[i*3+j])
				}
			}
			paintCastColor(p, at.X+i%columns*2, at.Y+i/columns*2, 2, rgb)
		}
	}
	slots = layout.Slots()
	cell(slots[0], h.Encode(), 3, 12)
	for i := 0; i < len(data); i += 36 {
		cell(slots[1+i/36], data[i:min(i+36, len(data))], 3, 12)
	}
	points, media, ok := meterViewPositionsDensity(layout, len(data), rows, primary, masks, 6)
	if !ok {
		panic("fixture geometry")
	}
	at := points[0]
	paintNativeColor(p, at.X+6, at.Y, "0123456789abcdefX")
	count := 3 + rows
	if !primary {
		count += rows
	}
	for row := 0; row < count; row++ {
		at = points[row+1]
		tag := []byte{173, byte(row), 118, 0, 0, 7, 59, 0, 93, 205, 127, 61}
		for i := 0; i < 7; i++ {
			tag[7] += tag[i] * byte(i+1)
		}
		cell(at, tag, 2, 4)
		n := int64(418240)
		if row == 0 {
			n = 12
		}
		if primary && row == 1 {
			n = int64(rows) * 418240
		}
		if !primary && row >= 3 && row%2 == 0 {
			n = 16141
		}
		text := fmt.Sprintf("%015dX", n)
		paintNativeColor(p, at.X+6, at.Y, text)
		paintNativeColor(p, at.X+104, at.Y, text)
	}
	if len(aura) > 0 {
		at := points[4]
		for bit := 0; bit < 4; bit++ {
			color := 0xa02060
			if aura[0]&(1<<bit) != 0 {
				color = 0x18e060
			}
			paintCastColor(p, at.X+200+bit%2*4, at.Y+bit/2*2, 2, color)
			paintCastColor(p, at.X+202+bit%2*4, at.Y+bit/2*2, 2, color)
		}
	}
	paintCastColor(p, 0, media, 2, 0x33937f)
	if masks > 0 {
		names := media + (rows+p.Width/16-1)/(p.Width/16)*16
		paintCastColor(p, 0, names, 2, 0xffffff)
	}
	return p, h
}
func TestCompactColorNativeViews(t *testing.T) {
	for _, primary := range []bool{false, true} {
		for _, rows := range []int{1, 5, 25, 40} {
			p, h := meterColorFixture(rows, primary)
			n := DecodeMeterBorder(p).NativeMeter
			if n == nil || !n.Captured || !n.View.ColorCompact || n.View.Rows[0].Total != 418240 {
				t.Fatalf("rows%d primary%v: %+v", rows, primary, n)
			}
			old := planMeterView(h.BodyBytes, rows, primary, map[bool]int{true: 0, false: 1}[primary], p.Width)
			if h.Layout.Height() > old.Height() {
				t.Fatal("compact layout grew")
			}
		}
	}
}
func TestCompactColorRejectsCorruptCopiesTagsAndCalibration(t *testing.T) {
	for _, target := range []string{"copy", "tag", "calibration", "sum"} {
		p, h := meterColorFixture(5, true)
		pts, _, _ := meterViewPositionsDensity(h.Layout, h.BodyBytes, 5, true, 0, 6)
		at := pts[4]
		switch target {
		case "copy":
			paintNativeColor(p, at.X+104, at.Y, "000000000418241X")
		case "tag":
			paintCastColor(p, at.X, at.Y, 2, 0)
		case "calibration":
			at = pts[0]
			for y := 0; y < 4; y++ {
				for x := 0; x < 6; x++ {
					r, g, b := p.rgb(at.X+6+x, at.Y+y)
					paintCastColor(p, at.X+12+x, at.Y+y, 1, int(r)<<16|int(g)<<8|int(b))
				}
			}
		case "sum":
			paintNativeColor(p, at.X+6, at.Y, "000000000418241X")
			paintNativeColor(p, at.X+104, at.Y, "000000000418241X")
		}
		n := DecodeMeterBorder(p).NativeMeter
		if n == nil || n.Captured {
			t.Fatalf("%s accepted", target)
		}
	}
}
func TestCompactColorDecimalRangeAndExactSentinel(t *testing.T) {
	p := Pixels{Width: 208, Height: 8, BGRA: make([]byte, 208*8*4)}
	paintNativeColor(p, 6, 0, "0123456789abcdefX")
	cal, ok := meterColorCalibration(p, 6, 0, 6)
	if !ok {
		t.Fatal("native color calibration")
	}
	for _, text := range []string{"000000000000000X", "000004294967296X", "123456789012345X", "999999999999999X"} {
		var gs []compactGlyph
		var want int64
		for i, c := range text {
			id := strings.IndexRune("0123456789abcdefX", c)
			gs = append(gs, cal[id])
			if i < 15 {
				want = want*10 + int64(id)
			}
		}
		got, ok := monoDecimalValue(gs, cal)
		if !ok || got != want {
			t.Fatal(text, got, ok)
		}
		gs[15] = cal[0]
		if _, ok = monoDecimalValue(gs, cal); ok {
			t.Fatal("invalid sentinel")
		}
	}
}

func TestCompactColorAugAurasUseTwoByTwoSlotsAndDuplicatePixels(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		p, h := meterColorFixture(5, true, mask)
		n := DecodeMeterBorder(p).NativeMeter
		if n == nil || !n.Captured || n.Members[0].AuraMask != mask {
			t.Fatalf("mask%d: %+v", mask, n)
		}
		pts, _, _ := meterViewPositionsDensity(h.Layout, h.BodyBytes, 5, true, 0, 6)
		at := pts[4]
		paintCastColor(p, at.X+202, at.Y, 2, 0)
		n = DecodeMeterBorder(p).NativeMeter
		if n == nil || n.Captured || n.Error != "meter_view_aura_mismatch" {
			t.Fatalf("inconsistent aura accepted: %+v", n)
		}
	}
}
