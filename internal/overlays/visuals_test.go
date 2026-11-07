package overlays

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

func TestVisualValidationAndPersistence(t *testing.T) {
	r := rule()
	r.Style.Visual = &VisualStyle{Width: 280, Height: 48, Rotation: -35, FlipX: true, Layer: 2, Shape: "arrow", Direction: "up", TrackColor: "violet", TrackOpacity: ptr(0.0), FillOpacity: ptr(100.0), TextAlign: "left", BarIcon: "right", FontFamily: "georgia", Bold: ptr(false), Italic: true, TextEffect: "outline"}
	r.Roots[0].Children[0].Children[0].Props["display"] = "shape"
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	var restored Rule
	if err = json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored.Style.Visual, r.Style.Visual) {
		t.Fatal("visual data lost", err)
	}
	for _, bad := range []func(*VisualStyle){
		func(v *VisualStyle) { v.Width = 2 }, func(v *VisualStyle) { v.Height = math.NaN() },
		func(v *VisualStyle) { v.Rotation = math.Inf(1) }, func(v *VisualStyle) { v.Layer = 101 },
		func(v *VisualStyle) { v.Shape = "url(file)" }, func(v *VisualStyle) { v.TrackColor = "invalid" },
		func(v *VisualStyle) { v.FillOpacity = ptr(101.0) }, func(v *VisualStyle) { v.TrackOpacity = ptr(-1.0) },
		func(v *VisualStyle) { v.Direction = "sideways" }, func(v *VisualStyle) { v.BarIcon = "outside" },
		func(v *VisualStyle) { v.TextAlign = "bad" },
		func(v *VisualStyle) { v.FontFamily = "url(file)" }, func(v *VisualStyle) { v.TextEffect = "unknown" },
	} {
		json.Unmarshal(raw, &restored)
		bad(restored.Style.Visual)
		if Validate([]Rule{restored}) == nil {
			t.Fatal("invalid visual accepted", restored.Style.Visual)
		}
	}
	r.Style.Visual = &VisualStyle{}
	if err = Validate([]Rule{r}); err != nil {
		t.Fatal("automatic dimensions rejected", err)
	}
}

func TestVisualGeometryGroupsAndStablePaintOrder(t *testing.T) {
	rules, ds := groupFixture()
	ds[0].Visual = &VisualStyle{Width: 200, Height: 30, Rotation: 90, FlipY: true, Layer: 9}
	ds[1].Visual = &VisualStyle{Width: 60, Height: 200, Layer: -2}
	layoutGroups(ds, rules, 1920, 1080)
	if *ds[1].PixelX != 1168 || *ds[2].PixelX != 1236 {
		t.Fatal("group ignored explicit dimensions", ds)
	}
	w, h := displayGeometry(ds[0])
	if w != 200 || h != 30 {
		t.Fatal("transform changed layout box")
	}
	legacy := engine.OverlayDisplay{Kind: "text", Size: 40, Font: 48, Text: "A😀"}
	w, h = displayGeometry(legacy)
	if w != 144 || h != 96 {
		t.Fatal("legacy text geometry changed")
	}
	legacy.Visual = &VisualStyle{Width: 100}
	w, h = displayGeometry(legacy)
	if w != 100 || h != 96 {
		t.Fatal("height must still grow when only width is set")
	}
	for i := range rules {
		rules[i].Style.Visual = ds[i].Visual
		rules[i].Roots = []Node{{ID: rules[i].ID + "-when", Type: "when", Props: map[string]any{"event": "state"}, Children: []Node{{ID: rules[i].ID + "-show", Type: "show", Props: map[string]any{"display": "shape", "text": "", "visibility": "conditions", "fill": "full"}}}}}
	}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	s.Accept(sample(rules, now, 1), now)
	for j := 0; j < 2; j++ {
		out := s.Merge(engine.FrameDisplay{}, now)
		if len(out.Overlays) != 3 || out.Overlays[0].Rule != "Bravo" || out.Overlays[1].Rule != "Alpha" || out.Overlays[2].Rule != "Charlie" {
			t.Fatal("paint layers not applied", out.Overlays)
		}
		if *out.Overlays[0].PixelX != 1168 || *out.Overlays[2].PixelX != 960 {
			t.Fatal("paint order changed layout")
		}
	}
}

func TestVisualProgressAndBarIconResolve(t *testing.T) {
	r := rule()
	show := r.Roots[0].Children[0].Children[0]
	show.Props["display"] = "shape"
	show.Props["fill"] = "source"
	show.Props["dataSource"] = "health"
	show.Props["visibility"] = "conditions"
	r.Roots[0].Children[0].Children = []Node{show}
	r.Style.Visual = &VisualStyle{Shape: "star", TrackColor: "violet", TrackOpacity: ptr(25.0)}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	for i, hp := range []*float64{ptr(25.0), ptr(0.0), nil, ptr(100.0)} {
		at := now.Add(time.Duration(i) * time.Second)
		capture := sample([]Rule{r}, at, i+1)
		capture.Overlays.Health = hp
		s.Accept(capture, at)
		out := s.Merge(engine.FrameDisplay{}, at)
		if hp == nil {
			if len(out.Overlays) != 0 {
				t.Fatal("unknown progress shown")
			}
			continue
		}
		if len(out.Overlays) != 1 || out.Overlays[0].Fraction == nil || *out.Overlays[0].Fraction != *hp/100 {
			t.Fatal("shape did not follow progress", out.Overlays)
		}
		if out.Overlays[0].Visual.TrackColor != colors["violet"] || r.Style.Visual.TrackColor != "violet" {
			t.Fatal("track color not resolved independently")
		}
	}
	show.Props["display"] = "bar"
	show.Props["icon"] = "363916"
	v := engine.OverlayDisplay{Kind: "bar", Visual: &VisualStyle{BarIcon: "left"}}
	if !resolveDisplay(&v, show, displaySource{Kind: "health", Unit: "player"}, sample([]Rule{r}, now, 1).Overlays) || v.Spell != 363916 || !v.IconRequested {
		t.Fatal("bar icon not resolved")
	}
}
