package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readHistory(t *testing.T, h *trackingHistory) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, path := range []string{h.path + ".2", h.path + ".1", h.path} {
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			var r map[string]any
			if err := json.Unmarshal(scan.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			records = append(records, r)
		}
		if err := scan.Err(); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	return records
}

func TestTrackingHistoryRetainsAcrossRestartAndSnapshotsMutableState(t *testing.T) {
	dir := t.TempDir()
	h := newTrackingHistory(dir)
	counts := map[string]int{"party": 3}
	h.Record(map[string]any{"kind": "health", "counts": counts})
	counts["party"] = 0
	if _, err := os.Stat(h.path); !os.IsNotExist(err) {
		t.Fatal("wrote on observation path")
	}
	if err := h.Flush(); err != nil {
		t.Fatal(err)
	}
	next := newTrackingHistory(dir)
	next.Record(map[string]any{"kind": "engine_start"})
	if err := next.Flush(); err != nil {
		t.Fatal(err)
	}
	r := readHistory(t, next)
	if len(r) != 2 || r[0]["session"] == r[1]["session"] {
		t.Fatal(r)
	}
	if r[0]["data"].(map[string]any)["counts"].(map[string]any)["party"] != float64(3) {
		t.Fatal(r)
	}
}

func TestTrackingHistoryRotationAndBufferBoundAreExplicit(t *testing.T) {
	h := newTrackingHistory(t.TempDir())
	h.fileLimit = 1024
	for i := 0; i < 1100; i++ {
		h.Record(map[string]any{"kind": "event", "value": i})
	}
	if len(h.pending) > 1024 || h.bytes > 2<<20 || h.dropped == 0 {
		t.Fatal("unbounded buffer", h.dropped)
	}
	if err := h.Flush(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(h.path + "*")
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	for _, path := range files {
		info, _ := os.Stat(path)
		if info.Size() > h.fileLimit {
			t.Fatal("rotation limit", info.Size())
		}
	}
	r := readHistory(t, h)
	if r[len(r)-1]["data"].(map[string]any)["kind"] != "history_dropped" {
		t.Fatal("silent diagnostic loss")
	}
	if len(h.pending) != 0 || h.bytes != 0 || h.dropped != 0 {
		t.Fatal("buffer not drained")
	}
}

func TestTrackingHistoryWriteFailureRetainsPendingBatch(t *testing.T) {
	dir := t.TempDir()
	h := newTrackingHistory(filepath.Join(dir, "missing"))
	h.Record("event")
	if h.Flush() == nil || len(h.pending) != 1 {
		t.Fatal("failed write lost evidence")
	}
	if err := os.Mkdir(filepath.Dir(h.path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := h.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(readHistory(t, h)) != 1 {
		t.Fatal("retry duplicated record")
	}
}
