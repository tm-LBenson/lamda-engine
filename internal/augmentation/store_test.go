package augmentation

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const aug = "Player-1-Aug"
const other = "Player-1-Other"

// Fictional, format-faithful records. No simulated gameplay is claimed.
func record(second int, fields ...string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(fields)
	w.Flush()
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.FixedZone("test", -4*3600)).Add(time.Duration(second) * time.Second)
	return at.Format("01/02/2006 15:04:05.000") + "-4  " + strings.TrimSpace(b.String())
}
func ingest(s *Store, second int, f ...string) {
	s.Ingest(record(second, f...), time.Date(2026, 10, 26, 16, 0, second+20, 0, time.UTC))
}
func ready() *Store {
	s := New("")
	s.SetEnabled(true)
	ingest(s, 0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")
	return s
}
func actor(event, owner, target string, id int) []string {
	return []string{event, owner, "Name", "0x514", "0", target, "Target", "0x514", "0", fmt.Sprint(id), "Spell", "0xc"}
}
func cast(s *Store, second int, owner string, id int) {
	ingest(s, second, actor("SPELL_CAST_SUCCESS", owner, "Creature-1", id)...)
}
func auraEvent(s *Store, second int, event, owner, target string, id int) {
	ingest(s, second, append(actor(event, owner, target, id), "BUFF")...)
}
func spec(s *Store, second int, guid string, id int) {
	f := make([]string, 26)
	for i := range f {
		f[i] = "0"
	}
	f[0] = "COMBATANT_INFO"
	f[1] = guid
	f[25] = fmt.Sprint(id)
	ingest(s, second, f...)
}
func start(s *Store, second int) { ingest(s, second, "ENCOUNTER_START", "123", "Test boss", "8", "5") }
func end(s *Store, second int)   { ingest(s, second, "ENCOUNTER_END", "123", "Test boss", "8", "5", "1") }
func support(s *Store, second int, source, owner string, id int, amount int) {
	f := actor("SPELL_DAMAGE_SUPPORT", source, "Creature-1", id)
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, fmt.Sprint(amount), fmt.Sprint(amount), "-1", "12", "0", "0", "0", "nil", "nil", "nil", owner)
	ingest(s, second, f...)
	f[0] = "SPELL_DAMAGE"
	f[41] = "AOE"
	ingest(s, second, f...)
}

func TestSupportMirrorOwnerAndNonAugIsolation(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	spec(s, 1, other, 1473)
	spec(s, 1, "Player-1-Devo", 1467)
	support(s, 2, "Player-1-Ally", aug, 434481, 100)
	support(s, 3, "Player-1-Ally", other, 434481, 400)
	support(s, 4, "Player-1-Ally", "Player-1-Devo", 434481, 900)
	support(s, 5, "Player-1-Ally", aug, 999999, 50000)
	end(s, 10)
	state := s.Snapshot()
	if len(state.Reports) != 2 {
		t.Fatalf("non-Aug received a report: %+v", state.Reports)
	}
	for _, r := range state.Reports {
		if r.GUID == aug && (r.RecordedSupportDamage != 100 || r.UnknownSupportEvents != 1) {
			t.Fatal("mirror or unmodeled support incorrectly credited", r)
		}
		if r.GUID == other && r.RecordedSupportDamage != 400 {
			t.Fatal("another Aug's contribution moved", r)
		}
	}
	if state.DelaySeconds != 20 {
		t.Fatal(state.DelaySeconds)
	}
}

func TestPrepullRefreshAndOverlappingBossKeyIntervals(t *testing.T) {
	s := ready()
	auraEvent(s, 1, "SPELL_AURA_APPLIED", aug, aug, 395296)
	ingest(s, 5, "CHALLENGE_MODE_START", "Test dungeon", "1", "2", "13")
	ingest(s, 6, "ZONE_CHANGE", "1", "New room", "8")
	start(s, 10)
	auraEvent(s, 20, "SPELL_AURA_REFRESH", aug, aug, 395296)
	auraEvent(s, 20, "SPELL_AURA_APPLIED", aug, "Player-1-Ally", 410089)
	auraEvent(s, 30, "SPELL_AURA_REFRESH", aug, "Player-1-Ally", 410089)
	auraEvent(s, 40, "SPELL_AURA_REMOVED", aug, aug, 395296)
	auraEvent(s, 45, "SPELL_AURA_REMOVED", aug, "Player-1-Ally", 410089)
	cast(s, 46, aug, 403631)
	end(s, 50)
	ingest(s, 60, "CHALLENGE_MODE_END", "1", "1", "13", "10000", "0.000000", "0.000000")
	state := s.Snapshot()
	if len(state.Reports) != 2 {
		t.Fatal(state)
	}
	for _, r := range state.Reports {
		want := 30.0
		if r.Kind == "key" {
			want = 35
		}
		if r.EbonSeconds != want || r.PrescienceTargetSeconds != 25 || r.Casts[403631] != 1 {
			t.Fatalf("refresh/boundary double count: %+v", r)
		}
	}
}

func TestPartialAttachmentAndResetNeverComplete(t *testing.T) {
	s := ready()
	cast(s, 5, aug, 395152)
	end(s, 20)
	if len(s.Snapshot().Reports) != 0 {
		t.Fatal("mid-fight attachment produced complete report")
	}
	start(s, 30)
	cast(s, 31, aug, 395152)
	s.ResetInput("missing")
	end(s, 40)
	if len(s.Snapshot().Reports) != 0 {
		t.Fatal("reset retained partial segment")
	}
}

func TestMetadataAndDisableGates(t *testing.T) {
	for _, header := range [][]string{{"COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "0", "BUILD_VERSION", "12.1.5"}, {"COMBAT_LOG_VERSION", "23", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5"}, {"COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "13.0.0"}} {
		s := ready()
		ingest(s, 1, header...)
		start(s, 2)
		cast(s, 3, aug, 395152)
		end(s, 9)
		if len(s.Snapshot().Reports) != 0 {
			t.Fatal("unsupported format accepted")
		}
	}
	s := ready()
	start(s, 2)
	cast(s, 3, aug, 395152)
	s.SetEnabled(false)
	end(s, 9)
	if len(s.Snapshot().Reports) != 0 || s.Snapshot().Enabled {
		t.Fatal("disabled module kept running")
	}
}

func TestWrongBoundaryAndMalformedInput(t *testing.T) {
	s := ready()
	start(s, 1)
	cast(s, 2, aug, 395152)
	ingest(s, 3, "ENCOUNTER_END", "999", "Other", "8", "5", "1")
	if len(s.Snapshot().Reports) != 0 {
		t.Fatal("wrong encounter closed segment")
	}
	s.Ingest("truncated", time.Now())
	end(s, 10)
	if len(s.Snapshot().Reports) != 0 {
		t.Fatal("malformed segment accepted")
	}
}

func TestOutOfOrderAndZoneChangeDiscardSegments(t *testing.T) {
	s := ready()
	ingest(s, 1, "ZONE_CHANGE", "1", "A", "8")
	start(s, 2)
	cast(s, 3, aug, 395152)
	ingest(s, 4, "ZONE_CHANGE", "1", "Room B", "8")
	end(s, 5)
	if len(s.Snapshot().Reports) != 1 {
		t.Fatal("subzone reset complete fight")
	}
	start(s, 10)
	cast(s, 11, aug, 395152)
	ingest(s, 12, "ZONE_CHANGE", "2", "B", "8")
	end(s, 13)
	if len(s.Snapshot().Reports) != 1 {
		t.Fatal("zone change kept incomplete fight")
	}
	start(s, 20)
	cast(s, 21, aug, 395152)
	cast(s, 16, aug, 409311)
	end(s, 30)
	if len(s.Snapshot().Reports) != 1 {
		t.Fatal("out-of-order fight accepted")
	}
}

func TestPersistencePublicationAndTargetIsolation(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "data", "reports.json")
	s := New(statePath)
	s.SetEnabled(true)
	ingest(s, 0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")
	start(s, 1)
	cast(s, 2, aug, 395152)
	end(s, 10)
	addons := filepath.Join(dir, "AddOns")
	ui := filepath.Join(addons, "lamdaUI")
	_ = os.MkdirAll(ui, 0700)
	toc := filepath.Join(ui, "lamdaUI.toc")
	_ = os.WriteFile(toc, []byte("## X-Lamda-Augmentation-Client: forever\n"), 0600)
	if s.Flush(addons) == nil {
		t.Fatal("published to wrong client")
	}
	if _, err := os.Stat(filepath.Join(ui, "AugmentationResults.lua")); !os.IsNotExist(err) {
		t.Fatal("wrong target changed")
	}
	_ = os.WriteFile(toc, []byte("## X-Lamda-Augmentation-Client: retail\n"), 0600)
	if err := s.Flush(addons); err != nil {
		t.Fatal(err)
	}
	if len(New(statePath).Snapshot().Reports) != 1 {
		t.Fatal("history lost")
	}
	data, err := os.ReadFile(filepath.Join(ui, "AugmentationResults.lua"))
	if err != nil || !strings.Contains(string(data), "full support DPS unavailable") {
		t.Fatal(string(data), err)
	}
	if luaString("bad\"\\\n\x00x") != "\"bad\\\"\\\\\\010\\000x\"" {
		t.Fatal(luaString("bad\"\\\n\x00x"))
	}
}

func TestHeaderSeedDoesNotReplayOldFights(t *testing.T) {
	s := ready()
	path := filepath.Join(t.TempDir(), "log.txt")
	_ = os.WriteFile(path, []byte(record(0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")+"\n"+record(1, "ENCOUNTER_START", "123", "Old", "8", "5")+"\n"), 0600)
	s.ResetInput(path)
	cast(s, 3, aug, 395152)
	end(s, 5)
	if !s.supported || len(s.Snapshot().Reports) != 0 {
		t.Fatal("seed replayed old fight or failed header")
	}
	start(s, 10)
	cast(s, 11, aug, 395152)
	end(s, 20)
	if len(s.Snapshot().Reports) != 1 {
		t.Fatal("fresh fight not collected after seed")
	}
}

func TestCompletedHistoryBoundedAndLiveSnapshotsIndependent(t *testing.T) {
	s := ready()
	start(s, 1)
	cast(s, 2, aug, 395152)
	view := s.Snapshot()
	cast(s, 3, aug, 395152)
	if view.Live[0].Casts[395152] != 1 {
		t.Fatal("snapshot retained mutable cast map")
	}
	end(s, 4)
	for i := 0; i < 120; i++ {
		start(s, 10+i*3)
		cast(s, 11+i*3, aug, 395152)
		end(s, 12+i*3)
	}
	if len(s.Snapshot().Reports) != 100 {
		t.Fatal("unbounded history")
	}
}

func TestRepeatedCompatibleHeaderDoesNotLoseRunningKey(t *testing.T) {
	s := ready()
	s.own = aug
	ingest(s, 1, "CHALLENGE_MODE_START", "Test dungeon", "1", "2", "13")
	auraEvent(s, 2, "SPELL_AURA_APPLIED", aug, aug, 395296)
	ingest(s, 50, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")
	ingest(s, 4, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5")
	if s.active["key"] == nil || s.own != aug {
		t.Fatal("reload metadata erased an uninterrupted key")
	}
	ingest(s, 10, "CHALLENGE_MODE_END", "1", "1", "10000")
	if len(s.damageMeters) != 1 || s.damageMeters[0].Kind != "key" || s.damageMeters[0].Seconds != 9 {
		t.Fatal(s.damageMeters)
	}
	ingest(s, 11, "CHALLENGE_MODE_START", "Test dungeon", "1", "2", "13")
	ingest(s, 12, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "0", "BUILD_VERSION", "12.1.5")
	if s.supported || len(s.active) != 0 {
		t.Fatal("incompatible log retained live input")
	}
}
