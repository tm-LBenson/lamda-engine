package augmentation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func archiveFixture(s *Store, number int, owner string) savedReport {
	m := DamageMeter{Owner: owner, ID: fmt.Sprintf("key/1877/%d", number), Kind: "key", Name: "Temple of Sethraliss +10", OutcomeKnown: true, Success: true, EndedAt: time.Unix(1800000000+int64(number), 0)}
	v := ChatSummary{rankingRows: 1, awardsHeader: "[LamdaMeters] Temple awards", MetricsKnown: true, Lines: []string{"[LamdaMeters] Temple damage", "[LamdaMeters] 1. Aug: 1.0M DPS", "[LamdaMeters] Damage MVP: Aug", "[LamdaMeters] Human Ground Wire: Ally - 40 beam ticks"}}
	s.mu.Lock()
	s.archiveReportLocked(m, nil, v, v)
	s.mu.Unlock()
	return s.reportHistory[0]
}

func TestReportHistoryBoundedRecapsSurviveMeterEvictionAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := New(path)
	for i := 0; i < 140; i++ {
		archiveFixture(s, i, aug)
	}
	if len(s.reportHistory) != 100 || len(s.damageMeters) != 0 || len(s.bossAttempts) != 0 {
		t.Fatal("history grew full meter storage", len(s.reportHistory))
	}
	id := s.reportHistory[0].ID
	s.Close()
	bytes, err := os.ReadFile(filepath.Join(filepath.Dir(path), "report-history.json"))
	if err != nil || len(bytes) > 100000 {
		t.Fatal(err, len(bytes))
	}
	loaded := New(path)
	defer loaded.Close()
	index, err := loaded.PreparedSummaryForPlayer("history:1:100:all", aug)
	if err != nil || len(index.Lines) != 9 || !strings.Contains(index.Text, "HISTORY 1 13 100 100 all") {
		t.Fatal(err, index)
	}
	for _, line := range index.Lines {
		if len(line) > 255 {
			t.Fatal(line)
		}
	}
	ranks, err := loaded.PreparedSummaryForPlayer("saved:"+id, aug, 4)
	if err != nil || !strings.Contains(ranks.Text, "1. Aug") || strings.Contains(ranks.Text, "Ground Wire") || !strings.Contains(ranks.Lines[0], "saved ") {
		t.Fatal(err, ranks)
	}
	awards, err := loaded.PreparedSummaryForPlayer("saved:"+id, aug, 3)
	if err != nil || !strings.Contains(awards.Text, "Ground Wire") || strings.Contains(awards.Text, "1. Aug") || strings.Contains(awards.Text, "Damage MVP") || !awards.MetricsKnown {
		t.Fatal(err, awards)
	}
	state, _ := json.Marshal(loaded.Snapshot())
	if strings.Contains(string(state), "Human Ground Wire") {
		t.Fatal("archive copied into the live polling payload")
	}
}

func TestReportHistoryFoldsBossAttemptsAndNeverDowngradesLatestRecap(t *testing.T) {
	s := ready()
	m := DamageMeter{Owner: aug, ID: "boss/3421/1", Kind: "boss", Name: "The Twin Fangs", BossVisit: "raid-evening", Difficulty: "15", OutcomeKnown: true, EndedAt: time.Unix(1800000000, 0)}
	v := ChatSummary{awardsHeader: "[LamdaMeters] Awards", Lines: []string{"[LamdaMeters] Pull one"}}
	s.archiveReportLocked(m, []DamageMeter{m}, v, v)
	first := m
	m.ID = "boss/3421/2"
	m.EndedAt = m.EndedAt.Add(time.Minute)
	m.Success = true
	v.Lines = []string{"[LamdaMeters] Kill on pull two"}
	uptime := v
	uptime.Lines = append(append([]string{}, v.Lines...), "[LamdaMeters] Aug Ebon Might uptime: 80.0% (recorded)")
	s.archiveReportLocked(m, []DamageMeter{first, m}, v, uptime)
	s.archiveReportLocked(first, []DamageMeter{first}, ChatSummary{Lines: []string{"[LamdaMeters] Old wipe"}}, v)
	if len(s.reportHistory) != 1 || s.reportHistory[0].Pulls != 2 || s.reportHistory[0].Outcome != "kill" || !strings.Contains(s.reportHistory[0].Lines[0], "Kill on pull two") {
		t.Fatal(s.reportHistory)
	}
	r, err := s.PreparedSummaryForPlayer("savedu:"+s.reportHistory[0].ID, aug)
	if err != nil || !strings.Contains(r.Text, "uptime") {
		t.Fatal(err, r)
	}
	m.BossVisit = "next-evening"
	m.EndedAt = m.EndedAt.Add(24 * time.Hour)
	s.archiveReportLocked(m, nil, v, v)
	if len(s.reportHistory) != 2 {
		t.Fatal("separate raid visit collapsed")
	}
}

func TestReportHistoryRetentionFiltersAndCharacterIsolation(t *testing.T) {
	s := ready()
	for i := 0; i < 80; i++ {
		archiveFixture(s, i, aug)
	}
	foreign := archiveFixture(s, 90, other)
	_, err := s.PreparedSummaryForPlayer("saved:"+foreign.ID, aug)
	if err == nil {
		t.Fatal("another character's report was accessible")
	}
	index, err := s.PreparedSummaryForPlayer("history:25:25:key", aug)
	if err != nil || len(s.reportHistory) != 25 || !strings.Contains(index.Text, "HISTORY 3 3 24 25 key") {
		t.Fatal(err, index, len(s.reportHistory))
	}
	index, err = s.PreparedSummaryForPlayer("history:1:25:boss", aug)
	if err != nil || len(index.Lines) != 1 || !strings.Contains(index.Text, "HISTORY 1 1 0 25 boss") {
		t.Fatal(err, index)
	}
	for _, scope := range []string{"history:0:100:all", "history:1:999:all", "history:1:26:all", "history:1:100:unknown"} {
		if _, err := s.PreparedSummaryForPlayer(scope, aug); err == nil {
			t.Fatal(scope)
		}
	}
}

func TestReportHistoryHardByteBudgetAndMalformedDiskEntries(t *testing.T) {
	s := ready()
	s.reportHistoryLimit = 200
	for i := 0; i < 200; i++ {
		m := DamageMeter{Owner: aug, Kind: "key", ID: fmt.Sprint("key/1877/", i), Name: "Temple", OutcomeKnown: true, Success: true, EndedAt: time.Unix(1800000000+int64(i), 0)}
		v := ChatSummary{awardsHeader: "[LamdaMeters] Awards"}
		for n := 0; n < 30; n++ {
			v.Lines = append(v.Lines, "[LamdaMeters] "+strings.Repeat("a", 220))
		}
		s.archiveReportLocked(m, nil, v, v)
	}
	bytes, _ := json.Marshal(s.reportHistory)
	if len(bytes) > reportHistoryBudget+500 || len(s.reportHistory) < 100 || len(s.reportHistory) >= 200 {
		t.Fatal(len(bytes), len(s.reportHistory))
	}
	path := filepath.Join(t.TempDir(), "state.json")
	r := s.reportHistory[0]
	r.Lines[0] = "[LamdaMeters] |Hbad:link|h"
	bad, _ := json.Marshal(reportHistoryFile{Schema: 1, Client: "retail", Limit: 100, Reports: []savedReport{r}})
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "report-history.json"), bad, 0600); err != nil {
		t.Fatal(err)
	}
	loaded := New(path)
	defer loaded.Close()
	if len(loaded.reportHistory) != 0 {
		t.Fatal("unverified disk report accepted")
	}
}

func TestReportHistoryConcurrentBackgroundSaveAndRead(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "state.json"))
	defer s.Close()
	var wg sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if worker == 0 {
					archiveFixture(s, i, aug)
				} else if worker == 1 {
					_, _ = s.PreparedSummaryForPlayer("history:1:100:all", aug)
				} else if err := s.flushReportHistory(); err != nil {
					t.Error(err)
				}
			}
		}(worker)
	}
	wg.Wait()
}

func TestReportHistoryDoesNotTurnMissingBoundariesIntoCompletedReports(t *testing.T) {
	s := ready()
	m := DamageMeter{Owner: aug, Kind: "key", ID: "key/1877/1", Name: "Temple", EndedAt: time.Unix(1800000000, 0)}
	v := ChatSummary{awardsHeader: "[LamdaMeters] Awards", Lines: []string{"[LamdaMeters] Incomplete old copy"}}
	s.archiveReportLocked(m, nil, v, v)
	if len(s.reportHistory) != 0 {
		t.Fatal("unverified end boundary archived")
	}
	m.OutcomeKnown = true
	m.Success = true
	m.Metric = 2
	s.archiveReportLocked(m, nil, v, v)
	if len(s.reportHistory) != 0 {
		t.Fatal("non-damage totals archived as DPS")
	}
	m.Metric = 0
	s.archiveReportLocked(m, nil, v, v)
	if len(s.reportHistory) != 1 {
		t.Fatal("verified complete key omitted")
	}
}
