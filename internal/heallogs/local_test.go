package heallogs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(day int, death, broken, complete bool) string {
	var b strings.Builder
	line := func(second int, payload string) {
		fmt.Fprintf(&b, "9/%d/2026 13:00:%02d.000-4  %s\n", day, second, payload)
	}
	line(0, "COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1,BUILD_VERSION,12.1.0,PROJECT_ID,1")
	line(0, "ENCOUNTER_START,12,Fixture,2,2")
	for _, p := range []struct{ guid, spec string }{{"Player-A", "1468"}, {"Player-B", "73"}} {
		line(0, "COMBATANT_INFO,"+p.guid+strings.Repeat(",0", 23)+","+p.spec)
		amount := "100000"
		if broken {
			amount = "NaN"
		}
		line(1, "SPELL_DAMAGE,Creature-1,Enemy,0,0,"+p.guid+",Ally,0,0,123,Hit,1,"+p.guid+",0,700000,800000"+strings.Repeat(",0", 15)+","+amount+strings.Repeat(",0", 9))
	}
	if death {
		line(2, "UNIT_DIED,0000000000000000,nil,0,0,Player-B,Ally,0,0")
	}
	if complete {
		line(40, "ENCOUNTER_END,12,Fixture,2,2,1")
	}
	return b.String()
}
func TestParseRejectsCensoredCorruptAndUnfinishedFights(t *testing.T) {
	input := fixture(20, false, false, true) + fixture(21, true, false, true) + fixture(22, false, true, true) + fixture(23, false, false, false)
	samples, rejected, err := Parse(context.Background(), strings.NewReader(input), "fixture.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || len(samples[0].Replay.Events) != 2 {
		t.Fatal("wrong accepted fights", samples, rejected)
	}
	if rejected["player death truncates future damage"] != 1 || rejected["invalid damage amount"] != 1 || rejected["unfinished fight"] != 1 {
		t.Fatal("missing rejection reasons", rejected)
	}
	if samples[0].Replay.Members[0].Label != "You" || samples[0].Replay.Members[1].MaxHealth != 800000 {
		t.Fatal("wrong roster or health")
	}
}
func TestLearnCachesDeduplicatesAndRebuildsGrowingFiles(t *testing.T) {
	dir, cache := t.TempDir(), t.TempDir()
	path := filepath.Join(dir, "WoWCombatLog-a.txt")
	data := fixture(20, false, false, true) + fixture(21, false, false, true) + fixture(22, false, false, true)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "WoWCombatLog-copy.txt"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	a, catalog, audit, err := Learn(context.Background(), dir, cache, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 1 || len(a.Samples) != 2 || audit.Duplicates != 3 || audit.Cached != 0 {
		t.Fatal("duplicate recordings changed training", audit)
	}
	_, _, audit, err = Learn(context.Background(), dir, cache, "")
	if err != nil || audit.Cached != 2 {
		t.Fatal("cache not reused", audit, err)
	}
	if err := os.WriteFile(path, []byte(data+fixture(23, false, false, true)), 0600); err != nil {
		t.Fatal(err)
	}
	b, _, audit, err := Learn(context.Background(), dir, cache, a.Summary.Cohort)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Samples) != 3 || b.Holdout.ID == a.Holdout.ID || audit.Cached != 1 {
		t.Fatal("growing log was not refreshed", audit)
	}
	if _, _, _, err := Learn(context.Background(), dir, cache, "missing cohort"); err == nil {
		t.Fatal("unavailable selection silently fell back")
	}
}
func TestKeystoneMetadataDoesNotCarryAcrossLogSession(t *testing.T) {
	data := fixture(20, false, false, true)
	data = strings.Replace(data, "ENCOUNTER_START,12,Fixture,2,2", "CHALLENGE_MODE_START,Dungeon,100,100,10,[160,10,9]\n9/20/2026 13:00:00.000-4  ENCOUNTER_START,12,Fixture,8,2", 1)
	data = strings.Replace(data, "ENCOUNTER_END,12,Fixture,2,2,1", "ENCOUNTER_END,12,Fixture,8,2,1", 1)
	second := strings.ReplaceAll(fixture(21, false, false, true), "Fixture,2,2", "Fixture,8,2")
	samples, rejected, err := Parse(context.Background(), strings.NewReader(data+second), "fixture.txt")
	if err != nil || len(samples) != 1 || !strings.Contains(samples[0].Cohort, "/key10/affixes[160,10,9]/") || rejected["unknown keystone level or affixes"] != 1 {
		t.Fatal("key metadata leaked", samples, rejected, err)
	}
}
