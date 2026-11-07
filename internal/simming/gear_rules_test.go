package simming

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSimCNumericItemSerialization(t *testing.T) {
	p, err := ParseProfile(profileFixture)
	if err != nil {
		t.Fatal(err)
	}
	input, err := p.Input(DefaultScenario(), []string{"item1"})
	if err != nil || !strings.Contains(input, "main_hand=,id=123,") || !strings.Contains(input, "profileset.item1=main_hand=,id=124,") {
		t.Fatalf("numeric items lost their empty name field: %s %v", input, err)
	}
	scenario := DefaultScenario()
	scenario.Talents = []string{"ABC\noutput=/private/file"}
	if _, err := lootInput(p, scenario, []Comparison{{ID: "talent1", Talents: scenario.Talents[0]}}); err == nil {
		t.Fatal("unvalidated talent directive reached preparedinput")
	}
	if simcItemValue("named_item,id=123") != "named_item,id=123" || simcItemValue("") != "" {
		t.Fatal("named item or offhand removal changed")
	}
}

func TestCapturedGearRulesFilterLimitsAndTwoHandedSets(t *testing.T) {
	s := fixture(time.Now())
	s.Profile += "off_hand=,id=100\nhead=,id=101\nchest=,id=102\n"
	p, err := ParseProfile(s.Profile)
	if err != nil {
		t.Fatal(err)
	}
	s.Rules = []ItemRule{
		{ID: 123, BonusIDs: []int{2, 1}, InventoryType: "INVTYPE_WEAPONMAINHAND"},
		{ID: 100, InventoryType: "INVTYPE_HOLDABLE"},
		{ID: 101, InventoryType: "INVTYPE_HEAD", LimitCategory: 42, LimitMax: 2},
		{ID: 102, InventoryType: "INVTYPE_CHEST", LimitCategory: 42, LimitMax: 2},
		{ID: 200, InventoryType: "INVTYPE_2HWEAPON"},
		{ID: 201, InventoryType: "INVTYPE_FEET", LimitCategory: 42, LimitMax: 2},
	}
	input := []Comparison{
		{ID: "staff", Items: []Candidate{{Slot: "main_hand", Value: "id=200"}}},
		{ID: "three_embellishments", Items: []Candidate{{Slot: "feet", Value: "id=201"}}},
		{ID: "staff_offhand", Items: []Candidate{{Slot: "main_hand", Value: "id=200"}, {Slot: "off_hand", Value: "id=100"}}},
	}
	accepted, rejected, warnings, err := applyGearRules(s, p, input, true)
	if err != nil || len(accepted) != 1 || len(rejected) != 2 || len(warnings) != 0 || accepted[0].Items[1].Slot != "off_hand" || accepted[0].Items[1].Value != "" {
		t.Fatalf("rules failed: %+v %+v %+v %v", accepted, rejected, warnings, err)
	}
	p.Class, p.Spec = "evoker", "augmentation"
	s.Rules = append(s.Rules, ItemRule{ID: 202, InventoryType: "INVTYPE_WEAPON"})
	accepted, rejected, _, err = applyGearRules(s, p, []Comparison{{ID: "dual_wield", Items: []Candidate{{Slot: "off_hand", Value: "id=202"}}}}, true)
	if err != nil || len(accepted) != 0 || len(rejected) != 1 || rejected[0].Reason != "Item cannot be equipped in this slot." {
		t.Fatalf("Evoker dual-wield loadout was not rejected: %+v %+v %v", accepted, rejected, err)
	}
	s.Rules = s.Rules[1:]
	if _, _, _, err := applyGearRules(s, p, input, true); err == nil {
		t.Fatal("complete gear sets accepted without current-weapon metadata")
	}
	if err := ValidateItemRules([]ItemRule{{ID: 123, BonusIDs: []int{1, 1}, InventoryType: "INVTYPE_HEAD"}}); err == nil {
		t.Fatal("repeated bonus IDs accepted")
	}
}

func TestWorkerGearVerificationRejectsSilentlyDroppedItems(t *testing.T) {
	s := fixture(time.Now())
	comparison := []Comparison{{ID: "test", Items: []Candidate{{Slot: "head", Value: "id=456,ilevel=289"}}}}
	report := map[string]any{"sim": map[string]any{"players": []any{map[string]any{"gear": map[string]any{"main_hand": map[string]any{"encoded_item": "id=123"}}}}, "profilesets": map[string]any{"results": []any{map[string]any{"name": "test", "overrides": map[string]any{"gear": map[string]any{"head": map[string]any{"item_id": 456, "item_level": 289}}}}}}}}
	data, _ := json.Marshal(report)
	gear := []byte("main_hand=fixture_weapon,id=123\n")
	if err := verifyWorkerGear(data, gear, s, comparison); err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"item_id":456`, `"item_id":0`, 1))
	if err := verifyWorkerGear(data, gear, s, comparison); !comparisonRetryable(err) {
		t.Fatalf("missing requested item wasn't isolated: %v", err)
	}
	data = []byte(strings.Replace(string(data), `"encoded_item":"id=123"`, `"encoded_item":"id=0"`, 1))
	if err := verifyWorkerGear(data, gear, s, comparison); err == nil || comparisonRetryable(err) {
		t.Fatal("broken baseline was not fatal")
	}
}

func TestWorkerGearVerificationIncludesProcOnlyItems(t *testing.T) {
	s := fixture(time.Now())
	s.Profile += "\ntrinket2=,id=250224,bonus_id=12843/13440/6652/12699\n"
	data := []byte(`{"sim":{"players":[{"gear":{"main_hand":{"encoded_item":"fixture_weapon,id=123"}}}]}}`)
	gear := "main_hand=fixture_weapon,id=123\ntrinket2=proc_only_trinket,id=250224,bonus_id=6652/12699/12843/13440\n"
	if err := verifyWorkerGear(data, []byte(gear), s, nil); err != nil {
		t.Fatalf("active proc-only item omitted from JSON was rejected: %v", err)
	}
	for name, bad := range map[string]string{
		"missing output":         "",
		"missing item":           "main_hand=fixture_weapon,id=123\n",
		"wrong item":             strings.Replace(gear, "id=250224", "id=250225", 1),
		"ID in name":             strings.Replace(gear, "proc_only_trinket,id=250224", "id=250224", 1),
		"ID in name with fields": strings.Replace(gear, "proc_only_trinket,id=250224", "id=250224,id=250224", 1),
		"duplicate ID":           strings.Replace(gear, "id=250224", "id=250224,id=250224", 1),
		"duplicate slot":         gear + "trinket2=proc_only_trinket,id=250224\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := verifyWorkerGear(data, []byte(bad), s, nil); err == nil || comparisonRetryable(err) {
				t.Fatalf("invalid baseline equipment was not fatal: %v", err)
			}
		})
	}
	conflict := []byte(strings.Replace(string(data), `"gear":{`, `"gear":{"trinket2":{"encoded_item":"wrong_item,id=250225"},`, 1))
	if err := verifyWorkerGear(conflict, []byte(gear), s, nil); err == nil {
		t.Fatal("saved gear overrode conflicting JSON gear")
	}
	resolved, err := resolvedGearIDs([]byte("# Gear\nshoulders=shoulder_item,id=42\nwrists=wrist_item,id=43\n# gear_intellect=123\n"))
	if err != nil || resolved["shoulder"] != "42" || resolved["wrist"] != "43" {
		t.Fatalf("native slot names were not normalized: %v %v", resolved, err)
	}
}

func TestComparisonRetryDoesNotRetryBrokenWorker(t *testing.T) {
	s := fixture(time.Now())
	p := testProvider(t)
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	dir := t.TempDir()
	t.Setenv("LAMDA_SIM_TEST_WORKER", "fail")
	if _, err := p.Run(context.Background(), s, DefaultScenario(), []string{"item1"}, dir); err == nil {
		t.Fatal("generic process failure was hidden")
	}
	if _, err := os.Stat(filepath.Join(dir, "retry-baseline")); !os.IsNotExist(err) {
		t.Fatal("runtime failure incorrectly triggered retries")
	}
	// A changed registered executable must also fail before any retry.
	p.Config.SHA256 = strings.Repeat("0", 64)
	if _, err := p.Run(context.Background(), s, DefaultScenario(), []string{"item1"}, t.TempDir()); err == nil || comparisonRetryable(err) {
		t.Fatal("checksum failure was treated as an item failure")
	}
}

func TestWorkerGearVerificationRejectsStaleArtifact(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "missing_gear")
	p := testProvider(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "verified-gear.simc")
	if err := os.WriteFile(path, []byte("main_hand=old_weapon,id=123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := p.Run(context.Background(), fixture(time.Now()), DefaultScenario(), nil, dir)
	if err == nil || !strings.Contains(err.Error(), "equipment report unavailable") || comparisonRetryable(err) {
		t.Fatalf("missing fresh equipment report was not fatal: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale equipment report was retained: %v", err)
	}
}

func TestComparisonRetryKeepsValidItems(t *testing.T) {
	s := fixture(time.Now())
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 125, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	p := testProvider(t)
	// The standard test process emits a known rejected item code for item2 and
	// reports the actual requested replacement for item1.
	t.Setenv("LAMDA_SIM_TEST_WORKER", "item_rejection")
	result, err := p.Run(context.Background(), s, DefaultScenario(), []string{"item1", "item2"}, t.TempDir())
	if err != nil || len(result.Comparisons) != 1 || result.Comparisons[0].Name != "item1" || len(result.RejectedComparisons) != 1 {
		t.Fatalf("valid comparison lost: %+v %v", result, err)
	}
}

func TestComparisonKeepsValidProfilesWhenWorkerSilentlyIgnoresAnotherItem(t *testing.T) {
	s := fixture(time.Now())
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 125, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	t.Setenv("LAMDA_SIM_TEST_WORKER", "partial_gear")
	result, err := testProvider(t).Run(context.Background(), s, DefaultScenario(), []string{"item1", "item2"}, t.TempDir())
	if err != nil || len(result.Comparisons) != 1 || result.Comparisons[0].Name != "item1" || len(result.RejectedComparisons) != 1 || result.RejectedComparisons[0].ID != "item2" {
		t.Fatalf("valid profileset was lost with a silently ignored item: %+v %v", result, err)
	}
}
