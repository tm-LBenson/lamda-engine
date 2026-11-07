package overlays

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestSavingOtherWatchesPreservesLoadedStackArches(t *testing.T) {
	for _, tc := range []struct {
		name, kind, unit string
		spell            int
		load             Load
	}{
		{"another class", "buff", "player", 59638, Load{Class: "MAGE", Spec: 64}},
		{"another spec", "buff", "player", 59638, Load{Spec: 1467}},
		{"earlier slot", "buff", "player", 12, Load{}},
		{"later slot", "buff", "player", 456, Load{}},
		{"same spell debuff", "debuff", "player", 123, Load{}},
		{"same spell other unit", "buff", "target", 123, Load{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			original := archRule()
			originalRules := []Rule{original}
			added := Rule{ID: "new", Name: "New watch", Enabled: true, Style: original.Style, Load: tc.load,
				Roots: []Node{{ID: "new_when", Type: "when", Props: map[string]any{"event": "state"}, Children: []Node{{
					ID: "new_if", Type: "if", Props: map[string]any{"condition": tc.kind, "aura": fmt.Sprint(tc.spell), "unit": tc.unit, "check": "missing"},
					Children: []Node{{ID: "new_show", Type: "show", Props: map[string]any{"display": "text", "text": "Missing", "visibility": "conditions", "duration": 6.0}}},
				}}}}}
			added.Style.Arc = nil
			allRules := []Rule{original, added}
			save := func(rules []Rule) {
				if err := store.Save(store.doc.Revision, rules); err != nil {
					t.Fatal(err)
				}
			}
			now := time.Unix(2000000000, 0)
			seq := 0
			capture := func(installed []Rule) lookup.BridgeSample {
				seq++
				s := sample(installed, now, seq)
				s.Overlays.Native = true
				s.Overlays.Watches = nil
				for _, w := range Watches(installed) {
					// Native pixel order follows the loaded addon list, even after
					// the editor publishes a different list for the next reload.
					a := lookup.OverlayAura{Spell: w.Spell, Kind: w.Kind, Unit: w.Unit, Available: true, Active: ptr(false)}
					if w.Key() == "player:buff:123" {
						a.Active, a.Stacks, a.Remaining = ptr(true), ptr(2.0), ptr(10.0)
					}
					s.Overlays.Watches = append(s.Overlays.Watches, a)
				}
				return s
			}
			check := func(want int) {
				d := store.Merge(engine.FrameDisplay{}, now)
				if len(d.Overlays) != want {
					t.Fatalf("got %d displays, want %d; status %s", len(d.Overlays), want, store.runtime.problem)
				}
				for _, display := range d.Overlays {
					if display.Rule == original.ID && display.Arc != nil && display.Arc.Count == 2 {
						return
					}
				}
				t.Fatal("existing two-stack arches stopped")
			}
			save(originalRules)
			store.Accept(capture(originalRules), now)
			check(1)
			save(allRules)
			s := capture(originalRules)
			store.Accept(s, now)
			check(1) // An untracked buff must not match the new "missing" condition.
			applies := loaded(added.Load, s.Overlays)
			if strings.Contains(store.runtime.problem, "Reload WoW") != applies {
				t.Fatalf("reload warning did not follow current character/spec: %s", store.runtime.problem)
			}
			// Disabling/re-enabling the new watch must also preserve the old one.
			allRules[1].Enabled = false
			save(allRules)
			store.Accept(capture(originalRules), now)
			check(1)
			if store.runtime.problem != "" {
				t.Fatal(store.runtime.problem)
			}
			allRules[1].Enabled = true
			save(allRules)
			store.Accept(capture(originalRules), now)
			check(1)
			// A real reload supplies the new sorted slot order and enables the
			// new rule without reinterpreting the original aura's pixels.
			s = capture(allRules)
			s.Overlays.Epoch = "after-reload"
			store.Accept(s, now)
			if applies {
				check(2)
			} else {
				check(1)
			}
			if store.runtime.problem != "" {
				t.Fatal(store.runtime.problem)
			}
			// Deletion leaves extra installed slots until reload; those are safe.
			save(originalRules)
			s = capture(allRules)
			s.Overlays.Epoch = "after-reload"
			store.Accept(s, now)
			check(1)
			if store.runtime.problem != "" {
				t.Fatal("removing a watch unnecessarily required reload", store.runtime.problem)
			}
		})
	}
}

func TestDuplicateCapturedWatchIdentityRemainsUnknown(t *testing.T) {
	rules := []Rule{archRule()}
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	rt := store.runtime
	now := time.Unix(2000000000, 0)
	s := sample(rules, now, 1)
	s.Overlays.Native = true
	s.Overlays.Watches = append(s.Overlays.Watches, s.Overlays.Watches[0])
	s.Overlays.Hash = WatchHash([]Watch{{123, "buff", "player"}, {123, "buff", "player"}})
	rt.Accept(rules, s, now)
	if len(rt.last.Watches) != 0 || len(store.Merge(engine.FrameDisplay{}, now).Overlays) != 0 || rt.problem == "" {
		t.Fatal("ambiguous duplicate identity was trusted")
	}
}
