package lookup

import "math"

// The original data font renders hex as small marker/quadrant tiles. Values
// still reach native display sinks only; Lua never computes their digits.
type compactGlyph [48]bool

func compactGlyphAt(p Pixels, x, y int, pitch float64) (compactGlyph, bool) {
	var g compactGlyph
	if !p.valid() || pitch < 5 || pitch > 12 || x < 0 || y < 0 || x+int(math.Ceil(pitch)) > p.Width || y+8 > p.Height {
		return g, false
	}
	for yy := 0; yy < 8; yy++ {
		for xx := 0; xx < 6; xx++ {
			g[yy*6+xx] = p.ink(x+int((float64(xx)+.5)*pitch/6), y+yy)
		}
	}
	return g, true
}
func compactDistance(a, b compactGlyph) float64 {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return float64(n) / 48
}
func compactCalibration(p Pixels, x, y int, pitch float64) ([]compactGlyph, bool) {
	result := make([]compactGlyph, 17)
	for i := range result {
		g, ok := compactGlyphAt(p, x+int(float64(i)*pitch), y, pitch)
		if !ok {
			return nil, false
		}
		ink := 0
		for _, on := range g {
			if on {
				ink++
			}
		}
		if ink < 4 {
			return nil, false
		}
		for _, old := range result[:i] {
			if compactDistance(g, old) < .06 {
				return nil, false
			}
		}
		result[i] = g
	}
	return result, true
}

// Native monospace decimal capture needs ten digits and its end sentinel.
// Keep a two-pixel separation, and require exact matches when decoding it.
// Unused a-f shapes cannot make an otherwise unambiguous decimal alphabet fail.
func monoDecimalCalibration(p Pixels, x, y int) ([]compactGlyph, bool) {
	result := make([]compactGlyph, 17)
	indices := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 16}
	for at, i := range indices {
		g, ok := compactGlyphAt(p, x+i*5, y, 5)
		if !ok {
			return nil, false
		}
		ink := 0
		for _, on := range g {
			if on {
				ink++
			}
		}
		if ink < 4 {
			return nil, false
		}
		for _, j := range indices[:at] {
			if compactDistance(g, result[j]) < 2.0/48 {
				return nil, false
			}
		}
		result[i] = g
	}
	return result, true
}

func monoDecimalValue(gs []compactGlyph, calibration []compactGlyph) (int64, bool) {
	if len(gs) != 16 || len(calibration) != 17 {
		return 0, false
	}
	var value int64
	for at, g := range gs {
		if at == 15 {
			if g != calibration[16] {
				return 0, false
			}
			break
		}
		id := -1
		for digit := 0; digit < 10; digit++ {
			if g == calibration[digit] {
				if id >= 0 {
					return 0, false
				}
				id = digit
			}
		}
		if id < 0 {
			return 0, false
		}
		value = value*10 + int64(id)
	}
	return value, value <= 999999999999999
}
func compactNumber(p Pixels, x, y int, pitch float64, calibration []compactGlyph) (int64, bool) {
	gs := make([]compactGlyph, 14)
	for char := range gs {
		g, ok := compactGlyphAt(p, x+int(float64(char)*pitch), y, pitch)
		if !ok {
			return 0, false
		}
		gs[char] = g
	}
	return compactValue(gs, 16, calibration)
}
func compactValue(gs []compactGlyph, base int, calibration []compactGlyph) (int64, bool) {
	if len(calibration) != 17 {
		return 0, false
	}
	var value int64
	if (base == 16 && len(gs) != 14) || (base == 10 && len(gs) != 16) || (base != 16 && base != 10) {
		return 0, false
	}
	for char, g := range gs {
		best, second, id := 1.0, 1.0, -1
		for i, template := range calibration {
			d := compactDistance(g, template)
			if d < best {
				second, best, id = best, d, i
			} else if d < second {
				second = d
			}
		}
		if best > .06 || second-best < .02 {
			return 0, false
		}
		if char == len(gs)-1 {
			if id != 16 {
				return 0, false
			}
		} else {
			if id < 0 || id >= base {
				return 0, false
			}
			value = value*int64(base) + int64(id)
		}
	}
	return value, value <= 999999999999999
}
