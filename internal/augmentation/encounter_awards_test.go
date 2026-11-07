package augmentation

import (
	"fmt"
	"strings"
	"testing"
)

func stackAura(s *Store, sec int, event, guid string, spell int, stacks string) {
	f := actor(event, "Creature-0-1-3004-1-257361-1", guid, spell)
	f[3], f[7] = "0xa48", "0x514"
	ingest(s, sec, append(f, "DEBUFF", stacks)...)
}

func TestEncounterObservationCountsAssignmentsNotTicksRefreshesOrDuplicatePackets(t *testing.T) {
	for boss, spells := range encounterObservations {
		for spell, key := range spells {
			t.Run(boss+"/"+spell, func(t *testing.T) {
				s := ready()
				ingest(s, 1, "ENCOUNTER_START", boss, "Boss", "15", "20")
				n := 0
				fmt.Sscan(spell, &n)
				mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, n)
				mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, n)
				mechanicAura(s, 4, "SPELL_AURA_REFRESH", other, n)
				stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", other, n, "3")
				mechanicAura(s, 6, "SPELL_AURA_REMOVED", other, n)
				mechanicAura(s, 7, "SPELL_AURA_APPLIED", other, n)
				if got := s.active["boss"].damage.mechanicTotals[key][other]; got != 2 {
					t.Fatalf("%s got %d assignments", key, got)
				}
				if strings.HasPrefix(key, "target/") && s.active["boss"].damage.mechanicTotals["target/all"][other] != 2 {
					t.Fatal("combined assignment count")
				}
				ingest(s, 10, "ENCOUNTER_END", boss, "Boss", "15", "20", "0")
			})
		}
	}
}

func TestAssignmentFamiliesCombineEffectVariantsAndRequireRepeatedSelections(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3420", "Sszorak", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1285425)
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1285453)
	mechanicAura(s, 6, "SPELL_AURA_APPLIED", other, 1297111)
	ingest(s, 10, "ENCOUNTER_END", "3420", "Sszorak", "15", "20", "0")
	m := s.damageMeters[0]
	if m.Utility.MechanicTotals["target/crosswinds"][other] != 2 {
		t.Fatal(m.Utility)
	}
	if strings.Contains(strings.Join(bossAwardLines(m, []DamageMeter{m}), "~"), "Frequent Flyer") {
		t.Fatal("two assignments called repeated targeting")
	}
	m.Utility.MechanicTotals["target/crosswinds"][other] = 3
	if !strings.Contains(strings.Join(bossAwardLines(m, []DamageMeter{m}), "~"), "Frequent Flyer") {
		t.Fatal("qualified repeated targeting missing")
	}
}

func TestCoiledAltarCreditsOrbPickupsWithoutClaimingCorrectPlacement(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3429", "The Coiled Altar", "16", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1282419)
	mechanicAura(s, 4, "SPELL_AURA_REFRESH", other, 1282419)
	mechanicAura(s, 9, "SPELL_AURA_APPLIED", other, 1282419)
	mechanicAura(s, 10, "SPELL_AURA_APPLIED", aug, 1310498)
	ingest(s, 20, "ENCOUNTER_END", "3429", "The Coiled Altar", "16", "20", "0")
	m := s.damageMeters[0]
	text := strings.Join(bossAwardLines(m, []DamageMeter{m}), "~")
	if m.Utility.MechanicTotals["use/venom_orb"][other] != 2 || !strings.Contains(text, "Orb Hauler") || !strings.Contains(text, "Purple Parcel Service") {
		t.Fatal(text, m.Utility)
	}
	for _, claim := range []string{"correct placement", "orbs cleared", "Sever clears"} {
		if strings.Contains(text, claim) {
			t.Fatal("unobserved geometry claim", text)
		}
	}
}

func TestStillStandingRanksSurvivedAssignmentsInsteadOfTyingEverySurvivor(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3429", "The Coiled Altar", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	for i := 0; i < 5; i++ {
		mechanicAura(s, 3+i*2, "SPELL_AURA_APPLIED", aug, 1283485)
		if i < 3 {
			mechanicAura(s, 3+i*2, "SPELL_AURA_APPLIED", other, 1285911)
		}
	}
	damageRecord(s, 14, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	damageRecord(s, 14, "SPELL_DAMAGE", other, "Creature-Boss", 100, 1, "")
	ingest(s, 15, "ENCOUNTER_END", "3429", "The Coiled Altar", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if c.MechanicTotals["finish/assignment_survivor"][aug] != 5 || c.MechanicTotals["finish/assignment_survivor"][other] != 3 {
		t.Fatal(c.MechanicTotals)
	}
}

func TestMythicRaidWideSplintersDoesNotAwardEveryRaiderForOpeningBoxes(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3497", "The Lost Explorers", "16", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", aug, 1308853)
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1308853)
	ingest(s, 10, "ENCOUNTER_END", "3497", "The Lost Explorers", "16", "20", "1")
	m := s.damageMeters[0]
	if len(m.Utility.MechanicTotals["use/crate"]) != 0 || strings.Contains(strings.Join(bossAwardLines(m, []DamageMeter{m}), "~"), "Crate Expectations") {
		t.Fatal("raid-wide bleed misattributed as a box opener", m.Utility)
	}
}

func TestTwinFangsFinishUsesObservedStacksAndIgnoresCleanup(t *testing.T) {
	for _, tc := range []struct {
		name              string
		setup             func(*Store)
		kill              bool
		nine, zero, exact bool
	}{
		{"nine-cleanup", func(s *Store) {
			stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "9")
			mechanicAura(s, 14, "SPELL_AURA_REMOVED", aug, 1290336)
		}, true, true, false, true},
		{"nine-then-eight", func(s *Store) {
			stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "9")
			stackAura(s, 8, "SPELL_AURA_REMOVED_DOSE", aug, 1290336, "8")
		}, true, true, false, false},
		{"zero-removal-first", func(s *Store) {
			mechanicAura(s, 8, "SPELL_AURA_REMOVED", aug, 1290336)
			mechanicAura(s, 8, "SPELL_AURA_APPLIED", aug, 1310096)
		}, true, false, true, false},
		{"zero-feast-first", func(s *Store) {
			mechanicAura(s, 8, "SPELL_AURA_APPLIED", aug, 1310096)
			mechanicAura(s, 8, "SPELL_AURA_REMOVED", aug, 1290336)
		}, true, false, true, false},
		{"cleanup-is-not-zero", func(s *Store) { mechanicAura(s, 14, "SPELL_AURA_REMOVED", aug, 1290336) }, true, false, false, false},
		{"nine-removal-is-not-zero", func(s *Store) {
			stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "9")
			mechanicAura(s, 8, "SPELL_AURA_REMOVED", aug, 1290336)
			mechanicAura(s, 8, "SPELL_AURA_APPLIED", aug, 1310096)
		}, true, false, false, false},
		{"death", func(s *Store) {
			stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "9")
			ingest(s, 14, actor("UNIT_DIED", "Creature-Boss", aug, 1)...)
		}, true, false, false, false},
		{"resurrection", func(s *Store) {
			stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "9")
			ingest(s, 8, actor("UNIT_DIED", "Creature-Boss", aug, 1)...)
			ingest(s, 9, actor("SPELL_RESURRECT", other, aug, 123)...)
		}, true, false, false, false},
		{"malformed-dose", func(s *Store) { stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "not-a-number") }, true, false, false, false},
		{"ten-stacks", func(s *Store) { stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "10") }, true, false, false, false},
		{"wipe", func(s *Store) { stackAura(s, 5, "SPELL_AURA_APPLIED_DOSE", aug, 1290336, "9") }, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ready()
			ingest(s, 1, "ENCOUNTER_START", "3421", "The Twin Fangs", "15", "20")
			damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
			mechanicAura(s, 3, "SPELL_AURA_APPLIED", aug, 1290336)
			tc.setup(s)
			// All scenarios end at the same time, with recent real participation.
			// Late packet order is deliberately not used to imply a resurrection.
			damageRecord(s, 14, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
			kill := "0"
			if tc.kill {
				kill = "1"
			}
			ingest(s, 14, "ENCOUNTER_END", "3421", "The Twin Fangs", "15", "20", kill)
			c := s.damageMeters[0].Utility
			for key, want := range map[string]bool{"finish/venom_nine_survived": tc.nine, "finish/venom_zero": tc.zero, "finish/venom_nine": tc.exact} {
				if (c.MechanicTotals[key][aug] > 0) != want {
					t.Fatalf("%s want %v got %v", key, want, c.MechanicTotals)
				}
			}
		})
	}
}

func TestTwinFangsZeroFinishRejectsLatePlayersAndRewardsObservedCleanParticipation(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3421", "The Twin Fangs", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1290336)
	late := "Player-1-Late"
	damageRecord(s, 55, "SPELL_DAMAGE", late, "Creature-Boss", 100, 1, "")
	damageRecord(s, 56, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	ingest(s, 60, "ENCOUNTER_END", "3421", "The Twin Fangs", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if c.MechanicTotals["finish/venom_zero"][aug] != 1 || c.MechanicTotals["finish/venom_zero"][late] != 0 {
		t.Fatal(c.MechanicTotals)
	}
}

func TestNewAwardCoverageDoesNotRelabelOldSavedHistoryAndCanUpgradeInBackground(t *testing.T) {
	m := summaryFixture()
	m.ID = "boss/3429/1"
	c := &summaryCounts{Known: true, AwardVersion: awardStatsVersion, MechanicTotals: map[string]map[string]int64{"use/venom_orb": {aug: 5}}}
	m.Utility = c
	if strings.Contains(strings.Join(bossAwardLines(m, []DamageMeter{m}), "~"), "Orb Hauler") {
		t.Fatal("old data claimed new coverage")
	}
	newer := *c
	newer.EncounterAwardVersion = encounterAwardVersion
	m.Utility = upgradeAwardCounts(c, &newer)
	if !strings.Contains(strings.Join(bossAwardLines(m, []DamageMeter{m}), "~"), "Orb Hauler") {
		t.Fatal("recovered coverage not used")
	}
}

func TestLargerAwardPoolKeepsWholeLinesAndExistingFastTransferBudget(t *testing.T) {
	m := summaryFixture()
	m.ID = "boss/3429/1"
	m.Kind = "boss"
	m.Seconds = 30
	m.Success = true
	c := &summaryCounts{Known: true, MechanicsKnown: true, Deaths: map[string]int64{other: 3}, Dispels: map[string]int64{aug: 9}}
	initAggregateAwardCounts(c)
	for _, a := range encounterAwards["3429"] {
		for _, k := range a.keys {
			c.MechanicTotals[k] = map[string]int64{aug: 999999999999999}
		}
	}
	m.Utility = c
	good, bad, _, _, _, _ := detailedBossAwards(m)
	if len(good) <= 2 || len(bad) <= 2 || len(good) > bossAwardLimit || len(bad) > bossAwardLimit {
		t.Fatal(good, bad)
	}
	r := progressionAwards(m, []DamageMeter{m}, makeChatSummary(m, *c))
	for _, options := range []uint8{0, 1, 2, 3, 4} {
		v := summaryWithOptions(r, options)
		if len(v.Text) > 4096 || len(v.Lines) > 50 {
			t.Fatal("transfer overflow", len(v.Text))
		}
		for _, line := range v.Lines {
			if len(line) > 255 {
				t.Fatal("chat line overflow", line)
			}
		}
	}
	if len(deathAwardTitles) < 12 {
		t.Fatal("death title rotation too small")
	}
	for i := 0; i < 20; i++ {
		if deathAwardLabel("boss/3429/1") != deathAwardLabel("boss/3429/1") {
			t.Fatal("refresh changed an award title")
		}
	}
}
