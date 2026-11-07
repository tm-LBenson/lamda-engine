package healsim

import (
	"context"
	"math"
	"math/rand"
	"sort"
)

// Values are an original transcription of numeric spell facts from SimC's
// 12.1.0.69933 DBC (fa22f84d), not an imported rotation or QE implementation.
// Talents/procs absent from modeledTalents are deliberately not inferred.
var modeledTalents = []int{355936, 364343, 366155, 360995, 355913, 359816, 369297, 372527, 375554, 376930, 378196, 363534, 370537, 376239, 373834, 376240, 371257, 381922, 376150, 375722, 1264269, 373861, 1291636, 376237, 431715, 1239581, 1264321, 444845, 1218447}

type spell struct {
	name                                                   string
	direct, hot, period, duration, cast, cost, essence, cd float64
	targets, charges                                       int
	echo                                                   bool
	id, talent                                             int
	shield, damage, bloom, chain                           float64
	channel, ground, initial, hastedCD, passive            bool
}

var spells = []spell{
	{id: 361469, name: "Living Flame", direct: 7.8 * 1.5, cast: 2, cost: .022, targets: 1, charges: 1, echo: true},
	{id: 364343, name: "Echo", direct: 2.4, cost: .019, essence: 2, targets: 1, charges: 1},
	{id: 366155, name: "Reversion", hot: .4275, period: 2, duration: 12, cost: .022, cd: 9, targets: 1, charges: 2, echo: true},
	{id: 355936, name: "Dream Breath", direct: .9216, hot: .5352, period: 2, duration: 16, cast: 1, cost: .049, cd: 30, targets: 6, charges: 1, echo: true},
	{id: 360995, name: "Verdant Embrace", direct: 13.754, cost: .024, cd: 18, targets: 1, charges: 1, echo: true},
	{id: 355913, name: "Emerald Blossom", direct: 3.23, cost: .0353, essence: 3, targets: 3, charges: 1},
	{id: 359816, name: "Dream Flight", direct: 5, hot: 2, period: 3, duration: 15, cast: 2, cost: .04, cd: 120, targets: 20, charges: 1},
	{id: 363534, name: "Rewind", cost: .055, cd: 240, targets: 40, charges: 1},
	{id: 370537, name: "Stasis", cost: .04, cd: 90, targets: 1, charges: 1},
	{id: 370564, name: "Stasis release", targets: 1, charges: 1},
	{id: 378196, name: "Golden Hour", passive: true, charges: 1},
	{id: 373861, name: "Temporal Anomaly", shield: 6, duration: 15, cast: 1.5, cost: .075, cd: 15, targets: 5, charges: 1, hastedCD: true},
	{id: 1291636, name: "Temporal Barrier", shield: 11.7, duration: 15, cast: 1.5, cost: .075, cd: 15, targets: 5, charges: 1, hastedCD: true},
}

const (
	flame = iota
	echo
	reversion
	breath
	embrace
	blossom
	flight
	rewind
	stasis
	stasisRelease
	goldenHour
	anomaly
	barrier
)

type hot struct {
	spell                   int
	factor, next, end, last float64
	extended                float64
}
type ally struct {
	temporalShields                                                    []temporalShield
	hp, max, echo, echoEnd                                             float64
	dead                                                               bool
	hots                                                               map[int]hot
	damage                                                             [100]float64
	atonement, shield, shieldEnd, masteryPool, masteryNext, masteryEnd float64
	pom, hymn                                                          int
	pomEnd, hymnEnd                                                    float64
}
type pending struct {
	at            float64
	spell, target int
}
type simulation struct {
	stats                                                    Stats
	options                                                  Options
	policy                                                   string
	now, mana, essence, nextAction, burst, critical, starved float64
	people                                                   []ally
	ready                                                    [][]float64
	spells                                                   []spell
	spec                                                     int
	holyPower, wings                                         float64
	ground                                                   []groundEffect
	pending                                                  []pending
	result                                                   Outcome
	totals                                                   []SpellResult
	healRNG, damageRNG                                       *rand.Rand
	deaths                                                   int
	burstStacks                                              int
	burstEnd                                                 float64
	echoBoost                                                bool
	replayIndex                                              int
	stasisState                                              stasisState
	stasisQueue                                              []storedReplay
	rewinds                                                  []rewindHot
}

func groupSize(scenario string) int {
	if scenario == "tank" {
		return 2
	}
	if scenario == "raid" {
		return 20
	}
	return 5
}

func Run(ctx context.Context, stats Stats, options Options) (*Report, error) {
	if err := stats.Validate(); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateSpellbook(options.Spec, stats); err != nil {
		return nil, err
	}
	if options.Spec == 0 {
		options.Spec = 1468
	}
	r := &Report{Spec: options.Spec, SpecName: SpecName(options.Spec), Version: Version, Options: options, Stats: stats, GroupSize: groupSize(options.Scenario), ModeledTalents: append([]int{}, modeledTalents...), Warnings: []string{
		"Experimental core-spell model. HPS is effective healing in the selected workload; incomplete build coverage prevents a calibrated prediction of your dungeon or raid log.",
		"Only the listed core spells and talents are modeled. Most hero/apex effects (apart from listed Dream Breath modifiers and Nozdormu Adept), Merithra's Blessing, Twin Echoes, Lifebind, Consume Flame, Inner Flame, Tempo Charged, set bonuses, trinket procs, leech, and external buffs are not modeled yet. Do not use this model to rank builds or gear.",
		"One healer; other players never heal, use defensives, or resurrect. Raid damage represents an assigned healing workload. All targets are in range; movement, cone positioning, dispels and damage casting are omitted.",
		"Stats are captured with current buffs. Mana starts full and uses captured regeneration; base mana costs assume max mana equals base mana. Cooldowns start ready. Dream Breath uses rank 1; GCD/cast and HoT timing resolve at 50 ms steps.",
		"Consensus is a Lamda core-spell policy informed by guide agreement, not a full implementation of any author's rotation. Reactive is a comparison without damage forecasting. Traces show the first seed; summary values average all seeds.",
	}}
	if options.Spec == 1468 {
		r.Warnings = append(r.Warnings, preservationWarnings(stats)...)
	}
	if options.Spec != 1468 {
		r.Warnings = coreWarnings(options.Spec)
		r.ModeledTalents = modeledFor(options.Spec)
		for _, sp := range spellsFor(options.Spec) {
			if !sp.passive && !stats.KnownSpells[sp.id] {
				r.Warnings = append(r.Warnings, sp.name+" is unavailable or replaced in this capture and is excluded from casts.")
			}
		}
	}
	if options.Replay != nil {
		meta := *options.Replay
		meta.Events = nil
		r.Replay = &meta
		r.GroupSize = len(meta.Members)
		r.Warnings = append(r.Warnings, meta.Warnings...)
		r.Warnings = append(r.Warnings, "Replay retains recorded post-mitigation health damage. Logged absorbs/defensives are baked into that workload. Other healers' healing is omitted; replaying a whole raid against one healer is a stress test, not a fair personal HPS prediction. Damage after recorded deaths is unavailable. Preparation assumes perfect knowledge of recorded damage during the next five seconds; the reactive comparison has no forecast.")
	}
	if options.Training != nil {
		summary := options.Training.Summary
		r.Training = &summary
		meta := *options.Training.sample(options.Seed)
		meta.Events = nil
		r.Replay = &meta
		r.GroupSize = len(meta.Members)
		r.Warnings = append(r.Warnings, meta.Warnings...)
		r.Warnings = append(r.Warnings, "Learned workload resamples complete recorded attempts, preserving bursts, targets and duration. The latest independent fight is held out from training and forecasts. Preparation uses training fights only, without reading the sampled fight's future. Damage and burst validation measure workload fit, not healing-model accuracy.", "Small datasets do not describe all encounter variation. Recorded mitigation remains baked in; absent co-healing and unmodeled build effects still limit predicted outcomes.")
		for run := 0; run < options.Runs; run++ {
			r.Workloads = append(r.Workloads, options.Training.draw(options.Seed+int64(run)).ID)
		}
	}
	policies := []string{options.Policy}
	if options.Policy == "compare" {
		policies = []string{"consensus", "reactive"}
	}
	for _, policy := range policies {
		out := Outcome{Policy: policy}
		values := make([]float64, 0, options.Runs)
		var raw, healed float64
		spellSums := make([]SpellResult, len(spellsFor(options.Spec)))
		for run := 0; run < options.Runs; run++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			runOptions := options
			if options.Training != nil {
				runOptions.Replay = options.Training.sample(options.Seed + int64(run))
				runOptions.Seconds = int(math.Ceil(runOptions.Replay.Seconds))
			}
			s := newSimulation(stats, runOptions, policy, options.Seed+int64(run))
			if err := s.run(ctx, run == 0); err != nil {
				return nil, err
			}
			value := s.result
			values = append(values, value.HPS)
			out.HPS += value.HPS
			out.RawHPS += value.RawHPS
			out.AbsorbPS += value.AbsorbPS
			out.Deaths += float64(s.deaths)
			if s.deaths == 0 {
				out.SurvivalPercent += 100
			}
			out.CriticalSeconds += s.critical
			out.ManaStarvedSeconds += s.starved
			out.ManaEnd += 100 * s.mana / stats.Mana
			for i, v := range s.totals {
				spellSums[i].Name = v.Name
				spellSums[i].Casts += v.Casts
				spellSums[i].Replays += v.Replays
				spellSums[i].Healing += v.Healing
				spellSums[i].Overheal += v.Overheal
				spellSums[i].Absorbed += v.Absorbed
				raw += v.Healing + v.Overheal
				healed += v.Healing
			}
			if run == 0 {
				out.Timeline = value.Timeline
				out.Casts = value.Casts
			}
		}
		n := float64(options.Runs)
		out.HPS /= n
		out.RawHPS /= n
		out.AbsorbPS /= n
		out.Deaths /= n
		out.SurvivalPercent /= n
		out.CriticalSeconds /= n
		out.ManaStarvedSeconds /= n
		out.ManaEnd /= n
		if raw > 0 {
			out.OverhealPercent = 100 * (raw - healed) / raw
		}
		if n > 1 {
			for _, v := range values {
				out.StdDev += (v - out.HPS) * (v - out.HPS)
			}
			out.StdDev = math.Sqrt(out.StdDev / (n - 1))
			out.MeanError = 1.96 * out.StdDev / math.Sqrt(n)
		}
		// Spell totals deliberately cover ALL runs; casts stay integer counts.
		out.Spells = spellSums
		r.Outcomes = append(r.Outcomes, out)
	}
	if options.Training != nil {
		holdout := options
		holdout.forecast = options.Training
		holdout.Replay = &options.Training.Holdout.Replay
		holdout.Seconds = int(math.Ceil(holdout.Replay.Seconds))
		holdout.Training = nil
		holdout.Scenario = "replay"
		validation, err := Run(ctx, stats, holdout)
		if err != nil {
			return nil, err
		}
		r.HoldoutOutcomes = validation.Outcomes
	}
	return r, nil
}
func newSimulation(stats Stats, o Options, policy string, seed int64) *simulation {
	if o.Spec == 0 {
		o.Spec = 1468
	}
	s := &simulation{spec: o.Spec, spells: append([]spell(nil), spellsFor(o.Spec)...), stats: stats, options: o, policy: policy, mana: stats.Mana, essence: 5, healRNG: rand.New(rand.NewSource(seed)), damageRNG: rand.New(rand.NewSource(seed ^ 0x5eed0123))}
	for i := 0; i < groupSize(o.Scenario); i++ {
		hp := stats.Health
		if i == 1 {
			hp *= 1.6
		}
		s.people = append(s.people, ally{hp: hp, max: hp, hots: map[int]hot{}})
	}
	if o.Replay != nil {
		s.people = nil
		for i, p := range o.Replay.Members {
			hp := p.MaxHealth
			if i == 0 {
				hp = stats.Health
			}
			s.people = append(s.people, ally{hp: hp, max: hp, hots: map[int]hot{}})
		}
	}
	if s.spec == 1468 {
		s.configurePreservation()
	}
	s.ready = make([][]float64, len(s.spells))
	s.totals = make([]SpellResult, len(s.spells))
	for i, sp := range s.spells {
		s.ready[i] = make([]float64, sp.charges)
		s.totals[i].Name = sp.name
	}
	if s.spec != 1468 {
		s.essence = 0
	}
	return s
}

func (s *simulation) has(id int) bool { return s.stats.Talents[id] > 0 }
func (s *simulation) available(id int) bool {
	if s.spec != 1468 {
		sp := s.spells[id]
		return !sp.passive && (sp.talent == 0 || s.has(sp.talent)) && (s.stats.KnownSpells == nil || s.stats.KnownSpells[sp.id])
	}
	return s.preservationAvailable(id)
}
func (s *simulation) cost(id int) float64 {
	if s.spells[id].essence > 0 && s.burstStacks > 0 && s.burstEnd >= s.now {
		return 0
	}
	return s.spells[id].cost * s.stats.Mana
}
func (s *simulation) can(id int) bool {
	if id < 0 || id >= len(s.spells) {
		return false
	}
	if s.spec == 1468 && id == stasisRelease {
		return len(s.stasisState.spells) == 3
	}
	if s.spec == 1468 && id == stasis && (s.stasisState.loading || len(s.stasisState.spells) > 0) {
		return false
	}
	if s.spec == 65 && (s.spells[id].id == 85673 || s.spells[id].id == 85222) && s.holyPower < 3 {
		return false
	}
	if !s.available(id) || s.mana+1e-8 < s.cost(id) || (s.spells[id].essence > s.essence+1e-8 && s.burstStacks == 0) {
		return false
	}
	for _, t := range s.ready[id] {
		if t <= s.now+1e-8 {
			return true
		}
	}
	return false
}
func (s *simulation) targets(n int) []int {
	ids := make([]int, 0, len(s.people))
	for i, p := range s.people {
		if !p.dead {
			ids = append(ids, i)
		}
	}
	sort.SliceStable(ids, func(i, j int) bool {
		return s.people[ids[i]].hp/s.people[ids[i]].max < s.people[ids[j]].hp/s.people[ids[j]].max
	})
	if n < len(ids) {
		ids = ids[:n]
	}
	return ids
}
func (s *simulation) recent(target int) float64 {
	sum := 0.
	for _, v := range s.people[target].damage {
		sum += v
	}
	return sum
}
func (s *simulation) heal(id, target int, amount float64) {
	if s.spec != 1468 {
		s.healCore(id, target, amount, true, true)
		return
	}
	s.healPreservation(id, target, amount)
}
func (s *simulation) addHot(id, target int, factor float64, replica bool) {
	sp := s.spells[id]
	key := id
	if replica {
		key += len(s.spells)
	}
	// Normal and Echo versions each retain their own tick phase and carryover.
	p := &s.people[target]
	h := hot{spell: id, factor: factor, next: s.now + sp.period/(1+s.stats.Haste), end: s.now + sp.duration, last: s.now}
	if old, ok := p.hots[key]; ok && old.end > s.now {
		h.next = old.next
		h.last = old.last
		if id == reversion || id == breath {
			h.end += math.Min(old.end-s.now, .3*sp.duration)
		}
	}
	p.hots[key] = h
}
func (s *simulation) apply(id, target int, factor float64, replica bool) {
	sp := s.spells[id]
	if s.people[target].dead {
		return
	}
	if sp.direct > 0 {
		s.heal(id, target, sp.direct*s.stats.SpellPower*factor)
	}
	if id == reversion && s.has(378196) {
		// Golden Hour derives from damage already taken, not spell power. It
		// ignores mastery/crit/healing multipliers, avoiding double scaling.
		amount := s.recent(target) * .15 * factor
		p := &s.people[target]
		effective := math.Min(amount, p.max-p.hp)
		p.hp += effective
		s.totals[goldenHour].Healing += effective
		s.totals[goldenHour].Overheal += amount - effective
	}
	if sp.hot > 0 {
		s.addHot(id, target, factor, replica)
	}
	s.rollEssenceBurst(id)
}
func (s *simulation) finish(id, target int) {
	if s.spec != 1468 {
		s.finishCore(id, target)
		return
	}
	s.finishPreservation(id, target, false)
}
func (s *simulation) finishPreservation(id, target int, replayed bool) {
	if s.people[0].dead {
		return
	}
	sp := s.spells[id]
	if id == rewind {
		s.startRewind()
		return
	}
	if sp.targets == 1 && s.people[target].dead {
		return
	}
	if id == barrier && s.people[target].dead {
		return
	}
	if !replayed {
		s.storeStasis(id, target)
	}
	if id == anomaly || id == barrier {
		s.finishTemporal(id, target)
		return
	}
	if id == echo {
		s.apply(id, target, 1, false)
		p := &s.people[target]
		p.echo = .7 * s.echoMultiplier()
		if s.echoBoost {
			p.echo *= 1.35
			s.echoBoost = false
		}
		p.echoEnd = s.now + 15*(1+.15*float64(s.stats.Talents[376240]))
		return
	}
	if sp.targets == 1 {
		s.apply(id, target, 1, false)
	} else {
		ids := s.targets(sp.targets)
		if id == breath { // Dream Breath always includes the caster.
			found := false
			for _, i := range ids {
				if i == 0 {
					found = true
				}
			}
			if !found && len(ids) > 0 {
				ids[len(ids)-1] = 0
			}
		}
		factor := 1.
		if id == flight && len(ids) > 5 {
			factor = math.Sqrt(5 / float64(len(ids)))
		}
		for _, i := range ids {
			s.apply(id, i, factor, false)
		}
	}
	if sp.echo {
		for i := range s.people {
			p := &s.people[i]
			if p.echo > 0 && p.echoEnd >= s.now && !p.dead {
				factor := p.echo
				p.echo = 0
				p.echoEnd = 0
				s.apply(id, i, factor, true)
			}
		}
	}

}
func (s *simulation) cast(id, target int, trace bool) {
	if s.spec != 1468 {
		s.castCore(id, target, trace)
		return
	}
	if id == stasis || id == stasisRelease {
		s.castStasis(id, trace)
		return
	}
	sp := s.spells[id]
	s.echoBoost = id == echo && s.burstStacks > 0
	s.mana -= s.cost(id)
	if sp.essence > 0 {
		if s.burstStacks > 0 {
			s.burstStacks--
		} else {
			s.essence -= sp.essence
		}
	}
	cd := sp.cd
	if id == reversion || id == embrace || sp.hastedCD {
		cd /= 1 + s.stats.Haste
	}
	// Charges recharge sequentially, never both in parallel.
	for i, t := range s.ready[id] {
		if t <= s.now+1e-8 {
			latest := s.now
			for _, other := range s.ready[id] {
				latest = math.Max(latest, other)
			}
			s.ready[id][i] = latest + cd
			break
		}
	}
	duration := sp.cast / (1 + s.stats.Haste)
	s.nextAction = s.now + math.Max(.75, math.Max(1.5/(1+s.stats.Haste), duration)) + .15
	s.totals[id].Casts++
	if trace && len(s.result.Casts) < 500 {
		s.result.Casts = append(s.result.Casts, Cast{Time: s.now, Spell: sp.name, Target: target + 1})
	}
	if duration > 0 {
		s.pending = append(s.pending, pending{s.now + duration, id, target})
	} else if id == blossom {
		// The instant cast is stored now; its delayed heal must not be stored again.
		s.storeStasis(id, target)
		s.pending = append(s.pending, pending{s.now + 1.5, id, target})
	} else {
		s.finish(id, target)
	}
}
func (s *simulation) tickPending() {
	kept := s.pending[:0]
	for _, p := range s.pending {
		if p.at <= s.now+1e-8 {
			if s.spec == 1468 && p.spell == blossom {
				// Both paid and Stasis casts have already completed at launch.
				s.finishPreservation(p.spell, p.target, true)
			} else {
				s.finish(p.spell, p.target)
			}
		} else {
			kept = append(kept, p)
		}
	}
	s.pending = kept
}
func (s *simulation) choose() (int, int) {
	if s.spec != 1468 {
		return s.chooseCore()
	}
	ids := s.targets(len(s.people))
	if len(ids) == 0 {
		return -1, 0
	}
	target := ids[0]
	lowest := s.people[target].hp / s.people[target].max
	injured := 0
	for _, i := range ids {
		if s.people[i].hp/s.people[i].max < .85 {
			injured++
		}
	}
	// Foreknowledge applies only to this synthetic script's fixed burst times.
	phase := math.Mod(s.now, 20)
	prep := s.policy == "consensus" && s.options.Replay == nil && phase >= 6 && phase < 10
	if s.policy == "consensus" && s.options.Replay != nil {
		prep = s.replayBurstSoon()
	}
	if len(s.stasisState.spells) == 3 && (injured >= 2 || lowest < .45 || s.stasisState.expires-s.now < 1) {
		return stasisRelease, target
	}
	if s.can(rewind) && s.rewindUseful(ids) {
		return rewind, target
	}
	if s.can(stasis) && (prep || injured >= 2) {
		return stasis, target
	}
	if lowest < .40 && s.can(embrace) {
		return embrace, target
	}
	if injured >= 3 && lowest < .5 && s.can(flight) {
		return flight, target
	}
	if prep || injured >= 2 || lowest < .75 {
		for _, id := range []int{barrier, anomaly} {
			if s.can(id) && s.temporalUseful(id, target) {
				return id, target
			}
		}
	}
	if s.stasisState.loading && (prep || injured >= 2) {
		if s.can(breath) {
			return breath, target
		}
		if lowest < .9 && s.can(embrace) {
			return embrace, target
		}
		if s.can(reversion) {
			return reversion, target
		}
	}
	if prep && s.can(echo) {
		for _, i := range ids {
			if s.people[i].echoEnd <= s.now {
				return echo, i
			}
		}
	}
	if s.can(breath) && injured >= 2 {
		return breath, target
	}
	if s.can(reversion) && (lowest < .80 || prep) {
		for _, i := range ids {
			if h, ok := s.people[i].hots[reversion]; !ok || h.end-s.now < 2 {
				return reversion, i
			}
		}
	}
	if injured >= 3 && s.can(blossom) {
		return blossom, target
	}
	if lowest < .7 && s.can(embrace) {
		return embrace, target
	}
	if lowest < .9 && s.can(flame) {
		return flame, target
	}
	return -1, target
}
func (s *simulation) damage(tick int) {
	// Fixed random stream and fixed per-target draws preserve identical damage
	// schedules between policies, even after a target dies in only one run.
	for i := range s.people {
		p := &s.people[i]
		jitter := .9 + .2*s.damageRNG.Float64()
		fraction := .0
		if tick%20 == 0 { // once per second
			switch s.options.Scenario {
			case "tank":
				if i == 1 {
					fraction = .035
				} else {
					fraction = .003
				}
			case "dungeon":
				fraction = .007
				if i == 1 {
					fraction = .025
				}
			case "raid":
				fraction = .003
				if i == 1 {
					fraction = .015
				}
			}
		}
		if tick%400 == 200 {
			if s.options.Scenario != "tank" || i == 1 {
				fraction += .35
			}
		}
		if tick%400 > 200 && tick%400 <= 260 && tick%20 == 0 {
			if s.options.Scenario != "tank" || i == 1 {
				fraction += .025
			}
		}
		amount := fraction * p.max * jitter * float64(s.options.Intensity) / 100
		p.damage[tick%len(p.damage)] = 0
		if p.dead {
			continue
		}
		amount = s.absorbDamage(i, amount)
		p.damage[tick%len(p.damage)] = math.Min(amount, p.hp)
		p.hp = math.Max(0, p.hp-amount)
		if p.hp == 0 {
			p.dead = true
			p.hots = map[int]hot{}
			s.deaths++
		}
		if amount > 0 {
			s.damageTriggers(i)
		}
	}
}
func (s *simulation) run(ctx context.Context, trace bool) error {
	const step = .05
	duration := float64(s.options.Seconds)
	if s.options.Replay != nil {
		duration = s.options.Replay.Seconds
	}
	lastTime := 0.
	for tick := 0; tick <= int(math.Ceil(duration/step)); tick++ {
		if tick%400 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		s.now = math.Min(float64(tick)*step, duration)
		if s.now > s.burstEnd {
			s.burstStacks = 0
		}
		elapsed := s.now - lastTime
		lastTime = s.now
		if tick > 0 {
			s.mana = math.Min(s.stats.Mana, s.mana+s.stats.ManaRegen*elapsed)
			if s.spec == 1468 {
				s.essence = math.Min(5, s.essence+elapsed*(1+s.stats.Haste)/5)
			}
		}
		if s.options.Replay != nil {
			s.replayDamage(tick)
		} else {
			s.damage(tick)
		}
		if s.spec == 1468 {
			s.tickPreservation(trace)
		}
		s.tickCore()
		for i := range s.people {
			p := &s.people[i]
			if p.dead {
				continue
			}
			// Iterate stable keys so RNG consumption is reproducible across Go runs.
			for key := 0; key < 2*len(s.spells); key++ {
				h, ok := p.hots[key]
				if !ok {
					continue
				}
				sp := s.spells[h.spell]
				period := sp.period / (1 + s.stats.Haste)
				if s.now+1e-8 >= h.next || s.now+1e-8 >= h.end {
					end := math.Min(h.next, h.end)
					fraction := math.Min(1, (end-h.last)/period)
					if fraction > 0 {
						if s.spec == 1468 {
							critical := s.healPreservation(h.spell, i, sp.hot*s.stats.SpellPower*h.factor*fraction)
							if h.spell == reversion && critical && fraction >= 1-1e-8 && h.extended < 12 {
								extra := math.Min(2, 12-h.extended)
								h.end += extra
								h.extended += extra
							}
						} else {
							s.healCore(h.spell, i, sp.hot*s.stats.SpellPower*h.factor*fraction, true, false)
						}
					}
					h.last = end
					h.next += period
					if s.now+1e-8 >= h.end {
						delete(p.hots, key)
						if s.spec != 1468 && sp.bloom > 0 {
							s.healCore(h.spell, i, sp.bloom*s.stats.SpellPower, true, true)
						}
					} else {
						p.hots[key] = h
					}
				}
			}
		}
		s.tickPending()
		if s.now < duration && !s.people[0].dead && s.now+1e-8 >= s.nextAction {
			id, target := s.choose()
			if id >= 0 {
				s.cast(id, target, trace)
			} else if s.people[target].hp/s.people[target].max < .9 && s.mana < s.cost(flame) {
				s.starved += math.Min(step, duration-s.now)
			}
		}
		if s.now < duration {
			for _, p := range s.people {
				if !p.dead && p.hp/p.max < .3 {
					s.critical += math.Min(step, duration-s.now)
					break
				}
			}
		}
		if trace && (tick%40 == 0 || s.now == duration) {
			point := Point{Time: s.now, Mana: 100 * s.mana / s.stats.Mana, Essence: s.essence, ResourceName: resourceName(s.spec), Resource: s.holyPower}
			for _, p := range s.people {
				point.Health = append(point.Health, 100*p.hp/p.max)
			}
			s.result.Timeline = append(s.result.Timeline, point)
		}
	}
	for _, v := range s.totals {
		s.result.HPS += v.Healing / duration
		s.result.AbsorbPS += v.Absorbed / duration
		s.result.RawHPS += (v.Healing + v.Overheal) / duration
	}
	return nil
}
