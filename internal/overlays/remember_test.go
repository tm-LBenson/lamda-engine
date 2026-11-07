package overlays

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func memoryRule() Rule {
	r := persistentRule()
	r.Roots[0].Props = map[string]any{"event": "state"}
	r.Roots[0].Children = []Node{{ID: "memory-display", Type: "show", Props: map[string]any{"display": "text", "text": "Count {value:count}", "visibility": "conditions"}}}
	r.Logic = &Logic{Values: []NamedValue{{Key: "count", Name: "Count", Expr: calc("add", ref("count"), val(1)), Remember: &RememberValue{Initial: 0.0, When: map[string]any{"event": "ability"}, Reset: "loaded"}}}}
	return r
}

func TestRememberCaptureAndResetLifecycle(t *testing.T) {
	rule := memoryRule()
	s, _ := Open(t.TempDir())
	if err := s.Save(0, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	seq := 0
	accept := func(cast int, combat bool, change func(*lookup.OverlayInput), expected string) {
		seq++
		at := now.Add(time.Duration(seq) * 100 * time.Millisecond)
		sample := resourceSample([]Rule{rule}, at, seq, 30)
		sample.Overlays.CastSeq = cast
		sample.Overlays.Combat = combat
		if change != nil {
			change(sample.Overlays)
		}
		s.Accept(sample, at)
		ds := s.Merge(engine.FrameDisplay{}, at).Overlays
		if len(ds) != 1 || ds[0].Text != expected {
			t.Fatalf("seq=%d: %#v, want %s", seq, ds, expected)
		}
	}
	accept(2, false, nil, "Count 0") // Connecting does not replay past casts.
	accept(3, false, nil, "Count 1")
	accept(3, false, nil, "Count 1")
	accept(4, false, nil, "Count 2")
	// Saving definitions clears history without rewriting the initial value.
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	accept(4, false, nil, "Count 0")
	rule.Logic.Values[0].Remember.Reset = "combat"
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	accept(4, false, nil, "Count 0")
	accept(5, false, nil, "Count 1")
	accept(5, true, nil, "Count 0")
	accept(6, true, nil, "Count 1")
	// A new character must never inherit another character's counter.
	accept(6, true, func(p *lookup.OverlayInput) { p.Character = "Another" }, "Count 0")
	if len(s.Merge(engine.FrameDisplay{}, now.Add(time.Minute)).Overlays) != 0 || len(s.runtime.memory) != 0 {
		t.Fatal("stale memory survived")
	}
}

func TestRememberSimultaneousUpdatesAndUnknown(t *testing.T) {
	rule := memoryRule()
	rule.Roots[0].Children[0].Props["text"] = "{value:a} {value:b}"
	rule.Logic.Values = []NamedValue{
		{Key: "a", Name: "A", Expr: ref("b"), Remember: &RememberValue{Initial: 1.0, When: map[string]any{"event": "ability"}, Reset: "loaded"}},
		{Key: "b", Name: "B", Expr: ref("a"), Remember: &RememberValue{Initial: 2.0, When: map[string]any{"event": "ability"}, Reset: "loaded"}},
	}
	if err := Validate([]Rule{rule}); err != nil {
		t.Fatal(err)
	}
	r := NewRuntime()
	r.rememberEvent(rule, "ability", &lookup.OverlayInput{})
	context := r.ruleContext(rule, nil)
	e := newValueEvaluation(rule.Logic, nil, "player", context)
	if e.named("a").number != 2 || e.named("b").number != 1 {
		t.Fatal("updates read partially committed state", r.memory)
	}
	r.rememberEvent(rule, "ability", &lookup.OverlayInput{})
	e = newValueEvaluation(rule.Logic, nil, "player", context)
	if e.named("a").number != 1 || e.named("b").number != 2 {
		t.Fatal("second swap failed")
	}
	rule.Logic.Values[0].Expr = calc("div", val(1), val(0))
	rule.Logic.Values[1].Expr = val("wrong type")
	r.rememberEvent(rule, "ability", &lookup.OverlayInput{})
	e = newValueEvaluation(rule.Logic, nil, "player", context)
	if e.named("a").number != 1 || e.named("b").number != 2 {
		t.Fatal("unknown or wrong type replaced history")
	}
}

func TestRememberContinuousCaptureAndRearm(t *testing.T) {
	rule := memoryRule()
	config := rule.Logic.Values[0].Remember
	config.When = map[string]any{"event": "state"}
	config.Check = map[string]any{"condition": "health", "operator": "lt", "value": 20.0}
	r := NewRuntime()
	for i, test := range []struct {
		health *float64
		want   float64
	}{{ptr(10.0), 1}, {ptr(10.0), 1}, {ptr(30.0), 1}, {ptr(10.0), 2}, {nil, 2}, {ptr(10.0), 3}} {
		r.rememberEvent(rule, "state", &lookup.OverlayInput{Health: test.health})
		got := newValueEvaluation(rule.Logic, nil, "player", r.ruleContext(rule, nil)).named("count")
		if got.number != test.want {
			t.Fatalf("sample %d: %#v", i, got)
		}
	}
	config.When = map[string]any{"event": "resource"}
	config.Check = nil
	for i := 0; i < 3; i++ {
		r.rememberEvent(rule, "resource", &lookup.OverlayInput{})
	}
	if got := newValueEvaluation(rule.Logic, nil, "player", r.ruleContext(rule, nil)).named("count"); got.number != 6 {
		t.Fatal("resource changes did not each capture", got)
	}
}

func TestRememberWatchesFiltersAndOriginalEvent(t *testing.T) {
	rule := memoryRule()
	v := &rule.Logic.Values[0]
	v.Remember.When = map[string]any{"event": "combatlog", "combatEvent": "damage", "sourceUnit": "player", "targetUnit": "target"}
	v.Remember.Reset = "encounter"
	v.Expr = ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "trigger"}, Field: "amount"}
	if err := Validate([]Rule{rule}); err != nil {
		t.Fatal(err)
	}
	if len(CombatWatches([]Rule{rule})) != 2 || !needsEncounters([]Rule{rule}) {
		t.Fatal("remembered event dependencies missing")
	}
	r := NewRuntime()
	p := &lookup.OverlayInput{Identities: []lookup.OverlayIdentity{{Unit: "player", ID: "Player-one"}, {Unit: "target", ID: "Creature-one"}}}
	r.currentTrigger = &triggerContext{Combat: &combatEvent{Kind: "damage", Source: combatActor{GUID: "Player-one"}, Target: combatActor{GUID: "Creature-one"}, Values: map[string]float64{"amount": 1234.567}}}
	r.rememberEvent(rule, "combatlog", p)
	if got := newValueEvaluation(rule.Logic, p, "player", r.ruleContext(rule, nil)).named("count"); got.number != 1234.567 {
		t.Fatal("event not captured precisely", got)
	}
	r.currentTrigger.Combat.Target.GUID = "Creature-other"
	r.currentTrigger.Combat.Values["amount"] = 1
	r.rememberEvent(rule, "combatlog", p)
	if got := newValueEvaluation(rule.Logic, p, "player", r.ruleContext(rule, nil)).named("count"); got.number != 1234.567 {
		t.Fatal("wrong target changed capture", got)
	}
	r.currentTrigger = nil
	r.rememberEvent(rule, "encounterstart", p)
	if got := newValueEvaluation(rule.Logic, p, "player", r.ruleContext(rule, nil)).named("count"); got.number != 0 {
		t.Fatal("encounter did not reset")
	}
	rule.Roots[0].Children[0].Props["text"] = "Unused"
	if len(CombatWatches([]Rule{rule})) != 0 || needsEncounters([]Rule{rule}) || len(rememberedValues(rule)) != 0 {
		t.Fatal("unused definitions capture or add watches")
	}
	rule.Roots[0].Children[0].Props["text"] = "{value:count}"
	v.Remember.When = map[string]any{"event": "equipment"}
	if !needsLoadoutChange([]Rule{rule}, "equipment") {
		t.Fatal("remembered loadout dependency missing")
	}
}

func TestRememberValidation(t *testing.T) {
	rule := memoryRule()
	raw, _ := json.Marshal(rule)
	for _, change := range []func(*Rule){
		func(r *Rule) { r.Logic.Values[0].Remember = nil },
		func(r *Rule) { r.Logic.Values[0].Remember.Reset = "disk" },
		func(r *Rule) { r.Logic.Values[0].Remember.When["event"] = "arbitrary" },
		func(r *Rule) { r.Logic.Values[0].Remember.When["future"] = true },
		func(r *Rule) { r.Logic.Values[0].Remember.Initial = nil },
		func(r *Rule) {
			r.Logic.Values[0].Remember.Check = map[string]any{"condition": "value", "key": "count", "operator": "true"}
		},
	} {
		var copy Rule
		json.Unmarshal(raw, &copy)
		change(&copy)
		if Validate([]Rule{copy}) == nil {
			t.Fatal("invalid memory configuration accepted")
		}
	}
	var m RememberValue
	if json.Unmarshal([]byte(`{"initial":0,"when":{"event":"ability"},"reset":"loaded","future":true}`), &m) == nil {
		t.Fatal("unknown memory field silently discarded")
	}
}

func TestRememberBrowserParity(t *testing.T) {
	path := os.Getenv("LAMDA_MEMORY_PARITY_FILE")
	if path == "" {
		t.Skip("fixture output not requested")
	}
	var cases []map[string]any
	for _, kind := range []string{"counter", "health", "text", "boolean", "edge"} {
		rule := memoryRule()
		v := &rule.Logic.Values[0]
		v.Remember.Reset = "combat"
		switch kind {
		case "health":
			v.Expr = calc("max", ref("count"), ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "health"}, Field: "percent"})
		case "text":
			v.Remember.Initial = ""
			v.Expr = ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "none"}, Field: "character"}
		case "boolean":
			v.Remember.Initial = false
			v.Expr = calc("not", ref("count"))
		case "edge":
			v.Remember.When = map[string]any{"event": "state"}
			v.Remember.Check = map[string]any{"condition": "health", "operator": "lt", "value": 20.0}
		}
		r := NewRuntime()
		var steps []map[string]any
		for i, event := range []string{"ability", "ability", "state", "state", "combat", "ability", "state", "state", "ability", "state", "combat", "ability"} {
			p := &lookup.OverlayInput{Character: []string{"First", "Second", "Third"}[i%3]}
			if i%4 != 3 {
				p.Health = ptr([]float64{19.99, 2, 40}[i%3])
			}
			r.rememberEvent(rule, event, p)
			got := newValueEvaluation(rule.Logic, p, "player", r.ruleContext(rule, nil)).named("count")
			var expected any
			switch got.kind {
			case "number":
				expected = got.number
			case "text":
				expected = got.text
			case "boolean":
				expected = got.boolean
			}
			steps = append(steps, map[string]any{"sample": map[string]any{"event": event, "health": p.Health, "character": p.Character, "unit": "player"}, "expected": expected})
		}
		cases = append(cases, map[string]any{"name": kind, "rule": rule, "steps": steps})
	}
	b, _ := json.MarshalIndent(cases, "", "  ")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
