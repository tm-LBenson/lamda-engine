package augmentation

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func debugStore(t *testing.T) *Store {
	t.Helper()
	s := New(filepath.Join(t.TempDir(), "reports.json"))
	s.SetEnabled(true)
	s.SetDebug(true)
	t.Cleanup(s.Close)
	ingest(s, 0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")
	return s
}
func readDebug(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []map[string]any
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 1<<20)
	for scan.Scan() {
		var event map[string]any
		if err := json.Unmarshal(scan.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestDebugRetainsTrashInputsAndBindsIndependentWindows(t *testing.T) {
	s := debugStore(t)
	damageRecord(s, 1, "SPELL_DAMAGE", aug, "Creature-1", 10, 1, "")
	now := time.Date(2026, 10, 26, 16, 0, 10, 0, time.UTC)
	s.ObserveNativeSlot(windowFrame(1, 0, 1, 10, 0), 0, now, "")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 20, 1, "")
	if err := s.Flush(""); err != nil {
		t.Fatal(err)
	}
	var inputs []map[string]any
	for _, e := range readDebug(t, s.debug.Path) {
		if e["kind"] == "damage_input" {
			inputs = append(inputs, e)
		}
	}
	if len(inputs) != 2 || len(inputs[0]["segments"].([]any)) != 0 || len(inputs[1]["segments"].([]any)) != 1 || inputs[1]["effectiveAmount"] != float64(20) {
		t.Fatal(inputs)
	}
}
func TestDebugLedgerLinksOriginalEventsAndFinalDamage(t *testing.T) {
	s := debugStore(t)
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 10, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 100, 2, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 20, 395152, aug)
	damageRecord(s, 4, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", -1, 410089, aug)
	end(s, 10)
	if err := s.Flush(""); err != nil {
		t.Fatal(err)
	}
	events := readDebug(t, s.debug.Path)
	inputs, final, skipped := 0, 0, 0
	for _, e := range events {
		if e["kind"] == "damage_input" {
			inputs++
			hash := e["normalizedLineSHA256"].(string)
			if len(hash) != 64 {
				t.Fatal(e)
			}
			if e["decision"] == "skipped" {
				skipped++
				if e["reason"] != "invalid_amount_or_overkill" {
					t.Fatal(e)
				}
			}
			if e["effectiveAmount"] == float64(20) && e["supportOwner"] != aug {
				t.Fatal(e)
			}
		}
		if e["kind"] == "meter_output" && e["final"] == true {
			final++
			m := e["meter"].(map[string]any)
			if m["rawTotal"] != float64(110) || m["adjustedTotal"] != float64(110) || e["conserved"] != true {
				t.Fatal(e)
			}
		}
	}
	if inputs != 4 || final != 1 || skipped != 1 {
		t.Fatal(inputs, final, skipped)
	}
	line := "a raw record\r"
	s.debugInput(line)
	h := sha256.Sum256([]byte("a raw record"))
	if s.debug.lineHash != hex.EncodeToString(h[:]) {
		t.Fatal("line endings cannot be matched")
	}
}

func TestDebugBreakdownRecordsTheDisplayedAugCreditAndDeduplicatesIdleReads(t *testing.T) {
	s := debugStore(t)
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 1000, 20, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 200, 395152, aug)
	end(s, 10)
	id := s.damageMeters[0].ID
	for i := 0; i < 2; i++ {
		if _, err := s.DamageBreakdown(aug, id, aug, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Flush(""); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range readDebug(t, s.debug.Path) {
		if event["kind"] != "meter_breakdown" {
			continue
		}
		count++
		view := event["view"].(map[string]any)
		if view["actor"] != aug || view["segment"] != id || view["total"] != float64(300) || view["expected"] != float64(300) || view["complete"] != true {
			t.Fatal(event)
		}
	}
	if count != 1 {
		t.Fatal("unchanged idle breakdown produced duplicate debug records", count)
	}
}
func TestDebugRecordsChangedOutputsWithinOneSecond(t *testing.T) {
	s := debugStore(t)
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 10, 1, "")
	now := time.Now()
	v := *s.DamageView("boss")
	s.debugDisplay(v, now)
	count := s.debug.Records
	s.debugDisplay(v, now.Add(100*time.Millisecond))
	if s.debug.Records != count {
		t.Fatal("unchanged duplicate snapshot")
	}
	damageRecord(s, 3, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	s.debugDisplay(*s.DamageView("boss"), now.Add(200*time.Millisecond))
	if s.debug.Records <= count {
		t.Fatal("changed output missing from ledger")
	}
}
func TestDebugDisabledAndIOErrorsNeverStopMeter(t *testing.T) {
	s := debugStore(t)
	s.SetDebug(false)
	count := s.debug.Records
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	if s.debug.Records != count || s.DamageView("boss").RawTotal != 100 {
		t.Fatal("disabled debug changed tracking")
	}
	bad := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(bad, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	s.debug.Path = filepath.Join(bad, "ledger.jsonl")
	s.SetDebug(true)
	damageRecord(s, 3, "SPELL_DAMAGE", aug, "Creature-1", 50, 1, "")
	if s.Snapshot().Debug.Error == "" || s.DamageView("boss").RawTotal != 150 {
		t.Fatal("logging error stopped meter")
	}
}
func TestDebugRotationIsBoundedAndEachRetainedLineIsJSON(t *testing.T) {
	d := newDebug(filepath.Join(t.TempDir(), "ledger.jsonl"))
	d.Enabled = true
	d.maxBytes = 1024
	d.keep = 3
	for i := 0; i < 30; i++ {
		d.write(map[string]any{"kind": "fixture", "text": strings.Repeat("x", 200)})
	}
	d.close()
	files, err := filepath.Glob(d.Path + "*")
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	for _, p := range files {
		info, err := os.Stat(p)
		if err != nil || info.Size() > d.maxBytes {
			t.Fatal(p, info, err)
		}
		if len(readDebug(t, p)) == 0 {
			t.Fatal("empty rotated file")
		}
	}
}
