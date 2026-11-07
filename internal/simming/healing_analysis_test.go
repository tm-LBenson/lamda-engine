package simming

import (
	"testing"
	"time"
)

func TestHealingAnalysisCandidatesIncludeOwnedMaximumsAndEquippedRanks(t *testing.T) {
	snapshot := fixture(time.Now())
	profile, err := ParseProfile(snapshot.Profile)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Upgrades = []ItemUpgrade{
		{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3", Label: "Bag staff", Level: 289, MaxLevel: 300},
		{Source: "equipped", Slot: "main_hand", Value: "id=123,bonus_id=1/2", Label: "Equipped staff", Level: 280, NextLevel: 284, MaxLevel: 300, Track: "Hero", CurrentRank: 4, MaxRank: 6, RankLevels: []int{280, 284, 300}},
	}
	candidates := healingAnalysisCandidates(snapshot, profile)
	found := map[string]Candidate{}
	for _, candidate := range candidates {
		found[candidate.ComparisonID+"\x00"+candidate.Variant] = candidate
	}
	for _, key := range []string{"item1\x00current", "item1\x00max", "item2\x00current", "equipped_main_hand\x00next", "equipped_main_hand\x00max", "equipped_all\x00all_max"} {
		if _, ok := found[key]; !ok {
			t.Fatalf("missing healer analysis candidate %q: %+v", key, candidates)
		}
	}
	if got := found["item1\x00max"]; got.Level != 300 || got.MaxLevel != 300 || itemLevelFromValue(got.Value) != 300 || got.ID == got.ComparisonID {
		t.Fatalf("owned maximum was not materialized safely: %+v", got)
	}
	if got := found["equipped_main_hand\x00next"]; got.Level != 284 || got.Source != "equipped" || got.ID == got.ComparisonID {
		t.Fatalf("equipped next rank metadata is incomplete: %+v", got)
	}
}
