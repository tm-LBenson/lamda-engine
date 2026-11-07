package augmentation

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func summaryFixture() DamageMeter {
	return DamageMeter{HasAug: true, ID: "key/123/1", Name: "Fixture +10", Kind: "key", Owner: aug, Valid: true, AttributionMode: "all", DamageBasis: "health+absorbs", Rows: []DamageRow{{GUID: aug, Name: "Aug-Realm", Raw: 10, Adjusted: 1000}, {GUID: other, Name: "Ally-Realm", Raw: 1000, Adjusted: 10}}}
}
func TestChatSummaryUsesAdjustedDamageAndKnownAwards(t *testing.T) {
	m := summaryFixture()
	s := makeChatSummary(m, summaryCounts{Known: true, Dispels: map[string]int64{other: 4}, Deaths: map[string]int64{aug: 2}})
	for _, part := range []string{"Damage MVP: Aug 1.0k", "Clean-up crew: Ally (4 dispels)", "Aug (2 deaths)"} {
		if !strings.Contains(s.Text, part) {
			t.Fatalf("missing %s: %s", part, s.Text)
		}
	}
	unknown := makeChatSummary(m, summaryCounts{})
	if strings.Contains(unknown.Text, "death") || strings.Contains(unknown.Text, "dispels") {
		t.Fatal(unknown)
	}
	zero := makeChatSummary(m, summaryCounts{Known: true})
	if !strings.Contains(zero.Text, "Deathless") {
		t.Fatal(zero)
	}
}
func TestChatSummaryTiesUTF8AndChatLimit(t *testing.T) {
	m := summaryFixture()
	m.Name = strings.Repeat("界|\n", 60)
	m.Rows[0].Name = strings.Repeat("界", 30)
	m.Rows[1].Name = strings.Repeat("Я", 40)
	m.Rows[1].Adjusted = m.Rows[0].Adjusted
	c := summaryCounts{Known: true, Dispels: map[string]int64{}, Deaths: map[string]int64{}}
	for _, r := range m.Rows {
		c.Dispels[r.GUID] = 99999
		c.Deaths[r.GUID] = 9999
	}
	s := makeChatSummary(m, c)
	if len(s.Text) > 4096 || !utf8.ValidString(s.Text) || strings.ContainsAny(s.Text, "|\n") || !strings.Contains(s.Text, "+1 tied") || !strings.Contains(s.Text, "Damage MVP") || !strings.Contains(s.Text, "dispels") || !strings.Contains(s.Text, "deaths") {
		t.Fatal(s.Text)
	}
}
func TestSummaryRejectsLiveWrongRosterAndIncompleteLatest(t *testing.T) {
	m := summaryFixture()
	s := ready()
	s.own = aug
	s.damageMeters = []DamageMeter{m}
	if _, e := s.Summary(context.Background(), "key", rosterHash(m.Rows)); e != nil {
		t.Fatal(e)
	}
	s.active["key"] = &segment{}
	if _, e := s.Summary(context.Background(), "key", rosterHash(m.Rows)); e == nil {
		t.Fatal("old awards during a new key")
	}
	delete(s.active, "key")
	if _, e := s.Summary(context.Background(), "key", 42); e == nil {
		t.Fatal("wrong party")
	}
	m.Valid = false
	s.damageMeters = []DamageMeter{m, summaryFixture()}
	if _, e := s.Summary(context.Background(), "key", rosterHash(m.Rows)); e == nil {
		t.Fatal("fell back to old key")
	}
	m.Valid = true
	m.Provisional = true
	s.damageMeters = []DamageMeter{m}
	if _, e := s.Summary(context.Background(), "key", rosterHash(m.Rows)); e == nil {
		t.Fatal("provisional damage ranked")
	}
	m.Provisional = false
	m.Live = true
	s.damageMeters = []DamageMeter{m}
	if _, e := s.Summary(context.Background(), "key", rosterHash(m.Rows)); e == nil {
		t.Fatal("live damage ranked")
	}
}
func TestSummaryCountsBoundariesPetsAndCoverage(t *testing.T) {
	m := summaryFixture()
	m.StartedAt, _, _ = engine.RecordFields(record(2, "CHALLENGE_MODE_START", "Fixture", "123", "1", "10"))
	m.EndedAt = m.StartedAt.Add(8 * time.Second)
	lines := []string{record(1, actor("SPELL_SUMMON", other, "Pet-1", 1)...), record(2, "CHALLENGE_MODE_START", "Fixture", "123", "1", "10"), record(3, actor("SPELL_DISPEL", other, "Creature-1", 1)...), record(4, actor("SPELL_STOLEN", "Pet-1", "Creature-1", 1)...), record(5, actor("UNIT_DIED", "Creature-1", aug, 1)...), record(6, actor("UNIT_DIED", other, "Pet-1", 1)...), record(10, "CHALLENGE_MODE_END", "123", "1", "8000"), record(11, actor("UNIT_DIED", "Creature-1", other, 1)...)}
	path := filepath.Join(t.TempDir(), "original.log")
	write := func(ls []string) {
		t.Helper()
		if e := os.WriteFile(path, []byte(strings.Join(ls, "\n")+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write(lines)
	c := readSummaryCounts(context.Background(), path, m)
	if !c.Known || c.Dispels[other] != 2 || c.Deaths[aug] != 1 || c.Deaths[other] != 0 {
		t.Fatal(c)
	}
	write(lines[1:])
	if readSummaryCounts(context.Background(), path, m).Known {
		t.Fatal("unowned pet counted")
	}
	write(lines[:6])
	if readSummaryCounts(context.Background(), path, m).Known {
		t.Fatal("missing end counted as zero")
	}
	lines[1] = record(2, "CHALLENGE_MODE_START", "Other", "456", "1", "10")
	write(lines)
	if readSummaryCounts(context.Background(), path, m).Known {
		t.Fatal("wrong key boundary")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if readSummaryCounts(ctx, path, m).Known {
		t.Fatal("cancel ignored")
	}
}

func TestIncompleteDamageCanStillShareVerifiedUtility(t *testing.T) {
	m := summaryFixture()
	m.Valid = false
	m.Partial = true
	s := makeChatSummary(m, summaryCounts{Known: true, Dispels: map[string]int64{other: 3}, Deaths: map[string]int64{aug: 2}})
	if strings.Contains(s.Text, "Damage MVP") || !strings.Contains(s.Text, "incomplete log") || !strings.Contains(s.Text, "3 dispels") || !strings.Contains(s.Text, "2 deaths") {
		t.Fatal(s)
	}
}

// Optional read-only verification against a local snapshot and its original log.
func TestSummaryOriginalLogAudit(t *testing.T) {
	path := os.Getenv("LAMDA_SUMMARY_SNAPSHOT")
	if path == "" {
		t.Skip("local log audit not requested")
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var state struct {
		DamageMeters []DamageMeter `json:"damageMeters"`
	}
	if e = json.Unmarshal(data, &state); e != nil {
		t.Fatal(e)
	}
	kind := os.Getenv("LAMDA_SUMMARY_KIND")
	if kind == "" {
		kind = "key"
	}
	for _, m := range state.DamageMeters {
		if m.Kind != kind {
			continue
		}
		c := readSummaryCounts(context.Background(), os.Getenv("LAMDA_SUMMARY_LOG"), m)
		if !c.Known {
			t.Fatal("original log coverage unknown")
		}
		b, _ := json.Marshal(c)
		t.Log(m.Name, string(b))
		want := makeChatSummary(m, c)
		if os.Getenv("LAMDA_SUMMARY_PREPARED") == "1" {
			// Replay this exact pull through normal collection. The original
			// file may already contain later raid pulls by the time QA starts.
			s := New("")
			defer s.Close()
			s.SetEnabled(true)
			f, err := os.Open(os.Getenv("LAMDA_SUMMARY_LOG"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			scan := bufio.NewScanner(f)
			scan.Buffer(make([]byte, 64<<10), 2<<20)
			for scan.Scan() {
				s.Ingest(scan.Text(), m.EndedAt)
				if len(s.damageMeters) > 0 && s.damageMeters[0].ID == m.ID && s.damageMeters[0].Owner == m.Owner {
					break
				}
			}
			if err := scan.Err(); err != nil {
				t.Fatal(err)
			}
			got, err := s.PreparedSummaryForPlayer("recorded:"+m.ID, m.Owner)
			if err != nil || !got.MetricsKnown {
				t.Fatal("prepared report differs from original log", got, err)
			}
			for _, line := range want.Lines {
				if !strings.Contains(got.Text, line) {
					t.Fatal("missing original ranking/award", line, got)
				}
			}
			if kind == "boss" && len(s.bossAttempts) > 0 {
				t.Log("boss progression", s.bossAttempts[0].BossVisit, s.bossAttempts[0].Utility)
			}
			t.Log("prepared report matches original counts and rankings", len(got.Lines), "lines")
		}
		t.Log(want.Text)
		return
	}
	t.Fatal("no matching segment in snapshot", kind)
}

func TestRankedSummaryUsesTransferredDamageAndWholeFightClock(t *testing.T) {
	m := summaryFixture()
	m.Seconds = 10
	m.Rows[0].Added = 990
	m.Rows[1].Removed = 990
	m.Rows[0].DPS = 999999
	m.Rows[1].DPS = 999999
	v := makeChatSummary(m, summaryCounts{})
	if !strings.Contains(v.Text, "1. Aug 100 DPS~[LamdaMeters] 2. Ally 1 DPS") || !strings.Contains(v.Text, "Aug-adjusted DPS") || strings.Contains(v.Text, "cast") {
		t.Fatal(v)
	}
	m.Partial = true
	if !strings.Contains(makeChatSummary(m, summaryCounts{}).Text, "partial log") {
		t.Fatal("partial attribution concealed")
	}
}
func TestRankedRaidSummaryKeepsTopFiveAndBoundsEachLine(t *testing.T) {
	m := summaryFixture()
	m.Seconds = 100
	m.Rows = nil
	for i := 0; i < 40; i++ {
		m.Rows = append(m.Rows, DamageRow{GUID: fmt.Sprint(i), Name: fmt.Sprintf("Player%02d-LongRealm", i), Adjusted: int64(40000 - i*100)})
	}
	v := makeChatSummary(m, summaryCounts{Known: true})
	for i := 0; i < 40; i++ {
		if strings.Contains(v.Text, fmt.Sprintf("Player%02d ", i)) != (i < 5) {
			t.Fatalf("unexpected top-five rank %d: %s", i, v.Text)
		}
	}
	if len(v.Lines) != 8 || len(v.Text) > 4096 {
		t.Fatal(v)
	}
	for _, line := range v.Lines {
		if len(line) > 255 || strings.ContainsAny(line, "|\n~") {
			t.Fatal(line)
		}
	}
}
func TestBossUptimeIsOptInAndNeverAddedToKeyReport(t *testing.T) {
	s := ready()
	m := summaryFixture()
	m.Kind = "boss"
	m.ID = "boss/123/1"
	m.Seconds = 10
	s.own = aug
	s.damageMeters = []DamageMeter{m}
	s.reports = []Report{{ID: m.ID, GUID: aug, Player: "Aug-Realm", Seconds: 10, EbonSeconds: 8}}
	for _, scope := range []string{"boss", "boss-uptime"} {
		v, e := s.Summary(context.Background(), scope, rosterHash(m.Rows))
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(v.Text, "uptime") != (scope == "boss-uptime") {
			t.Fatal(v)
		}
		if scope == "boss-uptime" && !strings.Contains(v.Text, "80.0%") {
			t.Fatal(v)
		}
	}
}

func TestVerifiedPreviewIdentityWorksAfterRestartWithoutChangingCollector(t *testing.T) {
	s := ready()
	m := summaryFixture()
	s.damageMeters = []DamageMeter{m}
	s.own = "Player-Other-Character"
	v, err := s.SummaryForPlayer(context.Background(), "key", rosterHash(m.Rows), m.Owner)
	if err != nil || v.Segment != m.ID || s.own != "Player-Other-Character" {
		t.Fatal(v, err, s.own)
	}
	if _, err := s.SummaryForPlayer(context.Background(), "key", rosterHash(m.Rows), "Player-Different-Character"); err == nil {
		t.Fatal("another character's report was shared")
	}
}

func TestReviewedHistorySurvivesGroupAndLogRollover(t *testing.T) {
	s := ready()
	m := summaryFixture()
	m.Seconds = 10
	s.own = aug
	s.damageMeters = []DamageMeter{m}
	if _, err := s.SummaryForPlayer(context.Background(), "key", 42, aug); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Summary(context.Background(), "key", 42); err == nil {
		t.Fatal("unreviewed group mismatch allowed")
	}
	dir := t.TempDir()
	old := filepath.Join(dir, "WoWCombatLog-102726_114021.txt")
	current := filepath.Join(dir, "WoWCombatLog-102726_154911.txt")
	for _, p := range []string{old, current} {
		if err := os.WriteFile(p, []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
	}
	m.StartedAt = time.Date(2026, 10, 27, 13, 39, 58, 0, time.Local)
	if p := summaryLogPath(current, m); p != old {
		t.Fatal(p)
	}
}
func TestAwardsAreIndividualLinesAndPartialDamageIsQualified(t *testing.T) {
	m := summaryFixture()
	m.Seconds = 10
	m.Partial = true
	v := makeChatSummary(m, summaryCounts{Known: true, Dispels: map[string]int64{other: 1}, Deaths: map[string]int64{aug: 1}})
	if !strings.Contains(v.Text, "Recorded damage MVP") || !strings.Contains(v.Text, "1 dispel)") || !strings.Contains(v.Text, "1 death)") {
		t.Fatal(v)
	}
	for _, line := range v.Lines {
		if strings.Contains(line, ";") || len(line) > 255 || !utf8.ValidString(line) {
			t.Fatal(line)
		}
	}
	if len(v.Lines) != 6 {
		t.Fatal(v.Lines)
	}
}

func TestCompletedUtilityAwardsAreReadyWithoutOriginalLog(t *testing.T) {
	s := ready()
	ingest(s, 1, "CHALLENGE_MODE_START", "Immediate key", "123", "1", "10")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	ingest(s, 3, actor("SPELL_SUMMON", other, "Pet-1", 1)...)
	ingest(s, 4, actor("SPELL_DISPEL", other, "Creature-1", 1)...)
	ingest(s, 5, actor("SPELL_STOLEN", "Pet-1", "Creature-1", 1)...)
	ingest(s, 6, actor("UNIT_DIED", "Creature-1", other, 1)...)
	ingest(s, 10, "CHALLENGE_MODE_END", "123", "1", "10000")
	m := s.damageMeters[0]
	if m.Utility == nil || !m.Utility.Known || m.Utility.Dispels[other] != 2 || m.Utility.Deaths[other] != 1 {
		t.Fatal(m.Utility)
	}
	v, err := s.SummaryForPlayer(context.Background(), "key", 42, aug)
	if err != nil || !v.MetricsKnown || !strings.Contains(v.Text, "2 dispels") || !strings.Contains(v.Text, "1 death)") {
		t.Fatal(v, err)
	}
	// Persistence keeps the fast path even when the original log is unavailable.
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
	got, err := loaded.SummaryForPlayer(context.Background(), "key", 42, aug)
	if err != nil || !got.MetricsKnown || got.Text != v.Text {
		t.Fatal(got, err)
	}
}

func TestHistoricalUtilityScanIsCachedAfterFirstVerifiedRead(t *testing.T) {
	m := summaryFixture()
	m.StartedAt, _, _ = engine.RecordFields(record(2, "CHALLENGE_MODE_START", "Fixture", "123", "1", "10"))
	m.EndedAt = m.StartedAt.Add(8 * time.Second)
	path := filepath.Join(t.TempDir(), "original.txt")
	lines := []string{record(2, "CHALLENGE_MODE_START", "Fixture", "123", "1", "10"), record(3, actor("SPELL_DISPEL", other, "Creature-1", 1)...), record(10, "CHALLENGE_MODE_END", "123", "1", "8000")}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := ready()
	s.own = aug
	s.damageMeters = []DamageMeter{m}
	s.debug = newDebug(filepath.Join(t.TempDir(), "ledger.jsonl"))
	s.debug.SourceFile = path
	first, err := s.Summary(context.Background(), "key", rosterHash(m.Rows))
	if err != nil || !first.MetricsKnown {
		t.Fatal(first, err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	second, err := s.Summary(context.Background(), "key", rosterHash(m.Rows))
	if err != nil || !second.MetricsKnown || second.Text != first.Text {
		t.Fatal(second, err)
	}
}

func TestSummaryRequestWaitsForFinalKeyRecords(t *testing.T) {
	s := ready()
	ingest(s, 1, "CHALLENGE_MODE_START", "Buffered key", "123", "1", "10")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	type reply struct {
		summary ChatSummary
		err     error
	}
	done := make(chan reply, 1)
	go func() {
		v, err := s.SummaryForPlayerWhenReady(ctx, "key", 42, aug)
		done <- reply{v, err}
	}()
	select {
	case v := <-done:
		t.Fatal("returned before key finished", v)
	case <-time.After(30 * time.Millisecond):
	}
	ingest(s, 10, "CHALLENGE_MODE_END", "123", "1", "10000")
	select {
	case v := <-done:
		if v.err != nil || !v.summary.MetricsKnown || !strings.Contains(v.summary.Text, "Buffered key") || !strings.Contains(v.summary.Text, "Damage MVP") {
			t.Fatal(v)
		}
	case <-ctx.Done():
		t.Fatal("ready report was not returned")
	}
}

func TestSummaryWaitHonorsDeadlineCancellationAndPermanentErrors(t *testing.T) {
	s := ready()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := s.SummaryForPlayerWhenReady(ctx, "key", 42, aug); !errors.Is(err, errSummaryPending) || !strings.Contains(err.Error(), "no completed") {
		t.Fatal("lost pending reason", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err := s.SummaryForPlayerWhenReady(ctx, "key", 42, aug); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
	if _, err := s.SummaryForPlayerWhenReady(context.Background(), "key", 42, "invalid-player"); err == nil || errors.Is(err, errSummaryPending) {
		t.Fatal("permanent identity error should not retry", err)
	}
}
