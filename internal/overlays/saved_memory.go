package overlays

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxMemoryRecords = 4096
const maxMemoryCharacters = 64
const maxMemoryBytes = 16 * 1024 * 1024

type MemoryCharacter struct {
	Character string `json:"character"`
	Realm     string `json:"realm"`
}
type memoryRecord struct {
	MemoryCharacter
	Rule      string `json:"rule"`
	Key       string `json:"key"`
	Value     any    `json:"value"`
	UpdatedAt string `json:"updatedAt"`
}
type memoryFile struct {
	Version int            `json:"version"`
	Records []memoryRecord `json:"records"`
}
type savedMemory struct {
	records    map[string]memoryRecord
	generation uint64
	dirty      bool
	loadErr    error
	writeErr   error
	limitErr   error
}

func newSavedMemory() *savedMemory { return &savedMemory{records: map[string]memoryRecord{}} }
func memoryProfile(character, realm string) string {
	valid := func(v string, max int) bool {
		return v != "" && utf8.ValidString(v) && utf8.RuneCountInString(v) <= max && strings.IndexFunc(v, unicode.IsControl) < 0
	}
	character, realm = strings.TrimSpace(character), strings.TrimSpace(realm)
	if !valid(character, 64) || !valid(realm, 128) {
		return ""
	}
	realm = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, realm)
	if realm == "" {
		return ""
	}
	return strings.ToLower(character) + "\x00" + realm
}
func memoryKey(profile, rule, key string) string { return profile + "\x00" + rule + "\x00" + key }
func (m *savedMemory) changed()                  { m.generation++; m.dirty = true }
func (m *savedMemory) problem() string {
	for _, err := range []error{m.loadErr, m.writeErr, m.limitErr} {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}
func (m *savedMemory) characters() []MemoryCharacter {
	profiles := map[string]MemoryCharacter{}
	for _, r := range m.records {
		profiles[memoryProfile(r.Character, r.Realm)] = r.MemoryCharacter
	}
	keys := make([]string, 0, len(profiles))
	for key := range profiles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]MemoryCharacter, 0, len(keys))
	for _, key := range keys {
		out = append(out, profiles[key])
	}
	return out
}
func (m *savedMemory) bytes() ([]byte, error) {
	keys := make([]string, 0, len(m.records))
	for key := range m.records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	f := memoryFile{Version: 1, Records: make([]memoryRecord, 0, len(keys))}
	for _, key := range keys {
		f.Records = append(f.Records, m.records[key])
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if len(b) > maxMemoryBytes {
		return nil, fmt.Errorf("Saved history exceeds its storage limit")
	}
	return b, err
}
func (v logicValue) scalar() any {
	switch v.kind {
	case "number":
		return v.number
	case "text":
		return v.text
	case "boolean":
		return v.boolean
	}
	return nil
}
func (m *savedMemory) capture(character, realm, rule, key string, v logicValue) bool {
	profile := memoryProfile(character, realm)
	if profile == "" || v.kind == "" || m.loadErr != nil {
		return false
	}
	id := memoryKey(profile, rule, key)
	old, exists := m.records[id]
	if !exists {
		if len(m.records) >= maxMemoryRecords {
			m.limitErr = fmt.Errorf("Saved history is full. Reset unused history to continue")
			return false
		}
		profiles := m.characters()
		found := false
		for _, p := range profiles {
			if memoryProfile(p.Character, p.Realm) == profile {
				found = true
				break
			}
		}
		if !found && len(profiles) >= maxMemoryCharacters {
			m.limitErr = fmt.Errorf("Saved history has reached 64 characters. Reset unused history to continue")
			return false
		}
	}
	if exists && logicLiteral(old.Value) == v {
		return true
	}
	m.records[id] = memoryRecord{MemoryCharacter{strings.TrimSpace(character), strings.TrimSpace(realm)}, rule, key, v.scalar(), time.Now().UTC().Format(time.RFC3339Nano)}
	m.changed()
	return true
}
func savedDefinitions(rules []Rule) map[string]string {
	out := map[string]string{}
	for _, r := range rules {
		if r.Logic == nil {
			continue
		}
		for _, v := range r.Logic.Values {
			if v.Remember != nil && v.Remember.Reset == "saved" {
				out[r.ID+"\x00"+v.Key] = logicLiteral(v.Remember.Initial).kind
			}
		}
	}
	return out
}
func (m *savedMemory) prune(rules []Rule) {
	if m.loadErr != nil {
		return
	}
	definitions := savedDefinitions(rules)
	for id, r := range m.records {
		if definitions[r.Rule+"\x00"+r.Key] != logicLiteral(r.Value).kind {
			delete(m.records, id)
			m.changed()
		}
	}
	m.limitErr = nil
}
func (s *Store) loadMemory() {
	m := newSavedMemory()
	s.runtime.savedMemory = m
	path := filepath.Join(s.dir, "memory.json")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err == nil {
		defer f.Close()
		var b []byte
		b, err = io.ReadAll(io.LimitReader(f, maxMemoryBytes+1))
		if err == nil && len(b) > maxMemoryBytes {
			err = fmt.Errorf("file exceeds limit")
		}
		if err == nil {
			var data memoryFile
			d := json.NewDecoder(bytes.NewReader(b))
			d.DisallowUnknownFields()
			err = d.Decode(&data)
			if err == nil && (d.Decode(new(any)) != io.EOF || data.Version != 1 || len(data.Records) > maxMemoryRecords) {
				err = fmt.Errorf("invalid file")
			}
			for _, r := range data.Records {
				if err != nil {
					break
				}
				profile := memoryProfile(r.Character, r.Realm)
				id := memoryKey(profile, r.Rule, r.Key)
				_, duplicate := m.records[id]
				_, timeErr := time.Parse(time.RFC3339Nano, r.UpdatedAt)
				if profile == "" || !validID.MatchString(r.Rule) || !logicKey.MatchString(r.Key) || logicLiteral(r.Value).kind == "" || duplicate || timeErr != nil {
					err = fmt.Errorf("invalid record")
					break
				}
				m.records[id] = r
			}
			if len(m.characters()) > maxMemoryCharacters {
				err = fmt.Errorf("too many characters")
			}
		}
	}
	if err != nil {
		m.records = map[string]memoryRecord{}
		m.loadErr = fmt.Errorf("Saved history could not be read: %w", err)
		return
	}
	m.prune(s.doc.Rules)
}

// RunMemory is owned by the service context, never started by Open or a sample.
func (s *Store) RunMemory(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	defer s.FlushMemory()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.FlushMemory()
		}
	}
}

// The service calls FlushMemory periodically and at shutdown. Capture handlers
// only change the bounded in-memory table; normal disk I/O never holds s.mu.
func (s *Store) FlushMemory() error {
	s.memoryIO.Lock()
	defer s.memoryIO.Unlock()
	s.mu.Lock()
	m := s.runtime.savedMemory
	if m == nil || !m.dirty || m.loadErr != nil {
		s.mu.Unlock()
		return nil
	}
	generation := m.generation
	b, err := m.bytes()
	s.mu.Unlock()
	if err == nil {
		err = s.writeMemory(b)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m.writeErr = err
	if err == nil && generation == m.generation {
		m.dirty = false
	}
	return err
}
func (s *Store) writeMemory(b []byte) error {
	path := filepath.Join(s.dir, "memory.json")
	old, err := os.ReadFile(path)
	if err == nil {
		err = atomic(filepath.Join(s.dir, "memory.previous.json"), old)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Saved history could not be written: %w", err)
	}
	if err = atomic(path, b); err != nil {
		return fmt.Errorf("Saved history could not be written: %w", err)
	}
	return nil
}

type MemoryStatus struct {
	Revision    int64             `json:"revision"`
	Characters  []MemoryCharacter `json:"characters"`
	Error       string            `json:"error,omitempty"`
	Recoverable bool              `json:"recoverable,omitempty"`
	Value       any               `json:"value"`
	Captured    bool              `json:"captured"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
}

func (s *Store) savedDefinition(rule, key string) *NamedValue {
	for _, r := range s.doc.Rules {
		if r.ID == rule && r.Logic != nil {
			v := r.Logic.named(key)
			if v != nil && v.Remember != nil && v.Remember.Reset == "saved" {
				return v
			}
		}
	}
	return nil
}
func (s *Store) Memory(rule, key, character, realm string) (MemoryStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.savedDefinition(rule, key)
	if v == nil {
		return MemoryStatus{}, fmt.Errorf("Save this remembered value first")
	}
	m := s.runtime.savedMemory
	out := MemoryStatus{Revision: s.doc.Revision, Characters: m.characters(), Error: m.problem(), Recoverable: m.loadErr != nil, Value: v.Remember.Initial}
	if r, ok := m.records[memoryKey(memoryProfile(character, realm), rule, key)]; ok {
		out.Value, out.Captured, out.UpdatedAt = r.Value, true, r.UpdatedAt
	}
	return out, nil
}
func (s *Store) MemoryCharacters() []MemoryCharacter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runtime.savedMemory.characters()
}
func (s *Store) ResetMemory(revision int64, rule, key, character, realm string) error {
	s.memoryIO.Lock()
	defer s.memoryIO.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.doc.Revision {
		return ErrConflict
	}
	if s.savedDefinition(rule, key) == nil {
		return fmt.Errorf("Saved value no longer exists")
	}
	profile := memoryProfile(character, realm)
	if profile == "" {
		return fmt.Errorf("Choose a character")
	}
	m := s.runtime.savedMemory
	if m.loadErr != nil {
		return m.loadErr
	}
	id := memoryKey(profile, rule, key)
	old, exists := m.records[id]
	delete(m.records, id)
	b, err := m.bytes()
	if err == nil {
		err = s.writeMemory(b)
	}
	if err != nil {
		if exists {
			m.records[id] = old
		}
		m.writeErr = err
		return err
	}
	m.changed()
	m.dirty = false
	m.writeErr = nil
	m.limitErr = nil
	if live := s.runtime.memory[rule]; live != nil && live.Profile == profile {
		delete(live.Values, key) // Preserve the edge latch until its condition rearms.
	}
	return nil
}
func (s *Store) RecoverMemory(revision int64) error {
	s.memoryIO.Lock()
	defer s.memoryIO.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.doc.Revision {
		return ErrConflict
	}
	if s.runtime.savedMemory.loadErr == nil {
		return fmt.Errorf("Saved history does not need recovery")
	}
	path := filepath.Join(s.dir, "memory.json")
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Saved history is not a regular file")
	}
	backup := filepath.Join(s.dir, "memory.corrupt-"+time.Now().UTC().Format("20060102-150405.000000000")+".json")
	if err = os.Rename(path, backup); err != nil {
		return err
	}
	m := newSavedMemory()
	b, _ := m.bytes()
	if err = atomic(path, b); err != nil {
		if restore := os.Rename(backup, path); restore != nil {
			return fmt.Errorf("History reset failed: %v; original retained at %s", err, backup)
		}
		return err
	}
	s.runtime.savedMemory = m
	s.runtime.Reset()
	return nil
}
