package lookup

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Only explicit unit tokens are accepted. Arbitrary Lua or named globals are not
// unit selectors. Current is expanded by the rule compiler before publication.
func OverlayUnitToken(unit string) bool {
	if unit == "player" || unit == "pet" || unit == "target" || unit == "focus" {
		return true
	}
	for prefix, max := range map[string]int{"party": 4, "raid": 40, "boss": 8, "arena": 5, "nameplate": 40} {
		if !strings.HasPrefix(unit, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(unit, prefix)
		n, e := strconv.Atoi(suffix)
		return e == nil && n >= 1 && n <= max && strconv.Itoa(n) == suffix
	}
	return false
}

type OverlayMeasurement struct {
	Generation int      `json:"g,omitempty"`
	Unit       string   `json:"unit"`
	Kind       string   `json:"kind"`
	Resource   string   `json:"resource,omitempty"`
	Power      int      `json:"power"`
	Mod        float64  `json:"mod"`
	Available  bool     `json:"available"`
	Current    *float64 `json:"current,omitempty"`
	Max        *float64 `json:"max,omitempty"`
}

// Native metadata uses compact tuples to retain the existing strip footprint.
// Expanded records preserve diagnostic/replay JSON. parseOverlays erases all
// numeric readings from incoming metadata before native pixels are decoded.
func (m *OverlayMeasurement) UnmarshalJSON(data []byte) error {
	var out OverlayMeasurement
	if strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		type record OverlayMeasurement
		if err := json.Unmarshal(data, (*record)(&out)); err != nil {
			return err
		}
	} else {
		var tuple []json.RawMessage
		if err := json.Unmarshal(data, &tuple); err != nil || (len(tuple) != 6 && len(tuple) != 7) {
			return fmt.Errorf("invalid unit measurement tuple")
		}
		targets := []any{&out.Unit, &out.Kind, &out.Resource, &out.Power, &out.Mod, &out.Available}
		if len(tuple) == 7 {
			targets = append(targets, &out.Generation)
		}
		for i, target := range targets {
			if err := json.Unmarshal(tuple[i], target); err != nil {
				return err
			}
		}
	}
	if out.Generation < 0 || out.Generation > 2147483647 || !ValidMeasurement(out.Unit, out.Kind, out.Resource) || math.IsNaN(out.Mod) || math.IsInf(out.Mod, 0) || out.Mod < 0 || out.Mod > 1000 || out.Power < -1 || out.Power >= 32 {
		return fmt.Errorf("invalid unit measurement")
	}
	*m = out
	return nil
}
func ValidMeasurement(unit, kind, resource string) bool {
	if !OverlayUnitToken(unit) || unit == "player" {
		return false
	}
	if kind == "health" {
		return resource == ""
	}
	if kind != "resource" {
		return false
	}
	if resource == "primary" {
		return true
	}
	n, e := strconv.Atoi(resource)
	return e == nil && n >= 0 && n < 32 && strconv.Itoa(n) == resource
}
func (m OverlayMeasurement) Key() string { return m.Unit + ":" + m.Kind + ":" + m.Resource }

// The first 52 spare cells start at native x48/y0; the final 64 use row36.
// Six cells per watch: independently rendered marker/current/max copies.
func OverlayMeasurementPoint(index, field, copy int) (int, int) {
	cell := index*6 + copy*3 + field
	if cell < 52 {
		return 48 + cell*4, 0
	}
	return (cell - 52) * 4, 36
}
func decodeOverlayMeasurements(p Pixels, panelY int, input *OverlayInput) {
	if input.MeasurementsVersion != 1 || len(input.Measurements) > overlaySourceLimit(input.NativeVersion) {
		return
	}
	for i := range input.Measurements {
		m := &input.Measurements[i]
		m.Current = nil
		m.Max = nil
		if !m.Available {
			continue
		}
		values := [3]int{}
		valid := true
		for field := 0; field < 3; field++ {
			x, y := overlayMeasurementPoint(i, field, 0, input.NativeVersion)
			x2, y2 := overlayMeasurementPoint(i, field, 1, input.NativeVersion)
			a, ok := overlayNativeColor(p, x, panelY+y, input.NativeVersion)
			b, ok2 := overlayNativeColor(p, x2, panelY+y2, input.NativeVersion)
			if !ok || !ok2 || a != b {
				valid = false
				break
			}
			values[field] = a
		}
		if !valid || values[0] != 0x28c6a1 || values[2] <= 0 || values[2] >= 0xffffff || values[1] > values[2] {
			continue
		}
		mod := m.Mod
		if m.Kind == "health" {
			mod = 1
			if values[2] != 100 {
				continue
			}
		}
		if mod <= 0 || mod > 1000 || math.IsNaN(mod) || math.IsInf(mod, 0) {
			continue
		}
		current, maximum := float64(values[1])/mod, float64(values[2])/mod
		m.Current = &current
		m.Max = &maximum
	}
}
