package overlays

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

func groupFixture() ([]Rule, []engine.OverlayDisplay) {
	g := DisplayGroup{ID: "layout", Name: "Alerts", Mode: "row", Sort: "order", Gap: 8, Columns: 2, X: 50, Y: 50}
	var rules []Rule
	var ds []engine.OverlayDisplay
	for i, name := range []string{"Charlie", "Bravo", "Alpha"} {
		r := rule()
		r.ID = name
		r.Name = name
		r.Roots = []Node{}
		gcopy := g
		r.Style.Group = &gcopy
		rules = append(rules, r)
		ds = append(ds, engine.OverlayDisplay{ID: name + "/show", Rule: name, Kind: "icon", Size: float64(40 + i*20), SortRemaining: ptr(float64(3 - i)), SortStacks: ptr(float64(i))})
	}
	return rules, ds
}
func TestGroupLayoutCompactionBoundsAndSorting(t *testing.T) {
	rules, ds := groupFixture()
	layoutGroups(ds, rules, 1920, 1080)
	if *ds[0].PixelX != 960 || *ds[1].PixelX != 1008 || *ds[2].PixelX != 1076 {
		t.Fatalf("wrong row: %+v", ds)
	}
	ds = append(ds[:1], ds[2])
	layoutGroups(ds, rules, 1920, 1080)
	if *ds[1].PixelX != 1008 {
		t.Fatal("hidden member left a hole")
	}
	for _, r := range rules {
		r.Style.Group.X = 100
		r.Style.Group.Y = 100
		r.Style.Group.Mode = "column"
	}
	layoutGroups(ds, rules, 1920, 1080)
	for _, d := range ds {
		if *d.PixelX < 0 || *d.PixelY < 0 || *d.PixelX+d.Size > 1920 || *d.PixelY+d.Size > 1080 {
			t.Fatal("group clamped each member instead of its bounds")
		}
	}
	rules, ds = groupFixture()
	for _, r := range rules {
		r.Style.Group.Sort = "remaining"
		r.Style.Group.Reverse = true
	}
	ds[1].SortRemaining = nil
	layoutGroups(ds, rules, 1920, 1080)
	if !(*ds[0].PixelX < *ds[2].PixelX && *ds[2].PixelX < *ds[1].PixelX) {
		t.Fatal("unknown did not stay last when reversed")
	}
	for _, r := range rules {
		r.Style.Group.Mode = "grid"
		r.Style.Group.Sort = "name"
		r.Style.Group.Reverse = false
	}
	layoutGroups(ds, rules, 1920, 1080)
	if !(*ds[2].PixelX == *ds[0].PixelX && *ds[0].PixelY > *ds[2].PixelY && *ds[1].PixelX > *ds[2].PixelX) {
		t.Fatal("grid sorting/columns failed")
	}
}
func TestGroupCircleDoesNotOverlapAndStaticKeepsPositions(t *testing.T) {
	rules, ds := groupFixture()
	for _, r := range rules {
		r.Style.Group.Mode = "circle"
	}
	layoutGroups(ds, rules, 1920, 1080)
	for i, a := range ds {
		for _, b := range ds[i+1:] {
			if *a.PixelX < *b.PixelX+b.Size && *a.PixelX+a.Size > *b.PixelX && *a.PixelY < *b.PixelY+b.Size && *a.PixelY+a.Size > *b.PixelY {
				t.Fatal("circle members overlap")
			}
		}
	}
	rules, ds = groupFixture()
	for _, r := range rules {
		r.Style.Group.Mode = "static"
	}
	ds[0].X = 37
	ds[0].Y = 42
	layoutGroups(ds, rules, 1920, 1080)
	if ds[0].PixelX != nil || ds[0].X != 37 || ds[0].Y != 42 {
		t.Fatal("static position changed")
	}
}
func TestGroupValidationPortableRoundtrip(t *testing.T) {
	rules, _ := groupFixture()
	raw, _ := json.Marshal(rules)
	var copy []Rule
	json.Unmarshal(raw, &copy)
	if err := Validate(copy); err != nil {
		t.Fatal(err)
	}
	copy[1].Style.Group.Gap = 9
	if Validate(copy) == nil {
		t.Fatal("inconsistent copies accepted")
	}
	for _, patch := range []func(*DisplayGroup){func(g *DisplayGroup) { g.Columns = 0 }, func(g *DisplayGroup) { g.Mode = "bad" }, func(g *DisplayGroup) { g.X = math.NaN() }, func(g *DisplayGroup) { g.Gap = -1 }} {
		r := rules[0]
		g := *r.Style.Group
		r.Style.Group = &g
		patch(&g)
		if Validate([]Rule{r}) == nil {
			t.Fatal("invalid layout accepted")
		}
	}
}
func TestGroupLiveMergeSortsResolvedDataAndOmitsEmptyArcs(t *testing.T) {
	r := rule()
	r.Roots[0].Children[0].Children = r.Roots[0].Children[0].Children[:1]
	r.Style.Group = &DisplayGroup{ID: "g", Name: "Group", Mode: "row", Sort: "stacks", Columns: 2, X: 50, Y: 50, Gap: 8}
	r.Roots[0].Children[0].Children[0].Props["dataSource"] = "health"
	r.Roots[0].Children[0].Children[0].Props["fill"] = "source"
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	sample := sample([]Rule{r}, now, 1)
	sample.Overlays.Health = ptr(0.0)
	s.Accept(sample, now)
	d := s.Merge(engine.FrameDisplay{}, now)
	if len(d.Overlays) != 0 {
		t.Fatal("zero arc reserves group slot")
	}
	sample.Overlays.Seq = 2
	sample.Overlays.Health = ptr(50.0)
	s.Accept(sample, now.Add(time.Millisecond))
	d = s.Merge(engine.FrameDisplay{}, now.Add(time.Millisecond))
	if len(d.Overlays) != 1 || d.Overlays[0].PixelX == nil {
		t.Fatal("group not positioned after data returned")
	}
}
