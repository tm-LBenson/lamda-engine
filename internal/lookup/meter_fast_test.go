package lookup

import (
	"fmt"
	"hash/crc32"
	"image"
	"strings"
	"testing"
)

func fastFixture(members int) (Pixels, MeterBorderHeader) {
	body := "6|fast:123:1|7|69|Player-1-Aug|1473|1|6000|32|12345|288|344|380|206|boss|0|0|0|0|0|0|1|-|0|-|-|0|-|-|1|1"
	for i := 0; i < members; i++ {
		guid := fmt.Sprintf("Player-1-%d", i)
		if i == 0 {
			guid = "Player-1-Aug"
		}
		body += fmt.Sprintf("\n%d|%s|13|Player%d|15", i+1, guid, i)
	}
	layout, _ := PlanFastMeterStrip(len(body), members, 800, 1920, 1080)
	p := Pixels{Width: layout.Width(), Height: layout.Height(), BGRA: make([]byte, layout.Width()*layout.Height()*4)}
	cells := func(at image.Point, data []byte) {
		for i := 0; i < 12; i++ {
			color := 0
			for j := 0; j < 3; j++ {
				color <<= 8
				if i*3+j < len(data) {
					color |= int(data[i*3+j])
				}
			}
			paintCastColor(p, at.X+i%3*2, at.Y+i/3*2, 2, color)
		}
	}
	h := MeterBorderHeader{Layout: layout, BodyBytes: len(body), Serial: 7, Tiles: layout.capacity(), BodyCRC: crc32.ChecksumIEEE([]byte(body))}
	slots := layout.Slots()
	cells(slots[0], h.Encode())
	for i := 0; i < len(body); i += 36 {
		cells(slots[1+i/36], []byte(body[i:min(i+36, len(body))]))
	}
	font := func(x, y int, text string) {
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
	cal := fastMeterOrigin(layout, len(body), 0)
	font(cal.X+8, cal.Y, "0123456789abcdefX")

	for row := 0; row <= members; row++ {
		at := fastMeterOrigin(layout, len(body), row+1)
		tag := []byte{173, byte(row), 118, 0, 0, 7, 59, 0, 93, 205, 127, 61}
		for i := 0; i < 7; i++ {
			tag[7] += tag[i] * byte(i+1)
		}
		cells(at, tag)
		value := int64(12)
		if row > 0 {
			value = 4294967296 + int64(row)
		}
		text := fmt.Sprintf("%015dX", value)
		font(at.X+8, at.Y, text)
		font(at.X+106, at.Y, text)
		for bit := 0; bit < 4; bit++ {
			c := 0xa02060
			if bit < 2 {
				c = 0x18e060
			}
			paintCastColor(p, at.X+204, at.Y+bit*2, 2, c)
			paintCastColor(p, at.X+206, at.Y+bit*2, 2, c)
		}
	}
	return p, h
}
func TestFastStripFullRaidAndDuplicateGuard(t *testing.T) {
	for _, members := range []int{1, 5, 25, 40} {
		t.Run(fmt.Sprint(members), func(t *testing.T) {
			p, h := fastFixture(members)
			s := DecodeMeterBorder(p)
			if s.NativeMeter == nil || !s.NativeMeter.Captured || len(s.NativeMeter.Members) != members {
				t.Fatal(s.Error, s.NativeMeter)
			}
			for i, m := range s.NativeMeter.Members {
				if m.Total != 4294967296+int64(i+1) || m.AuraMask != 3 {
					t.Fatal(m)
				}
			}
			max := 48
			if members <= 25 {
				max = 32
			}
			if p.Height > max {
				t.Fatal("oversized strip", p.Width, p.Height)
			}
			at := fastMeterOrigin(h.Layout, h.BodyBytes, 2)
			paintCastColor(p, at.X+106, at.Y, 6, 0)
			broken := DecodeMeterBorder(p)
			if broken.NativeMeter == nil || broken.NativeMeter.Captured {
				t.Fatal("contradictory copy accepted")
			}
		})
	}
}
func TestFastStripRejectsCroppedMetadataAndDamagedCRC(t *testing.T) {
	p, h := fastFixture(25)
	paintCastColor(p, 0, 0, 2, 0)
	s := DecodeMeterBorder(p)
	if s.Error == "" {
		t.Fatal("damaged header accepted")
	}
	h.BodyBytes = 6134
	h.Layout.Edges[0] = 8
	h.Tiles = h.Layout.capacity()
	small := Pixels{Width: h.Layout.Width(), Height: 8, BGRA: make([]byte, h.Layout.Width()*8*4)}
	if s := decodeFastMeterStrip(small, h); s.Error == "" {
		t.Fatal("cropped metadata")
	}
	if _, e := PlanFastMeterStrip(6000, 40, 800, 400, 1080); e == nil {
		t.Fatal("narrow client accepted")
	}
	if strings.Contains(fmt.Sprint(h), "secret") {
		t.Fatal("private formatting")
	}
}
