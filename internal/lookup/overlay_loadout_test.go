package lookup

import (
	"math"
	"os"
	"testing"
)

func TestPublicTalentAndEquipmentValidation(t *testing.T) {
	ip := func(v int) *int { return &v }
	fp := func(v float64) *float64 { return &v }
	p := OverlayInput{TalentsVersion: 1, Talents: []OverlayTalent{{Kind: "entry", ID: 1, Rank: ip(2)}, {Kind: "known", ID: 2}}}
	sanitizeOverlayTalents(&p)
	if len(p.Talents) != 2 || p.Talents[1].Rank != nil {
		t.Fatal("valid talent unknown lost")
	}
	for _, tals := range [][]OverlayTalent{{{Kind: "known", ID: 1, Rank: ip(2)}}, {{Kind: "entry", ID: 1}, {Kind: "entry", ID: 1}}, {{Kind: "spell", ID: 0}}, {{Kind: "entry", ID: 1, Rank: ip(-1)}}} {
		p.Talents = tals
		sanitizeOverlayTalents(&p)
		if p.Talents != nil {
			t.Fatal("bad talent survived")
		}
	}
	base := func() *OverlayEquipment {
		return &OverlayEquipment{Slots: []OverlayEquipmentSlot{{Slot: 1, Item: ip(123), Level: fp(777)}, {Slot: 2, Item: ip(0)}, {Slot: 3}}, Sets: []OverlayEquipmentSet{{Key: "0123456789abcdef"}}}
	}
	p.EquipmentVersion = 1
	p.Equipment = base()
	sanitizeOverlayEquipment(&p)
	if p.Equipment == nil || p.Equipment.Slots[2].Item != nil {
		t.Fatal("gear unknown lost")
	}
	for _, edit := range []func(*OverlayEquipment){func(e *OverlayEquipment) { e.Slots = append(e.Slots, e.Slots[0]) }, func(e *OverlayEquipment) { e.Slots[0].Slot = 20 }, func(e *OverlayEquipment) { e.Slots[0].Level = fp(math.NaN()) }, func(e *OverlayEquipment) { e.Slots[1].Set = ip(0) }, func(e *OverlayEquipment) { e.Sets[0].Key = "bad" }, func(e *OverlayEquipment) { e.Slots[0].Name = string([]byte{255}) }} {
		p.Equipment = base()
		edit(p.Equipment)
		sanitizeOverlayEquipment(&p)
		if p.Equipment != nil {
			t.Fatal("bad gear survived")
		}
	}
}

func TestBundledLuaLoadoutMaximumMetadata(t *testing.T) {
	body, e := os.ReadFile("testdata/overlays-loadout-deflate.bin")
	if e != nil {
		t.Fatal(e)
	}
	s := DecodeSample(simPacketFixture("LUO1", SimBridgeBytes, body))
	p := s.Overlays
	if s.Error != "" || p == nil {
		t.Fatal(s.Error)
	}
	if p.Equipment == nil || len(p.Equipment.Slots) != 19 || len(p.Equipment.Sets) != 8 || len(p.Talents) != 16 || p.Bosses == nil || len(p.Bosses.Events) != 8 || len(p.Bosses.Timers) != 16 {
		t.Fatal("maximum packet lost data")
	}
	for _, slot := range p.Equipment.Slots {
		if slot.Item == nil || slot.Level == nil || slot.Set == nil {
			t.Fatal("required gear facts dropped")
		}
	}
	for _, ev := range p.Bosses.Events {
		if ev.Kind != "bosswarning" {
			t.Fatal("weak duplicate-name fixture")
		}
	}
	if p.Equipment.Slots[0].Name != "" {
		t.Fatal("fixture should exercise optional-label fallback")
	}
}

func TestLoadoutChangeMetadataValidation(t *testing.T) {
	ip := func(i int) *int { return &i }
	p := OverlayInput{LoadoutChangesVersion: 1, LoadoutChanges: []OverlayLoadoutChange{{Kind: "talents", Seq: 1, Age: ip(0)}, {Kind: "equipment", Seq: 0}}}
	sanitizeLoadoutChanges(&p)
	if len(p.LoadoutChanges) != 2 {
		t.Fatal("valid changes lost")
	}
	for _, rows := range [][]OverlayLoadoutChange{{{Kind: "bad"}}, {{Kind: "talents", Seq: 0, Age: ip(0)}}, {{Kind: "talents", Seq: -1}}, {{Kind: "talents", Seq: 1, Age: ip(3001)}}, {{Kind: "talents"}, {Kind: "talents"}}} {
		p.LoadoutChanges = rows
		sanitizeLoadoutChanges(&p)
		if p.LoadoutChanges != nil {
			t.Fatal("bad changes survived")
		}
	}
}

func TestBundledLuaLoadoutChangesMetadata(t *testing.T) {
	body, e := os.ReadFile("testdata/overlays-loadout-changes-deflate.bin")
	if e != nil {
		t.Fatal(e)
	}
	s := DecodeSample(simPacketFixture("LUO1", SimBridgeBytes, body))
	p := s.Overlays
	if s.Error != "" || p == nil {
		t.Fatal(s.Error)
	}
	if p.LoadoutChangesVersion != 1 || len(p.LoadoutChanges) != 2 || p.Equipment == nil || len(p.Equipment.Slots) != 19 || len(p.Talents) != 16 || len(p.Watches) != 16 || len(p.Casts) != 8 {
		t.Fatal("maximum change metadata lost facts")
	}
	for _, change := range p.LoadoutChanges {
		if change.Age == nil || change.Seq != 2147483647 {
			t.Fatal("maximum event lost", change)
		}
	}
}
