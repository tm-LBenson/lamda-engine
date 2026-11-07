package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Owned by the engine loop. Encoding is bounded in memory; disk writes happen
// in batches every ten seconds, never on the screen-capture worker.
type trackingHistory struct {
	path, session  string
	pending        [][]byte
	bytes, dropped int
	sequence       uint64
	fileLimit      int64
}

func newTrackingHistory(dir string) *trackingHistory {
	return &trackingHistory{path: filepath.Join(dir, "tracking-history.jsonl"),
		session: fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid()), fileLimit: 4 << 20}
}

func (h *trackingHistory) Record(data any) {
	h.sequence++
	b, err := json.Marshal(struct {
		Schema     int       `json:"schema"`
		Session    string    `json:"session"`
		Sequence   uint64    `json:"sequence"`
		RecordedAt time.Time `json:"recordedAt"`
		Data       any       `json:"data"`
	}{1, h.session, h.sequence, time.Now(), data})
	if err != nil || len(b) > 128<<10 || len(h.pending) >= 1024 || h.bytes+len(b)+1 > 2<<20 {
		h.dropped++
		return
	}
	b = append(b, '\n')
	h.pending = append(h.pending, b)
	h.bytes += len(b)
}

func (h *trackingHistory) Flush() error {
	for len(h.pending) > 0 {
		f, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		size := info.Size()
		if size > 0 && size+int64(len(h.pending[0])) > h.fileLimit {
			f.Close()
			if err := os.Remove(h.path + ".2"); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(h.path+".1", h.path+".2"); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(h.path, h.path+".1"); err != nil {
				return err
			}
			continue
		}
		for len(h.pending) > 0 && (size == 0 || size+int64(len(h.pending[0])) <= h.fileLimit) {
			b := h.pending[0]
			n, err := f.Write(b)
			if err != nil || n != len(b) {
				// Remove a partial record before retrying the intact queued entry.
				truncateErr := f.Truncate(size)
				f.Close()
				if truncateErr != nil {
					return truncateErr
				}
				if err != nil {
					return err
				}
				return fmt.Errorf("short tracking-history write")
			}
			size += int64(len(b))
			h.bytes -= len(b)
			h.pending[0] = nil
			h.pending = h.pending[1:]
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	if h.dropped > 0 {
		n := h.dropped
		h.dropped = 0
		h.Record(map[string]any{"kind": "history_dropped", "records": n})
		return h.Flush()
	}
	return nil
}
