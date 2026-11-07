package lookup

// Native display clips can round by a pixel. Read each six-column window in
// the padded tile, normalize its ink origin, and require an exact alphabet
// shape. Neighboring characters in the padding are never part of the digit.
func borderWindowGlyph(p Pixels, x, y, offset int) (uint64, bool) {
	if !p.valid() || x < 0 || y < 0 || x+8 > p.Width || y+8 > p.Height {
		return 0, false
	}
	minX, minY := 6, 8
	var raw uint64
	for yy := 0; yy < 8; yy++ {
		for xx := 0; xx < 6; xx++ {
			if p.ink(x+offset+xx, y+yy) {
				raw |= uint64(1) << uint(yy*8+xx)
				minX = min(minX, xx)
				minY = min(minY, yy)
			}
		}
	}
	if raw == 0 {
		return 0, false
	}
	var normalized uint64
	for yy := minY; yy < 8; yy++ {
		for xx := minX; xx < 6; xx++ {
			if raw&(uint64(1)<<uint(yy*8+xx)) != 0 {
				normalized |= uint64(1) << uint((yy-minY)*8+xx-minX)
			}
		}
	}
	return normalized, true
}
func borderGlyphCandidates(p Pixels, x, y int) []uint64 {
	var result []uint64
	for offset := 0; offset <= 2; offset++ {
		g, ok := borderWindowGlyph(p, x, y, offset)
		if !ok {
			continue
		}
		found := false
		for _, old := range result {
			found = found || old == g
		}
		if !found {
			result = append(result, g)
		}
	}
	return result
}
func expectedBorderGlyph(id int) uint64 {
	value, marker := id, id != 16
	if !marker {
		value = 9
	}
	var raw uint64
	shift := 0
	if !marker {
		shift = 2
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 6; x++ {
			on := marker && x < 2
			if x >= 2 {
				on = value&(1<<uint((y/2)*2+(x-2)/2)) != 0
			}
			if on {
				raw |= uint64(1) << uint(y*8+x-shift)
			}
		}
	}
	return raw
}
func borderValue(gs [][]uint64, calibration []uint64) (int64, bool) {
	if len(gs) != 16 || len(calibration) != 17 {
		return 0, false
	}
	var value int64
	for at, candidates := range gs {
		id := -1
		for _, g := range candidates {
			for i, c := range calibration {
				if c == g {
					if id >= 0 && id != i {
						return 0, false
					}
					id = i
				}
			}
		}
		if at == 15 {
			if id != 16 {
				return 0, false
			}
		} else {
			if id < 0 || id > 9 {
				return 0, false
			}
			value = value*10 + int64(id)
		}
	}
	return value, value <= 999999999999999
}

// Decimal digits always have the complete two-column marker. A preceding
// decimal digit cannot have a full-height rightmost pair, so its pixels in the
// padding cannot be mistaken for this marker. This also rejects an overflowing
// numeric field where the required literal end sentinel should have been.
func borderMarker(p Pixels, x, y int) (int, bool) {
	for xx := 0; xx <= 2; xx++ {
		for yy := 0; yy <= 4; yy++ {
			all := true
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					all = all && p.ink(x+xx+dx, y+yy+dy)
				}
			}
			if all {
				return xx, true
			}
		}
	}
	return 0, false
}
func borderCharacter(p Pixels, x, y int, end bool, cal []uint64) (int, bool) {
	marker, found := borderMarker(p, x, y)
	if end {
		if found {
			return 0, false
		}
		for _, g := range borderGlyphCandidates(p, x, y) {
			if g == cal[16] {
				return 16, true
			}
		}
		return 0, false
	}
	if !found {
		return 0, false
	}
	g, ok := borderWindowGlyph(p, x, y, marker)
	if !ok {
		return 0, false
	}
	for id := 0; id < 10; id++ {
		if g == cal[id] {
			return id, true
		}
	}
	return 0, false
}
