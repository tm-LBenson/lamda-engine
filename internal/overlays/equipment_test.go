package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
)

func gearInput() *lookup.OverlayInput {
	p := &lookup.OverlayInput{EquipmentVersion: 1, Equipment: &lookup.OverlayEquipment{}}
	for i := 1; i <= 19; i++ {
		p.Equipment.Slots = append(p.Equipment.Slots, lookup.OverlayEquipmentSlot{Slot: i, Item: ptr(0)})
	}
	p.Equipment.Slots[0] = lookup.OverlayEquipmentSlot{Slot: 1, Item: ptr(123), Level: ptr(777.5), Set: ptr(456), Class: ptr(4), Subclass: ptr(3)}
	p.Equipment.Slots[4] = lookup.OverlayEquipmentSlot{Slot: 5, Item: ptr(321), Level: ptr(780.0), Set: ptr(456), Class: ptr(4), Subclass: ptr(3)}
	p.Equipment.Sets = []lookup.OverlayEquipmentSet{{Key: equipmentSetKey("Healing É"), Equipped: ptr(true)}}
	return p
}
func TestEquipmentFactsComparisonsAndUnknowns(t *testing.T) {
	p := gearInput()
	cases := []struct {
		q    map[string]any
		want truth
	}{
		{map[string]any{"source": "item", "id": "123", "check": "equipped"}, yes},
		{map[string]any{"source": "item", "id": "123", "slot": float64(5), "check": "equipped"}, no},
		{map[string]any{"source": "item", "id": "999", "check": "missing"}, yes},
		{map[string]any{"source": "slot", "slot": float64(1), "check": "level", "operator": "gte", "value": 777.5}, yes},
		{map[string]any{"source": "slot", "slot": float64(2), "check": "level", "operator": "lt", "value": 777.5}, no},
		{map[string]any{"source": "slot", "slot": float64(2), "check": "empty"}, yes},
		{map[string]any{"source": "set", "id": "456", "check": "count", "operator": "eq", "value": float64(2)}, yes},
		{map[string]any{"source": "type", "itemClass": float64(4), "itemSubclass": float64(3), "check": "equipped"}, yes},
		{map[string]any{"source": "type", "itemClass": float64(2), "itemSubclass": float64(-1), "check": "missing"}, yes},
		{map[string]any{"source": "saved", "name": "Healing É", "check": "equipped"}, yes},
		{map[string]any{"source": "saved", "name": "Healing é", "check": "equipped"}, unknown},
	}
	for _, tc := range cases {
		tc.q["condition"] = "equipment"
		if !validEquipment(tc.q) {
			t.Fatal("valid condition rejected", tc.q)
		}
		if got := predicate(tc.q, p, "party1"); got != tc.want {
			t.Fatalf("%v: %v want %v", tc.q, got, tc.want)
		}
	}
	q := cases[2].q
	p.Equipment.Slots[1].Item = nil
	if predicate(q, p, "") != unknown || predicate(map[string]any{"condition": "group", "mode": "not", "tests": []any{q}}, p, "") != unknown {
		t.Fatal("unknown slot proved absence")
	}
	if predicate(cases[0].q, p, "") != yes {
		t.Fatal("unknown unrelated slot hid confirmed item")
	}
	if predicate(cases[6].q, p, "") != unknown {
		t.Fatal("partial gear counted as complete set")
	}
	p.Equipment.Slots[0].Level = nil
	if predicate(cases[3].q, p, "") != unknown {
		t.Fatal("missing level guessed")
	}
}
func TestEquipmentDemandAndNewOutfit(t *testing.T) {
	r := rule()
	q := map[string]any{"condition": "equipment", "source": "saved", "name": "Healing É", "check": "equipped"}
	r.Roots[0].Children = []Node{{ID: "gear", Type: "if", Props: q}}
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	if !needsEquipment(rs) || len(EquipmentSets(rs)) != 1 {
		t.Fatal("missing demand")
	}
	rt := &Runtime{}
	p := gearInput()
	rt.validateEquipment(p, rs)
	if rt.problem != "" {
		t.Fatal(rt.problem)
	}
	q["name"] = "Damage"
	rt.validateEquipment(p, rs)
	if rt.problem != "Reload WoW to watch new equipment sets" {
		t.Fatal(rt.problem)
	}
	rs[0].Enabled = false
	if needsEquipment(rs) || len(EquipmentSets(rs)) != 0 {
		t.Fatal("disabled demand")
	}
	for _, q := range []map[string]any{{"source": "slot", "slot": float64(20), "check": "empty"}, {"source": "set", "id": "123", "check": "count", "value": 2.5, "operator": "eq"}, {"source": "saved", "name": "\n", "check": "equipped"}, {"source": "type", "itemClass": float64(2), "itemSubclass": -1.5, "check": "equipped"}} {
		if validEquipment(q) {
			t.Fatal("invalid accepted", q)
		}
	}
}
