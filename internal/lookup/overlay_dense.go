package lookup

const OverlaySourceLimit = 64
const OverlayTimerLimit = 90
const OverlayAnchorLimit = 64

func overlaySourceLimit(version int) int {
	if version >= 2 {
		return OverlaySourceLimit
	}
	return 16
}

// Compact formats keep the same virtual 256x40 panel and duplicate sinks.
// Version 3 moves the measurement bank after 90 timer slots; version 2 keeps 64.
// Each patch contains the four inner pixels sampled by the older 4x4 patch.
func overlayDensePoint(cell int) (int, int) { return cell % 128 * 2, cell / 128 * 2 }

func overlayAuraPoint(index, field, copy, version int) (int, int) {
	if version >= 2 {
		return overlayDensePoint(144 + index*8 + copy*4 + field)
	}
	return index%8*32 + copy*16 + field*4, 8 + index/8*16
}
func overlayTimerPoint(index, field, copy, version int) (int, int) {
	if version >= 2 {
		return overlayDensePoint(656 + index*16 + field*2 + copy)
	}
	return OverlayCooldownPoint(index, field, copy)
}
func overlayMeasurementPoint(index, field, copy, version int) (int, int) {
	if version >= 2 {
		base := 1680
		if version == 3 {
			base = 2096
		}
		return overlayDensePoint(base + index*6 + copy*3 + field)
	}
	return OverlayMeasurementPoint(index, field, copy)
}
func overlayHealthPoint(copy, version int) (int, int) {
	if version >= 2 {
		return overlayDensePoint(8 + copy)
	}
	return 40 + copy*4, 0
}
func overlayResourcePoint(power, field, version int) (int, int) {
	if version >= 2 {
		return overlayDensePoint(16 + power*4 + field)
	}
	return power%16*16 + field*4, 12 + power/16*16
}
func overlayCalibrationPoint(index, version int) (int, int) {
	if version >= 2 {
		return overlayDensePoint(index)
	}
	return index * 4, 0
}
func overlayNativeColor(p Pixels, x, y, version int) (int, bool) {
	if version < 2 {
		return overlayColor(p, x, y)
	}
	if x < 0 || y < 0 || x+2 > p.Width || y+2 > p.Height {
		return 0, false
	}
	r, g, b := p.rgb(x, y)
	for _, v := range [][2]int{{1, 0}, {0, 1}, {1, 1}} {
		a, c, d := p.rgb(x+v[0], y+v[1])
		if a != r || c != g || d != b {
			return 0, false
		}
	}
	return int(r)<<16 | int(g)<<8 | int(b), true
}

func overlayTimerLimit(version int) int {
	if version == 3 {
		return OverlayTimerLimit
	}
	return overlaySourceLimit(version)
}
