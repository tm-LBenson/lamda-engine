package lookup

import "image"

// The native meter's spell rows identify interrupted enemy spells, not the
// interrupt ability. Read only its owner total; resolve abilities separately.
func decodeMeter(s *ScreenCasts, p Pixels, y int) {
	if !image.Rect(0, y, 128, y+MeterCastHeight).In(image.Rect(0, 0, p.Width, p.Height)) {
		s.Error = "meter_panel_bounds"
		return
	}
	for _, member := range s.Members {
		index := member.Slot
		x, yy := index*12, y+12
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
		serial, age := int(h[3])<<16|int(h[4])<<8|int(h[5]), int(h[6])<<8|int(h[7])
		if serial == 0 {
			continue
		} // Unavailable/disabled source, not an empty counter.
		amount, ok := solidCastColor(p, x+4, yy)
		copy, copyOK := solidCastColor(p, x+8, yy)
		if age > 1500 || !ok || !copyOK || amount != copy || amount > 1000000 {
			s.Rejected++
			continue
		}
		s.Meter = append(s.Meter, MeterOwner{GUID: member.GUID, Serial: serial, AgeMillis: age, Amount: amount})
	}
}
