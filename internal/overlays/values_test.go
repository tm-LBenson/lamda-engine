package overlays

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func val(v any) ValueExpr                         { return ValueExpr{Kind: "literal", Value: v} }
func calc(op string, args ...ValueExpr) ValueExpr { return ValueExpr{Kind: "op", Op: op, Args: args} }
func ref(key string) ValueExpr                    { return ValueExpr{Kind: "value", Key: key} }
func TestValueOperatorsAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		expr ValueExpr
		want any
	}{
		{"arithmetic", calc("div", calc("mul", val(8), val(3)), val(2)), 12.0},
		{"negative", calc("sub", val(2), val(5)), -3.0},
		{"mod", calc("mod", val(-5), val(2)), -1.0},
		{"clamp", calc("clamp", val(150), val(0), val(100)), 100.0},
		{"abs", calc("abs", val(-5)), 5.0},
		{"round", calc("round", val(-1.5)), -1.0},
		{"fraction", calc("lt", val(19.99), val(20)), true},
		{"bool", calc("not", val(false)), true},
		{"concat", calc("concat", val("Mana "), val(12.5), val("%")), "Mana 12.5%"},
		{"choose", calc("choose", val(false), calc("div", val(1), val(0)), val("safe")), "safe"},
		{"unknownNot", calc("not", ref("missing")), nil},
		{"unknownCompare", calc("ne", ref("missing"), val(0)), nil},
		{"allKnownFalse", calc("all", ref("missing"), val(false)), false},
		{"anyKnownTrue", calc("any", ref("missing"), val(true)), true},
		{"allUnknown", calc("all", ref("missing"), val(true)), nil},
		{"divZero", calc("div", val(1), val(0)), nil},
		{"overflow", calc("mul", val(1e12), val(2)), nil},
		{"noCoercion", calc("eq", val("2"), val(2)), nil},
		{"concatBound", calc("concat", val(strings.Repeat("x", 512)), val("y")), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newValueEvaluation(nil, &lookup.OverlayInput{}, "player", nil)
			got := e.eval(tc.expr, 0)
			want := logicLiteral(tc.want)
			if got != want {
				t.Fatalf("got %#v want %#v", got, want)
			}
		})
	}
}
func TestValueGraphValidation(t *testing.T) {
	l := &Logic{Options: []LogicOption{{Key: "threshold", Name: "Low mana", Type: "number", Default: 20.0, Min: ptr(0.0), Max: ptr(100.0)}}, Values: []NamedValue{{Key: "first", Name: "First", Expr: ref("second")}, {Key: "second", Name: "Second", Expr: ValueExpr{Kind: "option", Key: "threshold"}}}}
	if !validLogic(l) {
		t.Fatal("forward graph invalid")
	}
	if n := newValueEvaluation(l, nil, "", nil).named("first"); n.number != 20 {
		t.Fatal(n)
	}
	raw, _ := json.Marshal(l)
	for _, change := range []func(*Logic){
		func(l *Logic) { l.Values[1].Expr = ref("first") }, func(l *Logic) { l.Values[1].Expr = ref("missing") }, func(l *Logic) { l.Values[1].Expr = ValueExpr{Kind: "option", Key: "missing"} },
		func(l *Logic) { l.Options[0].Value = 101.0 }, func(l *Logic) { l.Options[0].Default = "20" }, func(l *Logic) { l.Values[1].Expr = val(math.NaN()) }, func(l *Logic) { l.Values[1].Key = l.Values[0].Key },
		func(l *Logic) {
			l.Values[0].Expr = ValueExpr{Kind: "predicate", Check: map[string]any{"condition": "value", "key": "first", "operator": "true"}}
		},
	} {
		var copy Logic
		if err := json.Unmarshal(raw, &copy); err != nil {
			t.Fatal(err)
		}
		change(&copy)
		if validLogic(&copy) {
			t.Fatal("invalid graph accepted", copy)
		}
	}
	var expr ValueExpr
	if json.Unmarshal([]byte(`{"kind":"literal","value":1,"future":true}`), &expr) == nil {
		t.Fatal("future expression fields discarded")
	}
	expanded := &Logic{Values: []NamedValue{{Key: "v0", Name: "V0", Expr: val(1)}}}
	for i := 1; i < 14; i++ {
		key := "v" + string(rune('a'+i))
		previous := expanded.Values[len(expanded.Values)-1].Key
		expanded.Values = append(expanded.Values, NamedValue{Key: key, Name: key, Expr: calc("add", ref(previous), ref(previous))})
	}
	if validLogic(expanded) {
		t.Fatal("exponential work accepted")
	}
}
func valueRule() Rule {
	r := persistentRule()
	r.Logic = &Logic{Options: []LogicOption{{Key: "threshold", Name: "Low mana", Type: "number", Default: 20.0, Min: ptr(0.0), Max: ptr(100.0)}}, Values: []NamedValue{
		{Key: "mana", Name: "Mana", Expr: ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "resource", "dataResource": "19", "dataUnit": "player"}, Field: "percent"}},
		{Key: "low", Name: "Low", Expr: calc("lt", ref("mana"), ValueExpr{Kind: "option", Key: "threshold"})},
	}}
	r.Roots[0].Children[0].Props = map[string]any{"condition": "value", "key": "low", "operator": "true"}
	show := &r.Roots[0].Children[0].Children[0]
	show.Props["text"] = "Mana {value:mana}"
	show.Props["dataSource"] = "value"
	show.Props["dataValue"] = "mana"
	show.Props["fill"] = "source"
	show.Props["display"] = "bar"
	return r
}
func TestValuesRuntimeOptionsPrecisionAppearanceAndUnknown(t *testing.T) {
	r := valueRule()
	r.Style.Modifiers = []AppearanceModifier{{Condition: map[string]any{"condition": "value", "key": "mana", "operator": "lt", "value": 19.98}, Color: "red"}}
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	for i, n := range []float64{19.97, 19.99, 20, 10, -1} {
		at := now.Add(time.Duration(i) * 100 * time.Millisecond)
		p := resourceSample(rs, at, i+1, math.Max(0, n))
		if n < 0 {
			p.Overlays.Resources = nil
		}
		s.Accept(p, at)
		ds := s.Merge(engine.FrameDisplay{}, at).Overlays
		if n >= 20 || n < 0 {
			if len(ds) != 0 {
				t.Fatal("false/unknown remained", n, ds)
			}
			continue
		}
		if len(ds) != 1 {
			t.Fatal("calculated IF missing", n, ds)
		}
		v := ds[0]
		if v.Text != "Mana "+logicNumber(n).display() || v.Fraction == nil || math.Abs(*v.Fraction-n/100) > 1e-10 {
			t.Fatal("calculation rounded or missing", v)
		}
		want := colorValue(r.Style.Color)
		if n < 19.98 {
			want = colors["red"]
		}
		if v.Color != want {
			t.Fatal("modifier lost logic", v.Color, want)
		}
	}
	r.Logic.Options[0].Value = 5.0

	if err := s.Save(s.doc.Revision, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	at := now.Add(time.Second)
	s.Accept(resourceSample([]Rule{r}, at, 9, 10), at)
	if len(s.Merge(engine.FrameDisplay{}, at).Overlays) != 0 {
		t.Fatal("option override ignored")
	}
}
func TestValueDataRetainsOriginalEventAndRawPrecision(t *testing.T) {
	p := &lookup.OverlayInput{ResourcesVersion: 1, Native: true, Resources: map[string]lookup.OverlayResource{"0": {Current: 19.97, Max: 100}}, PrimaryPower: 0}
	e := newValueEvaluation(nil, p, "player", nil)
	data := ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "resource", "dataResource": "0"}, Field: "current"}
	if v := e.eval(data, 0); v.number != 19.97 {
		t.Fatal("resource rounded", v)
	}
	c := &triggerContext{Combat: &combatEvent{Values: map[string]float64{"amount": 123456789.125}}}
	v := logicData(map[string]any{"dataSource": "trigger"}, "amount", p, "player", c)
	if v.number != 123456789.125 {
		t.Fatal("event amount lost", v)
	}
	if v := logicData(map[string]any{"dataSource": "trigger"}, "effective", p, "player", c); v.kind != "" {
		t.Fatal("unknown event value invented")
	}
}

func TestValuesWatchDiscoveryClonesAndUnusedDefinitions(t *testing.T) {
	r := cloneRule()
	r.Logic = &Logic{Values: []NamedValue{
		{Key: "stacks", Name: "Stacks", Expr: ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "buff", "dataID": "123", "dataUnit": "current"}, Field: "stacks"}},
		{Key: "unused", Name: "Unused", Expr: ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "health", "dataUnit": "target"}, Field: "percent"}},
	}}
	n := &r.Roots[0].Children[0].Children[0]
	n.Props = map[string]any{"condition": "value", "key": "stacks", "operator": "gt", "value": 0.0}
	n.Children[0].Props["text"] = "{unitName}: {value:stacks}"
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	if len(Watches(rs)) != 5 || len(MeasurementWatches(rs)) != 0 {
		t.Fatal("wrong dependency selection", Watches(rs), MeasurementWatches(rs))
	}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	p := cloneSample(rs, now, 1)
	s.Accept(p, now)
	ds := s.Merge(engine.FrameDisplay{}, now).Overlays
	if len(ds) != 2 {
		t.Fatal("clones missing", ds)
	}
	for _, v := range ds {
		want := "Alice: 2"
		if v.Actor == "Player-1-B" {
			want = "Bob: 1"
		}
		if v.Text != want {
			t.Fatal("wrong cloned value", v.Text, want)
		}
	}
	for i := range p.Overlays.Watches {
		if p.Overlays.Watches[i].Unit == "party1" {
			p.Overlays.Watches[i].Available = false
			p.Overlays.Watches[i].Active = nil
		}
	}
	p.Overlays.Seq++
	at := now.Add(100 * time.Millisecond)
	s.Accept(p, at)
	ds = s.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 1 || ds[0].Actor != "Player-1-A" {
		t.Fatal("unknown clone retained or disturbed others", ds)
	}
	// A second watched definition waiting for reload must not disable the first.
	r.Logic.Values[1].Expr = ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "buff", "dataID": "456", "dataUnit": "player"}, Field: "stacks"}
	r.Style.Modifiers = []AppearanceModifier{{Condition: map[string]any{"condition": "value", "key": "unused", "operator": "gt", "value": 0.0}, Color: "red"}}
	if err := s.Save(s.doc.Revision, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	p.Overlays.Seq++
	at = at.Add(100 * time.Millisecond)
	s.Accept(p, at)
	ds = s.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 1 || ds[0].Actor != "Player-1-A" {
		t.Fatal("new watch interrupted existing formula", ds)
	}
	if !strings.Contains(s.runtime.problem, "new auras") {
		t.Fatal("missing dependency was not reported", s.runtime.problem)
	}
}
func TestValuesKeepEventThroughWait(t *testing.T) {
	r := combatTestRule("damage")
	r.Logic = &Logic{Values: []NamedValue{{Key: "hit", Name: "Hit", Expr: ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "trigger"}, Field: "amount"}}, {Key: "twice", Name: "Twice", Expr: calc("mul", ref("hit"), val(2))}}}
	show := r.Roots[0].Children[0]
	show.Props["text"] = "{value:twice}"
	show.Props["visibility"] = "conditions"
	r.Roots[0].Children = []Node{{ID: "check", Type: "if", Props: map[string]any{"condition": "value", "key": "hit", "operator": "gte", "value": 6000.0}, Children: []Node{{ID: "pause", Type: "wait", Props: map[string]any{"seconds": 1.0}}, show}}}
	s, p, at := combatSetup(t, r)
	f := combatMine(t, "SPELL_DAMAGE")
	s.IngestCombatLog(combatAt(f, at.Add(100*time.Millisecond)), at.Add(100*time.Millisecond))
	if len(s.runtime.queue) != 1 {
		t.Fatal("formula event not queued")
	}
	other := combatMine(t, "SPELL_HEAL")
	s.IngestCombatLog(combatAt(other, at.Add(200*time.Millisecond)), at.Add(200*time.Millisecond))
	p.Overlays.Seq++
	p.Overlays.At = at.Add(time.Second).Unix()
	s.Accept(p, at.Add(1200*time.Millisecond))
	ds := s.Merge(engine.FrameDisplay{}, at.Add(1200*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Text != "12268" {
		t.Fatal("formula lost original trigger", ds)
	}
	p.Overlays.Seq++
	s.Accept(p, at.Add(1300*time.Millisecond))
	if len(s.Merge(engine.FrameDisplay{}, at.Add(1300*time.Millisecond)).Overlays) != 1 {
		t.Fatal("formula guard forgot its trigger")
	}
}

func TestValueBrowserParity(t *testing.T) {
	path := os.Getenv("LAMDA_VALUES_PARITY_FILE")
	if path == "" {
		t.Skip("browser parity fixture not requested")
	}
	type vector struct {
		Expr     ValueExpr `json:"expr"`
		Expected any       `json:"expected"`
	}
	cases := []vector{}
	add := func(e ValueExpr) {
		v := newValueEvaluation(nil, &lookup.OverlayInput{}, "player", nil).eval(e, 0)
		var raw any
		switch v.kind {
		case "number":
			raw = v.number
		case "boolean":
			raw = v.boolean
		case "text":
			raw = v.text
		}
		cases = append(cases, vector{e, raw})
	}
	for _, op := range []string{"add", "sub", "mul", "div", "mod", "min", "max", "eq", "ne", "lt", "lte", "gt", "gte"} {
		for _, n := range []float64{-20, -1.5, 0, 19.97, 1e12} {
			add(calc(op, val(n), val(2.5)))
		}
	}
	for _, op := range []string{"abs", "neg", "floor", "ceil", "round"} {
		for _, n := range []float64{-1.5, -.5, 0, .49, .5, 1.5, 19.97} {
			add(calc(op, val(n)))
		}
	}
	for _, e := range []ValueExpr{calc("div", val(1), val(0)), calc("clamp", val(15), val(0), val(10)), calc("clamp", val(15), val(10), val(0)), calc("not", ref("missing")), calc("not", val(false)), calc("all", ref("missing"), val(false)), calc("any", ref("missing"), val(true)), calc("eq", val("1"), val(1)), calc("eq", val("text"), val("text")), calc("choose", val(false), ref("missing"), val("yes")), calc("concat", val("Mana "), val(19.97)), calc("ne", ref("missing"), val(0))} {
		add(e)
	}
	b, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkValuesFortyOverlays(b *testing.B) {
	rs := make([]Rule, 40)
	for i := range rs {
		r := valueRule()
		r.ID = fmt.Sprintf("rule%d", i)
		var rename func([]Node)
		rename = func(ns []Node) {
			for j := range ns {
				ns[j].ID = fmt.Sprintf("r%d_%s", i, ns[j].ID)
				rename(ns[j].Children)
			}
		}
		rename(r.Roots)
		rs[i] = r
	}
	if err := Validate(rs); err != nil {
		b.Fatal(err)
	}
	s, _ := Open(b.TempDir())
	s.doc.Rules = rs
	now := time.Unix(2000000000, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		at := now.Add(time.Duration(i) * 100 * time.Millisecond)
		p := resourceSample(rs, at, i+1, 15)
		s.Accept(p, at)
		if n := len(s.Merge(engine.FrameDisplay{}, at).Overlays); n != 40 {
			b.Fatal(n)
		}
	}
}
