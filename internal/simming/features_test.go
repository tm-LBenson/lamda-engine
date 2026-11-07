package simming

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExportExtrasAndTalentSets(t *testing.T) {
	p, err := ParseProfile(profileFixture + "omnium_talents=136814:1/136815:1\n# Saved Loadout: Other\n# talents=ABCDdef==\n# bonus_roll_items=3418:268459:4:1:249306:103\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Talents) != 1 || p.Talents[0].Label != "Other" {
		t.Fatalf("loadouts: %+v", p.Talents)
	}
	scenario := DefaultScenario()
	scenario.Talents = []string{"ABCxyz=="}
	input, err := p.Input(scenario, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input, "omnium_talents=136814:1/136815:1") || !strings.Contains(input, "profileset.talent1=talents=ABCxyz==") || strings.Contains(input, "bonus_roll_items") {
		t.Fatal(input)
	}
	for _, code := range []string{"abc\noutput=/tmp/x", "a\"", "a b"} {
		scenario.Talents = []string{code}
		if _, err = p.Input(scenario, nil); err == nil {
			t.Fatalf("accepted %q", code)
		}
	}
}
func TestGearSetsKeepSlotsAndVaultExclusive(t *testing.T) {
	p, err := ParseProfile(profileFixture + "# Vault second\n# head=,id=126\n### Gear from Bags\n# Ring\n# finger1=,id=127\n")
	if err != nil {
		t.Fatal(err)
	}
	s := DefaultScenario()
	s.CompareMode = "topgear"
	variants, err := p.Comparisons(s, []string{"item1", "item2", "item3", "item4"})
	if err != nil {
		t.Fatal(err)
	}
	joint := false
	for _, v := range variants {
		slots := map[string]bool{}
		vault := 0
		for _, c := range v.Items {
			if slots[c.Slot] {
				t.Fatal("repeated slot")
			}
			slots[c.Slot] = true
			if c.Source == "vault" {
				vault++
			}
		}
		if vault > 1 {
			t.Fatal("multiple Vault rewards")
		}
		joint = joint || len(v.Items) > 1
	}
	if !joint {
		t.Fatal("no joint sets")
	}
	input, err := p.Input(s, []string{"item1", "item4"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input, "profileset.set3+=finger1=") {
		t.Fatal("missing joint profileset", input)
	}
	if _, err = p.Input(s, []string{"item1", "item1"}); err == nil {
		t.Fatal("duplicate item accepted")
	}
}

func TestPlannedLoadoutCombinesTheExactSelectedSetTalentAndPlacement(t *testing.T) {
	p, err := ParseProfile(profileFixture + "### Gear from Bags\n# Bag helm\n# head=,id=126\n# Bag ring\n# finger1=,id=127\n")
	if err != nil {
		t.Fatal(err)
	}
	s := DefaultScenario()
	s.CompareMode = "loadout"
	s.Talents = []string{"LOADOUT=="}
	s.LoadoutSlots = map[string]string{"item4": "finger2"}
	s.LoadoutLevels = map[string]int{"item3": 300}
	sets, err := p.Comparisons(s, []string{"item1", "item3", "item4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].ID != "loadout" || sets[0].Talents != "LOADOUT==" || len(sets[0].Items) != 3 || sets[0].Items[1].Level != 300 || !strings.Contains(sets[0].Items[1].Value, "ilevel=300") || sets[0].Items[2].Slot != "finger2" {
		t.Fatalf("planned loadout was not kept exact: %+v", sets)
	}
	prepared, err := lootInput(p, s, sets)
	if err != nil || !strings.Contains(prepared, "profileset.loadout=main_hand=") || !strings.Contains(prepared, "profileset.loadout+=head=") || !strings.Contains(prepared, "profileset.loadout+=finger2=") {
		t.Fatalf("planned loadout was not prepared for the worker: %v\n%s", err, prepared)
	}
	input, err := p.Input(s, []string{"item1", "item3", "item4"})
	if err != nil || !strings.Contains(input, "profileset.loadout=main_hand=") || !strings.Contains(input, "profileset.loadout+=head=") || !strings.Contains(input, "profileset.loadout+=finger2=") || strings.Contains(input, "profileset.loadout+=finger1=") || !strings.Contains(input, "profileset.loadout+=talents=LOADOUT==") || strings.Contains(input, "profileset.item1=") {
		t.Fatalf("planned loadout input is wrong: %v\n%s", err, input)
	}
}
func TestHealerRoutingAndClientIsolation(t *testing.T) {
	p := LoadProviders(t.TempDir(), nil)
	s := fixture(time.Now())
	s.Character.Class = "EVOKER"
	s.Character.Spec = 1468
	s.Character.Role = "HEALER"
	s.Profile = strings.NewReplacer("mage=", "evoker=", "spec=frost", "spec=preservation").Replace(s.Profile)
	if c := p.Capability(s, time.Now()); c.ID != "qe" || c.Available {
		t.Fatal(c)
	}
	s.Client = "forever"
	if c := p.Capability(s, time.Now()); c.ID == "qe" || c.Available {
		t.Fatal(c)
	}
}
func TestReviewNotesPersistWithoutApprovingModels(t *testing.T) {
	dir := t.TempDir()
	p := &Provider{}
	w, err := NewWorkspace(context.Background(), dir, "retail", p)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := fixture(time.Now())
	if _, err = w.Receive(s); err != nil {
		t.Fatal(err)
	}
	if _, err = w.RecordReview(s.ID, "short"); err == nil {
		t.Fatal("empty evidence accepted")
	}
	note := "Import and item-level check passed; combat validation pending."
	if _, err = w.RecordReview(s.ID, note); err != nil {
		t.Fatal(err)
	}
	state := w.State()
	if len(state.Reviews) != 1 || state.Reviews[0].Notes != note || state.Snapshots[0].Capability.Validation != "unreviewed" {
		t.Fatalf("bad state: %+v", state.Reviews)
	}
}
func TestProviderAuditIsolation(t *testing.T) {
	a := Audit{Revision: "2026-09-23.1", CheckedAt: time.Now(), Sources: []SourceAudit{{Name: "SimulationCraft", Status: "changed"}, {Name: "Questionably Epic", Status: "unchanged"}}}
	if len(a.ProviderWarnings(time.Now(), "Questionably Epic")) != 0 {
		t.Fatal("DPS source drift marked healer results")
	}
	if len(a.ProviderWarnings(time.Now(), "SimulationCraft")) != 1 {
		t.Fatal("missing DPS drift warning")
	}
}
