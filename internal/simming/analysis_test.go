package simming

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAutomaticAnalysisRunsEveryCapturedCandidateForRaidAndDungeon(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "analysis")
	snapshot := fixture(time.Now())
	snapshot.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 125, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	scenario := DefaultScenario()
	scenario.Task = "analysis"
	result, err := testProvider(t).Run(context.Background(), snapshot, scenario, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.TargetResults) != 2 || len(result.Comparisons) != 0 || result.TargetResults[0].Targets != 1 || result.TargetResults[1].Targets != 5 {
		t.Fatalf("unexpected target result shape: %+v", result.TargetResults)
	}
	if first, second := result.TargetResults[0], result.TargetResults[1]; first.FightStyle != "patchwerk" || first.Seconds != 300 ||
		second.FightStyle != "dungeon_slice" || second.Seconds != 360 || second.RaidBuffs != "none" {
		t.Fatalf("analysis did not retain its raid and dungeon contexts: %+v", result.TargetResults)
	}
	for _, target := range result.TargetResults {
		if len(target.Comparisons) != 2 || target.Baseline.Mean <= 0 {
			t.Fatalf("target %d did not retain all candidates: %+v", target.Targets, target)
		}
		if target.Comparisons[0].CandidateID == "" || target.Comparisons[0].Source == "" || target.Comparisons[0].Slot != "main_hand" {
			t.Fatalf("candidate identity was not retained: %+v", target.Comparisons[0])
		}
		if target.Refined == 0 || !target.Comparisons[0].Refined {
			t.Fatalf("precise comparisons were not identified: %+v", target)
		}
	}
}

func TestLeadingComparisonsReservesPrecisionForEachDecisionPage(t *testing.T) {
	metrics := []Metric{
		{Name: "owned_max_1", Source: "owned", Variant: "max", Mean: 120},
		{Name: "owned_max_2", Source: "owned", Variant: "max", Mean: 119},
		{Name: "owned_max_3", Source: "owned", Variant: "max", Mean: 118},
		{Name: "owned_now_1", Source: "owned", Variant: "current", Mean: 110},
		{Name: "owned_now_2", Source: "owned", Variant: "current", Mean: 109},
		{Name: "equipped_next", Source: "equipped", Variant: "next", Mean: 108},
		{Name: "vault_now", Source: "vault", Variant: "current", Mean: 107},
		{Name: "talent", Source: "talent", Variant: "talent", Mean: 106},
	}
	comparisons := make([]Comparison, 0, len(metrics))
	for _, metric := range metrics {
		comparisons = append(comparisons, Comparison{ID: metric.Name})
	}
	got := leadingComparisons(metrics, comparisons, 6)
	wanted := map[string]bool{}
	for _, comparison := range got {
		wanted[comparison.ID] = true
	}
	for _, id := range []string{"owned_now_1", "owned_now_2", "owned_max_1", "owned_max_2", "equipped_next", "vault_now"} {
		if !wanted[id] {
			t.Fatalf("%s did not receive reserved refinement: %+v", id, got)
		}
	}
}

func TestBestCandidateMetricsKeepsTheBestPlacement(t *testing.T) {
	metrics := []Metric{{Name: "item1", CandidateID: "item1", Mean: 100}, {Name: "item1v2", CandidateID: "item1", Mean: 110}, {Name: "item2", CandidateID: "item2", Mean: 105}}
	got := bestCandidateMetrics(metrics)
	if len(got) != 2 || got[0].Name != "item1v2" || got[1].Name != "item2" {
		t.Fatalf("placement collapse was not ranked: %+v", got)
	}
}

func TestAutomaticAnalysisIncludesSavedTalentsAndMaximumUpgradeVariants(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "analysis")
	snapshot := fixture(time.Now())
	snapshot.Profile = strings.Replace(snapshot.Profile, "### Gear from Bags", "# Saved Loadout: Raid\n# talents=RAID==\n### Gear from Bags", 1)
	snapshot.GameRequest = &GameRequest{Task: "analysis", Talents: []TalentBuild{{Label: "Imported build", Code: "IMPORT=="}}}
	snapshot.Upgrades = []ItemUpgrade{
		{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3", Label: "Bag staff", Level: 289, NextLevel: 292, MaxLevel: 300},
		{Source: "equipped", Slot: "main_hand", Value: "id=123,bonus_id=1/2", Label: "Equipped staff", Level: 280, NextLevel: 284, MaxLevel: 300, Track: "Hero", CurrentRank: 3, MaxRank: 6, RankLevels: []int{280, 284, 292, 300}},
	}
	snapshot.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 125, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	scenario := DefaultScenario()
	scenario.Task = "analysis"
	result, err := testProvider(t).Run(context.Background(), snapshot, scenario, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range result.TargetResults {
		var current, maximum, equippedNext, equippedMiddle, equippedMaximum, equippedSet, saved, imported bool
		for _, metric := range target.Comparisons {
			if metric.CandidateID == "item1" && metric.Variant == "current" {
				current = true
			}
			if metric.CandidateID == "item1" && metric.Variant == "max" && metric.Level == 300 && metric.MaxLevel == 300 {
				maximum = true
			}
			if metric.CandidateID == "equipped_main_hand" && metric.Source == "equipped" && metric.Slot == "main_hand" && metric.Variant == "next" && metric.Level == 284 && metric.MaxLevel == 300 {
				equippedNext = true
			}
			if metric.CandidateID == "equipped_main_hand" && metric.Source == "equipped" && metric.Variant == "rank_5" && metric.Level == 292 {
				equippedMiddle = true
			}
			if metric.CandidateID == "equipped_main_hand" && metric.Source == "equipped" && metric.Variant == "max" && metric.Level == 300 {
				equippedMaximum = true
			}
			if metric.CandidateID == "equipped_all" && metric.Source == "equipped" && metric.Variant == "all_max" && metric.Level == 300 {
				equippedSet = true
			}
			if metric.Source == "talent" && metric.Label == "Raid" {
				saved = true
			}
			if metric.Source == "talent" && metric.Label == "Imported build" {
				imported = true
			}
		}
		if !current || !maximum || !equippedNext || !equippedMiddle || !equippedMaximum || !equippedSet || !saved || !imported || !target.Complete || target.Tested != target.Requested {
			t.Fatalf("automatic coverage is incomplete: %+v", target)
		}
	}
}

func TestAutomaticAnalysisRetainsWorkerRejectedItemsInEachTargetResult(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "partial_gear")
	snapshot := fixture(time.Now())
	snapshot.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 125, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	scenario := DefaultScenario()
	scenario.Task = "analysis"
	result, err := testProvider(t).Run(context.Background(), snapshot, scenario, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RejectedComparisons) != 1 || len(result.TargetResults) != 2 {
		t.Fatalf("rejected item summary missing: %+v", result)
	}
	for _, target := range result.TargetResults {
		if len(target.Comparisons) != 1 || len(target.RejectedComparisons) != 1 || target.RejectedComparisons[0].ID != "item2" {
			t.Fatalf("target rejection missing: %+v", target)
		}
	}
}

func TestRetryMetricsAreNormalizedToTheCanonicalBaseline(t *testing.T) {
	metrics := []Metric{{Mean: 110, StdDev: 4, MeanError: 2}}
	normalizeComparisonMetrics(metrics, Metric{Mean: 100}, Metric{Mean: 200})
	if metrics[0].Mean != 220 || metrics[0].StdDev != 8 || metrics[0].MeanError != 4 {
		t.Fatalf("retry metric was not normalized: %+v", metrics[0])
	}
}
