package lookup

import (
	"fmt"
	"image"
)

var castCalibrationColors = []int{0x055c6a, 0x001992, 0xabcdef, 0x123456, 0xffffff, 0x000001, 0x00ff00, 0xff0000}

// Read the four center pixels of a clipped solid glyph. Exact agreement is
// required: no nearest-color guessing can turn a corrupt ID into another cast.
func solidCastColor(p Pixels, x, y int) (int, bool) {
	value := -1
	for yy := y + 1; yy <= y+2; yy++ {
		for xx := x + 1; xx <= x+2; xx++ {
			r, g, b := p.rgb(xx, yy)
			v := int(r)<<16 | int(g)<<8 | int(b)
			if value >= 0 && value != v {
				return 0, false
			}
			value = v
		}
	}
	return value, true
}

func decodeColorCasts(s *ScreenCasts, p Pixels, y int) {
	if !p.valid() || !image.Rect(0, y, 128, y+ColorCastHeight).In(image.Rect(0, 0, p.Width, p.Height)) {
		s.Error = "color_panel_bounds"
		return
	}
	for i, want := range castCalibrationColors {
		got, ok := solidCastColor(p, i*4, y)
		if !ok || got != want {
			s.Error = "color_calibration_mismatch"
			return
		}
	}
	s.Captured = true
	owners := map[int]string{}
	for _, m := range s.Members {
		owners[m.Slot] = m.GUID
	}
	for index := 0; index < 20; index++ {
		guid := owners[index/4]
		if guid == "" {
			continue
		}
		x, yy := index%10*12, y+4+index/10*4
		var h []byte
		for cell := 0; cell < 4; cell++ {
			r, g, b := p.rgb(x+1+cell%2*2, yy+1+cell/2*2)
			h = append(h, r, g, b)
		}
		sum := 0
		for i := 0; i < 9; i++ {
			sum += int(h[i]) * (i + 1)
		}
		if h[0] != 172 || int(h[1]) != index || h[2] != 117 || h[8] != 57 || h[9] != byte(sum%256) || h[10] != 91 || h[11] != 203 {
			s.Rejected++
			continue
		}
		serial := int(h[3])<<16 | int(h[4])<<8 | int(h[5])
		age := int(h[6])<<8 | int(h[7])
		if serial == 0 {
			continue
		}
		id, ok := solidCastColor(p, x+4, yy)
		copy, copyOK := solidCastColor(p, x+8, yy)
		if age > 30000 || !ok || !copyOK || id != copy || id <= 0 || id > 10000000 {
			s.Rejected++
			continue
		}
		s.Casts = append(s.Casts, LiveCast{ID: fmt.Sprintf("Screen:%s:%d", s.ID, serial), GUID: guid, Spell: id, AgeMillis: age})
	}
}
