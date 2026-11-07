package simming

import (
	"fmt"
	"strings"
)

// Encounter is a named, reproducible set of conditions. Targets is an actual
// target count; TargetResult.Targets remains the legacy in-game view selector.
type Encounter struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FightStyle string `json:"fightStyle"`
	Targets    int    `json:"targets"`
	Seconds    int    `json:"seconds"`
	RaidBuffs  string `json:"raidBuffs"`
	Bloodlust  string `json:"bloodlust,omitempty"`
}

func (e Encounter) Validate() error {
	if !word.MatchString(e.ID) || len(e.Name) == 0 || len(e.Name) > 80 || strings.ContainsAny(e.Name, "\r\n\x00") {
		return fmt.Errorf("encounter needs a valid ID and short name")
	}
	if e.FightStyle != "patchwerk" && e.FightStyle != "dungeon_slice" {
		return fmt.Errorf("unknown encounter fight style")
	}
	if e.RaidBuffs != "none" && e.RaidBuffs != "full" {
		return fmt.Errorf("unknown encounter raid buffs")
	}
	if e.Bloodlust != "" && e.Bloodlust != "on" && e.Bloodlust != "off" {
		return fmt.Errorf("unknown encounter Bloodlust setting")
	}
	if e.Targets < 1 || e.Targets > 20 || e.Seconds < 30 || e.Seconds > 600 {
		return fmt.Errorf("encounter requires 1–20 targets and 30–600 seconds")
	}
	if e.FightStyle == "dungeon_slice" && (e.Targets != 1 || e.Seconds != 360 || e.RaidBuffs != "none" || e.Bloodlust == "on") {
		return fmt.Errorf("Dungeon Slice uses its fixed six-minute self-buffed model without forced Bloodlust")
	}
	return nil
}

func validateWorkflowScenario(s Scenario) error {
	if s.Bloodlust != "" && s.Bloodlust != "on" && s.Bloodlust != "off" {
		return fmt.Errorf("unknown Bloodlust setting")
	}
	if s.FightStyle == "dungeon_slice" && s.Bloodlust == "on" {
		return fmt.Errorf("Dungeon Slice cannot force Bloodlust")
	}
	if len(s.Encounters) > 4 || (len(s.Encounters) > 0 && s.Task != "" && s.Task != "analysis") {
		return fmt.Errorf("choose up to four encounters for analysis")
	}
	seen := map[string]bool{}
	for _, e := range s.Encounters {
		if err := e.Validate(); err != nil {
			return err
		}
		if seen[e.ID] {
			return fmt.Errorf("duplicate encounter ID")
		}
		seen[e.ID] = true
	}
	if len(s.ItemOptions) > 18 || (len(s.ItemOptions) > 0 && s.CompareMode != "topgear" && s.CompareMode != "loadout") {
		return fmt.Errorf("item alternatives require Top Gear or a loadout")
	}
	for id, options := range s.ItemOptions {
		if !requestID.MatchString(id) || len(options) == 0 || len(options) > 8 {
			return fmt.Errorf("choose up to eight alternatives per selected item")
		}
		for _, option := range options {
			if err := option.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func analysisEncounters(p *Profile, s Scenario) ([]Encounter, error) {
	if len(s.Encounters) > 0 {
		for _, e := range s.Encounters {
			if p.Spec == "augmentation" && e.FightStyle != "patchwerk" {
				return nil, fmt.Errorf("Augmentation requires stationary targets with the fixed ally model")
			}
		}
		return append([]Encounter{}, s.Encounters...), nil
	}
	buffs := s.RaidBuffs
	if buffs == "" {
		buffs = "full"
	}
	if p.Spec == "augmentation" {
		return []Encounter{{"raid", "Single target", "patchwerk", 1, s.Seconds, "full", s.Bloodlust}, {"aoe", "Five targets", "patchwerk", 5, s.Seconds, "full", s.Bloodlust}}, nil
	}
	return []Encounter{{"raid", "Raid boss", "patchwerk", 1, 300, buffs, s.Bloodlust}, {"dungeon", "Dungeon Slice", "dungeon_slice", 1, 360, "none", ""}}, nil
}

func (e Encounter) apply(s Scenario) Scenario {
	s.FightStyle, s.Targets, s.Seconds, s.RaidBuffs, s.Bloodlust = e.FightStyle, e.Targets, e.Seconds, e.RaidBuffs, e.Bloodlust
	s.Encounters = nil
	return s
}
