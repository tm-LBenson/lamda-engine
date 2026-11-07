package simming

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/heallogs"
	"github.com/tm-LBenson/lamda-engine/internal/healsim"
	"github.com/tm-LBenson/lamda-engine/internal/wowspec"
)

// New encounter/reference tasks don't require SimC or a separately installed
// QE worker. They use the same persisted, cancellable request/result pipeline.
type encounterProvider struct{ task, cache, logs string }

func builtinTask(task string) bool { return task == "healing" || task == "builds" }
func (p *encounterProvider) Capability(s *Snapshot, _ time.Time) Capability {
	c := Capability{ID: "lamda-healing", Name: "Lamda Healing Lab", Version: healsim.Version, Validation: "experimental"}
	if p.task == "builds" {
		c.ID = "build-reference"
		c.Name = "Build trends"
		c.Validation = "source-snapshot"
	}
	if s == nil || s.Client != "retail" || s.Character.Spec == 0 {
		c.Reason = "Capture a Retail character with an active specialization."
		return c
	}
	spec, ok := wowspec.Lookup(s.Character.Spec)
	if !ok || spec.Class != s.Character.Class {
		c.Reason = "Unrecognized class and specialization."
		return c
	}
	if p.task == "healing" {
		if !spec.Healer {
			c.Reason = "Select a healer specialization for Healing Lab."
			return c
		}
		if s.Character.Level != 90 || s.Interface < 120100 || s.Interface >= 120200 {
			c.Reason = "The experimental model is pinned to level 90 and Patch 12.1."
			return c
		}
		if s.HealingStats == nil {
			c.Reason = "Update your character outside combat to capture healing stats and talents."
			return c
		}
		if err := s.HealingStats.Validate(); err != nil {
			c.Reason = err.Error()
			return c
		}
		if err := healsim.ValidateSpellbook(s.Character.Spec, *s.HealingStats); err != nil {
			c.Reason = err.Error()
			return c
		}
	}
	c.Available = true
	return c
}
func (p *encounterProvider) Run(ctx context.Context, s *Snapshot, scenario Scenario, selected []string, dir string) (*Result, error) {
	c := p.Capability(s, time.Now())
	if !c.Available {
		return nil, fmt.Errorf("%s", c.Reason)
	}
	if len(selected) > 0 || len(scenario.Talents) > 0 || len(scenario.LoadoutSlots) > 0 || len(scenario.LoadoutLevels) > 0 || len(scenario.ItemOptions) > 0 || len(scenario.Encounters) > 0 || scenario.Bloodlust != "" || (scenario.CompareMode != "" && scenario.CompareMode != "individual") {
		return nil, fmt.Errorf("healing encounters and build references use their own settings, without gear or talent replacements")
	}
	r := &Result{Provider: c.Name, ProviderVersion: c.Version, Validation: c.Validation, Comparisons: []Metric{}, ModelInputs: map[string]string{"sourceReview": "2026-09-29", "scope": "Retail 12.1"}}
	if p.task == "builds" {
		mode := scenario.BuildMode
		if mode == "" {
			mode = "m+"
		}
		specID := scenario.BuildSpec
		if specID == 0 {
			specID = s.Character.Spec
		}
		report, err := fetchBuilds(ctx, specID, mode, p.cache)
		if err != nil {
			return nil, err
		}
		r.Builds = report
		r.Metric = "build_reference"
		r.Model = "Murlok public summary"
		r.Warnings = report.Warnings
	} else {
		opts := healsim.Options{Spec: s.Character.Spec, Scenario: scenario.HealingScenario, Seconds: scenario.Seconds, Intensity: scenario.HealingIntensity, Runs: scenario.HealingRuns, Policy: scenario.HealingPolicy, Seed: 1729}
		var learned []healsim.TrainingSummary
		if opts.Scenario == "learned" {
			if p.logs == "" {
				return nil, fmt.Errorf("the companion needs the Retail Logs folder to learn encounters")
			}
			training, catalog, audit, err := heallogs.Learn(ctx, p.logs, filepath.Join(p.cache, "..", "healing-log-cache"), scenario.HealingCohort)
			if saveErr := writeJSON(filepath.Join(dir, "damage-training-audit.json"), audit); saveErr != nil {
				return nil, saveErr
			}
			if err != nil {
				return nil, err
			}
			data, err := json.Marshal(training)
			if err != nil {
				return nil, err
			}
			if err = os.WriteFile(filepath.Join(dir, "damage-training.json"), data, 0600); err != nil {
				return nil, err
			}
			r.ModelInputs["damageTrainingSHA256"] = fmt.Sprintf("%x", sha256.Sum256(data))
			r.ModelInputs["damageTraining"] = fmt.Sprintf("%d training fights; one held-out fight; %d cached / %d log files", len(training.Samples), audit.Cached, audit.Files)
			opts.Training = training
			learned = catalog
		}
		if opts.Scenario == "replay" {
			data, err := readBounded(filepath.Join(p.cache, "..", "healing-replay.json"), 12*1024*1024)
			if err != nil {
				return nil, fmt.Errorf("import a damage profile into the companion's healing-replay.json first")
			}
			var replay healsim.Replay
			if err = json.Unmarshal(data, &replay); err != nil {
				return nil, fmt.Errorf("invalid damage replay JSON")
			}
			if err = replay.Validate(); err != nil {
				return nil, err
			}
			// Keep the exact input with the job before the shared import slot can
			// be replaced by the next encounter. Results retain only metadata.
			if err = os.WriteFile(filepath.Join(dir, "damage-replay.json"), data, 0600); err != nil {
				return nil, err
			}
			r.ModelInputs["damageProfileSHA256"] = fmt.Sprintf("%x", sha256.Sum256(data))
			opts.Replay = &replay
			opts.Seconds = int(math.Ceil(replay.Seconds))
		}
		report, err := healsim.Run(ctx, *s.HealingStats, opts)
		if err != nil {
			return nil, err
		}
		r.Healing = report
		r.Healing.LearnedEncounters = learned
		r.Metric = "healing_hps"
		r.Model = healsim.Version
		r.Warnings = report.Warnings
		r.ModelInputs["capturedBuild"] = s.Build
		r.ModelInputs["buildCoverage"] = "Incomplete: captured sheet stats include passive gear stats; dynamic item effects, set bonuses and unlisted talent interactions are omitted."
		for _, item := range s.Items {
			if item.Slot == 13 || item.Slot == 14 {
				r.ModelInputs[fmt.Sprintf("unmodeledTrinketSlot%d", item.Slot)] = item.Link
			}
		}
		r.ModelInputs["spellDataBuild"] = "12.1.0.69933 / hotfix 2026-09-22"
		if s.Build != "69933" {
			r.Warnings = append(r.Warnings, "The captured game build differs from the pinned spell-data build 69933; throughput is not calibrated for these changes.")
		}
		for i, out := range report.Outcomes {
			metric := Metric{Name: out.Policy, Mean: out.HPS, StdDev: out.StdDev, MeanError: out.MeanError}
			if i == 0 {
				r.Baseline = metric
			} else {
				r.Comparisons = append(r.Comparisons, metric)
			}
		}
	}
	if err := writeJSON(filepath.Join(dir, "encounter-result.json"), r); err != nil {
		return nil, err
	}
	return r, nil
}
