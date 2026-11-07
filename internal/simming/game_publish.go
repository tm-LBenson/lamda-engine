package simming

import (
	"context"
	"log"
	"path/filepath"
	"time"
)

type GameStatus struct {
	Schema      int             `json:"schema"`
	Client      string          `json:"client"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Jobs        []GameJobStatus `json:"jobs"`
	ExportError string          `json:"exportError,omitempty"`
}
type GameJobStatus struct {
	ID         string     `json:"id"`
	RequestID  string     `json:"requestID,omitempty"`
	Task       string     `json:"task"`
	State      string     `json:"state"`
	CreatedAt  time.Time  `json:"createdAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// PublishResults serializes both periodic exports and reload-triggered exports.
// State is read inside the lock so an older publisher cannot overwrite a newer
// report. A reload also requires its exact request to survive history limits.
func (w *Workspace) PublishResults(addonsDir, requiredRequest string) (WorkspaceState, error) {
	w.publicationMu.Lock()
	defer w.publicationMu.Unlock()
	now := time.Now().UTC()
	w.mu.Lock()
	revision := w.revision
	w.mu.Unlock()
	// Refresh capability/audit summaries at least once a minute. Job and capture
	// changes invalidate immediately; explicit reloads always validate fresh state.
	if requiredRequest != "" || w.publicationData == nil || revision != w.publicationRevision || now.Sub(w.publicationAt) >= time.Minute {
		state := w.State()
		data, err := buildGameResults(state, now, requiredRequest)
		if err != nil {
			return state, err
		}
		w.publicationState, w.publicationData = state, data
		w.publicationRevision, w.publicationAt = revision, now
	}
	return w.publicationState, writeGameResults(addonsDir, w.publicationState.Client, w.publicationData)
}

// The addon reads generated results only at reload. The tray reads this separate
// small status file and reports completion without controlling the game.
func PublishGameResults(ctx context.Context, w *Workspace, addonsDir string, enabled ...func() bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastProblem := ""
	for {
		if len(enabled) > 0 && enabled[0] != nil && !enabled[0]() {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}
		state, err := w.PublishResults(addonsDir, "")
		now := time.Now().UTC()
		problem := ""
		if err != nil {
			problem = err.Error()
		}
		if problem != "" && problem != lastProblem {
			log.Printf("In-game sim results: %s", problem)
		}
		lastProblem = problem
		status := GameStatus{Schema: 1, Client: state.Client, UpdatedAt: now, Jobs: []GameJobStatus{}, ExportError: problem}
		for _, j := range state.Jobs {
			if j.RequestID == "" {
				continue
			}
			status.Jobs = append(status.Jobs, GameJobStatus{ID: j.ID, RequestID: j.RequestID, Task: j.GameTask, State: j.State, CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt, Error: j.Error})
			if len(status.Jobs) == 12 {
				break
			}
		}
		if err := writeJSON(filepath.Join(w.dir, "game-status.json"), status); err != nil {
			log.Printf("In-game sim status: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
