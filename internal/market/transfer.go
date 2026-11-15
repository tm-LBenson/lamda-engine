package market

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

type assembly struct {
	at                            time.Time
	count, size, decoded, version int
	parts                         map[int][]byte
}
type Assembler struct {
	mu       sync.Mutex
	pending  map[string]*assembly
	complete map[string]*completedAssembly
}
type completedAssembly struct {
	at                            time.Time
	count, size, decoded, version int
	parts                         map[int][32]byte
}

func NewAssembler() *Assembler {
	return &Assembler{pending: map[string]*assembly{}, complete: map[string]*completedAssembly{}}
}

// Failed ingestion must remain retryable on the next optical pass.
func (a *Assembler) Retry(body []byte) {
	line, _, ok := bytes.Cut(body, []byte{'\n'})
	if !ok {
		return
	}
	p := strings.Split(string(line), "|")
	if len(p) < 2 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.complete, p[1])
}
func (a *Assembler) Accept(body []byte, now time.Time) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	line, data, ok := bytes.Cut(body, []byte{'\n'})
	if !ok || len(line) > 300 {
		return nil, fmt.Errorf("invalid market chunk header")
	}
	p := strings.Split(string(line), "|")
	if len(p) < 5 || !validID(p[1]) || (p[0] != "1" && p[0] != "2" && p[0] != "3" && p[0] != "4") || ((p[0] == "1" || p[0] == "3") && len(p) != 5) || ((p[0] == "2" || p[0] == "4") && len(p) != 6) {
		return nil, fmt.Errorf("invalid market chunk version")
	}
	version, _ := strconv.Atoi(p[0])
	index, e1 := strconv.Atoi(p[2])
	count, e2 := strconv.Atoi(p[3])
	size, e3 := strconv.Atoi(p[4])
	decoded := size
	compressed := version == 2 || version == 4
	chunk := 5600
	if version >= 3 {
		chunk = 24000
	}
	if compressed {
		var err error
		decoded, err = strconv.Atoi(p[5])
		if err != nil || decoded < 2 || decoded > MaxEventBytes {
			return nil, fmt.Errorf("invalid expanded size")
		}
	}
	if e1 != nil || e2 != nil || e3 != nil || size < 2 || size > MaxEventBytes || count != (size+chunk-1)/chunk || index < 1 || index > count || len(data) != min(chunk, size-(index-1)*chunk) {
		return nil, fmt.Errorf("invalid market chunk bounds")
	}
	for id, v := range a.pending {
		if now.Sub(v.at) > 3*time.Minute {
			delete(a.pending, id)
		}
	}
	for id, v := range a.complete {
		if now.Sub(v.at) > 10*time.Minute {
			delete(a.complete, id)
		}
	}
	if done := a.complete[p[1]]; done != nil {
		if done.count != count || done.size != size || done.decoded != decoded || done.version != version || done.parts[index] != sha256.Sum256(data) {
			return nil, fmt.Errorf("conflicting completed market event")
		}
		return nil, nil
	}
	v := a.pending[p[1]]
	if v == nil {
		if len(a.pending) >= 8 {
			// Lost chunks must not block every newer page until the timeout.
			// The scan stays incomplete and its missing pages can be resent.
			var oldest string
			var stamp time.Time
			for id, pending := range a.pending {
				if oldest == "" || pending.at.Before(stamp) {
					oldest, stamp = id, pending.at
				}
			}
			delete(a.pending, oldest)
		}
		v = &assembly{at: now, count: count, size: size, decoded: decoded, version: version, parts: map[int][]byte{}}
		a.pending[p[1]] = v
	}
	if v.count != count || v.size != size || v.decoded != decoded || v.version != version {
		return nil, fmt.Errorf("conflicting market chunks")
	}
	if old := v.parts[index]; old != nil && !bytes.Equal(old, data) {
		delete(a.pending, p[1])
		return nil, fmt.Errorf("conflicting market chunk content")
	}
	v.parts[index] = append([]byte(nil), data...)
	if len(v.parts) != count {
		return nil, nil
	}
	out := make([]byte, 0, size)
	for i := 1; i <= count; i++ {
		out = append(out, v.parts[i]...)
	}
	delete(a.pending, p[1])
	if compressed {
		input := bytes.NewReader(out)
		reader := flate.NewReader(input)
		expanded, err := io.ReadAll(io.LimitReader(reader, MaxEventBytes+1))
		closeErr := reader.Close()
		if err != nil || closeErr != nil || len(expanded) != decoded || input.Len() != 0 {
			return nil, fmt.Errorf("invalid compressed market event")
		}
		out = expanded
	}
	var event struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(out, &event) != nil || event.ID != p[1] {
		return nil, fmt.Errorf("market chunk identity mismatch")
	}
	if len(a.complete) >= 256 {
		var oldest string
		var stamp time.Time
		for id, done := range a.complete {
			if oldest == "" || done.at.Before(stamp) {
				oldest, stamp = id, done.at
			}
		}
		delete(a.complete, oldest)
	}
	done := &completedAssembly{at: now, count: count, size: size, decoded: decoded, version: version, parts: map[int][32]byte{}}
	for i, part := range v.parts {
		done.parts[i] = sha256.Sum256(part)
	}
	a.complete[p[1]] = done
	return out, nil
}
