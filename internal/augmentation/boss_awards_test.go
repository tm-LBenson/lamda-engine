package augmentation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveredBossArchiveUpgradesLegacyWindowAfterRestart(t *testing.T) {
	s := ready()
	bossPull(s, 1, 11, "3445", "Sentinels", "15", 100, 0, false)
	bossPull(s, 21, 31, "3445", "Sentinels", "15", 200, 0, true)
	m := s.bossAttempts[1]
	c := *m.Utility
	c.MathFails = map[string]int64{aug: 1}
	m.Utility = &c
	s.bossAttempts[1] = m
	for i, m := range s.damageMeters {
		m.OutcomeKnown = false
		m.Success = false
		m.BossVisit = ""
		m.Difficulty = ""
		s.damageMeters[i] = m
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
	r, err := loaded.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "kill, 2 pulls") || !strings.Contains(r.Text, "Bad Math:") {
		t.Fatal(r, err)
	}
	if !loaded.damageMeters[0].OutcomeKnown || !loaded.damageMeters[0].Success || loaded.damageMeters[0].BossVisit == "" || loaded.damageMeters[0].Difficulty != "15" {
		t.Fatal(loaded.damageMeters[0])
	}
}

func TestBadMathIsSentinelsWrongMergeNotDamageOrDeaths(t *testing.T) {
	for _, id := range []string{"3445", "3497"} {
		s := ready()
		ingest(s, 1, "ENCOUNTER_START", id, "Boss", "15", "20")
		damageRecord(s, 2, "SPELL_DAMAGE", aug, councilFishTarget, 100, 1, "")
		for _, e := range []struct {
			at    int
			event string
			n     string
		}{
			{3, "SPELL_AURA_APPLIED", "1"}, {4, "SPELL_AURA_APPLIED_DOSE", "3"},
			{5, "SPELL_AURA_APPLIED_DOSE", "6"}, {6, "SPELL_AURA_APPLIED_DOSE", "7"},
			{7, "SPELL_AURA_REMOVED", ""},
		} {
			f := actor(e.event, "0000000000000000", aug, 1284590)
			f[7] = "0x511"
			f = append(f, "DEBUFF", e.n)
			ingest(s, e.at, f...)
		}
		ingest(s, 10, "ENCOUNTER_END", id, "Boss", "15", "20", "0")
		r, err := s.PreparedSummaryForPlayer("boss", aug)
		if err != nil || strings.Contains(r.Text, "Bad Math:") != (id == "3445") {
			t.Fatal(r, err)
		}
		want := int64(0)
		if id == "3445" {
			want = 1
		}
		if s.damageMeters[0].Utility.MathFails[aug] != want {
			t.Fatal(s.damageMeters[0].Utility)
		}
	}
}

func TestBossAwardSelectionIncludesExpandedGoodAndBanterPoolAndBoundsTransport(t *testing.T) {
	m := summaryFixture()
	m.ID = "boss/3445/123"
	m.Kind = "boss"
	m.Success = true
	m.Seconds = 20
	c := &summaryCounts{Known: true, MechanicsKnown: true, Dispels: map[string]int64{aug: 10}, Deaths: map[string]int64{other: 3}, Soaks: map[string]int64{aug: 5}, Avoidable: map[string]int64{other: 1234567}}
	initAggregateAwardCounts(c)
	c.Healing[aug] = 1000000
	c.Overhealing[aug] = 2000000
	c.Interrupts[other] = 5
	c.Resurrections[aug] = 1
	c.FirstDeaths[other] = 2
	c.AvoidableHits[other] = 8
	c.MathFails[other] = 1
	m.Utility = c
	lines := bossAwardLines(m, []DamageMeter{m})
	if len(lines) != 12 {
		t.Fatal(lines)
	}
	good := strings.Join(lines[:6], "~")
	bad := strings.Join(lines[6:], "~")
	for _, label := range []string{"Damage MVP", "Lifeguard", "Clean-up crew", "Team Player", "Quiet Please"} {
		if !strings.Contains(good, label) {
			t.Fatal(lines)
		}
	}
	for _, label := range []string{"Bad Math", "deaths", "Healer's Problem", "First to the Afterparty", "Repeat Customer"} {
		if !strings.Contains(bad, label) {
			t.Fatal(lines)
		}
	}
	r := progressionAwards(m, []DamageMeter{m}, makeChatSummary(m, *c))
	if len(r.Text) > 4096 {
		t.Fatal("transport overflow", len(r.Text))
	}
	for _, line := range r.Lines {
		if len(line) > 255 {
			t.Fatal(line)
		}
	}
	if strings.Count(r.Text, "Damage MVP") != 1 {
		t.Fatal("duplicate awards", r)
	}
}

func TestSeekerTakesPriorityAndNoDataDoesNotInventAwards(t *testing.T) {
	m := summaryFixture()
	m.ID = "boss/3497/123"
	m.Kind = "boss"
	m.Seconds = 20
	c := &summaryCounts{Known: true, FishKnown: true, Fish: map[string]int64{other: 6}, Dispels: map[string]int64{aug: 10}, Soaks: map[string]int64{aug: 5}, MechanicsKnown: true}
	initAggregateAwardCounts(c)
	c.Healing[aug] = 1000000
	c.Interrupts[other] = 5
	m.Utility = c
	lines := bossAwardLines(m, []DamageMeter{m})
	if !strings.Contains(lines[0], "Seeker:") || !strings.Contains(strings.Join(lines, "~"), "Quiet Please") {
		t.Fatal(lines)
	}
	m.Utility = nil
	for _, line := range bossAwardLines(m, []DamageMeter{m}) {
		if strings.Contains(line, "Deathless") || strings.Contains(line, "Bad Math") || strings.Contains(line, "Seeker:") {
			t.Fatal(line)
		}
	}
}

func TestLiveAwardStatsCountEffectiveHealingPetsInterruptsAndFirstDeaths(t *testing.T) {
	s := ready()
	s.pets["Pet-Healer"] = other
	ingest(s, 1, "ENCOUNTER_START", "3445", "Boss", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, councilFishTarget, 100, 1, "")
	f := actor("SPELL_HEAL", "Pet-Healer", aug, 123)
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, "1000", "1000", "400", "0", "nil")
	ingest(s, 3, f...)
	f = actor("SPELL_INTERRUPT", other, councilFishTarget, 123)
	f[7] = "0xa48"
	ingest(s, 4, f...)
	ingest(s, 5, actor("SPELL_RESURRECT", other, aug, 123)...)
	ingest(s, 6, actor("UNIT_DIED", councilFishTarget, aug, 1)...)
	ingest(s, 7, actor("UNIT_DIED", councilFishTarget, other, 1)...)
	ingest(s, 10, "ENCOUNTER_END", "3445", "Boss", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if c.AwardVersion != awardStatsVersion || !c.HealingKnown || c.Healing[other] != 600 || c.Overhealing[other] != 400 || c.Interrupts[other] != 1 || c.Resurrections[other] != 1 || c.FirstDeaths[aug] != 1 || c.FirstDeaths[other] != 0 {
		t.Fatal(c)
	}
}

func TestPartialAwardCoverageDoesNotProduceHealerOrMechanicBlame(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3445", "Boss", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, councilFishTarget, 100, 1, "")
	ingest(s, 3, actor("SPELL_HEAL", aug, other, 123)...)
	ingest(s, 10, "ENCOUNTER_END", "3445", "Boss", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if c.HealingKnown {
		t.Fatal("malformed heal silently accepted")
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || strings.Contains(r.Text, "Lifeguard") || strings.Contains(r.Text, "Just In Case") || strings.Contains(r.Text, "Bad Math") {
		t.Fatal(r, err)
	}
}
