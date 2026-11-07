package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestMeasurementWatchScopeDeduplicationAndPlayerReuse(t *testing.T) {
	r := cloneRule()
	r.Roots[0].Children[0].Children[0].Props = map[string]any{"condition": "resource", "resource": "0", "operator": "lt", "measure": "percent", "value": float64(30)}
	if got := MeasurementWatches([]Rule{r}); len(got) != 4 {
		t.Fatal("clone resource watch expansion", got)
	}
	r.Style.Modifiers = []AppearanceModifier{{Condition: r.Roots[0].Children[0].Children[0].Props}}
	if len(MeasurementWatches([]Rule{r})) != 4 {
		t.Fatal("modifier watch not deduplicated")
	}
	r.Style.Modifiers = nil
	r.Roots[0].Children = r.Roots[0].Children[0].Children
	if len(MeasurementWatches([]Rule{r})) != 0 {
		t.Fatal("player consumed new measurement slots")
	}
	r.Roots[0].Children[0].Props["unit"] = "target"
	if got := MeasurementWatches([]Rule{r}); len(got) != 1 || got[0].Unit != "target" {
		t.Fatal(got)
	}
}
func TestMeasurementHelpersPreserveUnknownAndExplicitUnit(t *testing.T) {
	h, max, current := 42.0, 100.0, 3.0
	p := &lookup.OverlayInput{Health: &h, MeasurementsVersion: 1, Measurements: []lookup.OverlayMeasurement{{Unit: "party1", Kind: "resource", Resource: "19", Power: 19, Available: true, Current: &current, Max: &max}}}
	if unitHealth(p, "party1") != nil || unitHealth(p, "player") == nil {
		t.Fatal("health inherited wrong unit")
	}
	v, power, ok := unitResource(p, "party1", "19")
	if !ok || power != 19 || v.Current != 3 {
		t.Fatal(v, power, ok)
	}
	if _, _, ok = unitResource(p, "party2", "19"); ok {
		t.Fatal("missing unit resource guessed")
	}
	p.Measurements[0].Available = false
	if _, _, ok = unitResource(p, "party1", "19"); ok {
		t.Fatal("stale unavailable value reused")
	}
}
func TestUnitMeasurementRuntimeProgressAndSaveContinuity(t *testing.T) {
	r := rule()
	r.Roots[0].Props["event"] = "state"
	c := &r.Roots[0].Children[0]
	c.Props = map[string]any{"condition": "health", "unit": "target", "operator": "lt", "value": float64(50)}
	c.Children = c.Children[:1]
	c.Children[0].Props = map[string]any{"display": "bar", "text": "{unit} {percent}", "fill": "source", "visibility": "conditions"}
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	store, _ := Open(t.TempDir())
	store.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	s := sample(rs, at, 1)
	p := s.Overlays
	p.Native = true
	p.Watches = nil
	p.Hash = WatchHash([]Watch{})
	p.MeasurementsVersion = 1
	p.MeasurementHash = MeasurementHash(MeasurementWatches(rs))
	p.Measurements = []lookup.OverlayMeasurement{{Unit: "target", Kind: "health", Mod: 1, Available: true, Current: ptr(25.0), Max: ptr(100.0), Generation: 1}}
	store.Accept(s, at)
	ds := store.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 1 || ds[0].Fraction == nil || *ds[0].Fraction != .25 || ds[0].Text != "target 25" {
		t.Fatal(ds, store.runtime.problem)
	}
	copy := r
	copy.ID = "new"
	copy.Roots = copyNodesForAnchor(r.Roots)
	copy.Roots[0].Children[0].Props = map[string]any{"condition": "health", "unit": "focus", "operator": "lt", "value": float64(50)}
	store.doc.Rules = append(rs, copy)
	p.Seq = 2
	store.Accept(s, at.Add(time.Millisecond))
	ds = store.Merge(engine.FrameDisplay{}, at.Add(time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Rule != r.ID {
		t.Fatal("new unloaded unit broke old reading", ds)
	}
	if store.runtime.problem != "Reload WoW for new unit readings" {
		t.Fatal(store.runtime.problem)
	}
	p.Seq = 3
	p.Measurements[0].Available = false
	store.Accept(s, at.Add(2*time.Millisecond))
	if len(store.Merge(engine.FrameDisplay{}, at.Add(2*time.Millisecond)).Overlays) != 0 {
		t.Fatal("unavailable unit still shown")
	}
	if predicate(map[string]any{"condition": "group", "mode": "not", "tests": []any{c.Props}}, p, "player") != unknown {
		t.Fatal("unknown negation matched")
	}
}
func TestUnitReplacementCancelsPreviousDelayedAction(t *testing.T) {
	r := rule()
	r.Roots[0].Props["event"] = "state"
	c := &r.Roots[0].Children[0]
	c.Props = map[string]any{"condition": "health", "unit": "target", "operator": "lt", "value": float64(50)}
	c.Children = append([]Node{{ID: "delay", Type: "wait", Props: map[string]any{"seconds": float64(2)}}}, c.Children[:1]...)
	rs := []Rule{r}
	rt := NewRuntime()
	at := time.Unix(2000000000, 0)
	s := sample(rs, at, 1)
	p := s.Overlays
	p.Native = true
	p.Watches = nil
	p.Hash = WatchHash([]Watch{})
	p.MeasurementsVersion = 1
	p.MeasurementHash = MeasurementHash(MeasurementWatches(rs))
	p.Measurements = []lookup.OverlayMeasurement{{Unit: "target", Kind: "health", Available: true, Current: ptr(25.0), Max: ptr(100.0), Generation: 1}}
	rt.Accept(rs, s, at)
	p.Seq = 2
	p.Measurements[0].Generation = 2
	rt.Accept(rs, s, at.Add(500*time.Millisecond))
	rt.advance(at.Add(2100 * time.Millisecond))
	if len(rt.displays) != 0 {
		t.Fatal("old target's delayed display fired")
	}
	rt.advance(at.Add(2600 * time.Millisecond))
	if len(rt.displays) != 1 {
		t.Fatal("new target did not get independent timer")
	}
}
