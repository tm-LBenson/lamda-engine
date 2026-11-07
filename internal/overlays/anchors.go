package overlays

import (
	"math"
	"sort"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type AnchorStyle struct {
	Kind      string  `json:"kind"`
	Target    string  `json:"target"`
	Point     string  `json:"point"`
	SelfPoint string  `json:"selfPoint"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}
type AnchorWatch struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

func (a AnchorWatch) Key() string { return a.Kind + ":" + a.Target }
func validAnchor(a *AnchorStyle) bool {
	return a == nil || (lookup.ValidAnchor(a.Kind, a.Target) || one(a.Kind, "unit", "nameplate") && a.Target == "current") && one(a.Point, "topleft", "top", "topright", "left", "center", "right", "bottomleft", "bottom", "bottomright") && one(a.SelfPoint, "topleft", "top", "topright", "left", "center", "right", "bottomleft", "bottom", "bottomright") && bounded(a.X, -4000, 4000) && bounded(a.Y, -4000, 4000)
}
func Anchors(rules []Rule) []AnchorWatch {
	rules = executionRules(rules)
	seen := map[string]AnchorWatch{}
	for _, r := range rules {
		if r.Enabled && r.Style.Anchor != nil {
			a := r.Style.Anchor
			targets := []string{a.Target}
			if a.Target == "current" {
				targets = cloneUnits(ruleScopes(r.Roots), "current")
			}
			for _, target := range targets {
				w := AnchorWatch{a.Kind, target}
				seen[w.Key()] = w
			}
		}
	}
	result := make([]AnchorWatch, 0, len(seen))
	for _, a := range seen {
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result
}
func anchorPoint(name string) (float64, float64) {
	switch name {
	case "topleft":
		return 0, 0
	case "top":
		return .5, 0
	case "topright":
		return 1, 0
	case "left":
		return 0, .5
	case "right":
		return 1, .5
	case "bottomleft":
		return 0, 1
	case "bottom":
		return .5, 1
	case "bottomright":
		return 1, 1
	}
	return .5, .5
}
func placeAnchors(displays []engine.OverlayDisplay, rules []Rule, p *lookup.OverlayInput) []engine.OverlayDisplay {
	byRule := map[string]Rule{}
	batches := map[string][]int{}
	hidden := map[int]bool{}
	for _, r := range rules {
		byRule[r.ID] = r
	}
	for i, d := range displays {
		r := byRule[d.Rule]
		if r.Style.Anchor != nil {
			key := r.ID
			if r.Style.Group != nil {
				key = "group/" + r.Style.Group.ID
			}
			if r.Style.Anchor.Target == "current" {
				key += "/" + d.Actor
			}
			batches[key] = append(batches[key], i)
		}
	}
	for _, indices := range batches {
		a := byRule[displays[indices[0]].Rule].Style.Anchor
		target := a.Target
		if target == "current" {
			target = boundUnit(p, displays[indices[0]].Actor)
		}
		var rect *lookup.OverlayAnchor
		if p.AnchorsVersion == 1 {
			for i, v := range p.Anchors {
				if v.Available && v.Kind == a.Kind && v.Target == target {
					rect = &p.Anchors[i]
					break
				}
			}
		}
		if rect == nil {
			for _, i := range indices {
				hidden[i] = true
			}
			continue
		}
		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, i := range indices {
			d := &displays[i]
			w, h := displayGeometry(*d)
			if d.PixelX == nil {
				x := d.X * float64(p.Width) / 100
				y := d.Y * float64(p.Height) / 100
				d.PixelX = &x
				d.PixelY = &y
			}
			minX = math.Min(minX, *d.PixelX)
			minY = math.Min(minY, *d.PixelY)
			maxX = math.Max(maxX, *d.PixelX+w)
			maxY = math.Max(maxY, *d.PixelY+h)
		}
		px, py := anchorPoint(a.Point)
		sx, sy := anchorPoint(a.SelfPoint)
		x, y := rect.X+rect.Width*px-(maxX-minX)*sx+a.X, rect.Y+rect.Height*py-(maxY-minY)*sy+a.Y
		for _, i := range indices {
			*displays[i].PixelX += x - minX
			*displays[i].PixelY += y - minY
		}
	}
	result := displays[:0]
	for i, d := range displays {
		if !hidden[i] {
			result = append(result, d)
		}
	}
	return result
}
