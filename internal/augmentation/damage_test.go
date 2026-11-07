package augmentation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestPetOwnershipAfterBossEndRepairsCompletedSnapshotAndSurvivesRestart(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	pet := "Creature-1-LateGuardian"
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 50, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", pet, "Creature-1", 100, 2, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", pet, "Creature-1", 20, 395152, aug)
	end(s, 10)
	old := s.Snapshot()
	v := s.DamageView("boss")
	if v.Valid || v.UnassignedPetDamage != 100 || v.PendingSupportOwners[aug][pet] != 20 {
		t.Fatal(v)
	}
	path := filepath.Join(t.TempDir(), "reports.json")
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded := New(path)
	defer loaded.Close()
	for _, store := range []*Store{s, loaded} {
		store.petOwner(pet, other, "advanced_unit_data")
		store.petOwner(pet, other, "advanced_unit_data") // same fact cannot double-credit
		v := store.damageMeters[0]
		if !v.Valid || v.Partial || v.UnassignedPetDamage != 0 || v.RawTotal != 150 || v.AdjustedTotal != 150 {
			t.Fatal(v)
		}
		if findRow(t, &v, aug).Adjusted != 70 || findRow(t, &v, other).Adjusted != 80 {
			t.Fatal(v.Rows)
		}
	}
	if old.DamageMeters[0].PendingRaw[pet] != 100 || old.DamageMeters[0].UnassignedPetDamage != 100 {
		t.Fatal("changed a previously returned snapshot")
	}
}

func damageRecord(s *Store, second int, event, source, target string, amount, id int, credit string) {
	f := actor(event, source, target, id)
	f[7] = "0xa48"
	if source == aug {
		f[3] = "0x511"
	} else {
		f[3] = "0x512"
	}
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, intString(amount), intString(amount), "-1", "12", "0", "0", "0", "nil", "nil", "nil")
	if credit != "" {
		f = append(f, credit)
	} else {
		f = append(f, "ST")
	}
	ingest(s, second, f...)
}
func intString(n int) string { return strconv.Itoa(n) }
func findRow(t *testing.T, v *DamageMeter, guid string) DamageRow {
	t.Helper()
	if v == nil {
		t.Fatal("missing meter")
	}
	for _, r := range v.Rows {
		if r.GUID == guid {
			return r
		}
	}
	t.Fatal("missing row", guid)
	return DamageRow{}
}

func TestDamageRedistributionConservesTotalAndCreditsEverySupportingAug(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 1200, 2, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 200, 395152, aug)
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 300, 410089, "Player-1-ForeignAug")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", aug, "Creature-1", 50, 413984, aug) // self already included
	v := s.DamageView("boss")
	own, ally := findRow(t, v, aug), findRow(t, v, other)
	if own.Raw != 100 || own.Added != 200 || own.Adjusted != 300 || ally.Adjusted != 700 || ally.Removed != 500 || findRow(t, v, "Player-1-ForeignAug").Added != 300 {
		t.Fatal(own, ally)
	}
	if !v.Valid || v.RawTotal != 1300 || v.AdjustedTotal != 1300 {
		t.Fatal(v)
	}
}

func TestPetDamageAndSupportStayWithExactOwnerAfterLateOwnership(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-1", "Creature-1", 1000, 2, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", "Pet-1", "Creature-1", 200, 395152, aug)
	if v := s.DamageView("boss"); v.Valid || v.UnassignedPetDamage != 1000 {
		t.Fatal(v)
	}
	ingest(s, 4, actor("SPELL_SUMMON", other, "Pet-1", 5)...)
	v := s.DamageView("boss")
	if !v.Valid || findRow(t, v, other).Adjusted != 800 || findRow(t, v, aug).Adjusted != 300 {
		t.Fatal(v)
	}
}

func TestSwingLandedSupportDoesNotDuplicateOrdinaryMelee(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 10, 1, "")
	f := actor("SWING_DAMAGE", other, "Creature-1", 0)[:9]
	f[7] = "0xa48"
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, "1000", "1000", "-1", "1", "0", "0", "0", "nil", "nil", "nil")
	ingest(s, 3, f...)
	f[0] = "SWING_DAMAGE_LANDED"
	ingest(s, 3, f...)
	damageRecord(s, 3, "SWING_DAMAGE_LANDED_SUPPORT", other, "Creature-1", 100, 395152, aug)
	v := s.DamageView("boss")
	if findRow(t, v, other).Raw != 1000 || findRow(t, v, other).Adjusted != 900 || v.RawTotal != 1010 {
		t.Fatal(v)
	}
}

func TestInvalidHookIsVisibleAsPartialAndNegativeRowsStayPending(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 10, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 1000, 2, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", -20, 395152, aug)
	v := s.DamageView("boss")
	if !v.Valid || !v.Partial || v.RejectedHooks != 1 || v.RawTotal != v.AdjustedTotal {
		t.Fatal(v)
	}
	damageRecord(s, 4, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 2000, 395152, aug)
	if s.DamageView("boss").Valid {
		t.Fatal("over-attribution presented as settled")
	}
}

func TestDamageSegmentBoundariesHistoryAndEverySpecCollection(t *testing.T) {
	s := ready()
	damageRecord(s, 1, "SPELL_DAMAGE", aug, "Creature-1", 999, 1, "")
	if s.DamageView("boss") != nil {
		t.Fatal("missing specialization accepted")
	}
	start(s, 2)
	spec(s, 2, aug, 1473)
	damageRecord(s, 3, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	end(s, 12)
	v := s.DamageView("boss")
	if v.Live || v.RawTotal != 100 || v.Seconds != 10 {
		t.Fatal(v)
	}
	ingest(s, 13, "ZONE_CHANGE", "999", "Outside", "0")
	if v = s.DamageView("boss"); v == nil || v.RawTotal != 100 || v.Live {
		t.Fatal("completed meter lost on leaving dungeon", v)
	}
	start(s, 14)
	spec(s, 14, aug, 1467)
	damageRecord(s, 15, "SPELL_DAMAGE", aug, "Creature-1", 200, 1, "")
	if v := s.DamageView("boss"); v == nil || !v.Live || v.RawTotal != 200 {
		t.Fatal("ordinary damage specialization not collected", v)
	}
	state := s.Snapshot()
	if len(state.DamageMeters) != 2 {
		t.Fatal(state)
	}
	s.SetEnabled(false)
	if s.DamageView("boss") != nil {
		t.Fatal("disabled meter visible")
	}
}

func TestAdvancedCastMetadataResolvesPetWhoseSummonWasSkipped(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE", "Pet-1", "Creature-1", 1000, 2, "")
	f := actor("SPELL_CAST_SUCCESS", "Pet-1", "Creature-1", 999)
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f[12] = "Pet-1"
	f[13] = other
	ingest(s, 4, f...)
	v := s.DamageView("boss")
	if !v.Valid || findRow(t, v, other).Raw != 1000 {
		t.Fatal(v)
	}
}

func TestDamageOverlayUsesVerifiedWindowAndHidesWhenUnfocused(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	w := lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}
	now := time.Now()
	v := s.Merge(engine.FrameDisplay{}, c, w, now)
	if !v.Enabled || !v.OverlaysActive || v.Window != "123" || len(v.Overlays) < 4 {
		t.Fatal(v)
	}
	for _, entry := range v.Overlays {
		if entry.Ends <= v.At {
			t.Fatal("renderer needs a future display lease even for persistent bars", entry)
		}
		if entry.Opacity != 100 {
			t.Fatal("renderer opacity is a percentage", entry)
		}
	}
	w.Foreground = false
	if s.Merge(engine.FrameDisplay{}, c, w, now).Enabled {
		t.Fatal("drew over other app")
	}
	w.Foreground = true
	c.AugShowMeter = false
	if s.Merge(engine.FrameDisplay{}, c, w, now).Enabled {
		t.Fatal("ignored hidden meter")
	}
	c.AugShowMeter = true
	if len(s.Merge(engine.FrameDisplay{Enabled: true, Window: "999", Width: 1920, Height: 1080}, c, w, now).Overlays) > 0 {
		t.Fatal("merged unrelated HWND")
	}
}
