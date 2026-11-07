package overlays

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func rule() Rule {
	show := Node{ID: "show", Type: "show", Props: map[string]any{"display": "ring", "text": "Go", "duration": float64(5)}}
	sound := Node{ID: "sound", Type: "sound", Props: map[string]any{"sound": "chime"}}
	cond := Node{ID: "buff", Type: "if", Props: map[string]any{"condition": "buff", "aura": "123", "unit": "player", "check": "stacks", "operator": "gte", "value": float64(3)}, Children: []Node{show, sound}}
	when := Node{ID: "when", Type: "when", Props: map[string]any{"event": "aura"}, Children: []Node{cond}}
	return Rule{ID: "one", Name: "Stacks", Enabled: true, Style: Style{Size: 90, Font: 24, Opacity: 100, Color: "gold", X: 50, Y: 50, Transparent: true}, Roots: []Node{when}}
}
func ptr[T any](v T) *T { return &v }

func TestCaptureSetupFailureReportsReasonWithoutMatchingAbsentBuff(t *testing.T) {
	for _, check := range []string{"missing", "stacks"} {
		t.Run(check, func(t *testing.T) {
			aura := rule()
			aura.Roots[0].Children[0].Props["check"] = check
			resource := persistentRule()
			resource.ID = "resource"
			resource.Roots[0].Children[0].Children = resource.Roots[0].Children[0].Children[:1]
			rules := []Rule{aura, resource}
			rt := NewRuntime()
			now := time.Unix(2000000000, 0)
			makeSample := func(seq int) lookup.BridgeSample {
				s := sample(rules, now, seq)
				s.Overlays.Native = true
				s.Overlays.ResourcesVersion = 1
				s.Overlays.Resources = map[string]lookup.OverlayResource{"19": {Current: 10, Max: 100}}
				s.Overlays.PrimaryPower = 19
				s.Overlays.Watches[0].Active = ptr(check != "missing")
				return s
			}
			s := makeSample(1)
			s.Overlays.CaptureError = "Native binding failed"
			rt.Accept(rules, s, now)
			if rt.problem != "Aura capture: Native binding failed" || len(rt.displays) != 1 || len(rt.sounds) != 0 {
				t.Fatalf("failed capture matched an aura or suppressed resources: %s %+v", rt.problem, rt.displays)
			}
			rt.Accept(rules, makeSample(2), now.Add(200*time.Millisecond))
			if rt.problem != "" || len(rt.displays) != 2 || len(rt.sounds) != 1 {
				t.Fatalf("healthy capture did not recover: %s %+v", rt.problem, rt.displays)
			}
		})
	}
}
func sample(rules []Rule, now time.Time, seq int) lookup.BridgeSample {
	return lookup.BridgeSample{Window: 123, WindowPID: 42, Overlays: &lookup.OverlayInput{Epoch: "first", Seq: seq, At: now.Unix(), Width: 1920, Height: 1080, Hash: WatchHash(Watches(rules)), Character: "Lamda", Realm: "Gurubashi", Spec: 1468, Class: "EVOKER", Race: "Dracthyr", Faction: "Horde", Location: "dungeon", Alive: true, Health: ptr(90.0), Watches: []lookup.OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true, Active: ptr(true), Stacks: ptr(3.0), Remaining: ptr(4.0)}}}}
}
func TestRuntimeThresholdEdgesUnknownAndExpiry(t *testing.T) {
	r := rule()
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	rules := []Rule{r}
	rt.Accept(rules, sample(rules, now, 1), now)
	if len(rt.displays) != 1 || len(rt.sounds) != 1 {
		t.Fatal("matching aura did not render/play")
	}
	rt.Accept(rules, sample(rules, now, 2), now.Add(200*time.Millisecond))
	if len(rt.sounds) != 1 {
		t.Fatal("sound repeats every native tick")
	}
	s := sample(rules, now, 3)
	s.Overlays.Watches[0].Active = nil
	rt.Accept(rules, s, now.Add(400*time.Millisecond))
	if len(rt.latches) != 0 {
		t.Fatal("unknown did not clear conditions")
	}
	s = sample(rules, now, 4)
	rt.Accept(rules, s, now.Add(600*time.Millisecond))
	if len(rt.sounds) != 1 || len(rt.displays) != 1 {
		t.Fatal("unknown recovery replayed sound or failed to restore display")
	}
	rt.advance(now.Add(7 * time.Second))
	if len(rt.displays) != 0 || len(rt.sounds) != 0 {
		t.Fatal("expired output retained")
	}
	missing := r.Roots[0].Children[0]
	missing.Props["check"] = "missing"
	if condition(missing, &lookup.OverlayInput{}, "player") {
		t.Fatal("unknown matched missing")
	}
}
func TestLoadRestrictionsAndFreshness(t *testing.T) {
	r := rule()
	r.Load = Load{Character: "lamda", Spec: 1468, Race: "dracthyr", Faction: "Horde", Alive: "yes"}
	rules := []Rule{r}
	now := time.Unix(2000000000, 0)
	s := sample(rules, now, 1)
	if !loaded(r.Load, s.Overlays) {
		t.Fatal("same character blocked")
	}
	s.Overlays.Spec = 1467
	if loaded(r.Load, s.Overlays) {
		t.Fatal("wrong spec loaded")
	}
	store, _ := Open(t.TempDir())
	store.doc.Rules = rules
	s.Overlays.Spec = 1468
	store.Accept(s, now)
	d := store.Merge(engine.FrameDisplay{}, now)
	if !d.Enabled || len(d.Overlays) != 1 || d.WindowPID != 42 {
		t.Fatal(d)
	}
	store.Accept(s, now.Add(time.Second))
	d = store.Merge(engine.FrameDisplay{}, now.Add(1300*time.Millisecond))
	if d.Enabled {
		t.Fatal("frozen strip renewed lease")
	}
}

func TestDeletingFinalRuleClearsRuntimeAndStaysEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := []Rule{rule()}
	if err := store.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	store.Accept(sample(rules, now, 1), now)
	if d := store.Merge(engine.FrameDisplay{}, now); len(d.Overlays) != 1 || len(d.OverlaySounds) != 1 {
		t.Fatal("fixture did not trigger")
	}
	if err := store.Save(1, []Rule{}); err != nil {
		t.Fatal(err)
	}
	if d := store.Merge(engine.FrameDisplay{}, now); len(d.Overlays) != 0 || len(d.OverlaySounds) != 0 {
		t.Fatal("deleted rule still drawing or playing audio", d)
	}
	reopened, err := Open(dir)
	if err != nil || len(reopened.doc.Rules) != 0 || reopened.doc.Revision != 2 {
		t.Fatal("deleted final rule came back", err)
	}
	reopened.Accept(sample(rules, now, 2), now)
	if d := reopened.Merge(engine.FrameDisplay{}, now); len(d.Overlays) != 0 || len(d.OverlaySounds) != 0 {
		t.Fatal("new samples revived a deleted rule", d)
	}
}

func TestCaptureReservationFollowsAcceptedOverlayShape(t *testing.T) {
	now := time.Unix(2000000000, 0)
	for _, tc := range []struct {
		width, x, y, offset int
		want                engine.CaptureArea
	}{
		{1152, 0, 1064, 32, engine.CaptureArea{X: 0, Y: 1064, Width: 1152, Height: 16}},
		{64, 1500, 700, 32, engine.CaptureArea{X: 1500, Y: 700, Width: 64, Height: 288}},
		{0, 0, 100, 32, engine.CaptureArea{X: 0, Y: 100, Width: 256, Height: 72}},
		{0, 0, 1048, -40, engine.CaptureArea{X: 0, Y: 1008, Width: 256, Height: 72}},
	} {
		store, _ := Open(t.TempDir())
		store.doc.Rules = []Rule{rule()}
		s := sample(store.doc.Rules, now, 1)
		s.Position = lookup.BridgePosition{X: tc.x, Y: tc.y}
		s.Overlays.StripWidth, s.Overlays.Offset = tc.width, tc.offset
		store.Accept(s, now)
		d := store.Merge(engine.FrameDisplay{}, now)
		if len(d.CaptureAreas) != 1 || d.CaptureAreas[0] != tc.want {
			t.Fatal("accepted shape reserved the wrong screen area", tc, d.CaptureAreas)
		}
	}
}
func TestNestedWhenWaitAndLoadCancellation(t *testing.T) {
	r := rule()
	r.Roots = []Node{{ID: "enter", Type: "when", Props: map[string]any{"event": "enter"}, Children: []Node{{ID: "wait", Type: "wait", Props: map[string]any{"seconds": float64(1)}}, r.Roots[0]}}}
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	rt.Accept(rules, sample(rules, now, 1), now)
	if len(rt.displays) != 0 {
		t.Fatal("nested WHEN used same event / skipped WAIT")
	}
	rt.Accept(rules, sample(rules, now, 2), now.Add(500*time.Millisecond))
	if len(rt.displays) != 0 {
		t.Fatal("nested event before WAIT")
	}
	rt.Accept(rules, sample(rules, now.Add(time.Second), 3), now.Add(time.Second))
	if len(rt.displays) != 1 {
		t.Fatal("nested aura never fired")
	}
	rules[0].Enabled = false
	rt.Accept(rules, sample(rules, now.Add(time.Second), 4), now.Add(1100*time.Millisecond))
	if len(rt.displays) != 0 || len(rt.queue) != 0 {
		t.Fatal("disabled rule retained output")
	}
}
func TestPersistenceConflictPublishAndValidation(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "OverlaysData.lua")
	os.WriteFile(path, []byte(Marker+"\nLamdaOverlaysData={}\n"), 0600)
	s.SetGameFile(path)
	if e = s.Save(0, []Rule{rule()}); e != nil {
		t.Fatal(e)
	}
	if e = s.Save(0, []Rule{}); e != ErrConflict {
		t.Fatal("stale browser overwrote rules", e)
	}
	reopened, e := Open(dir)
	if e != nil || reopened.doc.Revision != 1 || len(reopened.doc.Rules) != 1 {
		t.Fatal("restart lost rules", e)
	}
	b, _ := os.ReadFile(path)
	if len(b) < 100 {
		t.Fatal("watch publisher empty")
	}
	bad := rule()
	bad.Roots[0].Children[0].Props["aura"] = "not-a-spell"
	if Validate([]Rule{bad}) == nil {
		t.Fatal("unwatchable aura accepted")
	}
	bad = rule()
	bad.Roots[0].Children[0].Children[0].Props["display"] = "script"
	if Validate([]Rule{bad}) == nil {
		t.Fatal("arbitrary action accepted")
	}
}

func TestEmptyWhenSurvivesJSON(t *testing.T) {
	n := Node{ID: "empty", Type: "when", Props: map[string]any{"event": "state"}}
	b, e := n.MarshalJSON()
	if e != nil || !strings.Contains(string(b), `"children":[]`) {
		t.Fatal(string(b), e)
	}
}
func TestNestedWhenWaitsForNextCapture(t *testing.T) {
	r := rule()
	r.Roots = []Node{{ID: "enter", Type: "when", Props: map[string]any{"event": "enter"}, Children: r.Roots}}
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	rt.Accept(rules, sample(rules, now, 1), now)
	if len(rt.displays) != 0 {
		t.Fatal("same capture satisfied two events")
	}
	rt.Accept(rules, sample(rules, now, 2), now.Add(200*time.Millisecond))
	if len(rt.displays) != 1 {
		t.Fatal("later aura was lost")
	}
}

func TestFocusGapDoesNotInventLogin(t *testing.T) {
	r := rule()
	r.Roots[0].Props["event"] = "login"
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	rt.Accept(rules, sample(rules, now, 1), now)
	if len(rt.displays) != 1 {
		t.Fatal("login did not run")
	}
	rt.Reset()
	rt.Accept(rules, sample(rules, now.Add(3*time.Second), 2), now.Add(3*time.Second))
	if len(rt.displays) != 0 {
		t.Fatal("focus return invented a login")
	}
}
func TestWatchMappingMustMatchCapturedMetadata(t *testing.T) {
	r := rule()
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	s := sample(rules, now, 1)
	s.Overlays.Watches[0].Spell = 456
	rt.Accept(rules, s, now)
	if len(rt.displays) != 0 || len(rt.last.Watches) != 0 {
		t.Fatal("accepted mismatched aura mapping")
	}
	if len(s.Overlays.Watches) != 1 {
		t.Fatal("runtime validation erased the shared capture sample")
	}
}

func resourceRule(operator, measure string, threshold float64) Rule {
	r := rule()
	r.Name = "Resource"
	r.Roots[0].Props["event"] = "resource"
	r.Roots[0].Children[0].Props = map[string]any{"condition": "resource", "resource": "19", "operator": operator, "measure": measure, "value": threshold}
	return r
}
func TestResourceComparisons(t *testing.T) {
	p := &lookup.OverlayInput{PrimaryPower: 19, Resources: map[string]lookup.OverlayResource{"19": {Current: 3, Max: 6}, "0": {Current: 15000, Max: 100000}}}
	for _, tc := range []struct {
		op, measure string
		n           float64
		match       bool
	}{{"lt", "amount", 4, true}, {"lt", "amount", 3, false}, {"lte", "amount", 3, true}, {"gt", "amount", 2, true}, {"gte", "amount", 3, true}, {"eq", "amount", 3, true}, {"ne", "amount", 3, false}, {"ne", "amount", 4, true}, {"eq", "percent", 50, true}, {"gt", "percent", 50, false}, {"cap", "amount", 0, false}} {
		t.Run(tc.op+tc.measure+fmt.Sprint(tc.n), func(t *testing.T) {
			r := resourceRule(tc.op, tc.measure, tc.n)
			if e := Validate([]Rule{r}); e != nil {
				t.Fatal(e)
			}
			if condition(r.Roots[0].Children[0], p, "player") != tc.match {
				t.Fatal("wrong resource comparison")
			}
		})
	}
	r := resourceRule("cap", "amount", 0)
	p.Resources["19"] = lookup.OverlayResource{Current: 6, Max: 6}
	n := r.Roots[0].Children[0]
	if !condition(n, p, "player") {
		t.Fatal("cap")
	}
	n.Props["resource"] = "primary"
	if !condition(n, p, "player") {
		t.Fatal("primary")
	}
	p.PrimaryPower = 0
	if condition(n, p, "player") {
		t.Fatal("primary did not change")
	}
	for _, op := range []string{"lt", "lte", "eq", "ne", "gt", "gte", "cap"} {
		n.Props["operator"] = op
		for _, unknown := range []*lookup.OverlayInput{{}, {PrimaryPower: 19, Resources: map[string]lookup.OverlayResource{"19": {Current: 0, Max: 0}}}} {
			if condition(n, unknown, "player") {
				t.Fatal("unknown matched", op)
			}
		}
	}
	for _, bad := range []Rule{resourceRule("script", "amount", 1), resourceRule("lt", "amount", -1), resourceRule("lt", "percent", 101)} {
		if Validate([]Rule{bad}) == nil {
			t.Fatal("invalid resource accepted")
		}
	}
}
func TestResourceAlertsRearmAfterRecoveryAndCapacityChanges(t *testing.T) {
	for _, op := range []string{"lt", "cap"} {
		t.Run(op, func(t *testing.T) {
			r := resourceRule(op, "percent", 20)
			rules := []Rule{r}
			rt := NewRuntime()
			now := time.Unix(2000000000, 0)
			feed := func(seq int, current, max float64) {
				s := sample(rules, now, seq)
				s.Overlays.Resources = map[string]lookup.OverlayResource{"19": {Current: current, Max: max}}
				rt.Accept(rules, s, now.Add(time.Duration(seq)*100*time.Millisecond))
			}
			if op == "lt" {
				feed(1, 50, 100)
				feed(2, 20, 100)
				if len(rt.sounds) != 0 {
					t.Fatal("threshold is strictly below")
				}
				feed(3, 19, 100)
				feed(4, 18, 100)
				feed(5, 18, 100)
			} else {
				feed(1, 5, 6)
				feed(2, 6, 6)
				feed(3, 6, 6)
			}
			if len(rt.sounds) != 1 {
				t.Fatal("onset must play once", len(rt.sounds))
			}
			if op == "lt" {
				feed(6, 25, 100)
				feed(7, 15, 100)
			} else {
				feed(6, 6, 7)
				feed(7, 7, 7)
			}
			if len(rt.sounds) != 2 {
				t.Fatal("recovery/capacity change did not rearm", len(rt.sounds))
			}
		})
	}
}

func TestLegacyHealthComparisonAndNewOperators(t *testing.T) {
	n := Node{Type: "if", Props: map[string]any{"condition": "health", "value": 50.0}}
	p := &lookup.OverlayInput{Health: ptr(40.0)}
	if !condition(n, p, "player") {
		t.Fatal("legacy below changed")
	}
	p.Health = ptr(50.0)
	if condition(n, p, "player") {
		t.Fatal("legacy boundary changed")
	}
	for op, want := range map[string]bool{"eq": true, "ne": false, "lte": true, "gte": true, "gt": false, "lt": false} {
		n.Props["operator"] = op
		if condition(n, p, "player") != want {
			t.Fatal(op)
		}
	}
	n.Props["operator"] = "ne"
	p.Health = nil
	if condition(n, p, "player") {
		t.Fatal("unknown matched not equal")
	}
}

func persistentRule() Rule {
	r := resourceRule("lt", "percent", 20)
	r.Roots[0].Children[0].Children[0].Props["visibility"] = "conditions"
	return r
}
func resourceSample(rules []Rule, now time.Time, seq int, current float64) lookup.BridgeSample {
	s := sample(rules, now, seq)
	s.Overlays.Watches = nil
	s.Overlays.Native = true
	s.Overlays.ResourcesVersion = 1
	s.Overlays.Resources = map[string]lookup.OverlayResource{"19": {Current: current, Max: 100}}
	return s
}
func TestPersistentDisplayLivesUntilAllConditionsStopMatching(t *testing.T) {
	r := persistentRule()
	root := &r.Roots[0]
	outer := &root.Children[0]
	outer.Children = []Node{{ID: "healthguard", Type: "if", Props: map[string]any{"condition": "health", "value": 95.0}, Children: outer.Children}}
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	for seq := 1; seq <= 120; seq++ {
		at := now.Add(time.Duration(seq) * 500 * time.Millisecond)
		rt.Accept(rules, resourceSample(rules, at, seq, 15), at)
		if len(rt.displays) != 1 || len(rt.held) != 1 {
			t.Fatal("persistent display expired", seq)
		}
		for _, v := range rt.displays {
			if !v.Persistent || v.Ends <= float64(at.UnixMilli())/1000 {
				t.Fatal("invalid lease")
			}
		}
	}
	if rt.serial != 2 {
		t.Fatal("show/sound repeats on held condition", rt.serial)
	}
	at := now.Add(61 * time.Second)
	s := resourceSample(rules, at, 121, 15)
	s.Overlays.Health = ptr(100.0)
	rt.Accept(rules, s, at)
	if len(rt.displays) != 0 || len(rt.held) != 0 {
		t.Fatal("health guard ignored without another resource event")
	}
	at = at.Add(200 * time.Millisecond)
	rt.Accept(rules, resourceSample(rules, at, 122, 14), at)
	if len(rt.displays) != 1 || rt.serial != 4 {
		t.Fatal("condition did not rearm")
	}
	at = at.Add(200 * time.Millisecond)
	s = resourceSample(rules, at, 123, 14)
	s.Overlays.Resources = nil
	rt.Accept(rules, s, at)
	if len(rt.displays) != 0 {
		t.Fatal("unknown data retained display")
	}
	at = at.Add(200 * time.Millisecond)
	rt.Accept(rules, resourceSample(rules, at, 124, 14), at)
	rules[0].Load.Combat = "yes"
	rt.Accept(rules, resourceSample(rules, at, 125, 14), at.Add(100*time.Millisecond))
	if len(rt.displays) != 0 {
		t.Fatal("load failure retained display")
	}
}
func TestPersistentWaitIsCancelledEvenWithoutAnotherStartingEvent(t *testing.T) {
	r := persistentRule()
	r.Roots[0].Props["event"] = "login"
	c := &r.Roots[0].Children[0]
	c.Children = append([]Node{{ID: "wait-before-show", Type: "wait", Props: map[string]any{"seconds": 2.0}}}, c.Children...)
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	rt.Accept(rules, resourceSample(rules, now, 1, 15), now)
	if len(rt.queue) != 2 {
		t.Fatal("actions not delayed")
	}
	at := now.Add(500 * time.Millisecond)
	rt.Accept(rules, resourceSample(rules, at, 2, 50), at)
	rt.advance(now.Add(3 * time.Second))
	if len(rt.displays) != 0 || len(rt.sounds) != 0 || len(rt.queue) != 0 {
		t.Fatal("delayed action ignored lost guard")
	}
}
func TestPersistentHideStalenessAndTimedCompatibility(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		r := persistentRule()
		if !persistent {
			delete(r.Roots[0].Children[0].Children[0].Props, "visibility")
		}
		rules := []Rule{r}
		rt := NewRuntime()
		now := time.Unix(2000000000, 0)
		rt.Accept(rules, resourceSample(rules, now, 1, 15), now)
		at := now.Add(time.Second)
		rt.Accept(rules, resourceSample(rules, at, 2, 50), at)
		if (len(rt.displays) == 0) != persistent {
			t.Fatal("timed/conditional lifetimes mixed", persistent)
		}
	}
	r := persistentRule()
	c := &r.Roots[0].Children[0]
	c.Children = append(c.Children, Node{ID: "wait-hide", Type: "wait", Props: map[string]any{"seconds": 1.0}}, Node{ID: "hide", Type: "hide", Props: map[string]any{}})
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	for seq := 1; seq <= 4; seq++ {
		at := now.Add(time.Duration(seq) * 500 * time.Millisecond)
		rt.Accept(rules, resourceSample(rules, at, seq, 15), at)
	}
	if len(rt.displays) != 0 || len(rt.held) != 0 {
		t.Fatal("HIDE lost to persistent refresh")
	}
	store, _ := Open(t.TempDir())
	store.doc.Rules = []Rule{persistentRule()}
	store.Accept(resourceSample(store.doc.Rules, now, 1, 15), now)
	if len(store.Merge(engine.FrameDisplay{}, now).Overlays) != 1 {
		t.Fatal("held display absent")
	}
	if len(store.Merge(engine.FrameDisplay{}, now.Add(1300*time.Millisecond)).Overlays) != 0 {
		t.Fatal("frozen stream retained persistent display")
	}
}
func TestPersistentSaveAppliesWithUnchangedResource(t *testing.T) {
	store, _ := Open(t.TempDir())
	r := resourceRule("lt", "percent", 20)
	if e := store.Save(0, []Rule{r}); e != nil {
		t.Fatal(e)
	}
	now := time.Unix(2000000000, 0)
	store.Accept(resourceSample(store.doc.Rules, now, 1, 15), now)
	r.Roots[0].Children[0].Children[0].Props["visibility"] = "conditions"
	if e := store.Save(1, []Rule{r}); e != nil {
		t.Fatal(e)
	}
	at := now.Add(200 * time.Millisecond)
	store.Accept(resourceSample(store.doc.Rules, at, 2, 15), at)
	d := store.Merge(engine.FrameDisplay{}, at)
	if len(d.Overlays) != 1 || !d.Overlays[0].Persistent {
		t.Fatal("save needed a new resource change")
	}
	r.Roots[0].Children[0].Children[0].Props["visibility"] = "forever-script"
	if Validate([]Rule{r}) == nil {
		t.Fatal("invalid mode accepted")
	}
}
