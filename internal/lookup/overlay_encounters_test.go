package lookup

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestEncounterMetadataBoundsAndUnknowns(t *testing.T) {
	yes := true
	elapsed := 60.0
	base := OverlayEncounter{Active: &yes, ID: 1234, Name: "Boss", Difficulty: 15, GroupSize: 20, Elapsed: &elapsed, Seq: 1, Events: []OverlayEncounterEvent{{Seq: 1, Kind: "encounterstart", ID: 1234}}}
	p := OverlayInput{EncountersVersion: 1, Encounter: &base}
	sanitizeOverlayEncounter(&p)
	if p.Encounter == nil {
		t.Fatal("valid rejected")
	}
	for _, edit := range []func(*OverlayEncounter){
		func(e *OverlayEncounter) { e.ID = -1 }, func(e *OverlayEncounter) { e.Difficulty = 1001 }, func(e *OverlayEncounter) { e.GroupSize = 101 }, func(e *OverlayEncounter) { e.Name = string(make([]byte, 161)) },
		func(e *OverlayEncounter) { v := math.NaN(); e.Elapsed = &v }, func(e *OverlayEncounter) { e.Events = append(e.Events, e.Events[0]) },
		func(e *OverlayEncounter) { e.Events[0].Success = &yes }, func(e *OverlayEncounter) { e.Events[0].Age = 3001 }, func(e *OverlayEncounter) { e.Events[0].Kind = "invented" }, func(e *OverlayEncounter) { e.Events[0].Seq = 2 }, func(e *OverlayEncounter) { v := -1.0; e.Events[0].Elapsed = &v },
	} {
		e := base
		e.Events = append([]OverlayEncounterEvent(nil), base.Events...)
		edit(&e)
		p.Encounter = &e
		sanitizeOverlayEncounter(&p)
		if p.Encounter != nil {
			t.Fatal("bad metadata accepted", e)
		}
	}
	p.Encounter = &OverlayEncounter{}
	sanitizeOverlayEncounter(&p)
	if p.Encounter == nil || p.Encounter.Active != nil || p.Encounter.Elapsed != nil {
		t.Fatal("unknown changed")
	}
	p.EncountersVersion = 2
	sanitizeOverlayEncounter(&p)
	if p.Encounter != nil {
		t.Fatal("unsupported version retained")
	}
}
func TestBundledLuaCompressionDecoderInteroperability(t *testing.T) {
	b, e := os.ReadFile("testdata/overlays-max-deflate.bin")
	if e != nil {
		t.Fatal(e)
	}
	data, e := expandOverlayMetadata(b)
	if e != nil || len(data) <= 6134 {
		t.Fatal("not a maximum compressed packet", e, len(data))
	}
	packet := simPacketFixture("LUO1", SimBridgeBytes, b)
	s := DecodeSample(packet)
	if s.Error != "" || s.Overlays == nil {
		t.Fatal("Lua compression incompatible", s.Error)
	}
	packet[12] ^= 1
	if corrupt := DecodeSample(packet); corrupt.Error != "bridge_checksum" {
		t.Fatal("compressed CRC not checked", corrupt.Error)
	}
	p := s.Overlays
	if len(p.Watches) != 16 || len(p.Casts) != 8 || len(p.Measurements) != 16 || len(p.Identities) != 8 || len(p.Units) != 5 || len(p.Anchors) != 8 || p.Encounter == nil || len(p.Encounter.Events) != 4 {
		t.Fatalf("source lost: %+v", p)
	}
	for _, u := range p.Units {
		if u.Name == "" {
			t.Fatal("name dropped")
		}
	}
	if p.Health != nil || p.Watches[0].Active != nil {
		t.Fatal("public metadata supplied native values")
	}
	var parsed map[string]any
	if e = json.Unmarshal(data, &parsed); e != nil {
		t.Fatal(e)
	}
	if parsed["encountersVersion"] != float64(1) {
		t.Fatal("encounter version missing")
	}
}
