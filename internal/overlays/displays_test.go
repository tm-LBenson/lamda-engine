package overlays

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestLiveTextUpdatesWithoutReplayingActions(t *testing.T) {
	r := rule()
	n := &r.Roots[0].Children[0].Children[0]
	n.Props = map[string]any{"display": "text", "text": "{name}: {stacks} / {remaining}s / {character}", "visibility": "conditions"}
	rules := []Rule{r}
	rt := NewRuntime()
	store := &Store{runtime: rt, doc: Document{Rules: rules}}
	now := time.Unix(2000000000, 0)
	s := sample(rules, now, 1)
	s.Overlays.Watches[0].Stacks = ptr(4.0)
	rt.Accept(rules, s, now)
	first := store.Merge(engine.FrameDisplay{}, now)
	if len(first.Overlays) != 1 || first.Overlays[0].Text != "Spell 123: 4 / 4s / Lamda" || len(rt.sounds) != 1 {
		t.Fatalf("first: %+v sounds=%d", first.Overlays, len(rt.sounds))
	}
	s = sample(rules, now, 2)
	s.Overlays.Watches[0].Remaining = ptr(2.7)
	rt.Accept(rules, s, now.Add(200*time.Millisecond))
	next := store.Merge(engine.FrameDisplay{}, now.Add(200*time.Millisecond))
	if len(next.Overlays) != 1 || next.Overlays[0].Text != "Spell 123: 3 / 2.7s / Lamda" || len(rt.sounds) != 1 {
		t.Fatalf("live: %+v sounds=%d", next.Overlays, len(rt.sounds))
	}
	// Text measurement failure must not invent zero or disturb other values.
	s = sample(rules, now, 3)
	s.Overlays.Watches[0].Remaining = nil
	rt.Accept(rules, s, now.Add(400*time.Millisecond))
	next = store.Merge(engine.FrameDisplay{}, now.Add(400*time.Millisecond))
	if next.Overlays[0].Text != "Spell 123: 3 / —s / Lamda" {
		t.Fatal(next.Overlays[0].Text)
	}
}
func TestDisplaySourceNearestRelevantAndManualWatch(t *testing.T) {
	r := rule()
	parent := &r.Roots[0].Children[0]
	show := parent.Children[0]
	parent.Children = []Node{{ID: "role", Type: "if", Props: map[string]any{"condition": "role", "value": "damage"}, Children: []Node{show}}}
	_, source, ok := showSource(r.Roots, "show", displaySource{})
	if !ok || source.Kind != "buff" || source.ID != 123 {
		t.Fatal(source)
	}
	show.Props["dataSource"] = "debuff"
	show.Props["dataID"] = "456"
	show.Props["dataUnit"] = "focus"
	watches := Watches([]Rule{r})
	if len(watches) != 2 {
		t.Fatal(watches)
	}
	_, source, _ = showSource(r.Roots, "show", displaySource{})
	if source.Unit != "focus" || source.ID != 456 {
		t.Fatal(source)
	}
	show.Props["dataSource"] = "spell"
	show.Props["dataID"] = "363916"
	if len(Watches([]Rule{r})) != 1 || len(CooldownWatches([]Rule{r})) != 1 {
		t.Fatal("watch lists cross-contaminated")
	}
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	var copy Rule
	json.Unmarshal(raw, &copy)
	_, again, _ := showSource(copy.Roots, "show", displaySource{})
	if again != source { // source changed from debuff to spell above
		if again.Kind != "spell" || again.ID != 363916 {
			t.Fatal(again)
		}
	}
	show.Props["dataID"] = "0"
	if Validate([]Rule{r}) == nil {
		t.Fatal("invalid binding accepted")
	}
}
func TestLiveProgressUnknownZeroPermanentAndCharges(t *testing.T) {
	p := &lookup.OverlayInput{Resources: map[string]lookup.OverlayResource{"19": {Current: 3, Max: 6}}, PrimaryPower: 19}
	source := displaySource{Kind: "resource", Resource: "primary"}
	v := sourceValues(source, p)
	if v.Fraction == nil || *v.Fraction != .5 || v.Text["name"] != "Essence" || v.Text["percent"] != "50" {
		t.Fatal(v)
	}
	p.Resources["19"] = lookup.OverlayResource{Current: math.NaN(), Max: 6}
	if sourceValues(source, p).Fraction != nil {
		t.Fatal("NaN rendered")
	}
	p.Watches = []lookup.OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true, Active: ptr(true), Remaining: ptr(5.0), Duration: ptr(20.0)}}
	source = displaySource{Kind: "buff", ID: 123, Unit: "player"}
	v = sourceValues(source, p)
	if v.Fraction == nil || *v.Fraction != .25 {
		t.Fatal(v)
	}
	p.Watches[0].Duration = nil
	display := engine.OverlayDisplay{Kind: "bar", Text: "{remaining}"}
	show := Node{Props: map[string]any{"fill": "source", "text": "{remaining}"}}
	if resolveDisplay(&display, show, source, p) {
		t.Fatal("unknown total displayed a guessed fraction")
	}
	p.Watches[0].Permanent = true
	if !resolveDisplay(&display, show, source, p) || *display.Fraction != 1 || display.Text != "∞" {
		t.Fatal(display)
	}
	p.Watches[0].Active = ptr(false)
	if !resolveDisplay(&display, show, source, p) || *display.Fraction != 0 {
		t.Fatal("known absence did not empty progress")
	}
	p.Cooldowns = []lookup.OverlayCooldown{{Kind: "spell", ID: 363916, Available: true, Remaining: ptr(0.0), Duration: ptr(0.0), Charges: ptr(1.0), MaxCharges: ptr(2.0), Recharge: ptr(15.0), RechargeDuration: ptr(60.0)}}
	source = displaySource{Kind: "spell", ID: 363916}
	v = sourceValues(source, p)
	if *v.Fraction != 0 || *v.Charges != .5 || *v.Recharge != .25 || v.Text["name"] != "Obsidian Scales" {
		t.Fatal(v)
	}
	p.Cooldowns[0].Charges = ptr(2.0)
	p.Cooldowns[0].Recharge = nil
	p.Cooldowns[0].RechargeDuration = nil
	v = sourceValues(source, p)
	if v.Recharge == nil || *v.Recharge != 0 || v.Text["recharge"] != "0" {
		t.Fatal("full charges needs no recharge timer", v)
	}
	p.Cooldowns[0].Available = false
	show.Props["fill"] = "charges"
	if resolveDisplay(&display, show, source, p) {
		t.Fatal("unavailable charge display rendered")
	}
}
func TestDisplayIconsAndLiteralCompatibility(t *testing.T) {
	p := &lookup.OverlayInput{}
	source := displaySource{Kind: "buff", ID: 436336}
	for _, tc := range []struct {
		icon string
		want int
	}{{"", 0}, {"shield", 0}, {"auto", 436336}, {"363916", 363916}} {
		v := engine.OverlayDisplay{Kind: "icon", Text: "literal {notAToken}"}
		n := Node{Props: map[string]any{"icon": tc.icon, "text": "literal {notAToken}"}}
		if !resolveDisplay(&v, n, source, p) || v.Spell != tc.want || v.Text != "literal {notAToken}" {
			t.Fatal(v)
		}
	}
}
