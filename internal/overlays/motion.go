package overlays

import "github.com/tm-LBenson/lamda-engine/internal/engine"

func validMotion(v *engine.OverlayMotion) bool {
	if v == nil {
		return true
	}
	if v.Finish != nil && v.Finish.Duration > 30 {
		return false
	}
	for _, e := range []*engine.OverlayMotionEffect{v.Start, v.Main, v.Finish} {
		if e == nil {
			continue
		}
		if !bounded(e.Duration, .05, 300) || !one(e.Easing, "", "linear", "in", "out", "inOut") ||
			!one(e.Cycle, "", "repeat", "pingpong") || !bounded(e.X, -2048, 2048) || !bounded(e.Y, -2048, 2048) ||
			!bounded(e.Rotation, -3600, 3600) || e.Color != "" && colorValue(e.Color) == "" {
			return false
		}
		for _, field := range []struct {
			n   *float64
			max float64
		}{{e.Alpha, 100}, {e.ScaleX, 4}, {e.ScaleY, 4}} {
			if field.n != nil && !bounded(*field.n, 0, field.max) {
				return false
			}
		}
	}
	return true
}

func resolveMotion(v *engine.OverlayMotion) *engine.OverlayMotion {
	if v == nil {
		return nil
	}
	copy := *v
	for _, e := range []**engine.OverlayMotionEffect{&copy.Start, &copy.Main, &copy.Finish} {
		if *e == nil {
			continue
		}
		next := **e
		if next.Color != "" {
			next.Color = colorValue(next.Color)
		}
		*e = &next
	}
	return &copy
}
