package augmentation

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func templeDamage(s *Store, sec int, source, target string, spell int) {
	f := actor("SPELL_DAMAGE", source, target, spell)
	f[3], f[7] = "0xa48", "0x514"
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	ingest(s, sec, append(f, "100", "100", "-1", "8", "0", "0", "0", "nil", "nil", "nil", "AOE")...)
}

func TestTempleSplitsThunderSoaksFromSecondBlastAndExcludesMandatoryDamage(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "2124", "Adderis and Aspix", "8", "5", "1877")
	boss := "Creature-0-1-1877-1-262530-1"
	templeDamage(s, 2, boss, aug, 1288092)
	templeDamage(s, 2, boss, aug, 1288092) // duplicate record
	templeDamage(s, 3, boss, other, 1314051)
	templeDamage(s, 4, "Vehicle-0-1-1877-1-262822-1", aug, 1311808) // initial pool drop
	templeDamage(s, 5, boss, aug, 1288457)                          // required Gust
	d := s.active["boss"].damage
	if d.mechanicTotals["soak/thunder"][aug] != 1 || d.avoidableHits[aug] != 0 || d.avoidableHits[other] != 1 {
		t.Fatal("soak/escape impacts conflated", d.mechanicTotals, d.avoidableHits)
	}
}

func TestTempleBeamWorkCleanFinishAndContactWithoutDamage(t *testing.T) {
	for _, contact := range []bool{false, true} {
		s := ready()
		ingest(s, 0, "CHALLENGE_MODE_START", "Temple of Sethraliss", "1877", "250", "10")
		ingest(s, 1, "ENCOUNTER_START", "2126", "Galvazzt", "8", "5", "1877")
		for sec := 2; sec < 24; sec++ {
			templeDamage(s, sec, "Creature-0-1-1877-1-135445-1", aug, 266923)
		}
		if contact {
			mechanicAura(s, 24, "SPELL_AURA_APPLIED", aug, 1291815)
		}
		damageRecord(s, 29, "SPELL_DAMAGE", aug, "Creature-0-1-1877-1-263658-1", 10, 1, "")
		ingest(s, 30, "ENCOUNTER_END", "2126", "Galvazzt", "8", "5", "1")
		m := s.damageMeters[0]
		if m.Utility.MechanicTotals["beam/ticks"][aug] != 22 || m.Utility.Avoidable[aug] != 0 {
			t.Fatal("useful beam duty counted as failure", m.Utility)
		}
		want := int64(1)
		if contact {
			want = 0
		}
		if m.Utility.MechanicTotals["finish/clean_beams"][aug] != want || s.active["key"].damage.mechanicTotals["finish/clean_beams"][aug] != want {
			t.Fatal("clean finish ignored ground contact or did not reach key", m.Utility)
		}
	}
}

func TestTempleKnotControlCountsOnlyFirstHardControlPerCreature(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "2125", "Merektha", "8", "5", "1877")
	for i, spell := range []int{204242, 3355, 105421, 3355} {
		f := actor("SPELL_AURA_APPLIED", aug, "Creature-0-1-1877-1-134388-1", spell)
		f[3], f[7] = "0x511", "0xa28"
		ingest(s, i+2, append(f, "DEBUFF")...)
	}
	if s.active["boss"].damage.mechanicTotals["cc/knot"][aug] != 1 {
		t.Fatal(s.active["boss"].damage.mechanicTotals)
	}
}

func TestTempleAvatarHealingOrbIdentityAndReportScope(t *testing.T) {
	s := ready()
	ingest(s, 0, "CHALLENGE_MODE_START", "Temple of Sethraliss", "1877", "250", "10")
	ingest(s, 1, "ENCOUNTER_START", "2127", "Avatar of Sethraliss", "8", "5", "1877")
	avatar := "Creature-0-1-1877-1-133392-1"
	for _, source := range []string{aug, avatar} {
		f := actor("SPELL_HEAL", source, avatar, 2061)
		f[3], f[7] = "0x511", "0xa18"
		if source == avatar {
			f[3] = "0xa18"
		}
		for i := 0; i < 19; i++ {
			f = append(f, "0")
		}
		ingest(s, 2, append(f, "1000", "1000", "250", "0", "nil")...)
	}
	for _, event := range []string{"SPELL_AURA_APPLIED", "SPELL_AURA_APPLIED_DOSE", "SPELL_AURA_APPLIED"} {
		f := actor(event, "Creature-0-1-1877-1-268364-1", aug, 1300877)
		f[3], f[7] = "0xa48", "0x511"
		ingest(s, 3, append(f, "DEBUFF", "2")...)
	}
	f := actor("SPELL_INTERRUPT", aug, "Creature-0-1-1877-1-268491-1", 147362)
	f[7] = "0xa48"
	ingest(s, 4, append(f, "1302158", "Flame Shock", "4")...)
	damageRecord(s, 29, "SPELL_DAMAGE", aug, "Creature-0-1-1877-1-268427-1", 100, 1, "")
	ingest(s, 30, "ENCOUNTER_END", "2127", "Avatar of Sethraliss", "8", "5", "1")
	ingest(s, 31, "CHALLENGE_MODE_END", "1877", "1")
	m := s.damageMeters[0]
	if m.Utility.MechanicTotals["heal/avatar"][aug] != 750 || m.Utility.MechanicTotals["soak/lifeforce"][aug] != 1 {
		t.Fatal("objective healing or orb identity incorrect", m.Utility)
	}
	text := strings.Join(bossAwardLines(m, nil), "\n")
	for _, label := range []string{"Loa Life Support", "Lifeforce Delivery", "Defiler Eviction", "Flame Moderator"} {
		if !strings.Contains(text, label) {
			t.Fatal(label, text)
		}
	}
	r, err := s.PreparedSummaryForPlayer("key", aug, 1)
	if err != nil || !strings.Contains(r.Text, "Loa Life Support") || strings.Contains(r.Text, "Raid Carpet") || strings.Contains(r.Text, "one pull, one kill") {
		t.Fatal(err, r.Text)
	}
	for _, line := range r.Lines {
		if len(line) > 255 {
			t.Fatal("oversized chat line", line)
		}
	}
}

// Replay only the original, bounded Temple extraction; never reads live data
// or sends chat. Its output is compared with an independent raw-log audit.
func TestTempleOriginalLogAudit(t *testing.T) {
	path := os.Getenv("LAMDA_TEMPLE_AUDIT_LOG")
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
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	results := []DamageMeter{}
	for _, m := range s.damageMeters {
		if !templeBoss(encounterID(m)) && awardScope(m) != templeKeyScope {
			continue
		}
		results = append(results, m)
		for _, line := range bossAwardLines(m, []DamageMeter{m}) {
			if len(line) > 255 {
				t.Fatal("oversized line", line)
			}
		}
		t.Log(m.Kind, m.Name, len(m.Rows), m.Utility.EncounterAwardVersion)
	}
	if len(results) != 5 {
		t.Fatal("missing original encounters", len(results))
	}
	if output := os.Getenv("LAMDA_TEMPLE_AUDIT_OUT"); output != "" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTempleMalformedObjectiveHealDoesNotProduceAnAward(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "2127", "Avatar", "8", "5", "1877")
	f := actor("SPELL_HEAL", aug, "Creature-0-1-1877-1-133392-1", 2061)
	f[3], f[7] = "0x511", "0xa18"
	ingest(s, 2, f...)
	if !s.active["boss"].damage.unknownMechanics["heal/avatar"] {
		t.Fatal("incomplete healing payload accepted")
	}
	// Ensure temple names do not leak into an unrelated dungeon key.
	for _, mapID := range []int{1877, 2923, 123} {
		m := DamageMeter{Kind: "key", ID: "key/" + strconv.Itoa(mapID) + "/1"}
		if hasDungeonAwards(m) != (mapID != 123) {
			t.Fatal(mapID, awardScope(m))
		}
	}
}
