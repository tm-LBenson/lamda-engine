package healsim

import "math"

// Pinned 12.1 DBC: 373861/373862 and 1291636; 356810 effect 12
// supplies hasted cooldowns. Geometry and additional raid targets are not
// inferred from a health-only workload. See the report's coverage notes.
type temporalShield struct {
	spell       int
	amount, end float64
}

func (s *simulation) temporalAmount(id, target int) float64 {
	amount := s.spells[id].shield * s.stats.SpellPower * (1 + s.stats.Versatility)
	p := &s.people[target]
	if p.hp/p.max <= s.people[0].hp/s.people[0].max {
		amount *= 1 + s.stats.Mastery
	}
	// These absorbs do not roll a healing crit. Other percentage modifiers
	// need matched-stat calibration; do not silently reuse the direct-heal path.
	return amount
}

func (s *simulation) echoMultiplier() float64 {
	// Time Lord has two ranks: +25% / +50%, not a one-point toggle.
	return 1 + .25*float64(s.stats.Talents[372527])
}

func (s *simulation) temporalEchoStrength() float64 {
	return .3 * s.echoMultiplier()
}

func (s *simulation) temporalTargets(id, target int) []int {
	primary := target
	if id == anomaly {
		primary = 0 // Anomaly originates on the caster.
	}
	if s.people[primary].dead {
		return nil
	}
	ids := []int{primary}
	for _, i := range s.targets(len(s.people)) {
		if i != primary {
			ids = append(ids, i)
		}
		if len(ids) == s.spells[id].targets {
			break
		}
	}
	return ids
}

func (s *simulation) temporalUseful(id, target int) bool {
	needed := 0
	for _, i := range s.temporalTargets(id, target) {
		p := &s.people[i]
		amount := 0.
		for _, sh := range p.temporalShields {
			if sh.end > s.now {
				amount += sh.amount
			}
		}
		if p.echoEnd <= s.now || amount < .5*s.temporalAmount(id, i) {
			needed++
		}
	}
	return needed >= 2
}

func (s *simulation) addTemporalShield(id, target int) {
	p := &s.people[target]
	amount := s.temporalAmount(id, target)
	end := s.now + s.spells[id].duration
	for i := range p.temporalShields {
		sh := &p.temporalShields[i]
		if sh.spell == id {
			if sh.end <= s.now {
				sh.amount = 0
			}
			sh.amount += amount
			sh.end = end
			return
		}
	}
	p.temporalShields = append(p.temporalShields, temporalShield{id, amount, end})
}

func (s *simulation) finishTemporal(id, target int) {
	ids := s.temporalTargets(id, target)
	for _, i := range ids {
		s.addTemporalShield(id, i)
	}
	if id == anomaly {
		// The orb may pass over stronger Echoes and apply to other allies.
		// Additional shields beyond the first five are deliberately omitted.
		ids = s.targets(len(s.people))
	}
	strength := s.temporalEchoStrength()
	remaining := s.spells[id].targets
	for _, i := range ids {
		p := &s.people[i]
		if p.echoEnd > s.now && p.echo > strength+1e-8 {
			continue
		}
		p.echo = strength
		p.echoEnd = s.now + 15*(1+.15*float64(s.stats.Talents[376240]))
		remaining--
		if remaining == 0 {
			break
		}
	}
	if id == anomaly && s.has(376237) {
		// Reduces remaining recharge, not the base CD, and works on replay.
		s.reduceCooldown(355936, 5)
	}
}

func (s *simulation) absorbTemporal(target int, amount float64) float64 {
	p := &s.people[target]
	kept := p.temporalShields[:0]
	for _, sh := range p.temporalShields {
		if sh.end <= s.now || sh.amount <= 0 {
			continue
		}
		used := math.Min(amount, sh.amount)
		sh.amount -= used
		amount -= used
		s.totals[sh.spell].Absorbed += used
		if sh.amount > 0 {
			kept = append(kept, sh)
		}
	}
	p.temporalShields = kept
	return amount
}
