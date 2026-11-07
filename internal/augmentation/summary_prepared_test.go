package augmentation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRaidSummaryRanksTopFiveAndAwardsUseEntireRaid(t *testing.T) {
	m := summaryFixture()
	m.Seconds = 10
	m.Rows = nil
	c := summaryCounts{Known: true, Dispels: map[string]int64{}, Deaths: map[string]int64{}}
	for i := 1; i <= 20; i++ {
		guid := fmt.Sprintf("Player-%d", i)
		m.Rows = append(m.Rows, DamageRow{GUID: guid, Name: fmt.Sprintf("Raider%02d-Realm", i), Adjusted: int64(i * 1000)})
	}
	c.Dispels["Player-1"], c.Deaths["Player-2"] = 12, 3
	v := makeChatSummary(m, c)
	if len(v.Lines) != 9 || !strings.Contains(v.Lines[0], "top 5") || !strings.Contains(v.Lines[1], "1. Raider20") || !strings.Contains(v.Lines[5], "5. Raider16") || strings.Contains(v.Text, "6. Raider") || !strings.Contains(v.Text, "Raider01 (12 dispels)") || !strings.Contains(v.Text, "Raider02 (3 deaths)") {
		t.Fatal(v)
	}
}

func TestReportAlreadyPreparedAtSegmentEndAndReadDoesNotRecoverLog(t *testing.T) {
	s := ready()
	ingest(s, 1, "CHALLENGE_MODE_START", "Ready key", "123", "1", "10")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	ingest(s, 3, actor("SPELL_DISPEL", other, "Creature-1", 1)...)
	ingest(s, 10, "CHALLENGE_MODE_END", "123", "1", "10000")
	if len(s.preparedSummaries) != 1 {
		t.Fatal("report was not prepared before Share")
	}
	// A different, incomplete boundary would make on-demand recovery reject
	// this report. The share read must use the completed collection instead.
	path := filepath.Join(t.TempDir(), "unrelated-log.txt")
	if err := os.WriteFile(path, []byte(record(20, "CHALLENGE_MODE_END", "456", "1", "10000")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.debug = newDebug(filepath.Join(t.TempDir(), "ledger.jsonl"))
	s.debug.SourceFile = path
	v, err := s.PreparedSummaryForPlayer("key", aug)
	if err != nil || !v.MetricsKnown || !strings.Contains(v.Text, "Ready key") || !strings.Contains(v.Text, "1 dispel)") {
		t.Fatal(v, err)
	}
	v.Lines[0] = "changed"
	got, err := s.PreparedSummaryForPlayer("key", aug)
	if err != nil || got.Lines[0] == "changed" {
		t.Fatal("caller mutated prepared report", got, err)
	}
}

func TestPreparedBossWaitsForNewBoundaryAndIncludesUptime(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	auraEvent(s, 2, "SPELL_AURA_APPLIED", aug, aug, 395296)
	end(s, 10)
	v, err := s.PreparedSummaryForPlayer("boss-uptime", aug)
	if err != nil || !strings.Contains(v.Text, "88.9%") {
		t.Fatal(v, err)
	}
	standard, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || strings.Contains(standard.Text, "uptime") {
		t.Fatal(standard, err)
	}
	start(s, 20)
	if _, err = s.PreparedSummaryForPlayer("boss", aug); !errors.Is(err, errSummaryPending) {
		t.Fatal("used the previous boss while final records were missing", err)
	}
}

func TestCombatAwardsUseLastCompletedFightWhileRankingsWait(t *testing.T) {
	for _, scope := range []string{"boss", "boss-uptime", "key"} {
		t.Run(scope, func(t *testing.T) {
			s := ready()
			if scope == "key" {
				ingest(s, 1, "CHALLENGE_MODE_START", "Finished key", "123", "1", "10")
			} else {
				start(s, 1)
			}
			damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
			ingest(s, 3, actor("SPELL_DISPEL", other, "Creature-1", 1)...)
			if scope == "key" {
				ingest(s, 10, "CHALLENGE_MODE_END", "123", "1", "10000")
			} else {
				end(s, 10)
			}
			before, err := s.PreparedSummaryForPlayer(scope, aug, 1)
			if err != nil || !strings.Contains(before.Text, "1 dispel") {
				t.Fatal(before, err)
			}
			if scope == "key" {
				ingest(s, 20, "CHALLENGE_MODE_START", "Next key", "456", "1", "10")
			} else {
				start(s, 20)
			}
			for _, options := range []uint8{1, 3} {
				awards, err := s.PreparedSummaryForPlayer(scope, aug, options)
				if err != nil || awards.EndedAt != before.EndedAt || strings.Contains(awards.Text, "1. ") {
					t.Fatal("completed awards unavailable during next fight", awards, err)
				}
			}
			if _, err := s.PreparedSummaryForPlayer(scope, aug, 4); !errors.Is(err, errSummaryPending) {
				t.Fatal("rankings reused a previous fight", err)
			}
		})
	}
}

func TestLateOwnershipRebuildsPreparedReport(t *testing.T) {
	s := ready()
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 50, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-Late", "Creature-1", 100, 2, "")
	end(s, 10)
	before, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(before.Text, "Damage award pending") {
		t.Fatal(before, err)
	}
	s.petOwner("Pet-Late", other, "summon")
	after, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(after.Text, "Damage MVP") || strings.Contains(after.Text, "Damage award pending") || after.Text == before.Text {
		t.Fatal(after, err)
	}
}

func TestOlderUtilityIsPreparedInBackgroundAndPersists(t *testing.T) {
	m := summaryFixture()
	m.Seconds = 8
	m.StartedAt = time.Date(2026, 10, 26, 12, 0, 2, 0, time.FixedZone("test", -4*3600))
	m.EndedAt = m.StartedAt.Add(8 * time.Second)
	m.ID = fmt.Sprintf("key/123/%d", m.StartedAt.UnixMilli())
	path := filepath.Join(t.TempDir(), "original.txt")
	lines := []string{record(2, "CHALLENGE_MODE_START", "Fixture", "123", "1", "10"), record(3, actor("SPELL_DISPEL", other, "Creature-1", 1)...), record(10, "CHALLENGE_MODE_END", "123", "1", "8000")}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := ready()
	s.damageMeters = []DamageMeter{m}
	s.prepareSummaryLocked(m)
	s.prepareSummaryHistory(context.Background(), path, aug)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	v, err := s.PreparedSummaryForPlayer("key", aug)
	if err != nil || !v.MetricsKnown || !strings.Contains(v.Text, "1 dispel)") || s.Snapshot().DamageMeters[0].Utility == nil {
		t.Fatal(v, err)
	}
}

func originalSummaryDamage(second int) string {
	f := actor("SPELL_DAMAGE", aug, "Creature-1", 1)
	f[3], f[7] = "0x511", "0xa48"
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, "50", "50", "-1", "12", "0", "0", "0", "nil", "nil", "nil", "ST")
	return record(second, f...)
}

func TestMissingBossBoundaryDoesNotReusePreviousReportAndRepairsInBackground(t *testing.T) {
	s := ready()
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	end(s, 10)
	end(s, 20) // The live attachment missed the second start.
	if _, err := s.PreparedSummaryForPlayer("boss", aug); !errors.Is(err, errSummaryPending) {
		t.Fatal("previous fight reused", err)
	}
	path := filepath.Join(t.TempDir(), "original.txt")
	lines := []string{record(0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5"), record(12, "ENCOUNTER_START", "123", "Recovered boss", "8", "5"), originalSummaryDamage(13), record(20, "ENCOUNTER_END", "123", "Recovered boss", "8", "5", "1")}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.prepareSummaryHistory(context.Background(), path, aug)
	v, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(v.Text, "Recovered boss") || s.damageMeters[0].RawTotal != 50 {
		t.Fatal(v, err)
	}
}

func TestBackgroundRecoveryFindsKeyBeforeLargeTailOfLaterEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original.txt")
	lines := []string{record(0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5"), record(1, "CHALLENGE_MODE_START", "Older key", "123", "1", "10"), originalSummaryDamage(2), record(10, "CHALLENGE_MODE_END", "123", "1", "10000")}
	data := strings.Join(lines, "\n") + "\n" + strings.Repeat("later event\n", 500000)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	s := ready()
	s.prepareSummaryHistory(context.Background(), path, aug)
	v, err := s.PreparedSummaryForPlayer("key", aug)
	if err != nil || !strings.Contains(v.Text, "Older key") || !v.MetricsKnown {
		t.Fatal(v, err)
	}
}
