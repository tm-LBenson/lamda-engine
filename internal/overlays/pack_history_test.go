package overlays

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func historyStore(t *testing.T) (*Store, []Rule) {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rules := packRules()
	if err = s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	return s, copyPackRules(rules)
}
func packVersion(rules []Rule, version int) []Rule {
	out := copyPackRules(rules)
	for i := range out {
		out[i].Pack.Version = version
	}
	return out
}

func TestPackHistoryCapturesBeforeUpdateAndSurvivesRestart(t *testing.T) {
	s, rules := historyStore(t)
	if len(s.PackHistory().Entries) != 0 {
		t.Fatal("ordinary save created a backup")
	}
	if err := s.SavePackUpdates(1, rules, []string{"local-pack"}); err != nil {
		t.Fatal(err)
	}
	if len(s.PackHistory().Entries) != 0 {
		t.Fatal("unchanged update created a backup")
	}
	next := packVersion(rules, 2)
	next[0].Style.X = 73
	if err := s.SavePackUpdates(2, next, []string{"local-pack"}); err != nil {
		t.Fatal(err)
	}
	h := s.PackHistory()
	if len(h.Entries) != 1 || h.Entries[0].Version != 1 || h.Entries[0].Members != 2 {
		t.Fatal(h)
	}
	snapshot, err := s.PackSnapshot(h.Entries[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Rules[0].Style.X == 73 || snapshot.Rules[0].Pack.Version != 1 {
		t.Fatal("captured updated data")
	}
	snapshot.Rules[0].Style.X = 99
	again, _ := s.PackSnapshot(snapshot.ID)
	if again.Rules[0].Style.X == 99 {
		t.Fatal("caller changed stored snapshot")
	}
	before, _ := os.ReadFile(filepath.Join(s.dir, "rules.json"))
	if err = s.SavePackUpdates(2, packVersion(next, 3), []string{"local-pack"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if len(s.PackHistory().Entries) != 1 {
		t.Fatal("conflict created a backup")
	}
	if err = s.SavePackUpdates(3, packVersion(next, 3), []string{"missing"}); err == nil {
		t.Fatal("invalid target accepted")
	}
	after, _ := os.ReadFile(filepath.Join(s.dir, "rules.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed update replaced rules")
	}
	s, err = Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if h = s.PackHistory(); h.Error != "" || len(h.Entries) != 1 {
		t.Fatal(h)
	}
	snapshot, err = s.PackSnapshot(h.Entries[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// A reviewed restore is another revisioned save and backs up the replaced version.
	if err = s.SavePackUpdates(3, snapshot.Rules, []string{"local-pack"}); err != nil {
		t.Fatal(err)
	}
	h = s.PackHistory()
	if len(h.Entries) != 2 || h.Entries[0].Version != 2 || s.doc.Rules[0].Pack.Version != 1 {
		t.Fatal("restore lost redo snapshot", h)
	}
	if err = s.DeletePackSnapshot(h.Revision-1, h.Entries[1].ID); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = s.DeletePackSnapshot(h.Revision, h.Entries[1].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PackSnapshot(h.Entries[1].ID); !os.IsNotExist(err) {
		t.Fatal("deleted snapshot survives", err)
	}
}

func TestPackHistoryRetentionAndFailurePreserveCurrentRules(t *testing.T) {
	s, rules := historyStore(t)
	for version := 2; version <= 7; version++ {
		if err := s.SavePackUpdates(s.doc.Revision, packVersion(rules, version), []string{"local-pack"}); err != nil {
			t.Fatal(err)
		}
	}
	h := s.PackHistory()
	if len(h.Entries) != 3 || h.Entries[0].Version != 6 || h.Entries[2].Version != 4 {
		t.Fatal("wrong retention", h)
	}
	before, _ := os.ReadFile(filepath.Join(s.dir, "rules.json"))
	revision := s.doc.Revision
	previous := filepath.Join(s.dir, "pack-history.previous.json")
	if err := os.Remove(previous); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(previous, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePackUpdates(revision, packVersion(rules, 8), []string{"local-pack"}); err == nil {
		t.Fatal("failed backup accepted")
	}
	after, _ := os.ReadFile(filepath.Join(s.dir, "rules.json"))
	if !bytes.Equal(before, after) || s.doc.Revision != revision || s.PackHistory().Revision != h.Revision {
		t.Fatal("backup failure replaced current rules")
	}
	if err := os.Remove(previous); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePackUpdates(revision, packVersion(rules, 8), []string{"local-pack"}); err != nil {
		t.Fatal(err)
	}
}

func TestPackHistoryCorruptionDoesNotBlockOrdinaryEdits(t *testing.T) {
	s, rules := historyStore(t)
	path := filepath.Join(s.dir, "pack-history.json")
	bad := []byte("{broken backups")
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	h := s.PackHistory()
	if h.Error == "" || !h.Recoverable {
		t.Fatal("corruption hidden", h)
	}
	if err = s.Save(1, packVersion(rules, 2)); err != nil {
		t.Fatal("ordinary edits blocked", err)
	}
	if err = s.SavePackUpdates(2, packVersion(rules, 3), []string{"local-pack"}); err == nil {
		t.Fatal("corrupt backups overwritten")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, bad) {
		t.Fatal("bad file changed")
	}
	if err = s.RecoverPackHistory(h.Revision); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(s.dir, "pack-history.corrupt-*.json"))
	if len(backups) != 1 {
		t.Fatal(backups)
	}
	got, _ = os.ReadFile(backups[0])
	if !bytes.Equal(got, bad) {
		t.Fatal("recovery backup differs")
	}
	if err = s.SavePackUpdates(2, packVersion(rules, 3), []string{"local-pack"}); err != nil {
		t.Fatal(err)
	}
	if s.PackHistory().Error != "" || len(s.PackHistory().Entries) != 1 {
		t.Fatal("recovery failed")
	}
}

func TestPackHistoryValidatesSnapshotsAndProtectsMedia(t *testing.T) {
	s, rules := historyStore(t)
	lib, err := s.SaveMedia(0, []MediaUpload{{Name: "Old alert", Data: soundMedia()}})
	if err != nil {
		t.Fatal(err)
	}
	id := lib.Assets[0].ID
	rules[0].Style.OnShow = "media:" + id
	if err = s.Save(1, rules); err != nil {
		t.Fatal(err)
	}
	next := packVersion(rules, 2)
	next[0].Style.OnShow = "chime"
	if err = s.SavePackUpdates(2, next, []string{"local-pack"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditMedia(lib.Revision, id, "", true); err == nil || !strings.Contains(err.Error(), "backup") {
		t.Fatal("removed rollback media", err)
	}
	h := s.PackHistory()
	if err = s.DeletePackSnapshot(h.Revision, h.Entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditMedia(lib.Revision, id, "", true); err != nil {
		t.Fatal("deleted backup still retains media", err)
	}
	entry := PackSnapshot{ID: "test", CreatedAt: "2026-10-13T00:00:00Z", Rules: packRules()}
	for _, change := range []func(*packHistoryFile){func(f *packHistoryFile) { f.Version = 2 }, func(f *packHistoryFile) { f.Entries[0].Rules[0].Pack.Version = 2 }, func(f *packHistoryFile) { f.Entries[0].CreatedAt = "invalid" }, func(f *packHistoryFile) { f.Entries = append(f.Entries, f.Entries[0]) }} {
		dir := t.TempDir()
		f := packHistoryFile{Version: 1, Entries: []PackSnapshot{{ID: entry.ID, CreatedAt: entry.CreatedAt, Rules: copyPackRules(entry.Rules)}}}
		change(&f)
		b, _ := json.Marshal(f)
		_ = os.WriteFile(filepath.Join(dir, "pack-history.json"), b, 0600)
		s, err := Open(dir)
		if err != nil || s.PackHistory().Error == "" {
			t.Fatal("invalid history accepted", string(b), err)
		}
	}
}

func TestPackHistoryGlobalRetention(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var rules []Rule
	var instances []string
	for i := 0; i < 6; i++ {
		pack := packRules()
		suffix := fmt.Sprint(i)
		for j := range pack {
			pack[j].ID += suffix
			pack[j].Pack.Instance += suffix
			pack[j].Roots[0].ID += suffix
			pack[j].Roots[0].Children[0].ID += suffix
			for key, local := range pack[j].Pack.Nodes {
				pack[j].Pack.Nodes[key] = local + suffix
			}
		}
		rules = append(rules, pack...)
		instances = append(instances, pack[0].Pack.Instance)
	}
	if err = s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	if err = s.SavePackUpdates(1, packVersion(rules, 2), instances); err != nil {
		t.Fatal(err)
	}
	h := s.PackHistory()
	if len(h.Entries) != 5 || h.Entries[0].Instance != instances[5] || h.Entries[4].Instance != instances[1] {
		t.Fatal(h)
	}
	s, err = Open(s.dir)
	if err != nil || s.PackHistory().Error != "" || len(s.PackHistory().Entries) != 5 {
		t.Fatal("retained backups cannot reopen", err)
	}
}
