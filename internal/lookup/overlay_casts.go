package lookup

import (
	"encoding/json"
	"fmt"
)

// A watched cast uses two timer slots. State and four recent events are native
// measurements; the envelope contains only public unit/event/sequence data.
type OverlayCast struct {
	Unit          string             `json:"unit"`
	Generation    int                `json:"g"`
	Available     bool               `json:"available"`
	Seq           int                `json:"seq"`
	Events        []OverlayCastEvent `json:"events"`
	Mode          *int               `json:"mode,omitempty"` // 0 idle, 1 cast, 2 channel, 3 empowered
	Spell         *int               `json:"spell,omitempty"`
	Remaining     *float64           `json:"remaining,omitempty"`
	Duration      *float64           `json:"duration,omitempty"`
	Interruptible *bool              `json:"interruptible,omitempty"`
}
type OverlayCastEvent struct {
	Seq   int    `json:"seq"`
	Kind  string `json:"kind"`
	Bar   int    `json:"bar"`
	Age   int    `json:"age"` // milliseconds since native event
	Spell *int   `json:"spell,omitempty"`
}

var CastEvents = [...]string{"", "caststart", "castsuccess", "caststop", "castfailed", "castinterrupt", "channelstart", "channelstop", "empowerstart", "empowerstop"}

func ValidCastEvent(kind string) bool {
	for _, k := range CastEvents[1:] {
		if k == kind {
			return true
		}
	}
	return false
}
func (c *OverlayCast) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		var parts []json.RawMessage
		if json.Unmarshal(b, &parts) != nil || len(parts) != 5 {
			return fmt.Errorf("invalid cast tuple")
		}
		for i, target := range []any{&c.Unit, &c.Generation, &c.Available, &c.Seq} {
			if e := json.Unmarshal(parts[i], target); e != nil {
				return e
			}
		}
		var rows [][]int
		if json.Unmarshal(parts[4], &rows) != nil {
			return fmt.Errorf("invalid cast events")
		}
		for _, row := range rows {
			if len(row) != 4 || row[1] < 1 || row[1] >= len(CastEvents) {
				return fmt.Errorf("invalid cast event")
			}
			c.Events = append(c.Events, OverlayCastEvent{Seq: row[0], Kind: CastEvents[row[1]], Bar: row[2], Age: row[3]})
		}
	} else {
		type raw OverlayCast
		if e := json.Unmarshal(b, (*raw)(c)); e != nil {
			return e
		}
	}
	if !OverlayUnitToken(c.Unit) || c.Generation < 0 || c.Generation > 0xfffffe || c.Seq < 0 || c.Seq > 0xfffffe || len(c.Events) > 4 {
		return fmt.Errorf("invalid cast metadata")
	}
	seen := map[int]bool{}
	for _, e := range c.Events {
		if e.Seq < 1 || e.Seq > c.Seq || c.Seq-e.Seq >= 4 || seen[e.Seq] || !ValidCastEvent(e.Kind) || e.Age < 0 || e.Age > 3000 || e.Bar < 0 || e.Bar > 2147483647 {
			return fmt.Errorf("invalid cast event metadata")
		}
		seen[e.Seq] = true
	}
	return nil
}
func decodeOverlayCasts(p Pixels, panelY int, input *OverlayInput) {
	if input.CastsVersion != 1 || len(input.Cooldowns)+len(input.Casts)*2 > overlayTimerLimit(input.NativeVersion) {
		return
	}
	for i := range input.Casts {
		c := &input.Casts[i]
		read := func(field int) (int, bool) {
			x, y := overlayTimerPoint(len(input.Cooldowns)+i*2, field, 0, input.NativeVersion)
			x2, y2 := overlayTimerPoint(len(input.Cooldowns)+i*2, field, 1, input.NativeVersion)
			a, ok := overlayNativeColor(p, x, panelY+y, input.NativeVersion)
			b, ok2 := overlayNativeColor(p, x2, panelY+y2, input.NativeVersion)
			return a, ok && ok2 && a == b && a != 0xffffff
		}
		g, ok := read(5)
		if !ok || g != c.Generation {
			continue
		}
		// An event may precede unit disappearance; its paired identity is still
		// usable. Current-state availability is deliberately independent.
		for j := range c.Events {
			e := &c.Events[j]
			field := 8 + ((e.Seq-1)%4)*2
			seq, ok := read(field + 1)
			id, ok2 := read(field)
			if ok && ok2 && seq == e.Seq && id > 0 && id < 10000000 {
				e.Spell = &id
			}
		}
		if !c.Available {
			continue
		}
		mode, ok := read(3)
		spell, ok2 := read(2)
		if !ok || mode < 0x100000 || mode > 0x100003 || !ok2 || spell < 0 || spell >= 10000000 {
			continue
		}
		mode -= 0x100000
		if (mode == 0) != (spell == 0) {
			continue
		}
		c.Mode, c.Spell = &mode, &spell
		if mode == 0 {
			continue
		}
		for field, target := range []**float64{&c.Remaining, &c.Duration} {
			v, ok := read(field)
			if ok && v >= 0x100000 && v <= 0x100000+864000 {
				f := float64(v-0x100000) / 10
				*target = &f
			}
		}
		if c.Remaining != nil && c.Duration != nil && *c.Remaining > *c.Duration {
			c.Remaining = nil
		}
		if n, ok := read(4); ok && (n == 0x100000 || n == 0x100001) {
			v := n == 0x100001
			c.Interruptible = &v
		}
	}
}
