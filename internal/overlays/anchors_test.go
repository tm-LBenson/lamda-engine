package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestAnchorSingleGroupHiddenAndMissing(t *testing.T) {
	rules, ds := groupFixture()
	a := AnchorStyle{Kind: "frame", Target: "Minimap", Point: "bottom", SelfPoint: "top", X: 5, Y: 10}
	for i := range rules {
		copy := a
		rules[i].Style.Anchor = &copy
	}
	if err := Validate(rules); err != nil {
		t.Fatal(err)
	}
	p := &lookup.OverlayInput{Width: 1920, Height: 1080, AnchorsVersion: 1, Anchors: []lookup.OverlayAnchor{{Kind: "frame", Target: "Minimap", Available: true, X: 1600, Y: 50, Width: 200, Height: 200}}}
	layoutGroups(ds, rules, p.Width, p.Height)
	ds = placeAnchors(ds, rules, p)
	if len(ds) != 3 || *ds[0].PixelX != 1607 || *ds[0].PixelY != 260 || *ds[1].PixelX != 1655 {
		t.Fatalf("wrong attachment: %+v", ds)
	}
	p.Anchors[0].Available = false
	if len(placeAnchors(ds, rules, p)) != 0 {
		t.Fatal("hidden frame reused stale coordinates")
	}
	p.Anchors = nil
	if len(placeAnchors(ds, rules, p)) != 0 {
		t.Fatal("unloaded frame did not stay unknown")
	}
}
func TestAnchorValidationCollectionAndPortableCopy(t *testing.T) {
	r := rule()
	r.Style.Anchor = &AnchorStyle{Kind: "unit", Target: "party1", Point: "right", SelfPoint: "left", X: 8}
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	copy := r
	copy.ID = "copy"
	copy.Roots = nil
	if len(Anchors([]Rule{r, copy})) != 1 {
		t.Fatal("anchors not deduplicated")
	}
	r.Style.Anchor.Target = "bad()"
	if Validate([]Rule{r}) == nil {
		t.Fatal("executable expression allowed")
	}
}
func TestAnchorSaveNewWatchRetainsExistingAndHidesOnlyNew(t *testing.T) {
	r := rule()
	r.Roots[0].Children[0].Children = r.Roots[0].Children[0].Children[:1]
	r.Style.Anchor = &AnchorStyle{Kind: "frame", Target: "Minimap", Point: "center", SelfPoint: "center"}
	store, _ := Open(t.TempDir())
	if err := store.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(2000000000, 0)
	s := sample([]Rule{r}, at, 1)
	s.Overlays.Native = true
	s.Overlays.AnchorsVersion = 1
	s.Overlays.Anchors = []lookup.OverlayAnchor{{Kind: "frame", Target: "Minimap", Available: true, X: 100, Y: 100, Width: 100, Height: 100}}
	store.Accept(s, at)
	if len(store.Merge(engine.FrameDisplay{}, at).Overlays) != 1 {
		t.Fatal("existing attachment missing")
	}
	copy := r
	copy.ID = "new"
	copy.Style.Anchor = &AnchorStyle{Kind: "frame", Target: "ObjectiveTrackerFrame", Point: "center", SelfPoint: "center"}
	copy.Roots = copyNodesForAnchor(r.Roots)
	if err := store.Save(1, []Rule{r, copy}); err != nil {
		t.Fatal(err)
	}
	s.Overlays.Seq = 2
	store.Accept(s, at.Add(time.Millisecond))
	d := store.Merge(engine.FrameDisplay{}, at.Add(time.Millisecond))
	if len(d.Overlays) != 1 || d.Overlays[0].Rule != r.ID {
		t.Fatal("new unloaded attachment broke existing", d.Overlays)
	}
	if store.runtime.problem != "Reload WoW for new attachments" {
		t.Fatal(store.runtime.problem)
	}
}
func copyNodesForAnchor(ns []Node) []Node {
	out := make([]Node, len(ns))
	for i, n := range ns {
		out[i] = n
		out[i].ID = "copy_" + n.ID
		out[i].Children = copyNodesForAnchor(n.Children)
	}
	return out
}
