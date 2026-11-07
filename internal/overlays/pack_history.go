package overlays

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxPackHistoryBytes = 24 * 1024 * 1024

type PackSnapshot struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Rules     []Rule `json:"rules"`
}
type packHistoryFile struct {
	Version  int            `json:"version"`
	Revision int64          `json:"revision"`
	Entries  []PackSnapshot `json:"entries"`
}
type PackSnapshotInfo struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Instance  string `json:"instance"`
	PackID    string `json:"packID"`
	Name      string `json:"name"`
	Version   int    `json:"version"`
	Members   int    `json:"members"`
}
type PackHistoryStatus struct {
	Revision    int64              `json:"revision"`
	Entries     []PackSnapshotInfo `json:"entries"`
	Error       string             `json:"error,omitempty"`
	Recoverable bool               `json:"recoverable,omitempty"`
}

func (s *Store) packHistoryProblem() string {
	if s.historyErr != nil {
		return s.historyErr.Error()
	}
	return ""
}
func validPackSnapshot(entry PackSnapshot) bool {
	if !validID.MatchString(entry.ID) || len(entry.Rules) == 0 || Validate(entry.Rules) != nil || entry.Rules[0].Pack == nil {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, entry.CreatedAt); err != nil {
		return false
	}
	instance := entry.Rules[0].Pack.Instance
	for _, r := range entry.Rules {
		if r.Pack == nil || r.Pack.Instance != instance {
			return false
		}
	}
	return true
}
func (s *Store) loadPackHistory() {
	s.history = packHistoryFile{Version: 1, Entries: []PackSnapshot{}}
	f, err := os.Open(filepath.Join(s.dir, "pack-history.json"))
	if os.IsNotExist(err) {
		return
	}
	if err == nil {
		defer f.Close()
		var b []byte
		b, err = io.ReadAll(io.LimitReader(f, maxPackHistoryBytes+1))
		if err == nil && len(b) > maxPackHistoryBytes {
			err = fmt.Errorf("file exceeds limit")
		}
		if err == nil {
			var file packHistoryFile
			d := json.NewDecoder(bytes.NewReader(b))
			d.DisallowUnknownFields()
			err = d.Decode(&file)
			if err == nil && (d.Decode(new(any)) != io.EOF || file.Version != 1 || file.Revision < 0 || len(file.Entries) > 5) {
				err = fmt.Errorf("invalid file")
			}
			ids := map[string]bool{}
			instances := map[string]int{}
			for _, entry := range file.Entries {
				if err != nil {
					break
				}
				if !validPackSnapshot(entry) || ids[entry.ID] {
					err = fmt.Errorf("invalid snapshot")
					break
				}
				ids[entry.ID] = true
				instances[entry.Rules[0].Pack.Instance]++
				if instances[entry.Rules[0].Pack.Instance] > 3 {
					err = fmt.Errorf("too many snapshots")
				}
			}
			if err == nil {
				s.history = file
			}
		}
	}
	if err != nil {
		s.historyErr = fmt.Errorf("Pack backups could not be read: %w", err)
	}
}
func (s *Store) writePackHistory(next packHistoryFile) error {
	if s.historyErr != nil {
		return s.historyErr
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > maxPackHistoryBytes {
		return fmt.Errorf("Pack backups exceed their storage limit")
	}
	path := filepath.Join(s.dir, "pack-history.json")
	old, err := os.ReadFile(path)
	if err == nil {
		if err = atomic(filepath.Join(s.dir, "pack-history.previous.json"), old); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = atomic(path, b); err != nil {
		return err
	}
	s.history = next
	return nil
}
func packMembers(rules []Rule, instance string) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Pack != nil && r.Pack.Instance == instance {
			out = append(out, r)
		}
	}
	return out
}
func copyPackRules(rules []Rule) []Rule {
	b, _ := json.Marshal(rules)
	var result []Rule
	_ = json.Unmarshal(b, &result)
	return result
}

// Capture before replacing rules. Even if that later write fails, this is a
// valid recovery snapshot of the unchanged pack, not proof an update succeeded.
func (s *Store) backupPackUpdatesLocked(rules []Rule, instances []string) error {
	if len(instances) == 0 {
		return nil
	}
	if len(instances) > MaxRules {
		return fmt.Errorf("Too many pack updates")
	}
	next := s.history
	next.Entries = append([]PackSnapshot(nil), s.history.Entries...)
	seen := map[string]bool{}
	for _, instance := range instances {
		if !validID.MatchString(instance) || seen[instance] {
			return fmt.Errorf("Invalid pack update")
		}
		seen[instance] = true
		before, after := packMembers(s.doc.Rules, instance), packMembers(rules, instance)
		if len(before) == 0 || len(after) == 0 || before[0].Pack.ID != after[0].Pack.ID {
			return fmt.Errorf("Pack update target changed")
		}
		oldJSON, _ := json.Marshal(before)
		newJSON, _ := json.Marshal(after)
		if bytes.Equal(oldJSON, newJSON) {
			continue
		}
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		entry := PackSnapshot{ID: hex.EncodeToString(random[:]), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Rules: copyPackRules(before)}
		next.Entries = append([]PackSnapshot{entry}, next.Entries...)
	}
	if len(next.Entries) == len(s.history.Entries) {
		return nil
	}
	count := map[string]int{}
	retained := make([]PackSnapshot, 0, 5)
	for _, entry := range next.Entries {
		instance := entry.Rules[0].Pack.Instance
		if count[instance] >= 3 {
			continue
		}
		retained = append(retained, entry)
		count[instance]++
		if len(retained) == 5 {
			break
		}
	}
	next.Entries = retained
	next.Revision++
	if err := s.writePackHistory(next); err != nil {
		return fmt.Errorf("Could not back up the pack: %w", err)
	}
	return nil
}
func (s *Store) PackHistory() PackHistoryStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := PackHistoryStatus{Revision: s.history.Revision, Entries: []PackSnapshotInfo{}, Error: s.packHistoryProblem(), Recoverable: s.historyErr != nil}
	for _, entry := range s.history.Entries {
		p := entry.Rules[0].Pack
		out.Entries = append(out.Entries, PackSnapshotInfo{entry.ID, entry.CreatedAt, p.Instance, p.ID, p.Name, p.Version, len(entry.Rules)})
	}
	return out
}
func (s *Store) PackSnapshot(id string) (PackSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.historyErr != nil {
		return PackSnapshot{}, s.historyErr
	}
	for _, entry := range s.history.Entries {
		if entry.ID == id {
			entry.Rules = copyPackRules(entry.Rules)
			return entry, nil
		}
	}
	return PackSnapshot{}, os.ErrNotExist
}
func (s *Store) DeletePackSnapshot(revision int64, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.history.Revision {
		return ErrConflict
	}
	next := packHistoryFile{Version: 1, Revision: revision + 1, Entries: []PackSnapshot{}}
	found := false
	for _, entry := range s.history.Entries {
		if entry.ID == id {
			found = true
		} else {
			next.Entries = append(next.Entries, entry)
		}
	}
	if !found {
		return os.ErrNotExist
	}
	return s.writePackHistory(next)
}
func (s *Store) RecoverPackHistory(revision int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.history.Revision {
		return ErrConflict
	}
	if s.historyErr == nil {
		return fmt.Errorf("Pack backups do not need recovery")
	}
	path := filepath.Join(s.dir, "pack-history.json")
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Pack history is not a regular file")
	}
	backup := filepath.Join(s.dir, "pack-history.corrupt-"+time.Now().UTC().Format("20060102-150405.000000000")+".json")
	if err = os.Rename(path, backup); err != nil {
		return err
	}
	next := packHistoryFile{Version: 1, Revision: revision + 1, Entries: []PackSnapshot{}}
	b, _ := json.MarshalIndent(next, "", "  ")
	if err = atomic(path, b); err != nil {
		if restore := os.Rename(backup, path); restore != nil {
			return fmt.Errorf("Backup reset failed: %v; original retained at %s", err, backup)
		}
		return err
	}
	s.history = next
	s.historyErr = nil
	return nil
}
func (s *Store) historyUsesMedia(id string) bool {
	for _, entry := range s.history.Entries {
		for _, used := range MediaReferences(entry.Rules) {
			if used == id {
				return true
			}
		}
	}
	return false
}
