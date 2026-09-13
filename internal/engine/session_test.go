package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionPreservesCooldownsAcrossSubzones(t *testing.T) {
	s := &LogSession{}
	for _, tc := range []struct {
		event string
		reset bool
	}{
		{`ZONE_CHANGE,2813,"The Coiled Isle",205`, true},
		{`ZONE_CHANGE,2813,"Murder Row",205`, false},
		{`ZONE_CHANGE,2813,"Den of Indulgence",205`, false},
		{`ZONE_CHANGE,2813,"Murder Row",1`, true},
		{`ZONE_CHANGE,1,"Outside",0`, true},
		{`ZONE_CHANGE,1,"A different room",0`, false},
		{`ZONE_CHANGE,nope,"Malformed",0`, false},
		{`ZONE_CHANGE,1,"Missing difficulty"`, false},
		{`CHALLENGE_MODE_START,1,"Dungeon",2`, true},
		{`CHALLENGE_MODE_END,1,0`, true},
		{`COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1`, true},
		{`ZONE_CHANGE,1,"Outside",0`, true},
	} {
		line := "9/13/2026 11:35:00.312-4\t" + tc.event
		if got := s.ResetFor(line); got != tc.reset {
			t.Fatalf("%s: reset=%v, want %v", tc.event, got, tc.reset)
		}
	}
	if s.ResetFor(mirror) {
		t.Fatal("cast reset the session")
	}
}

func TestSessionSeedsMetadataWithoutReplayingHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WoWCombatLog-a.txt")
	zone := "9/13/2026 11:35:00.312-4  ZONE_CHANGE,2813,\"Murder Row\",205\n"
	os.WriteFile(path, []byte(zone+mirror+"\n"), 0600)
	tail := &Tail{}
	lines, err := tail.Poll(dir)
	if err != nil || len(lines) != 0 || tail.StartOffset == 0 {
		t.Fatal("live start replayed history", lines, err)
	}
	session := &LogSession{}
	if err := session.Seed(path, tail.StartOffset); err != nil {
		t.Fatal(err)
	}
	if session.ResetFor(strings.TrimSpace(strings.Replace(zone, "Murder Row", "Den of Indulgence", 1))) {
		t.Fatal("first room transition erased observations despite known starting map")
	}
	if !session.ResetFor(strings.TrimSpace(strings.Replace(zone, ",205", ",1", 1))) {
		t.Fatal("real difficulty change preserved stale observations")
	}
	// A newer log-session marker invalidates metadata from the previous session.
	content := zone + "9/13/2026 11:35:01.312-4  COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1\n"
	os.WriteFile(path, []byte(content), 0600)
	session.Seed(path, int64(len(content)))
	if !session.ResetFor(strings.TrimSpace(zone)) {
		t.Fatal("previous session metadata survived a new log session")
	}
}

func TestSessionSeedIsBoundedAndKeepsUnknownConservative(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WoWCombatLog-a.txt")
	zone := "9/13/2026 11:35:00.312-4  ZONE_CHANGE,2813,\"Murder Row\",205\n"
	content := zone + strings.Repeat("x", (8<<20)+1) + "\n"
	os.WriteFile(path, []byte(content), 0600)
	session := &LogSession{}
	if err := session.Seed(path, int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if session.known || !session.ResetFor(strings.TrimSpace(zone)) {
		t.Fatal("metadata beyond scan bound treated as known")
	}
}
