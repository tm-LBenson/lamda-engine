package augmentation

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func detailRow(t *testing.T, view MeterBreakdown, id string) MeterBreakdownRow {
	t.Helper()
	for _, row := range view.Rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing %s in %+v", id, view)
	return MeterBreakdownRow{}
}

func TestSpellAndTargetDetailsTransferSupportInsteadOfDuplicatingDamage(t *testing.T) {
	s := ready()
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 1000, 20, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 200, 395152, aug)
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 100, 410089, "Player-1-SecondAug")
	damageRecord(s, 4, "SPELL_DAMAGE_SUPPORT", aug, "Creature-1", 50, 395152, aug) // self already in raw
	live := s.DamageView("boss")
	view, err := s.DamageBreakdown(aug, live.ID, other, "")
	if err != nil || !view.Complete || view.Expected != 700 || view.Total != 700 || len(view.Rows) != 1 || view.Rows[0].ID != "raw/20" || view.Rows[0].Removed != 300 {
		t.Fatal(view, err)
	}
	end(s, 10)
	for actor, expected := range map[string]int64{aug: 300, other: 700, "Player-1-SecondAug": 100} {
		view, err := s.DamageBreakdown(aug, live.ID, actor, "")
		if err != nil || !view.Complete || view.Total != expected || view.Expected != expected {
			t.Fatal(actor, view, err)
		}
		for _, spell := range view.Rows {
			targets, err := s.DamageBreakdown(aug, live.ID, actor, spell.ID)
			if err != nil || !targets.Complete || targets.View != "targets" || targets.Total != spell.Amount {
				t.Fatal(targets, err)
			}
		}
	}
	view, _ = s.DamageBreakdown(aug, live.ID, aug, "")
	if len(view.Rows) != 2 || detailRow(t, view, "raw/10").Amount != 100 || detailRow(t, view, "support/"+other+"/395152").Amount != 200 {
		t.Fatal(view)
	}
}

func TestUnmatchedBuffHookDoesNotBecomeAnUnrelatedSpellSubtraction(t *testing.T) {
	s := ready()
	s.own = aug
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 1000, 20, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 200, 395152, aug)
	end(s, 10)
	id := s.damageMeters[0].ID
	view, err := s.DamageBreakdown(aug, id, other, "")
	if err != nil || view.Complete || view.Total != 800 || view.Expected != 800 || view.Reason != "unlinked_support_spell" {
		t.Fatal(view, err)
	}
	if detailRow(t, view, "raw/20").Amount != 1000 || detailRow(t, view, "credit/395152").Amount != -200 {
		t.Fatal(view)
	}
	view, err = s.DamageBreakdown(aug, id, aug, "")
	if err != nil || !view.Complete || view.Total != 200 {
		t.Fatal(view, err)
	}
}

func TestSwingLandedIdentityIsNotAdditionalDamage(t *testing.T) {
	s := ready()
	s.own = aug
	start(s, 1)
	f := actor("SWING_DAMAGE", other, "Creature-1", 1)[:9]
	f[3], f[7] = "0x512", "0xa48"
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, "1000", "1000", "-1", "1", "0", "0", "0", "nil", "nil", "nil")
	ingest(s, 2, f...)
	f[0], f[9] = "SWING_DAMAGE_LANDED", "Creature-1"
	ingest(s, 2, f...)
	hook := actor("SWING_DAMAGE_LANDED_SUPPORT", other, "Creature-1", 395152)
	hook[3], hook[7] = "0x512", "0xa48"
	hook = append(hook, f[9:]...)
	hook[31], hook[32] = "200", "200"
	hook = append(hook, aug)
	ingest(s, 2, hook...)
	end(s, 10)
	view, err := s.DamageBreakdown(aug, s.damageMeters[0].ID, other, "")
	if err != nil || !view.Complete || view.Total != 800 || len(view.Rows) != 1 || view.Rows[0].ID != "raw/1" || s.damageMeters[0].RawTotal != 1000 {
		t.Fatal(view, err, s.damageMeters[0])
	}
}

func TestLatePetDetailsAndHistorySurviveEngineRestart(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-1", "Creature-1", 1000, 20, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", "Pet-1", "Creature-1", 200, 395152, aug)
	end(s, 10)
	s.petOwner("Pet-1", other, "test")
	id := s.damageMeters[0].ID
	if err := s.Flush(""); err != nil {
		t.Fatal(err)
	}
	s.Close()
	loaded := New(s.path)
	defer loaded.Close()
	if len(loaded.pets) != 0 {
		t.Fatal("test must use the retained resolved owner")
	}
	for _, actor := range []string{aug, other} {
		before, err := s.DamageBreakdown(aug, id, actor, "")
		if err != nil {
			t.Fatal(err)
		}
		after, err := loaded.DamageBreakdown(aug, id, actor, "")
		if err != nil || !after.Complete || after.Total != before.Total {
			t.Fatal(before, after, err)
		}
	}
}

func TestMissingTruncatedAndFailedPersistenceStayVisible(t *testing.T) {
	s := ready()
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	end(s, 10)
	id := s.damageMeters[0].ID
	cache := s.meterDetails[aug+"/"+id]
	cache.Truncated = true
	s.meterDetails[aug+"/"+id] = cache
	view, err := s.DamageBreakdown(aug, id, aug, "")
	if err != nil || view.Complete || view.Reason != "incomplete_recorded_details" {
		t.Fatal(view, err)
	}
	if _, err := s.DamageBreakdown(aug, id, aug, "raw/missing"); err == nil {
		t.Fatal("missing spell accepted")
	}
	s.meterDetails = nil
	if _, err := s.DamageBreakdown(aug, id, aug, ""); err == nil {
		t.Fatal("missing history became zero details")
	}
	dir := t.TempDir()
	block := filepath.Join(dir, "file")
	if err := os.WriteFile(block, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(block, "reports.json")
	if err := s.flushMeterDetails(); err == nil || s.Snapshot().MeterDetailsError == "" {
		t.Fatal("save failure hidden", err)
	}
}

func TestConcurrentBreakdownReadPersistenceAndLatePetOwnership(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-1", "Creature-1", 1000, 20, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", "Pet-1", "Creature-1", 200, 395152, aug)
	end(s, 10)
	id := s.damageMeters[0].ID
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = s.DamageBreakdown(aug, id, aug, "")
				_ = s.flushMeterDetails()
			}
		}()
	}
	ingest(s, 11, actor("SPELL_SUMMON", other, "Pet-1", 1)...)
	wg.Wait()
	if err := s.Flush(""); err != nil {
		t.Fatal(err)
	}
	s.Close()
	loaded := New(s.path)
	defer loaded.Close()
	view, err := loaded.DamageBreakdown(aug, id, aug, "")
	if err != nil || !view.Complete || view.Total != 300 {
		t.Fatal(view, err)
	}
}

// Optional private evidence test. No growing live log is altered, and no
// personal log is embedded in the distributable Engine sources.
func TestRecordedKeySpellDetailsConserveEveryActor(t *testing.T) {
	path := os.Getenv("LAMDA_METER_RECORDED_LOG")
	if path == "" {
		t.Skip("private recorded key not provided")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	s := ready()
	scanner := bufio.NewScanner(z)
	scanner.Buffer(make([]byte, 65536), 1<<20)
	for scanner.Scan() {
		s.Ingest(scanner.Text(), time.Now())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	var key *DamageMeter
	for _, m := range s.damageMeters {
		if m.Kind == "key" {
			copy := m
			key = &copy
			break
		}
	}
	if key == nil || key.RawTotal != 333753630 || key.AdjustedTotal != 333753630 {
		t.Fatal(key)
	}
	var out []MeterBreakdown
	for _, row := range key.Rows {
		view, err := s.DamageBreakdown(key.Owner, key.ID, row.GUID, "")
		if err != nil || view.Total != row.Adjusted || view.Expected != row.Adjusted {
			t.Fatal(view, err)
		}
		if row.GUID == key.Owner && (!view.Complete || view.Total != 80301125) {
			t.Fatal("Aug contribution missing from spell detail", view)
		}
		out = append(out, view)
		t.Logf("actor=%s rows=%d total=%d complete=%t reason=%s", row.GUID, len(view.Rows), view.Total, view.Complete, view.Reason)
	}
	if target := os.Getenv("LAMDA_METER_DETAIL_EVIDENCE"); target != "" {
		data, _ := json.MarshalIndent(out, "", "  ")
		if err := os.WriteFile(target, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = io.Copy(io.Discard, z)
}
