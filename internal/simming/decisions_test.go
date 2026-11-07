package simming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecisionsKeepPersonalResultsSeparateFromReferencesAndStaleBuilds(t *testing.T) {
	snapshot := fixture(time.Now())
	hash := setupHash(snapshot)
	state := WorkspaceState{Client: "retail", Snapshots: []Summary{{Snapshot: *snapshot, SetupHash: hash}}, Jobs: []Job{
		{ID: "old", SnapshotID: snapshot.ID, SetupHash: "changed", State: "complete", Result: &Result{Metric: "dps", Baseline: Metric{Mean: 100}, Comparisons: []Metric{{Mean: 9999}}}},
		{ID: "fresh", SnapshotID: snapshot.ID, SetupHash: hash, State: "complete", CreatedAt: time.Now(), Scenario: DefaultScenario(), Result: &Result{Metric: "dps", Validation: "unreviewed", Baseline: Metric{Mean: 1000, MeanError: 10}, Comparisons: []Metric{
			{Name: "joint", Source: "set", Label: "Two-item swap", Mean: 1100, MeanError: 5, Items: []ResultItem{{ID: 101, Source: "owned"}, {ID: 102, Source: "owned"}}},
			{Name: "close", Source: "owned", Mean: 1001, MeanError: 5},
			{Name: "planned", Source: "equipped", Variant: "next", Mean: 1200, Items: []ResultItem{{ID: 123, Level: 350, Variant: "planned"}}}},
			Upgrades: []Upgrade{{ID: 201, Label: "Raid target", Kind: "Raid", Gain: 5, LootVerified: true, PoolComplete: true, BIS: true}, {ID: 202, Label: "Dungeon target", Kind: "Dungeon", Gain: 8, LootVerified: true, PoolComplete: false}, {ID: 203, Kind: "Dungeon", Gain: 99, Owned: true, LootVerified: true}}}},
		{ID: "trend", State: "complete", Result: &Result{Builds: &BuildReport{Spec: 64, Mode: "m+", Source: "Murlok", URL: "https://murlok.io/mage/frost/mm+", UpdatedAt: time.Now(), Sample: 50, Gear: []BuildGear{{ID: 999, BIS: true}}}}},
	}}
	report, err := buildDecisions(state, snapshot.ID, "m+")
	if err != nil {
		t.Fatal(err)
	}
	if report.StaleJobs != 1 || len(report.Views) != 1 {
		t.Fatalf("stale result reused: %+v", report)
	}
	view := report.Views[0]
	if len(view.Equip) != 2 || len(view.Equip[0].Items) != 2 || view.Equip[0].Absolute != 100 || view.Equip[0].Percent != 10 {
		t.Fatal("joint gains were lost or added together")
	}
	if !view.Equip[1].WithinMargin || view.Equip[1].Advice != "within-margin" {
		t.Fatal("uncertainty lost")
	}
	if len(view.Upgrade) != 1 || !view.Upgrade[0].Investment || view.Upgrade[0].CostKnown {
		t.Fatal("upgrade cost invented")
	}
	if len(view.Farm) != 1 || view.Farm[0].Item.ID != 202 || !strings.Contains(view.Farm[0].Evidence, "incomplete") || view.Farm[0].Item.BIS {
		t.Fatal("farm context or evidence lost")
	}
	if len(report.References) != 2 || report.References[0].PersonalRecommendation || report.References[0].Gear[0].BIS || report.References[1].Available {
		t.Fatal("community data became a personal recommendation")
	}
	if !state.Jobs[2].Result.Builds.Gear[0].BIS || !state.Jobs[1].Result.Upgrades[0].BIS {
		t.Fatal("historical evidence mutated")
	}
}

func TestSetupIdentityIgnoresBagOrderButIncludesTalentsAndExactGear(t *testing.T) {
	original := fixture(time.Now())
	copy := *original
	copy.Profile = strings.Replace(original.Profile, "### Gear from Bags", "### Gear from Bags\n# New loot\n# neck=,id=888", 1)
	if setupHash(original) != setupHash(&copy) {
		t.Fatal("new bag item invalidated unchanged equipped baseline")
	}
	copy.Profile = strings.Replace(original.Profile, "id=123,bonus_id=1/2", "id=123,bonus_id=1/2,gem_id=99", 1)
	if setupHash(original) == setupHash(&copy) {
		t.Fatal("gem change reused old gains")
	}
	copy.Profile = strings.Replace(original.Profile, "talents=ABCD+ef/==", "talents=CHANGED==", 1)
	if setupHash(original) == setupHash(&copy) {
		t.Fatal("talent change reused old gains")
	}
}

func TestDecisionsRequireAvailablePhysicalItemsAndRemoveAcquiredFarmTargets(t *testing.T) {
	s := fixture(time.Now())
	bag := Candidate{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3"}
	state := WorkspaceState{Snapshots: []Summary{{Snapshot: *s, SetupHash: setupHash(s), Candidates: []Candidate{bag}}}, Jobs: []Job{{ID: "fresh", SnapshotID: s.ID, SetupHash: setupHash(s), State: "complete", Scenario: DefaultScenario(), Result: &Result{Metric: "dps", Baseline: Metric{Mean: 1000}, Comparisons: []Metric{
		{Name: "equip", Mean: 1100, Items: []ResultItem{{ID: 124, Source: "owned", Key: bag.Value}}},
		{Name: "upgraded", Mean: 1200, Items: []ResultItem{{ID: 124, Source: "owned", Key: bag.Value + ",ilevel=350", BaseKey: bag.Value, Variant: "planned"}}},
		{Name: "missing", Mean: 1500, Items: []ResultItem{{ID: 999, Source: "owned", Key: "id=999"}}},
		{Name: "duplicated", Mean: 1600, Items: []ResultItem{{ID: 124, Source: "owned", Key: bag.Value}, {ID: 124, Source: "owned", Key: bag.Value}}},
	}, Upgrades: []Upgrade{{ID: 124, Kind: "dungeon", Gain: 20, LootVerified: true}, {ID: 125, Kind: "dungeon", Gain: 10, LootVerified: true}}}}}}
	report, err := buildDecisions(state, s.ID, "m+")
	if err != nil {
		t.Fatal(err)
	}
	v := report.Views[0]
	if len(v.Equip) != 1 || v.Equip[0].Comparison != "equip" || !v.Equip[0].AvailabilityVerified || len(v.Upgrade) != 1 || !v.Upgrade[0].AvailabilityVerified {
		t.Fatalf("unavailable/duplicate items reused or original planned identity lost: %+v", v)
	}
	if len(v.Farm) != 1 || v.Farm[0].Item.ID != 125 {
		t.Fatal("acquired target stayed in farm recommendations")
	}
	state.Snapshots[0].Candidates = nil
	report, err = buildDecisions(state, s.ID, "m+")
	if err != nil || len(report.Views[0].Equip) != 0 || len(report.Views[0].Upgrade) != 0 {
		t.Fatal("removed item stayed actionable")
	}
}

func TestPlannedVariantVerificationRejectsIgnoredEnchantGemAndRank(t *testing.T) {
	dir := t.TempDir()
	zero := 0
	candidate := Candidate{ID: "equipped_finger1", Source: "equipped", Slot: "finger1", Value: "id=201,gem_id=901,enchant_id=999"}
	wanted, err := adjustedItem(candidate, ItemAdjustment{Level: 350, EnchantID: &zero, GemIDs: []int{902}})
	if err != nil {
		t.Fatal(err)
	}
	comparison := Comparison{ID: "planned", Items: []Candidate{wanted}}
	write := func(gear string, level int) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "verified-gear.simc"), []byte(gear), 0600); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(map[string]any{"sim": map[string]any{"players": []any{map[string]any{"gear": map[string]any{"finger1": map[string]any{"ilevel": level}}}}}})
		if err := os.WriteFile(filepath.Join(dir, "result.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("finger1=ring,id=201,gem_id=901,enchant_id=999\n", 350)
	if verifyPlannedItems(dir, comparison) == nil {
		t.Fatal("ignored modifiers accepted")
	}
	write("finger1=ring,id=201,gem_id=902\n", 340)
	if verifyPlannedItems(dir, comparison) == nil {
		t.Fatal("ignored item level accepted")
	}
	write("finger1=ring,id=201,gem_id=902\n", 350)
	if err := verifyPlannedItems(dir, comparison); err != nil {
		t.Fatal(err)
	}
}

func TestEquippedAlternativesCannotDuplicateOnePhysicalRing(t *testing.T) {
	s := fixture(time.Now())
	s.Profile += "finger1=,id=201\nfinger2=,id=202\n"
	p, err := ParseProfile(s.Profile)
	if err != nil {
		t.Fatal(err)
	}
	scenario := DefaultScenario()
	scenario.CompareMode = "topgear"
	prepared, err := prepareComparisonProfile(s, p, scenario, []string{"equipped_finger1"})
	if err != nil {
		t.Fatal(err)
	}
	combinations, err := prepared.Comparisons(scenario, []string{"equipped_finger1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range combinations {
		for _, item := range c.Items {
			if item.Slot != "finger1" {
				t.Fatal("one worn ring was copied into its other slot")
			}
		}
	}
}
