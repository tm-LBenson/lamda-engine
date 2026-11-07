package augmentation

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Debug files are local diagnostics, never addon payloads. Writes are buffered
// and bounded. Logging failures are visible but cannot stop damage tracking.
type DebugStatus struct {
	Enabled    bool   `json:"enabled"`
	Path       string `json:"path"`
	Session    string `json:"session"`
	Records    uint64 `json:"records"`
	Error      string `json:"error,omitempty"`
	SourceFile string `json:"sourceFile,omitempty"`
}
type debugRecorder struct {
	DebugStatus
	file           *os.File
	buffer         *bufio.Writer
	size           int64
	maxBytes       int64
	keep           int
	lineHash       string
	lastOutput     map[string]time.Time
	lastOutputHash map[string][32]byte
}

func newDebug(path string) *debugRecorder {
	return &debugRecorder{DebugStatus: DebugStatus{Path: path, Session: fmt.Sprintf("%s-%d", time.Now().UTC().Format("20060102T150405.000000000Z"), os.Getpid())},
		maxBytes: 16 << 20, keep: 16, lastOutput: map[string]time.Time{}, lastOutputHash: map[string][32]byte{}}
}
func (d *debugRecorder) open() error {
	if d.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(d.Path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(d.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	d.file = f
	d.size = info.Size()
	d.buffer = bufio.NewWriterSize(f, 256<<10)
	return nil
}
func (d *debugRecorder) flush() {
	if d.buffer != nil {
		if err := d.buffer.Flush(); err != nil {
			d.Error = err.Error()
		}
	}
}
func (d *debugRecorder) close() {
	d.flush()
	if d.file != nil {
		if err := d.file.Close(); err != nil {
			d.Error = err.Error()
		}
	}
	d.file = nil
	d.buffer = nil
}
func (d *debugRecorder) rotate() error {
	d.close()
	for i := d.keep - 1; i >= 1; i-- {
		from := d.Path
		if i > 1 {
			from = fmt.Sprintf("%s.%d", d.Path, i-1)
		}
		to := fmt.Sprintf("%s.%d", d.Path, i)
		if i == d.keep-1 {
			if err := os.Remove(to); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return d.open()
}
func (d *debugRecorder) write(event map[string]any) {
	if d == nil || !d.Enabled || d.Path == "" {
		return
	}
	event["schema"] = 1
	event["debugSession"] = d.Session
	event["sequence"] = d.Records + 1
	event["writtenAt"] = time.Now().UTC()
	if d.SourceFile != "" {
		event["sourceFile"] = d.SourceFile
	}
	data, err := json.Marshal(event)
	if err != nil {
		d.Error = err.Error()
		return
	}
	data = append(data, '\n')
	if err = d.open(); err != nil {
		d.Error = err.Error()
		return
	}
	if d.size+int64(len(data)) > d.maxBytes {
		if err = d.rotate(); err != nil {
			d.Error = err.Error()
			return
		}
	}
	if _, err = d.buffer.Write(data); err != nil {
		d.Error = err.Error()
		return
	}
	d.size += int64(len(data))
	d.Records++
}
func (s *Store) SetDebug(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debug == nil || s.debug.Enabled == enabled {
		return
	}
	if enabled {
		s.debug.Enabled = true
		s.debug.Error = ""
		s.debug.lastOutput = map[string]time.Time{}
		s.debug.lastOutputHash = map[string][32]byte{}
		s.debug.write(map[string]any{"kind": "debug_enabled", "maxBytesPerFile": s.debug.maxBytes, "retainedFiles": s.debug.keep})
	} else {
		s.debug.write(map[string]any{"kind": "debug_disabled"})
		s.debug.close()
		s.debug.Enabled = false
	}
}
func (s *Store) debugInput(line string) {
	if s.debug == nil || !s.debug.Enabled {
		return
	}
	digest := sha256.Sum256([]byte(strings.TrimSuffix(line, "\r")))
	s.debug.lineHash = hex.EncodeToString(digest[:])
}
func (s *Store) debugDecision(decision, reason string, f []string, amount int64, support bool) {
	if s.debug == nil || !s.debug.Enabled {
		return
	}
	segments := []string{}
	for _, seg := range s.active {
		segments = append(segments, fmt.Sprintf("%s/%s/%d", seg.kind, seg.id, seg.start.UnixMilli()))
	}
	seenNative := map[string]bool{}
	addNative := func(h *hybridState) {
		if h == nil || h.frame == nil || h.frame.Metric > 1 {
			return
		}
		end := h.start.Add(time.Duration(h.frame.DurationMillis) * time.Millisecond)
		if !s.last.Before(h.start.Add(-300*time.Millisecond)) && !s.last.After(end.Add(200*time.Millisecond)) {
			id := fmt.Sprintf("native/%s/%d", h.frame.Player, h.frame.SessionID)
			if !seenNative[id] {
				segments = append(segments, id)
				seenNative[id] = true
			}
		}
	}
	addNative(s.hybrid)
	for _, h := range s.nativeWindows {
		addNative(h)
	}
	// Keep unscoped friendly inputs too: first hits can precede the first
	// captured frame, and ordinary trash/dummies have no boss/key segment.
	// The original timestamp/hash still binds them to the comparison log.
	sort.Strings(segments)
	e := map[string]any{"kind": "damage_input", "eventAt": s.last, "receivedAt": s.received,
		"normalizedLineSHA256": s.debug.lineHash, "decision": decision, "reason": reason, "segments": segments,
		"event": f[0], "sourceGUID": f[1], "targetGUID": f[5], "resolvedSourceOwner": s.sourceOwner(f[1]), "selectedAugGUID": s.own,
		"effectiveAmount": amount, "isSupport": support}
	base, _, ok := damagePayload(f)
	if ok && len(f) > base+21 {
		e["loggedAmount"] = f[base+19]
		e["loggedOverkill"] = f[base+21]
		if len(f) > base+25 {
			e["loggedAbsorbed"] = f[base+25]
		}
	} else if ok && strings.HasSuffix(f[0], "_MISSED") && len(f) > base+2 {
		e["loggedMissType"] = f[base]
		e["loggedAbsorbed"] = f[base+2]
	}
	if base == 12 && len(f) > 11 {
		e["spellID"] = f[9]
		e["spellName"] = f[10]
	}
	if support && len(f) == base+30 {
		e["supportOwner"] = f[base+29]
	}
	s.debug.write(e)
}
func (s *Store) debugOwnership(pet, owner, method string) {
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "pet_owner", "petGUID": pet, "ownerGUID": owner, "method": method,
			"eventAt": s.last, "receivedAt": s.received, "normalizedLineSHA256": s.debug.lineHash})
	}
}
func (s *Store) debugMeter(view DamageMeter, target string, now time.Time, force bool) {
	if s.debug == nil || !s.debug.Enabled {
		return
	}
	data, _ := json.Marshal(view)
	fingerprint := sha256.Sum256(data)
	if !force && fingerprint == s.debug.lastOutputHash[target] && now.Sub(s.debug.lastOutput[target]) < time.Second {
		return
	}
	s.debug.lastOutput[target] = now
	s.debug.lastOutputHash[target] = fingerprint
	s.debug.write(map[string]any{"kind": "meter_output", "target": target, "receivedAt": s.received, "eventAt": s.last, "meter": view,
		"conserved": view.RawTotal == view.AdjustedTotal, "final": !view.Live && !view.Provisional && view.Valid && (view.InputSource != "native" || view.CollectionValid)})
}
func (s *Store) debugDisplay(view DamageMeter, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.debugMeter(view, "overlay", now, false)
}
func (s *Store) debugBreakdown(view MeterBreakdown) {
	if s.debug == nil || !s.debug.Enabled {
		return
	}
	data, _ := json.Marshal(view)
	fingerprint := sha256.Sum256(data)
	if fingerprint == s.debug.lastOutputHash["breakdown"] && time.Since(s.debug.lastOutput["breakdown"]) < time.Second {
		return
	}
	s.debug.lastOutputHash["breakdown"] = fingerprint
	s.debug.lastOutput["breakdown"] = time.Now()
	s.debug.write(map[string]any{"kind": "meter_breakdown", "eventAt": s.last, "receivedAt": s.received, "view": view})
}
func (s *Store) Close() {
	_ = s.flushMeterDetails()
	_ = s.flushReportHistory()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "debug_shutdown"})
		s.debug.close()
	}
}
