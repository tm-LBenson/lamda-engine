package lookup

import (
	"fmt"
	"hash/crc32"
	"image"
	"testing"
)

func borderFixture(body string, values []int64) (Pixels, []image.Point) {
	return meterLayoutFixture(body, values, false)
}
func meterLayoutFixture(body string, values []int64, strip bool) (Pixels, []image.Point) {
	tiles, _ := MeterBorderTiles(len(body), len(values))
	layout, _ := PlanMeterBorder(380, 220, 8, tiles, [4]int{0, 0, 1, 0}, 1920, 1080)
	if strip {
		layout, _ = PlanMeterStrip(800, tiles, 1920, 1080)
	}
	p := Pixels{Width: layout.Width(), Height: layout.Height(), BGRA: make([]byte, layout.Width()*layout.Height()*4)}
	slots := layout.Slots()[:tiles]
	index := 0
	paintCells := func(data []byte) {
		padded := make([]byte, 36)
		copy(padded, data)
		data = padded
		at := slots[index]
		index++
		for i := 0; i < 12; i++ {
			var r, g, b byte
			if i*3+2 < len(data) {
				r, g, b = data[i*3], data[i*3+1], data[i*3+2]
			}
			for y := 0; y < 2; y++ {
				for x := 0; x < 2; x++ {
					off := ((at.Y+(i/3)*2+y)*p.Width + at.X + (i%3)*2 + x) * 4
					p.BGRA[off], p.BGRA[off+1], p.BGRA[off+2] = b, g, r
				}
			}
		}
	}
	paintGlyph := func(char byte) {
		at := slots[index]
		index++
		value := 9
		marker := char != 'X'
		if marker {
			if char >= '0' && char <= '9' {
				value = int(char - '0')
			} else {
				value = int(char-'a') + 10
			}
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 6; x++ {
				on := marker && x < 2
				if x >= 2 {
					bit := (y/2)*2 + (x-2)/2
					on = value&(1<<bit) != 0
				}
				if on {
					off := ((at.Y+2+y)*p.Width + at.X + 1 + x) * 4
					p.BGRA[off], p.BGRA[off+1], p.BGRA[off+2] = 255, 255, 255
				}
			}
		}
	}
	h := MeterBorderHeader{Layout: layout, BodyBytes: len(body), Serial: 7, Tiles: tiles, BodyCRC: crc32.ChecksumIEEE([]byte(body))}
	paintCells(h.Encode())
	for i := 0; i < len(body); i += 36 {
		end := min(i+36, len(body))
		paintCells([]byte(body[i:end]))
	}
	for _, char := range []byte("0123456789abcdefX") {
		paintGlyph(char)
	}
	for row := 0; row <= len(values); row++ {
		header := []byte{173, byte(row), 118, 0, 0, 7, 59, 0, 93, 205, 127, 61}
		for i := 0; i < 7; i++ {
			header[7] += header[i] * byte(i+1)
		}
		paintCells(header)
		text := fmt.Sprintf("%015dX", 12)
		if row > 0 {
			text = fmt.Sprintf("%015dX", values[row-1])
		}
		for copy := 0; copy < 2; copy++ {
			for _, char := range []byte(text) {
				paintGlyph(char)
			}
		}
		if row > 0 {
			mask := 3
			if row > 1 {
				mask = 12
			}
			raw := make([]byte, 36)
			for bit := 0; bit < 4; bit++ {
				color := []byte{160, 32, 96}
				if mask&(1<<bit) != 0 {
					color = []byte{24, 224, 96}
				}
				copy(raw[bit*9:bit*9+3], color)
				copy(raw[bit*9+3:bit*9+6], color)
			}
			paintCells(raw)
		}
	}
	if index != len(slots) {
		panic("fixture tile count")
	}
	return p, slots
}
func TestCompactBorderRoundTripAndDuplicateGuard(t *testing.T) {
	body := "2|border:123:1|7|69|Player-1-Aug|1473|1|6000|32|12345|296|328|380|220|overall|0\n1|Player-1-Aug|13|Aug|15\n2|Player-1-Ally|8|Ally|15"
	p, slots := borderFixture(body, []int64{4294967296, 999999999999999})
	sample := DecodeMeterBorder(p)
	n := sample.NativeMeter
	if sample.Error != "" || n == nil || !n.Captured || n.Frame.Scope != "overall" || n.Members[0].Total != 4294967296 || n.Members[1].Total != 999999999999999 || n.Members[0].AuraMask != 3 || n.Members[1].AuraMask != 12 {
		t.Fatal(sample.Error, n)
	}
	// Copy a different but valid digit into just one repeated reading. Valid
	// glyphs with contradictory counters must not become a trusted total.
	meta := (len(body) + 35) / 36
	start := 1 + meta + 17 + 33 + 1
	from, to := slots[1+meta+1], slots[start]
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			a := ((from.Y+y)*p.Width + from.X + x) * 4
			b := ((to.Y+y)*p.Width + to.X + x) * 4
			copy(p.BGRA[b:b+4], p.BGRA[a:a+4])
		}
	}
	bad := DecodeMeterBorder(p)
	if bad.NativeMeter == nil || bad.NativeMeter.Captured || bad.NativeMeter.Error != "meter_border_duplicate_mismatch" {
		t.Fatal(bad)
	}
}

func TestStandaloneStripUsesSameValidatedCountersWithoutAnInnerWindow(t *testing.T) {
	body := "4|strip:123:1|7|69|Player-1-Aug|1473|1|6000|32|12345|288|344|380|220|boss|0|0|0|0|0|0\n1|Player-1-Aug|13|Aug|15\n2|Player-1-Ally|8|Ally|15"
	p, _ := meterLayoutFixture(body, []int64{4294967296, 999999999999999}, true)
	n := DecodeMeterBorder(p).NativeMeter
	if n == nil || !n.Captured || !n.Frame.Standalone || n.Members[0].Total != 4294967296 || n.Members[1].Total != 999999999999999 || n.Members[0].AuraMask != 3 {
		t.Fatal(n)
	}
	if p.Width != 800 || p.Height >= 40 {
		t.Fatal(p.Width, p.Height)
	}
	pointer := MeterBorderPointer{X: 288, Y: 320, Width: p.Width, Height: p.Height}
	if got, err := ParseMeterBorderPointer(pointer.Encode()); err != nil || got != pointer {
		t.Fatal(got, err)
	}
	for _, members := range []int{1, 5, 40} {
		tiles, _ := MeterBorderTiles(100+members*110, members)
		layout, err := PlanMeterStrip(800, tiles, 1920, 1080)
		if err != nil || !layout.Standalone() || len(layout.Slots()) < tiles || layout.Width() != 800 {
			t.Fatal(layout, err)
		}
	}
	if _, err := PlanMeterStrip(319, 100, 1920, 1080); err == nil {
		t.Fatal("accepted too narrow strip")
	}
	if _, err := PlanMeterStrip(320, 4096, 1920, 100); err == nil {
		t.Fatal("accepted overflowing strip")
	}
}

func TestMeterBorderThinDefaultAndBottomOverflow(t *testing.T) {
	for _, group := range []struct{ members, bytes int }{{1, 160}, {5, 550}, {40, 5500}} {
		tiles, err := MeterBorderTiles(group.bytes, group.members)
		if err != nil {
			t.Fatal(err)
		}
		b, err := PlanMeterBorder(380, 220, 8, tiles, [4]int{0, 0, 1, 0}, 1920, 1080)
		if err != nil {
			t.Fatal(err)
		}
		if b.Edges[0] != 8 || b.Edges[1] != 8 || b.Edges[3] != 8 || len(b.Slots()) < tiles {
			t.Fatal(b)
		}
		if group.members == 40 && b.Edges[2] <= 8 {
			t.Fatal("raid did not overflow below")
		}
		seen := map[image.Point]bool{}
		inner := image.Rect(b.Edges[3], b.Edges[0], b.Edges[3]+380, b.Edges[0]+220)
		for _, p := range b.Slots() {
			r := image.Rectangle{Min: p, Max: p.Add(image.Pt(8, 8))}
			if seen[p] || !r.In(image.Rect(0, 0, b.Width(), b.Height())) || r.Overlaps(inner) {
				t.Fatal("invalid slot", p, b)
			}
			seen[p] = true
		}
	}
}

func TestBorderNumericGlyphsSurviveNativePixelRounding(t *testing.T) {
	body := "2|border:123:1|7|69|Player-1-Aug|1473|1|6000|32|12345|296|328|380|220|boss|0\n1|Player-1-Aug|13|Aug|15"
	original, slots := borderFixture(body, []int64{123456789098765})
	first := 1 + (len(body)+35)/36 + 17 + 1
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			p := original
			p.BGRA = append([]byte(nil), original.BGRA...)
			for row := 0; row < 2; row++ {
				for copy := 0; copy < 2; copy++ {
					for char := 0; char < 16; char++ {
						at := slots[first+row*33+copy*16+char]
						for y := 0; y < 8; y++ {
							for x := 0; x < 8; x++ {
								to := ((at.Y+y)*p.Width + at.X + x) * 4
								clear(p.BGRA[to : to+4])
							}
						}
						mx, my := dx, dy
						if copy == 1 {
							mx, my = -dx, -dy
						}
						for y := 0; y < 8; y++ {
							for x := 0; x < 8; x++ {
								if x+mx < 0 || x+mx >= 8 || y+my < 0 || y+my >= 8 {
									continue
								}
								from := ((at.Y+y)*p.Width + at.X + x) * 4
								to := ((at.Y+y+my)*p.Width + at.X + x + mx) * 4
								copyBytes := original.BGRA[from : from+4]
								for i, b := range copyBytes {
									p.BGRA[to+i] = b
								}
							}
						}
					}
				}
			}
			n := DecodeMeterBorder(p).NativeMeter
			if n == nil || !n.Captured || n.Members[0].Total != 123456789098765 {
				t.Fatalf("phase %d,%d: %+v", dx, dy, n)
			}
		}
	}
}
func TestMeterBorderWeightedDistributionAndBounds(t *testing.T) {
	b, err := PlanMeterBorder(380, 220, 8, 1500, [4]int{1, 1, 4, 1}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}
	if b.Edges[2] <= b.Edges[0] || b.Edges[0] <= 8 || b.Edges[1] <= 8 || b.Edges[3] <= 8 {
		t.Fatal(b)
	}
	if _, err := PlanMeterBorder(380, 1000, 8, 4000, [4]int{0, 0, 1, 0}, 1920, 1080); err == nil {
		t.Fatal("off-screen overflow accepted")
	}
}
func TestMeterBorderPublicHeaderRejectsCorruption(t *testing.T) {
	b, _ := PlanMeterBorder(380, 220, 8, 220, [4]int{0, 0, 1, 0}, 1920, 1080)
	want := MeterBorderHeader{Layout: b, BodyBytes: 550, Serial: 125, Tiles: 220, BodyCRC: crc32.ChecksumIEEE([]byte("public metadata"))}
	p := want.Encode()
	got, err := ParseMeterBorderHeader(p)
	if err != nil || got != want {
		t.Fatal(got, err)
	}
	p[12] ^= 1
	if _, err := ParseMeterBorderHeader(p); err == nil {
		t.Fatal("corrupt geometry accepted")
	}
}

func TestMeterPointerDistinguishesCapturePausedFromExplicitHide(t *testing.T) {
	for _, flag := range []int{0, 1, 2} {
		p := MeterBorderPointer{Width: 396, Height: 236, Inactive: flag == 1, Paused: flag == 2}
		decoded, err := ParseMeterBorderPointer(p.Encode())
		if err != nil || decoded != p {
			t.Fatalf("flag %d: %+v %v", flag, decoded, err)
		}
	}
}
