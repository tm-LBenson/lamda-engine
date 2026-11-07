package healsim

import (
	"fmt"
	"math"
)

// Core facts: pinned 12.1.0.69933 spell IDs 363534, 370537/370564,
// 366155/367364, 376239, 376240, 373834, 371257, 381922, 376150,
// 375722 and 1264269. Timing assumptions are surfaced in every report.
// Four saved Retail Stasis releases showed adjacent copies 0.409–0.474s apart.
// Use a 0.4s nominal interval; server latency is not simulated.
const stasisReplayInterval = .4

type storedSpell struct{ spell, target int }
type storedReplay struct {
	storedSpell
	at float64
}
type stasisState struct {
	loading bool
	spells  []storedSpell
	expires float64
}
type rewindHot struct {
	target, ticks        int
	amount, next, period float64
}

func validatePreservation(stats Stats) error {
	limits := map[int]int{376240: 2, 371257: 2, 376930: 2, 375554: 2, 372527: 2, 376239: 1, 373834: 1, 381922: 1, 376150: 1, 375722: 1, 1264269: 1, 373861: 1, 1291636: 1, 376237: 1, 431715: 1, 1239581: 1, 1264321: 1, 444845: 1, 1218447: 1}
	for id, max := range limits {
		if stats.Talents[id] > max {
			return fmt.Errorf("captured talent %d exceeds its modeled rank limit", id)
		}
	}
	if stats.KnownSpells == nil {
		return nil
	} // Legacy alpha1/alpha2 captures.
	if stats.KnownSpells[373861] && stats.KnownSpells[1291636] {
		return fmt.Errorf("Temporal Barrier replaces Temporal Anomaly; update your character to capture the active spell")
	}
	useful := false
	for i, sp := range spells {
		if sp.passive || i == stasisRelease {
			continue
		}
		known, exists := stats.KnownSpells[sp.id]
		// Original seven entries are mandatory. Older captures may lack the newly
		// added cooldowns; those remain excluded until recaptured, never inferred.
		if !exists && i <= flight {
			return fmt.Errorf("update your character outside combat to capture the complete Preservation spellbook")
		}
		if known && i != stasis {
			useful = true
		}
	}
	if !useful {
		return fmt.Errorf("none of this capture's Preservation healing spells are available in the model")
	}
	return nil
}
func preservationWarnings(stats Stats) []string {
	out := []string{
		"Preservation additions: Stasis stores three completed core healing spells, replays them free in order on their original direct targets, and automatically releases after 30 seconds. Cooldown starts on the third stored spell. Copies use a nominal 0.4-second gap, informed by four saved-log sequences (0.409–0.474s); server latency and cast interruption on automatic release are omitted. Transient proc-buff interactions remain uncalibrated.",
		"Rewind snapshots five seconds of health damage, uses a 2 x spell-power minimum, and heals in five equal pulses from 0 through 4 seconds (hasted). It uses 30% in raids and triple healing outside raids, without crit/mastery/versatility/healing multipliers. Snapshot timing and the minimum are pinned DBC interpretations pending live calibration.",
		"Reversion crits extend that variant by 2 seconds, with a modeled 12-second total extension budget per application. Reversion and Dream Breath preserve tick phase and carry up to 30% of base duration on refresh; normal and Echo variants remain separate. Extension/refresh boundaries need live calibration.",
		"Temporal shields use 6 x spell power for Anomaly and 11.7 x for Barrier, versatility and Life-Binder at application, without crit. Equal-health and self targets qualify for Life-Binder. Other shield-specific healing modifiers and stack caps remain uncalibrated. Shields stack and refresh their duration; only damage actually absorbed counts as Absorb/s, never HPS.",
		"Temporal coverage assumes reachable allies at cast completion: Anomaly shields five and can place up to five weaker Echoes while skipping stronger ones; extra raid shields, orb travel, pets and geometry are omitted. Barrier includes its selected target plus four allies and preserves stronger Echoes. Echoes are 30%, or 37.5% / 45% with one / two Time Lord ranks. Nozdormu's Teachings reduces modeled Dream Breath recharge by five seconds, including on Stasis replay; Fire Breath is not modeled.",
	}
	if stats.KnownSpells == nil {
		out = append(out, "Legacy capture without a spellbook: active spells use selected talent IDs. Update character for availability and override checks.")
	} else {
		for i, sp := range spells {
			if !sp.passive && i != stasisRelease && !stats.KnownSpells[sp.id] {
				reason := "unavailable or replaced"
				if _, ok := stats.KnownSpells[sp.id]; !ok {
					reason = "not captured"
				}
				out = append(out, sp.name+" is "+reason+" and is excluded from casts.")
			}
		}
	}
	return out
}
func (s *simulation) preservationAvailable(id int) bool {
	sp := s.spells[id]
	if sp.passive || id == stasisRelease {
		return false
	}
	if s.stats.KnownSpells != nil {
		return s.stats.KnownSpells[sp.id]
	}
	if id == anomaly && s.has(1291636) {
		return false
	}
	return id == flame || id == blossom || s.has(sp.id)
}
func (s *simulation) configurePreservation() {
	rank := float64(s.stats.Talents[376240])
	s.spells[reversion].duration *= 1 + .15*rank
	s.spells[reversion].cost *= 1 - .15*rank
	s.spells[echo].cost *= 1 - .15*rank
	if s.has(373834) {
		s.spells[flame].direct *= 1.3
		s.spells[breath].direct *= 1.2
		s.spells[breath].hot *= 1.2
	}
	renewal := 1 + .15*float64(s.stats.Talents[371257])
	s.spells[breath].direct *= renewal
	s.spells[breath].hot *= renewal
	if s.has(376150) {
		s.spells[breath].cd -= 6
	}
	if s.has(1264269) {
		s.spells[breath].charges++
	}
	// Numeric effects transcribed from the same pinned 12.1 DBC. These apply to
	// original and Echo/Stasis copies because all use this configured spell.
	if s.has(1239581) {
		s.spells[breath].direct *= 1.1
		s.spells[breath].hot *= 1.1
	}
	if s.has(1264321) {
		s.spells[breath].duration += 6
	}
	if s.has(444845) {
		s.spells[breath].hot *= 1.2
	}
	if s.has(1218447) {
		s.spells[breath].period *= .85
	}
	if s.has(381922) {
		s.spells[rewind].cd -= 60
	}
	for _, id := range []int{anomaly, barrier} {
		s.spells[id].duration *= 1 + .15*rank
		s.spells[id].cost *= 1 - .15*rank
		if s.has(431715) {
			s.spells[id].cost *= .85
			s.spells[id].cd -= 4
		}
	}
}
func (s *simulation) healPreservation(id, target int, amount float64) bool {
	p := &s.people[target]
	if p.dead {
		return false
	}
	amount *= .8 * (1 + s.stats.Versatility) * (1 + .03*float64(s.stats.Talents[376930]))
	if target == 0 {
		amount *= 1 + .03*float64(s.stats.Talents[376930])
	}
	if id == flame {
		amount *= 1 + .03*float64(s.stats.Talents[375554])
	}
	if s.has(376239) {
		for _, key := range []int{reversion, reversion + len(s.spells)} {
			if h, ok := p.hots[key]; ok && h.end >= s.now-1e-8 {
				amount *= 1.1
			}
		}
	}
	if p.hp/p.max <= s.people[0].hp/s.people[0].max {
		amount *= 1 + s.stats.Mastery
	}
	critical := s.healRNG.Float64() < s.stats.Crit
	if critical {
		amount *= 2
	}
	s.applyRaw(id, target, amount)
	return critical
}
func (s *simulation) rollEssenceBurst(id int) {
	if !s.has(369297) || (id != flame && id != reversion) {
		return
	}
	chance := .2
	if id == reversion {
		chance = .15
	}
	if s.healRNG.Float64() >= chance {
		return
	}
	if s.burstEnd <= s.now {
		s.burstStacks = 0
	}
	cap := 1
	if s.has(375722) {
		cap = 2
	}
	if s.burstStacks < cap {
		s.burstStacks++
	}
	s.burstEnd = s.now + 15
}
func (s *simulation) castStasis(id int, trace bool) {
	if id == stasisRelease {
		s.releaseStasis(trace, false)
		s.nextAction = s.now + math.Max(.75, 1.5/(1+s.stats.Haste)) + .15
		return
	}
	s.mana -= s.cost(stasis)
	s.stasisState = stasisState{loading: true}
	s.ready[stasis][0] = math.Inf(1)
	s.totals[stasis].Casts++
	if trace && len(s.result.Casts) < 500 {
		s.result.Casts = append(s.result.Casts, Cast{Time: s.now, Spell: "Stasis (store)", Target: 1})
	}
	// Activation is off the GCD; the shared reaction interval still applies.
	s.nextAction = s.now + .15
}
func (s *simulation) storeStasis(id, target int) {
	if !s.stasisState.loading || (id != anomaly && id != barrier && (id == rewind || id == flight || id >= stasis)) {
		return
	}
	s.stasisState.spells = append(s.stasisState.spells, storedSpell{id, target})
	if len(s.stasisState.spells) == 3 {
		s.stasisState.loading = false
		s.stasisState.expires = s.now + 30
		s.ready[stasis][0] = s.now + s.spells[stasis].cd
	}
}
func (s *simulation) releaseStasis(trace, automatic bool) {
	if len(s.stasisState.spells) != 3 || s.people[0].dead {
		return
	}
	for i, sp := range s.stasisState.spells {
		s.stasisQueue = append(s.stasisQueue, storedReplay{sp, s.now + stasisReplayInterval*float64(i)})
	}
	s.stasisState = stasisState{}
	s.totals[stasisRelease].Casts++
	if trace && len(s.result.Casts) < 500 {
		origin := ""
		if automatic {
			origin = "automatic"
		}
		s.result.Casts = append(s.result.Casts, Cast{Time: s.now, Spell: "Stasis (release)", Target: 1, Origin: origin})
	}
}
func (s *simulation) raidWorkload() bool {
	return s.options.Scenario == "raid" || s.options.Replay != nil && len(s.people) > 5
}
func (s *simulation) rewindAmount(target int) float64 {
	amount := math.Max(.3*s.recent(target), 2*s.stats.SpellPower)
	if !s.raidWorkload() {
		amount *= 3
	}
	return amount
}
func (s *simulation) rewindUseful(ids []int) bool {
	useful, missing := 0., 0.
	for _, i := range ids {
		missing += s.people[i].max - s.people[i].hp
		useful += math.Min(s.people[i].max-s.people[i].hp, s.rewindAmount(i))
	}
	// Do not spend a major cooldown merely for its minimum heal on chip damage.
	return missing > 0 && useful >= .3*s.stats.Health && useful >= .25*missing
}
func (s *simulation) startRewind() {
	period := 1 / (1 + s.stats.Haste)
	for i, p := range s.people {
		if p.dead {
			continue
		}
		amount := s.rewindAmount(i) / 5
		s.applyRaw(rewind, i, amount)
		s.rewinds = append(s.rewinds, rewindHot{target: i, ticks: 4, amount: amount, next: s.now + period, period: period})
	}
}
func (s *simulation) tickPreservation(trace bool) {
	if s.people[0].dead {
		s.stasisState = stasisState{}
		s.stasisQueue = nil
	}
	if len(s.stasisState.spells) == 3 && s.stasisState.expires <= s.now+1e-8 {
		s.releaseStasis(trace, true)
	}
	kept := s.stasisQueue[:0]
	for _, p := range s.stasisQueue {
		if p.at > s.now+1e-8 {
			kept = append(kept, p)
			continue
		}
		s.echoBoost = false // A prior paid cast's one-use boost cannot leak into replay.
		s.totals[p.spell].Replays++
		if trace && len(s.result.Casts) < 500 {
			s.result.Casts = append(s.result.Casts, Cast{Time: s.now, Spell: s.spells[p.spell].name, Target: p.target + 1, Origin: "stasis"})
		}
		if p.spell == blossom {
			// Stasis repeats the cast, including Blossom's delay before it blooms.
			s.pending = append(s.pending, pending{s.now + 1.5, blossom, p.target})
		} else {
			s.finishPreservation(p.spell, p.target, true)
		}
	}
	s.stasisQueue = kept
	hots := s.rewinds[:0]
	for _, h := range s.rewinds {
		if s.people[h.target].dead {
			continue
		}
		for h.ticks > 0 && h.next <= s.now+1e-8 {
			s.applyRaw(rewind, h.target, h.amount)
			h.next += h.period
			h.ticks--
		}
		if h.ticks > 0 {
			hots = append(hots, h)
		}
	}
	s.rewinds = hots
}
