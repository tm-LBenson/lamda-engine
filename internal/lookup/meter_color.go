package lookup

// Compact color glyphs keep the native decimal formatter and two independent
// display copies. Only the raster changes: cyan marks on a black backing.
func meterColorGlyph(p Pixels, x, y, pitch int) (compactGlyph, bool) {
	var g compactGlyph
	if !p.valid() || pitch < 4 || pitch > 6 || x < 0 || y < 0 || x+pitch > p.Width || y+4 > p.Height {
		return g, false
	}
	for yy := 0; yy < 4; yy++ {
		for xx := 0; xx < pitch; xx++ {
			r, green, b := p.rgb(x+xx, y+yy)
			g[yy*6+xx] = r <= 64 && green >= 128 && b >= 160 && int(b)-int(green) <= 64
		}
	}
	return g, true
}
func meterColorCalibration(p Pixels, x, y, pitch int) ([]compactGlyph, bool) {
	cal := make([]compactGlyph, 17)
	for i := range cal {
		g, ok := meterColorGlyph(p, x+i*pitch, y, pitch)
		if !ok {
			return nil, false
		}
		ink := 0
		for _, on := range g {
			if on {
				ink++
			}
		}
		if ink < 2 {
			return nil, false
		}
		for _, old := range cal[:i] {
			if g == old {
				return nil, false
			}
		}
		cal[i] = g
	}
	return cal, true
}
