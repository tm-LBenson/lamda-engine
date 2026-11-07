package augmentation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func saveNavigation(t *testing.T, s *Store, v meterNavigation) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(s.path), "view.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestOverlaySelectionUsesActorIdentityAfterAugRowsReorder(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	damageRecord(s, 3, "SPELL_DAMAGE", other, "Creature-1", 1000, 20, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 800, 395152, aug)
	end(s, 10)
	m := s.damageMeters[0]
	if m.Rows[0].GUID != aug {
		t.Fatal("fixture needs Aug to overtake the raw damage leader", m)
	}
	nav := meterNavigation{Schema: 1, Owner: aug, Segment: m.ID, Actor: aug}
	saveNavigation(t, s, nav)
	body := s.presentMeterView(m, defaultMeterAppearance(), nil, 236, "Current", "Recorded")
	if body.Actor != aug || body.Segment != m.ID || body.View != "spells" || len(body.Rows) != 2 {
		t.Fatal(body)
	}
	var found bool
	for _, row := range body.Rows {
		if row.ID == "support/"+other+"/395152" {
			found = true
			nav.Spell = row.ID
		}
	}
	if !found {
		t.Fatal("wrong actor drilldown", body)
	}
	saveNavigation(t, s, nav)
	body = s.presentMeterView(m, defaultMeterAppearance(), nil, 236, "Current", "Recorded")
	if body.View != "targets" || len(body.Rows) != 1 || body.Rows[0].Value != "800  |  89/s" {
		t.Fatal(body)
	}
	// A new combat or a different character cannot inherit a stale row action.
	m.ID = "boss/new"
	body = s.presentMeterView(m, defaultMeterAppearance(), nil, 236, "Current", "Recorded")
	if body.View != "actors" || body.Actor != "" {
		t.Fatal(body)
	}
}

func TestOverlayNavigationRejectsInvalidSelectionAndRetainsMissingData(t *testing.T) {
	s := ready()
	s.path = filepath.Join(t.TempDir(), "reports.json")
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 10, "")
	end(s, 10)
	m := s.damageMeters[0]
	base := meterNavigation{Schema: 1, Owner: aug, Segment: m.ID, Actor: aug}
	for _, change := range []func(*meterNavigation){func(v *meterNavigation) { v.Schema = 2 }, func(v *meterNavigation) { v.Owner = other }, func(v *meterNavigation) { v.Offset = -1 }, func(v *meterNavigation) { v.Actor = "Creature-1" }, func(v *meterNavigation) { v.Spell = "raw/1\n" }, func(v *meterNavigation) { v.Spell = "raw/1000000000" }} {
		v := base
		change(&v)
		saveNavigation(t, s, v)
		if got := s.meterNavigation(m); got.Schema != 0 {
			t.Fatal("invalid navigation accepted", got)
		}
	}
	saveNavigation(t, s, base)
	delete(s.meterDetails, aug+"/"+m.ID)
	body := s.presentMeterView(m, defaultMeterAppearance(), nil, 236, "Current", "Recorded")
	if len(body.Rows) != 0 || body.Empty != "Spell details unavailable for this session" || body.View != "spells" {
		t.Fatal(body)
	}
}
