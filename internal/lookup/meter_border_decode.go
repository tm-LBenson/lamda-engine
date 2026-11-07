package lookup

import (
	"hash/crc32"
	"image"
	"math"
)

// Pixels contains only the four native border bands; the inner meter is zeroed
// by the Windows reader. All values are decoded from that same bound reading.
func DecodeMeterBorder(p Pixels) BridgeSample {
	bad := BridgeSample{Source: "augmentation", Error: "meter_border_unreadable"}
	h, err := ParseMeterBorderHeader(p.Cells(image.Rect(0, 0, 6, 8)))
	if err != nil || h.Layout.Width() != p.Width || h.Layout.Height() != p.Height {
		return bad
	}
	if h.Layout.Generic {
		return decodeMeterView(p, h)
	}
	if h.Layout.Fast {
		return decodeFastMeterStrip(p, h)
	}
	slots := h.Layout.Slots()
	if len(slots) < h.Tiles {
		return bad
	}
	slots = slots[:h.Tiles]
	cells := func(i int) []byte {
		if i < 0 || i >= len(slots) {
			return nil
		}
		s := slots[i]
		return p.Cells(image.Rect(s.X, s.Y, s.X+6, s.Y+8))
	}
	metaTiles := (h.BodyBytes + 35) / 36
	body := make([]byte, 0, metaTiles*36)
	for i := 1; i <= metaTiles; i++ {
		body = append(body, cells(i)...)
	}
	if len(body) < h.BodyBytes {
		return bad
	}
	body = body[:h.BodyBytes]
	if crc32.ChecksumIEEE(body) != h.BodyCRC {
		bad.Error = "meter_border_metadata_checksum"
		return bad
	}
	s := parseNativeMeter(body)
	if s.Error != "" || s.NativeMeter == nil {
		return s
	}
	n := s.NativeMeter
	n.Border = &h.Layout
	if n.Frame != nil {
		n.Frame.Standalone = h.Layout.Standalone()
	}
	n.Captured = false
	n.Error = "meter_border_unreadable"
	need, e := MeterBorderTiles(h.BodyBytes, len(n.Members))
	if e != nil || need != h.Tiles || n.Serial != h.Serial || n.Frame == nil || (!h.Layout.Standalone() && (n.Frame.Width != h.Layout.InnerWidth || n.Frame.Height != h.Layout.InnerHeight)) {
		n.Error = "meter_border_metadata_mismatch"
		return s
	}
	index := 1 + metaTiles
	cal := make([]uint64, 17)
	for i := range cal {
		point := slots[index]
		g := expectedBorderGlyph(i)
		found := false
		for _, candidate := range borderGlyphCandidates(p, point.X, point.Y) {
			found = found || candidate == g
		}
		if !found {
			n.Error = "meter_border_font_unreadable"
			return s
		}
		index++
		for _, old := range cal[:i] {
			if g == old {
				n.Error = "meter_border_font_ambiguous"
				return s
			}
		}
		cal[i] = g
	}
	amounts := make([]int64, len(n.Members)+1)
	masks := make([]int, len(n.Members))
	for row := range amounts {
		raw := cells(index)
		index++
		if len(raw) != 36 {
			return s
		}
		sum := 0
		for i := 0; i < 7; i++ {
			sum += int(raw[i]) * (i + 1)
		}
		serial := int(raw[3])<<16 | int(raw[4])<<8 | int(raw[5])
		if raw[0] != 173 || int(raw[1]) != row || raw[2] != 118 || serial != h.Serial || raw[6] != 59 || raw[7] != byte(sum%256) || raw[8] != 93 || raw[9] != 205 || raw[10] != 127 || raw[11] != 61 {
			n.Error = "meter_border_row_serial"
			return s
		}
		count := 16
		var copies [2]int64
		for copy := 0; copy < 2; copy++ {
			var value int64
			for char := 0; char < count; char++ {
				point := slots[index]
				id, ok := borderCharacter(p, point.X, point.Y, char == count-1, cal)
				if !ok {
					n.Error = "meter_border_value_unreadable"
					return s
				}
				if char < count-1 {
					value = value*10 + int64(id)
				}
				index++
			}
			copies[copy] = value
		}
		if copies[0] != copies[1] {
			n.Error = "meter_border_duplicate_mismatch"
			return s
		}
		amounts[row] = copies[0]
		if row > 0 {
			raw = cells(index)
			index++
			masks[row-1] = -1
			if n.Members[row-1].AuraMode == 15 {
				mask := 0
				for bit := 0; bit < 4; bit++ {
					i := bit * 9
					if raw[i] != raw[i+3] || raw[i+1] != raw[i+4] || raw[i+2] != raw[i+5] {
						n.Error = "meter_border_aura_mismatch"
						return s
					}
					if raw[i] == 24 && raw[i+1] == 224 && raw[i+2] == 96 {
						mask |= 1 << bit
					} else if raw[i] != 160 || raw[i+1] != 32 || raw[i+2] != 96 {
						n.Error = "meter_border_aura_unreadable"
						return s
					}
				}
				masks[row-1] = mask
			}
		}
	}
	if index != len(slots) {
		n.Error = "meter_border_tile_count"
		return s
	}
	if n.DurationMillis < 0 {
		n.DurationMillis = amounts[0] * 1000
	}
	if n.DurationMillis > 604800000 || math.Abs(float64(n.DurationMillis)/1000-float64(amounts[0])) > .75 {
		n.Error = "meter_border_duration"
		return s
	}
	for i := range n.Members {
		n.Members[i].Total = amounts[i+1]
		n.Members[i].AuraMask = masks[i]
	}
	n.Captured = true
	n.Error = ""
	return s
}
