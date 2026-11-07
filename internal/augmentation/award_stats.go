package augmentation

import (
	"strconv"
	"time"
)

const awardStatsVersion = 2

func (s *Store) trackAwardStats(seg *segment, f []string) {
	d := seg.damage
	if len(f) < 9 {
		return
	}
	s.trackDetailedMechanics(seg, f)
	s.trackEncounterAwards(seg, f)
	s.trackVoidscarAwards(seg, f)
	s.trackTempleAwards(seg, f)
	if f[0] == "UNIT_DIED" && player(f[5]) && friendlyGroup(f[7]) {
		if d.firstDeathAt.IsZero() {
			d.firstDeathAt = s.last
		}
		if s.last.Sub(d.firstDeathAt) <= 100*time.Millisecond {
			d.firstDeaths[f[5]] = 1
		}
	}
	// A stack increase above four is an explicit wrong Helical Toxins merge,
	// not the normal damage every carrier takes. Count once per puzzle/player.
	if seg.kind == "boss" && seg.id == "3445" && len(f) >= 13 && f[9] == "1284590" && player(f[5]) && friendlyGroup(f[7]) {
		if f[0] == "SPELL_AURA_APPLIED" {
			delete(d.mathFailed, f[5])
		}
		if f[0] == "SPELL_AURA_APPLIED_DOSE" && len(f) > 13 {
			n, err := strconv.Atoi(f[13])
			if err == nil && n > 4 && !d.mathFailed[f[5]] {
				d.mathFails[f[5]]++
				d.mathFailed[f[5]] = true
			}
		}
	}
	if !friendlyGroup(f[3]) {
		return
	}
	switch f[0] {
	case "SPELL_INTERRUPT":
		if !friendlyGroup(f[7]) {
			d.interrupts[f[1]]++
		}
	case "SPELL_RESURRECT":
		if player(f[5]) && friendlyGroup(f[7]) {
			d.resurrections[f[1]]++
		}
	case "SPELL_HEAL", "SPELL_PERIODIC_HEAL":
		if !friendlyGroup(f[7]) {
			return
		}
		// Advanced Retail heal payload: amount, total, overheal, absorbed, crit.
		if len(f) < 36 {
			d.healingInvalid = true
			return
		}
		amount, e1 := strconv.ParseInt(f[31], 10, 64)
		over, e2 := strconv.ParseInt(f[33], 10, 64)
		if e1 != nil || e2 != nil || amount < 0 || over < 0 || amount > 1e12 || over > amount {
			d.healingInvalid = true
			return
		}
		d.healing[f[1]] += amount - over
		d.overhealing[f[1]] += over
		if player(f[5]) {
			if d.healTargets == nil {
				d.healTargets = map[healingTarget]healingAmounts{}
			}
			key := healingTarget{f[1], f[5]}
			v := d.healTargets[key]
			v.effective += amount - over
			v.over += over
			d.healTargets[key] = v
		}
	}
}

func copyCounts(m map[string]int64) map[string]int64 {
	v := map[string]int64{}
	for id, n := range m {
		v[id] = n
	}
	return v
}

func (s *Store) awardCountView(seg *segment, c *summaryCounts) {
	s.finishBombDrops(seg)
	c.AwardVersion, c.HealingKnown = awardStatsVersion, !seg.damage.healingInvalid
	c.EncounterAwardVersion = encounterAwardVersion
	s.healingRoleCountView(seg, c)
	c.Healing, c.Overhealing = map[string]int64{}, map[string]int64{}
	c.Interrupts, c.Resurrections = map[string]int64{}, map[string]int64{}
	for _, pair := range []struct {
		from, to map[string]int64
		healing  bool
	}{
		{seg.damage.healing, c.Healing, true}, {seg.damage.overhealing, c.Overhealing, true},
		{seg.damage.interrupts, c.Interrupts, false}, {seg.damage.resurrections, c.Resurrections, false},
	} {
		for source, n := range pair.from {
			owner := s.sourceOwner(source)
			if owner == "" {
				if pair.healing {
					c.HealingKnown = false
				} else {
					c.AwardVersion = 0
				}
				continue
			}
			pair.to[owner] += n
		}
	}
	c.FirstDeaths, c.AvoidableHits, c.MathFails = copyCounts(seg.damage.firstDeaths), copyCounts(seg.damage.avoidableHits), copyCounts(seg.damage.mathFails)
	c.MechanicTotals = map[string]map[string]int64{}
	c.MechanicUnknown = map[string]bool{}
	for key, unknown := range seg.damage.unknownMechanics {
		c.MechanicUnknown[key] = unknown
	}
	for key, counts := range seg.damage.mechanicTotals {
		values := map[string]int64{}
		for source, n := range counts {
			owner := s.sourceOwner(source)
			if owner == "" {
				c.AwardVersion = 0
				continue
			}
			values[owner] += n
		}
		c.MechanicTotals[key] = values
	}
}

func upgradeAwardCounts(current, newer *summaryCounts) *summaryCounts {
	if newer == nil {
		return current
	}
	if current == nil {
		return nil
	}
	c := *current
	if newer.EncounterAwardVersion > c.EncounterAwardVersion {
		c.EncounterAwardVersion = newer.EncounterAwardVersion
		c.MechanicTotals, c.MechanicUnknown = newer.MechanicTotals, newer.MechanicUnknown
	}
	if newer.OverhealRoleVersion > c.OverhealRoleVersion {
		c.OverhealRoleVersion, c.HealingRoles = newer.OverhealRoleVersion, newer.HealingRoles
		c.HealingOthers, c.OverhealingOthers = newer.HealingOthers, newer.OverhealingOthers
	}
	if newer.FishKnown && !c.FishKnown {
		c.FishKnown, c.Fish = true, newer.Fish
	}
	if newer.AwardVersion > c.AwardVersion {
		c.MechanicTotals = newer.MechanicTotals
		c.MechanicUnknown = newer.MechanicUnknown
		c.AwardVersion, c.HealingKnown = newer.AwardVersion, newer.HealingKnown
		c.Healing, c.Overhealing, c.Interrupts, c.Resurrections = newer.Healing, newer.Overhealing, newer.Interrupts, newer.Resurrections
		c.FirstDeaths, c.AvoidableHits, c.MathFails = newer.FirstDeaths, newer.AvoidableHits, newer.MathFails
	}
	return &c
}

func initAggregateAwardCounts(c *summaryCounts) {
	c.EncounterAwardVersion = encounterAwardVersion
	c.OverhealRoleVersion = overhealRoleVersion
	c.HealingRoles = map[string]string{}
	c.HealingOthers = map[string]int64{}
	c.OverhealingOthers = map[string]int64{}
	c.MechanicUnknown = map[string]bool{}
	c.MechanicTotals = map[string]map[string]int64{}
	c.AwardVersion, c.HealingKnown = awardStatsVersion, true
	c.Healing, c.Overhealing, c.Interrupts, c.Resurrections = map[string]int64{}, map[string]int64{}, map[string]int64{}, map[string]int64{}
	c.FirstDeaths, c.AvoidableHits, c.MathFails = map[string]int64{}, map[string]int64{}, map[string]int64{}
}

func addAwardCounts(c, p *summaryCounts) {
	c.EncounterAwardVersion = min(c.EncounterAwardVersion, p.EncounterAwardVersion)
	c.OverhealRoleVersion = min(c.OverhealRoleVersion, p.OverhealRoleVersion)
	for id, role := range p.HealingRoles {
		if old, ok := c.HealingRoles[id]; ok && old != role {
			c.HealingRoles[id] = "MIXED"
		} else {
			c.HealingRoles[id] = role
		}
	}
	for key, unknown := range p.MechanicUnknown {
		c.MechanicUnknown[key] = c.MechanicUnknown[key] || unknown
	}
	for key, counts := range p.MechanicTotals {
		if c.MechanicTotals[key] == nil {
			c.MechanicTotals[key] = map[string]int64{}
		}
		for id, n := range counts {
			c.MechanicTotals[key][id] += n
		}
	}
	c.AwardVersion = min(c.AwardVersion, p.AwardVersion)
	c.HealingKnown = c.HealingKnown && p.HealingKnown
	for _, pair := range []struct{ to, from map[string]int64 }{
		{c.HealingOthers, p.HealingOthers}, {c.OverhealingOthers, p.OverhealingOthers},
		{c.Healing, p.Healing}, {c.Overhealing, p.Overhealing}, {c.Interrupts, p.Interrupts}, {c.Resurrections, p.Resurrections},
		{c.FirstDeaths, p.FirstDeaths}, {c.AvoidableHits, p.AvoidableHits}, {c.MathFails, p.MathFails},
	} {
		for id, n := range pair.from {
			pair.to[id] += n
		}
	}
}
