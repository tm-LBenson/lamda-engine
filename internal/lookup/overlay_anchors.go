package lookup

import (
	"math"
	"regexp"
)

type OverlayAnchor struct {
	Kind      string  `json:"kind"`
	Target    string  `json:"target"`
	Available bool    `json:"available"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	Width     float64 `json:"width,omitempty"`
	Height    float64 `json:"height,omitempty"`
}

func (a OverlayAnchor) Key() string { return a.Kind + ":" + a.Target }

var frameName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,79}$`)
var frameUnit = regexp.MustCompile(`^(player|pet|target|focus|party[1-4]|raid([1-9]|[1-3][0-9]|40)|boss[1-8]|arena[1-5]|nameplate([1-9]|[1-3][0-9]|40))$`)

func ValidAnchor(kind, target string) bool {
	if kind == "frame" {
		return frameName.MatchString(target)
	}
	return (kind == "unit" || kind == "nameplate") && frameUnit.MatchString(target)
}
func sanitizeAnchors(p *OverlayInput) {
	if p.AnchorsVersion != 1 || len(p.Anchors) > OverlayAnchorLimit {
		p.Anchors = nil
		return
	}
	seen := map[string]bool{}
	for i, a := range p.Anchors {
		if !ValidAnchor(a.Kind, a.Target) || seen[a.Key()] {
			p.Anchors = nil
			return
		}
		seen[a.Key()] = true
		valid := a.Available
		for _, n := range []float64{a.X, a.Y, a.Width, a.Height} {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				valid = false
			}
		}
		if a.Width <= 0 || a.Height <= 0 || a.X < 0 || a.Y < 0 || a.X+a.Width > float64(p.Width)+1 || a.Y+a.Height > float64(p.Height)+1 {
			valid = false
		}
		if !valid {
			p.Anchors[i] = OverlayAnchor{Kind: a.Kind, Target: a.Target}
		}
	}
}
