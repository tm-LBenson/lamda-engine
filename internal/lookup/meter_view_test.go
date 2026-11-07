package lookup

import (
	"fmt"
	"hash/crc32"
	"image"
	"strings"
	"testing"
)

func meterViewFixture(rows int, primary bool) (Pixels, MeterBorderHeader) {
	return meterViewFixtureFont(rows, primary, false)
}
func meterViewFixtureFont(rows int, primary bool, mono bool, extra ...string) (Pixels, MeterBorderHeader) {
	view := "spells"
	pflag := 0
	masks := 0
	if primary {
		view = "actors"
		pflag = 1
	} else {
		masks = 1
	}
	body := fmt.Sprintf("8|overlay:123:1|7|69|Player-1-Aug|1473|0|6000|32|12000|100|300|323|200|boss|0|0|0|0|0|0|3|-|0|-|-|0|-|-|1|1|1|45|18|9|880|both|1|0|Current|%s|Player-1-Aug|0|%d|%d|%d|%d", view, rows, pflag, rows, masks)
	if mono {
		body = strings.Replace(body, "|6000|32|", "|5000|32|", 1)
	}
	for i := 0; i < rows; i++ {
		key := fmt.Sprintf("row:%d", i+1)
		name := "Fire Breath"
		native := i + 1
		mask := 0
		if primary {
			key = fmt.Sprintf("Player-1-%d", i)
			if i == 0 {
				key = "Player-1-Aug"
			}
			name = "Player"
			native = 0
		} else if i == 0 {
			mask = 1
			name = "_"
		}
		body += fmt.Sprintf("\n%d|%s|13|%s|362969|0|%d|%d|0", i+1, key, name, native, mask)
	}
	body += "\n!boss|Current\n!overall|Overall"
	for _, line := range extra {
		body += "\n" + line
	}
	layout := planMeterView(len(body), rows, primary, masks, 832)
	h := MeterBorderHeader{Layout: layout, BodyBytes: len(body), Serial: 7, Tiles: layout.capacity(), BodyCRC: crc32.ChecksumIEEE([]byte(body))}
	p := Pixels{Width: layout.Width(), Height: layout.Height(), BGRA: make([]byte, layout.Width()*layout.Height()*4)}
	cell := func(at image.Point, b []byte) {
		for i := 0; i < 12; i++ {
			rgb := 0
			for j := 0; j < 3; j++ {
				rgb <<= 8
				if i*3+j < len(b) {
					rgb |= int(b[i*3+j])
				}
			}
			paintCastColor(p, at.X+i%3*2, at.Y+i/3*2, 2, rgb)
		}
	}
	slots := layout.Slots()
	cell(slots[0], h.Encode())
	for i := 0; i < len(body); i += 36 {
		cell(slots[1+i/36], []byte(body[i:min(i+36, len(body))]))
	}
	font := func(x, y int, text string) {
		if mono {
			paintNativeMono(p, x, y, text)
			return
		}
		for i, ch := range []byte(text) {
			id := 9
			if ch != 'X' {
				if ch <= '9' {
					id = int(ch - '0')
				} else {
					id = int(ch-'a') + 10
				}
			}
			for yy := 0; yy < 4; yy++ {
				for xx := 0; xx < 6; xx++ {
					ink := ch != 'X' && xx < 2
					if xx >= 2 {
						ink = id&(1<<((yy/2)*2+(xx-2)/2)) != 0
					}
					if ink {
						paintCastColor(p, x+i*6+xx, y+2+yy, 1, 0xffffff)
					}
				}
			}
		}
	}
	at := fastMeterOrigin(layout, len(body), 0)
	font(at.X+8, at.Y, "0123456789abcdefX")
	count := 3 + rows
	if !primary {
		count += rows
	}
	for row := 0; row < count; row++ {
		at := fastMeterOrigin(layout, len(body), row+1)
		tag := []byte{173, byte(row), 118, 0, 0, 7, 59, 0, 93, 205, 127, 61}
		for i := 0; i < 7; i++ {
			tag[7] += tag[i] * byte(i+1)
		}
		cell(at, tag)
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
		font(at.X+8, at.Y, text)
		font(at.X+106, at.Y, text)
	}
	media := viewMediaOrigin(layout, len(body), rows, primary)
	paintCastColor(p, 0, media, 2, 0x33937f)
	if masks > 0 {
		names := media + (rows+layout.Width()/16-1)/(layout.Width()/16)*16
		paintCastColor(p, 0, names, 2, 0xffffff)
	}
	return p, h
}
func TestMeterViewPreservesNativeSpellRatePicturesAndUnknownNames(t *testing.T) {
	p, _ := meterViewFixture(3, false)
	s := DecodeMeterBorder(p)
	n := s.NativeMeter
	if n == nil || !n.Captured || n.View == nil {
		t.Fatalf("%+v %+v", s, n)
	}
	v := n.View
	if v.Primary || v.View != "spells" || v.Rows[0].Total != 418240 || v.Rows[0].Rate != 16141 || v.Rows[0].Name != "_" || v.Rows[0].NameImage == nil || v.Rows[0].Icon == nil || len(n.Members) != 0 || len(v.Choices) != 2 {
		t.Fatal(v)
	}
	if c := v.Rows[0].NameImage.NRGBAAt(0, 0); c.A != 255 {
		t.Fatal(c)
	}
}
func TestMeterViewFullRaidAndDuplicateGuard(t *testing.T) {
	for _, count := range []int{1, 5, 25, 40} {
		p, h := meterViewFixture(count, true)
		n := DecodeMeterBorder(p).NativeMeter
		if n == nil || !n.Captured || len(n.Members) != count || n.Members[0].Total != 418240 {
			t.Fatalf("%d: %+v", count, n)
		}
		at := fastMeterOrigin(h.Layout, h.BodyBytes, 4)
		paintCastColor(p, at.X+106, at.Y, 6, 0xffffff)
		n = DecodeMeterBorder(p).NativeMeter
		if n == nil || n.Captured || n.Error != "meter_view_duplicate_mismatch" {
			t.Fatalf("damaged number trusted: %+v", n)
		}
	}
}
func TestMeterViewRejectsForeignOrAmbiguousMetadata(t *testing.T) {
	p, h := meterViewFixture(1, true)
	slots := h.Layout.Slots()
	body := []byte{}
	for i := 1; i <= (h.BodyBytes+35)/36; i++ {
		at := slots[i]
		body = append(body, p.Cells(image.Rect(at.X, at.Y, at.X+6, at.Y+8))...)
	}
	text := string(body[:h.BodyBytes])
	for _, bad := range []string{strings.Replace(text, "Player-1-Aug|13|Player", "row:1|13|Player", 1), strings.Replace(text, "Current|actors|Player-1-Aug|0|1|1|1|0", "Current|actors|Player-1-Aug|0|1|1|1|1", 1), strings.Replace(text, "!boss|Current", "!unknown|Current", 1)} {
		if s, _ := parseMeterView([]byte(bad)); s.Error == "" {
			t.Fatal("malformed view accepted", bad)
		}
	}
}

func TestMeterViewLiveHistoryFallback(t *testing.T) {
	// A real native Current session can coexist with an unnamed saved fight.
	// The addon supplies the fallback label; the rest of this live header must
	// remain accepted, including its independently public roster identity.
	body := "8|overlay:1793245639:490290268:1|56|4|Player-1-Aug|1473|0|6000|32|17000|1419|767|324|274|boss|0|0|0|0|0|0|3|-|0|-|-|0|-|-|1|1|1|53|21|11|880|both|1|0|Current|actors|_|0|1|1|1|0\n1|Player-1-Aug|13|Aug|0|0|0|0|15\n!boss|Current\n!overall|Overall\n!previous|Previous\n!native:4|Combat"
	sample, _ := parseMeterView([]byte(body))
	if sample.Error != "" || sample.NativeMeter == nil || sample.NativeMeter.Frame.WindowID != 1 {
		t.Fatalf("live header rejected: %+v", sample)
	}
}

func repackMeterView(p Pixels, h MeterBorderHeader, b MeterBorderLayout) (Pixels, MeterBorderHeader) {
	oldSlots := h.Layout.Slots()
	body := []byte{}
	for i := 1; i <= (h.BodyBytes+35)/36; i++ {
		at := oldSlots[i]
		body = append(body, p.Cells(image.Rect(at.X, at.Y, at.X+6, at.Y+8))...)
	}
	body = body[:h.BodyBytes]
	s, masks := parseMeterView(body)
	v := s.NativeMeter.View
	oldPoints, oldMedia, _ := meterViewPositions(h.Layout, len(body), len(v.Rows), v.Primary, masks)
	points, media, ok := meterViewPositions(b, len(body), len(v.Rows), v.Primary, masks)
	if !ok {
		panic("fixture border has insufficient space")
	}
	out := Pixels{Width: b.Width(), Height: b.Height(), BGRA: make([]byte, b.Width()*b.Height()*4)}
	copyRect := func(from image.Rectangle, to image.Point) {
		for y := 0; y < from.Dy(); y++ {
			for x := 0; x < from.Dx(); x++ {
				a := ((from.Min.Y+y)*p.Width + from.Min.X + x) * 4
				z := ((to.Y+y)*out.Width + to.X + x) * 4
				copy(out.BGRA[z:z+4], p.BGRA[a:a+4])
			}
		}
	}
	h.Layout = b
	h.Tiles = b.capacity()
	slots := b.Slots()
	for i := 1; i <= (h.BodyBytes+35)/36; i++ {
		a := oldSlots[i]
		copyRect(image.Rect(a.X, a.Y, a.X+8, a.Y+8), slots[i])
	}
	for i, at := range points {
		a := oldPoints[i]
		copyRect(image.Rect(a.X, a.Y, a.X+208, a.Y+8), at)
	}
	for i, r := range v.Rows {
		x, y := i%(p.Width/16)*16, oldMedia+i/(p.Width/16)*16
		copyRect(image.Rect(x, y, x+16, y+16), image.Pt(i%(out.Width/16)*16, media+i/(out.Width/16)*16))
		if r.Mask > 0 {
			oldY := oldMedia + (len(v.Rows)+p.Width/16-1)/(p.Width/16)*16
			newY := media + (len(v.Rows)+out.Width/16-1)/(out.Width/16)*16
			x, y = (r.Mask-1)%(p.Width/192)*192, oldY+(r.Mask-1)/(p.Width/192)*16
			copyRect(image.Rect(x, y, x+192, y+16), image.Pt((r.Mask-1)%(out.Width/192)*192, newY+(r.Mask-1)/(out.Width/192)*16))
		}
	}
	for i, n := range h.Encode() {
		j := i / 3
		r, g, blue := out.rgb(j%3*2, j/3*2)
		c := []byte{r, g, blue}
		c[i%3] = n
		paintCastColor(out, j%3*2, j/3*2, 2, int(c[0])<<16|int(c[1])<<8|int(c[2]))
	}
	return out, h
}

func TestMeterViewCompactAndHollowBorder(t *testing.T) {
	for _, primary := range []bool{true, false} {
		p, h := meterViewFixture(1, primary)
		rows, masks := 1, 0
		if !primary {
			masks = 1
		}
		for _, b := range []MeterBorderLayout{planMeterView(h.BodyBytes, rows, primary, masks, 208), {Generic: true, Fast: true, InnerWidth: 323, InnerHeight: 200, Edges: [4]int{136, 8, 16, 8}}, {Generic: true, Fast: true, InnerWidth: 323, InnerHeight: 200, Edges: [4]int{136, 2, 2, 2}}} {
			q, _ := repackMeterView(p, h, b)
			n := DecodeMeterBorder(q).NativeMeter
			if n == nil || !n.Captured || n.Frame.Standalone != b.Standalone() || n.View.Rows[0].Total != 418240 {
				t.Fatalf("%+v: %+v", b, n)
			}
			if !primary && n.View.Rows[0].NameImage == nil {
				t.Fatal("protected target name was lost")
			}
		}
	}
}
