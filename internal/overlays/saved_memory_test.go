package overlays

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func savedRule() Rule { r := memoryRule(); r.Logic.Values[0].Remember.Reset = "saved"; return r }
func savedStore(t *testing.T) (*Store, Rule) {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := savedRule()
	if err = s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	return s, r
}
func historyValue(t *testing.T, s *Store, r Rule, name, realm string, want float64, captured bool) {
	t.Helper()
	got, err := s.Memory(r.ID, "count", name, realm)
	if err != nil || logicLiteral(got.Value).number != want || got.Captured != captured {
		t.Fatalf("history %s/%s: %#v %v, want %g/%v", name, realm, got, err, want, captured)
	}
}
func captureHistory(s *Store, r Rule, name, realm string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime.rememberEvent(r, "ability", &lookup.OverlayInput{Character: name, Realm: realm})
}
func TestSavedMemoryRestartCharacterAndSessionIsolation(t *testing.T) {
	s, rule := savedStore(t)
	now := time.Unix(2000000000, 0)
	seq := 0
	accept := func(name, realm string, cast int, epoch string, spec int, expected string) {
		t.Helper()
		seq++
		at := now.Add(time.Duration(seq) * 100 * time.Millisecond)
		v := resourceSample([]Rule{rule}, at, seq, 30)
		v.Overlays.Character, v.Overlays.Realm = name, realm
		v.Overlays.CastSeq = cast
		v.Overlays.Epoch = epoch
		v.Overlays.Spec = spec
		s.Accept(v, at)
		out := s.Merge(engine.FrameDisplay{}, at).Overlays
		if len(out) != 1 || out[0].Text != expected {
			t.Fatalf("step %d: %#v want %s", seq, out, expected)
		}
	}
	accept("Alice", "Wyrmrest Accord", 1, "first", 1468, "Count 0")
	accept("Alice", "Wyrmrest Accord", 2, "first", 1468, "Count 1")
	accept("Alice", "Wyrmrest Accord", 3, "first", 1468, "Count 2")
	if _, err := os.Stat(filepath.Join(s.dir, "memory.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("capture performed disk I/O", err)
	}
	accept("Bob", "Wyrmrest Accord", 3, "first", 1468, "Count 0")
	accept("Bob", "Wyrmrest Accord", 4, "first", 1468, "Count 1")
	accept("Alice", "Different Realm", 1, "other", 1468, "Count 0")
	accept("Alice", "WyrmrestAccord", 3, "reload", 1468, "Count 2")
	accept("Alice", "WyrmrestAccord", 3, "reload", 1467, "Count 2")
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	accept("Alice", "WyrmrestAccord", 3, "reload", 1467, "Count 2")
	if err := s.FlushMemory(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, "memory.json")
	before, _ := os.Stat(path)
	if err := s.FlushMemory(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("clean flush rewrote history")
	}
	var err error
	s, err = Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	accept("Alice", "Wyrmrest Accord", 8, "restart", 1468, "Count 2")
	accept("Alice", "Wyrmrest Accord", 9, "restart", 1468, "Count 3")
	historyValue(t, s, rule, "Bob", "WyrmrestAccord", 1, true)
	if err = s.FlushMemory(); err != nil {
		t.Fatal(err)
	}
	previous, _ := os.ReadFile(filepath.Join(s.dir, "memory.previous.json"))
	if !bytes.Contains(previous, []byte(`"value": 2`)) {
		t.Fatal("previous snapshot missing")
	}
}

func TestSavedMemoryResetLatchEditsAndUnknown(t *testing.T) {
	s, rule := savedStore(t)
	v := &rule.Logic.Values[0]
	v.Remember.When = map[string]any{"event": "state"}
	v.Remember.Check = map[string]any{"condition": "health", "operator": "lt", "value": 20.0}
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	p := &lookup.OverlayInput{Character: "Alice", Realm: "Moon Guard", Health: ptr(10.0)}
	s.runtime.rememberEvent(rule, "state", p)
	historyValue(t, s, rule, "Alice", "MoonGuard", 1, true)
	if err := s.ResetMemory(0, rule.ID, "count", "Alice", "Moon Guard"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale reset", err)
	}
	if err := s.ResetMemory(s.doc.Revision, rule.ID, "count", "Alice", "Moon Guard"); err != nil {
		t.Fatal(err)
	}
	s.runtime.rememberEvent(rule, "state", p)
	historyValue(t, s, rule, "Alice", "MoonGuard", 0, false)
	p.Health = ptr(30.0)
	s.runtime.rememberEvent(rule, "state", p)
	p.Health = ptr(10.0)
	s.runtime.rememberEvent(rule, "state", p)
	historyValue(t, s, rule, "Alice", "MoonGuard", 1, true)
	v.Expr = calc("div", val(1), val(0))
	p.Health = nil
	s.runtime.rememberEvent(rule, "state", p)
	p.Health = ptr(10.0)
	s.runtime.rememberEvent(rule, "state", p)
	historyValue(t, s, rule, "Alice", "MoonGuard", 1, true)
	// Compatible initial edits, disabled rules and unused definitions keep history.
	v.Remember.Initial = 9.0
	rule.Enabled = false
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	historyValue(t, s, rule, "Alice", "MoonGuard", 1, true)
	v.Remember.Initial = "new"
	v.Expr = val("updated")
	if err := s.Save(s.doc.Revision, []Rule{rule}); err != nil {
		t.Fatal(err)
	}
	state, _ := s.Memory(rule.ID, "count", "Alice", "MoonGuard")
	if state.Value != "new" || state.Captured {
		t.Fatal("type change restored incompatible value", state)
	}
	if err := s.Save(s.doc.Revision, nil); err != nil {
		t.Fatal(err)
	}
	if len(s.runtime.savedMemory.records) != 0 {
		t.Fatal("deleted rules retained history")
	}
}

func TestSavedMemoryFailedWritesAndRecovery(t *testing.T) {
	s, rule := savedStore(t)
	captureHistory(s, rule, "Alice", "Realm")
	if err := s.FlushMemory(); err != nil {
		t.Fatal(err)
	}
	captureHistory(s, rule, "Alice", "Realm")
	blocker := filepath.Join(s.dir, "memory.previous.json")
	if err := os.Mkdir(blocker, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.FlushMemory(); err == nil || !s.runtime.savedMemory.dirty || s.runtime.savedMemory.problem() == "" {
		t.Fatal("write failure lost dirty history", err)
	}
	if err := s.ResetMemory(s.doc.Revision, rule.ID, "count", "Alice", "Realm"); err == nil {
		t.Fatal("failed reset reported success")
	}
	historyValue(t, s, rule, "Alice", "Realm", 2, true)
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if err := s.FlushMemory(); err != nil {
		t.Fatal(err)
	}
	if s.runtime.savedMemory.problem() != "" {
		t.Fatal("retry did not clear error")
	}
	path := filepath.Join(s.dir, "memory.json")
	corrupt := []byte("{ broken history")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(s.dir)
	if err != nil {
		t.Fatal("corrupt history blocked all overlays", err)
	}
	state, _ := s.Memory(rule.ID, "count", "Alice", "Realm")
	if !state.Recoverable || state.Error == "" {
		t.Fatal("corruption hidden", state)
	}
	captureHistory(s, rule, "Alice", "Realm")
	_ = s.FlushMemory()
	b, _ := os.ReadFile(path)
	if !bytes.Equal(b, corrupt) {
		t.Fatal("corrupt file overwritten without recovery")
	}
	if err = s.RecoverMemory(s.doc.Revision - 1); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = s.RecoverMemory(s.doc.Revision); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(s.dir, "memory.corrupt-*.json"))
	if len(backups) != 1 {
		t.Fatal("corrupt backup missing", backups)
	}
	b, _ = os.ReadFile(backups[0])
	if !bytes.Equal(b, corrupt) {
		t.Fatal("wrong recovery backup")
	}
	captureHistory(s, rule, "Alice", "Realm")
	historyValue(t, s, rule, "Alice", "Realm", 1, true)
}

func TestSavedMemoryStrictFilesAndBounds(t *testing.T) {
	base := memoryRecord{MemoryCharacter{"Alice", "Realm"}, "rule", "count", 3.0, time.Now().UTC().Format(time.RFC3339Nano)}
	for _, change := range []func(*memoryFile){
		func(f *memoryFile) { f.Version = 9 }, func(f *memoryFile) { f.Records = append(f.Records, base) },
		func(f *memoryFile) { f.Records[0].Value = map[string]any{"bad": 1} }, func(f *memoryFile) { f.Records[0].Realm = "" },
		func(f *memoryFile) { f.Records[0].Key = "../escape" }, func(f *memoryFile) { f.Records[0].UpdatedAt = "bad" },
	} {
		dir := t.TempDir()
		f := memoryFile{1, []memoryRecord{base}}
		change(&f)
		b, _ := json.Marshal(f)
		_ = os.WriteFile(filepath.Join(dir, "memory.json"), b, 0600)
		s, err := Open(dir)
		if err != nil || s.runtime.savedMemory.loadErr == nil {
			t.Fatal("invalid file accepted", string(b), err)
		}
	}
	m := newSavedMemory()
	for i := 0; i < maxMemoryCharacters; i++ {
		if !m.capture(fmt.Sprint("Char", i), "Realm", "r", "count", logicNumber(1)) {
			t.Fatal(i)
		}
	}
	if m.capture("Overflow", "Realm", "r", "count", logicNumber(1)) || len(m.records) != maxMemoryCharacters {
		t.Fatal("character bound evicted history")
	}
	if !m.capture("Char0", "Realm", "r", "count", logicNumber(2)) {
		t.Fatal("full storage blocked existing record")
	}
	m = newSavedMemory()
	for i := 0; i < maxMemoryRecords; i++ {
		m.records[fmt.Sprint(i)] = base
	}
	if m.capture("New", "Realm", "r", "count", logicNumber(1)) || len(m.records) != maxMemoryRecords {
		t.Fatal("record bound failed")
	}
}

func TestSavedMemoryConcurrentFlushAndReset(t *testing.T) {
	s, rule := savedStore(t)
	var wg sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				switch n {
				case 0:
					captureHistory(s, rule, "Alice", "Realm")
				case 1:
					if err := s.FlushMemory(); err != nil {
						t.Error(err)
					}
				case 2:
					_, _ = s.Memory(rule.ID, "count", "Alice", "Realm")
					if i%10 == 0 {
						if err := s.ResetMemory(1, rule.ID, "count", "Alice", "Realm"); err != nil {
							t.Error(err)
						}
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	if err := s.FlushMemory(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.dir)
	if err != nil || reopened.runtime.savedMemory.loadErr != nil {
		t.Fatal("racing writes corrupted file", err)
	}
	got, _ := s.Memory(rule.ID, "count", "Alice", "Realm")
	other, _ := reopened.Memory(rule.ID, "count", "Alice", "Realm")
	if logicLiteral(got.Value) != logicLiteral(other.Value) || got.Captured != other.Captured {
		t.Fatal("last flush lost update", got, other)
	}
}

func TestSavedMemoryPeriodicFlushAndShutdown(t *testing.T) {
	s, rule := savedStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.RunMemory(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("history worker did not stop")
		}
	})
	captureHistory(s, rule, "Alice", "Realm")
	// Check eventual persistence, not host scheduling or disk latency under load.
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(s.dir, "memory.json")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			state, _ := s.Memory(rule.ID, "count", "Alice", "Realm")
			t.Fatal("periodic flush did not write", state.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	captureHistory(s, rule, "Alice", "Realm")
	cancel()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("flush loop did not stop")
	}
	reopened, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	historyValue(t, reopened, rule, "Alice", "Realm", 2, true)
}
