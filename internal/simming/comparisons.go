package simming

import (
	"fmt"
	"sort"
	"strings"
)

type Comparison struct {
	ID      string
	Label   string
	Items   []Candidate
	Talents string
}

func (p *Profile) Comparisons(s Scenario, selected []string) ([]Comparison, error) {
	limit := 8
	if s.CompareMode == "loadout" {
		limit = 18
	}
	if len(selected) > limit {
		return nil, fmt.Errorf("choose at most %d items per run", limit)
	}
	seen := map[string]bool{}
	items := []Candidate{}
	for _, id := range selected {
		if seen[id] {
			return nil, fmt.Errorf("repeated candidate")
		}
		seen[id] = true
		found := false
		for _, c := range p.Candidates {
			if c.ID == id {
				if s.CompareMode == "loadout" {
					if slot := s.LoadoutSlots[id]; slot != "" {
						c.Slot = slot
					}
					if level := s.LoadoutLevels[id]; level > 0 {
						c.BaseValue = c.Value
						c.Value = withItemLevel(c.Value, level)
						c.Level = level
						c.Variant = "planned"
					}
				}
				items = append(items, c)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("candidate is not in this snapshot")
		}
	}
	for id := range s.LoadoutSlots {
		if !seen[id] {
			return nil, fmt.Errorf("planned placement does not belong to a selected item")
		}
	}
	for id := range s.LoadoutLevels {
		if !seen[id] {
			return nil, fmt.Errorf("planned item level does not belong to a selected item")
		}
	}
	for id := range s.ItemOptions {
		if !seen[id] {
			return nil, fmt.Errorf("item alternatives do not belong to a selected item")
		}
	}
	out := []Comparison{}
	if s.CompareMode == "topgear" {
		// Enumerate legal slot/Vault combinations. The simulator also validates its
		// item-specific restrictions before any result is accepted.
		for mask := 1; mask < (1 << len(items)); mask++ {
			used := map[string]bool{}
			vault := 0
			set := []Candidate{}
			labels := []string{}
			valid := true
			for i, c := range items {
				if mask&(1<<i) == 0 {
					continue
				}
				if used[c.Slot] && !strings.HasPrefix(c.Slot, "finger") && !strings.HasPrefix(c.Slot, "trinket") {
					valid = false
					break
				}
				used[c.Slot] = true
				if c.Source == "vault" {
					vault++
				}
				if vault > 1 {
					valid = false
					break
				}
				set = append(set, c)
				labels = append(labels, c.Label)
			}
			if valid {
				out = append(out, Comparison{ID: fmt.Sprintf("set%d", mask), Label: strings.Join(labels, " + "), Items: set})
			}
		}
	} else if s.CompareMode == "loadout" {
		vault := 0
		for _, item := range items {
			if item.Source == "vault" {
				vault++
			}
		}
		if vault > 1 {
			return nil, fmt.Errorf("a planned loadout can include one Vault choice")
		}
		out = append(out, Comparison{ID: "loadout", Label: "Planned loadout", Items: append([]Candidate{}, items...)})
	} else {
		for _, c := range items {
			out = append(out, Comparison{ID: c.ID, Label: c.Label, Items: []Candidate{c}})
		}
	}
	// Test either position for rings and trinkets; avoid proposing the same
	// unique item twice. Item-specific weapon/embellishment rules remain enforced
	// by the registered simulator rather than guessed from item names.
	expanded := []Comparison{}
	equipped := map[string]string{}
	for _, line := range strings.Split(p.Base, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && slots[key] {
			equipped[key] = value
		}
	}
	for _, comparison := range out {
		variants := [][]Candidate{{}}
		for _, item := range comparison.Items {
			options := []string{item.Slot}
			if s.CompareMode != "loadout" && (strings.HasPrefix(item.Slot, "finger") || strings.HasPrefix(item.Slot, "trinket")) {
				other := item.Slot[:len(item.Slot)-1] + "1"
				if other == item.Slot {
					other = item.Slot[:len(item.Slot)-1] + "2"
				}
				options = append(options, other)
			}
			next := [][]Candidate{}
			for _, variant := range variants {
				for _, slot := range options {
					free := true
					for _, existing := range variant {
						if existing.Slot == slot {
							free = false
						}
					}
					if free {
						copy := append([]Candidate{}, variant...)
						i := item
						i.Slot = slot
						next = append(next, append(copy, i))
					}
				}
			}
			variants = next
		}
		seenPlacements := map[string]bool{}
		for _, variant := range variants {
			gear := map[string]string{}
			for slot, value := range equipped {
				gear[slot] = value
			}
			for _, item := range variant {
				gear[item.Slot] = item.Value
			}
			valid := true
			for _, family := range []string{"finger", "trinket"} {
				a, b := itemID(gear[family+"1"]), itemID(gear[family+"2"])
				baseA, baseB := itemID(equipped[family+"1"]), itemID(equipped[family+"2"])
				if a != "" && a == b && !(baseA == a && baseB == b) {
					valid = false
				}
			}
			for _, item := range variant {
				if item.Source != "equipped" || item.ID == "equipped_"+item.Slot {
					continue
				}
				origin := strings.TrimPrefix(item.ID, "equipped_")
				replaced := false
				for _, other := range variant {
					if other.Slot == origin && other.ID != item.ID {
						replaced = true
					}
				}
				if !replaced {
					valid = false
				}
			}
			if !valid {
				continue
			}
			keyParts := []string{}
			for _, item := range variant {
				keyParts = append(keyParts, item.Slot+"="+item.Value)
			}
			sort.Strings(keyParts)
			key := strings.Join(keyParts, "|")
			if seenPlacements[key] {
				continue
			}
			seenPlacements[key] = true
			c := comparison
			c.Items = variant
			if len(seenPlacements) > 1 {
				c.ID += fmt.Sprintf("v%d", len(seenPlacements))
			}
			expanded = append(expanded, c)
		}
	}
	out = expanded
	if len(selected) > 0 && len(out) == 0 {
		return nil, fmt.Errorf("no legal item placement in the selected set")
	}
	// An explicit loadout applies its one selected talent build to the complete
	// planned set. Ordinary talent comparisons remain separate from gear.
	if s.CompareMode == "loadout" {
		if len(s.Talents) > 1 {
			return nil, fmt.Errorf("choose one talent build for a planned loadout")
		}
		if len(s.Talents) == 1 {
			for i := range out {
				out[i].Talents = s.Talents[0]
			}
		}
	} else {
		for i, code := range s.Talents {
			out = append(out, Comparison{ID: fmt.Sprintf("talent%d", i+1), Label: fmt.Sprintf("Talent build %d", i+1), Talents: code})
		}
	}
	if len(out) > 256 {
		return nil, fmt.Errorf("comparison exceeds 256 sets; select fewer items")
	}
	return expandItemOptions(s, out)
}
func comparisonLines(c Comparison) []string {
	lines := []string{}
	for _, item := range c.Items {
		lines = append(lines, item.Slot+"="+simcItemValue(item.Value))
	}
	if c.Talents != "" {
		lines = append(lines, "talents="+c.Talents)
	}
	return lines
}
func sortComparisons(result *Result) {
	sort.SliceStable(result.Comparisons, func(i, j int) bool { return result.Comparisons[i].Mean > result.Comparisons[j].Mean })
}

func itemID(value string) string {
	for _, field := range strings.Split(value, ",") {
		key, val, ok := strings.Cut(field, "=")
		if ok && key == "id" {
			return val
		}
	}
	return ""
}

// SimC reserves the first comma-separated token for the optional item name.
// A numeric-first value must retain an empty name (",id=..."); without that
// comma SimC can silently model an empty slot instead of the requested item.
func simcItemValue(value string) string {
	if value != "" && strings.Contains(strings.SplitN(value, ",", 2)[0], "=") {
		return "," + value
	}
	return value
}
func simcBase(base string) string {
	lines := strings.Split(base, "\n")
	for i, line := range lines {
		if key, value, ok := strings.Cut(line, "="); ok && slots[key] {
			lines[i] = key + "=" + simcItemValue(value)
		}
	}
	return strings.Join(lines, "\n")
}
