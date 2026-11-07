package overlays

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func actionRule() Rule {
	r := rule()
	r.Roots[0].Props["event"] = "state"
	first := r.Roots[0].Children[0]
	first.Props["check"] = "active"
	first.Children = []Node{{ID: "first-call", Type: "call", Props: map[string]any{"action": "notice"}}}
	second := Node{ID: "second-buff", Type: "if", Props: map[string]any{"condition": "buff", "aura": "456", "unit": "player", "check": "active"}, Children: []Node{{ID: "second-call", Type: "call", Props: map[string]any{"action": "notice"}}}}
	r.Roots[0].Children = []Node{first, second}
	r.Roots = append(r.Roots, Node{ID: "notice", Type: "action", Props: map[string]any{"name": "Notice"}, Children: []Node{{ID: "notice-show", Type: "show", Props: map[string]any{"display": "text", "text": "{name}: {stacks}", "visibility": "conditions"}}}})
	return r
}

func TestActionsCallerSourcesIndependentIdentityAndGuards(t *testing.T) {
	rule := actionRule()
	before, _ := json.Marshal(rule)
	compiled, err := compileActions(rule)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(rule)
	if string(before) != string(after) || len(compiled.Roots) != 1 {
		t.Fatal("compiler changed declarations")
	}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	packet := sample([]Rule{rule}, now, 1)
	packet.Overlays.Native = true
	packet.Overlays.Watches = append(packet.Overlays.Watches, lookup.OverlayAura{Spell: 456, Kind: "buff", Unit: "player", Available: true, Active: ptr(true), Stacks: ptr(2.0)})
	s.Accept(packet, now)
	ds := s.Merge(engine.FrameDisplay{}, now).Overlays
	if len(ds) != 2 {
		t.Fatal("calls did not both show", ds)
	}
	texts := map[string]bool{}
	for _, d := range ds {
		texts[d.Text] = true
	}
	if !texts["Spell 123: 3"] || !texts["Spell 456: 2"] || ds[0].ID == ds[1].ID {
		t.Fatal("caller sources/identity mixed", ds)
	}
	packet.Overlays.Seq++
	packet.Overlays.Watches[0].Active = ptr(false)
	s.Accept(packet, now.Add(100*time.Millisecond))
	ds = s.Merge(engine.FrameDisplay{}, now.Add(100*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Text != "Spell 456: 2" {
		t.Fatal("one caller ended another", ds)
	}
	// Saving a definition edits every caller while leaving stored calls intact.
	rule.Roots[1].Children[0].Props["text"] = "Changed {name}"
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	packet.Overlays.Seq++
	s.Accept(packet, now.Add(200*time.Millisecond))
	ds = s.Merge(engine.FrameDisplay{}, now.Add(200*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Text != "Changed Spell 456" {
		t.Fatal("stale compiled definition", ds)
	}
}

func TestActionsKeepCloneActors(t *testing.T) {
	r := cloneRule()
	condition := &r.Roots[0].Children[0].Children[0]
	show := condition.Children[0]
	condition.Children = []Node{{ID: "call-notice", Type: "call", Props: map[string]any{"action": "notice"}}}
	r.Roots = append(r.Roots, Node{ID: "notice", Type: "action", Props: map[string]any{"name": "Notice"}, Children: []Node{show}})
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	s, _ := Open(t.TempDir())
	s.Save(0, []Rule{r})
	now := time.Unix(2000000000, 0)
	s.Accept(cloneSample([]Rule{r}, now, 1), now)
	ds := s.Merge(engine.FrameDisplay{}, now).Overlays
	if len(ds) != 2 {
		t.Fatal(ds)
	}
	for _, d := range ds {
		if d.Actor == "Player-1-B" && d.Text != "Bob: 1" {
			t.Fatal("call lost clone context", d)
		}
	}
}

func TestActionsUnusedWatchesAndValidation(t *testing.T) {
	r := actionRule()
	r.Roots = append(r.Roots, Node{ID: "unused", Type: "action", Props: map[string]any{"name": "Unused"}, Children: []Node{{ID: "unused-source", Type: "show", Props: map[string]any{"display": "text", "text": "{name}", "visibility": "conditions", "dataSource": "buff", "dataID": "789", "dataUnit": "focus"}}}})
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	if len(Watches([]Rule{r})) != 2 {
		t.Fatal("unused definition adds watches")
	}
	raw, _ := json.Marshal(r)
	for _, change := range []func(*Rule){
		func(r *Rule) { r.Roots[0].Children[0].Children[0].Props["action"] = "missing" },
		func(r *Rule) {
			r.Roots[1].Children = []Node{{ID: "self", Type: "call", Props: map[string]any{"action": "notice"}}}
		},
		func(r *Rule) {
			r.Roots[2].Children = []Node{{ID: "self", Type: "call", Props: map[string]any{"action": "unused"}}}
		},
		func(r *Rule) { r.Roots[1].Children = []Node{r.Roots[2]} },
		func(r *Rule) { r.Roots[2].Children[0].Props["dataID"] = "0" },
		func(r *Rule) { r.Roots[2].Props["future"] = true },
		func(r *Rule) { r.Roots[1].Type = "sequence" },
	} {
		var copy Rule
		json.Unmarshal(raw, &copy)
		change(&copy)
		if Validate([]Rule{copy}) == nil {
			t.Fatal("invalid action accepted", copy)
		}
	}
	// Expansion work is bounded even for a short, acyclic doubling graph.
	r = actionRule()
	r.Roots[1].Children = nil
	previous := "notice"
	for i := 0; i < 7; i++ {
		key := fmt.Sprintf("def%d", i)
		r.Roots = append(r.Roots, Node{ID: key, Type: "action", Props: map[string]any{"name": key}, Children: []Node{{ID: key + "left", Type: "call", Props: map[string]any{"action": previous}}, {ID: key + "right", Type: "call", Props: map[string]any{"action": previous}}}})
		previous = key
	}
	r.Roots[0].Children[0].Children[0].Props["action"] = previous
	if Validate([]Rule{r}) == nil {
		t.Fatal("excessive expansion accepted")
	}
}

func TestActionsDelayAndNestedCalls(t *testing.T) {
	r := actionRule()
	r.Roots[0].Children = r.Roots[0].Children[:1]
	show := r.Roots[1].Children[0]
	r.Roots[1].Children = []Node{{ID: "delay", Type: "wait", Props: map[string]any{"seconds": 0.2}}, {ID: "inner-call", Type: "call", Props: map[string]any{"action": "inner"}}}
	r.Roots = append(r.Roots, Node{ID: "inner", Type: "action", Props: map[string]any{"name": "Inner"}, Children: []Node{show}})
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	packet := sample([]Rule{r}, now, 1)
	packet.Overlays.Native = true
	s.Accept(packet, now)
	if len(s.Merge(engine.FrameDisplay{}, now).Overlays) != 0 {
		t.Fatal("WAIT skipped")
	}
	if ds := s.Merge(engine.FrameDisplay{}, now.Add(300*time.Millisecond)).Overlays; len(ds) != 1 || ds[0].Text != "Spell 123: 3" {
		t.Fatal("nested delay/source failed", ds)
	}
	if !reflect.DeepEqual(s.doc.Rules[0].Roots, r.Roots) {
		t.Fatal("stored actions expanded")
	}
}

func TestActionsBrowserFixture(t *testing.T) {
	path := os.Getenv("LAMDA_ACTIONS_PARITY_FILE")
	if path == "" {
		t.Skip("fixture output not requested")
	}
	r := actionRule()
	compiled, err := compileActions(r)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(map[string]any{"raw": r, "compiled": compiled}, "", "  ")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestActionsNestedWhenAndFinishHistory(t *testing.T) {
	r := actionRule()
	r.Roots[0].Props["event"] = "enter"
	r.Roots[0].Children = r.Roots[0].Children[:1]
	show := r.Roots[1].Children[0]
	r.Roots[1].Children = []Node{{ID: "later", Type: "when", Props: map[string]any{"event": "ability"}, Children: []Node{show}}}
	r.Style.Visual = &VisualStyle{Motion: &engine.OverlayMotion{Finish: &engine.OverlayMotionEffect{Duration: .5, Alpha: ptr(0.0)}}}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	p := sample([]Rule{r}, now, 1)
	p.Overlays.Native = true
	s.Accept(p, now)
	if len(s.Merge(engine.FrameDisplay{}, now).Overlays) != 0 {
		t.Fatal("nested WHEN fired during call")
	}
	p.Overlays.Seq = 2
	p.Overlays.CastSeq = 1
	s.Accept(p, now.Add(100*time.Millisecond))
	ds := s.Merge(engine.FrameDisplay{}, now.Add(100*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Text != "Spell 123: 3" {
		t.Fatal("nested WHEN lost caller source", ds)
	}
	first := ds[0]
	p.Overlays.Seq = 3
	p.Overlays.Watches[0].Active = ptr(false)
	s.Accept(p, now.Add(200*time.Millisecond))
	ds = s.Merge(engine.FrameDisplay{}, now.Add(200*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Finishing == 0 || ds[0].Text != first.Text || ds[0].Instance != first.Instance {
		t.Fatal("call finish history lost", ds)
	}
	p.Overlays.Seq = 4
	p.Overlays.Watches[0].Active = nil
	s.Accept(p, now.Add(300*time.Millisecond))
	if len(s.Merge(engine.FrameDisplay{}, now.Add(300*time.Millisecond)).Overlays) != 0 {
		t.Fatal("unknown caller did not cancel finishing call")
	}
}
