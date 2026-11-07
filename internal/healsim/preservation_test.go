package healsim

import (
	"context"
	"math"
	"testing"
)

func preservationFixture() *simulation {
	st := testStats()
	st.Crit = 0
	st.Mastery = 0
	st.Versatility = 0
	st.Talents = map[int]int{364343: 1, 366155: 1, 360995: 1, 355936: 1, 363534: 1, 370537: 1}
	return newSimulation(st, testOptions(), "consensus", 1729)
}
func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("got %.9f, want %.9f", got, want)
	}
}

func TestRewindSnapshotsHistoryAndIgnoresHealingModifiers(t *testing.T) {
	for _, raid := range []bool{false, true} {
		s := preservationFixture()
		s.stats.SpellPower = 1000
		s.stats.Haste = 1
		s.stats.Crit = 1
		s.stats.Mastery = 2
		s.stats.Versatility = 1
		if raid {
			s.options.Scenario = "raid"
		}
		s.people[1].hp = 1000
		s.people[1].damage[0] = 100000
		s.startRewind()
		want := 90000.
		if raid {
			want = 30000
		}
		near(t, s.people[1].hp, 1000+want/5)
		s.people[1].damage[1] = 900000 // Later damage must not increase the fixed HoT budget.
		for i := 1; i <= 4; i++ {
			s.now = float64(i) / 2
			s.tickPreservation(false)
		}
		near(t, s.people[1].hp, 1000+want)
		if len(s.rewinds) != 0 {
			t.Fatal("wrong pulse count or haste duration")
		}
	}
}
func TestRewindMinimumAndDeathBoundaries(t *testing.T) {
	s := preservationFixture()
	s.stats.SpellPower = 1000
	s.stats.Haste = 0
	s.people[1].hp = 1000
	s.people[2].dead = true
	s.people[2].hp = 0
	s.startRewind()
	near(t, s.people[1].hp, 2200) // 2*SP*3 / five pulses.
	s.people[1].dead = true
	s.people[1].hp = 0
	s.now = 4
	s.tickPreservation(false)
	if s.people[1].hp != 0 || s.people[2].hp != 0 {
		t.Fatal("Rewind resurrected a dead target")
	}
}
func TestRewindCostCooldownAndEchoIndependence(t *testing.T) {
	st := testStats()
	st.Talents[363534] = 1
	st.Talents[381922] = 1
	s := newSimulation(st, testOptions(), "consensus", 1)
	s.now = 12
	s.people[1].echo = .7
	s.people[1].echoEnd = 30
	s.cast(rewind, 1, false)
	near(t, s.mana, st.Mana*(1-.055))
	near(t, s.ready[rewind][0], 192)
	if s.people[1].echo != .7 {
		t.Fatal("Rewind consumed Echo")
	}
}
func fillStasis(s *simulation) {
	s.cast(stasis, 0, true)
	s.now = 1
	s.cast(echo, 1, true)
	s.now = 3
	s.cast(embrace, 2, true)
	s.now = 5
	s.cast(reversion, 3, true)
}
func TestStasisOrderedFreeReplayPreservesTargetsAndCooldowns(t *testing.T) {
	s := preservationFixture()
	fillStasis(s)
	if len(s.stasisState.spells) != 3 || s.stasisState.loading {
		t.Fatal("not loaded")
	}
	near(t, s.stasisState.expires, 35)
	near(t, s.ready[stasis][0], 95)
	mana, essence, embraceCD, reversionCD := s.mana, s.essence, s.ready[embrace][0], s.ready[reversion][0]
	for i := 1; i < 4; i++ {
		s.people[i].hp = 1000
	}
	s.stats.Versatility = .5 // Replays must use release-time stats, not the old heal amount.
	s.now = 7
	s.cast(stasisRelease, 0, true)
	s.tickPreservation(true)
	if s.people[1].echo == 0 {
		t.Fatal("first stored Echo not replayed")
	}
	near(t, s.people[2].hp, 1000) // Later spell must not fire early.
	s.now = 7.4
	s.tickPreservation(true)
	near(t, s.people[2].hp, 1000+s.spells[embrace].direct*s.stats.SpellPower*.8*1.5)
	s.now = 7.8
	s.tickPreservation(true)
	if _, ok := s.people[3].hots[reversion]; !ok {
		t.Fatal("Reversion lost its original target")
	}
	near(t, s.mana, mana)
	near(t, s.essence, essence)
	near(t, s.ready[embrace][0], embraceCD)
	near(t, s.ready[reversion][0], reversionCD)
	for _, id := range []int{echo, embrace, reversion} {
		if s.totals[id].Casts != 1 || s.totals[id].Replays != 1 {
			t.Fatal("replay inflated paid casts")
		}
	}
	var seen []Cast
	for _, cast := range s.result.Casts {
		if cast.Origin == "stasis" {
			seen = append(seen, cast)
		}
	}
	if len(seen) != 3 || seen[0].Target != 2 || seen[1].Target != 3 || seen[2].Target != 4 {
		t.Fatal("trace lost ordered original targets", seen)
	}
	if len(s.stasisState.spells) != 0 || len(s.stasisQueue) != 0 {
		t.Fatal("replay recursively stored itself")
	}
}
func TestStasisStoresOnlyCompletedEligibleSpellsAndAutomaticallyReleases(t *testing.T) {
	s := preservationFixture()
	s.cast(stasis, 0, false)
	if !math.IsInf(s.ready[stasis][0], 1) {
		t.Fatal("cooldown ran before three spells")
	}
	s.finish(rewind, 0)
	s.finish(flight, 0)
	if len(s.stasisState.spells) != 0 {
		t.Fatal("stored a major cooldown")
	}
	s.people[1].dead = true
	s.people[1].hp = 0
	s.finish(flame, 1)
	if len(s.stasisState.spells) != 0 {
		t.Fatal("stored failed target cast")
	}
	s.finish(reversion, 2)
	s.finish(reversion, 3)
	s.finish(reversion, 4)
	s.now = 30
	s.tickPreservation(true)
	if s.totals[stasisRelease].Casts != 1 || s.totals[reversion].Replays != 1 {
		t.Fatal("no automatic release")
	}
	s.now = 30.8
	s.tickPreservation(true)
	if s.totals[reversion].Replays != 3 {
		t.Fatal("lost queued replay")
	}
}
func TestStasisBlossomStoresAtLaunchAndCopiesBloomAfterDelay(t *testing.T) {
	s := preservationFixture()
	s.cast(stasis, 0, false)
	s.finish(echo, 1)
	s.finish(echo, 2)
	s.now = 2
	s.cast(blossom, 1, true)
	if len(s.stasisState.spells) != 3 || s.stasisState.spells[2].spell != blossom {
		t.Fatal("instant Blossom was not stored at cast completion")
	}
	near(t, s.ready[stasis][0], 92)
	near(t, s.stasisState.expires, 32)
	for i := range s.people {
		s.people[i].hp = 1000
	}
	s.now = 3.49
	s.tickPending()
	near(t, s.totals[blossom].Healing, 0)
	s.now = 3.5
	s.tickPending()
	paidHealing := s.totals[blossom].Healing
	if paidHealing <= 0 || len(s.stasisState.spells) != 3 {
		t.Fatal("paid blossom failed or was stored twice")
	}
	mana, essence := s.mana, s.essence
	s.now = 4
	s.cast(stasisRelease, 0, true)
	s.tickPreservation(true)
	s.now = 4.4
	s.tickPreservation(true)
	s.now = 4.8
	s.tickPreservation(true)
	s.tickPending()
	near(t, s.totals[blossom].Healing, paidHealing)
	s.now = 6.29
	s.tickPending()
	near(t, s.totals[blossom].Healing, paidHealing)
	s.now = 6.3
	s.tickPending()
	near(t, s.totals[blossom].Healing, paidHealing*2)
	near(t, s.mana, mana)
	near(t, s.essence, essence)
	if s.totals[blossom].Casts != 1 || s.totals[blossom].Replays != 1 || len(s.stasisState.spells) != 0 {
		t.Fatal("replay changed paid casts or recursively stored its delayed impact")
	}
}

func TestStasisCannotHealDeadTargetOrReleaseAfterHealerDeath(t *testing.T) {
	s := preservationFixture()
	fillStasis(s)
	s.people[2].dead = true
	s.people[2].hp = 0
	s.now = 7
	s.releaseStasis(false, false)
	s.now = 7.8
	s.tickPreservation(false)
	if s.people[2].hp != 0 {
		t.Fatal("replayed into a resurrection")
	}
	other := preservationFixture()
	fillStasis(other)
	other.people[0].dead = true
	other.now = 36
	other.tickPreservation(false)
	if other.totals[stasisRelease].Casts != 0 || len(other.stasisQueue) != 0 {
		t.Fatal("dead caster released spells")
	}
}
func TestReversionRefreshKeepsTickPhaseAndSeparateEcho(t *testing.T) {
	s := preservationFixture()
	s.stats.Haste = 0
	s.addHot(reversion, 1, 1, false)
	s.addHot(reversion, 1, .7, true)
	h := s.people[1].hots[reversion]
	h.last = 4
	h.next = 6
	s.people[1].hots[reversion] = h
	s.now = 5
	s.addHot(reversion, 1, 1, false)
	normal := s.people[1].hots[reversion]
	copied := s.people[1].hots[reversion+len(s.spells)]
	near(t, normal.end, 20.6)
	near(t, normal.next, 6)
	near(t, normal.last, 4)
	near(t, copied.end, 12)
	near(t, copied.factor, .7)
	s.addHot(reversion, 1, 1, false)
	near(t, s.people[1].hots[reversion].end, 20.6)
}
func TestReversionCritExtensionHasFinitePerApplicationBudget(t *testing.T) {
	st := testStats()
	st.Crit = 1
	st.Haste = 0
	st.Mastery = 0
	st.Versatility = 0
	st.ManaRegen = 0
	st.Talents = map[int]int{366155: 1}
	o := testOptions()
	o.Seconds = 30
	o.Scenario = "replay"
	o.Replay = &Replay{Schema: 1, Source: "fixture", Seconds: 30, Members: []ReplayMember{{"You", 800000}, {"Tank", 1000000}}, Events: []DamageEvent{{29, 1, 0}}}
	s := newSimulation(st, o, "reactive", 1)
	s.mana = 0
	s.people[1].hp = 100000
	s.addHot(reversion, 1, 1, false)
	if err := s.run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	// Six base ticks plus six extensions, each at guaranteed double strength.
	near(t, s.totals[reversion].Healing, 12*.4275*st.SpellPower*.8*2)
	if len(s.people[1].hots) != 0 {
		t.Fatal("unbounded extension")
	}
}
func TestGracePeriodStacksByVariantNotEchoStrength(t *testing.T) {
	s := preservationFixture()
	s.stats.Talents[376239] = 1
	s.people[1].hp = 1000
	s.addHot(reversion, 1, 1, false)
	s.addHot(reversion, 1, .3, true)
	s.healPreservation(flame, 1, 1000)
	near(t, s.people[1].hp, 1000+800*1.21)
	s.now = 13
	s.healPreservation(flame, 1, 1000)
	near(t, s.people[1].hp, 1000+800*1.21+800)
	s.people[1].damage[0] = 100000
	s.apply(reversion, 1, 1, false)
	near(t, s.totals[goldenHour].Healing, 0) // No Golden Hour talent in this fixture.
	s.stats.Talents[378196] = 1
	s.apply(reversion, 1, 1, false)
	near(t, s.totals[goldenHour].Healing, 15000) // Grace/crit/mastery never scale this transfer.
}
func TestPreservationTalentConfigurationIsPerSimulation(t *testing.T) {
	st := testStats()
	st.Talents[376240] = 2
	st.Talents[373834] = 1
	st.Talents[371257] = 2
	st.Talents[376150] = 1
	st.Talents[1264269] = 1
	s := newSimulation(st, testOptions(), "consensus", 1)
	near(t, s.spells[reversion].duration, 15.6)
	near(t, s.cost(reversion), st.Mana*.022*.7)
	near(t, s.spells[breath].direct, .9216*1.2*1.3)
	near(t, s.spells[flame].direct, 7.8*1.5*1.3)
	near(t, s.spells[breath].cd, 24)
	if len(s.ready[breath]) != 2 {
		t.Fatal("missing second Dream Breath charge")
	}
	s.finish(echo, 1)
	near(t, s.people[1].echoEnd, 19.5)
	base := newSimulation(testStats(), testOptions(), "consensus", 1)
	near(t, base.spells[reversion].duration, 12)
	near(t, base.spells[breath].direct, .9216)
	if len(base.ready[breath]) != 1 {
		t.Fatal("talents leaked to another simulation")
	}
}
func TestEchoCopiesHaveIndependentEssenceBurstRollsAndStackCap(t *testing.T) {
	st := testStats()
	st.Crit = 0
	st.Talents[375722] = 1
	found := false
	for seed := int64(0); seed < 100; seed++ {
		s := newSimulation(st, testOptions(), "consensus", seed)
		for i := range s.people {
			s.people[i].echo = .7
			s.people[i].echoEnd = 15
		}
		s.finish(reversion, 1)
		if s.burstStacks > 2 {
			t.Fatal("exceeded Essence Attunement cap")
		}
		if s.burstStacks == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("copied Reversions did not get independent proc rolls")
	}
}
func TestPreservationSpellbookValidationAndLegacyFallback(t *testing.T) {
	st := testStats()
	if err := validatePreservation(st); err != nil {
		t.Fatal(err)
	}
	st.KnownSpells = map[int]bool{}
	for i, sp := range spells {
		if i <= flight {
			st.KnownSpells[sp.id] = true
		}
	}
	if err := validatePreservation(st); err != nil {
		t.Fatal(err)
	}
	s := newSimulation(st, testOptions(), "consensus", 1)
	if s.available(stasis) || s.available(rewind) {
		t.Fatal("uncaptured cooldown invented")
	}
	delete(st.KnownSpells, 361469)
	if validatePreservation(st) == nil {
		t.Fatal("partial core spellbook accepted")
	}
	st.KnownSpells[361469] = false
	for id := range st.KnownSpells {
		st.KnownSpells[id] = false
	}
	if validatePreservation(st) == nil {
		t.Fatal("all unavailable spells accepted")
	}
	st.KnownSpells = nil
	st.Talents[376240] = 10
	if validatePreservation(st) == nil {
		t.Fatal("impossible rank accepted")
	}
}
func TestHistoryAgesOutAtFiveSeconds(t *testing.T) {
	s := preservationFixture()
	s.options.Replay = &Replay{Events: []DamageEvent{{0, 1, 100}}}
	for tick := 0; tick <= 100; tick++ {
		s.now = float64(tick) * .05
		s.replayDamage(tick)
	}
	near(t, s.recent(1), 0)
}

func TestPreservationCooldownsRunThroughFullEncounterAccounting(t *testing.T) {
	st := testStats()
	delete(st.Talents, 359816)
	st.Talents[370537] = 1
	st.Talents[363534] = 1
	st.Talents[376239] = 1
	st.Talents[375722] = 1
	st.KnownSpells = map[int]bool{}
	for i, sp := range spells {
		if !sp.passive && i != stasisRelease {
			st.KnownSpells[sp.id] = i != flight && i < anomaly
		}
	}
	o := testOptions()
	o.Seconds = 180
	o.Runs = 2
	r, err := Run(context.Background(), st, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range r.Outcomes {
		if out.Spells[stasis].Casts == 0 || out.Spells[stasisRelease].Casts == 0 || out.Spells[rewind].Casts == 0 {
			t.Fatal("new cooldowns absent from policy")
		}
		healing, replays := 0., 0
		for _, sp := range out.Spells {
			healing += sp.Healing
			replays += sp.Replays
		}
		near(t, out.HPS, healing/float64(o.Seconds*o.Runs))
		if replays < 3 || out.Spells[flight].Casts != 0 {
			t.Fatal("missing replay or unavailable spell cast")
		}
		for _, p := range out.Timeline {
			if p.Mana < 0 || p.Mana > 100 || p.Essence < 0 || p.Essence > 5 {
				t.Fatal("resource bounds violated")
			}
		}
	}
}
