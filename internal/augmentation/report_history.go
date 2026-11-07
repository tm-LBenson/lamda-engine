package augmentation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

const reportHistoryBudget = 1 << 20
const reportHistoryPageSize = 8

// A report is a small, self-contained recap. It has no actor/spell/target
// tables or combat events and is never included in the polling State payload.
type savedReport struct {
	ID           string    `json:"id"`
	Owner        string    `json:"owner"`
	Segment      string    `json:"segment"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	EndedAt      time.Time `json:"endedAt"`
	Pulls        int       `json:"pulls,omitempty"`
	Outcome      string    `json:"outcome"`
	Lines        []string  `json:"lines"`
	RankingRows  int       `json:"rankingRows"`
	AwardsHeader string    `json:"awardsHeader"`
	UptimeLine   string    `json:"uptimeLine,omitempty"`
	MetricsKnown bool      `json:"metricsKnown,omitempty"`
	Coverage     int       `json:"coverage,omitempty"`
	AwardVersion int       `json:"awardVersion,omitempty"`
	RoleVersion  int       `json:"roleVersion,omitempty"`
}

type reportHistoryFile struct {
	Schema  int           `json:"schema"`
	Client  string        `json:"client"`
	Limit   int           `json:"limit"`
	Reports []savedReport `json:"reports"`
}

func historyLimit(n int) bool { return n >= 25 && n <= 200 && n%25 == 0 }
func historyText(v string, max int) bool {
	return len(v) <= max && utf8.ValidString(v) && !strings.ContainsAny(v, "|~") && !strings.ContainsFunc(v, unicode.IsControl)
}
func validSavedReport(r savedReport) bool {
	if len(r.ID) != 32 || !player(r.Owner) || !historyText(r.Owner, 160) || !historyText(r.Segment, 200) || !historyText(r.Name, 120) || r.EndedAt.IsZero() || (r.Kind != "boss" && r.Kind != "key") || r.RankingRows < 0 || r.RankingRows > 5 || r.Pulls < 0 || r.Pulls > 512 || (r.Outcome != "kill" && r.Outcome != "wipe" && r.Outcome != "complete" && r.Outcome != "saved") {
		return false
	}
	if _, err := hex.DecodeString(r.ID); err != nil || strings.ToLower(r.ID) != r.ID || len(r.Lines) == 0 || len(r.Lines) > 50 || r.RankingRows >= len(r.Lines) || !strings.HasPrefix(r.AwardsHeader, "[LamdaMeters] ") || !historyText(r.AwardsHeader, 255) || !historyText(r.UptimeLine, 255) {
		return false
	}
	for _, line := range r.Lines {
		if !strings.HasPrefix(line, "[LamdaMeters] ") || !historyText(line, 255) {
			return false
		}
	}
	data, _ := json.Marshal(r)
	return len(data) <= 8192
}

func (s *Store) archiveReportLocked(m DamageMeter, pulls []DamageMeter, standard, uptime ChatSummary) {
	if m.Kind != "boss" && m.Kind != "key" || m.Metric != 0 || m.Live || !m.OutcomeKnown || m.EndedAt.IsZero() {
		return
	}
	identity := summaryKey(m)
	if m.Kind == "boss" && m.BossVisit != "" {
		// One evolving recap per boss visit, rather than a menu row per wipe.
		identity = m.Owner + "/boss/" + m.BossVisit + "/" + encounterID(m) + "/" + m.Difficulty
	}
	sum := sha256.Sum256([]byte(identity))
	r := savedReport{ID: hex.EncodeToString(sum[:16]), Owner: m.Owner, Segment: m.ID, Name: summaryText(m.Name, 100), Kind: m.Kind, EndedAt: m.EndedAt, Pulls: len(pulls), Outcome: "saved", Lines: append([]string{}, standard.Lines...), RankingRows: standard.rankingRows, AwardsHeader: standard.awardsHeader}
	if m.OutcomeKnown {
		if m.Kind == "key" && m.Success {
			r.Outcome = "complete"
		} else if m.Kind == "boss" && m.Success {
			r.Outcome = "kill"
		} else if m.Kind == "boss" {
			r.Outcome = "wipe"
		}
	}
	if len(uptime.Lines) > len(standard.Lines) {
		r.UptimeLine = uptime.Lines[len(uptime.Lines)-1]
	}
	r.MetricsKnown = standard.MetricsKnown
	if m.Utility != nil {
		r.Coverage, r.AwardVersion, r.RoleVersion = m.Utility.EncounterAwardVersion, m.Utility.AwardVersion, m.Utility.OverhealRoleVersion
	}
	if !validSavedReport(r) {
		return
	}
	for i, old := range s.reportHistory {
		if old.ID == r.ID {
			if old.EndedAt.After(r.EndedAt) {
				return
			}
			if old.EndedAt.Equal(r.EndedAt) && (old.Coverage > r.Coverage || old.AwardVersion > r.AwardVersion || old.RoleVersion > r.RoleVersion) {
				return
			}
			if old.EndedAt.Equal(r.EndedAt) && r.UptimeLine == "" {
				r.UptimeLine = old.UptimeLine
			}
			a, _ := json.Marshal(old)
			b, _ := json.Marshal(r)
			if string(a) == string(b) {
				return
			}
			s.reportHistory = append(s.reportHistory[:i], s.reportHistory[i+1:]...)
			break
		}
	}
	s.reportHistory = append(s.reportHistory, r)
	s.trimReportHistoryLocked()
	s.historyRevision++
}

func (s *Store) trimReportHistoryLocked() {
	if !historyLimit(s.reportHistoryLimit) {
		s.reportHistoryLimit = 100
	}
	sort.Slice(s.reportHistory, func(i, j int) bool {
		if s.reportHistory[i].EndedAt.Equal(s.reportHistory[j].EndedAt) {
			return s.reportHistory[i].ID < s.reportHistory[j].ID
		}
		return s.reportHistory[i].EndedAt.After(s.reportHistory[j].EndedAt)
	})
	bytes, keep := 0, 0
	for _, r := range s.reportHistory {
		data, _ := json.Marshal(r)
		if keep >= s.reportHistoryLimit || bytes+len(data) > reportHistoryBudget {
			break
		}
		bytes += len(data)
		keep++
	}
	s.reportHistory = s.reportHistory[:keep]
}

func (s *Store) loadReportHistory() {
	s.reportHistoryLimit = 100
	if s.path == "" {
		return
	}
	f, err := os.Open(filepath.Join(filepath.Dir(s.path), "report-history.json"))
	if err != nil {
		return
	}
	defer f.Close()
	var data reportHistoryFile
	if json.NewDecoder(io.LimitReader(f, 2<<20)).Decode(&data) != nil || data.Schema != 1 || data.Client != "retail" || !historyLimit(data.Limit) || len(data.Reports) > 200 {
		return
	}
	s.reportHistoryLimit = data.Limit
	seen := map[string]bool{}
	for _, r := range data.Reports {
		if validSavedReport(r) && !seen[r.ID] {
			s.reportHistory = append(s.reportHistory, r)
			seen[r.ID] = true
		}
	}
	s.trimReportHistoryLocked()
}

// Only the background flush / shutdown path writes this small separate file.
func (s *Store) flushReportHistory() error {
	s.historyIO.Lock()
	defer s.historyIO.Unlock()
	s.mu.Lock()
	if s.path == "" || s.historyRevision == s.historySaved {
		s.mu.Unlock()
		return nil
	}
	rev := s.historyRevision
	data := reportHistoryFile{Schema: 1, Client: "retail", Limit: s.reportHistoryLimit, Reports: append([]savedReport{}, s.reportHistory...)}
	path := filepath.Join(filepath.Dir(s.path), "report-history.json")
	s.mu.Unlock()
	if err := engine.WriteJSON(path, data); err != nil {
		return err
	}
	s.mu.Lock()
	s.historySaved = rev
	s.mu.Unlock()
	return nil
}

func (s *Store) reportHistorySummary(scope, owner string, options []uint8) (ChatSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	option := uint8(0)
	if len(options) == 1 {
		option = options[0]
	}
	if len(options) > 1 || option > 7 || option&5 == 5 {
		return ChatSummary{}, fmt.Errorf("invalid report options")
	}
	if strings.HasPrefix(scope, "history:") {
		p := strings.Split(scope, ":")
		if len(p) != 4 {
			return ChatSummary{}, fmt.Errorf("invalid history page")
		}
		page, e1 := strconv.Atoi(p[1])
		limit, e2 := strconv.Atoi(p[2])
		if e1 != nil || e2 != nil || page < 1 || page > 25 || !historyLimit(limit) || (p[3] != "all" && p[3] != "boss" && p[3] != "key") {
			return ChatSummary{}, fmt.Errorf("invalid history page")
		}
		if limit != s.reportHistoryLimit {
			s.reportHistoryLimit = limit
			s.trimReportHistoryLocked()
			s.historyRevision++
		}
		rows := []savedReport{}
		for _, r := range s.reportHistory {
			if r.Owner == owner && (p[3] == "all" || r.Kind == p[3]) {
				rows = append(rows, r)
			}
		}
		pages := max(1, (len(rows)+reportHistoryPageSize-1)/reportHistoryPageSize)
		page = min(page, pages)
		lines := []string{fmt.Sprintf("[LamdaMeters] [HISTORY %d %d %d %d %s]", page, pages, len(rows), limit, p[3])}
		for i := (page - 1) * reportHistoryPageSize; i < min(page*reportHistoryPageSize, len(rows)); i++ {
			r := rows[i]
			lines = append(lines, fmt.Sprintf("[LamdaMeters] [H %s %d %s %d %s] %s", r.ID, r.EndedAt.Unix(), r.Kind, r.Pulls, r.Outcome, r.Name))
		}
		return ChatSummary{Segment: scope, Name: "Report history", Lines: lines, Text: strings.Join(lines, "~")}, nil
	}
	withUptime := strings.HasPrefix(scope, "savedu:")
	id := strings.TrimPrefix(strings.TrimPrefix(scope, "saved:"), "savedu:")
	for _, r := range s.reportHistory {
		if r.Owner == owner && r.ID == id {
			v := ChatSummary{Segment: r.Segment, Name: r.Name, EndedAt: r.EndedAt, Lines: append([]string{}, r.Lines...), rankingRows: r.RankingRows, awardsHeader: r.AwardsHeader, MetricsKnown: r.MetricsKnown}
			stamp := " · saved " + r.EndedAt.Format("Jan 02 15:04")
			v.Lines[0] = summaryText(v.Lines[0], 220) + stamp
			v.awardsHeader = summaryText(v.awardsHeader, 220) + stamp
			if withUptime && r.UptimeLine != "" {
				v.Lines = append(v.Lines, r.UptimeLine)
			}
			return summaryWithOptions(v, option), nil
		}
	}
	return ChatSummary{}, fmt.Errorf("Saved report is no longer retained for this character")
}
