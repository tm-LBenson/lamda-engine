package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func appendLog(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func TestTailDetectsRewriteAndSameNameReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WoWCombatLog-a.txt")
	os.WriteFile(path, []byte("previous session\n"), 0600)
	tail := &Tail{}
	tail.Poll(dir)
	generation := tail.Generation
	// A truncate followed by rapid regrowth can already be larger than Offset
	// before the next poll. Checking only current size misses this session change.
	os.WriteFile(path, []byte("new session that is already longer\n"), 0600)
	lines, err := tail.Poll(dir)
	if err != nil || !reflect.DeepEqual(lines, []string{"new session that is already longer"}) || tail.Generation != generation+1 {
		t.Fatal("rewrite not recognized", lines, tail.Generation, err)
	}
	generation = tail.Generation
	os.Rename(path, path+".old")
	os.WriteFile(path, []byte("replacement\n"), 0600)
	lines, err = tail.Poll(dir)
	if err != nil || !reflect.DeepEqual(lines, []string{"replacement"}) || tail.Generation != generation+1 {
		t.Fatal("same-name rotation not recognized", lines, tail.Generation, err)
	}
	generation = tail.Generation
	os.Remove(path)
	tail.Poll(dir)
	if tail.Generation != generation+1 || tail.Path != "" {
		t.Fatal("deleted log did not end session")
	}
}

func TestTailReadsNewLogAfterStartingWithoutOne(t *testing.T) {
	dir := t.TempDir()
	tail := &Tail{}
	tail.Poll(dir)
	path := filepath.Join(dir, "WoWCombatLog-a.txt")
	os.WriteFile(path, []byte("first fresh cast\n"), 0600)
	lines, err := tail.Poll(dir)
	if err != nil || !reflect.DeepEqual(lines, []string{"first fresh cast"}) {
		t.Fatal("fresh log skipped as old history", lines, err)
	}
}

func TestTailSkipsInitialPartialAndOversizedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WoWCombatLog-a.txt")
	os.WriteFile(path, []byte("old unfinished line"), 0600)
	tail := &Tail{}
	tail.Poll(dir)
	appendLog(t, path, " suffix\ngood cast\n")
	lines, err := tail.Poll(dir)
	if err != nil || !reflect.DeepEqual(lines, []string{"good cast"}) {
		t.Fatal("partial history returned", lines, err)
	}
	appendLog(t, path, strings.Repeat("x", (1<<20)+1))
	if lines, err := tail.Poll(dir); err != nil || len(lines) != 0 || len(tail.Pending) != 0 {
		t.Fatal("oversized partial line retained", len(lines), len(tail.Pending), err)
	}
	appendLog(t, path, "misleading suffix\nnext cast\n")
	lines, err = tail.Poll(dir)
	if err != nil || !reflect.DeepEqual(lines, []string{"next cast"}) {
		t.Fatal("oversized line suffix treated as event", lines, err)
	}
}
