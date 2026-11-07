package healsim

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func coreStats(spec int) Stats {
	st := testStats()
	st.KnownSpells = map[int]bool{}
	for _, sp := range spellsFor(spec) {
		if !sp.passive {
			st.KnownSpells[sp.id] = true
		}
	}
	st.Talents = map[int]int{274586: 1}
	return st
}
func coreTestSim(spec int) *simulation {
	o := testOptions()
	o.Spec = spec
	st := coreStats(spec)
	st.Crit = 0
	return newSimulation(st, o, "consensus", 1729)
}

func TestAllHealersRunDeterministicallyWithTheirOwnSpells(t *testing.T) {
	for _, spec := range []int{65, 105, 256, 257, 264, 270, 1468} {
		for _, scenario := range []string{"tank", "dungeon", "raid", "replay"} {
			t.Run(SpecName(spec)+"/"+scenario, func(t *testing.T) {
				st := coreStats(spec)
				if spec == 1468 {
					st = testStats()
				}
				o := testOptions()
				o.Spec = spec
				o.Scenario = scenario
				o.Seconds = 30
				o.Runs = 2
				if scenario == "replay" {
					o.Replay = &Replay{Schema: 1, Source: "fixture", Seconds: 30, Members: []ReplayMember{{"You", 800000}, {"Tank", 1200000}}, Events: []DamageEvent{{1, 0, 200000}, {2, 1, 300000}, {12, 0, 200000}, {20, 1, 400000}}}
				}
				a, err := Run(context.Background(), st, o)
				if err != nil {
					t.Fatal(err)
				}
				b, err := Run(context.Background(), st, o)
				if err != nil {
					t.Fatal(err)
				}
				aa, _ := json.Marshal(a)
				bb, _ := json.Marshal(b)
				if string(aa) != string(bb) {
					t.Fatal("unstable fixed seed")
				}
				if a.Spec != spec || a.SpecName != SpecName(spec) || len(a.Outcomes) != 2 || len(a.Warnings) < 4 {
					t.Fatal("missing spec identity or coverage")
				}
				for _, out := range a.Outcomes {
					if out.HPS <= 0 || out.RawHPS < out.HPS || out.ManaEnd < 0 || out.ManaEnd > 100 || out.Deaths < 0 || out.Deaths > float64(a.GroupSize) {
						t.Fatal("invalid result", out)
					}
					total := 0.
					absorb := 0.
					for _, sp := range out.Spells {
						total += sp.Healing
						absorb += sp.Absorbed
						if spec != 1468 && (sp.Name == "Echo" || sp.Name == "Dream Breath") {
							t.Fatal("Evoker spells leaked")
						}
					}
					if math.Abs(total/float64(o.Runs*o.Seconds)-out.HPS) > 1e-5 || math.Abs(absorb/float64(o.Runs*o.Seconds)-out.AbsorbPS) > 1e-5 {
						t.Fatal("rate accounting")
					}
					for _, p := range out.Timeline {
						for _, hp := range p.Health {
							if hp < 0 || hp > 100 || math.IsNaN(hp) {
								t.Fatal("invalid HP")
							}
						}
						if p.Resource < 0 || p.Resource > 5 {
							t.Fatal("invalid Holy Power")
						}
					}
				}
			})
		}
	}
}
func TestMissingAndReplacedSpellsAreNotInvented(t *testing.T) {
	st := coreStats(65)
	delete(st.KnownSpells, 20473)
	if ValidateSpellbook(65, st) == nil {
		t.Fatal("partial capture accepted")
	}
	st.KnownSpells[20473] = false
	st.KnownSpells[85673] = false
	o := testOptions()
	o.Spec = 65
	r, e := Run(context.Background(), st, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, out := range r.Outcomes {
		for _, sp := range out.Spells {
			if (sp.Name == "Holy Shock" || sp.Name == "Word of Glory") && sp.Casts > 0 {
				t.Fatal("cast unavailable spell")
			}
		}
	}
	if !strings.Contains(strings.Join(r.Warnings, " "), "Holy Shock is unavailable") {
		t.Fatal("coverage missing")
	}
	o.Spec = 62
	if _, e = Run(context.Background(), st, o); e == nil {
		t.Fatal("damage spec ran healer")
	}
}
func TestConsumedShieldIsSeparateAndExpires(t *testing.T) {
	s := coreTestSim(256)
	id := s.spellIndex(17)
	s.finishCore(id, 1)
	shield := s.people[1].shield
	if shield <= 0 || s.totals[id].Absorbed != 0 || s.totals[id].Healing != 0 {
		t.Fatal("application credited")
	}
	if got := s.absorbDamage(1, shield*.4); got != 0 {
		t.Fatal("shield failed")
	}
	if math.Abs(s.totals[id].Absorbed-shield*.4) > 1e-6 {
		t.Fatal("absorb accounting")
	}
	s.now = 16
	if s.absorbDamage(1, 100) != 100 || s.people[1].shield != 0 {
		t.Fatal("expired shield protects")
	}
}
func TestAtonementCritAndVersatilityApplyOnce(t *testing.T) {
	s := coreTestSim(256)
	s.stats.Crit = 1
	s.stats.Versatility = .2
	s.people[1].hp = 10000
	s.people[1].atonement = 15
	id := s.spellIndex(585)
	s.atonementDamage(id)
	expected := s.spells[id].damage * s.stats.SpellPower * 1.2 * 2 * .46 * (1 + s.stats.Mastery) * 1.4
	got := s.totals[s.spellIndex(81751)].Healing
	if math.Abs(expected-got) > 1e-5 {
		t.Fatalf("double scaling: %.4f != %.4f", got, expected)
	}
	s.now = 16
	s.atonementDamage(id)
	if s.totals[s.spellIndex(81751)].Healing != got {
		t.Fatal("expired atonement healed")
	}
}
func TestHolyWordCooldownAndRollingMastery(t *testing.T) {
	s := coreTestSim(257)
	serenity := s.spellIndex(2050)
	s.ready[serenity][0] = 60
	s.people[1].hp = 10000
	s.finishCore(s.spellIndex(2061), 1)
	if s.ready[serenity][0] != 54 {
		t.Fatal("Flash Heal did not reduce Serenity")
	}
	pool := s.people[1].masteryPool
	s.now = 1
	s.finishCore(s.spellIndex(2061), 1)
	if s.people[1].masteryPool <= pool || s.people[1].masteryNext != 2 {
		t.Fatal("mastery refresh lost pool or delayed tick")
	}
	s.now = 2
	s.tickCore()
	if s.totals[s.spellIndex(77489)].Healing <= 0 {
		t.Fatal("mastery not ticking")
	}
}
func TestPrayerOfMendingRequiresSurvivingDamage(t *testing.T) {
	s := coreTestSim(257)
	id := s.spellIndex(33076)
	s.finishCore(id, 1)
	if s.totals[id].Healing != 0 || s.people[1].pom != 5 {
		t.Fatal("healed at application")
	}
	s.people[1].hp -= 100000
	s.damageTriggers(1)
	if s.totals[id].Healing <= 0 || s.people[1].pom != 0 {
		t.Fatal("did not trigger and jump")
	}
	stacks := 0
	for _, p := range s.people {
		stacks += p.pom
	}
	if stacks != 4 {
		t.Fatal("jump duplicated stacks")
	}
	s.finishCore(id, 1)
	s.people[1].dead = true
	before := s.totals[id].Healing
	s.damageTriggers(1)
	if s.totals[id].Healing != before {
		t.Fatal("resurrected lethal target")
	}
}
func TestPaladinPowerAndBeaconAvoidDoubleScaling(t *testing.T) {
	s := coreTestSim(65)
	spender := s.spellIndex(85673)
	if s.can(spender) {
		t.Fatal("spent no Holy Power")
	}
	for i := 0; i < 3; i++ {
		s.finishCore(s.spellIndex(19750), 1)
	}
	if s.holyPower != 3 || !s.can(spender) {
		t.Fatal("failed generation")
	}
	s.castCore(spender, 1, false)
	if s.holyPower != 0 {
		t.Fatal("power not spent")
	}
	s.finishCore(s.spellIndex(53563), 1)
	s.people[1].hp = 10000
	s.people[2].hp = 10000
	id := s.spellIndex(20473)
	s.finishCore(id, 2)
	heal := s.totals[id].Healing
	transfer := s.totals[s.spellIndex(53652)].Healing
	if math.Abs(transfer-heal*.1) > 1e-5 {
		t.Fatal("beacon double-scaled", heal, transfer)
	}
}
func TestDruidLateBloomAndSwiftmendConsumption(t *testing.T) {
	s := coreTestSim(105)
	bloom := s.spellIndex(33763)
	s.finishCore(bloom, 1)
	s.people[1].hp = 10000
	s.now = 12
	s.finishCore(bloom, 1)
	if s.totals[bloom].Healing <= 0 {
		t.Fatal("late refresh did not bloom")
	}
	s.finishCore(s.spellIndex(774), 1)
	if !s.hotActive(1, 774) {
		t.Fatal("no Rejuvenation")
	}
	s.finishCore(s.spellIndex(18562), 1)
	if s.hotActive(1, 774) {
		t.Fatal("Swiftmend did not consume HoT")
	}
}
func TestShamanMasteryAndChainDecay(t *testing.T) {
	s := coreTestSim(264)
	s.stats.Mastery = 0
	for i := range s.people {
		s.people[i].hp = s.people[i].max * .1
	}
	id := s.spellIndex(1064)
	s.finishCore(id, 0)
	base := s.spells[id].direct * s.stats.SpellPower * (1 + s.stats.Versatility)
	for i := 0; i < 4; i++ {
		got := s.people[i].hp - s.people[i].max*.1
		if math.Abs(got-base*math.Pow(.7, float64(i))) > 1e-5 {
			t.Fatal("wrong chain bounce", i, got)
		}
	}
	a := coreTestSim(264)
	b := coreTestSim(264)
	a.people[1].hp = a.people[1].max * .1
	b.people[1].hp = b.people[1].max * .6
	a.finishCore(0, 1)
	b.finishCore(0, 1)
	if a.totals[0].Healing <= b.totals[0].Healing {
		t.Fatal("Deep Healing ignored health")
	}
}
func TestMistweaverCleaveAndGust(t *testing.T) {
	s := coreTestSim(270)
	for i := range s.people {
		s.people[i].hp = 10000
	}
	s.finishCore(s.spellIndex(115151), 1)
	if s.totals[s.spellIndex(191894)].Healing <= 0 {
		t.Fatal("no Gust")
	}
	s.finishCore(s.spellIndex(116670), 2)
	if s.totals[s.spellIndex(425804)].Healing <= 0 {
		t.Fatal("no Renewing Mist cleave")
	}
	s.now = 21
	before := s.totals[s.spellIndex(425804)].Healing
	s.finishCore(s.spellIndex(116670), 2)
	if s.totals[s.spellIndex(425804)].Healing != before {
		t.Fatal("cleaved expired HoT")
	}
}
func TestChannelPulsesBeforeCompletionAndStopsOnDeath(t *testing.T) {
	s := coreTestSim(105)
	for i := range s.people {
		s.people[i].hp = 10000
	}
	id := s.spellIndex(740)
	s.castCore(id, 1, true)
	if s.totals[id].Healing <= 0 || s.nextAction <= s.now {
		t.Fatal("channel delayed all healing or failed to occupy caster")
	}
	before := s.totals[id].Healing
	s.people[0].dead = true
	s.now = 2
	s.tickCore()
	if s.totals[id].Healing != before || len(s.ground) != 0 {
		t.Fatal("dead caster continued channel")
	}
}
