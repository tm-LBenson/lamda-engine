package simming

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func foreverFixture() *Snapshot {
	item := func(key string, id, slot int, loc string, stats map[string]float64) GearItem {
		return GearItem{Key: key, ID: id, Slot: slot, EquipLoc: loc, Label: key, Link: "item:" + key, Stats: stats, StatsReady: true}
	}
	s := &Snapshot{Client: "forever", Interface: 16001, Gear: &GearSnapshot{
		Equipped: []GearItem{
			item("head", 1, 1, "INVTYPE_HEAD", map[string]float64{"ITEM_MOD_INTELLECT_SHORT": 5}),
			item("ring_one", 2, 11, "INVTYPE_FINGER", map[string]float64{"ITEM_MOD_INTELLECT_SHORT": 2}),
			item("ring_two", 3, 12, "INVTYPE_FINGER", map[string]float64{"ITEM_MOD_SPIRIT_SHORT": 4}),
			item("wand", 4, 18, "INVTYPE_RANGEDRIGHT", map[string]float64{}),
		},
		Bags: []GearItem{
			item("bag_head", 5, 0, "INVTYPE_HEAD", map[string]float64{"ITEM_MOD_INTELLECT_SHORT": 8, "ITEM_MOD_STAMINA_SHORT": 2}),
			item("bag_ring", 6, 0, "INVTYPE_FINGER", map[string]float64{"ITEM_MOD_INTELLECT_SHORT": 3}),
		},
	}}
	for _, equipped := range s.Gear.Equipped {
		s.Items = append(s.Items, Item{Slot: equipped.Slot, Link: equipped.Link})
	}
	return s
}

func TestForeverRawItemComparisons(t *testing.T) {
	s := foreverFixture()
	p := &ForeverProvider{}
	if c := p.Capability(s, time.Now()); !c.Available || c.Validation != "item-stats" || !strings.Contains(c.Reason, "combat") {
		t.Fatalf("unexpected capability: %+v", c)
	}
	result, err := p.Run(context.Background(), s, DefaultScenario(), []string{"bag_head", "bag_ring"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Metric != "item_stats" || result.Baseline.Mean != 0 || result.TargetSet != nil || len(result.Comparisons) != 3 {
		t.Fatalf("item stats must not fabricate a combat score: %+v", result)
	}
	want := map[string]float64{"ITEM_MOD_INTELLECT_SHORT": 7, "ITEM_MOD_SPIRIT_SHORT": 4}
	if !reflect.DeepEqual(result.Baseline.Stats, want) {
		t.Fatalf("baseline = %v, want %v", result.Baseline.Stats, want)
	}
	for i, stats := range []map[string]float64{
		{"ITEM_MOD_INTELLECT_SHORT": 10, "ITEM_MOD_SPIRIT_SHORT": 4, "ITEM_MOD_STAMINA_SHORT": 2},
		{"ITEM_MOD_INTELLECT_SHORT": 8, "ITEM_MOD_SPIRIT_SHORT": 4},
		{"ITEM_MOD_INTELLECT_SHORT": 10},
	} {
		if !reflect.DeepEqual(result.Comparisons[i].Stats, stats) {
			t.Errorf("comparison %d = %v, want %v", i, result.Comparisons[i].Stats, stats)
		}
	}
	if s.Gear.Bags[0].Slot != 0 || s.Gear.Equipped[0].Stats["ITEM_MOD_INTELLECT_SHORT"] != 5 {
		t.Fatal("comparison mutated snapshot")
	}
	if len(ForeverCandidates(s)) != 2 {
		t.Fatal("bag selection catalog missing")
	}
}

func TestForeverEquipmentCacheAndIdentity(t *testing.T) {
	p := &ForeverProvider{}
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.Client = "retail" },
		func(s *Snapshot) { s.Interface = 120001 },
		func(s *Snapshot) { s.Gear = nil },
		func(s *Snapshot) { s.Gear.Equipped[0].StatsReady = false },
		func(s *Snapshot) { s.Gear.Equipped[0].Stats = nil },
		func(s *Snapshot) { s.Gear.Equipped = s.Gear.Equipped[1:] },
		func(s *Snapshot) { s.Items[0].Link = "item:different" },
		func(s *Snapshot) { s.Items[1] = s.Items[0] },
		func(s *Snapshot) { s.Gear.Equipped[1].Slot = s.Gear.Equipped[0].Slot },
		func(s *Snapshot) { s.Gear.Equipped[0].Stats["ITEM_MOD_INTELLECT_SHORT"] = math.NaN() },
		func(s *Snapshot) { s.Gear.Equipped[0].Stats["ITEM_MOD_INTELLECT_SHORT"] = math.Inf(1) },
		func(s *Snapshot) { s.Gear.Equipped[0].Stats["bad <html>"] = 1 },
		func(s *Snapshot) { s.Gear.Bags[0].Key = s.Gear.Equipped[0].Key },
	} {
		s := foreverFixture()
		mutate(s)
		if p.Capability(s, time.Now()).Available {
			t.Fatal("invalid or missing capture accepted")
		}
		if _, err := p.Run(context.Background(), s, DefaultScenario(), nil, ""); err == nil {
			t.Fatal("invalid or missing capture ran")
		}
	}
	s := foreverFixture()
	s.Gear.Bags[0].StatsReady = false
	s.Gear.Bags[0].Stats = nil
	if !p.Capability(s, time.Now()).Available || len(ForeverCandidates(s)) != 1 {
		t.Fatal("uncached bag item should not erase complete equipped stats")
	}
	result, err := p.Run(context.Background(), s, DefaultScenario(), nil, "")
	if err != nil || !strings.Contains(strings.Join(result.Warnings, " "), "bag item stats were not cached") {
		t.Fatalf("missing cache warning: %v %+v", err, result)
	}
}

func TestForeverWeaponsAndEmptySlots(t *testing.T) {
	s := foreverFixture()
	add := func(key string, slot int, loc string, intelligence float64) {
		item := GearItem{Key: key, ID: 100 + slot, Slot: slot, EquipLoc: loc, Label: key, Link: "item:" + key, StatsReady: true, Stats: map[string]float64{"ITEM_MOD_INTELLECT_SHORT": intelligence}}
		s.Gear.Equipped = append(s.Gear.Equipped, item)
		s.Items = append(s.Items, Item{Slot: slot, Link: item.Link})
	}
	add("main", 16, "INVTYPE_WEAPON", 4)
	add("off", 17, "INVTYPE_HOLDABLE", 2)
	s.Gear.Bags = []GearItem{{Key: "staff", ID: 200, Label: "staff", Link: "item:staff", EquipLoc: "INVTYPE_2HWEAPON", StatsReady: true, Stats: map[string]float64{"ITEM_MOD_INTELLECT_SHORT": 9}}}
	p := &ForeverProvider{}
	result, err := p.Run(context.Background(), s, DefaultScenario(), []string{"staff"}, "")
	if err != nil || len(result.Comparisons) != 1 || result.Comparisons[0].Stats["ITEM_MOD_INTELLECT_SHORT"] != 16 {
		t.Fatalf("twohand must remove both hands: %v %+v", err, result)
	}
	for _, item := range result.Comparisons[0].Items {
		if item.Slot == "off_hand" {
			t.Fatal("twohand left an offhand equipped")
		}
	}
	s.Gear.Equipped[4].EquipLoc = "INVTYPE_2HWEAPON"
	s.Gear.Bags[0].EquipLoc = "INVTYPE_HOLDABLE"
	if len(ForeverCandidates(s)) != 0 {
		t.Fatal("offhand alone cannot replace a twohand")
	}
	if _, err := p.Run(context.Background(), s, DefaultScenario(), []string{"staff"}, ""); err == nil {
		t.Fatal("offhand/twohand conflict accepted")
	}
	naked := &Snapshot{Client: "forever", Interface: 16001, Gear: &GearSnapshot{Equipped: []GearItem{}, Bags: foreverFixture().Gear.Bags}}
	result, err = p.Run(context.Background(), naked, DefaultScenario(), []string{"bag_head"}, "")
	if err != nil || len(result.Baseline.Stats) != 0 || result.Comparisons[0].Stats["ITEM_MOD_INTELLECT_SHORT"] != 8 {
		t.Fatalf("empty equipped slot: %v %+v", err, result)
	}
}

func TestForeverUnsupportedScenariosAndSelection(t *testing.T) {
	p, s := &ForeverProvider{}, foreverFixture()
	for _, mutate := range []func(*Scenario){
		func(s *Scenario) { s.Task = "upgrades" },
		func(s *Scenario) { s.Talents = []string{"abc"} },
		func(s *Scenario) { s.HealingMode = "raid" },
		func(s *Scenario) { s.CompareMode = "topgear" },
	} {
		scenario := DefaultScenario()
		mutate(&scenario)
		if _, err := p.Run(context.Background(), s, scenario, nil, ""); err == nil {
			t.Fatal("unsupported combat/optimization scenario accepted")
		}
	}
	for _, selected := range [][]string{{"unknown"}, {"bag_head", "bag_head"}, {"1", "2", "3", "4", "5", "6", "7", "8", "9"}} {
		if _, err := p.Run(context.Background(), s, DefaultScenario(), selected, ""); err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Run(ctx, s, DefaultScenario(), nil, ""); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}
