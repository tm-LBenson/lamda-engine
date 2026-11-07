package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cloneRule() Rule {
	r := rule()
	r.Roots[0].Props["event"] = "state"
	c := r.Roots[0].Children[0]
	c.Props["unit"] = "current"
	c.Props["check"] = "active"
	c.Children = c.Children[:1]
	c.Children[0].Props = map[string]any{"display": "text", "text": "{unitName}: {stacks}", "visibility": "conditions", "duration": float64(6)}
	r.Roots[0].Children = []Node{{ID: "each", Type: "each", Props: map[string]any{"scope": "party"}, Children: []Node{c}}}
	return r
}
func cloneSample(rules []Rule, at time.Time, seq int) lookup.BridgeSample {
	s := sample(rules, at, seq)
	p := s.Overlays
	p.Native = true
	p.UnitsVersion = 1
	p.Role = "healer"
	p.Units = []lookup.OverlayUnit{{Unit: "player", ID: "Player-1-A", Name: "Alice", Role: "healer"}, {Unit: "party1", ID: "Player-1-B", Name: "Bob", Role: "damage"}, {Unit: "party2", ID: "Player-1-C", Name: "Carol", Role: "tank"}}
	p.Watches = nil
	for _, w := range Watches(rules) {
		p.Watches = append(p.Watches, lookup.OverlayAura{Spell: w.Spell, Kind: w.Kind, Unit: w.Unit, Available: true, Active: ptr(w.Unit == "player" || w.Unit == "party1"), Stacks: ptr(map[string]float64{"player": 2, "party1": 1}[w.Unit]), Remaining: ptr(6.0), Duration: ptr(12.0)})
	}
	return s
}
func TestClonesStableIdentityReorderLossAndRecovery(t *testing.T) {
	r := cloneRule()
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	if len(Watches(rs)) != 5 {
		t.Fatal(Watches(rs))
	}
	store, _ := Open(t.TempDir())
	store.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	s := cloneSample(rs, at, 1)
	store.Accept(s, at)
	ds := store.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 2 {
		t.Fatal(ds)
	}
	ids := map[string]string{}
	for _, d := range ds {
		ids[d.Actor] = d.ID
		if d.Actor == "Player-1-B" && d.Text != "Bob: 1" {
			t.Fatal(d.Text)
		}
	}
	if ds[0].PixelX == nil || ds[1].PixelX == nil || *ds[0].PixelX == *ds[1].PixelX {
		t.Fatal("clones overlap", ds)
	}
	s = cloneSample(rs, at, 2)
	s.Overlays.Units[1].Unit = "party2"
	s.Overlays.Units[2].Unit = "party1"
	for i := range s.Overlays.Watches {
		w := &s.Overlays.Watches[i]
		if w.Unit == "party1" {
			w.Active = ptr(false)
		}
		if w.Unit == "party2" {
			w.Active = ptr(true)
			w.Stacks = ptr(4.0)
		}
	}
	store.Accept(s, at.Add(100*time.Millisecond))
	ds = store.Merge(engine.FrameDisplay{}, at.Add(100*time.Millisecond)).Overlays
	if len(ds) != 2 {
		t.Fatal(ds)
	}
	for _, d := range ds {
		if d.ID != ids[d.Actor] {
			t.Fatal("identity changed", d)
		}
		if d.Actor == "Player-1-B" && d.Text != "Bob: 4" {
			t.Fatal("wrong member reading", d)
		}
	}
	s.Overlays.Seq = 3
	s.Overlays.Units = s.Overlays.Units[:1]
	store.Accept(s, at.Add(200*time.Millisecond))
	ds = store.Merge(engine.FrameDisplay{}, at.Add(200*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Actor != "Player-1-A" {
		t.Fatal("departed clone retained", ds)
	}
	s.Overlays.Seq = 4
	s.Overlays.UnitsVersion = 0
	store.Accept(s, at.Add(300*time.Millisecond))
	if len(store.Merge(engine.FrameDisplay{}, at.Add(300*time.Millisecond)).Overlays) != 0 {
		t.Fatal("missing identities reused")
	}
	s = cloneSample(rs, at, 5)
	store.Accept(s, at.Add(400*time.Millisecond))
	if len(store.Merge(engine.FrameDisplay{}, at.Add(400*time.Millisecond)).Overlays) != 2 {
		t.Fatal("recovery failed")
	}
}
func TestClonesDepartureCancelsTimedAndPending(t *testing.T) {
	for _, delay := range []float64{0, 2} {
		t.Run(map[float64]string{0: "timed", 2: "pending"}[delay], func(t *testing.T) {
			r := cloneRule()
			c := &r.Roots[0].Children[0].Children[0]
			c.Children[0].Props["visibility"] = "timer"
			if delay > 0 {
				c.Children = append([]Node{{ID: "wait", Type: "wait", Props: map[string]any{"seconds": delay}}}, c.Children...)
			}
			rs := []Rule{r}
			rt := NewRuntime()
			at := time.Unix(2000000000, 0)
			s := cloneSample(rs, at, 1)
			rt.Accept(rs, s, at)
			s.Overlays.Seq = 2
			s.Overlays.Units = s.Overlays.Units[:1]
			rt.Accept(rs, s, at.Add(100*time.Millisecond))
			rt.advance(at.Add(2500 * time.Millisecond))
			for _, d := range rt.displays {
				if d.Actor != "Player-1-A" {
					t.Fatal("departed timed clone retained", d)
				}
			}
			for _, q := range rt.queue {
				if q.Actor != "Player-1-A" {
					t.Fatal("departed action retained")
				}
			}
		})
	}
}
func TestClonesExplicitUnitAndUnknownNegation(t *testing.T) {
	r := cloneRule()
	p := cloneSample([]Rule{r}, time.Unix(2000000000, 0), 1).Overlays
	c := r.Roots[0].Children[0].Children[0]
	if !condition(c, p, "Player-1-B") || condition(c, p, "Player-1-C") {
		t.Fatal("current unit binding failed")
	}
	c.Props["unit"] = "player"
	if !condition(c, p, "Player-1-C") {
		t.Fatal("explicit player retargeted")
	}
	c.Props["unit"] = "current"
	for i := range p.Watches {
		if p.Watches[i].Unit == "player" {
			p.Watches[i].Available = false
		}
	}
	not := map[string]any{"condition": "group", "mode": "not", "tests": []any{c.Props}}
	if predicate(not, p, "Player-1-A") != unknown || predicate(not, p, "Player-1-gone") != unknown {
		t.Fatal("unknown matched NOT")
	}
	if predicate(map[string]any{"condition": "health", "operator": "lt", "value": float64(100)}, p, "Player-1-B") != unknown {
		t.Fatal("party inherited player health")
	}
	if !condition(Node{Props: map[string]any{"condition": "role", "value": "tank"}}, p, "Player-1-C") {
		t.Fatal("party role failed")
	}
}
func TestClonesPerMemberAppearanceAndAttachments(t *testing.T) {
	r := cloneRule()
	r.Style.Modifiers = []AppearanceModifier{{Condition: map[string]any{"condition": "role", "value": "damage"}, Color: "red", Sound: "chime"}}
	r.Style.Anchor = &AnchorStyle{Kind: "unit", Target: "current", Point: "right", SelfPoint: "left", X: 8}
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	if len(Anchors([]Rule{r})) != 5 {
		t.Fatal(Anchors([]Rule{r}))
	}
	store, _ := Open(t.TempDir())
	store.doc.Rules = []Rule{r}
	at := time.Unix(2000000000, 0)
	s := cloneSample([]Rule{r}, at, 1)
	s.Overlays.AnchorsVersion = 1
	s.Overlays.Anchors = []lookup.OverlayAnchor{{Kind: "unit", Target: "player", Available: true, X: 100, Y: 100, Width: 200, Height: 50}, {Kind: "unit", Target: "party1", Available: true, X: 100, Y: 200, Width: 200, Height: 50}}
	store.Accept(s, at)
	ds := store.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 2 {
		t.Fatal(ds)
	}
	for _, d := range ds {
		if *d.PixelX != 308 {
			t.Fatal("wrong attachment", *d.PixelX)
		}
		if d.Actor == "Player-1-B" {
			if d.Color != colorValue("red") || *d.PixelY <= 200 {
				t.Fatal("member appearance/geometry", d)
			}
		} else if d.Color != colorValue("gold") {
			t.Fatal("appearance leaked", d)
		}
	}
	if len(store.runtime.sounds) != 1 {
		t.Fatal("member sound did not trigger once", store.runtime.sounds)
	}
	s.Overlays.Seq = 2
	store.Accept(s, at.Add(100*time.Millisecond))
	if len(store.runtime.sounds) != 1 {
		t.Fatal("member sound repeated")
	}
}
func TestClonesCurrentWatchScopeAndNestedValidation(t *testing.T) {
	r := cloneRule()
	c := r.Roots[0].Children[0].Children[0]
	r.Roots[0].Children = []Node{c}
	if len(Watches([]Rule{r})) != 1 {
		t.Fatal("current outside EACH expanded")
	}
	r = cloneRule()
	r.Roots[0].Children[0].Children = []Node{{ID: "nested", Type: "each", Props: map[string]any{"scope": "party"}}}
	if Validate([]Rule{r}) == nil {
		t.Fatal("nested clone explosion allowed")
	}
	r = cloneRule()
	r.Roots[0].Children[0].Props["scope"] = "unknown"
	if Validate([]Rule{r}) == nil {
		t.Fatal("unimplemented scope allowed")
	}
	r = cloneRule()
	r.Style.Modifiers = []AppearanceModifier{{Condition: map[string]any{"condition": "buff", "aura": "456", "unit": "current", "check": "active"}}}
	if len(Watches([]Rule{r})) != 10 {
		t.Fatal("modifier watches did not expand")
	}
	store, _ := Open(t.TempDir())
	if err := store.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "OverlaysData.lua")
	os.WriteFile(path, []byte(Marker+"\nLamdaOverlaysData={}\n"), 0600)
	store.SetGameFile(path)
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `["units"]=true`) {
		t.Fatal("unit tracking flag missing", string(b))
	}
}
func TestClonesActionsStayWithTheirMember(t *testing.T) {
	for _, action := range []string{"glow", "hide"} {
		t.Run(action, func(t *testing.T) {
			r := cloneRule()
			each := &r.Roots[0].Children[0]
			each.Children = append(each.Children, Node{ID: "damage", Type: "if", Props: map[string]any{"condition": "role", "value": "damage"}, Children: []Node{{ID: "action", Type: action, Props: map[string]any{"color": "red"}}}})
			rs := []Rule{r}
			rt := NewRuntime()
			at := time.Unix(2000000000, 0)
			rt.Accept(rs, cloneSample(rs, at, 1), at)
			if action == "hide" && len(rt.displays) != 1 {
				t.Fatal("hide affected other clones", rt.displays)
			}
			for _, d := range rt.displays {
				if d.Actor == "Player-1-A" && d.Glow || d.Actor == "Player-1-B" && (action == "hide" || !d.Glow) {
					t.Fatal("action leaked", d)
				}
			}
		})
	}
}
func TestClonesDataSourceKeepsEnclosingScope(t *testing.T) {
	r := cloneRule()
	cond := r.Roots[0].Children[0].Children[0]
	each := r.Roots[0].Children[0]
	each.Children = cond.Children
	cond.Children = []Node{each}
	r.Roots[0].Children = []Node{cond}
	_, source, ok := showSource(r.Roots, "show", displaySource{})
	if !ok || source.Unit != "player" {
		t.Fatal("outer IF rebound inside clone", source)
	}
	r = cloneRule()
	_, source, ok = showSource(r.Roots, "show", displaySource{})
	if !ok || source.Unit != "current" {
		t.Fatal("clone IF lost binding", source)
	}
}
