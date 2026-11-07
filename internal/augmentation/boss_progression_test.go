package augmentation

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bossPull(s *Store, startAt, endAt int, id, name, difficulty string, damage int, deaths int, kill bool) {
	ingest(s, startAt, "ENCOUNTER_START", id, name, difficulty, "20")
	damageRecord(s, startAt+1, "SPELL_DAMAGE", aug, "Creature-1", damage, 1, "")
	for i := 0; i < deaths; i++ {
		ingest(s, startAt+2, actor("UNIT_DIED", "Creature-1", aug, 1)...)
	}
	success := "0"
	if kill {
		success = "1"
	}
	ingest(s, endAt, "ENCOUNTER_END", id, name, difficulty, "20", success)
}

func TestBossProgressionCombinesWipesKillAndUsesWeightedClock(t *testing.T) {
	s := ready()
	bossPull(s, 1, 11, "123", "Raid boss", "16", 1000, 1, false)
	bossPull(s, 21, 41, "123", "Raid boss", "16", 1000, 0, true)
	v, pulls := s.bossProgressionLocked(s.damageMeters[0])
	if len(pulls) != 2 || v.RawTotal != 2000 || v.AdjustedTotal != 2000 || v.Seconds != 30 || math.Abs(v.Rows[0].DPS-2000.0/30) > .001 || v.Utility.Deaths[aug] != 1 {
		t.Fatal(v, pulls)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "kill, 2 pulls") || !strings.Contains(r.Text, "66 DPS") || !strings.Contains(r.Text, "First Try, Obviously") {
		t.Fatal(r, err)
	}
}

func TestBossOneShotHasSpecificBanterWithoutProgressionClaims(t *testing.T) {
	s := ready()
	bossPull(s, 1, 11, "3470", "Nek'zali the Soulcoiler", "16", 1000, 0, true)
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "kill, 1 pull") || !strings.Contains(r.Text, "Souls Denied") || strings.Contains(r.Text, "rehearsals") {
		t.Fatal(r, err)
	}
}

func TestDeathsNeverTriggerBadMath(t *testing.T) {
	s := ready()
	bossPull(s, 1, 11, "123", "Raid boss", "16", 1000, 2, false)
	bossPull(s, 21, 31, "123", "Raid boss", "16", 1000, 1, true)
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || strings.Contains(r.Text, "Bad Math:") || !strings.Contains(r.Text, "3 deaths") {
		t.Fatal(r, err)
	}
}

func TestBossProgressionSeparatesBossDifficultyVisitAndPreviousKill(t *testing.T) {
	s := ready()
	bossPull(s, 1, 11, "123", "Raid boss", "16", 1000, 0, true)
	bossPull(s, 21, 31, "123", "Raid boss", "16", 2000, 0, false)
	v, p := s.bossProgressionLocked(s.damageMeters[0])
	if len(p) != 1 || v.AdjustedTotal != 2000 {
		t.Fatal(v, p)
	}
	bossPull(s, 41, 51, "124", "Other boss", "16", 3000, 0, false)
	bossPull(s, 61, 71, "123", "Raid boss", "15", 4000, 0, false)
	v, p = s.bossProgressionLocked(s.damageMeters[0])
	if len(p) != 1 || v.AdjustedTotal != 4000 {
		t.Fatal(v, p)
	}
	s.bossVisit = "new-visit"
	bossPull(s, 81, 91, "123", "Raid boss", "15", 5000, 0, true)
	v, p = s.bossProgressionLocked(s.damageMeters[0])
	if len(p) != 1 || v.AdjustedTotal != 5000 {
		t.Fatal(v, p)
	}
}

func TestProgressionArchiveSurvivesWindowHistoryAndRestart(t *testing.T) {
	s := ready()
	for i := 0; i < 45; i++ {
		bossPull(s, i*20+1, i*20+11, "123", "Raid boss", "16", 1000, 0, i == 44)
	}
	if len(s.damageMeters) != 40 || len(s.bossAttempts) != 45 {
		t.Fatal(len(s.damageMeters), len(s.bossAttempts))
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
	if err != nil || !strings.Contains(r.Text, "kill, 45 pulls") {
		t.Fatal(r, err)
	}
}

func TestProgressionLateOwnershipChangesAreReflectedInAggregate(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "123", "Raid boss", "16", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 50, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-Late", "Creature-1", 100, 1, "")
	ingest(s, 11, "ENCOUNTER_END", "123", "Raid boss", "16", "20", "0")
	s.petOwner("Pet-Late", other, "summon")
	bossPull(s, 21, 31, "123", "Raid boss", "16", 1000, 0, true)
	v, p := s.bossProgressionLocked(s.damageMeters[0])
	if len(p) != 2 || !v.Valid || v.RawTotal != 1150 {
		t.Fatal(v, p)
	}
}

func TestProgressionUnknownUtilityDoesNotProduceFalseDeathless(t *testing.T) {
	s := ready()
	bossPull(s, 1, 11, "123", "Raid boss", "16", 1000, 0, false)
	m := s.damageMeters[0]
	m.Utility = nil
	s.rememberBossAttemptLocked(m)
	bossPull(s, 21, 31, "123", "Raid boss", "16", 1000, 0, true)
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || strings.Contains(r.Text, "Deathless") || r.MetricsKnown {
		t.Fatal(r, err)
	}
	for _, line := range r.Lines {
		if len(line) > 255 {
			t.Fatal(fmt.Sprint(line))
		}
	}
}

func TestOwnershipAfterLaterPullRefreshesWholeBossRecap(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "123", "Raid boss", "16", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 50, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-Late", "Creature-1", 100, 1, "")
	ingest(s, 11, "ENCOUNTER_END", "123", "Raid boss", "16", "20", "0")
	bossPull(s, 21, 31, "123", "Raid boss", "16", 1000, 0, true)
	s.petOwner("Pet-Late", other, "summon")
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "DPS") || strings.Contains(r.Text, "Damage award pending") {
		t.Fatal(r, err)
	}
}
