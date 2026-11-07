package simming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVenomstonesUseExactTracksAndPreserveInstanceModifiers(t *testing.T) {
	for _, tc := range []struct {
		name, value, bonus string
		level              int
	}{
		{"Hero lower rank", "id=159617,bonus_id=10334/12841/42,gem_id=123/456,enchant_id=789", "10334/12848/42", 328},
		{"Hero capped", "id=159617,bonus_id=10334/12846/42", "10334/12848/42", 328},
		{"Myth lower rank", "id=158374,bonus_id=10334/12849/42", "10334/12856/42", 341},
		{"Myth capped", "id=158374,bonus_id=10334/12854/42", "10334/12856/42", 341},
		{"Tidal Myth craft", "id=237850,bonus_id=8790/13751/13836/9627,crafted_stats=49/36", "8790/13751/13836/9627/13852", 337},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := Candidate{ID: "item1", Source: "vault", Slot: "trinket1", Value: tc.value, Label: "Exact item"}
			got, err := adjustedItem(original, ItemAdjustment{Venomstone: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.BaseValue != tc.value || got.Variant != "planned" || got.Level != tc.level || got.Venomstones != 10 || itemField(got.Value, "bonus_id") != tc.bonus {
				t.Fatalf("wrong transformation: %+v", got)
			}
			for _, key := range []string{"id", "gem_id", "enchant_id", "crafted_stats"} {
				if itemField(got.Value, key) != itemField(original.Value, key) {
					t.Fatalf("changed %s", key)
				}
			}
			if original.Value != tc.value || original.Venomstones != 0 {
				t.Fatal("original physical item mutated")
			}
		})
	}
}

func TestVenomstonesRejectUnsupportedAndAlreadyAscendedItems(t *testing.T) {
	for _, tc := range []struct{ slot, value string }{
		{"back", "id=1,bonus_id=12846"}, {"finger1", "id=1,bonus_id=12854"},
		{"neck", "id=1,bonus_id=12848"}, {"neck", "id=1,bonus_id=12856"},
		{"main_hand", "id=1,bonus_id=13751/13836/9627/13852"},
		{"neck", "id=1,ilevel=334"}, {"neck", "id=1,bonus_id=12841/12854"},
		{"main_hand", "id=1,bonus_id=13751/13836,crafting_quality=4"},
		{"main_hand", "id=1,bonus_id=13751/13835/9627"},
	} {
		if _, err := applyVenomstone(Candidate{Slot: tc.slot, Value: tc.value}); err == nil {
			t.Fatalf("accepted unsupported %s=%s", tc.slot, tc.value)
		}
	}
}

func TestVenomstoneChoicesRemainAlternativesForOnePhysicalBagOrVaultItem(t *testing.T) {
	for _, source := range []string{"owned", "vault"} {
		t.Run(source, func(t *testing.T) {
			s := fixture(time.Now())
			s.Interface = 120105
			p, err := ParseProfile(s.Profile)
			if err != nil {
				t.Fatal(err)
			}
			p.Candidates = []Candidate{{ID: "item1", Source: source, Slot: "trinket1", Value: "id=159617,bonus_id=12846", Label: "Plumage"}}
			scenario := DefaultScenario()
			scenario.CompareMode = "topgear"
			scenario.ItemOptions = map[string][]ItemAdjustment{"item1": {{Venomstone: true}}}
			prepared, err := prepareComparisonProfile(s, p, scenario, []string{"item1"})
			if err != nil {
				t.Fatal(err)
			}
			comparisons, err := prepared.Comparisons(scenario, []string{"item1"})
			if err != nil {
				t.Fatal(err)
			}
			normal, ascended := false, false
			for _, c := range comparisons {
				if len(c.Items) != 1 {
					t.Fatal("one physical trinket duplicated")
				}
				if c.Items[0].Venomstones == 10 {
					ascended = true
				} else {
					normal = true
				}
			}
			if !normal || !ascended {
				t.Fatal("normal and Venomforged alternatives were not both tested")
			}
			for _, change := range []string{"patch", "role", "client"} {
				copy := *s
				switch change {
				case "patch":
					copy.Interface = 120100
				case "role":
					copy.Character.Role = "HEALER"
				case "client":
					copy.Client = "forever"
				}
				if _, err := prepareComparisonProfile(&copy, p, scenario, []string{"item1"}); err == nil {
					t.Fatalf("unsupported %s accepted", change)
				}
			}
		})
	}
}

func TestVenomstoneVerificationRejectsCorrectLevelWithIgnoredAscensionBonus(t *testing.T) {
	item, err := adjustedItem(Candidate{Slot: "trinket1", Value: "id=159617,bonus_id=10334/12846"}, ItemAdjustment{Venomstone: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	report := []byte(`{"sim":{"players":[{"gear":{"trinket1":{"ilevel":328}}}]}}`)
	if err := os.WriteFile(filepath.Join(dir, "result.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "verified-gear.simc"), []byte("trinket1=plumage,"+value+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := Comparison{Items: []Candidate{item}}
	write("id=159617,bonus_id=10334/12846")
	if err := verifyPlannedItems(dir, c); err == nil || !strings.Contains(err.Error(), "Venomstone") {
		t.Fatalf("ignored ascension accepted: %v", err)
	}
	write(item.Value)
	if err := verifyPlannedItems(dir, c); err != nil {
		t.Fatal(err)
	}
	var metric Metric
	decorateMetric(&metric, c)
	data, _ := json.Marshal(metric)
	if !strings.Contains(string(data), `"venomstones":10`) {
		t.Fatal("native result lost stone count")
	}
}

func TestVenomstoneRestrictionInheritanceRetainsUniqueAndWeaponLimits(t *testing.T) {
	base := Candidate{Slot: "trinket1", Value: "id=159617,bonus_id=12846/42"}
	planned, err := adjustedItem(base, ItemAdjustment{Venomstone: true})
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]ItemRule{valueRuleKey(base.Value): {ID: 159617, BonusIDs: []int{12846, 42}, InventoryType: "INVTYPE_TRINKET", Unique: true, LimitCategory: 123, LimitMax: 1}}
	got, ok := candidateRule(rules, planned)
	if !ok || !got.Unique || got.LimitCategory != 123 || got.LimitMax != 1 {
		t.Fatal("captured restrictions lost")
	}
	bad := planned
	bad.Value += ",bonus_id=999"
	if _, ok := candidateRule(rules, bad); ok {
		t.Fatal("unreviewed bonus change inherited restrictions")
	}
	s := fixture(time.Now())
	s.Rules = []ItemRule{rules[valueRuleKey(base.Value)]}
	p := &Profile{Class: "hunter", Spec: "beast_mastery", Base: "trinket2=" + base.Value + "\n"}
	accepted, rejected, _, err := applyGearRules(s, p, []Comparison{{Items: []Candidate{planned}}}, true)
	if err != nil || len(accepted) != 0 || len(rejected) != 1 || !strings.Contains(strings.ToLower(rejected[0].Reason), "unique") {
		t.Fatalf("duplicate unique trinket accepted: %+v %+v %v", accepted, rejected, err)
	}
	weapon := Candidate{Slot: "main_hand", Value: "id=1,bonus_id=12854"}
	weaponPlan, err := adjustedItem(weapon, ItemAdjustment{Venomstone: true})
	if err != nil {
		t.Fatal(err)
	}
	off := Candidate{Slot: "off_hand", Value: "id=2,bonus_id=42"}
	s.Rules = []ItemRule{{ID: 1, BonusIDs: []int{12854}, InventoryType: "INVTYPE_2HWEAPON"}, {ID: 2, BonusIDs: []int{42}, InventoryType: "INVTYPE_HOLDABLE"}}
	p.Base = ""
	accepted, rejected, _, err = applyGearRules(s, p, []Comparison{{Items: []Candidate{weaponPlan, off}}}, true)
	if err != nil || len(accepted) != 0 || len(rejected) != 1 || !strings.Contains(rejected[0].Reason, "Two-handed") {
		t.Fatal("Venomforged two-hander lost offhand restrictions")
	}
}
