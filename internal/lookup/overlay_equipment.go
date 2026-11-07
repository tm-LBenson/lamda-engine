package lookup

import (
	"math"
	"regexp"
	"unicode/utf8"
)

// A nil item is unreadable. Item zero means a verified empty equipment slot.
type OverlayEquipmentSlot struct {
	Slot     int      `json:"slot"`
	Item     *int     `json:"item,omitempty"`
	Level    *float64 `json:"level,omitempty"`
	Set      *int     `json:"set,omitempty"`
	Class    *int     `json:"class,omitempty"`
	Subclass *int     `json:"subclass,omitempty"`
	Name     string   `json:"name,omitempty"`
}
type OverlayEquipmentSet struct {
	Key      string `json:"key"`
	Equipped *bool  `json:"equipped,omitempty"`
}
type OverlayEquipment struct {
	Slots []OverlayEquipmentSlot `json:"slots"`
	Sets  []OverlayEquipmentSet  `json:"sets"`
}

var equipmentKey = regexp.MustCompile(`^[0-9a-f]{16}$`)

func finiteRange(v, lo, hi float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= lo && v <= hi
}
func sanitizeOverlayEquipment(p *OverlayInput) {
	e := p.Equipment
	if p.EquipmentVersion != 1 || e == nil {
		p.Equipment = nil
		return
	}
	bad := len(e.Slots) > 19 || len(e.Sets) > 8
	slots := map[int]bool{}
	sets := map[string]bool{}
	for _, s := range e.Slots {
		bad = bad || s.Slot < 1 || s.Slot > 19 || slots[s.Slot] || s.Item != nil && (*s.Item < 0 || *s.Item >= 10000000) || s.Level != nil && !finiteRange(*s.Level, 0, 10000) || s.Set != nil && (*s.Set < 0 || *s.Set >= 10000000) || s.Class != nil && (*s.Class < 0 || *s.Class > 100) || s.Subclass != nil && (*s.Subclass < 0 || *s.Subclass > 100) || len(s.Name) > 80 || !utf8.ValidString(s.Name)
		bad = bad || (s.Item == nil || *s.Item == 0) && (s.Level != nil || s.Set != nil || s.Class != nil || s.Subclass != nil || s.Name != "")
		slots[s.Slot] = true
	}
	for _, s := range e.Sets {
		bad = bad || !equipmentKey.MatchString(s.Key) || sets[s.Key]
		sets[s.Key] = true
	}
	if bad {
		p.Equipment = nil
	}
}
