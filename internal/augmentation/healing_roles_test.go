package augmentation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func awardHeal(s *Store, second int, source, target string, amount, over int) {
	f := actor("SPELL_HEAL", source, target, 123)
	f[2] = "Medic"
	if source == aug {
		f[2] = "Tank"
		f[3] = "0x511"
	}
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, intString(amount), intString(amount), intString(over), "0", "nil")
	ingest(s, second, f...)
}

func TestOverhealingJokeOnlyComparesConfirmedHealers(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "123", "Boss", "15", "20")
	spec(s, 2, aug, 250)
	spec(s, 2, other, 264)
	awardHeal(s, 3, aug, aug, 100000000, 90000000)
	awardHeal(s, 4, other, aug, 3000000, 2000000)
	ingest(s, 10, "ENCOUNTER_END", "123", "Boss", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if c.Overhealing[aug] <= c.Overhealing[other] || c.HealingRoles[aug] != "TANK" || c.HealingRoles[other] != "HEALER" {
		t.Fatal(c)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Just In Case: Medic - 2.00M overhealing on others") || strings.Contains(r.Text, "Just In Case: Tank") {
		t.Fatal(r, err)
	}
}

func TestHealerSelfPetAndGuardianSelfHealingDoNotCountAgainstOthers(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "123", "Boss", "15", "20")
	spec(s, 2, aug, 250)
	spec(s, 2, other, 1468)
	awardHeal(s, 2, aug, aug, 100, 0)
	s.pets["Pet-Healer"] = other
	awardHeal(s, 3, other, other, 100000000, 90000000)
	awardHeal(s, 4, "Pet-Healer", other, 100000000, 90000000)
	awardHeal(s, 5, other, "Pet-Recipient", 100000000, 90000000)
	awardHeal(s, 6, "Pet-Healer", aug, 3000000, 2000000)
	ingest(s, 10, "ENCOUNTER_END", "123", "Boss", "15", "20", "1")
	c := s.damageMeters[0].Utility
	if c.HealingOthers[other] != 1000000 || c.OverhealingOthers[other] != 2000000 || c.Overhealing[other] != 272000000 {
		t.Fatal(c)
	}
}

func TestUnknownDPSAndStalePrepullRolesCannotWinOverhealingBanter(t *testing.T) {
	for _, specID := range []int{0, 1473, 250, 999999} {
		s := ready()
		spec(s, 0, other, 264) // A previous pull's healer metadata is insufficient.
		ingest(s, 1, "ENCOUNTER_START", "123", "Boss", "15", "20")
		awardHeal(s, 2, aug, aug, 100, 0)
		if specID != 0 {
			spec(s, 2, other, specID)
		}
		awardHeal(s, 3, other, aug, 10000000, 9000000)
		ingest(s, 10, "ENCOUNTER_END", "123", "Boss", "15", "20", "1")
		r, err := s.PreparedSummaryForPlayer("boss", aug)
		if err != nil || strings.Contains(r.Text, "Just In Case:") {
			t.Fatal(specID, r, err)
		}
	}
}

func TestRoleChangesAcrossBossAttemptsAreNotCombinedIntoHealerBanter(t *testing.T) {
	s := ready()
	for i, specID := range []int{264, 250} {
		at := i*20 + 1
		ingest(s, at, "ENCOUNTER_START", "123", "Boss", "15", "20")
		spec(s, at+1, other, specID)
		awardHeal(s, at+2, other, aug, 3000000, 2000000)
		awardHeal(s, at+3, aug, aug, 100, 0) // establish the recording character
		ingest(s, at+10, "ENCOUNTER_END", "123", "Boss", "15", "20", "0")
	}
	m, pulls := s.bossProgressionLocked(s.damageMeters[0])
	if len(pulls) != 2 || m.Utility.HealingRoles[other] != "MIXED" {
		t.Fatal(m.Utility, pulls)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || strings.Contains(r.Text, "Just In Case:") {
		t.Fatal(r, err)
	}
}

func TestRoleContextAndOtherPlayerTotalsSurviveRestartAndLegacyIsUnknown(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "123", "Boss", "15", "20")
	spec(s, 2, other, 264)
	awardHeal(s, 3, aug, aug, 100, 0)
	awardHeal(s, 4, other, aug, 3000000, 2000000)
	ingest(s, 10, "ENCOUNTER_END", "123", "Boss", "15", "20", "1")
	data, err := json.Marshal(s.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "history.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded := New(path)
	defer loaded.Close()
	r, err := loaded.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Just In Case: Medic") {
		t.Fatal(r, err)
	}
	legacy := *s.damageMeters[0].Utility
	legacy.OverhealRoleVersion = 0
	legacy.HealingRoles = nil
	legacy.HealingOthers = nil
	legacy.OverhealingOthers = nil
	m := s.damageMeters[0]
	m.Utility = &legacy
	s.bossAttempts = nil
	s.prepareSummaryLocked(m)
	r, err = s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || strings.Contains(r.Text, "Just In Case:") || !needsAwardCountUpgrade(&legacy) {
		t.Fatal(r, err)
	}
}
