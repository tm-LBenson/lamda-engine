package simming

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"
)

// GearSnapshot contains values read from this client's item API. It is not a
// combat model or a reconstructed character sheet. In particular, summing item
// stats cannot account for talents, racials, set bonuses, procs or weapon damage.
type GearSnapshot struct {
	Equipped []GearItem `json:"equipped"`
	Bags     []GearItem `json:"bags"`
}

type GearItem struct {
	Key        string             `json:"key"`
	ID         int                `json:"id"`
	Label      string             `json:"label"`
	Link       string             `json:"link"`
	EquipLoc   string             `json:"equipLoc"`
	Slot       int                `json:"slot"`
	Level      int                `json:"level"`
	Stats      map[string]float64 `json:"stats"`
	StatsReady bool               `json:"statsReady"`
}

var itemStatKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,95}$`)

// Validate bounds untrusted imports but preserves a failed item cache lookup.
// Capability reports incomplete equipped data instead of converting it to zero.
func (g *GearSnapshot) Validate() error {
	if g == nil {
		return nil
	}
	if len(g.Equipped) > 19 || len(g.Bags) > 256 {
		return fmt.Errorf("too many captured gear items")
	}
	keys, equippedSlots := map[string]bool{}, map[int]bool{}
	for source, items := range [][]GearItem{g.Equipped, g.Bags} {
		for _, item := range items {
			if !requestID.MatchString(item.Key) || keys[item.Key] || item.ID < 1 || item.ID > 100000000 || len(item.Label) > 256 || len(item.Link) == 0 || len(item.Link) > 4096 || len(item.EquipLoc) > 64 || item.Level < 0 || item.Level > 10000 || len(item.Stats) > 96 {
				return fmt.Errorf("invalid captured gear item")
			}
			keys[item.Key] = true
			if source == 0 {
				if item.Slot < 1 || item.Slot > 19 || equippedSlots[item.Slot] {
					return fmt.Errorf("invalid or duplicate captured equipment slot")
				}
				equippedSlots[item.Slot] = true
			} else if item.Slot != 0 {
				return fmt.Errorf("bag item contains an equipped slot")
			}
			if item.StatsReady && item.Stats == nil {
				return fmt.Errorf("captured item stats are missing")
			}
			for stat, value := range item.Stats {
				if !itemStatKey.MatchString(stat) || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 10000000 {
					return fmt.Errorf("invalid captured item stat")
				}
			}
		}
	}
	return nil
}

type ForeverProvider struct{}

const foreverStatsVersion = "client-item-stats-1"

func (p *ForeverProvider) Capability(s *Snapshot, _ time.Time) Capability {
	c := Capability{ID: "forever-stats", Name: "Forever item stats", Version: foreverStatsVersion, Validation: "item-stats"}
	if s == nil || s.Client != "forever" {
		c.Reason = "Item-stat comparison requires a Forever snapshot."
		return c
	}
	if err := validateForeverGear(s); err != nil {
		c.Reason = err.Error()
		return c
	}
	c.Available = true
	c.Reason = "Item stats only. Forever combat calculations are unavailable."
	return c
}

func validateForeverGear(s *Snapshot) error {
	if s == nil || s.Client != "forever" || s.Interface != 16001 {
		return fmt.Errorf("item-stat comparison requires a Forever snapshot")
	}
	if s.Gear == nil {
		return fmt.Errorf("capture a new snapshot to compare Forever item stats")
	}
	if err := s.Gear.Validate(); err != nil {
		return err
	}
	if len(s.Items) != len(s.Gear.Equipped) {
		return fmt.Errorf("equipped item capture is incomplete; capture a new snapshot")
	}
	seen := map[int]bool{}
	for _, item := range s.Items {
		if seen[item.Slot] {
			return fmt.Errorf("duplicate equipped item in snapshot")
		}
		seen[item.Slot] = true
		found := false
		for _, equipped := range s.Gear.Equipped {
			if equipped.Slot == item.Slot && equipped.Link == item.Link {
				if !equipped.StatsReady {
					return fmt.Errorf("equipped item stats were not cached; capture a new snapshot")
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("equipped item capture changed; capture a new snapshot")
		}
	}
	return nil
}

var foreverSlotNames = map[int]string{
	1: "head", 2: "neck", 3: "shoulder", 4: "shirt", 5: "chest", 6: "waist", 7: "legs", 8: "feet", 9: "wrist", 10: "hands",
	11: "finger1", 12: "finger2", 13: "trinket1", 14: "trinket2", 15: "back", 16: "main_hand", 17: "off_hand", 18: "ranged", 19: "tabard",
}

var foreverEquipSlots = map[string][]int{
	"INVTYPE_HEAD": {1}, "INVTYPE_NECK": {2}, "INVTYPE_SHOULDER": {3}, "INVTYPE_BODY": {4}, "INVTYPE_CHEST": {5}, "INVTYPE_ROBE": {5},
	"INVTYPE_WAIST": {6}, "INVTYPE_LEGS": {7}, "INVTYPE_FEET": {8}, "INVTYPE_WRIST": {9}, "INVTYPE_HAND": {10},
	"INVTYPE_FINGER": {11, 12}, "INVTYPE_TRINKET": {13, 14}, "INVTYPE_CLOAK": {15}, "INVTYPE_WEAPON": {16}, "INVTYPE_2HWEAPON": {16},
	"INVTYPE_WEAPONMAINHAND": {16}, "INVTYPE_WEAPONOFFHAND": {17}, "INVTYPE_HOLDABLE": {17}, "INVTYPE_SHIELD": {17},
	"INVTYPE_RANGED": {18}, "INVTYPE_THROWN": {18}, "INVTYPE_RANGEDRIGHT": {18}, "INVTYPE_RELIC": {18}, "INVTYPE_TABARD": {19},
}

func foreverPlacementSlots(s *Snapshot, item GearItem) []int {
	placements := foreverEquipSlots[item.EquipLoc]
	if len(placements) == 1 && placements[0] == 17 {
		for _, equipped := range s.Gear.Equipped {
			if equipped.Slot == 16 && equipped.EquipLoc == "INVTYPE_2HWEAPON" {
				// A lone off-hand cannot replace a two-handed weapon. Do not
				// infer that this character can dual wield either weapon type.
				return nil
			}
		}
	}
	return placements
}

// ForeverCandidates uses physical bag keys so two identical owned items retain
// their identity. The UI must not describe these as evaluated combat upgrades.
func ForeverCandidates(s *Snapshot) []Candidate {
	candidates := []Candidate{}
	if validateForeverGear(s) != nil {
		return candidates
	}
	for _, item := range s.Gear.Bags {
		placement := foreverPlacementSlots(s, item)
		if !item.StatsReady || len(placement) == 0 {
			continue
		}
		candidates = append(candidates, Candidate{ID: item.Key, Label: item.Label, Source: "owned", Slot: foreverSlotNames[placement[0]], Value: "id=" + strconv.Itoa(item.ID)})
	}
	return candidates
}

func foreverMetric(label string, items []GearItem) Metric {
	m := Metric{Name: label, Label: label, Model: "Observed item stats", Stats: map[string]float64{}, Items: []ResultItem{}}
	for _, item := range items {
		for stat, value := range item.Stats {
			m.Stats[stat] += value
		}
		m.Items = append(m.Items, ResultItem{ID: item.ID, Level: item.Level, Slot: foreverSlotNames[item.Slot], Label: item.Label, Key: item.Key})
	}
	return m
}

func (p *ForeverProvider) Run(ctx context.Context, s *Snapshot, scenario Scenario, selected []string, _ string) (*Result, error) {
	if len(scenario.ItemOptions) > 0 || len(scenario.Encounters) > 0 || scenario.Bloodlust != "" {
		return nil, fmt.Errorf("Forever has no combat encounter or item-alternative model")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateForeverGear(s); err != nil {
		return nil, err
	}
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	if scenario.Task != "" || len(scenario.Talents) > 0 || scenario.HealingMode != "" {
		return nil, fmt.Errorf("Forever supports captured item-stat comparisons only; combat, talent and loot models are unavailable")
	}
	if scenario.CompareMode == "topgear" || scenario.CompareMode == "loadout" {
		return nil, fmt.Errorf("Forever supports individual item-stat comparisons; it cannot simulate a complete gear set")
	}
	if len(selected) > 8 {
		return nil, fmt.Errorf("compare at most eight bag items per run")
	}
	result := &Result{Provider: "LamdaUI item stats", Model: "Observed item stats", Metric: "item_stats", Baseline: foreverMetric("Equipped item stats", s.Gear.Equipped), Comparisons: []Metric{}, ProviderVersion: foreverStatsVersion, Validation: "item-stats", Warnings: []string{
		"Item API values only; these are not character-sheet stats, DPS or healing predictions.",
		"Procs, set bonuses, talents, racial effects, weapon damage and enchant/gem effects absent from the item API are not calculated.",
		"Compare items you can equip. Class, level, proficiency and unique-equipped restrictions are not validated; no upgrade ranking is inferred.",
	}}
	result.Warnings = append(result.Warnings, s.Warnings...)
	for _, item := range s.Gear.Bags {
		if !item.StatsReady {
			result.Warnings = append(result.Warnings, "Some bag item stats were not cached and were excluded.")
			break
		}
	}
	seen := map[string]bool{}
	for _, key := range selected {
		if seen[key] {
			return nil, fmt.Errorf("duplicate selected bag item")
		}
		seen[key] = true
		var candidate *GearItem
		for i := range s.Gear.Bags {
			if s.Gear.Bags[i].Key == key {
				candidate = &s.Gear.Bags[i]
				break
			}
		}
		if candidate == nil || !candidate.StatsReady {
			return nil, fmt.Errorf("selected bag item stats are unavailable")
		}
		placements := foreverPlacementSlots(s, *candidate)
		if len(placements) == 0 {
			return nil, fmt.Errorf("selected item has no supported replacement slot")
		}
		for _, slot := range placements {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			items := make([]GearItem, 0, len(s.Gear.Equipped)+1)
			for _, equipped := range s.Gear.Equipped {
				if equipped.Slot == slot || (candidate.EquipLoc == "INVTYPE_2HWEAPON" && equipped.Slot == 17) {
					continue
				}
				items = append(items, equipped)
			}
			replacement := *candidate
			replacement.Slot = slot
			items = append(items, replacement)
			result.Comparisons = append(result.Comparisons, foreverMetric(candidate.Label+" ("+foreverSlotNames[slot]+")", items))
		}
	}
	return result, nil
}
