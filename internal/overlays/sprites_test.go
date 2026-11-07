package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"math"
	"testing"
	"time"
)

func TestSpriteBoundsAndAvailability(t *testing.T) {
	good := engine.OverlaySprite{Columns: 3, Rows: 2, Frames: 5, Start: 1, FPS: 12, Mode: "loop"}
	for _, mode := range []string{"loop", "once", "bounce", "progress"} {
		v := good
		v.Mode = mode
		if !validSprite(&v) {
			t.Fatal(mode)
		}
	}
	for _, bad := range []func(*engine.OverlaySprite){
		func(v *engine.OverlaySprite) { v.Columns = 0 }, func(v *engine.OverlaySprite) { v.Rows = 65 },
		func(v *engine.OverlaySprite) { v.Start = -1 }, func(v *engine.OverlaySprite) { v.Frames = 6 },
		func(v *engine.OverlaySprite) { v.FPS = math.NaN() }, func(v *engine.OverlaySprite) { v.FPS = 61 },
		func(v *engine.OverlaySprite) { v.Mode = "bad" }, func(v *engine.OverlaySprite) { v.Frames = 0 },
	} {
		v := good
		bad(&v)
		if validVisual(&VisualStyle{Sprite: &v}) {
			t.Fatal("invalid sprite accepted", v)
		}
	}
	if !spriteFits(&good, 23, 16) || spriteFits(&good, 2, 16) || spriteFits(&good, 24, 1) {
		t.Fatal("pixel bounds")
	}
	s, _ := Open(t.TempDir())
	lib, err := s.SaveMedia(0, []MediaUpload{{Name: "Atlas", Data: imageMedia(t)}})
	if err != nil {
		t.Fatal(err)
	}
	r := rule()
	r.Style.Visual = &VisualStyle{Image: lib.Assets[0].ID, Sprite: &good}
	show := &r.Roots[0].Children[0].Children[0]
	show.Props["display"] = "image"
	show.Props["fill"] = "full"
	r.Roots[0].Children[0].Children = append(r.Roots[0].Children[0].Children, Node{ID: "healthy", Type: "show", Props: map[string]any{"display": "text", "text": "Healthy", "duration": 5.0}})
	if err = s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	s.Accept(sample([]Rule{r}, now, 1), now)
	out := s.Merge(engine.FrameDisplay{}, now)
	if len(out.Overlays) != 2 || out.Overlays[0].Visual.Sprite.Frames != 5 {
		t.Fatal("sprite missing", out)
	}
	// A missing file or bad grid must not suppress unrelated displays.
	s.mu.Lock()
	s.doc.Rules[0].Style.Visual.Sprite.Columns = 25
	s.mu.Unlock()
	out = s.Merge(engine.FrameDisplay{}, now)
	if len(out.Overlays) != 1 || out.Overlays[0].Text != "Healthy" {
		t.Fatal("invalid grid affected healthy display", out)
	}
}

func TestDisplayClockSurvivesHeldRefreshAndMerge(t *testing.T) {
	r := persistentRule()
	rules := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	s.Accept(resourceSample(rules, now, 1, 15), now)
	first := s.Merge(engine.FrameDisplay{}, now).Overlays[0]
	if first.Started != float64(now.UnixMilli())/1000 || first.Instance == "" {
		t.Fatal("missing start clock", first)
	}
	for seq := 2; seq <= 20; seq++ {
		at := now.Add(time.Duration(seq) * 100 * time.Millisecond)
		s.Accept(resourceSample(rules, at, seq, 15), at)
		out := s.Merge(engine.FrameDisplay{}, at)
		if len(out.Overlays) != 1 || out.Overlays[0].Instance != first.Instance || out.Overlays[0].Started != first.Started || out.Overlays[0].Ends <= first.Ends {
			t.Fatal("lease restarted animation", seq, out)
		}
	}
	at := now.Add(3 * time.Second)
	s.Accept(resourceSample(rules, at, 21, 50), at)
	if len(s.Merge(engine.FrameDisplay{}, at).Overlays) != 0 {
		t.Fatal("condition end retained display")
	}
	at = at.Add(time.Second)
	s.Accept(resourceSample(rules, at, 22, 15), at)
	next := s.Merge(engine.FrameDisplay{}, at).Overlays[0]
	if next.Instance == first.Instance || next.Started <= first.Started {
		t.Fatal("retrigger retained clock")
	}
}
