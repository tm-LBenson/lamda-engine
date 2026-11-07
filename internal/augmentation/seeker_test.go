package augmentation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const councilFishTarget = "Creature-0-3022-3004-3418-261835-0000614020"

func fishEvent(kind, source, target string, spell int) []string {
	f := actor(kind, source, target, spell)
	f[2], f[7], f[10] = "Fisher-Realm", "0x10a48", "Disgusting Fish"
	return f
}

func TestSeekerCountsCompletedCouncilFishUsesOnly(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3497", "The Lost Explorers", "15", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, councilFishTarget, 100, 1, "")
	for _, event := range []struct {
		second               int
		kind, source, target string
		spell                int
	}{
		{3, "SPELL_CAST_SUCCESS", other, councilFishTarget, 1296535},
		{3, "SPELL_CAST_SUCCESS", other, councilFishTarget, 1296535}, // duplicate
		{3, "SPELL_AURA_APPLIED", other, councilFishTarget, 1296535},
		{4, "SPELL_CAST_FAILED", other, councilFishTarget, 1296535},
		{5, "SPELL_CAST_SUCCESS", "Creature-Fish", "Creature-Fish", 1292490},
		{6, "SPELL_SUMMON", "Creature-Junk", "Creature-Fish", 1306122},
		{7, "SPELL_CAST_SUCCESS", other, "Creature-0-3022-3004-3418-123-0", 1296535},
		{8, "SPELL_CAST_SUCCESS", other, councilFishTarget, 1296535},
	} {
		ingest(s, event.second, fishEvent(event.kind, event.source, event.target, event.spell)...)
	}
	ingest(s, 11, "ENCOUNTER_END", "3497", "The Lost Explorers", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if !c.FishKnown || c.Fish[other] != 2 || len(c.Fish) != 1 {
		t.Fatal(c)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Seeker: Fisher - 2 fish handled") {
		t.Fatal(r, err)
	}
}

func TestSeekerCombinesPullsAndSharesTies(t *testing.T) {
	s := ready()
	for _, start := range []int{1, 21} {
		ingest(s, start, "ENCOUNTER_START", "3497", "The Lost Explorers", "15", "20")
		damageRecord(s, start+1, "SPELL_DAMAGE", aug, councilFishTarget, 100, 1, "")
		ingest(s, start+2, fishEvent("SPELL_CAST_SUCCESS", aug, councilFishTarget, 1296535)...)
		ingest(s, start+3, fishEvent("SPELL_CAST_SUCCESS", other, councilFishTarget, 1296535)...)
		ingest(s, start+10, "ENCOUNTER_END", "3497", "The Lost Explorers", "15", "20", "0")
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Seeker: Fisher/+1 tied - 2 fish handled") || !strings.Contains(r.Text, "2 pulls") {
		t.Fatal(r, err)
	}
	data, _ := json.Marshal(s.Snapshot())
	path := filepath.Join(t.TempDir(), "history.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded := New(path)
	defer loaded.Close()
	again, err := loaded.PreparedSummaryForPlayer("boss", aug)
	if err != nil || again.Text != r.Text {
		t.Fatal(again, err)
	}
}

func TestSeekerRequiresCompleteKnownCoverageAndCorrectBoss(t *testing.T) {
	for _, id := range []string{"3497", "3470"} {
		s := ready()
		bossPull(s, 1, 11, id, "Boss", "15", 100, 0, true)
		m := s.damageMeters[0]
		c := *m.Utility
		c.Fish = map[string]int64{aug: 10}
		c.FishKnown = id != "3497" // unknown Council or wrong boss
		m.Utility = &c
		s.bossAttempts = nil
		s.prepareSummaryLocked(m)
		r, err := s.PreparedSummaryForPlayer("boss", aug)
		if err != nil || strings.Contains(r.Text, "Seeker:") {
			t.Fatal(r, err)
		}
	}
}

func TestSeekerBackfillsOlderCouncilAndSurvivesStaleHistory(t *testing.T) {
	lines := []string{record(0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")}
	for _, start := range []int{1, 21} {
		f := fishEvent("SPELL_CAST_SUCCESS", aug, councilFishTarget, 1296535)
		f[3] = "0x511"
		lines = append(lines, record(start, "ENCOUNTER_START", "3497", "The Lost Explorers", "15", "20"), record(start+1, f...), record(start+10, "ENCOUNTER_END", "3497", "The Lost Explorers", "15", "20", "0"))
	}
	lines = append(lines, record(41, "ENCOUNTER_START", "3470", "Later boss", "15", "20"), record(42, "SPELL_CAST_SUCCESS", aug, "Fisher", "0x511", "0", councilFishTarget, "Boss", "0xa48", "0", "1", "Test", "1"), record(51, "ENCOUNTER_END", "3470", "Later boss", "15", "20", "1"))
	path := filepath.Join(t.TempDir(), "original.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := ready()
	for _, line := range lines {
		s.Ingest(line, s.last)
	}
	for i, m := range s.damageMeters {
		if encounterID(m) == "3497" {
			c := *m.Utility
			c.FishKnown, c.Fish = false, nil
			c.AwardVersion, c.HealingKnown = 0, false
			m.Utility = &c
			s.damageMeters[i] = m
		}
	}
	s.bossAttempts = nil
	for _, m := range s.damageMeters {
		s.prepareSummaryLocked(m)
	}
	stale := s.damageMeters[1]
	s.prepareSummaryHistory(context.Background(), path, aug)
	for _, m := range s.damageMeters {
		if encounterID(m) != "3497" {
			continue
		}
		if m.Utility == nil || !m.Utility.FishKnown || m.Utility.Fish[aug] != 1 {
			t.Fatal(m.Utility)
		}
	}
	s.prepareSummaryLocked(stale)
	r, err := s.PreparedSummaryForPlayer("recorded:"+stale.ID, aug)
	if err != nil || !strings.Contains(r.Text, "Seeker: Fisher - 2 fish handled") {
		t.Fatal(r, err)
	}
	// No original log is needed to read or persist the prepared result.
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	again, err := s.PreparedSummaryForPlayer("recorded:"+stale.ID, aug)
	if err != nil || again.Text != r.Text {
		t.Fatal(again, err)
	}
}
