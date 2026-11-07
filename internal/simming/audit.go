package simming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type SourceAudit struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Pinned      string    `json:"pinned"`
	Observed    string    `json:"observed"`
	Status      string    `json:"status"`
	CheckedAt   time.Time `json:"checkedAt"`
	LastSuccess time.Time `json:"lastSuccess"`
}
type Audit struct {
	Revision  string        `json:"revision"`
	CheckedAt time.Time     `json:"checkedAt"`
	Sources   []SourceAudit `json:"sources"`
	Note      string        `json:"note"`
}
type Auditor struct {
	mu       sync.Mutex
	current  Audit
	path     string
	checking bool
}

func NewAuditor(dir string) *Auditor {
	a := &Auditor{path: filepath.Join(dir, "audit.json")}
	if data, err := os.ReadFile(a.path); err == nil {
		_ = json.Unmarshal(data, &a.current)
	}
	return a
}
func (a *Auditor) State() Audit {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.current
	r.Sources = append([]SourceAudit{}, r.Sources...)
	return r
}

// Monitoring detects drift. It never downloads/installs a calculator or marks a
// combat model as verified. Season rules and representative model checks remain
// part of the separate weekly review recorded in the provider registration.
func (a *Auditor) Check(ctx context.Context, force bool) bool {
	if ctx.Err() != nil {
		return false
	}
	a.mu.Lock()
	if a.checking || (!force && a.current.Revision == "2026-09-23.1" && time.Since(a.current.CheckedAt) < 24*time.Hour) {
		a.mu.Unlock()
		return false
	}
	a.checking = true
	previous := a.current
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.checking = false; a.mu.Unlock() }()
	sources := []SourceAudit{
		{Name: "SimulationCraft exporter", URL: "https://api.github.com/repos/simulationcraft/simc-addon/commits/master", Pinned: "305f97962d94428727d6b359199c5f8c2d998c76"},
		{Name: "SimulationCraft", URL: "https://api.github.com/repos/simulationcraft/simc/commits/midnight", Pinned: "fa22f84d2a13255d7325826f89a46bbb9181233d"},
		{Name: "WoWSims Forever", URL: "https://api.github.com/repos/wowsims/forever/commits/master", Pinned: "9e7fd62313cb27f1ae5e46bb172015f4149458a3"},
		{Name: "Questionably Epic", URL: "https://api.github.com/repos/Voulk/QuestionablyEpic/commits/dev", Pinned: "3e998d449f38739d15dad1b26aa954822de58216"},
	}
	client := &http.Client{Timeout: 12 * time.Second}
	for i := range sources {
		if ctx.Err() != nil {
			return false
		}
		s := &sources[i]
		s.CheckedAt = time.Now().UTC()
		s.Status = "unavailable"
		for _, p := range previous.Sources {
			if p.Name == s.Name {
				s.LastSuccess = p.LastSuccess
				s.Observed = p.Observed
			}
		}
		req, err := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "LamdaUI-Sim-Audit/1")
		response, err := client.Do(req)
		if err != nil {
			continue
		}
		var head struct {
			SHA string `json:"sha"`
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&head)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || err != nil || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(head.SHA) {
			continue
		}
		s.Observed = head.SHA
		s.LastSuccess = s.CheckedAt
		s.Status = "unchanged"
		if s.Observed != s.Pinned {
			s.Status = "changed"
		}
	}
	if ctx.Err() != nil {
		return false
	}
	current := Audit{Revision: "2026-09-23.1", CheckedAt: time.Now().UTC(), Sources: sources, Note: "Daily source monitoring only. Loot rules, mechanics and recommendation quality require a recorded weekly review. No automatic provider updates."}
	a.mu.Lock()
	a.current = current
	a.mu.Unlock()
	_ = writeJSON(a.path, current)
	return true
}
func (a Audit) Warnings(now time.Time) []string { return a.ProviderWarnings(now, "SimulationCraft") }
func (a Audit) ProviderWarnings(now time.Time, provider string) []string {
	if provider == "LamdaUI item stats" {
		return nil
	}
	var out []string
	if a.Revision != "2026-09-23.1" || a.CheckedAt.IsZero() || now.Sub(a.CheckedAt) > 36*time.Hour {
		out = append(out, "Daily source monitoring is overdue or has not completed.")
	}
	for _, s := range a.Sources {
		if (s.Name == provider || s.Name == "SimulationCraft exporter") && s.Status != "unchanged" {
			out = append(out, fmt.Sprintf("%s source audit: %s; review required.", s.Name, s.Status))
		}
	}
	return out
}
