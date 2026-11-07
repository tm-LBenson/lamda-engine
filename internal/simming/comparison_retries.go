package simming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type workerInputError struct{ reason string }

func (e *workerInputError) Error() string { return e.reason }

func comparisonRetryable(err error) bool {
	var inputError *workerInputError
	return errors.As(err, &inputError)
}

func (p *Provider) runComparisons(ctx context.Context, s *Snapshot, profile *Profile, scenario Scenario, comparisons []Comparison, dir string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	if len(scenario.ItemOptions) > 0 {
		return p.runPlannedComparisons(ctx, s, profile, scenario, comparisons, dir)
	}
	input, err := lootInput(profile, scenario, comparisons)
	if err != nil {
		return nil, err
	}
	result, err := p.runPrepared(ctx, s, scenario, input, comparisons, dir)
	if err == nil || len(comparisons) == 0 || !comparisonRetryable(err) {
		return result, err
	}
	_ = os.WriteFile(filepath.Join(dir, "retry-reason.txt"), []byte(err.Error()+"\n"), 0600)
	// Confirm the actual character baseline independently. A broken baseline,
	// damaged worker, timeout or runtime failure never becomes a partial result.
	input, err = lootInput(profile, scenario, nil)
	if err != nil {
		return nil, err
	}
	baseline, err := p.runPrepared(ctx, s, scenario, input, nil, filepath.Join(dir, "retry-baseline"))
	if err != nil {
		return nil, err
	}
	var retry func([]Comparison, string) error
	retry = func(group []Comparison, name string) error {
		if len(group) == 0 {
			return nil
		}
		input, inputErr := lootInput(profile, scenario, group)
		if inputErr != nil {
			return inputErr
		}
		r, runErr := p.runPrepared(ctx, s, scenario, input, group, filepath.Join(dir, "retry-"+name))
		if runErr == nil {
			normalizeComparisonMetrics(r.Comparisons, r.Baseline, baseline.Baseline)
			baseline.Comparisons = append(baseline.Comparisons, r.Comparisons...)
			return nil
		}
		if !comparisonRetryable(runErr) {
			return runErr
		}
		if len(group) == 1 {
			comparison := group[0]
			baseline.RejectedComparisons = append(baseline.RejectedComparisons, RejectedComparison{comparison.ID, comparison.Label, "SimulationCraft rejected this item or talent setup. See its retry log."})
			return nil
		}
		middle := len(group) / 2
		if err := retry(group[:middle], name+"-a"); err != nil {
			return err
		}
		return retry(group[middle:], name+"-b")
	}
	middle := len(comparisons) / 2
	groups := [][]Comparison{comparisons[:middle], comparisons[middle:]}
	for i, group := range groups {
		if err := retry(group, fmt.Sprintf("group-%d", i+1)); err != nil {
			return nil, err
		}
	}
	sortComparisons(baseline)
	if len(baseline.RejectedComparisons) > 0 {
		baseline.Warnings = append(baseline.Warnings, fmt.Sprintf("%d comparisons were rejected; only successful setups are shown.", len(baseline.RejectedComparisons)))
	}
	return baseline, nil
}

func normalizeComparisonMetrics(metrics []Metric, from, to Metric) {
	if from.Mean <= 0 || to.Mean <= 0 {
		return
	}
	factor := to.Mean / from.Mean
	for i := range metrics {
		metrics[i].Mean *= factor
		metrics[i].StdDev *= factor
		metrics[i].MeanError *= factor
	}
}

func reportedSlot(slot string) string {
	if slot == "shoulder" {
		return "shoulders"
	}
	if slot == "wrist" {
		return "wrists"
	}
	return slot
}

// SAVE_GEAR contains all active items. JSON gear deliberately omits items with
// no passive stats, including proc-only trinkets. Parse the worker's resolved
// ID after the item-name field, never an apparent ID inside that name.
func resolvedGearIDs(data []byte) (map[string]string, error) {
	gear := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		slot, value, ok := strings.Cut(line, "=")
		if slot == "shoulders" {
			slot = "shoulder"
		} else if slot == "wrists" {
			slot = "wrist"
		}
		if !ok || !slots[slot] {
			continue
		}
		name, fields, ok := strings.Cut(value, ",")
		if !ok || name == "" || name == "empty" || name == "none" || name == "nothing" || strings.Contains(name, "=") || gear[slot] != "" {
			return nil, fmt.Errorf("worker returned invalid equipment for %s", slot)
		}
		id := ""
		for _, field := range strings.Split(fields, ",") {
			if key, val, _ := strings.Cut(field, "="); key == "id" {
				n, err := strconv.Atoi(val)
				if err != nil || n <= 0 || id != "" {
					return nil, fmt.Errorf("worker returned invalid item ID for %s", slot)
				}
				id = val
			}
		}
		if id == "" {
			return nil, fmt.Errorf("worker returned no item ID for %s", slot)
		}
		gear[slot] = id
	}
	return gear, nil
}

// A positive DPS result is insufficient: incorrectly serialized items can be
// silently dropped. Verify baseline IDs against the fresh SAVE_GEAR report and
// any JSON entries, then verify profileset gear overrides.
func verifyWorkerGear(data, savedGear []byte, s *Snapshot, comparisons []Comparison) error {
	var report struct {
		Sim struct {
			Players []struct {
				Name           string `json:"name"`
				Specialization string `json:"specialization"`
				Gear           map[string]struct {
					Encoded string `json:"encoded_item"`
				} `json:"gear"`
			} `json:"players"`
			ProfileSets struct {
				Results []struct {
					Name      string `json:"name"`
					Overrides struct {
						Gear map[string]struct {
							ID    int `json:"item_id"`
							Level int `json:"item_level"`
						} `json:"gear"`
					} `json:"overrides"`
				} `json:"results"`
			} `json:"profilesets"`
		} `json:"sim"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	profile, err := ParseProfile(s.Profile)
	if err != nil {
		return err
	}
	aug := s.Character.Spec == 1473 && s.Character.Class == "EVOKER"
	if (!aug && len(report.Sim.Players) != 1) || (aug && (len(report.Sim.Players) != 5 && len(report.Sim.Players) != 9)) {
		return fmt.Errorf("worker did not report one character's equipment")
	}
	if aug && (report.Sim.Players[0].Name != "Lamda" || report.Sim.Players[0].Specialization != "Augmentation Evoker") {
		return fmt.Errorf("worker equipment report is not for the Augmentation character")
	}
	resolved, err := resolvedGearIDs(savedGear)
	if err != nil {
		return err
	}
	base := baseGear(profile)
	for slot, item := range base {
		if resolved[slot] != itemID(item.Value) {
			return fmt.Errorf("worker did not model the equipped %s item", slot)
		}
		if reported, present := report.Sim.Players[0].Gear[reportedSlot(slot)]; present && itemID(reported.Encoded) != resolved[slot] {
			return fmt.Errorf("worker returned conflicting equipment for %s", slot)
		}
	}
	for _, comparison := range comparisons {
		found := false
		for _, metric := range report.Sim.ProfileSets.Results {
			if metric.Name != comparison.ID {
				continue
			}
			found = true
			for _, item := range comparison.Items {
				id := itemID(item.Value)
				if id == "" || id == itemID(base[item.Slot].Value) {
					continue // SimC reports changes in item ID, not same-ID variants/removals.
				}
				if fmt.Sprint(metric.Overrides.Gear[reportedSlot(item.Slot)].ID) != id {
					return &workerInputError{fmt.Sprintf("worker did not model the requested %s item in %s", item.Slot, comparison.ID)}
				}
				for _, field := range strings.Split(item.Value, ",") {
					if key, val, _ := strings.Cut(field, "="); key == "ilevel" && fmt.Sprint(metric.Overrides.Gear[reportedSlot(item.Slot)].Level) != val {
						return &workerInputError{fmt.Sprintf("worker did not model the requested item level in %s", comparison.ID)}
					}
				}
			}
		}
		if !found {
			return &workerInputError{fmt.Sprintf("worker did not return the requested comparison %s", comparison.ID)}
		}
	}
	return nil
}
