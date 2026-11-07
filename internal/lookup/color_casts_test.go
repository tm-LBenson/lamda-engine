package lookup

import (
	"fmt"
	"strings"
	"testing"
)

func paintCastColor(p Pixels, x, y, size, value int) {
	for yy := y; yy < y+size; yy++ {
		for xx := x; xx < x+size; xx++ {
			i := (yy*p.Width + xx) * 4
			p.BGRA[i], p.BGRA[i+1], p.BGRA[i+2] = byte(value), byte(value>>8), byte(value>>16)
		}
	}
}

func colorFixture() (Pixels, *ScreenCasts) {
	p := Pixels{Width: 128, Height: 12, BGRA: make([]byte, 128*12*4)}
	s := &ScreenCasts{Version: 2, ID: "123:1"}
	for i, c := range castCalibrationColors {
		paintCastColor(p, i*4, 0, 4, c)
	}
	for slot := 0; slot < 5; slot++ {
		s.Members = append(s.Members, ScreenCastMember{Slot: slot, GUID: fmt.Sprintf("Player-1-%d", slot)})
	}
	for index := 0; index < 20; index++ {
		paintColorEntry(p, index, index+1, 250, 47528)
	}
	return p, s
}

func paintColorEntry(p Pixels, index, serial, age, spell int) {
	h := []byte{172, byte(index), 117, byte(serial >> 16), byte(serial >> 8), byte(serial), byte(age >> 8), byte(age), 57, 0, 91, 203}
	for i := 0; i < 9; i++ {
		h[9] += h[i] * byte(i+1)
	}
	x, y := index%10*12, 4+index/10*4
	for cell := 0; cell < 4; cell++ {
		i := cell * 3
		paintCastColor(p, x+cell%2*2, y+cell/2*2, 2, int(h[i])<<16|int(h[i+1])<<8|int(h[i+2]))
	}
	paintCastColor(p, x+4, y, 4, spell)
	paintCastColor(p, x+8, y, 4, spell)
}

func TestColorMetadataAndAllOwnerSlots(t *testing.T) {
	for _, offset := range []int{24, -20} {
		s, e := parseScreenCasts(strings.Split(fmt.Sprintf("T|2|123:1|%d|0,1;1,2|0", offset), "|"))
		if e != nil || s.Height() != 12 || s.Pitch != 0 {
			t.Fatal(s, e)
		}
	}
	for _, line := range []string{"T|2|123:1|-184|0,1|0", "T|2|123:1|24|0,1|7000", "T|4|123:1|24|0,1|0", "T|1|123:1|-20|0,1|7000"} {
		if _, e := parseScreenCasts(strings.Split(line, "|")); e == nil {
			t.Fatal("accepted", line)
		}
	}
	p, s := colorFixture()
	DecodeScreenCasts(s, p, 0)
	if !s.Captured || s.Error != "" || s.Rejected != 0 || len(s.Casts) != 20 {
		t.Fatal(s)
	}
	for i, c := range s.Casts {
		if c.Spell != 47528 || c.AgeMillis != 250 || c.GUID != fmt.Sprintf("Player-1-%d", i/4) || c.ID != fmt.Sprintf("Screen:123:1:%d", i+1) {
			t.Fatal(c)
		}
	}
}

func TestColorPanelRequiresExactCalibrationAndBounds(t *testing.T) {
	for _, kind := range []string{"empty", "short", "negative", "offset", "wrong_color", "edge_blending", "missing_glyph"} {
		t.Run(kind, func(t *testing.T) {
			p, s := colorFixture()
			y := 0
			switch kind {
			case "empty":
				p = Pixels{}
			case "short":
				p.BGRA = p.BGRA[:100]
			case "negative":
				y = -1
			case "offset":
				y = 1
			case "wrong_color":
				paintCastColor(p, 4, 0, 4, 0xabcdef)
			case "edge_blending":
				paintCastColor(p, 1, 1, 1, 0x055c69)
			case "missing_glyph":
				paintCastColor(p, 0, 0, 4, 0)
			}
			DecodeScreenCasts(s, p, y)
			if s.Captured || s.Error == "" || len(s.Casts) != 0 {
				t.Fatal(s)
			}
		})
	}
}

func TestColorSlotRejectsCorruptionWithoutGuessing(t *testing.T) {
	for _, kind := range []string{"checksum", "owner_index", "copy", "blending", "expired", "zero", "overflow"} {
		t.Run(kind, func(t *testing.T) {
			p, s := colorFixture()
			switch kind {
			case "checksum":
				paintCastColor(p, 2, 6, 2, 0)
			case "owner_index":
				paintCastColor(p, 0, 4, 2, 0xac1375)
			case "copy":
				paintCastColor(p, 8, 4, 4, 2139)
			case "blending":
				paintCastColor(p, 5, 5, 1, 47527)
			case "expired":
				paintColorEntry(p, 0, 1, 30001, 47528)
			case "zero":
				paintColorEntry(p, 0, 1, 250, 0)
			case "overflow":
				paintColorEntry(p, 0, 1, 250, 10000001)
			}
			DecodeScreenCasts(s, p, 0)
			if !s.Captured || s.Rejected != 1 || len(s.Casts) != 19 {
				t.Fatal(s)
			}
			for _, c := range s.Casts {
				if c.ID == "Screen:123:1:1" {
					t.Fatal("corrupt cast accepted")
				}
			}
		})
	}
	// Inactive headers and absent owners cannot publish stale color values.
	p, s := colorFixture()
	paintColorEntry(p, 0, 0, 250, 47528)
	s.Members = s.Members[:1]
	DecodeScreenCasts(s, p, 0)
	if s.Rejected != 0 || len(s.Casts) != 3 {
		t.Fatal(s)
	}
}
