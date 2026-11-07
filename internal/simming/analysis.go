package simming

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	analysisQuickIterations  = 20
	analysisRefineIterations = 100
	analysisBatchSize        = 16
	analysisRefineCount      = 12
)

// runAnalysis spends a bounded amount of time answering the routine questions
// together: equipped next/max ranks, bag gear, Vault choices, their maximum
// upgrade levels, and saved talent loadouts for a raid boss and Dungeon Slice.
// The broad pass is deliberately low precision; remaining time refines leading
// results. Augmentation retains its fixed one/five-target group model because
// that model is not a timed Mythic+ route.
func (p *Provider) runAnalysis(ctx context.Context, s *Snapshot, scenario Scenario, selected []string, dir string) (*Result, error) {
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	profile, err := ParseProfile(s.Profile)
	if err != nil {
		return nil, err
	}
	budget := scenario.BudgetSeconds
	if budget == 0 {
		budget = 60
	}
	if budget < 30 || budget > 60 {
		return nil, fmt.Errorf("analysis time budget must be 30–60 seconds")
	}
	scenario.Task = ""
	scenario.Iterations = analysisQuickIterations
	scenario.quick = true
	var comparisons []Comparison
	if len(selected) > 0 {
		if scenario.CompareMode != "topgear" {
			return nil, fmt.Errorf("selected analysis items require Top Gear mode")
		}
		profile, err = prepareComparisonProfile(s, profile, scenario, selected)
		if err != nil {
			return nil, err
		}
		comparisons, err = profile.Comparisons(scenario, selected)
		if err != nil {
			return nil, err
		}
	} else {
		scenario.CompareMode = "individual"
		scenario.Talents = nil
		if len(scenario.ItemOptions) > 0 {
			return nil, fmt.Errorf("select items before planning alternatives")
		}

		upgrades := map[string]ItemUpgrade{}
		for _, upgrade := range s.Upgrades {
			upgrades[upgrade.Source+"\x00"+upgrade.Slot+"\x00"+upgrade.Value] = upgrade
		}
		current, potential := []Comparison{}, []Comparison{}
		for _, raw := range profile.Candidates {
			if raw.Source != "owned" && raw.Source != "vault" {
				continue
			}
			candidate := raw
			candidate.Level = itemLevelFromValue(candidate.Value)
			candidate.Variant = "current"
			if upgrade, ok := upgrades[candidate.Source+"\x00"+candidate.Slot+"\x00"+candidate.Value]; ok {
				candidate.Level, candidate.MaxLevel = upgrade.Level, upgrade.MaxLevel
			}
			variants, compareErr := profile.Comparisons(scenario, []string{candidate.ID})
			if compareErr != nil {
				return nil, compareErr
			}
			for i := range variants {
				for j := range variants[i].Items {
					variants[i].Items[j].Level = candidate.Level
					variants[i].Items[j].MaxLevel = candidate.MaxLevel
					variants[i].Items[j].Variant = "current"
				}
			}
			current = append(current, variants...)
			if candidate.MaxLevel > candidate.Level {
				maxCandidate := candidate
				maxCandidate.BaseValue = candidate.Value
				maxCandidate.Value = withItemLevel(maxCandidate.Value, maxCandidate.MaxLevel)
				maxCandidate.Level = maxCandidate.MaxLevel
				maxCandidate.Variant = "max"
				copyProfile := *profile
				copyProfile.Candidates = []Candidate{maxCandidate}
				variants, compareErr = copyProfile.Comparisons(scenario, []string{maxCandidate.ID})
				if compareErr != nil {
					return nil, compareErr
				}
				for i := range variants {
					variants[i].ID += "_max"
					for j := range variants[i].Items {
						variants[i].Items[j].Value = maxCandidate.Value
						variants[i].Items[j].Level = maxCandidate.Level
						variants[i].Items[j].MaxLevel = maxCandidate.MaxLevel
						variants[i].Items[j].Variant = "max"
					}
				}
				potential = append(potential, variants...)
			}
		}

		crest := analysisEquippedComparisons(s, profile)
		talents := analysisTalentComparisons(s, profile)
		comparisons = append(crest, current...)
		comparisons = append(comparisons, talents...)
		comparisons = append(comparisons, potential...)
	}
	if len(comparisons) == 0 {
		return nil, fmt.Errorf("the character export contains no bag, Vault, upgrade, or saved-talent options to analyze")
	}
	if len(comparisons) > 768 {
		return nil, fmt.Errorf("the character export contains too many variants to analyze at once")
	}
	comparisons, rejected, warnings, err := applyGearRules(s, profile, comparisons, len(selected) > 0)
	if err != nil {
		return nil, err
	}
	if len(comparisons) == 0 {
		return nil, fmt.Errorf("no captured item or talent option can be modeled")
	}

	var combined *Result
	allRejected := []RejectedComparison{}
	rejectionSeen := map[string]bool{}
	addRejected := func(values []RejectedComparison) {
		for _, value := range values {
			key := value.ID + "\x00" + value.Reason
			if !rejectionSeen[key] {
				rejectionSeen[key] = true
				allRejected = append(allRejected, value)
			}
		}
	}
	addRejected(rejected)
	encounters, err := analysisEncounters(profile, scenario)
	if err != nil {
		return nil, err
	}
	perTarget := time.Duration(budget) * time.Second / time.Duration(len(encounters))
	for index, encounter := range encounters {
		key := index + 4
		if index == 0 {
			key = 1
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		runScenario := encounter.apply(scenario)
		targetCtx, cancel := context.WithTimeout(ctx, perTarget)
		run, tested, refined, elapsed, complete, runErr := p.runBudgetedAnalysisTarget(targetCtx, s, profile, runScenario, comparisons, filepath.Join(dir, fmt.Sprintf("encounter-%d", key)))
		cancel()
		if runErr != nil {
			return nil, runErr
		}
		collapsed := bestCandidateMetrics(run.Comparisons)
		targetRejected := append([]RejectedComparison{}, rejected...)
		targetRejected = append(targetRejected, run.RejectedComparisons...)
		addRejected(run.RejectedComparisons)
		if combined == nil {
			combined = run
			combined.Comparisons = []Metric{}
			combined.RejectedComparisons = nil
			combined.Warnings = append(combined.Warnings, warnings...)
			if len(selected) > 0 {
				combined.Warnings = append(combined.Warnings, "Top Gear compares legal combinations of the selected physical items and their explicit alternatives; gains are not added together.")
			} else {
				combined.Warnings = append(combined.Warnings, "Bag results are direct replacements. Crest results model one more rank and each equipped item's maximum captured rank.")
			}
		}
		combined.TargetResults = append(combined.TargetResults, TargetResult{
			Targets: key, EncounterID: encounter.ID, EncounterName: encounter.Name, TargetCount: encounter.Targets,
			FightStyle: encounter.FightStyle, RaidBuffs: encounter.RaidBuffs, Seconds: encounter.Seconds, Bloodlust: encounter.Bloodlust,
			Baseline: run.Baseline, Comparisons: collapsed, RejectedComparisons: targetRejected,
			Tested: tested, Requested: len(comparisons), Refined: refined, Iterations: analysisQuickIterations,
			ElapsedSeconds: elapsed.Seconds(), Complete: complete,
		})
		combined.Incomplete = combined.Incomplete || !complete
	}
	combined.Model = "SimulationCraft · timed crest, bag, Vault, upgrade, and talent analysis"
	combined.RejectedComparisons = allRejected
	if len(rejected) > 0 {
		combined.Warnings = append(combined.Warnings, fmt.Sprintf("%d variants were excluded by equipment restrictions.", len(rejected)))
	}
	return combined, nil
}

func analysisEquippedComparisons(s *Snapshot, profile *Profile) []Comparison {
	base := baseGear(profile)
	out := []Comparison{}
	allMax := []Candidate{}
	for _, upgrade := range s.Upgrades {
		if upgrade.Source != "equipped" || base[upgrade.Slot].Value != upgrade.Value {
			continue
		}
		id := "equipped_" + upgrade.Slot
		label := upgrade.Label
		if label == "" {
			label = upgrade.Slot
		}
		add := func(variant string, level int) {
			if level <= upgrade.Level || level > upgrade.MaxLevel {
				return
			}
			item := Candidate{ID: id, Label: label, Source: "equipped", Slot: upgrade.Slot, BaseValue: upgrade.Value,
				Value: withItemLevel(upgrade.Value, level), Level: level, NextLevel: upgrade.NextLevel,
				MaxLevel: upgrade.MaxLevel, Track: upgrade.Track, CurrentRank: upgrade.CurrentRank,
				MaxRank: upgrade.MaxRank, RankLevels: append([]int{}, upgrade.RankLevels...), Variant: variant}
			out = append(out, Comparison{ID: id + "_" + variant, Label: label, Items: []Candidate{item}})
		}
		if len(upgrade.RankLevels) == upgrade.MaxRank-upgrade.CurrentRank+1 && len(upgrade.RankLevels) > 1 {
			for index := 1; index < len(upgrade.RankLevels); index++ {
				rank := upgrade.CurrentRank + index
				variant := fmt.Sprintf("rank_%d", rank)
				if index == 1 {
					variant = "next"
				} else if index == len(upgrade.RankLevels)-1 {
					variant = "max"
				}
				add(variant, upgrade.RankLevels[index])
			}
		} else {
			add("next", upgrade.NextLevel)
			if upgrade.MaxLevel != upgrade.NextLevel {
				add("max", upgrade.MaxLevel)
			}
		}
		allMax = append(allMax, Candidate{ID: "equipped_all", Label: "All upgradeable equipped items", Source: "equipped",
			Slot: upgrade.Slot, BaseValue: upgrade.Value, Value: withItemLevel(upgrade.Value, upgrade.MaxLevel), Level: upgrade.MaxLevel,
			MaxLevel: upgrade.MaxLevel, Track: upgrade.Track, CurrentRank: upgrade.CurrentRank,
			MaxRank: upgrade.MaxRank, RankLevels: append([]int{}, upgrade.RankLevels...), Variant: "all_max"})
	}
	if len(allMax) > 0 {
		out = append(out, Comparison{ID: "equipped_all_max", Label: "All upgradeable equipped items maxed", Items: allMax})
	}
	return out
}

func (p *Provider) runBudgetedAnalysisTarget(ctx context.Context, s *Snapshot, profile *Profile, scenario Scenario, comparisons []Comparison, dir string) (*Result, int, int, time.Duration, bool, error) {
	started := time.Now()
	var combined *Result
	tested := 0
	for offset := 0; offset < len(comparisons); offset += analysisBatchSize {
		batch := comparisons[offset:min(offset+analysisBatchSize, len(comparisons))]
		run, err := p.runComparisons(ctx, s, profile, scenario, batch, filepath.Join(dir, fmt.Sprintf("screen-%03d", offset/analysisBatchSize)))
		if err != nil {
			if ctx.Err() != nil && combined != nil {
				break
			}
			return nil, tested, 0, time.Since(started), false, err
		}
		if combined == nil {
			combined = run
		} else {
			normalizeComparisonMetrics(run.Comparisons, run.Baseline, combined.Baseline)
			combined.Comparisons = append(combined.Comparisons, run.Comparisons...)
			combined.RejectedComparisons = append(combined.RejectedComparisons, run.RejectedComparisons...)
			combined.Warnings = append(combined.Warnings, run.Warnings...)
		}
		if run.Incomplete {
			tested += run.processedComparisons
			break
		}
		tested += len(batch)
	}
	if combined == nil {
		return nil, tested, 0, time.Since(started), false, fmt.Errorf("the time budget expired before SimulationCraft returned a result")
	}
	complete := tested == len(comparisons)
	refined := 0
	if complete {
		deadline, hasDeadline := ctx.Deadline()
		if !hasDeadline || time.Until(deadline) >= 2*time.Second {
			refine := leadingComparisons(combined.Comparisons, comparisons, analysisRefineCount)
			if len(refine) > 0 {
				refineScenario := scenario
				refineScenario.Iterations = analysisRefineIterations
				run, err := p.runComparisons(ctx, s, profile, refineScenario, refine, filepath.Join(dir, "refine"))
				if err == nil {
					oldBaseline := combined.Baseline
					normalizeComparisonMetrics(combined.Comparisons, oldBaseline, run.Baseline)
					byName := map[string]Metric{}
					for _, metric := range run.Comparisons {
						metric.Refined = true
						byName[metric.Name] = metric
					}
					for i, metric := range combined.Comparisons {
						if replacement, ok := byName[metric.Name]; ok {
							combined.Comparisons[i] = replacement
							refined++
						}
					}
					combined.Baseline = run.Baseline
					combined.RejectedComparisons = append(combined.RejectedComparisons, run.RejectedComparisons...)
				} else if ctx.Err() == nil {
					return nil, tested, refined, time.Since(started), complete, err
				}
			}
		}
	}
	sortComparisons(combined)
	if !complete {
		combined.Warnings = append(combined.Warnings, fmt.Sprintf("Time budget reached after %d of %d variants; completed results are shown.", tested, len(comparisons)))
	}
	return combined, tested, refined, time.Since(started), complete, nil
}

func analysisTalentComparisons(s *Snapshot, profile *Profile) []Comparison {
	current := ""
	for _, line := range strings.Split(profile.Base, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok && key == "talents" {
			current = value
			break
		}
	}
	builds := append([]TalentBuild{}, profile.Talents...)
	if s.GameRequest != nil {
		builds = append(builds, s.GameRequest.Talents...)
	}
	seen := map[string]bool{current: true}
	out := []Comparison{}
	for _, build := range builds {
		if build.Code == "" || seen[build.Code] {
			continue
		}
		seen[build.Code] = true
		label := build.Label
		if label == "" {
			label = fmt.Sprintf("Saved talent build %d", len(out)+1)
		}
		out = append(out, Comparison{ID: fmt.Sprintf("talent%d", len(out)+1), Label: label, Talents: build.Code})
	}
	return out
}

func itemLevelFromValue(value string) int {
	for _, field := range strings.Split(value, ",") {
		if key, raw, ok := strings.Cut(field, "="); ok && key == "ilevel" {
			level, _ := strconv.Atoi(raw)
			return level
		}
	}
	return 0
}

func withItemLevel(value string, level int) string {
	parts := strings.Split(value, ",")
	found := false
	for i, field := range parts {
		if key, _, ok := strings.Cut(field, "="); ok && key == "ilevel" {
			parts[i] = fmt.Sprintf("ilevel=%d", level)
			found = true
		}
	}
	if !found {
		parts = append(parts, fmt.Sprintf("ilevel=%d", level))
	}
	return strings.Join(parts, ",")
}

func leadingComparisons(metrics []Metric, comparisons []Comparison, limit int) []Comparison {
	ordered := append([]Metric{}, metrics...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Mean > ordered[j].Mean })
	wanted := map[string]bool{}
	// Reserve refinement space for the questions shown on each result page.
	// A pure top-N list can spend every precise comparison on max-level items,
	// leaving the "equip now" column as low-precision screening data.
	type quota struct {
		source, variant string
		count           int
	}
	for _, group := range []quota{
		{source: "owned", variant: "current", count: 3},
		{source: "owned", variant: "max", count: 2},
		{source: "equipped", count: 2},
		{source: "vault", variant: "current", count: 1},
		{source: "vault", variant: "max", count: 1},
		{source: "talent", count: 1},
	} {
		remaining := group.count
		for _, metric := range ordered {
			if len(wanted) >= limit || remaining == 0 {
				break
			}
			if !wanted[metric.Name] && metric.Source == group.source && (group.variant == "" || metric.Variant == group.variant) {
				wanted[metric.Name] = true
				remaining--
			}
		}
	}
	for _, metric := range ordered {
		if len(wanted) >= limit {
			break
		}
		wanted[metric.Name] = true
	}
	out := []Comparison{}
	for _, comparison := range comparisons {
		if wanted[comparison.ID] {
			out = append(out, comparison)
		}
	}
	return out
}

func bestCandidateMetrics(metrics []Metric) []Metric {
	best := map[string]Metric{}
	for _, metric := range metrics {
		key := metric.CandidateID + "\x00" + metric.Variant
		if metric.CandidateID == "" {
			key = metric.Name
		}
		if current, ok := best[key]; !ok || metric.Mean > current.Mean {
			best[key] = metric
		}
	}
	out := make([]Metric, 0, len(best))
	for _, metric := range best {
		out = append(out, metric)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Mean == out[j].Mean {
			return out[i].CandidateID+out[i].Variant < out[j].CandidateID+out[j].Variant
		}
		return out[i].Mean > out[j].Mean
	})
	return out
}
