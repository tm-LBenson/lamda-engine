package simming

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func decorateMetric(metric *Metric, c Comparison) {
	metric.Label = c.Label
	metric.Items = nil
	identities := map[string]bool{}
	for _, item := range c.Items {
		id, _ := strconv.Atoi(itemID(item.Value))
		metric.Items = append(metric.Items, ResultItem{ID: id, CandidateID: item.ID, Level: item.Level, Slot: item.Slot, Label: item.Label, Key: item.Value, BaseKey: item.BaseValue, Source: item.Source, Variant: item.Variant, Venomstones: item.Venomstones})
		if item.ID != "" && (item.Source == "equipped" || item.Source == "owned" || item.Source == "vault") {
			identities[item.ID] = true
			metric.CandidateID, metric.Source, metric.Slot, metric.Variant = item.ID, item.Source, item.Slot, item.Variant
			metric.Level, metric.MaxLevel = item.Level, item.MaxLevel
		}
	}
	if len(identities) > 1 || (len(c.Items) > 0 && c.Talents != "") {
		metric.CandidateID, metric.Source, metric.Slot, metric.Variant = c.ID, "set", "", "combined"
		metric.Level, metric.MaxLevel = 0, 0
	}
	if c.Talents != "" && len(c.Items) == 0 {
		metric.CandidateID, metric.Source, metric.Variant = c.ID, "talent", "talent"
	}
}

func comparisonProfile(p *Profile, c Comparison) Profile {
	copy := *p
	updates := map[string]string{}
	for _, item := range c.Items {
		updates[item.Slot] = item.Value
	}
	if c.Talents != "" {
		updates["talents"] = c.Talents
	}
	lines := []string{}
	for _, line := range strings.Split(p.Base, "\n") {
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if value, present := updates[key]; present {
			if value != "" {
				lines = append(lines, key+"="+value)
			}
			delete(updates, key)
		} else {
			lines = append(lines, line)
		}
	}
	// Slot order is fixed so repeated checks have reproducible input bytes.
	for _, slot := range []string{"head", "neck", "shoulder", "back", "chest", "wrist", "hands", "waist", "legs", "feet", "finger1", "finger2", "trinket1", "trinket2", "main_hand", "off_hand", "talents"} {
		if value := updates[slot]; value != "" {
			lines = append(lines, slot+"="+value)
		}
	}
	copy.Base = strings.Join(lines, "\n") + "\n"
	return copy
}

func normalizedIDs(value string) string {
	fields := strings.Split(value, "/")
	for len(fields) > 0 && (fields[len(fields)-1] == "0" || fields[len(fields)-1] == "") {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, "/")
}

// SimC profileset gear output omits same-ID changes, gems and enchants. For
// explicit alternatives run complete actors and verify their resolved output;
// a positive DPS value never substitutes for checking the requested variant.
func verifyPlannedItems(dir string, c Comparison) error {
	data, err := os.ReadFile(filepath.Join(dir, "verified-gear.simc"))
	if err != nil {
		return err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		slot, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		if slot == "shoulders" {
			slot = "shoulder"
		}
		if slot == "wrists" {
			slot = "wrist"
		}
		if slots[slot] {
			_, fields, ok := strings.Cut(value, ",")
			if ok {
				values[slot] = fields
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		return err
	}
	var report struct {
		Sim struct {
			Players []struct {
				Gear map[string]struct {
					Level int `json:"ilevel"`
				} `json:"gear"`
			} `json:"players"`
		} `json:"sim"`
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		return err
	}
	for _, item := range c.Items {
		if item.Variant != "planned" {
			continue
		}
		value := values[item.Slot]
		if itemID(value) != itemID(item.Value) {
			return &workerInputError{"worker did not resolve the planned item"}
		}
		if item.Venomstones > 0 {
			actual := map[int]bool{}
			for _, id := range itemBonuses(value) {
				actual[id] = true
			}
			for _, id := range itemBonuses(item.Value) {
				if !actual[id] {
					return &workerInputError{"worker did not apply the requested Venomstone item bonuses"}
				}
			}
		}
		for _, field := range []string{"gem_id", "enchant_id"} {
			if wanted := itemField(item.Value, field); wanted != "" && normalizedIDs(wanted) != normalizedIDs(itemField(value, field)) {
				return &workerInputError{fmt.Sprintf("worker did not apply planned %s for %s", field, item.Slot)}
			}
		}
		if level := itemField(item.Value, "ilevel"); level != "" {
			if len(report.Sim.Players) == 0 || strconv.Itoa(report.Sim.Players[0].Gear[reportedSlot(item.Slot)].Level) != level {
				return &workerInputError{"worker did not apply the planned item level"}
			}
		}
	}
	return nil
}

func (p *Provider) runPlannedComparisons(ctx context.Context, s *Snapshot, profile *Profile, scenario Scenario, comparisons []Comparison, dir string) (*Result, error) {
	input, err := lootInput(profile, scenario, nil)
	if err != nil {
		return nil, err
	}
	result, err := p.runPrepared(ctx, s, scenario, input, nil, filepath.Join(dir, "baseline"))
	if err != nil {
		return nil, err
	}
	for i, c := range comparisons {
		if err := ctx.Err(); err != nil {
			result.Incomplete = true
			break
		}
		alternate := comparisonProfile(profile, c)
		capture := *s
		capture.Profile = alternate.Base
		input, err := lootInput(&alternate, scenario, nil)
		if err != nil {
			return nil, err
		}
		trialDir := filepath.Join(dir, fmt.Sprintf("variant-%03d", i))
		trial, err := p.runPrepared(ctx, &capture, scenario, input, nil, trialDir)
		if err == nil {
			err = verifyPlannedItems(trialDir, c)
		}
		if err != nil {
			if ctx.Err() != nil {
				result.Incomplete = true
				break
			}
			if !comparisonRetryable(err) {
				return nil, err
			}
			result.RejectedComparisons = append(result.RejectedComparisons, RejectedComparison{c.ID, c.Label, err.Error()})
			result.processedComparisons++
			continue
		}
		metric := trial.Baseline
		metric.Name = c.ID
		decorateMetric(&metric, c)
		result.Comparisons = append(result.Comparisons, metric)
		result.processedComparisons++
	}
	if result.Incomplete {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Item-alternative calculation stopped after %d of %d variants; completed results are retained.", result.processedComparisons, len(comparisons)))
	}
	result.Warnings = append(result.Warnings, "Explicit item alternatives were checked as complete actors, including returned item levels, gems and enchants. Planned changes do not establish costs or available currencies.")
	sortComparisons(result)
	return result, nil
}
