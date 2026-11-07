package overlays

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func castRule(event string) Rule {
	r := rule()
	r.Roots[0].Props = map[string]any{"event": event, "unit": "target", "spell": ""}
	r.Roots[0].Children = []Node{{ID: "display", Type: "show", Props: map[string]any{"display": "icon", "icon": "auto", "text": "{name}", "duration": float64(5)}}}
	return r
}
func castSample(rs []Rule, at time.Time) lookup.BridgeSample {
	s := sample(rs, at, 1)
	p := s.Overlays
	p.Native = true
	p.Watches = nil
	p.Hash = WatchHash([]Watch{})
	p.CastsVersion = 1
	p.CastHash = CastHash(CastWatches(rs))
	p.Casts = []lookup.OverlayCast{{Unit: "target", Generation: 1, Available: true, Mode: ptr(1), Spell: ptr(361469), Remaining: ptr(2.0), Duration: ptr(3.0), Interruptible: ptr(true)}}
	return s
}
func TestCastingConditionsUnknownIdleModesAndFilters(t *testing.T) {
	r := castRule("state")
	s := castSample([]Rule{r}, time.Now())
	p := s.Overlays
	q := map[string]any{"condition": "casting", "unit": "target", "check": "active"}
	for _, test := range []struct {
		check string
		want  truth
	}{{"active", yes}, {"idle", no}, {"cast", yes}, {"channel", no}, {"empower", no}, {"interruptible", yes}, {"protected", no}} {
		q["check"] = test.check
		if got := predicate(q, p, "player"); got != test.want {
			t.Fatal(test, got)
		}
	}
	q["check"] = "remaining"
	q["operator"] = "lte"
	q["value"] = 2.0
	if predicate(q, p, "player") != yes {
		t.Fatal("time")
	}
	q["spell"] = "999"
	if predicate(q, p, "player") != no {
		t.Fatal("spell filter")
	}
	q["spell"] = ""
	p.Casts[0].Remaining = nil
	if predicate(q, p, "player") != unknown {
		t.Fatal("unknown time")
	}
	q["check"] = "idle"
	p.Casts[0].Mode = ptr(0)
	p.Casts[0].Spell = ptr(0)
	if predicate(q, p, "player") != yes {
		t.Fatal("known idle")
	}
	p.Casts[0].Available = false
	if predicate(map[string]any{"condition": "group", "mode": "not", "tests": []any{q}}, p, "player") != unknown {
		t.Fatal("unknown inverted")
	}
}
func TestCastEventRingOrderingRetryAndReconnect(t *testing.T) {
	r := NewRuntime()
	p := &lookup.OverlayInput{Casts: []lookup.OverlayCast{{Unit: "target", Generation: 1}}}
	if len(r.castEvents(p, true)) != 0 {
		t.Fatal("initial replay")
	}
	c := &p.Casts[0]
	c.Seq = 3
	c.Events = []lookup.OverlayCastEvent{{Seq: 3, Kind: "caststop", Age: 0, Spell: ptr(100)}, {Seq: 1, Kind: "caststart", Age: 1}, {Seq: 2, Kind: "castsuccess", Age: 0, Spell: ptr(100)}}
	if len(r.castEvents(p, false)) != 0 {
		t.Fatal("passed unreadable earlier event")
	}
	c.Events[1].Spell = ptr(100)
	out := r.castEvents(p, false)
	if len(out) != 3 || out[0].Kind != "caststart" || out[1].Kind != "castsuccess" {
		t.Fatal(out)
	}
	if len(r.castEvents(p, false)) != 0 {
		t.Fatal("duplicate")
	}
	c.Seq = 9
	c.Events = []lookup.OverlayCastEvent{{Seq: 9, Kind: "castsuccess", Spell: ptr(200)}}
	out = r.castEvents(p, false)
	if len(out) != 1 || r.problem == "" {
		t.Fatal("gap not reported")
	}
	c.Generation = 2
	c.Seq = 10
	c.Events = []lookup.OverlayCastEvent{{Seq: 10, Kind: "castsuccess", Spell: ptr(300)}}
	if len(r.castEvents(p, false)) != 0 {
		t.Fatal("new target historical replay")
	}
	c.Seq = 11
	c.Events = []lookup.OverlayCastEvent{{Seq: 11, Kind: "castsuccess", Spell: ptr(300)}}
	if len(r.castEvents(p, true)) != 0 {
		t.Fatal("reconnect replay")
	}
}
func TestCastOutcomeRetainsSpellThroughNextCastAndDelay(t *testing.T) {
	r := castRule("castsuccess")
	r.Roots[0].Props["spell"] = "361469"
	r.Roots[0].Children = append([]Node{{ID: "wait", Type: "wait", Props: map[string]any{"seconds": float64(1)}}}, r.Roots[0].Children...)
	rs := []Rule{r}
	if e := Validate(rs); e != nil {
		t.Fatal(e)
	}
	store, _ := Open(t.TempDir())
	store.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	s := castSample(rs, at)
	store.Accept(s, at)
	s.Overlays.Seq = 2
	s.Overlays.Casts[0].Seq = 1
	s.Overlays.Casts[0].Events = []lookup.OverlayCastEvent{{Seq: 1, Kind: "castsuccess", Spell: ptr(361469)}}
	s.Overlays.Casts[0].Spell = ptr(999)
	store.Accept(s, at.Add(time.Millisecond))
	if len(store.runtime.queue) != 1 {
		t.Fatal("event did not queue", store.runtime.problem)
	}
	s.Overlays.Seq = 3
	store.Accept(s, at.Add(1100*time.Millisecond))
	ds := store.Merge(engine.FrameDisplay{}, at.Add(1100*time.Millisecond)).Overlays
	if len(ds) != 1 || ds[0].Spell != 361469 {
		t.Fatal("display followed next cast", ds)
	}
	s.Overlays.Seq = 4
	s.Overlays.Casts[0].Generation = 2
	store.Accept(s, at.Add(1200*time.Millisecond))
	if len(store.runtime.displays) != 0 || len(store.runtime.displayTriggers) != 0 {
		t.Fatal("target replacement retained display")
	}
}
func TestCastNestedWatchRearmsAndMatchesUnit(t *testing.T) {
	r := castRule("caststart")
	inner := Node{ID: "finish", Type: "when", Props: map[string]any{"event": "castsuccess", "unit": "target"}, Children: r.Roots[0].Children}
	r.Roots[0].Children = []Node{inner}
	rs := []Rule{r}
	rt := NewRuntime()
	at := time.Unix(2000000000, 0)
	s := castSample(rs, at)
	rt.Accept(rs, s, at)
	s.Overlays.Seq = 2
	c := &s.Overlays.Casts[0]
	c.Seq = 2
	c.Events = []lookup.OverlayCastEvent{{Seq: 1, Kind: "caststart", Spell: ptr(100)}, {Seq: 2, Kind: "castsuccess", Spell: ptr(100)}}
	rt.Accept(rs, s, at.Add(time.Millisecond))
	if len(rt.displays) != 1 {
		t.Fatal("same capture nested events lost", rt.queue, rt.scopes)
	}
	c.Seq = 4
	c.Events = append(c.Events, lookup.OverlayCastEvent{Seq: 3, Kind: "caststart", Spell: ptr(200)}, lookup.OverlayCastEvent{Seq: 4, Kind: "castsuccess", Spell: ptr(200)})
	s.Overlays.Seq = 3
	rt.Accept(rs, s, at.Add(2*time.Millisecond))
	for _, e := range rt.displayTriggers {
		if e == nil || *e.Spell != 200 {
			t.Fatal("nested watcher failed to rearm", e)
		}
	}
	rt.currentTrigger = &triggerContext{Unit: "focus", OverlayCastEvent: lookup.OverlayCastEvent{Spell: ptr(100)}}
	if rt.castMatches(inner, s.Overlays, "player") {
		t.Fatal("wrong unit matched")
	}
}
func TestCastingSourceProgressAndLoadedWatchContinuity(t *testing.T) {
	r := castRule("state")
	show := r.Roots[0].Children[0]
	show.Props = map[string]any{"display": "bar", "text": "{remaining}/{duration}", "fill": "source", "visibility": "conditions"}
	r.Roots[0].Children = []Node{{ID: "casting", Type: "if", Props: map[string]any{"condition": "casting", "unit": "target", "check": "active"}, Children: []Node{show}}}
	rs := []Rule{r}
	store, _ := Open(t.TempDir())
	store.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	s := castSample(rs, at)
	store.Accept(s, at)
	ds := store.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 1 || ds[0].Fraction == nil || *ds[0].Fraction != 2.0/3 || ds[0].Text != "2/3" {
		t.Fatal(ds)
	}
	other := castRule("caststart")
	other.ID = "other"
	other.Roots[0].Props["unit"] = "focus"
	store.doc.Rules = append(rs, other)
	s.Overlays.Seq = 2
	store.Accept(s, at.Add(time.Millisecond))
	ds = store.Merge(engine.FrameDisplay{}, at.Add(time.Millisecond)).Overlays
	if len(ds) != 1 || store.runtime.problem != "Reload WoW for cast tracking" {
		t.Fatal(ds, store.runtime.problem)
	}
}
func TestCastWatchesCloneExpansionAndTimerBudget(t *testing.T) {
	r := castRule("caststart")
	r.Roots[0].Props = map[string]any{"event": "state"}
	r.Roots[0].Children = []Node{{ID: "each", Type: "each", Props: map[string]any{"scope": "party"}, Children: []Node{{ID: "cast", Type: "if", Props: map[string]any{"condition": "casting", "unit": "current", "check": "active"}, Children: r.Roots[0].Children}}}}
	if n := len(CastWatches([]Rule{r})); n != 5 {
		t.Fatal(n)
	}
	for i := 0; i < 41; i++ {
		u := fmt.Sprintf("raid%d", i+1)
		if i == 40 {
			u = "target"
		}
		child := Node{ID: "extra" + u, Type: "when", Props: map[string]any{"event": "caststart", "unit": u}}
		r.Roots = append(r.Roots, child)
		err := Validate([]Rule{r})
		if (i < 40) != (err == nil) {
			t.Fatal(i, err)
		}
	}
}
