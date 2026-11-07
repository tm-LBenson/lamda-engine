package overlays

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func archRule() Rule {
	r := rule()
	r.Roots = []Node{{ID: "when", Type: "when", Props: map[string]any{"event": "state"}, Children: []Node{{ID: "show", Type: "show", Props: map[string]any{"display": "arc", "text": "", "visibility": "conditions", "duration": 6.0}}}}}
	r.Style.Arc = &ArcStyle{Start: 0, Sweep: 160, Thickness: 8, Gap: 6, Fill: "full", Resource: "primary", Repeat: "stacks", Layout: "around", Pair: "sides", Aura: 123, Kind: "buff", Unit: "player", Limit: 2}
	return r
}

func TestArchStacksRefreshWithoutRetriggerAndHideUnknown(t *testing.T) {
	store, _ := Open(t.TempDir())
	r := archRule()
	rules := []Rule{r}
	if e := store.Save(0, rules); e != nil {
		t.Fatal(e)
	}
	now := time.Unix(2000000000, 0)
	var id string
	for i, stacks := range []float64{1, 2, 8, 0, 2} {
		at := now.Add(time.Duration(i) * 100 * time.Millisecond)
		s := sample(rules, at, i+1)
		s.Overlays.Watches[0].Stacks = ptr(stacks)
		store.Accept(s, at)
		d := store.Merge(engine.FrameDisplay{}, at)
		if stacks == 0 {
			if len(d.Overlays) != 0 {
				t.Fatal("zero stacks rendered")
			}
			continue
		}
		if len(d.Overlays) != 1 || d.Overlays[0].Arc.Count != min(2, int(stacks)) || d.Overlays[0].Arc.Capacity != 2 {
			t.Fatalf("stacks %v: %+v", stacks, d.Overlays)
		}
		if id != "" && d.Overlays[0].ID != id {
			t.Fatal("display replaced instead of refreshed")
		}
		id = d.Overlays[0].ID
	}
	for i, change := range []func(*lookup.OverlayInput){
		func(p *lookup.OverlayInput) { p.Watches[0].Stacks = nil }, func(p *lookup.OverlayInput) { p.Watches[0].Active = nil },
		func(p *lookup.OverlayInput) { p.Watches[0].Available = false }, func(p *lookup.OverlayInput) { p.Hash = "wrong mapping" },
		func(p *lookup.OverlayInput) { p.Watches[0].Active = ptr(false) },
	} {
		at := now.Add(time.Duration(i+5) * 100 * time.Millisecond)
		s := sample(rules, at, i+6)
		change(s.Overlays)
		store.Accept(s, at)
		if len(store.Merge(engine.FrameDisplay{}, at).Overlays) != 0 {
			t.Fatal("unreadable or missing stacks rendered", i)
		}
	}
	at := now.Add(time.Second)
	store.Accept(sample(rules, at, 11), at)
	if len(store.Merge(engine.FrameDisplay{}, at).Overlays) != 1 {
		t.Fatal("fresh stacks failed to recover")
	}
	if len(store.Merge(engine.FrameDisplay{}, at.Add(2*time.Second)).Overlays) != 0 {
		t.Fatal("stale arches survived")
	}
}

func TestArchFillSourcesAndSettingsRoundTrip(t *testing.T) {
	r := archRule()
	a := r.Style.Arc
	a.Repeat = "single"
	p := sample([]Rule{r}, time.Now(), 1).Overlays
	for _, tc := range []struct {
		fill  string
		want  float64
		timed bool
	}{{"full", 1, false}, {"timer", 1, true}, {"health", .9, false}, {"resource", .25, false}} {
		a.Fill = tc.fill
		p.PrimaryPower = 19
		p.Resources = map[string]lookup.OverlayResource{"19": {Current: 1, Max: 4}}
		v, ok := resolveArc(a, p)
		if !ok || v.Fraction != tc.want || v.Timed != tc.timed {
			t.Fatalf("%s: %+v %v", tc.fill, v, ok)
		}
	}
	p.Resources["19"] = lookup.OverlayResource{Current: 4, Max: 4}
	v, ok := resolveArc(a, p)
	if !ok || v.Fraction != 1 {
		t.Fatal("cap fill")
	}
	p.Resources["19"] = lookup.OverlayResource{Current: 0, Max: 4}
	if _, ok := resolveArc(a, p); ok {
		t.Fatal("empty resource should hide")
	}
	for _, bad := range []lookup.OverlayResource{{Current: 1, Max: 0}, {Current: 5, Max: 4}, {Current: math.NaN(), Max: 4}} {
		p.Resources["19"] = bad
		if _, ok := resolveArc(a, p); ok {
			t.Fatal("invalid resource rendered")
		}
	}
	a.Fill = "health"
	p.Health = nil
	if _, ok := resolveArc(a, p); ok {
		t.Fatal("unknown health rendered")
	}
	a.Fill = "full"
	a.Repeat = "stacks"
	a.Limit = 4
	a.Sweep = 70
	a.Pair = "vertical"
	a.Reverse = true
	dir := t.TempDir()
	store, _ := Open(dir)
	if e := store.Save(0, []Rule{r}); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(dir)
	if e != nil || !reflect.DeepEqual(reopened.doc.Rules[0].Style.Arc, a) {
		t.Fatal("arch settings lost", e)
	}
	for _, bad := range []func(*ArcStyle){func(a *ArcStyle) { a.Limit = 21 }, func(a *ArcStyle) { a.Thickness = 0 }, func(a *ArcStyle) { a.Sweep = 361 }, func(a *ArcStyle) { a.Fill = "script" }, func(a *ArcStyle) { a.Pair = "bad" }, func(a *ArcStyle) { a.Aura = -1 }} {
		copy := *a
		bad(&copy)
		r.Style.Arc = &copy
		if Validate([]Rule{r}) == nil {
			t.Fatal("invalid arch accepted", copy)
		}
	}
}

func TestArchWatchPublicationAndDeduplication(t *testing.T) {
	r := archRule()
	w := Watches([]Rule{r})
	if !reflect.DeepEqual(w, []Watch{{123, "buff", "player"}}) {
		t.Fatal(w)
	}
	r.Roots[0].Children = append(r.Roots[0].Children, Node{ID: "if", Type: "if", Props: map[string]any{"condition": "buff", "aura": "123", "unit": "player", "check": "active"}})
	if len(Watches([]Rule{r})) != 1 {
		t.Fatal("duplicate watch")
	}
	r.Roots[0].Children = r.Roots[0].Children[:1]
	r.Roots[0].Children[0].Props["display"] = "text"
	if len(Watches([]Rule{r})) != 0 {
		t.Fatal("unused arch watched")
	}
	r.Roots[0].Children[0].Props["display"] = "arc"
	r.Enabled = false
	if len(Watches([]Rule{r})) != 0 {
		t.Fatal("disabled arch watched")
	}
}
