package engine

import (
	_ "embed"
	"encoding/json"
	"math"
)

// Direct, passive, selected-talent effects from the pinned 12.1.0 game data.
// Proc-driven reductions, encounter resets, equipment and PvP overrides remain
// uncertain. A selected talent is not evidence of a proc or available charge.
//
//go:embed cooldown_rules.json
var cooldownRulesJSON []byte

type cooldownRule struct {
	Talent, Spell, MaxRank, Effect int
	Name, Mode                     string
	Value                          float64
}

var referenceRules struct {
	NonTalentSpells         map[int]bool
	SpecializationBaselines map[int]map[int]float64
	Game                    string
	Rules                   []cooldownRule
	Charges                 map[int]int
}

func init() {
	if err := json.Unmarshal(cooldownRulesJSON, &referenceRules); err != nil {
		panic(err)
	}
}

func applyBuildRules(e *CooldownEvidence, b BuildSnapshot) {
	if applyMageBuildRules(e, b) {
		return
	}
	if Forever || b.Source != "inspect" || b.SpellRanks == nil || b.GameBuild != referenceRules.Game {
		return
	}
	seconds, percent := 0.0, 1.0
	if baseline := referenceRules.SpecializationBaselines[b.SpecID][e.Spell]; baseline > 0 {
		e.BaselineSeconds = baseline
		e.Reasons = append(e.Reasons, "inspected_spec_baseline")
	}
	capacity := referenceRules.Charges[e.Spell]
	hasChargeRule := false
	for _, rule := range referenceRules.Rules {
		rank := b.SpellRanks[rule.Talent]
		if rule.Spell != e.Spell || rank <= 0 || rank > rule.MaxRank {
			continue
		}
		switch rule.Mode {
		case "seconds":
			seconds += rule.Value * float64(rank)
		case "percent":
			percent *= 1 + rule.Value*float64(rank)/100
		case "charges":
			hasChargeRule = true
			capacity += int(rule.Value) * rank
		}
		e.AppliedTalents = append(e.AppliedTalents, rule.Talent)
	}
	// Generic charge categories can gain charges through learned spell ranks,
	// which talent inspection does not enumerate. Keep that capacity unknown.
	if capacity >= 1 && capacity <= 4 && (hasChargeRule || !Catalog[e.Spell].Charges) {
		e.MaxCharges = &capacity
	}
	if capacity > 1 {
		e.MayHaveCharges = true
	}
	duration := math.Round(math.Max(0, e.BaselineSeconds+seconds)*percent*1000) / 1000
	if duration > 0 && duration <= 3600 {
		e.CooldownSeconds = duration
		e.EstimatedEnd = float64(e.EventAt.UnixMilli())/1000 + duration
	}
	if len(e.AppliedTalents) > 0 {
		e.Reasons = append(e.Reasons, "selected_talent_modifiers")
	}
	e.Reasons = append(e.Reasons, "dynamic_modifiers_unmodeled", "pvp_modifiers_unmodeled")
	filtered := e.Reasons[:0]
	for _, reason := range e.Reasons {
		if reason != "unmapped_build_modifiers" {
			filtered = append(filtered, reason)
		}
	}
	e.Reasons = filtered
}

func (e *CooldownEvidence) rechargeAfter(previous CooldownEvidence) {
	if (e.Kind != "cast" && !(e.Kind == "activation" && e.Spell == 45438)) || e.MaxCharges == nil || *e.MaxCharges <= 1 || e.CooldownSeconds <= 0 {
		return
	}
	start := float64(e.EventAt.UnixMilli()) / 1000
	if previous.BuildRevision == e.BuildRevision {
		for _, end := range previous.RechargeEnds {
			if end > start {
				e.RechargeEnds = append(e.RechargeEnds, end)
			}
		}
	}
	if len(e.RechargeEnds) >= *e.MaxCharges {
		// Evidence conflicts with the reference queue: a missed reset/reduction is
		// more plausible than inventing a negative charge or an extra-long lockout.
		e.RechargeEnds = nil
		e.Reasons = append(e.Reasons, "charge_queue_conflict")
	}
	if n := len(e.RechargeEnds); n > 0 {
		start = e.RechargeEnds[n-1]
	}
	e.RechargeEnds = append(e.RechargeEnds, start+e.CooldownSeconds)
	e.EstimatedEnd = e.RechargeEnds[0]
	e.Reasons = append(e.Reasons, "estimated_charge_queue")
}
