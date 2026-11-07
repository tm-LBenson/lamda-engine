package lookup

import (
	"fmt"
	"strings"
	"testing"
)

func meterFixture() (Pixels, *ScreenCasts) {
	old, s := colorFixture()
	s.Version = 3
	p := Pixels{Width: 128, Height: 16, BGRA: make([]byte, 128*16*4)}
	copy(p.BGRA, old.BGRA)
	for i := 0; i < 5; i++ {
		paintMeterEntry(p, i, i+1, 200, 7+i)
	}
	return p, s
}
func paintMeterEntry(p Pixels, index, serial, age, amount int) {
	h := []byte{172, byte(index), 117, byte(serial >> 16), byte(serial >> 8), byte(serial), byte(age >> 8), byte(age), 57, 0, 91, 203}
	for i := 0; i < 9; i++ {
		h[9] += h[i] * byte(i+1)
	}
	x, y := index*12, 12
	for cell := 0; cell < 4; cell++ {
		i := cell * 3
		paintCastColor(p, x+cell%2*2, y+cell/2*2, 2, int(h[i])<<16|int(h[i+1])<<8|int(h[i+2]))
	}
	paintCastColor(p, x+4, y, 4, amount)
	paintCastColor(p, x+8, y, 4, amount)
}
func TestMeterMetadataAndSnapshots(t *testing.T) {
	for _, offset := range []int{24, -24} {
		s, e := parseScreenCasts(strings.Split(fmt.Sprintf("T|3|123:1|%d|0,1;1,2|0", offset), "|"))
		if e != nil || s.Height() != 16 {
			t.Fatal(s, e)
		}
	}
	for _, line := range []string{"T|3|123:1|-20|0,1|0", "T|3|123:1|24|0,1|1"} {
		if _, e := parseScreenCasts(strings.Split(line, "|")); e == nil {
			t.Fatal(line)
		}
	}
	p, s := meterFixture()
	DecodeScreenCasts(s, p, 0)
	if !s.Captured || s.Error != "" || len(s.Meter) != 5 || len(s.Casts) != 20 {
		t.Fatal(s)
	}
	for i, owner := range s.Meter {
		if owner.GUID != fmt.Sprintf("Player-1-%d", i) || owner.Amount != 7+i || owner.Serial != i+1 {
			t.Fatal(owner)
		}
	}
}
func TestMeterRejectsStaleAndCorruptOwnerWithoutDroppingOtherOwners(t *testing.T) {
	for _, kind := range []string{"header", "stale", "amount", "copy", "antialias"} {
		t.Run(kind, func(t *testing.T) {
			p, s := meterFixture()
			switch kind {
			case "header":
				paintCastColor(p, 1, 13, 1, 0)
			case "stale":
				paintMeterEntry(p, 0, 1, 1501, 7)
			case "amount":
				paintMeterEntry(p, 0, 1, 200, 1000001)
			case "copy":
				paintCastColor(p, 8, 12, 4, 8)
			case "antialias":
				paintCastColor(p, 5, 13, 1, 99)
			}
			DecodeScreenCasts(s, p, 0)
			if !s.Captured || len(s.Meter) != 4 || s.Rejected != 1 || len(s.Casts) != 20 {
				t.Fatal(s)
			}
			for _, o := range s.Meter {
				if o.GUID == "Player-1-0" {
					t.Fatal("bad owner accepted")
				}
			}
		})
	}
	p, s := meterFixture()
	paintMeterEntry(p, 0, 0, 0, 0)
	DecodeScreenCasts(s, p, 0)
	if len(s.Meter) != 4 || s.Rejected != 0 {
		t.Fatal("unavailable source treated as zero or corrupt")
	}
}
