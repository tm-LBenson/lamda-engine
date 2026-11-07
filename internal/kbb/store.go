package kbb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrConflict = errors.New("This item changed since you opened it. Your draft is still open; reopen the item to use the current version.")

type Character struct {
	GUID     string            `json:"guid"`
	Name     string            `json:"name"`
	Realm    string            `json:"realm"`
	Records  map[string]Record `json:"records"`
	GameSeen map[string]Stamp  `json:"gameSeen"`
	History  []Record          `json:"history"`
	LastGame int64             `json:"lastGame"`
}
type State struct {
	Schema       int                   `json:"schema"`
	Client       string                `json:"client"`
	Characters   map[string]*Character `json:"characters"`
	PublishError string                `json:"publishError,omitempty"`
	Published    bool                  `json:"published"`
}
type Store struct {
	publication  []byte
	mu           sync.Mutex
	state        State
	path         string
	published    bool
	publishError string
}
type Change struct {
	Key     string          `json:"key"`
	Base    *Stamp          `json:"base"`
	Data    json.RawMessage `json:"data,omitempty"`
	Deleted bool            `json:"deleted,omitempty"`
}
type Edit struct {
	GUID    string   `json:"guid"`
	Changes []Change `json:"changes"`
}

func Open(dir, client string) (*Store, error) {
	if !one(client, "retail", "forever") {
		return nil, fmt.Errorf("invalid planner client")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "planner.json"), state: State{Schema: 1, Client: client, Characters: map[string]*Character{}}}
	b, err := readFile(s.path, MaxFile)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = Decode(b, &s.state); err != nil {
		return nil, fmt.Errorf("saved planner cannot be read: %w", err)
	}
	if s.state.Schema != 1 || s.state.Client != client || s.state.Characters == nil || len(s.state.Characters) > 100 {
		return nil, fmt.Errorf("saved planner identity is invalid")
	}
	for guid, c := range s.state.Characters {
		if c == nil || guid != c.GUID || c.Records == nil || c.GameSeen == nil || len(c.History) > 200 {
			return nil, fmt.Errorf("saved planner records are invalid")
		}
		snap := Snapshot{1, client, guid, c.Name, c.Realm, records(c)}
		if err = snap.Validate(client, time.Unix(4102444800, 0)); err != nil {
			return nil, err
		}
		for key, r := range c.Records {
			if key != r.Key {
				return nil, fmt.Errorf("saved planner record key mismatch")
			}
		}
		for i := range c.History {
			if err = c.History[i].Validate(); err != nil {
				return nil, err
			}
		}
	}
	s.state.PublishError = ""
	s.state.Published = false
	return s, nil
}
func readFile(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, fmt.Errorf("planner file exceeds its limit")
	}
	b := make([]byte, info.Size())
	_, err = io.ReadFull(f, b)
	return b, err
}
func atomicWrite(path, pattern string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := clone(s.state)
	out.Published = s.published
	out.PublishError = s.publishError
	return out
}

// CharacterNames supplies local pickers without copying board contents/history.
func (s *Store) CharacterNames() [][2]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][2]string, 0, len(s.state.Characters))
	for _, c := range s.state.Characters {
		out = append(out, [2]string{c.Name, c.Realm})
	}
	return out
}
func records(c *Character) []Record {
	out := make([]Record, 0, len(c.Records))
	for _, r := range c.Records {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
func equal(a, b Record) bool { return a.Deleted == b.Deleted && bytes.Equal(a.Data, b.Data) }
func remember(c *Character, r Record) bool {
	for _, h := range c.History {
		if h.Key == r.Key && h.Stamp == r.Stamp && equal(h, r) {
			return false
		}
	}
	c.History = append([]Record{r}, c.History...)
	if len(c.History) > 200 {
		c.History = c.History[:200]
	}
	return true
}
func (s *Store) commit(next State) error {
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if len(b) > MaxFile {
		return fmt.Errorf("planner storage is full; export history before adding more data")
	}
	if err = atomicWrite(s.path, ".planner-*", b); err != nil {
		return err
	}
	s.state = next
	s.publication = nil
	s.published = false
	return nil
}

// Merge only applies explicit records, so partial optical batches never delete
// unrelated cards. Losing edits are retained in bounded recovery history.
func (s *Store) Merge(snap Snapshot, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := snap.Validate(s.state.Client, now); err != nil {
		return err
	}
	next := clone(s.state)
	c := next.Characters[snap.GUID]
	if c == nil {
		if len(next.Characters) >= 100 {
			return fmt.Errorf("planner character limit reached")
		}
		c = &Character{GUID: snap.GUID, Records: map[string]Record{}, GameSeen: map[string]Stamp{}, History: []Record{}}
		next.Characters[snap.GUID] = c
	}
	changed := c.Name != snap.Name || c.Realm != snap.Realm
	c.Name, c.Realm = snap.Name, snap.Realm
	for _, incoming := range snap.Records {
		old, exists := c.Records[incoming.Key]
		cmp := incoming.Stamp.Compare(old.Stamp)
		if exists && cmp == 0 && !equal(incoming, old) {
			return fmt.Errorf("conflicting planner content with the same timestamp")
		}
		if !exists || cmp > 0 {
			if exists && !equal(old, incoming) {
				remember(c, old)
			}
			c.Records[incoming.Key] = incoming
			changed = true
		} else if cmp < 0 && !equal(old, incoming) {
			changed = remember(c, incoming) || changed
		}
		if seen, ok := c.GameSeen[incoming.Key]; !ok || incoming.Stamp.Compare(seen) > 0 {
			c.GameSeen[incoming.Key] = incoming.Stamp
			changed = true
		}
	}
	if len(c.Records) > MaxRecords {
		return fmt.Errorf("planner record limit reached")
	}
	// A minute-level receipt avoids rewriting the disk for every repeated pass.
	if now.Unix()-c.LastGame >= 60 {
		c.LastGame = now.Unix()
		changed = true
	}
	if !changed {
		return nil
	}
	return s.commit(next)
}
func nextStamp(c *Character, now time.Time) Stamp {
	v := Stamp{At: now.Unix() * 1000, Actor: "desktop"}
	for _, r := range c.Records {
		if r.Stamp.At > v.At {
			v.At = r.Stamp.At
			v.Seq = r.Stamp.Seq + 1
		} else if r.Stamp.At == v.At && r.Stamp.Seq >= v.Seq {
			v.Seq = r.Stamp.Seq + 1
		}
	}
	return v
}
func validateCollection(c *Character) error {
	boards := map[string]Board{}
	counts := map[string]int{}
	for key, r := range c.Records {
		if !r.Deleted && strings.HasPrefix(key, "b/") {
			var b Board
			_ = json.Unmarshal(r.Data, &b)
			boards[b.ID] = b
		}
	}
	if len(boards) < 1 || len(boards) > 20 {
		return fmt.Errorf("keep between 1 and 20 boards")
	}
	for key, r := range c.Records {
		if r.Deleted || !strings.HasPrefix(key, "c/") {
			continue
		}
		p := strings.Split(key, "/")
		b, ok := boards[p[1]]
		if !ok {
			return fmt.Errorf("card belongs to a deleted board")
		}
		var card Card
		_ = json.Unmarshal(r.Data, &card)
		found := false
		for _, col := range b.Columns {
			if col.ID == card.Column {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("move cards before removing their column")
		}
		counts[b.ID]++
		if counts[b.ID] > 1000 {
			return fmt.Errorf("a board can contain up to 1,000 cards")
		}
	}
	return nil
}
func (s *Store) Edit(edit Edit, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !guidPattern.MatchString(edit.GUID) || len(edit.Changes) == 0 || len(edit.Changes) > 1001 {
		return fmt.Errorf("invalid planner edit")
	}
	next := clone(s.state)
	c := next.Characters[edit.GUID]
	if c == nil {
		return fmt.Errorf("load this character from WoW first")
	}
	stamp := nextStamp(c, now)
	seen := map[string]bool{}
	for _, change := range edit.Changes {
		if seen[change.Key] {
			return fmt.Errorf("duplicate edit")
		}
		seen[change.Key] = true
		old, exists := c.Records[change.Key]
		if (exists && (change.Base == nil || *change.Base != old.Stamp)) || (!exists && change.Base != nil) {
			return ErrConflict
		}
		r := Record{Key: change.Key, Stamp: stamp, Deleted: change.Deleted, Data: change.Data}
		if err := r.Validate(); err != nil {
			return err
		}
		if exists && equal(old, r) {
			continue
		}
		if exists {
			remember(c, old)
		}
		c.Records[r.Key] = r
	}
	if len(c.Records) > MaxRecords {
		return fmt.Errorf("planner record limit reached")
	}
	if err := validateCollection(c); err != nil {
		return err
	}
	return s.commit(next)
}
