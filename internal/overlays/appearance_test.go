package overlays

import (
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestAppearancePriorityUnknownAndOriginalRestoration(t *testing.T) {
	base := engine.OverlayDisplay{Color: "#F4C879", Text: "Base", Font: 14, Size: 90, Opacity: 100}
	ms := []AppearanceModifier{
		{Condition: group("all", map[string]any{"condition": "health", "operator": "lt", "value": 50.0}), Color: "red", Font: ptr(32.0), Text: ptr("Low {percent}"), Glow: ptr(true), Opacity: ptr(50.0)},
		{Condition: group("all", map[string]any{"condition": "health", "operator": "lt", "value": 20.0}), Color: "#00ff00", Font: ptr(48.0), Glow: ptr(false), Visible: ptr(false)},
	}
	for _, tc := range []struct {
		hp            *float64
		color         string
		font          float64
		visible, glow bool
	}{{ptr(40.0), colors["red"], 32, true, true}, {ptr(10.0), "#00FF00", 48, false, false}, {nil, base.Color, 14, true, false}, {ptr(80.0), base.Color, 14, true, false}} {
		v := base
		if visible := applyAppearance(&v, ms, &lookup.OverlayInput{Health: tc.hp}); visible != tc.visible || v.Color != tc.color || v.Font != tc.font || v.Glow != tc.glow {
			t.Fatalf("bad appearance %+v, visible=%v", v, visible)
		}
	}
	if base.Text != "Base" || base.Opacity != 100 {
		t.Fatal("mutated original")
	}
}
func TestAppearanceRoundtripWatchesAndValidation(t *testing.T) {
	r := rule()
	r.Style.Color = "#abcdef"
	r.Style.OnShow = "chime"
	r.Style.OnHide = "soft"
	r.Style.Modifiers = []AppearanceModifier{{Condition: group("any", map[string]any{"condition": "buff", "aura": "456", "unit": "focus", "check": "active"}, map[string]any{"condition": "cooldown", "source": "spell", "id": "363916", "check": "ready"}), Color: "#123456", Visible: ptr(false), Opacity: ptr(0.0), Text: ptr("")}}
	raw, _ := json.Marshal(r)
	var copy Rule
	json.Unmarshal(raw, &copy)
	if err := Validate([]Rule{copy}); err != nil {
		t.Fatal(err)
	}
	if len(Watches([]Rule{copy})) != 2 || len(CooldownWatches([]Rule{copy})) != 1 {
		t.Fatal("modifier watches missing")
	}
	for _, bad := range []func(*Rule){func(r *Rule) { r.Style.Modifiers[0].Color = "url(fake)" }, func(r *Rule) { r.Style.Modifiers[0].Font = ptr(100.0) }, func(r *Rule) { r.Style.OnHide = "bad" }, func(r *Rule) {
		r.Style.Modifiers[0].Condition = group("not", map[string]any{"condition": "health", "value": 20.0}, map[string]any{"condition": "health", "value": 30.0})
	}} {
		json.Unmarshal(raw, &copy)
		bad(&copy)
		if Validate([]Rule{copy}) == nil {
			t.Fatal("invalid change accepted")
		}
	}
}
func TestAppearanceLiveTextAndSounds(t *testing.T) {
	r := rule()
	show := r.Roots[0].Children[0].Children[0]
	show.Props["display"] = "text"
	show.Props["visibility"] = "conditions"
	show.Props["dataSource"] = "health"
	show.Props["text"] = "Base {percent}"
	r.Roots[0].Children[0].Children = []Node{show}
	r.Style.OnShow = "chime"
	r.Style.OnHide = "soft"
	r.Style.Modifiers = []AppearanceModifier{{Condition: group("all", map[string]any{"condition": "health", "operator": "lt", "value": 50.0}), Text: ptr("Low {percent}"), Sound: "alert", Color: "red"}}
	store, _ := Open(t.TempDir())
	if err := store.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	for i, hp := range []*float64{ptr(40.0), ptr(30.0), nil, ptr(80.0), ptr(40.0)} {
		at := now.Add(time.Duration(i) * 1100 * time.Millisecond)
		s := sample([]Rule{r}, at, i+1)
		s.Overlays.Health = hp
		store.Accept(s, at)
		d := store.Merge(engine.FrameDisplay{}, at)
		if len(d.Overlays) != 1 {
			t.Fatal("missing display")
		}
		want := []string{"Low 40", "Low 30", "Base —", "Base 80", "Low 40"}[i]
		if d.Overlays[0].Text != want {
			t.Fatalf("text %q want %q", d.Overlays[0].Text, want)
		}
		sounds := []int{2, 0, 0, 0, 1}[i]
		if len(d.OverlaySounds) != sounds {
			t.Fatalf("step %d sounds %+v", i, d.OverlaySounds)
		}
		again := store.Merge(engine.FrameDisplay{}, at)
		if len(again.OverlaySounds) != sounds {
			t.Fatal("repeated renderer produced another sound")
		}
	}
	at := now.Add(5500 * time.Millisecond)
	s := sample([]Rule{r}, at, 6)
	s.Overlays.Watches[0].Active = ptr(false)
	store.Accept(s, at)
	d := store.Merge(engine.FrameDisplay{}, at)
	if len(d.Overlays) != 0 || len(d.OverlaySounds) != 1 || d.OverlaySounds[0].Name != "soft" {
		t.Fatalf("hide action missing: %+v", d.OverlaySounds)
	}
}
func TestLifecycleTimerExpiryAndMultipleDisplays(t *testing.T) {
	rt := NewRuntime()
	r := rule()
	r.Style.OnShow = "chime"
	r.Style.OnHide = "soft"
	now := time.Unix(2000000000, 0)
	ds := []engine.OverlayDisplay{{Rule: r.ID}, {Rule: r.ID}}
	rt.lifecycleSounds([]Rule{r}, ds, now)
	rt.lifecycleSounds([]Rule{r}, ds[:1], now)
	if len(rt.sounds) != 1 {
		t.Fatal("one sound per SHOW instead of rule")
	}
	rt.lifecycleSounds([]Rule{r}, nil, now)
	rt.lifecycleSounds([]Rule{r}, nil, now)
	if len(rt.sounds) != 2 || rt.sounds[1].Name != "soft" {
		t.Fatal("hide sound did not fire once")
	}
	rt.Reset()
	rt.lifecycleSounds([]Rule{r}, nil, now)
	if len(rt.sounds) != 0 {
		t.Fatal("reset played stale hide")
	}
}

func TestLifecycleHideOnActualTimerExpiry(t *testing.T) {
	r := rule()
	r.Style.OnHide = "soft"
	r.Roots[0].Children[0].Children = r.Roots[0].Children[0].Children[:1]
	store, _ := Open(t.TempDir())
	if err := store.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	for i := 0; i <= 5; i++ {
		at := now.Add(time.Duration(i) * time.Second)
		store.Accept(sample([]Rule{r}, at, i+1), at)
		d := store.Merge(engine.FrameDisplay{}, at)
		if i < 5 && len(d.Overlays) != 1 {
			t.Fatal("timer ended early")
		}
		if i == 5 && (len(d.Overlays) != 0 || len(d.OverlaySounds) != 1 || d.OverlaySounds[0].Name != "soft") {
			t.Fatalf("expiry output: %+v %+v", d.Overlays, d.OverlaySounds)
		}
	}
}
