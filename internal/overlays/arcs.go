package overlays

import (
	"math"
	"strconv"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Optional so existing arcs retain their timer, shape and thickness.
type ArcStyle struct {
	FillUnit  string  `json:"fillUnit,omitempty"`
	Pair      string  `json:"pair"`
	Start     float64 `json:"start"`
	Sweep     float64 `json:"sweep"`
	Thickness float64 `json:"thickness"`
	Gap       float64 `json:"gap"`
	Reverse   bool    `json:"reverse"`
	Fill      string  `json:"fill"`
	Resource  string  `json:"resource"`
	Repeat    string  `json:"repeat"`
	Layout    string  `json:"layout"`
	Aura      int     `json:"aura"`
	Kind      string  `json:"kind"`
	Unit      string  `json:"unit"`
	Limit     int     `json:"limit"`
}

func (a ArcStyle) valid() bool {
	resource, err := strconv.Atoi(a.Resource)
	return validUnitChoice(a.FillUnit) && bounded(a.Start, -360, 360) && bounded(a.Sweep, 1, 360) && bounded(a.Thickness, 1, 40) && bounded(a.Gap, 0, 40) &&
		one(a.Fill, "full", "timer", "health", "resource") && (a.Resource == "primary" || err == nil && resource >= 0 && resource < 32) &&
		one(a.Repeat, "single", "stacks") && one(a.Layout, "around", "nested") && one(a.Pair, "sides", "vertical") && a.Aura >= 0 && a.Aura < 10000000 &&
		one(a.Kind, "buff", "debuff") && validUnitChoice(a.Unit) && a.Limit >= 1 && a.Limit <= 20
}

func hasArcs(nodes []Node) bool {
	for _, n := range nodes {
		if n.Type == "show" && one(str(n.Props, "display"), "arc", "ring") || hasArcs(n.Children) {
			return true
		}
	}
	return false
}

// Resolve on every merge, even when the original WHEN hasn't fired again.
// Missing native readings hide the shape until a fresh valid reading arrives.
func resolveArc(a *ArcStyle, p *lookup.OverlayInput) (*engine.OverlayArc, bool) {
	v := &engine.OverlayArc{Start: a.Start, Sweep: a.Sweep, Thickness: a.Thickness, Gap: a.Gap, Reverse: a.Reverse, Layout: a.Layout, Pair: a.Pair, Count: 1, Capacity: 1, Fraction: 1, Timed: a.Fill == "timer"}
	unit := a.FillUnit
	if unit == "" {
		unit = "player"
	}
	switch a.Fill {
	case "health":
		h := unitHealth(p, unit)
		if h == nil {
			return nil, false
		}
		v.Fraction = *h / 100
	case "resource":
		r, _, ok := unitResource(p, unit, a.Resource)
		if !ok {
			return nil, false
		}
		v.Fraction = r.Current / r.Max
	}
	if a.Repeat == "stacks" {
		v.Capacity = a.Limit
		known := false
		for _, w := range p.Watches {
			if w.Spell != a.Aura || w.Kind != a.Kind || w.Unit != a.Unit {
				continue
			}
			if !w.Available || w.Active == nil {
				return nil, false
			}
			if !*w.Active {
				v.Count = 0
				return v, false
			}
			if w.Stacks == nil || !bounded(*w.Stacks, 0, 0xfffffe) {
				return nil, false
			}
			v.Count = min(a.Limit, int(math.Floor(*w.Stacks)))
			known = true
			break
		}
		if !known {
			return nil, false
		}
	}
	return v, v.Fraction > 0 && v.Count > 0
}

// A non-nil result with visible=false is a confirmed empty arc. Nil is unknown.
func resolveBoundArc(style *ArcStyle, show Node, display engine.OverlayDisplay, p *lookup.OverlayInput) (*engine.OverlayArc, bool) {
	arc := *style
	if arc.FillUnit == "current" {
		arc.FillUnit = boundUnit(p, display.Actor)
	}
	if arc.Unit == "current" {
		arc.Unit = boundUnit(p, display.Actor)
	}
	if !one(str(show.Props, "fill"), "", "timer") {
		arc.Fill = "full"
	}
	return resolveArc(&arc, p)
}
