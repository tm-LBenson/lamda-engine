package healsim

import "math"

type groundEffect struct {
	spell, target     int
	next, end, period float64
}

func (s *simulation) spellIndex(spellID int) int {
	for i, sp := range s.spells {
		if sp.id == spellID {
			return i
		}
	}
	return -1
}
func (s *simulation) hotActive(target, spellID int) bool {
	id := s.spellIndex(spellID)
	h, ok := s.people[target].hots[id]
	return ok && h.end > s.now
}
func (s *simulation) applyRaw(id, target int, amount float64) float64 {
	p := &s.people[target]
	if p.dead || amount <= 0 {
		return 0
	}
	effective := math.Min(amount, p.max-p.hp)
	p.hp += effective
	s.totals[id].Healing += effective
	s.totals[id].Overheal += amount - effective
	return effective
}

// Secondary transfers use applyRaw so crit/mastery/versatility cannot be applied
// twice. Direct heals and HoTs use this path with explicit mastery behavior.
func (s *simulation) healCore(id, target int, amount float64, crit, direct bool) {
	p := &s.people[target]
	if p.dead {
		return
	}
	amount *= 1 + s.stats.Versatility
	switch s.spec {
	case 105:
		// Conservative first-stack coverage. The uncalibrated diminishing stack
		// curve is explicitly excluded instead of invented as a throughput boost.
		if len(p.hots) > 0 {
			amount *= 1 + s.stats.Mastery
		}
	case 256:
		if p.atonement > s.now {
			amount *= 1 + s.stats.Mastery
		}
	case 264:
		amount *= 1 + s.stats.Mastery*(1-p.hp/p.max)
	case 65:
		if s.spells[id].id != 53563 {
			amount *= 1 + s.stats.Mastery
		}
		if s.wings > s.now {
			amount *= 1.2
		}
	case 270:
		if s.hotActive(target, 124682) {
			amount *= 1.1
		}
	}
	chance := s.stats.Crit
	if s.spec == 65 && s.wings > s.now {
		chance += .2
	}
	if crit && s.healRNG.Float64() < chance {
		amount *= 2
	}
	if p.hymnEnd > s.now {
		amount *= 1 + .04*float64(p.hymn)
	}
	effective := s.applyRaw(id, target, amount)
	if s.spec == 257 && direct && s.spells[id].id != 77489 {
		p.masteryPool += amount * s.stats.Mastery
		if p.masteryEnd <= s.now {
			p.masteryNext = s.now + 2
		}
		p.masteryEnd = s.now + 4
	}
	if s.spec == 65 && direct && s.spells[id].id != 53652 {
		beacon := 1
		if target != beacon && s.hotActive(beacon, 53563) {
			s.applyRaw(s.spellIndex(53652), beacon, effective*.1)
		}
	}
}

func (s *simulation) absorbDamage(target int, amount float64) float64 {
	if s.spec == 1468 {
		return s.absorbTemporal(target, amount)
	}
	if s.spec != 256 {
		return amount
	}
	p := &s.people[target]
	if p.shieldEnd <= s.now {
		p.shield = 0
	}
	used := math.Min(amount, p.shield)
	p.shield -= used
	s.totals[s.spellIndex(17)].Absorbed += used
	return amount - used
}
func (s *simulation) damageTriggers(target int) {
	if s.spec != 257 {
		return
	}
	p := &s.people[target]
	if p.dead || p.pom <= 0 || p.pomEnd <= s.now {
		return
	}
	stacks := p.pom - 1
	p.pom = 0
	id := s.spellIndex(33076)
	s.healCore(id, target, s.spells[id].direct*s.stats.SpellPower, true, true)
	if stacks == 0 {
		return
	}
	for _, i := range s.targets(len(s.people)) {
		if i != target && s.people[i].pom == 0 {
			s.people[i].pom = stacks
			s.people[i].pomEnd = s.now + 30
			return
		}
	}
}
func (s *simulation) atonementDamage(id int) {
	amount := s.spells[id].damage * s.stats.SpellPower * (1 + s.stats.Versatility)
	if s.healRNG.Float64() < s.stats.Crit {
		amount *= 2
	}
	count := 0
	for _, p := range s.people {
		if !p.dead && p.atonement > s.now {
			count++
		}
	}
	if count == 0 {
		return
	}
	amount *= .46 * (1 + s.stats.Mastery)
	raid := s.options.Scenario == "raid" || s.options.Replay != nil && len(s.people) > 5
	if !raid {
		amount *= 1.4
	}
	if count > 5 {
		amount *= math.Sqrt(5 / float64(count))
	}
	row := s.spellIndex(81751)
	for i, p := range s.people {
		if !p.dead && p.atonement > s.now {
			s.applyRaw(row, i, amount)
		}
	}
}
func (s *simulation) castCore(id, target int, trace bool) {
	sp := s.spells[id]
	s.mana = math.Max(0, s.mana-s.cost(id))
	if s.spec == 65 && (sp.id == 85673 || sp.id == 85222) {
		s.holyPower -= 3
	}
	cd := sp.cd
	if sp.hastedCD {
		cd /= 1 + s.stats.Haste
	}
	for i, t := range s.ready[id] {
		if t <= s.now+1e-8 {
			last := s.now
			for _, other := range s.ready[id] {
				last = math.Max(last, other)
			}
			s.ready[id][i] = last + cd
			break
		}
	}
	duration := sp.cast / (1 + s.stats.Haste)
	s.nextAction = s.now + math.Max(.75, math.Max(1.5/(1+s.stats.Haste), duration)) + .15
	s.totals[id].Casts++
	if trace && len(s.result.Casts) < 500 {
		s.result.Casts = append(s.result.Casts, Cast{Time: s.now, Spell: sp.name, Target: target + 1})
	}
	if sp.channel {
		s.startGround(id, target)
	} else if duration > 0 {
		s.pending = append(s.pending, pending{s.now + duration, id, target})
	} else {
		s.finishCore(id, target)
	}
}
func (s *simulation) reduceCooldown(spellID int, seconds float64) {
	id := s.spellIndex(spellID)
	if id < 0 {
		return
	}
	for i := range s.ready[id] {
		s.ready[id][i] = math.Max(s.now, s.ready[id][i]-seconds)
	}
}
func (s *simulation) addCoreHot(id, target int) {
	sp := s.spells[id]
	p := &s.people[target]
	old, exists := p.hots[id]
	end := s.now + sp.duration
	next := s.now + sp.period/(1+s.stats.Haste)
	last := s.now
	if exists && old.end > s.now {
		end += math.Min(old.end-s.now, .3*sp.duration)
		next = old.next
		last = old.last
		if sp.bloom > 0 && old.end-s.now <= .3*sp.duration {
			s.healCore(id, target, sp.bloom*s.stats.SpellPower, true, true)
		}
	}
	p.hots[id] = hot{spell: id, factor: 1, next: next, end: end, last: last}
	if sp.initial && !exists {
		s.healCore(id, target, sp.hot*s.stats.SpellPower, true, false)
	}
}
func (s *simulation) finishCore(id, target int) {
	if s.people[0].dead {
		return
	}
	sp := s.spells[id]
	if sp.id == 31884 {
		s.wings = s.now + sp.duration
		return
	}
	if sp.ground {
		s.startGround(id, target)
		return
	}
	if sp.damage > 0 {
		s.atonementDamage(id)
		return
	}
	if s.people[target].dead {
		return
	}
	if s.spec == 105 && sp.id == 18562 {
		// Recheck at completion; use the shortest eligible HoT.
		used := -1
		end := math.Inf(1)
		for _, spellID := range []int{774, 8936, 48438} {
			key := s.spellIndex(spellID)
			if h, ok := s.people[target].hots[key]; ok && h.end > s.now && h.end < end {
				used = key
				end = h.end
			}
		}
		if used < 0 {
			return
		}
		if !s.has(392410) {
			delete(s.people[target].hots, used)
		}
	}
	if sp.id == 33076 {
		s.people[target].pom = 5
		s.people[target].pomEnd = s.now + 30
		return
	}
	if sp.id == 17 {
		p := &s.people[target]
		p.atonement = s.now + 15
		shield := sp.shield * s.stats.SpellPower * (1 + s.stats.Versatility) * (1 + s.stats.Mastery)
		if s.healRNG.Float64() < s.stats.Crit {
			shield *= 2
		}
		p.shield = shield
		p.shieldEnd = s.now + sp.duration
		return
	}
	targets := []int{target}
	if sp.targets > 1 {
		targets = s.targets(sp.targets)
	}
	if s.spec == 256 && sp.id == 194509 {
		// Prefer uncovered allies during a ramp; the explicitly selected target is included.
		targets = []int{target}
		for _, onlyFresh := range []bool{true, false} {
			for _, i := range s.targets(len(s.people)) {
				if len(targets) >= sp.targets {
					break
				}
				present := false
				for _, j := range targets {
					if j == i {
						present = true
					}
				}
				if !present && (!onlyFresh || s.people[i].atonement <= s.now) {
					targets = append(targets, i)
				}
			}
		}
	}
	if sp.id == 33763 {
		for i := range s.people {
			if i != target {
				delete(s.people[i].hots, id)
			}
		}
	}
	for jump, i := range targets {
		if s.spec == 256 {
			if sp.id == 2061 {
				s.people[i].atonement = s.now + 15
			}
			if sp.id == 194509 {
				s.people[i].atonement = s.now + 9
			}
		}
		factor := 1.
		if sp.chain > 0 {
			factor = math.Pow(sp.chain, float64(jump))
		}
		if sp.id == 115310 && len(targets) > 5 {
			factor *= math.Sqrt(5 / float64(len(targets)))
		}
		if sp.direct > 0 {
			s.healCore(id, i, sp.direct*s.stats.SpellPower*factor, true, true)
		}
		if sp.hot > 0 {
			s.addCoreHot(id, i)
		}
	}
	if s.spec == 257 {
		if sp.id == 2061 {
			s.reduceCooldown(2050, 6)
		}
		if sp.id == 596 {
			s.reduceCooldown(34861, 6)
		}
	}
	// Tower of Radiance (231642) is a level-10 Holy passive in the pinned
	// DBC, not a selected talent. Every supported level-90 Holy capture has it.
	if s.spec == 65 && (sp.id == 20473 || sp.id == 19750 || sp.id == 82326) {
		s.holyPower = math.Min(5, s.holyPower+1)
	}
	if s.spec == 270 {
		if sp.id == 115151 || sp.id == 124682 {
			s.healCore(s.spellIndex(191894), target, (.001+s.stats.Mastery)*.95*s.stats.SpellPower, true, true)
		}
		if sp.id == 116670 && s.has(274586) {
			idx := s.spellIndex(425804)
			for i := range s.people {
				if s.hotActive(i, 115151) {
					s.healCore(idx, i, s.spells[idx].direct*s.stats.SpellPower, true, true)
				}
			}
		}
	}
}

func (s *simulation) startGround(id, target int) {
	sp := s.spells[id]
	period := sp.period / (1 + s.stats.Haste)
	duration := sp.duration
	if sp.channel {
		duration /= 1 + s.stats.Haste
	}
	kept := s.ground[:0]
	for _, g := range s.ground {
		if g.spell != id {
			kept = append(kept, g)
		}
	}
	s.ground = kept
	g := groundEffect{spell: id, target: target, next: s.now + period, end: s.now + duration, period: period}
	if sp.initial {
		s.pulseCore(g)
	}
	s.ground = append(s.ground, g)
}
func (s *simulation) pulseCore(g groundEffect) {
	sp := s.spells[g.spell]
	if sp.channel && s.people[0].dead {
		return
	}
	if sp.damage > 0 {
		s.atonementDamage(g.spell)
		return
	}
	targets := s.targets(sp.targets)
	factor := 1.
	if sp.id == 740 && len(targets) > 5 {
		factor = math.Sqrt(5 / float64(len(targets)))
	}
	for _, i := range targets {
		s.healCore(g.spell, i, sp.hot*s.stats.SpellPower*factor, true, sp.id == 64843)
		if sp.id == 64843 {
			p := &s.people[i]
			if p.hymnEnd <= s.now {
				p.hymn = 0
			}
			p.hymn++
			p.hymnEnd = s.now + 15
		}
	}
}
func (s *simulation) tickCore() {
	if s.spec == 1468 {
		return
	}
	kept := s.ground[:0]
	for _, g := range s.ground {
		for g.next <= s.now+1e-8 && g.next <= g.end+1e-8 {
			s.pulseCore(g)
			g.next += g.period
		}
		if g.end > s.now+1e-8 && !(s.spells[g.spell].channel && s.people[0].dead) {
			kept = append(kept, g)
		}
	}
	s.ground = kept
	if s.spec == 257 {
		for i := range s.people {
			p := &s.people[i]
			if p.dead {
				p.masteryPool = 0
				continue
			}
			if p.masteryPool > 0 && p.masteryNext <= s.now+1e-8 {
				ticks := math.Max(1, math.Ceil((p.masteryEnd-p.masteryNext)/2)+1)
				amount := p.masteryPool / ticks
				p.masteryPool -= amount
				p.masteryNext += 2
				s.applyRaw(s.spellIndex(77489), i, amount)
			}
		}
	}
}

func (s *simulation) chooseCore() (int, int) {
	ids := s.targets(len(s.people))
	if len(ids) == 0 {
		return -1, 0
	}
	target := ids[0]
	low := s.people[target].hp / s.people[target].max
	injured := 0
	for _, i := range ids {
		if s.people[i].hp/s.people[i].max < .85 {
			injured++
		}
	}
	prep := false
	if s.policy == "consensus" {
		if s.options.Replay != nil {
			prep = s.replayBurstSoon()
		} else {
			phase := math.Mod(s.now, 20)
			prep = phase >= 6 && phase < 10
		}
	}
	can := func(id int) bool { return s.can(s.spellIndex(id)) }
	pick := func(id, t int) (int, int) { return s.spellIndex(id), t }
	uncovered := func(spellID int, threshold float64) int {
		for _, i := range ids {
			idx := s.spellIndex(spellID)
			h, ok := s.people[i].hots[idx]
			if !ok || h.end-s.now < threshold {
				return i
			}
		}
		return -1
	}
	groundUp := func(id int) bool {
		for _, g := range s.ground {
			if s.spells[g.spell].id == id && g.end > s.now {
				return true
			}
		}
		return false
	}
	switch s.spec {
	case 105:
		if low < .45 && can(18562) {
			for _, i := range ids {
				if s.hotActive(i, 774) || s.hotActive(i, 8936) || s.hotActive(i, 48438) {
					return pick(18562, i)
				}
			}
		}
		if injured >= 3 && low < .5 && can(740) {
			return pick(740, target)
		}
		if can(33763) && !s.people[1].dead {
			h, ok := s.people[1].hots[s.spellIndex(33763)]
			if !ok || h.end-s.now < 4 {
				return pick(33763, 1)
			}
		}
		if can(145205) && !groundUp(145205) && (injured > 0 || prep) {
			return pick(145205, target)
		}
		if injured >= 2 && can(48438) {
			return pick(48438, target)
		}
		if (prep || low < .8) && can(774) {
			if i := uncovered(774, 2); i >= 0 {
				return pick(774, i)
			}
		}
		if low < .9 && can(8936) {
			return pick(8936, target)
		}
	case 256:
		if low < .45 && can(2061) {
			return pick(2061, target)
		}
		if can(17) && (prep || low < .85) {
			for _, i := range ids {
				p := s.people[i]
				if p.shield <= 0 || p.shieldEnd-s.now < 2 {
					return pick(17, i)
				}
			}
		}
		fresh := 0
		for _, i := range ids {
			if s.people[i].atonement > s.now+3 {
				fresh++
			}
		}
		if (prep || injured >= 2) && fresh < len(ids) && can(194509) {
			return pick(194509, target)
		}
		if (prep || low < .85) && fresh == 0 && can(2061) {
			return pick(2061, target)
		}
		if fresh > 0 {
			if can(47540) {
				return pick(47540, target)
			}
			if can(585) {
				return pick(585, target)
			}
		}
		if low < .9 && can(2061) {
			return pick(2061, target)
		}
	case 257:
		if low < .55 && can(2050) {
			return pick(2050, target)
		}
		if injured >= 3 && low < .5 && can(64843) {
			return pick(64843, target)
		}
		if injured >= 3 && can(34861) {
			return pick(34861, target)
		}
		if can(33076) && (prep || low < .9) {
			for _, i := range ids {
				if s.people[i].pom <= 0 || s.people[i].pomEnd <= s.now {
					return pick(33076, i)
				}
			}
		}
		if injured >= 3 && can(596) {
			return pick(596, target)
		}
		if low < .9 && can(2061) {
			return pick(2061, target)
		}
	case 264:
		if low < .4 && can(8004) {
			return pick(8004, target)
		}
		if injured >= 3 && low < .5 && can(108280) {
			return pick(108280, target)
		}
		if can(5394) && !groundUp(5394) && (low < .9 || prep) {
			return pick(5394, target)
		}
		if can(61295) && (prep || low < .85) {
			if i := uncovered(61295, 3); i >= 0 {
				return pick(61295, i)
			}
		}
		if (injured >= 2 || prep) && can(73920) && !groundUp(73920) {
			return pick(73920, target)
		}
		if injured >= 3 && can(1064) {
			return pick(1064, target)
		}
		if low < .9 && can(77472) {
			return pick(77472, target)
		}
	case 270:
		if injured >= 3 && low < .5 && can(115310) {
			return pick(115310, target)
		}
		if low < .4 && can(116670) {
			return pick(116670, target)
		}
		if (prep || low < .95) && can(115151) {
			if i := uncovered(115151, 2); i >= 0 {
				return pick(115151, i)
			}
		}
		if low < .65 && can(124682) && !s.hotActive(target, 124682) {
			return pick(124682, target)
		}
		if low < .9 && can(116670) {
			return pick(116670, target)
		}
	case 65:
		if can(53563) && !s.people[1].dead && !s.hotActive(1, 53563) {
			return pick(53563, 1)
		}
		if (injured >= 3 || prep) && s.wings <= s.now && can(31884) {
			return pick(31884, target)
		}
		if injured >= 3 && can(85222) {
			return pick(85222, target)
		}
		if low < .85 && can(85673) {
			return pick(85673, target)
		}
		if low < .95 && can(20473) {
			return pick(20473, target)
		}
		if low < .45 && can(82326) {
			return pick(82326, target)
		}
		if (low < .9 || prep && s.holyPower < 3) && can(19750) {
			return pick(19750, target)
		}
	}
	return -1, target
}
