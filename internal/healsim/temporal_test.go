package healsim

import (
	"context"
	"encoding/json"
	"testing"
)

func temporalFixture(id int) *simulation {
	st := testStats()
	st.Crit, st.Mastery, st.Versatility = 0, 0, 0
	st.Talents = map[int]int{spells[id].id: 1, 364343: 1, 366155: 1, 355936: 1}
	return newSimulation(st, testOptions(), "consensus", 1)
}

func TestTemporalShieldConsumesDamageWithoutBecomingHealing(t *testing.T) {
	for _, id := range []int{anomaly, barrier} {
		s := temporalFixture(id)
		s.stats.SpellPower = 1000
		s.finish(id, 1)
		budget := 6000.
		if id == barrier {
			budget = 11700
		}
		before := s.people[1].hp
		near(t, s.absorbDamage(1, 2000), 0)
		near(t, s.absorbDamage(1, budget), 2000)
		near(t, s.totals[id].Absorbed, budget)
		near(t, s.totals[id].Healing, 0)
		near(t, s.totals[id].Overheal, 0)
		near(t, s.people[1].hp, before)
		if len(s.people[1].temporalShields) != 0 {
			t.Fatal("spent shield retained")
		}
		// Unused shields expire, not score as healing or absorption.
		s.now = 15
		near(t, s.absorbDamage(2, 500), 500)
		near(t, s.totals[id].Absorbed, budget)
	}
}

func TestTemporalShieldStackRefreshAndExactExpiry(t *testing.T) {
	s := temporalFixture(anomaly)
	s.stats.SpellPower = 1000
	s.finish(anomaly, 1)
	near(t, s.absorbDamage(1, 1000), 0)
	s.now = 10
	s.finish(anomaly, 1)
	near(t, s.people[1].temporalShields[0].amount, 11000)
	near(t, s.people[1].temporalShields[0].end, 25)
	s.now = 15
	near(t, s.absorbDamage(1, 6000), 0)
	s.now = 25
	near(t, s.absorbDamage(1, 6000), 6000)
	near(t, s.totals[anomaly].Absorbed, 7000)
	s.finish(anomaly, 1)
	near(t, s.people[1].temporalShields[0].amount, 6000)
}

func TestTemporalMasterySnapshotsEqualHealthAndCannotCrit(t *testing.T) {
	s := temporalFixture(barrier)
	s.stats.SpellPower, s.stats.Crit, s.stats.Mastery, s.stats.Versatility = 1000, 1, .5, .2
	s.people[0].hp = .5 * s.people[0].max
	s.people[1].hp = .5 * s.people[1].max
	s.people[2].hp = .6 * s.people[2].max
	s.finish(barrier, 1)
	for _, i := range []int{0, 1} {
		near(t, s.people[i].temporalShields[0].amount, 11700*1.2*1.5)
	}
	near(t, s.people[2].temporalShields[0].amount, 11700*1.2)
	s.people[0].hp = 1000 // Later mastery/crit/stat changes cannot resize a shield.
	s.stats.Versatility = 0
	near(t, s.absorbDamage(1, 30000), 30000-11700*1.2*1.5)
	// The same equality boundary applies to ordinary healing, including self-heals.
	s.stats.Crit = 0
	s.people[0].hp = .5 * s.people[0].max
	s.people[1].hp = .5 * s.people[1].max
	s.heal(flame, 1, 1000)
	near(t, s.totals[flame].Healing, 1200)
	s.heal(flame, 0, 1000)
	near(t, s.totals[flame].Healing, 2400)
}

func TestTemporalEchoesPreserveStrongerEchoAndConsumeNormally(t *testing.T) {
	s := temporalFixture(barrier)
	s.stats.Talents[372527] = 2
	s.finish(echo, 1)
	strong, expires := s.people[1].echo, s.people[1].echoEnd
	s.now = 1
	s.finish(barrier, 2)
	near(t, s.people[1].echo, strong)
	near(t, s.people[1].echoEnd, expires)
	near(t, s.people[2].echo, .45)
	if s.totals[echo].Casts != 0 {
		t.Fatal("shield-granted Echo counted as a paid cast")
	}
	for i := range s.people {
		s.people[i].hp = s.people[i].max / 2
	}
	s.finish(reversion, 1)
	near(t, s.people[1].hots[reversion+len(s.spells)].factor, 1.05)
	near(t, s.people[2].hots[reversion+len(s.spells)].factor, .45)
	for _, p := range s.people {
		if p.echo != 0 || p.echoEnd != 0 {
			t.Fatal("shield Echo failed to consume")
		}
	}
	// Expired stronger Echo must no longer block a weak one.
	s.people[2].echo, s.people[2].echoEnd = 1.05, s.now
	s.finish(barrier, 2)
	near(t, s.people[2].echo, .45)
}

func TestTimeLordRanksAffectManualAndTemporalEchoes(t *testing.T) {
	for rank := 0; rank <= 2; rank++ {
		s := temporalFixture(anomaly)
		if rank > 0 {
			s.stats.Talents[372527] = rank
		}
		s.finish(echo, 1)
		s.finish(anomaly, 2)
		near(t, s.people[1].echo, []float64{.7, .875, 1.05}[rank])
		near(t, s.people[2].echo, []float64{.3, .375, .45}[rank])
	}
	st := testStats()
	st.Talents[372527] = 3
	if validatePreservation(st) == nil {
		t.Fatal("invalid Time Lord rank accepted")
	}
}

func TestTemporalRaidCoverageAndPrimaryTarget(t *testing.T) {
	for _, id := range []int{anomaly, barrier} {
		st := testStats()
		o := testOptions()
		o.Scenario = "raid"
		s := newSimulation(st, o, "consensus", 1)
		for i := 0; i < 4; i++ {
			s.people[i].echo, s.people[i].echoEnd = 1.05, 30
		}
		s.people[3].dead = true
		s.people[3].hp = 0
		s.finish(id, 19)
		shields, weak := 0, 0
		for i, p := range s.people {
			if len(p.temporalShields) > 0 {
				shields++
			}
			if p.echo == .3 {
				weak++
			}
			if i == 3 && len(p.temporalShields) > 0 {
				t.Fatal("shield applied to dead ally")
			}
		}
		if shields != 5 {
			t.Fatal("unmodeled extra raid shields invented", shields)
		}
		if id == anomaly && (weak != 5 || len(s.people[0].temporalShields) != 1) {
			t.Fatal("orb did not skip strong Echoes or shield caster", weak)
		}
		if id == barrier && (weak != 2 || len(s.people[19].temporalShields) != 1) {
			t.Fatal("Barrier lost primary or exceeded its five selected allies", weak)
		}
	}
}

func TestTemporalSpellAvailabilityAndCaptureValidation(t *testing.T) {
	st := testStats()
	st.Talents[373861], st.Talents[1291636] = 1, 1
	s := newSimulation(st, testOptions(), "consensus", 1)
	if s.available(anomaly) || !s.available(barrier) {
		t.Fatal("legacy capture did not honor Barrier replacement")
	}
	st.KnownSpells = map[int]bool{}
	for i, sp := range spells {
		if i <= flight {
			st.KnownSpells[sp.id] = true
		}
	}
	s = newSimulation(st, testOptions(), "consensus", 1)
	if s.available(anomaly) || s.available(barrier) {
		t.Fatal("uncaptured temporal spell inferred from talents")
	}
	st.KnownSpells[373861], st.KnownSpells[1291636] = true, true
	if validatePreservation(st) == nil {
		t.Fatal("both replacement and replaced spell accepted")
	}
	st.KnownSpells[373861] = false
	if err := validatePreservation(st); err != nil {
		t.Fatal(err)
	}
}

func TestTemporalCostsHastedCooldownAndNozdormuRecharge(t *testing.T) {
	st := testStats()
	st.Talents[373861], st.Talents[376237], st.Talents[431715], st.Talents[376240] = 1, 1, 1, 2
	st.Talents[1264269] = 1
	st.Haste = 1
	s := newSimulation(st, testOptions(), "consensus", 1)
	s.now = 10
	s.ready[breath] = []float64{30, 60}
	s.cast(anomaly, 1, true)
	near(t, s.mana, st.Mana*(1-.075*.7*.85))
	near(t, s.ready[anomaly][0], 15.5)
	s.now = 10.7
	s.tickPending()
	if len(s.people[1].temporalShields) != 0 {
		t.Fatal("shield landed before cast completed")
	}
	s.now = 10.75
	s.tickPending()
	near(t, s.ready[breath][0], 25)
	near(t, s.ready[breath][1], 55)
	near(t, s.people[1].temporalShields[0].end, 30.25)
	near(t, s.people[1].echoEnd, 30.25)
	// Barrier is a choice-node alternative, never a second Teachings trigger.
	s.finish(barrier, 1)
	near(t, s.ready[breath][0], 25)
	near(t, spells[anomaly].cost, .075) // Global kit never mutated.
}

func TestTemporalStasisReplayIsFreeAndReducesRechargeAgain(t *testing.T) {
	s := temporalFixture(anomaly)
	s.stats.Talents[376237] = 1
	s.cast(stasis, 0, false)
	s.ready[breath] = []float64{30}
	s.now = 1
	s.cast(anomaly, 1, true)
	if len(s.stasisState.spells) != 0 {
		t.Fatal("stored an unfinished cast")
	}
	s.now = 3
	s.tickPending()
	near(t, s.ready[breath][0], 25)
	s.finish(echo, 1)
	s.finish(reversion, 1)
	mana, essence, cd := s.mana, s.essence, s.ready[anomaly][0]
	s.now = 4
	s.cast(stasisRelease, 0, true)
	s.tickPreservation(true)
	near(t, s.ready[breath][0], 20)
	near(t, s.mana, mana)
	near(t, s.essence, essence)
	near(t, s.ready[anomaly][0], cd)
	if s.totals[anomaly].Casts != 1 || s.totals[anomaly].Replays != 1 {
		t.Fatal("Stasis did not replay temporal spell")
	}
	near(t, s.people[2].temporalShields[0].amount, 12*s.stats.SpellPower)
	// A dead primary makes Barrier fail and must not enter Stasis storage.
	other := temporalFixture(barrier)
	other.cast(stasis, 0, false)
	other.people[1].dead = true
	other.finish(barrier, 1)
	if len(other.stasisState.spells) != 0 || len(other.people[0].temporalShields) != 0 {
		t.Fatal("failed Barrier cast created effects")
	}
}

func TestTemporalReplayDamageDoesNotEnterRecentHealthHistory(t *testing.T) {
	s := temporalFixture(barrier)
	s.stats.SpellPower = 1000
	s.finish(barrier, 1)
	before := s.people[1].hp
	s.options.Replay = &Replay{Events: []DamageEvent{{1, 1, 10000}, {2, 1, 4000}}}
	s.now = 1
	s.replayDamage(20)
	near(t, s.recent(1), 0)
	s.now = 2
	s.replayDamage(40)
	near(t, s.recent(1), 2300)
	near(t, s.people[1].hp, before-2300)
	near(t, s.totals[barrier].Absorbed, 11700)
}

func TestTemporalPoliciesAndAggregateAccountingAreReproducible(t *testing.T) {
	for _, id := range []int{anomaly, barrier} {
		st := testStats()
		st.Talents[spells[id].id] = 1
		st.KnownSpells = map[int]bool{}
		for i, sp := range spells {
			if !sp.passive && i != stasisRelease {
				st.KnownSpells[sp.id] = i <= flight || i == id
			}
		}
		o := testOptions()
		o.Seconds, o.Runs = 180, 2
		a, err := Run(context.Background(), st, o)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Run(context.Background(), st, o)
		if err != nil {
			t.Fatal(err)
		}
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		if string(x) != string(y) {
			t.Fatal("temporal effects are not reproducible")
		}
		for _, out := range a.Outcomes {
			if out.Spells[id].Casts == 0 || out.AbsorbPS <= 0 {
				t.Fatal("policy never shielded or absorbed")
			}
			healing, absorbed := 0., 0.
			for _, sp := range out.Spells {
				healing += sp.Healing
				absorbed += sp.Absorbed
			}
			near(t, out.HPS, healing/360)
			near(t, out.AbsorbPS, absorbed/360)
			other := anomaly
			if id == anomaly {
				other = barrier
			}
			if out.Spells[other].Casts != 0 {
				t.Fatal("policy cast unavailable replacement")
			}
		}
	}
}
