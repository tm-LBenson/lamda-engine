package simming

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type GearDecision struct {
	Comparison           string       `json:"comparison"`
	Label                string       `json:"label"`
	Advice               string       `json:"advice"`
	Absolute             float64      `json:"absolute"`
	Percent              float64      `json:"percent"`
	Margin               float64      `json:"margin"`
	WithinMargin         bool         `json:"withinMargin"`
	Investment           bool         `json:"investment"`
	Venomstones          int          `json:"venomstones,omitempty"`
	CostKnown            bool         `json:"costKnown"`
	AvailabilityVerified bool         `json:"availabilityVerified"`
	Items                []ResultItem `json:"items"`
}
type FarmDecision struct {
	Item     Upgrade `json:"item"`
	Evidence string  `json:"evidence"`
}
type DecisionView struct {
	JobID      string         `json:"jobID"`
	Encounter  Encounter      `json:"encounter"`
	Metric     string         `json:"metric"`
	Validation string         `json:"validation"`
	Complete   bool           `json:"complete"`
	Equip      []GearDecision `json:"equip"`
	Upgrade    []GearDecision `json:"upgrade"`
	Farm       []FarmDecision `json:"farm"`
	Warnings   []string       `json:"warnings,omitempty"`
}
type ReferenceEvidence struct {
	Source                 string      `json:"source"`
	URL                    string      `json:"url,omitempty"`
	Context                string      `json:"context"`
	Available              bool        `json:"available"`
	PersonalRecommendation bool        `json:"personalRecommendation"`
	UpdatedAt              time.Time   `json:"updatedAt,omitempty"`
	Sample                 int         `json:"sample,omitempty"`
	Stale                  bool        `json:"stale,omitempty"`
	Gear                   []BuildGear `json:"gear,omitempty"`
	Note                   string      `json:"note"`
}
type DecisionReport struct {
	SnapshotID string              `json:"snapshotID"`
	Context    string              `json:"context"`
	Views      []DecisionView      `json:"views"`
	References []ReferenceEvidence `json:"references"`
	StaleJobs  int                 `json:"staleJobs"`
	Note       string              `json:"note,omitempty"`
}

// Bag reordering and character labels do not change the combat baseline.
// Talents, gems, enchants, Omnium, equipped modifiers and build identity do.
func setupHash(s *Snapshot) string {
	if s == nil {
		return ""
	}
	p, err := ParseProfile(s.Profile)
	if err != nil {
		return ""
	}
	lines := strings.FieldsFunc(p.Base, func(r rune) bool { return r == '\n' })
	sort.Strings(lines)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s:%s\n%s", s.Client, s.Character.Spec, s.Build, s.Character.Role, strings.Join(lines, "\n")))))
}

func buildDecisions(state WorkspaceState, snapshotID, context string) (*DecisionReport, error) {
	if context != "" && context != "raid" && context != "m+" {
		return nil, fmt.Errorf("choose Raid or Mythic+ context")
	}
	var current *Summary
	for i := range state.Snapshots {
		if state.Snapshots[i].Snapshot.ID == snapshotID {
			current = &state.Snapshots[i]
			break
		}
	}
	if current == nil {
		return nil, fmt.Errorf("snapshot not found")
	}
	out := &DecisionReport{SnapshotID: snapshotID, Context: context, Views: []DecisionView{}, References: []ReferenceEvidence{}}
	jobs := append([]Job{}, state.Jobs...)
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	summaries := map[string]Summary{}
	currentItems := map[string]int{}
	ownedIDs := map[int]bool{}
	for _, group := range [][]Candidate{current.Candidates, current.Equipped} {
		for _, item := range group {
			currentItems[item.Source+":"+item.Value]++
			if item.Source == "owned" || item.Source == "equipped" {
				id, _ := strconv.Atoi(itemID(item.Value))
				ownedIDs[id] = true
			}
		}
	}
	for _, s := range state.Snapshots {
		summaries[s.Snapshot.ID] = s
	}
	seen := map[string]bool{}
	referenceSeen := false
	for _, job := range jobs {
		if job.State != "complete" || job.Result == nil {
			continue
		}
		result := job.Result
		if result.Builds != nil && result.Builds.Spec == current.Snapshot.Character.Spec && !referenceSeen && (context == "" || result.Builds.Mode == context) {
			report := result.Builds
			gear := append([]BuildGear{}, report.Gear...)
			for i := range gear {
				gear[i].BIS = false
			}
			out.References = append(out.References, ReferenceEvidence{Source: report.Source, URL: report.URL, Context: report.Mode, Available: true, UpdatedAt: report.UpdatedAt, Sample: report.Sample, Stale: report.Stale || report.UpdatedAt.IsZero() || time.Since(report.UpdatedAt) > 48*time.Hour, Gear: gear, Note: "Community usage; not personalized stat weights or personal BiS."})
			referenceSeen = true
		}
		captured, ok := summaries[job.SnapshotID]
		if !ok || captured.Snapshot.Character.GUID != current.Snapshot.Character.GUID || captured.Snapshot.Character.Spec != current.Snapshot.Character.Spec {
			continue
		}
		if current.SetupHash == "" || job.SetupHash != current.SetupHash {
			out.StaleJobs++
			continue
		}
		metricName := strings.ToLower(result.Metric)
		if metricName != "dps" && metricName != "group_dps" && metricName != "healing_score" {
			continue
		}
		views := result.TargetResults
		if len(views) == 0 {
			views = []TargetResult{{EncounterID: "run", EncounterName: "Selected encounter", TargetCount: job.Scenario.Targets, Seconds: job.Scenario.Seconds, FightStyle: job.Scenario.FightStyle, RaidBuffs: job.Scenario.RaidBuffs, Bloodlust: job.Scenario.Bloodlust, Baseline: result.Baseline, Comparisons: result.Comparisons, Complete: !result.Incomplete && len(result.RejectedComparisons) == 0}}
		}
		for _, v := range views {
			if len(out.Views) >= 8 {
				break
			}
			e := Encounter{v.EncounterID, v.EncounterName, v.FightStyle, v.TargetCount, v.Seconds, v.RaidBuffs, v.Bloodlust}
			if e.Targets == 0 {
				e.Targets = v.Targets
				if e.FightStyle == "dungeon_slice" {
					e.Targets = 1
				}
			}
			if e.Seconds == 0 {
				e.Seconds = job.Scenario.Seconds
			}
			if e.FightStyle == "" {
				e.FightStyle = "patchwerk"
			}
			if e.RaidBuffs == "" {
				e.RaidBuffs = job.Scenario.RaidBuffs
			}
			key := fmt.Sprintf("%s:%d:%d:%s:%s:%s", e.FightStyle, e.Targets, e.Seconds, e.RaidBuffs, e.Bloodlust, gameJobTask(job))
			if seen[key] {
				continue
			}
			seen[key] = true
			d := DecisionView{JobID: job.ID, Encounter: e, Metric: metricName, Validation: result.Validation, Complete: v.Complete, Equip: []GearDecision{}, Upgrade: []GearDecision{}, Farm: []FarmDecision{}, Warnings: append([]string{}, result.Warnings...)}
			if v.Baseline.Mean <= 0 {
				continue
			}
			for _, metric := range v.Comparisons {
				if metric.Source == "talent" {
					continue
				}
				available := true
				counts := map[string]int{}
				for _, item := range metric.Items {
					if item.ID == 0 {
						continue
					}
					key := item.BaseKey
					if key == "" {
						key = item.Key
					}
					if key == "" {
						continue
					}
					physical := item.Source + ":" + key
					counts[physical]++
					if counts[physical] > currentItems[physical] {
						available = false
					}
				}
				if !available {
					continue
				}
				delta := metric.Mean - v.Baseline.Mean
				margin := math.Hypot(metric.MeanError, v.Baseline.MeanError)
				if math.IsNaN(delta) || math.IsInf(delta, 0) || delta <= 0 {
					continue
				}
				planned := metric.Variant == "planned" || metric.Variant == "next" || metric.Variant == "max" || metric.Variant == "all_max"
				for _, item := range metric.Items {
					if item.Variant == "planned" || (item.BaseKey != "" && item.BaseKey != item.Key) {
						planned = true
					}
				}
				advice := "model-favors"
				if delta <= margin {
					advice = "within-margin"
				}
				choice := GearDecision{Comparison: metric.Name, Label: metric.Label, Advice: advice, Absolute: delta, Percent: 100 * delta / v.Baseline.Mean, Margin: margin, WithinMargin: delta <= margin, Investment: planned, AvailabilityVerified: len(metric.Items) > 0 && len(counts) > 0, Items: append([]ResultItem{}, metric.Items...)}
				for _, item := range metric.Items {
					choice.Venomstones += item.Venomstones
				}
				if planned {
					d.Upgrade = append(d.Upgrade, choice)
				} else {
					d.Equip = append(d.Equip, choice)
				}
			}
			for _, u := range result.Upgrades {
				kind := strings.ToLower(u.Kind)
				if u.Owned || ownedIDs[u.ID] || u.Gain <= 0 || !u.LootVerified || (context == "raid" && kind != "raid") || (context == "m+" && kind != "dungeon") {
					continue
				}
				// Preserve the bounded-search evidence without turning its BiS
				// flag into an assertion of a globally optimal personal set.
				u.BIS = false
				u.BISRank = 0
				u.BISSlot = ""
				note := "Simulated captured drop; acquisition odds require separate assumptions."
				if !u.PoolComplete {
					note += " Source pool is incomplete."
				}
				d.Farm = append(d.Farm, FarmDecision{u, note})
			}
			sort.SliceStable(d.Equip, func(i, j int) bool { return d.Equip[i].Absolute > d.Equip[j].Absolute })
			sort.SliceStable(d.Upgrade, func(i, j int) bool { return d.Upgrade[i].Absolute > d.Upgrade[j].Absolute })
			sort.SliceStable(d.Farm, func(i, j int) bool { return d.Farm[i].Item.Gain > d.Farm[j].Item.Gain })
			d.Equip = d.Equip[:min(3, len(d.Equip))]
			d.Upgrade = d.Upgrade[:min(3, len(d.Upgrade))]
			d.Farm = d.Farm[:min(3, len(d.Farm))]
			out.Views = append(out.Views, d)
		}
	}
	if !referenceSeen {
		out.References = append(out.References, ReferenceEvidence{Source: "Murlok", Context: "m+", Note: "Refresh Build trends to capture a dated community reference."})
	}
	out.References = append(out.References, ReferenceEvidence{Source: "Archon", Context: "raid", Note: "Raid/boss usage data is not connected; no ranking was inferred."})
	if len(out.Views) == 0 {
		out.Note = "Run a fresh comparison for this equipped build; old gains are not reused after gear or talent changes."
	}
	return out, nil
}

func (w *Workspace) Decisions(snapshotID, context string) (*DecisionReport, error) {
	return buildDecisions(w.State(), snapshotID, context)
}
