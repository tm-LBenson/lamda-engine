package overlays

import (
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func group(mode string, tests ...map[string]any) map[string]any {
	values := make([]any, 0, len(tests))
	for _, p := range tests {
		values = append(values, p)
	}
	return map[string]any{"condition": "group", "mode": mode, "tests": values}
}
func TestPredicateTruthTablesAndUnknownNegation(t *testing.T) {
	health := map[string]any{"condition": "health", "operator": "lt", "value": 50.0}
	resource := map[string]any{"condition": "resource", "resource": "0", "operator": "lt", "measure": "percent", "value": 50.0}
	for _, a := range []truth{unknown, no, yes} {
		for _, b := range []truth{unknown, no, yes} {
			p := &lookup.OverlayInput{}
			if a != unknown {
				v := 75.0
				if a == yes {
					v = 25
				}
				p.Health = &v
			}
			if b != unknown {
				v := 75.0
				if b == yes {
					v = 25
				}
				p.Resources = map[string]lookup.OverlayResource{"0": {Current: v, Max: 100}}
			}
			all, any := unknown, unknown
			if a == no || b == no {
				all = no
			} else if a == yes && b == yes {
				all = yes
			}
			if a == yes || b == yes {
				any = yes
			} else if a == no && b == no {
				any = no
			}
			for _, tc := range []struct {
				mode string
				want truth
			}{{"all", all}, {"any", any}} {
				pred := group(tc.mode, health, resource)
				if got := predicate(pred, p, "player"); got != tc.want {
					t.Fatalf("%v %s %v: %v, want %v", a, tc.mode, b, got, tc.want)
				}
				want := unknown
				if tc.want != unknown {
					want = truthOf(tc.want == no)
				}
				if got := predicate(group("not", pred), p, "player"); got != want {
					t.Fatal("bad negation", got, want)
				}
			}
		}
	}
	for _, mode := range []string{"all", "any", "not"} {
		if predicate(group(mode), &lookup.OverlayInput{}, "player") != unknown {
			t.Fatal("empty group triggered")
		}
	}
}
func TestPredicateWatchesRoundtripLimitsAndSources(t *testing.T) {
	aura := map[string]any{"condition": "buff", "aura": "123", "unit": "focus", "check": "active"}
	cd := map[string]any{"condition": "cooldown", "source": "spell", "id": "363916", "check": "ready"}
	r := rule()
	r.Roots[0].Children[0].Props = group("all", group("not", aura), group("any", aura, cd))
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	if len(Watches([]Rule{r})) != 1 || len(CooldownWatches([]Rule{r})) != 1 {
		t.Fatal("group watches lost or duplicated")
	}
	raw, _ := json.Marshal(r)
	var copy Rule
	json.Unmarshal(raw, &copy)
	if err := Validate([]Rule{copy}); err != nil {
		t.Fatal(err)
	}
	if s := conditionSource(group("all", aura, aura)); s.ID != 123 || s.Unit != "focus" {
		t.Fatal(s)
	}
	if s := conditionSource(group("any", aura, cd)); s.Kind != "" {
		t.Fatal("ambiguous source guessed", s)
	}
	r.Roots[0].Children[0].Props = group("not", aura, cd)
	if Validate([]Rule{r}) == nil {
		t.Fatal("multiple NOT operands accepted")
	}
	deep := aura
	for range 5 {
		deep = group("not", deep)
	}
	r.Roots[0].Children[0].Props = deep
	if Validate([]Rule{r}) == nil {
		t.Fatal("unbounded group nesting")
	}
	many := []map[string]any{}
	for range 32 {
		many = append(many, aura)
	}
	r.Roots[0].Children[0].Props = group("all", many...)
	if Validate([]Rule{r}) == nil {
		t.Fatal("unbounded group size")
	}
}
func TestPredicatePersistentOutputClearsOnUnknownAndRearms(t *testing.T) {
	r := rule()
	r.Roots[0].Props["event"] = "state"
	r.Roots[0].Children[0].Props = group("not", map[string]any{"condition": "buff", "aura": "123", "unit": "player", "check": "active"})
	r.Roots[0].Children[0].Children[0].Props["visibility"] = "conditions"
	store, _ := Open(t.TempDir())
	if err := store.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	for i, v := range []*bool{ptr(false), nil, ptr(true), ptr(false)} {
		s := sample([]Rule{r}, now, i+1)
		s.Overlays.Watches[0].Active = v
		at := now.Add(time.Duration(i) * 200 * time.Millisecond)
		store.Accept(s, at)
		d := store.Merge(engine.FrameDisplay{}, at)
		want := 0
		if v != nil && !*v {
			want = 1
		}
		if len(d.Overlays) != want {
			t.Fatal("negative condition output", i, len(d.Overlays), want)
		}
	}
	// Availability loss cannot be turned into NOT true, even with a stale Active pointer.
	s := sample([]Rule{r}, now, 5)
	s.Overlays.Watches[0].Active = ptr(false)
	s.Overlays.Watches[0].Available = false
	store.Accept(s, now.Add(time.Second))
	if len(store.Merge(engine.FrameDisplay{}, now.Add(time.Second)).Overlays) != 0 {
		t.Fatal("unavailable aura satisfied NOT")
	}
}
func TestNotDoesNotMakeUnsupportedChargeOrPermanentTimeComparisonsTrue(t *testing.T) {
	p := &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{{ID: 363916, Kind: "spell", Available: true, MaxCharges: ptr(0.0), Charges: ptr(0.0)}}, Watches: []lookup.OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true, Active: ptr(true), Permanent: true}}}
	for _, q := range []map[string]any{
		{"condition": "cooldown", "source": "spell", "id": "363916", "check": "charges", "operator": "gte", "value": 1.0},
		{"condition": "buff", "aura": "123", "unit": "player", "check": "remaining", "operator": "gt", "value": 5.0},
	} {
		if got := predicate(group("not", q), p, "player"); got != unknown {
			t.Fatal("unsupported comparison satisfied negation", got)
		}
	}
}
