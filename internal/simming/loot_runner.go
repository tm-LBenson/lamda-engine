package simming

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type lootTrial struct {
	view       string
	comparison Comparison
	pool       int
	item       LootItem
}

// runLoot uses exact journal links and levels from this snapshot. A failed item
// invalidates its whole source for probability calculations, but successful
// individual comparisons remain useful. Every worker input and log is retained.
func (p *Provider) runLoot(ctx context.Context, s *Snapshot, scenario Scenario, catalog *LootCatalog, dir string) (*Result, error) {
	if err := ValidateLoot(catalog, s); err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, fmt.Errorf("capture loot in Retail before finding DPS upgrades")
	}
	if len(scenario.Talents) > 0 {
		return nil, fmt.Errorf("compare talents separately before finding upgrades")
	}
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	scenario.Task, scenario.CompareMode = "", "individual"
	profile, err := ParseProfile(s.Profile)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := applyGearRules(s, profile, nil, true); err != nil {
		return nil, err
	}
	baseInput, err := profile.Input(scenario, nil)
	if err != nil {
		return nil, err
	}
	result, err := p.runPrepared(ctx, s, scenario, baseInput, nil, filepath.Join(dir, "baseline"))
	if err != nil {
		return nil, err
	}
	if result.Model == "" {
		result.Model = "SimulationCraft"
	}
	result.Model += " · captured journal loot"
	result.Season = catalog.Season
	if result.ModelInputs == nil {
		result.ModelInputs = map[string]string{}
	}
	for key, value := range map[string]string{"loot": "Captured class/spec Encounter Journal filter", "levels": "Exact captured drop levels and client upgrade caps", "enchants": "Captured items; new drops are unenchanted", "targetSearch": "Two greedy passes over the 24 highest individual gains"} {
		result.ModelInputs[key] = value
	}
	result.Warnings = append(result.Warnings, "Loot uses the captured difficulties and key levels. Only client-reported upgrade caps are included; crafted items and bonus-roll outcomes are not synthesized.", "Farm probabilities require your assumptions; rare items do not have published equal drop odds.")
	failedPools := map[int]bool{}
	var trials []lootTrial
	for pi, pool := range catalog.Pools {
		if !pool.Complete {
			failedPools[pi] = true
		}
		for ii, item := range pool.Items {
			views := []string{"drop"}
			if item.MaxLevel > 0 {
				views = append(views, "max")
			}
			for _, view := range views {
				variant := item
				if view == "max" {
					variant.Level = item.MaxLevel
				}
				for _, comparison := range lootComparisons(profile, variant, fmt.Sprintf("loot%d_%d_%s", pi, ii, view)) {
					trials = append(trials, lootTrial{comparison: comparison, pool: pi, item: variant, view: view})
				}
			}
		}
	}
	if len(trials) == 0 {
		return nil, fmt.Errorf("captured loot has no equippable comparison items")
	}
	metrics, failures, err := p.runLootTrials(ctx, s, profile, scenario, trials, filepath.Join(dir, "items"))
	if err != nil {
		return nil, err
	}
	best := map[string]Metric{}
	bestTrial := map[string]lootTrial{}
	for _, trial := range trials {
		key := fmt.Sprintf("%d:%d:%s", trial.pool, trial.item.ID, trial.view)
		if metric, ok := metrics[trial.comparison.ID]; ok {
			if old, ok := best[key]; !ok || metric.Mean > old.Mean {
				best[key], bestTrial[key] = metric, trial
			}
		}
	}
	// One rejected placement does not make an item unmodeled if another valid
	// placement exists (for example a unique ring already in the opposite slot).
	failedViews := map[string]bool{}
	for _, trial := range trials {
		key := fmt.Sprintf("%d:%d:%s", trial.pool, trial.item.ID, trial.view)
		if _, ok := best[key]; !ok {
			failedViews[fmt.Sprintf("%d:%s", trial.pool, trial.view)] = true
		}
	}
	owned := map[int]bool{}
	for _, line := range strings.Split(profile.Base, "\n") {
		key, val, _ := strings.Cut(line, "=")
		if slots[key] {
			id, _ := strconv.Atoi(itemID(val))
			owned[id] = true
		}
	}
	for _, candidate := range profile.Candidates {
		if candidate.Source == "owned" {
			id, _ := strconv.Atoi(itemID(candidate.Value))
			owned[id] = true
		}
	}
	for key, metric := range best {
		trial := bestTrial[key]
		pool := catalog.Pools[trial.pool]
		capturedLevel := trial.item.Level
		for _, item := range pool.Items {
			if item.ID == trial.item.ID && item.Link == trial.item.Link {
				capturedLevel = item.Level
				break
			}
		}
		result.Upgrades = append(result.Upgrades, Upgrade{ID: trial.item.ID, Level: trial.item.Level, Slot: trial.comparison.Items[0].Slot, Label: trial.item.Label,
			Link: trial.item.Link, Key: trial.comparison.Items[0].Value, CapturedLevel: capturedLevel, Items: append([]ResultItem{}, metric.Items...),
			Source: pool.Name, SourceID: pool.SourceID(), Type: trial.view, Kind: pool.Type, Mean: metric.Mean, Gain: 100 * (metric.Mean/result.Baseline.Mean - 1), Owned: owned[trial.item.ID], DifficultyID: pool.DifficultyID, KeyLevel: pool.KeyLevel, Rare: trial.item.Rare, LootVerified: true, PoolComplete: !failedPools[trial.pool] && !failedViews[fmt.Sprintf("%d:%s", trial.pool, trial.view)], PoolSize: len(pool.Items), LootSourceID: pool.SourceID()})
	}
	sort.Slice(result.Upgrades, func(i, j int) bool {
		if result.Upgrades[i].Mean == result.Upgrades[j].Mean {
			return result.Upgrades[i].SourceID+result.Upgrades[i].Label < result.Upgrades[j].SourceID+result.Upgrades[j].Label
		}
		return result.Upgrades[i].Mean > result.Upgrades[j].Mean
	})
	for pi := range failedPools {
		result.IncompleteSources = append(result.IncompleteSources, catalog.Pools[pi].SourceID())
	}
	sort.Strings(result.IncompleteSources)
	if len(failures) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d item placements were excluded by equipment rules or SimulationCraft. Incomplete sources are excluded from farm rankings; see item logs.", len(failures)))
	}
	if len(best) == 0 {
		return nil, fmt.Errorf("SimulationCraft rejected every captured item; see the item logs")
	}
	if err := p.lootTarget(ctx, s, profile, scenario, best, bestTrial, result, filepath.Join(dir, "target")); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result.Warnings = append(result.Warnings, "Target-set search stopped: "+err.Error())
	}
	_ = os.WriteFile(filepath.Join(dir, "worker.log"), []byte(fmt.Sprintf("Journal loot run: %d item placements, %d modeled items, %d incomplete sources. Individual inputs and logs are in baseline/, items/ and target/.\n", len(trials), len(best), len(failedPools))), 0600)
	return result, nil
}

func lootComparisons(profile *Profile, item LootItem, id string) []Comparison {
	value, err := item.SimCValue()
	if err != nil {
		return nil // ValidateLoot has already reported malformed links.
	}
	options := []string{item.Slot}
	if item.Slot == "finger1" || item.Slot == "trinket1" {
		options = append(options, strings.TrimSuffix(item.Slot, "1")+"2")
	}
	// A one-handed weapon may be a main-hand or off-hand upgrade for dual
	// wielders. Captured equipment rules and worker initialization still apply.
	if item.InventoryType == "INVTYPE_WEAPON" && ((profile.Class == "rogue") || (profile.Class == "warrior" && profile.Spec == "fury") || (profile.Class == "shaman" && profile.Spec == "enhancement") || (profile.Class == "deathknight" && profile.Spec == "frost") || profile.Class == "demonhunter" || (profile.Class == "monk" && profile.Spec == "windwalker")) {
		options = append(options, "off_hand")
	}
	if item.InventoryType == "INVTYPE_2HWEAPON" && profile.Class == "warrior" && profile.Spec == "fury" {
		options = append(options, "off_hand")
	}
	var out []Comparison
	for i, slot := range options {
		items := []Candidate{{ID: id, Label: item.Label, Source: "loot", Slot: slot, Value: value}}
		if item.InventoryType == "INVTYPE_2HWEAPON" && !(profile.Class == "warrior" && profile.Spec == "fury") {
			items = append(items, Candidate{Slot: "off_hand", Value: ""})
		}
		out = append(out, Comparison{ID: fmt.Sprintf("%s_%d", id, i), Label: item.Label, Items: items})
	}
	return out
}

func lootInput(profile *Profile, scenario Scenario, comparisons []Comparison) (string, error) {
	if err := scenario.Validate(); err != nil {
		return "", err
	}
	inputScenario := scenario
	inputScenario.Talents = nil
	inputScenario.CompareMode = ""
	// Comparisons are already materialized below. Clear their planning metadata
	// with the mode so the baseline input remains valid for exact loadouts.
	inputScenario.LoadoutSlots = nil
	inputScenario.LoadoutLevels = nil
	inputScenario.ItemOptions = nil
	inputScenario.Encounters = nil
	input, err := profile.Input(inputScenario, nil)
	if err != nil {
		return "", err
	}
	for _, comparison := range comparisons {
		for i, line := range comparisonLines(comparison) {
			op := "="
			if i > 0 {
				op = "+="
			}
			input += fmt.Sprintf("profileset.%s%s%s\n", comparison.ID, op, line)
		}
	}
	return input, nil
}

func (p *Provider) runLootTrials(ctx context.Context, s *Snapshot, profile *Profile, scenario Scenario, trials []lootTrial, dir string) (map[string]Metric, map[string]error, error) {
	metrics, failures := map[string]Metric{}, map[string]error{}
	for offset := 0; offset < len(trials); offset += 24 {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		batch := trials[offset:min(offset+24, len(trials))]
		comparisons := make([]Comparison, len(batch))
		for i, trial := range batch {
			comparisons[i] = trial.comparison
		}
		accepted, rejected, _, ruleErr := applyGearRules(s, profile, comparisons, true)
		if ruleErr != nil {
			return nil, nil, ruleErr
		}
		for _, rejected := range rejected {
			failures[rejected.ID] = &workerInputError{rejected.Reason}
		}
		if len(accepted) == 0 {
			continue
		}
		comparisons = accepted
		keep := map[string]bool{}
		for _, accepted := range accepted {
			keep[accepted.ID] = true
		}
		filtered := batch[:0:0]
		for _, trial := range batch {
			if keep[trial.comparison.ID] {
				filtered = append(filtered, trial)
			}
		}
		batch = filtered
		input, err := lootInput(profile, scenario, comparisons)
		if err != nil {
			return nil, nil, err
		}
		r, err := p.runPrepared(ctx, s, scenario, input, comparisons, filepath.Join(dir, fmt.Sprintf("batch-%03d", offset/24)))
		if err == nil {
			for _, metric := range r.Comparisons {
				metrics[metric.Name] = metric
			}
			continue
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if !comparisonRetryable(err) {
			return nil, nil, err
		}
		// Isolate invalid weapon/unique-item placements without discarding other
		// sources. Retrying keeps the original failed batch for investigation.
		for _, trial := range batch {
			input, err := lootInput(profile, scenario, []Comparison{trial.comparison})
			if err != nil {
				return nil, nil, err
			}
			r, err := p.runPrepared(ctx, s, scenario, input, []Comparison{trial.comparison}, filepath.Join(dir, "retry-"+trial.comparison.ID))
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			if err != nil {
				if !comparisonRetryable(err) {
					return nil, nil, err
				}
				failures[trial.comparison.ID] = err
			} else {
				metrics[trial.comparison.ID] = r.Comparisons[0]
			}
		}
	}
	return metrics, failures, nil
}

func (p *Provider) lootTarget(ctx context.Context, s *Snapshot, profile *Profile, scenario Scenario, best map[string]Metric, bestTrial map[string]lootTrial, result *Result, dir string) error {
	keys := make([]string, 0, len(best))
	for key := range best {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if best[keys[i]].Mean == best[keys[j]].Mean {
			return keys[i] < keys[j]
		}
		return best[keys[i]].Mean > best[keys[j]].Mean
	})
	if len(keys) > 24 {
		keys = keys[:24]
	}
	gear := map[string]Candidate{}
	for _, line := range strings.Split(profile.Base, "\n") {
		slot, value, _ := strings.Cut(line, "=")
		if slots[slot] {
			gear[slot] = Candidate{Slot: slot, Value: value}
		}
	}
	current := result.Baseline
	for pass := 0; pass < 2; pass++ {
		improved := false
		for ki, key := range keys {
			trial := bestTrial[key]
			// Compare each candidate on top of the best complete set accepted so
			// far. Failed combinations (unique/weapon rules) are simply skipped.
			sets := lootComparisons(profile, trial.item, fmt.Sprintf("target%d_%d", pass, ki))
			for i := range sets {
				set := map[string]Candidate{}
				for slot, item := range gear {
					set[slot] = item
				}
				for _, item := range sets[i].Items {
					set[item.Slot] = item
				}
				sets[i].Items = nil
				for _, item := range set {
					sets[i].Items = append(sets[i].Items, item)
				}
				sort.Slice(sets[i].Items, func(a, b int) bool { return sets[i].Items[a].Slot < sets[i].Items[b].Slot })
			}
			var attempts []lootTrial
			for _, set := range sets {
				attempts = append(attempts, lootTrial{comparison: set})
			}
			metrics, _, err := p.runLootTrials(ctx, s, profile, scenario, attempts, filepath.Join(dir, fmt.Sprintf("pass-%d-item-%d", pass, ki)))
			if err != nil {
				return err
			}
			result.SearchTests += len(sets)
			for _, set := range sets {
				metric, ok := metrics[set.ID]
				if ok && metric.Mean > current.Mean {
					current, improved = metric, true
					for _, item := range set.Items {
						gear[item.Slot] = item
					}
				}
			}
		}
		if !improved {
			break
		}
	}
	current.Label = "Best tested target set"
	for _, item := range gear {
		if item.Value == "" {
			continue
		}
		id, _ := strconv.Atoi(itemID(item.Value))
		level := 0
		for _, field := range strings.Split(item.Value, ",") {
			if key, val, _ := strings.Cut(field, "="); key == "ilevel" {
				level, _ = strconv.Atoi(val)
			}
		}
		label := item.Label
		if label == "" {
			label = "Equipped item"
		}
		current.Items = append(current.Items, ResultItem{ID: id, Level: level, Slot: item.Slot, Label: label, Key: item.Value})
	}
	sort.Slice(current.Items, func(i, j int) bool { return current.Items[i].Slot < current.Items[j].Slot })
	result.TargetSet = &current
	result.Warnings = append(result.Warnings, "Target set is a bounded search, not a global BiS result.")
	return nil
}
