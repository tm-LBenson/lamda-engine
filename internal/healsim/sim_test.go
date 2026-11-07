package healsim

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

func testStats() Stats {
	return Stats{SpellPower: 20000, Health: 800000, Mana: 250000, ManaRegen: 1250, Crit: .2, Haste: .2, Mastery: .4, Versatility: .05, Talents: map[int]int{364343: 1, 366155: 1, 355936: 1, 360995: 1, 359816: 1, 369297: 1, 378196: 1}}
}
func testOptions() Options {
	return Options{Scenario: "dungeon", Seconds: 60, Intensity: 100, Runs: 3, Policy: "compare", Seed: 1729}
}
func TestReproducibleBoundedEncounter(t *testing.T) {
	a, e := Run(context.Background(), testStats(), testOptions())
	if e != nil {
		t.Fatal(e)
	}
	b, e := Run(context.Background(), testStats(), testOptions())
	if e != nil {
		t.Fatal(e)
	}
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(aa) != string(bb) {
		t.Fatal("seeded simulation is not reproducible")
	}
	if len(a.Outcomes) != 2 || a.Outcomes[0].HPS <= 0 {
		t.Fatal("no comparative throughput")
	}
	for _, out := range a.Outcomes {
		if out.RawHPS < out.HPS || out.Deaths < 0 || out.Deaths > 5 || out.OverhealPercent < 0 || out.OverhealPercent > 100 {
			t.Fatal("invalid accounting", out)
		}
		for _, p := range out.Timeline {
			if p.Mana < 0 || p.Mana > 100 || p.Essence < 0 || p.Essence > 5 {
				t.Fatal("unbounded resources", p)
			}
			for _, v := range p.Health {
				if v < 0 || v > 100 {
					t.Fatal("unbounded health", v)
				}
			}
		}
	}
}
func TestOverhealingAndDynamicMastery(t *testing.T) {
	s := newSimulation(testStats(), testOptions(), "consensus", 1)
	s.stats.Crit = 0
	s.stats.Versatility = 0
	s.stats.Talents = map[int]int{}
	s.people[1].hp -= 1000
	s.heal(flame, 1, 10000)
	if s.totals[flame].Healing != 1000 || math.Abs(s.totals[flame].Overheal-10200) > 1e-6 {
		t.Fatal("incorrect cap or mastery", s.totals[flame])
	}
	s.people[0].hp = .1 * s.people[0].max
	s.people[1].hp = .5 * s.people[1].max
	s.totals[flame] = SpellResult{}
	s.heal(flame, 1, 10000)
	if s.totals[flame].Healing != 8000 {
		t.Fatal("mastery applied while caster was lower")
	}
}
func TestNoHealingDeadTargetsOrFreeResurrection(t *testing.T) {
	s := newSimulation(testStats(), testOptions(), "consensus", 1)
	s.people[1].dead = true
	s.people[1].hp = 0
	s.apply(flame, 1, 1, false)
	if s.people[1].hp != 0 || s.totals[flame].Healing != 0 {
		t.Fatal("dead ally healed")
	}
	s.people[0].dead = true
	s.people[0].hp = 0
	s.people[2].hp -= 10000
	s.finish(flame, 2)
	if s.totals[flame].Healing != 0 {
		t.Fatal("dead healer cast")
	}
}
func TestChargesRechargeSequentially(t *testing.T) {
	s := newSimulation(testStats(), testOptions(), "consensus", 1)
	s.stats.Haste = 0
	s.cast(reversion, 1, false)
	s.now = 2
	s.cast(reversion, 1, false)
	if s.ready[reversion][0] != 9 || s.ready[reversion][1] != 18 {
		t.Fatal("parallel recharge", s.ready[reversion])
	}
	s.now = 8
	if s.can(reversion) {
		t.Fatal("charge ready early")
	}
	s.now = 9
	if !s.can(reversion) {
		t.Fatal("first charge missing")
	}
}
func TestEchoHoTsCoexistWithoutUnboundedRefreshStacks(t *testing.T) {
	s := newSimulation(testStats(), testOptions(), "consensus", 1)
	s.people[1].hp *= .5
	s.finish(echo, 1)
	s.finish(reversion, 1)
	if len(s.people[1].hots) != 2 || s.people[1].echo != 0 {
		t.Fatal("echo did not copy hot")
	}
	s.now = 1
	s.finish(reversion, 1)
	if len(s.people[1].hots) != 2 {
		t.Fatal("refresh stacked the same hot")
	}
	s.finish(echo, 1)
	s.finish(blossom, 1)
	if s.people[1].echo == 0 {
		t.Fatal("essence spell consumed echo")
	}
}
func TestSameDamageStreamEvenAfterDeath(t *testing.T) {
	a := newSimulation(testStats(), testOptions(), "consensus", 3)
	b := newSimulation(testStats(), testOptions(), "reactive", 3)
	a.people[2].dead = true
	for tick := 0; tick < 100; tick++ {
		a.damage(tick)
		b.damage(tick)
	}
	if a.people[1].hp != b.people[1].hp {
		t.Fatal("policy changed random incoming damage")
	}
}
func TestCancellationAndInvalidStats(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Run(ctx, testStats(), testOptions()); e != context.Canceled {
		t.Fatal(e)
	}
	s := testStats()
	s.SpellPower = math.NaN()
	if _, e := Run(context.Background(), s, testOptions()); e == nil {
		t.Fatal("accepted NaN")
	}
	s = testStats()
	s.Mana = 0
	if s.Validate() == nil {
		t.Fatal("missing mana became valid")
	}
}
func TestReplayExactDurationAndTargets(t *testing.T) {
	o := testOptions()
	o.Scenario = "replay"
	o.Seconds = 61
	o.Replay = &Replay{Schema: 1, Source: "test", Seconds: 60.125, Members: []ReplayMember{{"You", 800000}, {"Tank", 1000000}}, Events: []DamageEvent{{Time: 2, Target: 1, Amount: 250000}, {Time: 35, Target: 1, Amount: 250000}}}
	r, e := Run(context.Background(), testStats(), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Replay.Events) != 0 || r.GroupSize != 2 {
		t.Fatal("raw events leaked into envelope")
	}
	for _, out := range r.Outcomes {
		total := 0.
		for _, sp := range out.Spells {
			total += sp.Healing
		}
		want := total / float64(o.Runs) / 60.125
		if math.Abs(out.HPS-want) > 1e-7 {
			t.Fatal("HPS used rounded duration", out.HPS, want)
		}
	}
	o.Replay.Events[0].Target = 2
	if o.Validate() == nil {
		t.Fatal("accepted unknown replay target")
	}
}
func TestZeroDamageHasNoEffectiveHPS(t *testing.T) {
	o := testOptions()
	o.Scenario = "replay"
	o.Seconds = 30
	o.Replay = &Replay{Schema: 1, Source: "test", Seconds: 30, Members: []ReplayMember{{"You", 800000}, {"Tank", 1000000}}, Events: []DamageEvent{{Time: 2, Target: 1, Amount: 0}}}
	r, e := Run(context.Background(), testStats(), o)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range r.Outcomes {
		if v.HPS != 0 {
			t.Fatal("healing score invented HPS without damage")
		}
	}
}

func TestGuideAverageDeduplicatesAuthorsAndExcludesPopularity(t *testing.T) {
	value, n := averageOpinions(map[string]float64{"qe": 0, "wowhead": 1, "method": 0, "murlok": 1})
	if n != 2 || value != .5 {
		t.Fatal("duplicate author or popularity distorted consensus", value, n)
	}
}
func TestEssenceBurstMakesBothResourcesFreeAndExpires(t *testing.T) {
	s := newSimulation(testStats(), testOptions(), "consensus", 1)
	s.burstStacks = 1
	s.burstEnd = 15
	s.mana = 0
	s.essence = 0
	if !s.can(echo) {
		t.Fatal("free Echo rejected")
	}
	s.cast(echo, 1, false)
	if s.mana != 0 || s.essence != 0 || s.burstStacks != 0 || math.Abs(s.people[1].echo-.7*1.35) > 1e-8 {
		t.Fatal("proc consumption or strength wrong")
	}
}
