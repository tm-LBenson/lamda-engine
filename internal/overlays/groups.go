package overlays

import (
	"math"
	"reflect"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

// Members carry identical group settings so a single exported member remains
// portable. Imports remap group IDs, keeping independent copies independent.
type DisplayGroup struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Mode    string  `json:"mode"`
	Sort    string  `json:"sort"`
	Reverse bool    `json:"reverse"`
	Gap     float64 `json:"gap"`
	Columns int     `json:"columns"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

func validGroups(rules []Rule) bool {
	groups := map[string]DisplayGroup{}
	anchors := map[string]*AnchorStyle{}
	for _, r := range rules {
		g := r.Style.Group
		if g == nil {
			continue
		}
		if !validID.MatchString(g.ID) || strings.TrimSpace(g.Name) == "" || len(g.Name) > 240 || !one(g.Mode, "static", "row", "column", "grid", "circle") || !one(g.Sort, "order", "name", "remaining", "stacks") || !bounded(g.Gap, 0, 200) || g.Columns < 1 || g.Columns > 20 || !bounded(g.X, 0, 100) || !bounded(g.Y, 0, 100) {
			return false
		}
		if old, ok := groups[g.ID]; ok && old != *g {
			return false
		}
		if old, ok := anchors[g.ID]; ok && !reflect.DeepEqual(old, r.Style.Anchor) {
			return false
		}
		anchors[g.ID] = r.Style.Anchor
		groups[g.ID] = *g
	}
	return true
}
func displayGeometry(v engine.OverlayDisplay) (float64, float64) {
	w, h := v.Size, v.Size
	if one(v.Kind, "text", "bar") {
		w, h = math.Max(v.Size*3, float64(len(utf16.Encode([]rune(v.Text))))*v.Font), math.Max(32, v.Font*2)
	}
	if v.Visual != nil {
		if v.Visual.Width > 0 {
			w = v.Visual.Width
		}
		if v.Visual.Height > 0 {
			h = v.Visual.Height
		}
	}
	return w, h
}
func layoutGroups(displays []engine.OverlayDisplay, rules []Rule, width, height int) {
	byRule := map[string]Rule{}
	order := map[string]int{}
	members := map[string][]int{}
	for i, r := range rules {
		if r.Style.Group == nil && hasEach(r.Roots) {
			r.Style.Group = &DisplayGroup{ID: "clones-" + r.ID, Name: r.Name, Mode: "row", Sort: "order", Gap: 8, Columns: 4, X: r.Style.X, Y: r.Style.Y}
		}
		byRule[r.ID] = r
		order[r.ID] = i
	}
	for i, d := range displays {
		if g := byRule[d.Rule].Style.Group; g != nil && g.Mode != "static" {
			members[g.ID] = append(members[g.ID], i)
		}
	}
	for _, indices := range members {
		g := byRule[displays[indices[0]].Rule].Style.Group
		sort.SliceStable(indices, func(a, b int) bool {
			da, db := displays[indices[a]], displays[indices[b]]
			result := 0
			switch g.Sort {
			case "name":
				result = strings.Compare(strings.ToLower(byRule[da.Rule].Name), strings.ToLower(byRule[db.Rule].Name))
			case "remaining", "stacks":
				va, vb := da.SortRemaining, db.SortRemaining
				if g.Sort == "stacks" {
					va, vb = da.SortStacks, db.SortStacks
				}
				// Unknowns are always last, even in reverse order.
				if va == nil && vb != nil {
					return false
				}
				if va != nil && vb == nil {
					return true
				}
				if va != nil && vb != nil {
					if *va < *vb {
						result = -1
					} else if *va > *vb {
						result = 1
					}
				}
			}
			if result == 0 {
				if order[da.Rule] < order[db.Rule] {
					result = -1
				} else if order[da.Rule] > order[db.Rule] {
					result = 1
				} else {
					result = strings.Compare(da.ID, db.ID)
				}
			}
			if g.Reverse {
				return result > 0
			}
			return result < 0
		})
		type point struct{ x, y, w, h float64 }
		points := make([]point, len(indices))
		maxW, maxH := 0.0, 0.0
		for j, i := range indices {
			w, h := displayGeometry(displays[i])
			points[j].w = w
			points[j].h = h
			maxW = math.Max(maxW, w)
			maxH = math.Max(maxH, h)
		}
		x, y := 0.0, 0.0
		for j := range points {
			p := &points[j]
			switch g.Mode {
			case "row":
				p.x = x
				x += p.w + g.Gap
			case "column":
				p.y = y
				y += p.h + g.Gap
			case "grid":
				p.x = float64(j%g.Columns) * (maxW + g.Gap)
				p.y = float64(j/g.Columns) * (maxH + g.Gap)
			case "circle":
				if len(points) > 1 {
					radius := (math.Hypot(maxW, maxH) + g.Gap) / (2 * math.Sin(math.Pi/float64(len(points))))
					angle := float64(j)*2*math.Pi/float64(len(points)) - math.Pi/2
					p.x = radius*math.Cos(angle) - p.w/2
					p.y = radius*math.Sin(angle) - p.h/2
				}
			}
		}
		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, p := range points {
			minX = math.Min(minX, p.x)
			minY = math.Min(minY, p.y)
			maxX = math.Max(maxX, p.x+p.w)
			maxY = math.Max(maxY, p.y+p.h)
		}
		originX := math.Max(0, math.Min(float64(width)-(maxX-minX), g.X*float64(width)/100))
		originY := math.Max(0, math.Min(float64(height)-(maxY-minY), g.Y*float64(height)/100))
		for j, i := range indices {
			x, y := originX+points[j].x-minX, originY+points[j].y-minY
			displays[i].PixelX = &x
			displays[i].PixelY = &y
		}
	}
}
