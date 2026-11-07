package lookup

import (
	"fmt"
	"hash/crc32"
	"image"
	"math"
)

// Full native FontStrings replace the per-digit ScrollFrames. Public metadata
// still has its CRC; every number retains a serial tag, sentinel and two copies.
func PlanFastMeterStrip(bodyBytes, members, minWidth, maxWidth, maxHeight int) (MeterBorderLayout, error) {
	maxWidth = min(maxWidth, 4096)
	columns := min(max(2, (minWidth+207)/208), maxWidth/208)
	if columns < 2 || bodyBytes < 1 || bodyBytes > 6134 || members < 1 || members > 40 {
		return MeterBorderLayout{}, fmt.Errorf("invalid fast strip")
	}
	for {
		width := columns * 208
		meta := ((1 + (bodyBytes+35)/36 + width/8 - 1) / (width / 8)) * 8
		height := meta + ((members+2+columns-1)/columns)*8
		if height <= 32 || (columns+1)*208 > maxWidth {
			if height > maxHeight {
				return MeterBorderLayout{}, fmt.Errorf("fast strip does not fit")
			}
			return MeterBorderLayout{Fast: true, InnerWidth: width, Edges: [4]int{height, 0, 0, 0}}, nil
		}
		columns++
	}
}
func fastMeterOrigin(b MeterBorderLayout, bodyBytes, index int) image.Point {
	columns := b.InnerWidth / 208
	meta := ((1 + (bodyBytes+35)/36 + b.InnerWidth/8 - 1) / (b.InnerWidth / 8)) * 8
	return image.Pt(index%columns*208, meta+index/columns*8)
}
func decodeFastMeterStrip(p Pixels, h MeterBorderHeader) BridgeSample {
	bad := BridgeSample{Source: "augmentation", Error: "meter_fast_unreadable"}
	slots := h.Layout.Slots()
	meta := (h.BodyBytes + 35) / 36
	if meta+1 > len(slots) {
		return bad
	}
	body := make([]byte, 0, meta*36)
	for i := 1; i <= meta; i++ {
		s := slots[i]
		body = append(body, p.Cells(image.Rect(s.X, s.Y, s.X+6, s.Y+8))...)
	}
	if len(body) < h.BodyBytes {
		return bad
	}
	body = body[:h.BodyBytes]
	if crc32.ChecksumIEEE(body) != h.BodyCRC {
		bad.Error = "meter_fast_metadata_checksum"
		return bad
	}
	result := parseNativeMeter(body)
	n := result.NativeMeter
	if result.Error != "" || n == nil {
		return result
	}
	n.Border = &h.Layout
	if n.Frame != nil {
		n.Frame.Standalone = true
	}
	n.Captured = false
	n.Error = "meter_fast_unreadable"
	// Check exact dimensions independently of the configured minimum width.
	minimum, e := PlanFastMeterStrip(len(body), len(n.Members), h.Layout.InnerWidth, h.Layout.InnerWidth, p.Height)
	if e != nil || minimum.Height() != p.Height || n.Serial != h.Serial || n.Frame == nil {
		n.Error = "meter_fast_metadata_mismatch"
		return result
	}
	calAt := fastMeterOrigin(h.Layout, len(body), 0)
	cal, ok := compactCalibration(p, calAt.X+8, calAt.Y, 6)
	if !ok {
		n.Error = "meter_fast_calibration"
		return result
	}
	number := func(x, y int) (int64, bool) {
		gs := make([]compactGlyph, 16)
		for i := range gs {
			g, valid := compactGlyphAt(p, x+i*6, y, 6)
			if !valid {
				return 0, false
			}
			gs[i] = g
		}
		return compactValue(gs, 10, cal)
	}
	values := make([]int64, len(n.Members)+1)
	masks := make([]int, len(n.Members))
	for row := range values {
		at := fastMeterOrigin(h.Layout, len(body), row+1)
		raw := p.Cells(image.Rect(at.X, at.Y, at.X+6, at.Y+8))
		if len(raw) != 36 {
			return result
		}
		sum := 0
		for i := 0; i < 7; i++ {
			sum += int(raw[i]) * (i + 1)
		}
		serial := int(raw[3])<<16 | int(raw[4])<<8 | int(raw[5])
		if raw[0] != 173 || int(raw[1]) != row || raw[2] != 118 || serial != h.Serial || raw[6] != 59 || raw[7] != byte(sum%256) || raw[8] != 93 || raw[9] != 205 || raw[10] != 127 || raw[11] != 61 {
			n.Error = "meter_fast_row_serial"
			return result
		}
		first, a := number(at.X+8, at.Y)
		second, b := number(at.X+106, at.Y)
		if !a || !b {
			n.Error = "meter_fast_value_unreadable"
			return result
		}
		if first != second {
			n.Error = "meter_fast_duplicate_mismatch"
			return result
		}
		values[row] = first
		if row > 0 {
			masks[row-1] = -1
			if n.Members[row-1].AuraMode == 15 {
				mask := 0
				for bit := 0; bit < 4; bit++ {
					colors := p.Cells(image.Rect(at.X+204, at.Y+bit*2, at.X+208, at.Y+bit*2+2))
					if len(colors) != 6 || colors[0] != colors[3] || colors[1] != colors[4] || colors[2] != colors[5] {
						n.Error = "meter_fast_aura_mismatch"
						return result
					}
					left := int(colors[0])<<16 | int(colors[1])<<8 | int(colors[2])
					if left == 0x18e060 {
						mask |= 1 << bit
					} else if left != 0xa02060 {
						n.Error = "meter_fast_aura_unreadable"
						return result
					}
				}
				masks[row-1] = mask
			}
		}
	}
	if n.DurationMillis < 0 {
		n.DurationMillis = values[0] * 1000
	}
	if n.DurationMillis > 604800000 || math.Abs(float64(n.DurationMillis)/1000-float64(values[0])) > .75 {
		n.Error = "meter_fast_duration"
		return result
	}
	for i := range n.Members {
		n.Members[i].Total = values[i+1]
		n.Members[i].AuraMask = masks[i]
	}
	n.Captured = true
	n.Error = ""
	return result
}
