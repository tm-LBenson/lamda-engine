// Package healsim implements Lamda's experimental time-domain healing model.
// It is independent of the fixed cast-profile gear scorer and has no game I/O.
package healsim

import (
	"fmt"
	"math"
)

const Version = "healers-12.1-alpha5"

// Stats are captured together, outside combat. Percentages use fractions.
// Missing stats must reject the run, rather than become a fabricated baseline.
type Stats struct {
	KnownSpells map[int]bool `json:"knownSpells,omitempty"`
	SpellPower  float64      `json:"spellPower"`
	Health      float64      `json:"health"`
	Mana        float64      `json:"mana"`
	ManaRegen   float64      `json:"manaRegen"`
	Crit        float64      `json:"crit"`
	Haste       float64      `json:"haste"`
	Mastery     float64      `json:"mastery"`
	Versatility float64      `json:"versatility"`
	Talents     map[int]int  `json:"talents"`
}

func finiteRange(v, lo, hi float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= lo && v <= hi
}
func (s Stats) Validate() error {
	if !finiteRange(s.SpellPower, 1, 1e8) || !finiteRange(s.Health, 1, 1e9) || !finiteRange(s.Mana, 1, 1e9) || !finiteRange(s.ManaRegen, 0, s.Mana) || !finiteRange(s.Crit, 0, 1) || !finiteRange(s.Haste, 0, 3) || !finiteRange(s.Mastery, 0, 100) || !finiteRange(s.Versatility, 0, 2) || (len(s.Talents) == 0 || len(s.Talents) > 256) {
		return fmt.Errorf("healing stats are missing or outside the model's limits; update your character outside combat")
	}
	if len(s.KnownSpells) > 128 {
		return fmt.Errorf("too many captured spells")
	}
	for id := range s.KnownSpells {
		if id <= 0 || id > 10000000 {
			return fmt.Errorf("invalid captured spell")
		}
	}
	for id, rank := range s.Talents {
		if id <= 0 || id > 10000000 || rank < 1 || rank > 10 {
			return fmt.Errorf("invalid healing talent capture")
		}
	}
	return nil
}

type Options struct {
	forecast  *EncounterTraining
	Training  *EncounterTraining `json:"-"`
	Spec      int                `json:"spec"`
	Replay    *Replay            `json:"-"`
	Scenario  string             `json:"scenario"`
	Seconds   int                `json:"seconds"`
	Intensity int                `json:"intensity"` // percentage of the named damage script
	Runs      int                `json:"runs"`
	Policy    string             `json:"policy"`
	Seed      int64              `json:"seed"`
}

func (o Options) Validate() error {
	if o.Spec != 0 && !Supported(o.Spec) {
		return fmt.Errorf("unsupported healer specialization")
	}
	if o.Scenario != "tank" && o.Scenario != "dungeon" && o.Scenario != "raid" && o.Scenario != "replay" && o.Scenario != "learned" {
		return fmt.Errorf("choose tank burst, dungeon group, raid damage, or log replay")
	}
	if o.Scenario == "learned" {
		if err := o.Training.Validate(); err != nil {
			return err
		}
	} else if o.Training != nil {
		return fmt.Errorf("unexpected learned encounter")
	}
	if o.Scenario == "replay" {
		if err := o.Replay.Validate(); err != nil {
			return err
		}
		if o.Seconds != int(math.Ceil(o.Replay.Seconds)) {
			return fmt.Errorf("replay duration must match the recorded encounter")
		}
	} else if o.Replay != nil {
		return fmt.Errorf("unexpected replay")
	}
	if o.Seconds < 30 || o.Seconds > 600 || o.Intensity < 10 || o.Intensity > 200 || o.Runs < 1 || o.Runs > 100 || (o.Policy != "consensus" && o.Policy != "reactive" && o.Policy != "compare") {
		return fmt.Errorf("invalid healing encounter settings")
	}
	return nil
}

type SpellResult struct {
	Name     string  `json:"name"`
	Casts    int     `json:"casts"`
	Replays  int     `json:"replays,omitempty"`
	Healing  float64 `json:"healing"`
	Overheal float64 `json:"overheal"`
	Absorbed float64 `json:"absorbed"`
}
type Point struct {
	ResourceName string    `json:"resourceName,omitempty"`
	Resource     float64   `json:"resource,omitempty"`
	Time         float64   `json:"time"`
	Health       []float64 `json:"health"` // percentages; zero remains dead
	Mana         float64   `json:"mana"`
	Essence      float64   `json:"essence"`
}
type Cast struct {
	Origin string  `json:"origin,omitempty"`
	Time   float64 `json:"time"`
	Spell  string  `json:"spell"`
	Target int     `json:"target"`
}
type Outcome struct {
	Policy             string        `json:"policy"`
	HPS                float64       `json:"hps"`
	RawHPS             float64       `json:"rawHps"`
	AbsorbPS           float64       `json:"absorbPs"`
	StdDev             float64       `json:"stdDev"`
	MeanError          float64       `json:"meanError"` // approximate 95% Monte Carlo interval, not model accuracy
	OverhealPercent    float64       `json:"overhealPercent"`
	Deaths             float64       `json:"deaths"`
	SurvivalPercent    float64       `json:"survivalPercent"` // runs with no deaths
	CriticalSeconds    float64       `json:"criticalSeconds"` // at least one living ally below 30%
	ManaEnd            float64       `json:"manaEnd"`
	ManaStarvedSeconds float64       `json:"manaStarvedSeconds"`
	Spells             []SpellResult `json:"spells"`
	Timeline           []Point       `json:"timeline"` // first seed, not an average health trajectory
	Casts              []Cast        `json:"casts"`    // bounded first-seed trace
}
type Report struct {
	HoldoutOutcomes   []Outcome         `json:"holdoutOutcomes,omitempty"`
	Training          *TrainingSummary  `json:"training,omitempty"`
	LearnedEncounters []TrainingSummary `json:"learnedEncounters,omitempty"`
	Workloads         []string          `json:"workloads,omitempty"`
	Spec              int               `json:"spec"`
	SpecName          string            `json:"specName"`
	Replay            *Replay           `json:"replay,omitempty"` // metadata only; events omitted from result envelope
	Version           string            `json:"version"`
	Options           Options           `json:"options"`
	Stats             Stats             `json:"stats"`
	GroupSize         int               `json:"groupSize"`
	Outcomes          []Outcome         `json:"outcomes"`
	Warnings          []string          `json:"warnings"`
	ModeledTalents    []int             `json:"modeledTalents"`
}
