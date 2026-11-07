package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func finishRule() Rule {
	r := persistentRule()
	r.Style.Visual = &VisualStyle{Motion: &engine.OverlayMotion{Finish: &engine.OverlayMotionEffect{Duration: .5, Alpha: ptr(0.0)}}}
	return r
}
func TestFinishKnownEndUnknownAndRetrigger(t *testing.T) {
	now := time.Unix(2000000000, 0)
	r := finishRule()
	rules := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	seq := 0
	at := now
	step := func(offset time.Duration, current float64) engine.FrameDisplay {
		seq++
		at = now.Add(offset)
		s.Accept(resourceSample(rules, at, seq, current), at)
		return s.Merge(engine.FrameDisplay{}, at)
	}
	first := step(0, 15).Overlays[0]
	out := step(100*time.Millisecond, 50)
	if len(out.Overlays) != 1 || out.Overlays[0].Finishing == 0 || len(s.runtime.displays) != 0 || len(s.runtime.retired) != 1 {
		t.Fatal("known false did not finish", out.Overlays)
	}
	finishing := out.Overlays[0]
	if finishing.Instance != first.Instance || finishing.Started != first.Started || finishing.Ends <= finishing.Finishing || finishing.PixelX == nil {
		t.Fatal("finish snapshot wrong", finishing)
	}
	out = step(200*time.Millisecond, 50)
	if len(out.Overlays) != 1 || out.Overlays[0].Finishing != finishing.Finishing {
		t.Fatal("false capture restarted finish")
	}
	out = step(300*time.Millisecond, 15)
	if len(out.Overlays) != 1 || out.Overlays[0].Finishing != 0 || out.Overlays[0].Instance == first.Instance || len(s.runtime.retired) != 0 {
		t.Fatal("retrigger retained old visual", out.Overlays)
	}
	out = step(400*time.Millisecond, 50)
	if len(out.Overlays) != 1 {
		t.Fatal("second finish missing")
	}
	seq++
	at = now.Add(450 * time.Millisecond)
	unknownSample := resourceSample(rules, at, seq, 50)
	unknownSample.Overlays.Resources = nil
	s.Accept(unknownSample, at)
	out = s.Merge(engine.FrameDisplay{}, at)
	if len(out.Overlays) != 0 || len(s.runtime.retired) != 0 {
		t.Fatal("unknown retained retired visual", out.Overlays)
	}
	step(500*time.Millisecond, 15)
	seq++
	at = now.Add(550 * time.Millisecond)
	unknownSample = resourceSample(rules, at, seq, 15)
	unknownSample.Overlays.Resources = nil
	s.Accept(unknownSample, at)
	if out = s.Merge(engine.FrameDisplay{}, at); len(out.Overlays) != 0 || len(s.runtime.retired) != 0 {
		t.Fatal("unknown created finish")
	}
}
func TestFinishHideExpiryAndNoUnseenHistory(t *testing.T) {
	for _, kind := range []string{"hide", "timer", "unseen"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Unix(2000000000, 0)
			r := finishRule()
			show := &r.Roots[0].Children[0].Children[0]
			if kind == "timer" {
				show.Props["visibility"] = "timed"
				show.Props["duration"] = 1.0
			} else {
				r.Roots[0].Children[0].Children = append(r.Roots[0].Children[0].Children, Node{ID: "delay", Type: "wait", Props: map[string]any{"seconds": .3}}, Node{ID: "hide", Type: "hide", Props: map[string]any{}})
			}
			rules := []Rule{r}
			s, _ := Open(t.TempDir())
			if err := s.Save(0, rules); err != nil {
				t.Fatal(err)
			}
			end := 300 * time.Millisecond
			last := 4
			if kind == "timer" {
				end = time.Second
				last = 11
			}
			for seq := 1; seq <= last; seq++ {
				at := now.Add(time.Duration(seq-1) * 100 * time.Millisecond)
				s.Accept(resourceSample(rules, at, seq, 15), at)
				if kind != "unseen" && seq < last {
					s.Merge(engine.FrameDisplay{}, at)
				}
			}
			out := s.Merge(engine.FrameDisplay{}, now.Add(end))
			if kind == "unseen" {
				if len(out.Overlays) != 0 {
					t.Fatal("never-rendered SHOW/HIDE left history")
				}
				return
			}
			if len(out.Overlays) != 1 || out.Overlays[0].Finishing == 0 {
				t.Fatal("end did not finish", out.Overlays)
			}
			for seq := last + 1; seq <= last+7; seq++ {
				at := now.Add(time.Duration(seq-1) * 100 * time.Millisecond)
				s.Accept(resourceSample(rules, at, seq, 15), at)
				out = s.Merge(engine.FrameDisplay{}, at)
			}
			if len(out.Overlays) != 0 || len(s.runtime.retired) != 0 {
				t.Fatal("finish never expired")
			}
		})
	}
}
func TestFinishStaleAndWindowReset(t *testing.T) {
	for _, reset := range []string{"stale", "window", "unload", "resize"} {
		t.Run(reset, func(t *testing.T) {
			now := time.Unix(2000000000, 0)
			r := finishRule()
			if reset == "unload" {
				r.Load.Combat = "no"
			}
			rules := []Rule{r}
			s, _ := Open(t.TempDir())
			s.Save(0, rules)
			s.Accept(resourceSample(rules, now, 1, 15), now)
			s.Merge(engine.FrameDisplay{}, now)
			at := now.Add(100 * time.Millisecond)
			s.Accept(resourceSample(rules, at, 2, 50), at)
			s.Merge(engine.FrameDisplay{}, at)
			if len(s.runtime.retired) != 1 {
				t.Fatal("fixture not retiring")
			}
			if reset == "stale" {
				if len(s.Merge(engine.FrameDisplay{}, at.Add(engine.FrameLifetime+time.Millisecond)).Overlays) != 0 || len(s.runtime.retired) != 0 {
					t.Fatal("stale retained history")
				}
				return
			}
			at = at.Add(100 * time.Millisecond)
			p := resourceSample(rules, at, 3, 50)
			if reset == "window" {
				p.Window++
			}
			if reset == "resize" {
				p.Overlays.Width++
			}
			if reset == "unload" {
				p.Overlays.Combat = true
			}
			s.Accept(p, at)
			out := s.Merge(engine.FrameDisplay{}, at)
			if len(out.Overlays) != 0 || len(s.runtime.retired) != 0 {
				t.Fatal("reset retained history", reset, out.Overlays)
			}
		})
	}
}

func TestFinishGroupCompactsAndLifecycleEndsBeforeFade(t *testing.T) {
	now := time.Unix(2000000000, 0)
	a := finishRule()
	a.ID = "first"
	a.Style.OnHide = "soft"
	b := rule()
	b.ID = "second"
	b.Roots = []Node{{ID: "whenSecond", Type: "when", Props: map[string]any{"event": "state"}, Children: []Node{{ID: "showSecond", Type: "show", Props: map[string]any{"display": "icon", "visibility": "conditions", "text": ""}}}}}
	group := DisplayGroup{ID: "finishgroup", Name: "Alerts", Mode: "row", Sort: "order", Gap: 8, Columns: 2, X: 10, Y: 10}
	a.Style.Group = &group
	b.Style.Group = &group
	rules := []Rule{a, b}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	s.Accept(resourceSample(rules, now, 1, 15), now)
	first := s.Merge(engine.FrameDisplay{}, now)
	if len(first.Overlays) != 2 {
		t.Fatal("group fixture")
	}
	startX := *first.Overlays[0].PixelX
	at := now.Add(100 * time.Millisecond)
	s.Accept(resourceSample(rules, at, 2, 50), at)
	out := s.Merge(engine.FrameDisplay{}, at)
	if len(out.Overlays) != 2 {
		t.Fatal("group finish missing")
	}
	for _, v := range out.Overlays {
		if *v.PixelX != startX {
			t.Fatal("finish kept group gap or moved snapshot", v.Rule, *v.PixelX, startX)
		}
	}
	if s.runtime.lifecycle[a.ID] {
		t.Fatal("retired display counted as active")
	}
	soft := ""
	for _, v := range out.OverlaySounds {
		if v.Name == "soft" {
			if soft != "" {
				t.Fatal("duplicate hide sound")
			}
			soft = v.ID
		}
	}
	if soft == "" {
		t.Fatal("hide sound delayed until finish")
	}
	at = at.Add(100 * time.Millisecond)
	s.Accept(resourceSample(rules, at, 3, 50), at)
	out = s.Merge(engine.FrameDisplay{}, at)
	for _, v := range out.OverlaySounds {
		if v.Name == "soft" && v.ID != soft {
			t.Fatal("finish replayed sound")
		}
	}
}
func TestFinishFollowsAttachmentDeltaAndCancelsMissingFrame(t *testing.T) {
	now := time.Unix(2000000000, 0)
	r := finishRule()
	r.Style.Anchor = &AnchorStyle{Kind: "unit", Target: "player", Point: "right", SelfPoint: "left"}
	rules := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	p := resourceSample(rules, now, 1, 15)
	p.Overlays.AnchorsVersion = 1
	p.Overlays.Anchors = []lookup.OverlayAnchor{{Kind: "unit", Target: "player", Available: true, X: 100, Y: 100, Width: 80, Height: 40}}
	s.Accept(p, now)
	first := s.Merge(engine.FrameDisplay{}, now).Overlays[0]
	at := now.Add(100 * time.Millisecond)
	p = resourceSample(rules, at, 2, 50)
	p.Overlays.AnchorsVersion = 1
	p.Overlays.Anchors = []lookup.OverlayAnchor{{Kind: "unit", Target: "player", Available: true, X: 200, Y: 200, Width: 100, Height: 60}}
	s.Accept(p, at)
	out := s.Merge(engine.FrameDisplay{}, at)
	if len(out.Overlays) != 1 || *out.Overlays[0].PixelX != *first.PixelX+120 || *out.Overlays[0].PixelY != *first.PixelY+110 {
		t.Fatal("finish attachment delta wrong", out.Overlays)
	}
	at = at.Add(100 * time.Millisecond)
	p.Overlays.At = at.Unix()
	p.Overlays.Seq++
	p.Overlays.Anchors[0].Available = false
	s.Accept(p, at)
	if len(s.Merge(engine.FrameDisplay{}, at).Overlays) != 0 || len(s.runtime.retired) != 0 {
		t.Fatal("missing anchor retained finish")
	}
}

func TestFinishVisibilityChangesAndUnknownArc(t *testing.T) {
	for _, kind := range []string{"appearance", "stacks", "absent", "zeroFill"} {
		t.Run(kind, func(t *testing.T) {
			r := archRule()
			r.Style.Visual = finishRule().Style.Visual
			if kind == "appearance" {
				r.Style.Arc.Repeat = "single"
				r.Style.Modifiers = []AppearanceModifier{{Condition: group("all", map[string]any{"condition": "health", "operator": "lt", "value": 50.0}), Visible: ptr(false)}}
			}
			if kind == "zeroFill" {
				r.Style.Arc.Repeat = "single"
				r.Style.Arc.Fill = "health"
			}
			rs := []Rule{r}
			s, _ := Open(t.TempDir())
			if err := s.Save(0, rs); err != nil {
				t.Fatal(err)
			}
			now := time.Unix(2000000000, 0)
			seq := 0
			step := func(mode string) engine.FrameDisplay {
				seq++
				at := now.Add(time.Duration(seq) * 100 * time.Millisecond)
				p := sample(rs, at, seq)
				p.Overlays.Health = ptr(90.0)
				if len(p.Overlays.Watches) > 0 {
					p.Overlays.Watches[0].Stacks = ptr(2.0)
				}
				if mode == "hidden" {
					switch kind {
					case "appearance":
						p.Overlays.Health = ptr(20.0)
					case "zeroFill":
						p.Overlays.Health = ptr(0.0)
					case "stacks":
						p.Overlays.Watches[0].Stacks = ptr(0.0)
					case "absent":
						p.Overlays.Watches[0].Active = ptr(false)
					}
				}
				if mode == "unknown" {
					p.Overlays.Health = nil
					if len(p.Overlays.Watches) > 0 {
						p.Overlays.Watches[0].Active = nil
					}
				}
				s.Accept(p, at)
				return s.Merge(engine.FrameDisplay{}, at)
			}
			first := step("shown").Overlays[0]
			hidden := step("hidden")
			if len(hidden.Overlays) != 1 || hidden.Overlays[0].Finishing == 0 || len(s.runtime.displays) != 1 {
				t.Fatal("known visibility did not finish", hidden.Overlays)
			}
			at := hidden.Overlays[0].Finishing
			hidden = step("hidden")
			if len(hidden.Overlays) != 1 || hidden.Overlays[0].Finishing != at {
				t.Fatal("visibility finish restarted")
			}
			shown := step("shown")
			if len(shown.Overlays) != 1 || shown.Overlays[0].Finishing != 0 || shown.Overlays[0].Instance != first.Instance {
				t.Fatal("visibility resume replaced original clock")
			}
			step("hidden")
			unknown := step("unknown")
			if len(s.runtime.retired) != 0 {
				t.Fatal("unknown retained history")
			}
			if kind != "appearance" && len(unknown.Overlays) != 0 {
				t.Fatal("unknown arc remained")
			}
		})
	}
}

func TestFinishParentUnknownDominatesChildFalseAndPendingWait(t *testing.T) {
	r := finishRule()
	child := r.Roots[0].Children[0]
	child.Children = append(child.Children, Node{ID: "waitAfter", Type: "wait", Props: map[string]any{"seconds": 1.0}}, Node{ID: "late", Type: "show", Props: map[string]any{"display": "text", "text": "late", "visibility": "conditions"}})
	r.Roots[0].Children = []Node{{ID: "parent", Type: "if", Props: map[string]any{"condition": "health", "operator": "lt", "value": 50.0}, Children: []Node{child}}}
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	for i := 0; i < 15; i++ {
		at := now.Add(time.Duration(i) * 100 * time.Millisecond)
		p := resourceSample(rs, at, i+1, 15)
		p.Overlays.Health = ptr(20.0)
		if i > 0 {
			p.Overlays.Health = nil
			p.Overlays.Resources["19"] = lookup.OverlayResource{Current: 50, Max: 100}
		}
		s.Accept(p, at)
		out := s.Merge(engine.FrameDisplay{}, at)
		if i == 0 && len(out.Overlays) != 1 {
			t.Fatal("fixture")
		}
		if i > 0 && (len(out.Overlays) != 0 || len(s.runtime.retired) != 0 || len(s.runtime.queue) != 0) {
			t.Fatal("unknown parent left finish or delayed action")
		}
	}
}

func TestFinishCloneDepartureCancelsHistory(t *testing.T) {
	r := cloneRule()
	r.Style.Visual = finishRule().Style.Visual
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	p := cloneSample(rs, now, 1)
	s.Accept(p, now)
	s.Merge(engine.FrameDisplay{}, now)
	at := now.Add(100 * time.Millisecond)
	p = cloneSample(rs, at, 2)
	for i := range p.Overlays.Watches {
		if p.Overlays.Watches[i].Unit == "party1" {
			p.Overlays.Watches[i].Active = ptr(false)
		}
	}
	s.Accept(p, at)
	s.Merge(engine.FrameDisplay{}, at)
	if len(s.runtime.retired) != 1 {
		t.Fatal("clone finish missing")
	}
	at = at.Add(100 * time.Millisecond)
	p.Overlays.Seq++
	p.Overlays.Units = p.Overlays.Units[:1]
	s.Accept(p, at)
	out := s.Merge(engine.FrameDisplay{}, at)
	if len(s.runtime.retired) != 0 || len(out.Overlays) != 1 || out.Overlays[0].Actor != "Player-1-A" {
		t.Fatal("departed clone left history")
	}
}
