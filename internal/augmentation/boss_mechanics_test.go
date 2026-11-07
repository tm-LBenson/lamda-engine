package augmentation

import (
	"strings"
	"testing"
)

func mechanicHit(s *Store, second int, name string, amount int, periodic bool) {
	kind := "SPELL_DAMAGE"
	if periodic {
		kind = "SPELL_PERIODIC_DAMAGE"
	}
	f := actor(kind, "Creature-Hostile", aug, 1289855)
	f[3] = "0xa48"
	f[7] = "0x511"
	f[10] = name
	if name == "Spreading Flames" {
		f[9] = "1297650"
	}
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, intString(amount), intString(amount), "-1", "12", "0", "0", "0", "nil", "nil", "nil", "AOE")
	ingest(s, second, f...)
}

func TestGoodSoaksCountParticipationNotTicksOrAvoidableDamage(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3470", "Nek'zali the Soulcoiler", "16", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 1000, 1, "")
	mechanicHit(s, 3, "Hungering Pyre", 100, false)
	mechanicHit(s, 4, "Hungering Pyre", 100, false)
	mechanicHit(s, 5, "Hungering Pyre", 100, true)
	mechanicHit(s, 15, "Hungering Pyre", 100, false)
	ingest(s, 21, "ENCOUNTER_END", "3470", "Nek'zali the Soulcoiler", "16", "20", "1")
	c := s.damageMeters[0].Utility
	if !c.MechanicsKnown || c.Soaks[aug] != 2 || c.Avoidable[aug] != 0 {
		t.Fatal(c)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Fire Brigade:") || !strings.Contains(r.Text, "2 Hungering Pyre soaks") || strings.Contains(r.Text, "Healer's Problem") {
		t.Fatal(r, err)
	}
}

func TestIntentionalPatchClearingIsNotAvoidableAndKnownGroundIs(t *testing.T) {
	s := ready()
	ingest(s, 1, "ENCOUNTER_START", "3497", "The Lost Explorers", "16", "20")
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 1000, 1, "")
	mechanicHit(s, 3, "Frost Patch", 100, false)
	mechanicHit(s, 4, "Fire Patch", 100, false)
	mechanicHit(s, 5, "Spreading Flames", 250, false)
	mechanicHit(s, 6, "Spreading Flames", 250, true)
	ingest(s, 21, "ENCOUNTER_END", "3497", "The Lost Explorers", "16", "20", "1")
	c := s.damageMeters[0].Utility
	if c.Avoidable[aug] != 500 || c.Soaks[aug] != 0 {
		t.Fatal(c)
	}
	r, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(r.Text, "Spicy Footwork:") || !strings.Contains(r.Text, "500 Spreading Flames damage") {
		t.Fatal(r, err)
	}
}

func TestTankOnlySoaksDoNotRewardAccidentalDPSHits(t *testing.T) {
	for _, specID := range []int{0, 1473, 73} {
		s := ready()
		s.specs[aug] = specID
		ingest(s, 1, "ENCOUNTER_START", "3421", "The Twin Fangs", "16", "20")
		damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 1000, 1, "")
		mechanicHit(s, 3, "Stone Breaker", 100, false)
		ingest(s, 21, "ENCOUNTER_END", "3421", "The Twin Fangs", "16", "20", "1")
		want := int64(0)
		if specID == 73 {
			want = 1
		}
		if s.damageMeters[0].Utility.Soaks[aug] != want {
			t.Fatal(specID, s.damageMeters[0].Utility)
		}
	}
}
