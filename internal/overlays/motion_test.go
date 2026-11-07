package overlays

import (
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"math"
	"reflect"
	"testing"
)

func TestMotionValidationPersistenceAndIsolation(t *testing.T) {
	m := &engine.OverlayMotion{Start: &engine.OverlayMotionEffect{Duration: .4, Easing: "out", Alpha: ptr(0.0), ScaleX: ptr(0.0), ScaleY: ptr(0.0), Rotation: -180, Color: "gold"}, Main: &engine.OverlayMotionEffect{Duration: 1.5, Easing: "inOut", Cycle: "pingpong", Y: -24}}
	r := rule()
	r.Style.Visual = &VisualStyle{Motion: m}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.dir)
	if err != nil || !reflect.DeepEqual(reopened.doc.Rules[0].Style.Visual.Motion, m) {
		t.Fatal("motion persistence", err)
	}
	resolved := resolveVisual(r.Style.Visual)
	if resolved.Motion.Start.Color != "#F4C879" || m.Start.Color != "gold" {
		t.Fatal("resolution mutated source")
	}
	resolved.Motion.Main.Y = 30
	if m.Main.Y != -24 {
		t.Fatal("resolution aliases effect")
	}
	raw, _ := json.Marshal(m)
	for _, bad := range []func(*engine.OverlayMotionEffect){
		func(e *engine.OverlayMotionEffect) { e.Duration = 0 }, func(e *engine.OverlayMotionEffect) { e.Duration = 301 }, func(e *engine.OverlayMotionEffect) { e.Duration = math.NaN() },
		func(e *engine.OverlayMotionEffect) { e.Easing = "bad" }, func(e *engine.OverlayMotionEffect) { e.Cycle = "bad" }, func(e *engine.OverlayMotionEffect) { e.Alpha = ptr(-1.0) },
		func(e *engine.OverlayMotionEffect) { e.Alpha = ptr(101.0) }, func(e *engine.OverlayMotionEffect) { e.X = 2049 }, func(e *engine.OverlayMotionEffect) { e.Y = math.Inf(1) },
		func(e *engine.OverlayMotionEffect) { e.ScaleX = ptr(4.1) }, func(e *engine.OverlayMotionEffect) { e.ScaleY = ptr(-.1) }, func(e *engine.OverlayMotionEffect) { e.Rotation = 3601 }, func(e *engine.OverlayMotionEffect) { e.Color = "url(file)" },
	} {
		var copy engine.OverlayMotion
		json.Unmarshal(raw, &copy)
		bad(copy.Start)
		if validVisual(&VisualStyle{Motion: &copy}) {
			t.Fatal("invalid motion accepted", copy)
		}
	}
}
