package augmentation

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Optional read-only replay of the task's bounded original-log extraction.
func TestVoidscarOriginalLogAudit(t *testing.T) {
	path := os.Getenv("LAMDA_VOIDSCAR_AUDIT_LOG")
	if path == "" {
		t.Skip("original key replay not requested")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := New("")
	defer s.Close()
	s.SetEnabled(true)
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 2<<20)
	for scan.Scan() {
		s.Ingest(scan.Text(), time.Now())
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	results := []DamageMeter{}
	for _, m := range s.damageMeters {
		if !voidscarBoss(encounterID(m)) && awardScope(m) != voidscarKeyScope {
			continue
		}
		results = append(results, m)
		for _, line := range bossAwardLines(m, []DamageMeter{m}) {
			if len(line) > 255 {
				t.Fatal("oversized line", line)
			}
		}
	}
	if len(results) == 0 {
		t.Fatal("no original Voidscar encounters loaded")
	}
	if output := os.Getenv("LAMDA_VOIDSCAR_AUDIT_OUT"); output != "" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range results {
		t.Log(m.Kind, m.Name, len(m.Rows), m.Utility.EncounterAwardVersion)
	}
}

func voidscarRun(s *Store) {
	ingest(s, 1, "CHALLENGE_MODE_START", "Voidscar Arena", "2923", "585", "10")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-0-1-2923-1-238883-1", 100, 1, "")
}
func voidscarCast(s *Store, sec int, source, target string, spell int) {
	f := actor("SPELL_CAST_SUCCESS", source, target, spell)
	f[3], f[7] = "0xa48", "0x514"
	ingest(s, sec, f...)
}
func voidscarPurge(s *Store, sec int, source, target string, spell int) {
	f := actor("SPELL_DISPEL", source, target, 528)
	f[3], f[7] = "0x511", "0xa48"
	f = append(f, strconv.Itoa(spell), "Bolster", "1", "BUFF")
	ingest(s, sec, f...)
}

func TestVoidscarBossRulesAndEndOfKeyReportKeepBossAndTrashCounters(t *testing.T) {
	s := ready()
	voidscarRun(s)
	f := actor("SPELL_INTERRUPT", aug, "Creature-0-1-2923-1-252072-1", 147362)
	f[7] = "0xa48"
	ingest(s, 3, append(f, "1299938", "Shadowbolt Volley", "32")...)
	ingest(s, 5, "ENCOUNTER_START", "3286", "Atroxus", "8", "5", "2923")
	damageRecord(s, 6, "SPELL_DAMAGE", aug, "Creature-0-1-2923-1-239070-1", 200, 1, "")
	enemyMechanicDamage(s, 7, 1222484, "Poison Pool", other)
	damageRecord(s, 34, "SPELL_DAMAGE", aug, "Creature-0-1-2923-1-239008-1", 100, 1, "")
	ingest(s, 35, "ENCOUNTER_END", "3286", "Atroxus", "8", "5", "1")
	voidscarPurge(s, 36, aug, "Creature-0-1-2923-1-243985-1", 1310319)
	ingest(s, 40, "CHALLENGE_MODE_END", "2923", "1")
	var key, boss DamageMeter
	for _, m := range s.damageMeters {
		if m.Kind == "key" {
			key = m
		} else if encounterID(m) == "3286" {
			boss = m
		}
	}
	if key.Utility.MechanicTotals["work/239070"][aug] != 200 || boss.Utility.MechanicTotals["work/239070"][aug] != 200 {
		t.Fatal("boss/key work counted incorrectly", key.Utility, boss.Utility)
	}
	if key.Utility.MechanicTotals["purge/1310319"][aug] != 1 || boss.Utility.MechanicTotals["purge/1310319"][aug] != 0 {
		t.Fatal("trash purge crossed scopes")
	}
	if key.Utility.MechanicTotals["finish/clean_atrox"][aug] != 1 || key.Utility.MechanicTotals["finish/clean_atrox"][other] != 0 {
		t.Fatal("boss finish was not retained in key")
	}
	r, err := s.PreparedSummaryForPlayer("key", aug, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Creeper Keeper", "Volume Control", "Anger Management", "Pool Membership", "Clean Boots"} {
		if !strings.Contains(r.Text, name) {
			t.Fatal(name, r.Text)
		}
	}
	if strings.Contains(r.Text, "one pull, one kill") || strings.Contains(r.Text, "First-Try Finish") || strings.Contains(r.Text, "1. Aug") {
		t.Fatal("key awards had boss-only completion or rankings", r.Text)
	}
	data, err := json.Marshal(s.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "history.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded := New(path)
	defer loaded.Close()
	restored, err := loaded.PreparedSummaryForPlayer("key", aug, 1)
	if err != nil || !strings.Contains(restored.Text, "Creeper Keeper") {
		t.Fatal("awards did not survive restart", restored, err)
	}
}

func TestVoidscarTazDashMatchesSelectedPlayerInEitherLogOrder(t *testing.T) {
	for _, order := range []string{"selection-first", "hit-first", "missing-selection"} {
		t.Run(order, func(t *testing.T) {
			s := ready()
			voidscarRun(s)
			ingest(s, 5, "ENCOUNTER_START", "3285", "Taz'Rah", "8", "5", "2923")
			caster := "Creature-0-1-2923-1-254677-1"
			selectTarget := func() { voidscarCast(s, 6, caster, aug, 1222100) }
			if order == "selection-first" {
				selectTarget()
			}
			for _, target := range []string{aug, other} {
				f := actor("SPELL_DAMAGE", caster, target, 1222103)
				f[3], f[7], f[10] = "0xa48", "0x514", "Nether Dash"
				for i := 0; i < 19; i++ {
					f = append(f, "0")
				}
				f = append(f, "100", "100", "-1", "32", "0", "0", "0", "nil", "nil", "nil", "AOE")
				ingest(s, 6, f...)
			}
			if order == "hit-first" {
				selectTarget()
			}
			ingest(s, 30, "ENCOUNTER_END", "3285", "Taz'Rah", "8", "5", "0")
			var m DamageMeter
			for _, v := range s.damageMeters {
				if v.Kind == "boss" {
					m = v
				}
			}
			c := m.Utility
			if order == "missing-selection" {
				if !c.MechanicUnknown["avoid/extra_dash"] || c.MechanicTotals["avoid/extra_dash"][aug] != 0 {
					t.Fatal(c)
				}
				return
			}
			if c.MechanicTotals["target/dash"][aug] != 1 || c.MechanicTotals["avoid/extra_dash"][aug] != 0 || c.MechanicTotals["avoid/extra_dash"][other] != 100 {
				t.Fatal(c.MechanicTotals)
			}
		})
	}
}

func TestVoidscarMandatoryBloomAndTankHitsAreNotAvoidable(t *testing.T) {
	s := ready()
	voidscarRun(s)
	ingest(s, 5, "ENCOUNTER_START", "3285", "Taz'Rah", "8", "5", "2923")
	enemyMechanicDamage(s, 6, 1300259, "Dark Bloom", aug)
	enemyMechanicDamage(s, 7, 1297017, "Void Blast", aug)
	enemyMechanicDamage(s, 8, 1300262, "Dark Bloom", other)
	damageRecord(s, 34, "SPELL_DAMAGE", aug, "Creature-0-1-2923-1-238887-1", 100, 1, "")
	ingest(s, 35, "ENCOUNTER_END", "3285", "Taz'Rah", "8", "5", "1")
	c := s.damageMeters[0].Utility
	if c.Avoidable[aug] != 0 || c.Avoidable[other] != 100 || c.MechanicTotals["finish/clean_taz"][aug] != 1 || c.MechanicTotals["finish/clean_taz"][other] != 0 {
		t.Fatal(c)
	}
}

func TestVoidscarPoisonDebuffWithoutDamageStillDisqualifiesCleanBoots(t *testing.T) {
	s := ready()
	voidscarRun(s)
	ingest(s, 5, "ENCOUNTER_START", "3286", "Atroxus", "8", "5", "2923")
	damageRecord(s, 6, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	mechanicAura(s, 7, "SPELL_AURA_APPLIED", aug, 1263971)
	damageRecord(s, 34, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	ingest(s, 35, "ENCOUNTER_END", "3286", "Atroxus", "8", "5", "1")
	c := s.damageMeters[0].Utility
	if c.MechanicTotals["finish/clean_atrox"][aug] != 0 || c.MechanicTotals["aura/1263971"][aug] != 1 {
		t.Fatal(c)
	}
}

func TestVoidscarMassUsesStackGainsAndDistinctOrbIdentities(t *testing.T) {
	s := ready()
	voidscarRun(s)
	ingest(s, 5, "ENCOUNTER_START", "3287", "Charonus", "8", "5", "2923")
	orb := "Creature-0-1-2923-1-255001-1"
	aura := func(sec int, event string, n string, source string) {
		f := actor(event, source, aug, 1263983)
		f[3], f[7] = "0x2112", "0x511"
		f = append(f, "DEBUFF")
		if n != "" {
			f = append(f, n)
		}
		ingest(s, sec, f...)
	}
	aura(6, "SPELL_AURA_APPLIED", "", orb)
	aura(7, "SPELL_AURA_APPLIED_DOSE", "3", orb)
	aura(7, "SPELL_AURA_APPLIED_DOSE", "3", orb)
	aura(8, "SPELL_AURA_REMOVED_DOSE", "2", orb)
	aura(9, "SPELL_AURA_REMOVED", "", orb)
	aura(10, "SPELL_AURA_APPLIED", "", orb)
	aura(11, "SPELL_AURA_APPLIED", "", "Creature-0-1-2923-1-255001-2")
	ingest(s, 30, "ENCOUNTER_END", "3287", "Charonus", "8", "5", "0")
	c := s.damageMeters[0].Utility
	if c.MechanicTotals["target/orb"][aug] != 2 || c.MechanicTotals["exposure/mass"][aug] != 4 {
		t.Fatal(c.MechanicTotals)
	}
}

func TestVoidscarOrbClearRequiresExactSingularityEffectAndKnownOrbOwner(t *testing.T) {
	s := ready()
	voidscarRun(s)
	ingest(s, 5, "ENCOUNTER_START", "3287", "Charonus", "8", "5", "2923")
	orb := "Creature-0-1-2923-1-255001-1"
	f := actor("SPELL_AURA_APPLIED", orb, aug, 1263983)
	f[3], f[7] = "0x2112", "0x511"
	ingest(s, 6, append(f, "DEBUFF")...)
	f = actor("SPELL_INSTAKILL", "0000000000000000", orb, 1263984)
	f[3], f[7] = "0xa48", "0x2112"
	ingest(s, 7, f...)
	ingest(s, 7, f...)
	f[5] = "Creature-0-1-2923-1-255001-Unknown"
	ingest(s, 8, f...)
	ingest(s, 9, actor("UNIT_DIED", "0000000000000000", orb, 0)...)
	ingest(s, 30, "ENCOUNTER_END", "3287", "Charonus", "8", "5", "1")
	c := s.damageMeters[0].Utility
	if c.MechanicTotals["orb/clear"][aug] != 1 || len(c.MechanicTotals["orb/clear"]) != 1 {
		t.Fatal(c.MechanicTotals)
	}
}

func TestVoidscarDarkWavesRequiresKnownNonTankAndKeepsPetOwner(t *testing.T) {
	s := ready()
	voidscarRun(s)
	ingest(s, 5, "ENCOUNTER_START", "3287", "Charonus", "8", "5", "2923")
	s.specs[aug] = 66
	s.specs[other] = 253
	for i, id := range []string{aug, other, "Player-1-Unknown"} {
		enemyMechanicDamage(s, 6+i, 1311923, "Dark Waves", id)
	}
	ingest(s, 30, "ENCOUNTER_END", "3287", "Charonus", "8", "5", "0")
	c := s.damageMeters[0].Utility
	if c.MechanicTotals["avoid/dark_waves"][aug] != 0 || c.MechanicTotals["avoid/dark_waves"][other] != 100 || c.MechanicTotals["avoid/dark_waves"]["Player-1-Unknown"] != 0 {
		t.Fatal(c)
	}
	s = ready()
	voidscarRun(s)
	s.pets["Pet-Helper"] = other
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-Helper", "Creature-0-1-2923-1-248666-1", 80, 1, "")
	ingest(s, 10, "CHALLENGE_MODE_END", "2923", "1")
	if s.damageMeters[0].Utility.MechanicTotals["work/248666"][other] != 80 {
		t.Fatal("pet work was not credited to owner")
	}
}

func TestVoidscarCoverageDoesNotChangeOtherDungeonsOrDowngradeVenomAwards(t *testing.T) {
	s := ready()
	ingest(s, 1, "CHALLENGE_MODE_START", "Other dungeon", "123", "999", "10")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-Boss", 100, 1, "")
	enemyMechanicDamage(s, 3, 1222484, "Poison Pool", aug)
	ingest(s, 10, "CHALLENGE_MODE_END", "123", "1")
	if s.damageMeters[0].Utility.MechanicsKnown || len(s.damageMeters[0].Utility.MechanicTotals) > 0 {
		t.Fatal("Voidscar rules escaped dungeon")
	}
	m := summaryFixture()
	m.ID = "boss/3429/1"
	m.Utility = &summaryCounts{Known: true, AwardVersion: 2, EncounterAwardVersion: 1, MechanicTotals: map[string]map[string]int64{"use/venom_orb": {aug: 2}}}
	good, _, _, _, _, _ := detailedBossAwards(m)
	if !strings.Contains(strings.Join(good, "~"), "Orb Hauler") {
		t.Fatal("existing venom award downgraded", good)
	}
	m.ID = "boss/3286/1"
	m.Utility.MechanicTotals = map[string]map[string]int64{"work/239070": {aug: 100}}
	good, _, _, _, _, _ = detailedBossAwards(m)
	if len(good) != 0 {
		t.Fatal("old coverage invented new dungeon measurements", good)
	}
}

func TestVoidscarRankingsOnlyStaySeparateAndReportsKeepFastTransportBounds(t *testing.T) {
	s := ready()
	voidscarRun(s)
	damageRecord(s, 3, "SPELL_DAMAGE", aug, "Creature-0-1-2923-1-248666-1", 100, 1, "")
	ingest(s, 10, "CHALLENGE_MODE_END", "2923", "1")
	r, err := s.PreparedSummaryForPlayer("key", aug, 4)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Text, "Totem Deletion") || strings.Contains(r.Text, "Award") || len(r.Text) > 4096 {
		t.Fatal(r)
	}
	m := summaryFixture()
	m.Kind = "key"
	m.ID = "key/2923/1"
	m.Seconds = 120
	m.Success = true
	c := &summaryCounts{Known: true}
	initAggregateAwardCounts(c)
	m.Utility = c
	for _, a := range encounterAwards[voidscarKeyScope] {
		for _, key := range a.keys {
			c.MechanicTotals[key] = map[string]int64{aug: 12345678}
		}
	}
	v := progressionAwards(m, nil, makeChatSummary(m, *c))
	v = summaryWithOptions(v, 1)
	if len(v.Text) > 4096 || len(v.Lines) > 21 || strings.Contains(v.Text, "one pull") {
		t.Fatal(v)
	}
}
