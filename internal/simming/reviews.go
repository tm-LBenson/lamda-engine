package simming

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Review struct {
	ID         string    `json:"id"`
	At         time.Time `json:"at"`
	SnapshotID string    `json:"snapshotId"`
	Build      string    `json:"build"`
	Spec       int       `json:"spec"`
	Provider   string    `json:"provider"`
	Version    string    `json:"version"`
	Notes      string    `json:"notes"`
}

func (w *Workspace) RecordReview(id, notes string) (Review, error) {
	notes = strings.TrimSpace(notes)
	if len(notes) < 10 || len(notes) > 4000 {
		return Review{}, fmt.Errorf("add a review note between 10 and 4000 characters")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.snapshots[id]
	if s == nil {
		return Review{}, fmt.Errorf("snapshot not found")
	}
	now := time.Now().UTC()
	cap := w.provider.Capability(s, now)
	review := Review{ID: fmt.Sprintf("%d", now.UnixNano()), At: now, SnapshotID: id, Build: s.Build, Spec: s.Character.Spec, Provider: cap.Name, Version: cap.Version, Notes: notes}
	// A saved note is evidence, not automatic approval of a combat model.
	if err := writeJSON(filepath.Join(w.dir, "reviews", review.ID+".json"), review); err != nil {
		return Review{}, err
	}
	return review, nil
}
