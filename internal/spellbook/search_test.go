package spellbook

import (
	"reflect"
	"strings"
	"testing"
)

func TestOfflineSearchAndAuraVariants(t *testing.T) {
	result, err := Search(Options{Query: "Essence Burst", Kind: "auras", Prefer: "EVOKER"})
	if err != nil || result.Build != "12.1.0.69933" || len(result.Spells) < 3 {
		t.Fatal(result, err)
	}
	for _, id := range []int{359618, 369299, 392268} {
		var found *Spell
		for i := range result.Spells {
			if result.Spells[i].ID == id {
				found = &result.Spells[i]
			}
		}
		if found == nil || found.Name != "Essence Burst" || !found.Aura || !reflect.DeepEqual(found.Classes, []string{"Evoker"}) {
			t.Fatal("missing distinct buff variant", id, found)
		}
	}
	if !strings.Contains(result.Spells[0].Description, "Disintegrate") {
		t.Fatal("buff tooltip should rank above driver", result.Spells[0])
	}
	numeric, err := Search(Options{Query: "369299", Kind: "auras", Class: "MAGE"})
	if err != nil || numeric.Total != 1 || !strings.Contains(numeric.Spells[0].Description, "Echo") {
		t.Fatal("ID lookup must resolve even with filters", numeric, err)
	}
	name, err := Search(Options{Query: "  ANTI-magic SHELL ", Kind: "all"})
	if err != nil || name.Spells[0].Name != "Anti-Magic Shell" {
		t.Fatal(name, err)
	}
}

func TestSearchFiltersPagesValidationAndCoverage(t *testing.T) {
	page, err := Search(Options{Query: "fire", Kind: "all"})
	if err != nil || page.Total <= 30 || len(page.Spells) != 30 {
		t.Fatal(page, err)
	}
	next, err := Search(Options{Query: "fire", Kind: "all", Offset: 30})
	if err != nil || len(next.Spells) == 0 || page.Total != next.Total {
		t.Fatal(next, err)
	}
	seen := map[int]bool{}
	for _, s := range page.Spells {
		seen[s.ID] = true
	}
	for _, s := range next.Spells {
		if seen[s.ID] {
			t.Fatal("duplicate page result")
		}
	}
	filtered, err := Search(Options{Query: "fire", Kind: "auras", Class: "MAGE"})
	if err != nil || filtered.Total == 0 {
		t.Fatal(filtered, err)
	}
	for _, s := range filtered.Spells {
		if !s.Aura || !strings.Contains(strings.Join(s.Classes, " "), "Mage") {
			t.Fatal("filter leaked", s)
		}
	}
	for _, q := range []string{"", "z", "unmatchable_spell_xyzzy", "9999999"} {
		r, e := Search(Options{Query: q})
		if e != nil || r.Total != 0 {
			t.Fatal(q, r, e)
		}
	}
	for _, o := range []Options{{Query: strings.Repeat("x", 121)}, {Kind: "bad"}, {Class: "bad"}, {Prefer: "bad"}, {Offset: -1}, {Offset: 50001}} {
		if _, e := Search(o); e == nil {
			t.Fatal("bad search accepted", o)
		}
	}
	if len(catalog.Spells) != 31636 {
		t.Fatal("index incomplete", len(catalog.Spells))
	}
	for _, s := range catalog.Spells {
		if s.ID <= 0 || s.Name == "" || strings.ContainsAny(s.Description, "$|") {
			t.Fatal("bad index entry", s.Spell)
		}
	}
	if !strings.Contains(Attribution(), "GNU GENERAL PUBLIC LICENSE") || !strings.Contains(Attribution(), "fa22f84d") {
		t.Fatal("attribution not bundled")
	}
}
