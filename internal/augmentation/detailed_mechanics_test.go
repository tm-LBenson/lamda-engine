package augmentation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackgroundMechanicUpgradeAcrossExistingCharacterHistory(t *testing.T) {
	lines := []string{record(0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")}
	for i, owner := range []string{aug, other} {
		id, effect, name := "3470", "1287434", "Essence Rend"
		if i == 1 {
			id, effect, name = "3445", "1284471", "Blighted Blood"
		}
		at := i*20 + 1
		f := append(actor("SPELL_DISPEL", owner, owner, 528), effect, name, "20", "DEBUFF")
		f[3] = "0x511"
		lines = append(lines, record(at, "ENCOUNTER_START", id, "Boss", "15", "20"), record(at+1, f...), record(at+10, "ENCOUNTER_END", id, "Boss", "15", "20", "1"))
	}
	path := filepath.Join(t.TempDir(), "original.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, retainFirst := range []bool{true, false} {
		s := ready()
		for _, line := range lines {
			s.Ingest(line, s.last)
		}
		if !retainFirst {
			s.damageMeters = s.damageMeters[:1]
			s.bossAttempts = s.bossAttempts[:1]
		}
		for i, m := range s.damageMeters {
			c := *m.Utility
			c.AwardVersion = 1
			c.MechanicTotals = nil
			m.Utility = &c
			s.damageMeters[i] = m
		}
		s.bossAttempts = nil
		for _, m := range s.damageMeters {
			s.rememberBossAttemptLocked(m)
		}
		s.prepareSummaryHistory(context.Background(), path, other)
		if s.own != other {
			t.Fatal("live owner changed", s.own)
		}
		if len(s.bossAttempts) != len(s.damageMeters) {
			t.Fatal("unretained character imported", len(s.bossAttempts), len(s.damageMeters))
		}
		for _, m := range s.bossAttempts {
			if m.Utility.AwardVersion != awardStatsVersion {
				t.Fatal("archive not upgraded", m.Utility)
			}
			if m.Owner == aug && m.Utility.MechanicTotals["dispel/1287434"][aug] != 1 {
				t.Fatal(m.Utility)
			}
		}
	}
}

func enemyMechanicDamage(s *Store, sec int, id int, name, target string) {
	f := actor("SPELL_DAMAGE", "Creature-0-1-3004-1-257347-1", target, id)
	f[3], f[7], f[10] = "0xa48", "0x514", name
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, "100", "100", "-1", "8", "0", "0", "0", "nil", "nil", "nil", "AOE")
	ingest(s, sec, f...)
}

func mechanicAura(s *Store, sec int, event, guid string, id int) {
	f := actor(event, "Creature-0-1-3004-1-257347-1", guid, id)
	f[3], f[7] = "0xa48", "0x514"
	ingest(s, sec, append(f, "DEBUFF")...)
}

func TestBombCourierNeedsNaturalExpiryAndDetonationNotJustTargetOrBlast(t *testing.T) {
	for _, mode := range []string{"complete", "target-only", "blast-only", "early-remove", "death", "dispel"} {
		t.Run(mode, func(t *testing.T) {
			s := ready()
			ingest(s, 1, "ENCOUNTER_START", "3420", "Sszorak", "15", "20")
			damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
			if mode != "blast-only" {
				mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1305963)
			}
			if mode == "early-remove" {
				mechanicAura(s, 8, "SPELL_AURA_REMOVED", other, 1305963)
			} else if mode != "target-only" {
				mechanicAura(s, 13, "SPELL_AURA_REMOVED", other, 1305963)
			}
			if mode != "target-only" {
				enemyMechanicDamage(s, 13, 1306120, "Venomous Surge", aug)
				enemyMechanicDamage(s, 13, 1306120, "Venomous Surge", aug) // another raid hit, never another drop
				mechanicAura(s, 13, "SPELL_AURA_REMOVED", other, 1305963)
			}
			if mode == "death" {
				ingest(s, 13, actor("UNIT_DIED", "Creature-Boss", other, 1)...)
			}
			if mode == "dispel" {
				f := append(actor("SPELL_DISPEL", aug, other, 528), "1305963", "Venomous Surge", "8", "DEBUFF")
				ingest(s, 13, f...)
			}
			ingest(s, 20, "ENCOUNTER_END", "3420", "Sszorak", "15", "20", "1")
			c := s.damageMeters[0].Utility
			want := int64(0)
			if mode == "complete" {
				want = 1
			}
			if c.MechanicTotals["bomb/drop"][other] != want || c.MechanicTotals["bomb/drop"][aug] != 0 {
				t.Fatal(mode, c.MechanicTotals)
			}
			r, err := s.PreparedSummaryForPlayer("boss", aug)
			if err != nil || strings.Contains(r.Text, "Bomb Courier:") != (want > 0) {
				t.Fatal(r, err)
			}
			if strings.Contains(r.Text, "correct marker") {
				t.Fatal("unobserved placement quality claimed", r)
			}
		})
	}
}

func TestMutilateGoodSoaksExcludeAlreadyGashedHeroicDoubleDips(t *testing.T) {
	for _, difficulty := range []string{"14", "15", "16"} {
		s := ready()
		ingest(s, 1, "ENCOUNTER_START", "3420", "Sszorak", difficulty, "20")
		damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
		enemyMechanicDamage(s, 3, 1285999, "Mutilate", aug)
		mechanicAura(s, 3, "SPELL_AURA_APPLIED", aug, 1277051)
		mechanicAura(s, 12, "SPELL_AURA_APPLIED", aug, 1277051) // a reapplication precedes the next hit
		enemyMechanicDamage(s, 12, 1285999, "Mutilate", aug)
		enemyMechanicDamage(s, 13, 1285999, "Mutilate", aug)
		mechanicAura(s, 20, "SPELL_AURA_REMOVED", aug, 1277051)
		enemyMechanicDamage(s, 21, 1285999, "Mutilate", aug)
		ingest(s, 30, "ENCOUNTER_END", "3420", "Sszorak", difficulty, "20", "1")
		wantGood, wantBad := int64(2), int64(1)
		if difficulty == "14" {
			wantGood, wantBad = 3, 0
		}
		c := s.damageMeters[0].Utility
		if c.Soaks[aug] != wantGood || c.MechanicTotals["fail/double_mutilate"][aug] != wantBad {
			t.Fatal(difficulty, c)
		}
	}
}

func TestSpecificDispelInterruptAndAuraCountersAreBossScoped(t *testing.T) {
	for _, id := range []string{"3497", "3445"} {
		s := ready()
		ingest(s, 1, "ENCOUNTER_START", id, "Boss", "15", "20")
		damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
		f := actor("SPELL_INTERRUPT", aug, councilFishTarget, 147362)
		f[7] = "0xa48"
		ingest(s, 3, append(f, "1286922", "Icebound Flames", "20")...)
		ingest(s, 4, append(actor("SPELL_DISPEL", aug, other, 528), "1286922", "Icebound Flames", "20", "DEBUFF")...)
		mechanicAura(s, 5, "SPELL_AURA_APPLIED", other, 1306119)
		ingest(s, 10, "ENCOUNTER_END", id, "Boss", "15", "20", "1")
		c := s.damageMeters[0].Utility
		want := int64(0)
		if id == "3497" {
			want = 1
		}
		if c.MechanicTotals["interrupt/1286922"][aug] != want || c.MechanicTotals["dispel/1286922"][aug] != want || c.MechanicTotals["aura/1306119"][other] != 0 {
			t.Fatal(id, c.MechanicTotals)
		}
	}
}

func TestIncompleteImpactDoesNotSuppressOtherKnownBossMechanics(t *testing.T) {
	s := ready()
	for _, start := range []int{1, 21} {
		ingest(s, start, "ENCOUNTER_START", "3420", "Sszorak", "15", "20")
		damageRecord(s, start+1, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
		enemyMechanicDamage(s, start+2, 1285999, "Mutilate", aug)
		enemyMechanicDamage(s, start+3, 1305998, "Caustic Claws", aug)
		if start == 1 {
			f := actor("SPELL_DAMAGE", "Creature-Boss", aug, 1305998)
			f[3] = "0xa48"
			f[10] = "Caustic Claws"
			ingest(s, start+4, f...)
		}
		ingest(s, start+10, "ENCOUNTER_END", "3420", "Sszorak", "15", "20", "0")
	}
	m, pulls := s.bossProgressionLocked(s.damageMeters[0])
	if len(pulls) != 2 || !m.Utility.MechanicUnknown["avoid/1305998"] || m.Utility.MechanicUnknown["soak/Mutilate"] {
		t.Fatal(m.Utility)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Mutilate Mutual:") || strings.Contains(r.Text, "Puddle Patron:") {
		t.Fatal(r, err)
	}
}

func TestMechanicAuraCountsApplicationsNotRefreshesOrDuplicateRecords(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3492", "Ula'tek", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1306119)
	mechanicAura(s, 3, "SPELL_AURA_APPLIED", other, 1306119)
	mechanicAura(s, 4, "SPELL_AURA_REFRESH", other, 1306119)
	mechanicAura(s, 5, "SPELL_AURA_REMOVED", other, 1306119)
	mechanicAura(s, 7, "SPELL_AURA_APPLIED", other, 1306119)
	ingest(s, 10, "ENCOUNTER_END", "3492", "Ula'tek", "15", "20", "1")
	if s.damageMeters[0].Utility.MechanicTotals["aura/1306119"][other] != 2 {
		t.Fatal(s.damageMeters[0].Utility)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Fossil Collector:") {
		t.Fatal(r, err)
	}
}

func TestEachReviewedBossHasBoundedSpecificAwardsAndOldCoverageStaysUnknown(t *testing.T) {
	for id, defs := range encounterAwards {
		m := summaryFixture()
		m.ID = "boss/" + id + "/1"
		m.Kind = "boss"
		m.Seconds = 20
		c := &summaryCounts{Known: true, MechanicsKnown: true}
		initAggregateAwardCounts(c)
		for _, a := range defs {
			for _, key := range a.keys {
				c.MechanicTotals[key] = map[string]int64{aug: 1234}
			}
		}
		m.Utility = c
		good, bad, _, _, _, _ := detailedBossAwards(m)
		if len(good) == 0 || len(good) > bossAwardLimit || len(bad) > bossAwardLimit {
			t.Fatal(id, good, bad)
		}
		for _, line := range append(good, bad...) {
			if len(line)+len("[LamdaMeters] ") > 255 {
				t.Fatal(id, line)
			}
		}
		c.AwardVersion = awardStatsVersion - 1
		good, bad, _, _, _, _ = detailedBossAwards(m)
		if len(good)+len(bad) != 0 {
			t.Fatal("old report was treated as measured", id)
		}
	}
}
