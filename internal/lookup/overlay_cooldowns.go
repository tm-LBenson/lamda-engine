package lookup

// Only kind/ID/availability travel as metadata; all values are measured from
// paired native tiles. Existing aura/resource coordinates remain unchanged.
type OverlayCooldown struct {
	Kind             string   `json:"kind"`
	ID               int      `json:"id"`
	Available        bool     `json:"available"`
	Carried          *bool    `json:"carried,omitempty"`
	Remaining        *float64 `json:"remaining,omitempty"`
	Duration         *float64 `json:"duration,omitempty"`
	Charges          *float64 `json:"charges,omitempty"`
	MaxCharges       *float64 `json:"maxCharges,omitempty"`
	Recharge         *float64 `json:"recharge,omitempty"`
	RechargeDuration *float64 `json:"rechargeDuration,omitempty"`
	Usable           *bool    `json:"usable,omitempty"`
	Enabled          *bool    `json:"enabled,omitempty"`
}

func ValidCooldownWatch(kind string, id int) bool {
	switch kind {
	case "spell", "item":
		return id > 0 && id < 10000000
	case "slot":
		return id >= 1 && id <= 19
	case "gcd":
		return id == 61304
	}
	return false
}

func OverlayCooldownPoint(index, field, copy int) (int, int) {
	row := [...]int{4, 16, 20, 32}
	cell := index*16 + field*2 + copy
	return (cell % 64) * 4, row[cell/64]
}

// Compact FontStrings can rasterize the marked I into only the lower row of
// their 2x2 clip. Accept that exact glyph mask; the caller still requires the
// second independent copy and a field-specific marker/range. Black, white,
// arbitrary mixed pixels and disagreement never become a ready measurement.
func overlayCooldownColor(p Pixels, x, y, version int) (int, bool) {
	if v, ok := overlayNativeColor(p, x, y, version); ok {
		return v, true
	}
	if version < 2 || x < 0 || y < 0 || x+2 > p.Width || y+2 > p.Height {
		return 0, false
	}
	for dx := 0; dx < 2; dx++ {
		r, g, b := p.rgb(x+dx, y)
		if r != 0 || g != 0 || b != 0 {
			return 0, false
		}
	}
	r, g, b := p.rgb(x, y+1)
	r2, g2, b2 := p.rgb(x+1, y+1)
	v := int(r)<<16 | int(g)<<8 | int(b)
	return v, r == r2 && g == g2 && b == b2 && v >= 0x100000 && v <= 0x100000+864000
}

func decodeOverlayCooldowns(p Pixels, panelY int, input *OverlayInput) {
	if input.CooldownsVersion != 1 || len(input.Cooldowns) > overlayTimerLimit(input.NativeVersion) {
		return
	}
	for i := range input.Cooldowns {
		c := &input.Cooldowns[i]
		if !c.Available {
			continue
		}
		values := []*float64{nil, nil, nil, nil, nil, nil, nil, nil}
		for f := 0; f < 8; f++ {
			x, y := overlayTimerPoint(i, f, 0, input.NativeVersion)
			x2, y2 := overlayTimerPoint(i, f, 1, input.NativeVersion)
			a, ok := overlayCooldownColor(p, x, panelY+y, input.NativeVersion)
			b, ok2 := overlayCooldownColor(p, x2, panelY+y2, input.NativeVersion)
			// Every field has a marker byte: an occluding black rectangle must
			// not turn a cooldown into a valid zero (ready) measurement.
			limit := 864000
			if f == 2 || f == 3 {
				limit = 999
			}
			if f >= 6 {
				limit = 1
			}
			if !ok || !ok2 || a != b || a < 0x100000 || a > 0x100000+limit {
				continue
			}
			v := float64(a - 0x100000)
			if f == 0 || f == 1 || f == 4 || f == 5 {
				v /= 10
			}
			values[f] = &v
		}
		c.Remaining, c.Duration, c.Charges, c.MaxCharges, c.Recharge, c.RechargeDuration = values[0], values[1], values[2], values[3], values[4], values[5]
		if c.Charges != nil && c.MaxCharges != nil && *c.Charges > *c.MaxCharges {
			c.Charges = nil
			c.MaxCharges = nil
		}
		if values[6] != nil {
			b := *values[6] == 1
			c.Usable = &b
		}
		if values[7] != nil {
			b := *values[7] == 1
			c.Enabled = &b
		}
	}
}
