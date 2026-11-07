package simming

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

type PlanOptions struct {
	AssumeBonusPool  bool               `json:"assumeBonusPool"`
	HistoryConfirmed bool               `json:"historyConfirmed"`
	Mode             string             `json:"mode"`
	View             string             `json:"view"`
	Chance           float64            `json:"chance"`
	Cost             float64            `json:"cost"`
	Exclude          []int              `json:"exclude"`
	Currency         int                `json:"currency"`
	History          *BonusRoll         `json:"history,omitempty"`
	Weights          map[string]float64 `json:"weights,omitempty"`
}
type SourcePlan struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Pool          int     `json:"pool"`
	Excluded      int     `json:"excluded"`
	UpgradeCount  int     `json:"upgradeCount"`
	BISCount      int     `json:"bisCount"`
	Expected      float64 `json:"expected"`
	Rate          float64 `json:"rate"`
	UpgradeChance float64 `json:"upgradeChance"`
	Attempts50    *int    `json:"attempts50,omitempty"`
}
type AcquisitionPlan struct {
	JobID      string       `json:"jobID"`
	At         time.Time    `json:"at"`
	Options    PlanOptions  `json:"options"`
	Sources    []SourcePlan `json:"sources"`
	Warnings   []string     `json:"warnings"`
	Balance    *int         `json:"balance,omitempty"`
	Affordable *int         `json:"affordable,omitempty"`
}

func planAcquisition(job *Job, profile *Profile, spec int, o PlanOptions) (*AcquisitionPlan, error) {
	if job == nil || job.Result == nil || len(job.Result.Upgrades) == 0 {
		return nil, fmt.Errorf("run an upgrade comparison first")
	}
	if job.Result.Provider == "SimulationCraft" && job.Result.InputRevision != 2 {
		return nil, fmt.Errorf("rerun this comparison to verify its item import before planning")
	}
	if (o.Mode != "farm" && o.Mode != "reroll") || (o.View != "drop" && o.View != "max" && o.View != "bonus") || math.IsNaN(o.Chance) || o.Chance < 0 || o.Chance > 1 || math.IsNaN(o.Cost) || math.IsInf(o.Cost, 0) || o.Cost < 1 || o.Cost > 10000 || len(o.Exclude) > 512 || len(o.Weights) > 1024 {
		return nil, fmt.Errorf("invalid planning assumptions")
	}
	if o.Mode == "reroll" && o.View != "bonus" {
		return nil, fmt.Errorf("reroll planning requires bonus-reward outcomes")
	}
	if o.Mode == "reroll" && !o.AssumeBonusPool {
		return nil, fmt.Errorf("confirm the journal-pool assumption before estimating rerolls")
	}
	plan := &AcquisitionPlan{JobID: job.ID, At: time.Now().UTC(), Options: o, Sources: []SourcePlan{}, Warnings: []string{"Estimates use the entered equipment-reward chance and cost. Non-equipment rewards are excluded. Unspecified item weights are equal; Blizzard's exact item probabilities are not known."}}
	if o.Mode == "reroll" {
		plan.Warnings = append(plan.Warnings, "Bonus reward eligibility and duplicate rules are assumptions; the journal supplies regular loot eligibility only.")
		for _, c := range profile.Planning.Currencies {
			if c.ID == o.Currency {
				balance := c.Quantity
				affordable := int(float64(balance) / o.Cost)
				plan.Balance = &balance
				plan.Affordable = &affordable
			}
		}
		if plan.Balance == nil {
			plan.Warnings = append(plan.Warnings, "Currency balance is unknown. Export a fresh snapshot to update it.")
		}
	}
	r := job.Result
	excluded := map[int]bool{}
	for _, id := range o.Exclude {
		if id < 1 {
			return nil, fmt.Errorf("invalid excluded item")
		}
		excluded[id] = true
	}
	for key, weight := range o.Weights {
		if len(key) > 160 || math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 10000 {
			return nil, fmt.Errorf("invalid item weight")
		}
	}
	historySource := ""
	if o.Mode == "reroll" && o.History != nil {
		if !o.HistoryConfirmed {
			return nil, fmt.Errorf("confirm this history belongs to the shown reward pool")
		}
		h := o.History
		if h.Spec != spec || h.Currency != o.Currency {
			return nil, fmt.Errorf("recorded pool belongs to a different spec or currency")
		}
		historySource = r.RollSources[strconv.Itoa(h.Source)]
		if historySource == "" {
			return nil, fmt.Errorf("this recorded source is not mapped by the calculator")
		}
		found := false
		for _, row := range profile.Planning.Rolls {
			if sameRollPool(row, *h) {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("recorded roll pool is not in this snapshot")
		}
		plan.Warnings = append(plan.Warnings, "Recorded history may be incomplete. Check the remaining reward pool in game before spending currency.")
	}
	groups := map[string]map[int]Upgrade{}
	incomplete := map[string]bool{}
	for _, id := range r.IncompleteSources {
		incomplete[id] = true
	}
	for _, item := range r.Upgrades {
		if item.Type != o.View || item.Kind == "Crafted" {
			continue
		}
		if !item.PoolComplete || incomplete[item.SourceID] {
			incomplete[item.SourceID] = true
			continue
		}
		if historySource != "" && item.SourceID != historySource {
			continue
		}
		if groups[item.SourceID] == nil {
			groups[item.SourceID] = map[int]Upgrade{}
		}
		prev, ok := groups[item.SourceID][item.ID]
		if !ok || item.Gain > prev.Gain {
			groups[item.SourceID][item.ID] = item
		}
	}
	rare := false
	for source, pool := range groups {
		if incomplete[source] {
			continue
		}
		for _, item := range pool {
			if item.PoolSize != len(pool) || math.IsNaN(item.Gain) || math.IsInf(item.Gain, 0) {
				incomplete[source] = true
			}
		}
		if incomplete[source] {
			continue
		}
		counts := map[int]int{}
		if historySource == source {
			for _, row := range profile.Planning.Rolls {
				if sameRollPool(row, *o.History) {
					if _, ok := pool[row.ItemID]; !ok {
						return nil, fmt.Errorf("recorded history contains an item outside this reward pool; capture again or leave history unapplied")
					}
					counts[row.ItemID]++
				}
			}
			// Exporter sorts history, so use complete per-item cycles rather than order.
			// Never remove an item merely because it is equipped or in the bags.
			cycles := math.MaxInt
			for id := range pool {
				cycles = min(cycles, counts[id])
			}
			for id := range counts {
				counts[id] -= cycles
			}
		}
		result := SourcePlan{ID: source}
		sum, weighted, gainWeight := 0.0, 0.0, 0.0
		for id, item := range pool {
			result.Name = item.Source
			if o.Mode == "reroll" && (excluded[id] || counts[id] > 0) {
				result.Excluded++
				continue
			}
			result.Pool++
			weight, ok := o.Weights[source+"/"+strconv.Itoa(id)]
			if !ok {
				weight = 1
				rare = rare || item.Rare
			}
			sum += weight
			weighted += weight * math.Max(0, item.Gain)
			if item.Gain > 0 {
				gainWeight += weight
				result.UpgradeCount++
			}
			if item.BIS {
				result.BISCount++
			}
		}
		if sum <= 0 {
			continue
		}
		result.Expected = weighted / sum * o.Chance
		result.Rate = result.Expected / o.Cost
		if o.Mode == "farm" {
			result.Rate *= 60
		}
		result.UpgradeChance = gainWeight / sum * o.Chance
		if o.Mode == "farm" && result.UpgradeChance >= 1 {
			n := 1
			result.Attempts50 = &n
		} else if o.Mode == "farm" && result.UpgradeChance > 0 {
			n := math.Ceil(math.Log(0.5) / math.Log1p(-result.UpgradeChance))
			if n < 1e9 {
				count := int(n)
				result.Attempts50 = &count
			}
		}
		plan.Sources = append(plan.Sources, result)
	}
	if rare {
		plan.Warnings = append(plan.Warnings, "Some pools contain rare items. Equal-weight estimates can overstate their value.")
	}
	if len(incomplete) > 0 {
		plan.Warnings = append(plan.Warnings, "Unverified or incomplete loot pools are excluded. Capture current loot tables in WoW and calculate again.")
	}
	sort.Slice(plan.Sources, func(i, j int) bool {
		if plan.Sources[i].Rate == plan.Sources[j].Rate {
			return plan.Sources[i].ID < plan.Sources[j].ID
		}
		return plan.Sources[i].Rate > plan.Sources[j].Rate
	})
	return plan, nil
}
func sameRollPool(a, b BonusRoll) bool {
	return a.Currency == b.Currency && a.Source == b.Source && a.Context == b.Context && a.KeyLevel == b.KeyLevel && a.Spec == b.Spec
}

func (w *Workspace) Plan(id string, o PlanOptions) (*AcquisitionPlan, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	job := w.jobs[id]
	if job == nil {
		return nil, fmt.Errorf("run not found")
	}
	s := w.snapshots[job.SnapshotID]
	if s == nil {
		return nil, fmt.Errorf("snapshot not found")
	}
	p, err := ParseProfile(s.Profile)
	if err != nil {
		return nil, err
	}
	return planAcquisition(job, p, s.Character.Spec, o)
}
