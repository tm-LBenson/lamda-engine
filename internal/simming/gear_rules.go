package simming

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ItemRule records public client metadata for one item variant. The absence of
// a rule means unknown, not unrestricted. Bonus IDs can change limit categories.
type ItemRule struct {
	ID            int    `json:"id"`
	BonusIDs      []int  `json:"bonusIDs"`
	InventoryType string `json:"inventoryType"`
	Unique        bool   `json:"unique"`
	LimitCategory int    `json:"limitCategory"`
	LimitMax      int    `json:"limitMax"`
}

type RejectedComparison struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

func ruleKey(id int, bonuses []int) string {
	copy := append([]int{}, bonuses...)
	sort.Ints(copy)
	parts := []string{strconv.Itoa(id)}
	for _, bonus := range copy {
		parts = append(parts, strconv.Itoa(bonus))
	}
	return strings.Join(parts, ":")
}

func valueRuleKey(value string) string {
	id, _ := strconv.Atoi(itemID(value))
	bonuses := []int{}
	for _, part := range strings.Split(value, ",") {
		if key, val, _ := strings.Cut(part, "="); key == "bonus_id" {
			for _, text := range strings.Split(val, "/") {
				bonus, _ := strconv.Atoi(text)
				bonuses = append(bonuses, bonus)
			}
		}
	}
	return ruleKey(id, bonuses)
}

func ValidateItemRules(rules []ItemRule) error {
	if len(rules) > 768 {
		return fmt.Errorf("too many item restriction records")
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		if rule.ID <= 0 || rule.ID > 10000000 || len(rule.BonusIDs) > 64 || lootInventoryTypes[rule.InventoryType] == "" || rule.LimitCategory < 0 || rule.LimitCategory > 10000000 || rule.LimitMax < 0 || rule.LimitMax > 100 || (rule.LimitCategory > 0 && rule.LimitMax == 0) || (rule.LimitCategory == 0 && rule.LimitMax != 0) {
			return fmt.Errorf("invalid item restriction record")
		}
		bonusSeen := map[int]bool{}
		for _, bonus := range rule.BonusIDs {
			if bonus <= 0 || bonus > 10000000 || bonusSeen[bonus] {
				return fmt.Errorf("invalid item restriction bonuses")
			}
			bonusSeen[bonus] = true
		}
		key := ruleKey(rule.ID, rule.BonusIDs)
		if seen[key] {
			return fmt.Errorf("repeated item restriction record")
		}
		seen[key] = true
	}
	return nil
}

func ValidateSnapshotRules(s *Snapshot) error {
	if err := ValidateItemRules(s.Rules); err != nil {
		return err
	}
	seen := map[string]ItemRule{}
	for _, rule := range s.Rules {
		seen[ruleKey(rule.ID, rule.BonusIDs)] = rule
	}
	if s.Loot != nil {
		for _, pool := range s.Loot.Pools {
			for _, item := range pool.Items {
				if !item.RulesKnown {
					continue
				}
				value, err := item.SimCValue()
				if err != nil {
					return err
				}
				rule := ItemRule{ID: item.ID, InventoryType: item.InventoryType, Unique: item.Unique, LimitCategory: item.LimitCategory, LimitMax: item.LimitMax}
				key := valueRuleKey(value)
				if old, ok := seen[key]; ok && (old.InventoryType != rule.InventoryType || old.Unique != rule.Unique || old.LimitCategory != rule.LimitCategory || old.LimitMax != rule.LimitMax) {
					return fmt.Errorf("conflicting captured restrictions for item %d", item.ID)
				}
				seen[key] = rule
			}
		}
	}
	return nil
}

func snapshotRuleIndex(s *Snapshot) map[string]ItemRule {
	rules := map[string]ItemRule{}
	for _, rule := range s.Rules {
		rules[ruleKey(rule.ID, rule.BonusIDs)] = rule
	}
	if s.Loot != nil {
		for _, pool := range s.Loot.Pools {
			for _, item := range pool.Items {
				if item.RulesKnown {
					if value, err := item.SimCValue(); err == nil {
						rules[valueRuleKey(value)] = ItemRule{ID: item.ID, InventoryType: item.InventoryType, Unique: item.Unique, LimitCategory: item.LimitCategory, LimitMax: item.LimitMax}
					}
				}
			}
		}
	}
	return rules
}

func baseGear(profile *Profile) map[string]Candidate {
	gear := map[string]Candidate{}
	for _, line := range strings.Split(profile.Base, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && slots[key] && key != "shirt" && key != "tabard" && value != "" {
			gear[key] = Candidate{Slot: key, Value: value}
		}
	}
	return gear
}

func dualWields(p *Profile) bool {
	return p.Class == "rogue" || p.Class == "demonhunter" || (p.Class == "warrior" && p.Spec == "fury") || (p.Class == "deathknight" && p.Spec == "frost") || (p.Class == "shaman" && p.Spec == "enhancement") || (p.Class == "monk" && p.Spec == "windwalker")
}

func candidateRule(rules map[string]ItemRule, item Candidate) (ItemRule, bool) {
	if rule, ok := rules[valueRuleKey(item.Value)]; ok {
		return rule, true
	}
	// Only the reviewed ascension recipe can inherit the captured physical
	// item's restrictions. Arbitrary bonus changes may alter limit categories.
	if item.Venomstones == 10 && item.BaseValue != "" {
		planned, err := applyVenomstone(Candidate{Slot: item.Slot, Value: item.BaseValue})
		if err == nil && valueRuleKey(planned.Value) == valueRuleKey(item.Value) {
			rule, ok := rules[valueRuleKey(item.BaseValue)]
			return rule, ok
		}
	}
	return ItemRule{}, false
}

func legalSlot(p *Profile, rule ItemRule, slot string) bool {
	expected := lootInventoryTypes[rule.InventoryType]
	if strings.HasPrefix(expected, "finger") && strings.HasPrefix(slot, "finger") || strings.HasPrefix(expected, "trinket") && strings.HasPrefix(slot, "trinket") {
		return true
	}
	if slot == "off_hand" && rule.InventoryType == "INVTYPE_WEAPON" {
		return dualWields(p)
	}
	if slot == "off_hand" && rule.InventoryType == "INVTYPE_2HWEAPON" {
		return p.Class == "warrior" && p.Spec == "fury"
	}
	return expected == slot
}

func applyGearRules(s *Snapshot, profile *Profile, comparisons []Comparison, strict bool) ([]Comparison, []RejectedComparison, []string, error) {
	rules := snapshotRuleIndex(s)
	base := baseGear(profile)
	if strict {
		for _, item := range base {
			if _, ok := rules[valueRuleKey(item.Value)]; !ok {
				return nil, nil, nil, fmt.Errorf("export a fresh snapshot with item restrictions before comparing complete sets")
			}
		}
	}
	var accepted []Comparison
	var rejected []RejectedComparison
	unknown := false
	for _, comparison := range comparisons {
		if len(comparison.Items) == 0 {
			accepted = append(accepted, comparison)
			continue
		}
		gear := map[string]Candidate{}
		for slot, item := range base {
			gear[slot] = item
		}
		items := append([]Candidate{}, comparison.Items...)
		reason := ""
		mainChanged, offChanged := false, false
		for _, item := range items {
			if item.Slot == "main_hand" {
				mainChanged = true
			}
			if item.Slot == "off_hand" && item.Value != "" {
				offChanged = true
			}
			gear[item.Slot] = item
		}
		if mainChanged {
			if rule, ok := candidateRule(rules, gear["main_hand"]); ok && rule.InventoryType == "INVTYPE_2HWEAPON" && !(profile.Class == "warrior" && profile.Spec == "fury") {
				if offChanged {
					reason = "Two-handed weapon and offhand cannot be equipped together."
				} else if gear["off_hand"].Value != "" {
					gear["off_hand"] = Candidate{Slot: "off_hand"}
					items = append(items, Candidate{Slot: "off_hand"})
				}
			}
		}
		categories, categoryLimits, ids := map[int]int{}, map[int]int{}, map[int]int{}
		uniqueIDs := map[int]bool{}
		for slot, item := range gear {
			if reason != "" || item.Value == "" {
				continue
			}
			rule, ok := candidateRule(rules, item)
			if !ok {
				unknown = true
				if strict {
					reason = "Item restrictions are missing; export again."
				} else if (slot == "main_hand" && mainChanged) || (slot == "off_hand" && offChanged) {
					reason = "Weapon type is missing; export again before comparing it."
				}
				continue
			}
			if !legalSlot(profile, rule, slot) {
				reason = "Item cannot be equipped in this slot."
				continue
			}
			ids[rule.ID]++
			uniqueIDs[rule.ID] = uniqueIDs[rule.ID] || rule.Unique
			if uniqueIDs[rule.ID] && ids[rule.ID] > 1 {
				reason = "Unique item is equipped more than once."
			}
			if rule.LimitCategory > 0 {
				categories[rule.LimitCategory]++
				if old := categoryLimits[rule.LimitCategory]; old == 0 || rule.LimitMax < old {
					categoryLimits[rule.LimitCategory] = rule.LimitMax
				}
			}
		}
		for id, count := range ids {
			if uniqueIDs[id] && count > 1 {
				reason = "Unique item is equipped more than once."
			}
		}
		for category, count := range categories {
			if count > categoryLimits[category] {
				reason = "Equipped items exceed a shared unique or embellishment limit."
			}
		}
		if rule, ok := candidateRule(rules, gear["main_hand"]); ok && rule.InventoryType == "INVTYPE_2HWEAPON" && gear["off_hand"].Value != "" && !(profile.Class == "warrior" && profile.Spec == "fury") {
			reason = "Two-handed weapon and offhand cannot be equipped together."
		}
		if reason != "" {
			rejected = append(rejected, RejectedComparison{comparison.ID, comparison.Label, reason})
		} else {
			comparison.Items = items
			accepted = append(accepted, comparison)
		}
	}
	warnings := []string{}
	if unknown {
		warnings = append(warnings, "Some item restrictions are missing. Individual estimates do not verify every equipment limit; export a fresh snapshot.")
	}
	return accepted, rejected, warnings, nil
}
