package simming

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// These are explicit inputs to SimC's built-in simplified-ally model, not an
// inferred party or the character's personal damage contribution. Keep them
// fixed through every baseline, profileset, retry and target-set calculation.
type augmentationConfig struct {
	mode      string
	itemLevel int
	skill     float64
}

// Ordinary DPS keeps the original single-player contract. Augmentation must
// report its expected cohort and the aggregate metric, or fail closed.
func parseWorkerResult(data []byte, aug *augmentationConfig) (*Result, error) {
	if aug == nil {
		return ParseResult(data)
	}
	type sample struct {
		Mean       float64 `json:"mean"`
		StdDev     float64 `json:"std_dev"`
		MeanError  float64 `json:"mean_error"`
		MeanStdDev float64 `json:"mean_std_dev"`
	}
	var report struct {
		Logs []struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		} `json:"logs"`
		Sim struct {
			Options struct {
				ConfidenceEstimator float64 `json:"confidence_estimator"`
				ProfilesetMetric    string  `json:"profileset_metric"`
			} `json:"options"`
			Statistics struct {
				RaidDPS sample `json:"raid_dps"`
			} `json:"statistics"`
			Players []struct {
				Name           string `json:"name"`
				Race           string `json:"race"`
				Level          int    `json:"level"`
				Specialization string `json:"specialization"`
				Collected      struct {
					DPS sample `json:"dps"`
				} `json:"collected_data"`
			} `json:"players"`
			Profilesets struct {
				Metric  string `json:"metric"`
				Results []struct {
					Name      string  `json:"name"`
					Mean      float64 `json:"mean"`
					StdDev    float64 `json:"stddev"`
					MeanError float64 `json:"mean_error"`
				} `json:"results"`
			} `json:"profilesets"`
		} `json:"sim"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	if len(report.Sim.Players) != len(aug.allyNames())+1 {
		return nil, fmt.Errorf("worker did not return the requested Augmentation group")
	}
	actor := report.Sim.Players[0]
	if actor.Name != "Lamda" || actor.Specialization != "Augmentation Evoker" || actor.Level != 90 || actor.Collected.DPS.Mean <= 0 {
		return nil, fmt.Errorf("worker did not return the Augmentation character")
	}
	wanted := map[string]bool{}
	for _, name := range aug.allyNames() {
		wanted[name] = true
	}
	sum := actor.Collected.DPS.Mean
	for _, ally := range report.Sim.Players[1:] {
		if !wanted[ally.Name] || ally.Specialization != "Unknown" || ally.Race != "none" || ally.Level != 90 || ally.Collected.DPS.Mean < 0 {
			return nil, fmt.Errorf("worker returned an unexpected simulated ally")
		}
		delete(wanted, ally.Name)
		sum += ally.Collected.DPS.Mean
	}
	if report.Sim.Options.ProfilesetMetric != "raid_dps" || (len(report.Sim.Profilesets.Results) > 0 && report.Sim.Profilesets.Metric != "Raid Damage per Second") {
		return nil, fmt.Errorf("worker did not compare Group DPS")
	}
	dps := report.Sim.Statistics.RaidDPS
	if dps.Mean <= actor.Collected.DPS.Mean || math.Abs(sum-dps.Mean) > math.Max(1e-6, dps.Mean*1e-6) || dps.StdDev < 0 || dps.MeanError < 0 || dps.MeanStdDev < 0 {
		return nil, fmt.Errorf("worker returned inconsistent Group DPS")
	}
	if dps.MeanError == 0 {
		if dps.MeanStdDev > 0 && report.Sim.Options.ConfidenceEstimator <= 0 {
			return nil, fmt.Errorf("worker omitted group uncertainty settings")
		}
		dps.MeanError = dps.MeanStdDev * report.Sim.Options.ConfidenceEstimator
	}
	if math.IsInf(dps.MeanError, 0) || math.IsNaN(dps.MeanError) {
		return nil, fmt.Errorf("worker returned invalid group uncertainty")
	}
	r := &Result{Metric: "group_dps", Baseline: Metric{Name: "Current gear", Mean: dps.Mean, StdDev: dps.StdDev, MeanError: dps.MeanError}, Comparisons: []Metric{}, Warnings: []string{}}
	for _, entry := range report.Logs {
		if entry.Level == "error" {
			return nil, fmt.Errorf("SimulationCraft reported an error: %.1024s", entry.Message)
		}
		if (strings.Contains(entry.Level, "warn") || entry.Level == "implementation_not_yet_verified") && len(r.Warnings) < 32 {
			r.Warnings = append(r.Warnings, fmt.Sprintf("SimulationCraft: %.1024s", entry.Message))
		}
	}
	for _, v := range report.Sim.Profilesets.Results {
		if v.Mean <= 0 || v.StdDev < 0 || v.MeanError < 0 {
			return nil, fmt.Errorf("worker returned an invalid group comparison")
		}
		r.Comparisons = append(r.Comparisons, Metric{Name: v.Name, Mean: v.Mean, StdDev: v.StdDev, MeanError: v.MeanError})
	}
	return r, nil
}

func validateAugmentationSettings(s Scenario) error {
	if s.AugmentationMode != "" && s.AugmentationMode != "dungeon" && s.AugmentationMode != "raid" {
		return fmt.Errorf("choose Dungeon or Raid for Augmentation")
	}
	if s.AllyItemLevel < 0 || s.AllyItemLevel > 1000 {
		return fmt.Errorf("ally item level must be 1–1000")
	}
	if math.IsNaN(s.AllySkill) || math.IsInf(s.AllySkill, 0) || s.AllySkill < 0 || s.AllySkill > 1 || (s.AllySkill > 0 && s.AllySkill < .01) {
		return fmt.Errorf("ally performance must be 1–100%%")
	}
	return nil
}

func augmentationSettings(s Scenario) augmentationConfig {
	a := augmentationConfig{s.AugmentationMode, s.AllyItemLevel, s.AllySkill}
	if a.mode == "" {
		a.mode = "dungeon"
	}
	if a.itemLevel == 0 {
		a.itemLevel = 328
	}
	if a.skill == 0 {
		a.skill = .9
	}
	return a
}

func (a augmentationConfig) input() string {
	dungeon := 0
	if a.mode == "dungeon" {
		dungeon = 1
	}
	return fmt.Sprintf("evoker.make_simplified_if_alone=1\nevoker.patchwerk_in_dungeon=%d\nevoker.simplified_actor_ilevel=%d\nevoker.ally_skill=%g\nsingle_actor_batch=0\nprofileset_main_actor_index=0\nprofileset_report_player_index=0\n", dungeon, a.itemLevel, a.skill)
}

func (a augmentationConfig) allyNames() []string {
	if a.mode == "dungeon" {
		return []string{"Bob Arcane", "Bob Demo", "Bob BDK", "Bob Healer"}
	}
	return []string{"Bob Shadow1", "Bob Shadow2", "Bob Arcane", "Bob BM", "Bob Flat1", "Bob Demo", "Bob Healer1", "Bob BDK"}
}

func (a augmentationConfig) describe(r *Result) {
	r.Model = "Augmentation · " + a.mode + " group"
	r.ModelInputs = map[string]string{
		"metric":      "Total Group DPS, including the Evoker and simulated allies; not personal DPS or isolated Augmentation contribution",
		"allies":      fmt.Sprintf("%d simulated allies; item level %d; performance %g%%", len(a.allyNames()), a.itemLevel, a.skill*100),
		"comparisons": "The same ally model is used for every gear and talent comparison. Gain percentages use total Group DPS.",
		"encounter":   "Stationary targets, full raid buffs. Dungeon selects a party model, not a timed Mythic+ route.",
	}
	r.Warnings = append(r.Warnings, "Group DPS includes simulated allies. Results depend on this group model and are not a prediction for your current party.")
	if a.mode == "raid" {
		r.Warnings = append(r.Warnings, "The reduced raid model uses eight allies and may overestimate Ebon Might.")
	}
}
