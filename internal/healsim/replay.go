package healsim

import "fmt"

// Replay is a bounded, normalized input, not a combat log or a WCL response.
// Target zero is the healer being replaced by the simulated character.
type ReplayMember struct {
	Label     string  `json:"label"`
	MaxHealth float64 `json:"maxHealth"`
}
type DamageEvent struct {
	Time   float64 `json:"time"`
	Target int     `json:"target"`
	Amount float64 `json:"amount"`
}
type Replay struct {
	Schema     int            `json:"schema"`
	Source     string         `json:"source"`
	Encounter  string         `json:"encounter"`
	Difficulty string         `json:"difficulty"`
	Patch      string         `json:"patch"`
	Seconds    float64        `json:"seconds"`
	Members    []ReplayMember `json:"members"`
	Events     []DamageEvent  `json:"events"`
	Warnings   []string       `json:"warnings"`
}

func (r *Replay) Validate() error {
	if r == nil || r.Schema != 1 || !finiteRange(r.Seconds, 30, 600) || len(r.Members) < 2 || len(r.Members) > 40 || len(r.Events) < 1 || len(r.Events) > 100000 || len(r.Source) > 500 || r.Source == "" || len(r.Encounter) > 200 || len(r.Difficulty) > 80 || len(r.Patch) > 40 || len(r.Warnings) > 20 {
		return fmt.Errorf("invalid or incomplete damage replay")
	}
	for _, p := range r.Members {
		if !finiteRange(p.MaxHealth, 1, 1e9) || len(p.Label) > 100 {
			return fmt.Errorf("replay requires a maximum health for every target")
		}
	}
	last := 0.
	for _, e := range r.Events {
		if !finiteRange(e.Time, last, r.Seconds) || e.Target < 0 || e.Target >= len(r.Members) || !finiteRange(e.Amount, 0, 1e10) {
			return fmt.Errorf("damage replay has an invalid target, amount, or event order")
		}
		last = e.Time
	}
	for _, w := range r.Warnings {
		if len(w) > 1000 {
			return fmt.Errorf("damage replay warning is too long")
		}
	}
	return nil
}

func (s *simulation) replayDamage(tick int) {
	for i := range s.people {
		s.people[i].damage[tick%len(s.people[i].damage)] = 0
	}
	for s.replayIndex < len(s.options.Replay.Events) {
		e := s.options.Replay.Events[s.replayIndex]
		if e.Time > s.now+1e-8 {
			break
		}
		s.replayIndex++
		p := &s.people[e.Target]
		if p.dead {
			continue
		}
		amount := s.absorbDamage(e.Target, e.Amount*float64(s.options.Intensity)/100)
		actual := amount
		if actual > p.hp {
			actual = p.hp
		}
		p.damage[tick%len(p.damage)] += actual
		p.hp -= actual
		if p.hp <= 0 {
			p.hp = 0
			p.dead = true
			p.hots = map[int]hot{}
			s.deaths++
		}
		if amount > 0 {
			s.damageTriggers(e.Target)
		}
	}
}

// The preparation policy has perfect knowledge of the recorded damage script.
// That is an explicit simulation assumption, not a live predictive feature.
func (s *simulation) replayBurstSoon() bool {
	if s.options.Training != nil {
		return s.options.Training.burstSoon(s.now, s.options.Intensity)
	}
	if s.options.forecast != nil {
		return s.options.forecast.burstSoon(s.now, s.options.Intensity)
	}
	damage := make([]float64, len(s.people))
	for i := s.replayIndex; i < len(s.options.Replay.Events); i++ {
		e := s.options.Replay.Events[i]
		if e.Time > s.now+5 {
			break
		}
		if e.Time >= s.now+1 {
			damage[e.Target] += e.Amount * float64(s.options.Intensity) / 100
		}
	}
	for i, amount := range damage {
		if !s.people[i].dead && amount/s.people[i].max >= .3 {
			return true
		}
	}
	return false
}
