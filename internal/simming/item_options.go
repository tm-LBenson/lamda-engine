package simming

import (
	"fmt"
	"strconv"
	"strings"
)

// Adjustments are alternatives for one captured physical item, never extra
// copies of it. IDs and instance modifiers remain anchored to the capture.
type ItemAdjustment struct {
	Venomstone bool   `json:"venomstone,omitempty"`
	Label      string `json:"label,omitempty"`
	Level      int    `json:"level,omitempty"`
	EnchantID  *int   `json:"enchantID,omitempty"`
	GemIDs     []int  `json:"gemIDs,omitempty"`
}

func (a ItemAdjustment) Validate() error {
	if len(a.Label) > 80 || strings.ContainsAny(a.Label, "\r\n\x00") || a.Level < 0 || a.Level > 2000 || len(a.GemIDs) > 4 {
		return fmt.Errorf("invalid item alternative")
	}
	if a.Level == 0 && a.EnchantID == nil && len(a.GemIDs) == 0 && !a.Venomstone {
		return fmt.Errorf("item alternative has no change")
	}
	if a.EnchantID != nil && (*a.EnchantID < 0 || *a.EnchantID > 10000000) {
		return fmt.Errorf("invalid enchant ID")
	}
	for _, id := range a.GemIDs {
		if id < 0 || id > 10000000 {
			return fmt.Errorf("invalid gem ID")
		}
	}
	return nil
}

func itemField(value, key string) string {
	for _, part := range strings.Split(value, ",") {
		k, v, ok := strings.Cut(part, "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}

func setItemField(value, key, val string) string {
	parts := strings.Split(value, ",")
	for i, part := range parts {
		k, _, ok := strings.Cut(part, "=")
		if ok && k == key {
			parts[i] = key + "=" + val
			return strings.Join(parts, ",")
		}
	}
	return value + "," + key + "=" + val
}

func adjustedItem(item Candidate, a ItemAdjustment) (Candidate, error) {
	if item.BaseValue == "" {
		item.BaseValue = item.Value
	}
	if a.Level > 0 {
		item.Value = withItemLevel(item.Value, a.Level)
		item.Level = a.Level
	}
	if a.EnchantID != nil {
		item.Value = setItemField(item.Value, "enchant_id", strconv.Itoa(*a.EnchantID))
	}
	if len(a.GemIDs) > 0 {
		values := make([]string, len(a.GemIDs))
		for i, id := range a.GemIDs {
			values[i] = strconv.Itoa(id)
		}
		item.Value = setItemField(item.Value, "gem_id", strings.Join(values, "/"))
	}
	if a.Label != "" {
		item.Label += " · " + a.Label
	}
	if a.Venomstone {
		var err error
		item, err = applyVenomstone(item)
		if err != nil {
			return Candidate{}, err
		}
		if a.Label == "" {
			item.Label += " · Max rank + Venomstones"
		}
	}
	item.Variant = "planned"
	return item, nil
}

// Only selected equipped items are exposed as candidates. Ordinary bag lists
// and their positional IDs stay unchanged for existing addon clients.
func prepareComparisonProfile(s *Snapshot, p *Profile, scenario Scenario, selected []string) (*Profile, error) {
	copy := *p
	copy.Candidates = append([]Candidate{}, p.Candidates...)
	equipped := displayGear(p)
	chosen := map[string]bool{}
	for _, id := range selected {
		chosen[id] = true
		if strings.HasPrefix(id, "equipped_") {
			slot := strings.TrimPrefix(id, "equipped_")
			item, ok := equipped[slot]
			if !ok {
				return nil, fmt.Errorf("selected equipped item is not in the capture")
			}
			copy.Candidates = append(copy.Candidates, item)
		}
	}
	for id, options := range scenario.ItemOptions {
		if !chosen[id] {
			return nil, fmt.Errorf("item alternatives do not belong to a selected item")
		}
		var item *Candidate
		for i := range copy.Candidates {
			if copy.Candidates[i].ID == id {
				item = &copy.Candidates[i]
				break
			}
		}
		if item == nil {
			return nil, fmt.Errorf("item alternative is not in the capture")
		}
		for _, a := range options {
			if err := a.Validate(); err != nil {
				return nil, err
			}
			if a.Venomstone {
				if s.Client != "retail" || s.Interface != 120105 || s.Character.Role != "DAMAGER" {
					return nil, fmt.Errorf("Venomstone comparisons require the supported Retail 12.1.5 damage model")
				}
				if _, err := venomstoneRecipeFor(*item); err != nil {
					return nil, err
				}
			}
			if a.Level > 0 {
				legal := false
				for _, u := range s.Upgrades {
					if u.Source != item.Source || u.Slot != item.Slot || u.Value != item.Value {
						continue
					}
					for _, level := range u.RankLevels {
						if level == a.Level {
							legal = true
						}
					}
				}
				if !legal {
					return nil, fmt.Errorf("planned level requires a captured upgrade rank for %s", id)
				}
			}
			if len(a.GemIDs) > 0 {
				gems := itemField(item.Value, "gem_id")
				if gems == "" || len(strings.Split(gems, "/")) != len(a.GemIDs) {
					return nil, fmt.Errorf("gem alternatives must use the captured sockets; new sockets are not inferred")
				}
			}
		}
	}
	return &copy, nil
}

func expandItemOptions(s Scenario, comparisons []Comparison) ([]Comparison, error) {
	if len(s.ItemOptions) == 0 {
		return comparisons, nil
	}
	out := []Comparison{}
	for _, c := range comparisons {
		variants := [][]Candidate{{}}
		for _, item := range c.Items {
			choices := []Candidate{item}
			seen := map[string]bool{item.Value: true}
			for _, option := range s.ItemOptions[item.ID] {
				next, err := adjustedItem(item, option)
				if err != nil {
					return nil, err
				}
				if !seen[next.Value] {
					choices = append(choices, next)
					seen[next.Value] = true
				}
			}
			next := [][]Candidate{}
			for _, v := range variants {
				for _, choice := range choices {
					if len(next)+len(out) >= 256 {
						return nil, fmt.Errorf("comparison exceeds 256 sets; choose fewer item alternatives")
					}
					next = append(next, append(append([]Candidate{}, v...), choice))
				}
			}
			variants = next
		}
		for i, items := range variants {
			entry := c
			entry.Items = items
			if i > 0 {
				entry.ID += fmt.Sprintf("_o%d", i)
				entry.Label += " · planned alternatives"
			}
			stones := 0
			for _, item := range items {
				stones += item.Venomstones
			}
			if stones > 0 {
				entry.Label = c.Label + fmt.Sprintf(" · Venomstones (%d)", stones)
			}
			out = append(out, entry)
			if len(out) > 256 {
				return nil, fmt.Errorf("comparison exceeds 256 sets; choose fewer item alternatives")
			}
		}
	}
	return out, nil
}
