package simming

import (
	"fmt"
	"strconv"
	"strings"
)

// Reviewed 2026-10-29 against Blizzard's 12.1.5 eligibility rules and the
// pinned 70575 ItemBonus data. A Venomstone plan includes normal ranks first;
// it never adds item levels to an arbitrary item or reapplies an ascension.
type venomstoneRecipe struct{ From, To, Level int }

func itemBonuses(value string) []int {
	var ids []int
	for _, raw := range strings.Split(itemField(value, "bonus_id"), "/") {
		id, _ := strconv.Atoi(raw)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func venomstoneRecipeFor(item Candidate) (venomstoneRecipe, error) {
	if item.Slot != "neck" && item.Slot != "trinket1" && item.Slot != "trinket2" && item.Slot != "main_hand" && item.Slot != "off_hand" {
		return venomstoneRecipe{}, fmt.Errorf("Venomstones apply to eligible weapons, trinkets and necklaces")
	}
	bonuses := itemBonuses(item.Value)
	has := map[int]bool{}
	track, count := 0, 0
	for _, id := range bonuses {
		has[id] = true
		if id == 12848 || id == 12856 || id == 13852 {
			return venomstoneRecipe{}, fmt.Errorf("already Venomforged")
		}
		if id >= 12841 && id <= 12846 || id >= 12849 && id <= 12854 {
			track = id
			count++
		}
	}
	if has[13751] {
		quality := itemField(item.Value, "crafting_quality")
		if count != 0 || (quality != "5" && !(quality == "" && has[9627])) {
			return venomstoneRecipe{}, fmt.Errorf("maximum-quality Tidal crafted gear is required")
		}
		if has[13835] {
			return venomstoneRecipe{}, fmt.Errorf("Hero-crafted Venomstone scaling still needs live-client verification")
		}
		if has[13836] {
			return venomstoneRecipe{To: 13852, Level: 337}, nil
		}
		return venomstoneRecipe{}, fmt.Errorf("a verified maximum Tidal crafting cap is required")
	}
	if count != 1 {
		return venomstoneRecipe{}, fmt.Errorf("a Season 2 Hero or Myth upgrade track is required; elevated items cannot be upgraded")
	}
	if track <= 12846 {
		return venomstoneRecipe{From: track, To: 12848, Level: 328}, nil
	}
	return venomstoneRecipe{From: track, To: 12856, Level: 341}, nil
}

func applyVenomstone(item Candidate) (Candidate, error) {
	recipe, err := venomstoneRecipeFor(item)
	if err != nil {
		return Candidate{}, err
	}
	ids := itemBonuses(item.Value)
	found := false
	for i, id := range ids {
		if id == recipe.From {
			ids[i] = recipe.To
			found = true
		}
	}
	if !found {
		ids = append(ids, recipe.To)
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	item.Value = setItemField(item.Value, "bonus_id", strings.Join(parts, "/"))
	item.Value = withItemLevel(item.Value, recipe.Level)
	item.Level = recipe.Level
	item.Venomstones = 10
	return item, nil
}
