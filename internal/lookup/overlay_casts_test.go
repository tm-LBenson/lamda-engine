package lookup

import (
	"encoding/json"
	"testing"
)

func TestCastMetadataAndPairedPixelIdentity(t *testing.T) {
	input := OverlayInput{Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, CooldownsVersion: 1, CooldownHash: "1234567812345678", CastsVersion: 1, CastHash: "1234567812345678"}
	input.Cooldowns = []OverlayCooldown{{Kind: "spell", ID: 100, Available: true}}
	var c OverlayCast
	if err := json.Unmarshal([]byte(`["target",4,true,2,[[1,1,42,100],[2,2,42,0]]]`), &c); err != nil {
		t.Fatal(err)
	}
	input.Casts = []OverlayCast{c}
	p := cooldownPixels(input)
	fields := map[int]int{0: 0x10000a, 1: 0x10001e, 2: 361469, 3: 0x100001, 4: 0x100001, 5: 4, 8: 100, 9: 1, 10: 200, 11: 2}
	for field, v := range fields {
		for copy := 0; copy < 2; copy++ {
			x, y := OverlayCooldownPoint(1, field, copy)
			paintOverlay(&p, x, y+32, v)
		}
	}
	s := DecodeOverlayPixels(p, 0, 32)
	if s.Error != "" {
		t.Fatal(s.Error)
	}
	got := s.Overlays.Casts[0]
	if got.Spell == nil || *got.Spell != 361469 || got.Remaining == nil || *got.Remaining != 1 || got.Mode == nil || *got.Mode != 1 || got.Events[0].Spell == nil || *got.Events[1].Spell != 200 {
		t.Fatal(got)
	}
	x, y := OverlayCooldownPoint(1, 11, 1)
	paintOverlay(&p, x, y+32, 3)
	if DecodeOverlayPixels(p, 0, 32).Overlays.Casts[0].Events[1].Spell != nil {
		t.Fatal("torn event identity accepted")
	}
	for copy := 0; copy < 2; copy++ {
		x, y := OverlayCooldownPoint(1, 5, copy)
		paintOverlay(&p, x, y+32, 5)
	}
	got = DecodeOverlayPixels(p, 0, 32).Overlays.Casts[0]
	if got.Spell != nil || got.Events[0].Spell != nil {
		t.Fatal("old unit identity accepted")
	}
	input.Casts[0] = s.Overlays.Casts[0]
	b, _ := json.Marshal(input)
	got = parseOverlays(b).Overlays.Casts[0]
	if got.Mode != nil || got.Spell != nil || got.Remaining != nil || got.Events[0].Spell != nil {
		t.Fatal("metadata injected native readings")
	}
}
func TestCastTupleRejectsInvalidAndOversizedInputs(t *testing.T) {
	for _, b := range []string{`["target",1,true,1,[[1,99,0,0]]]`, `["invalid",1,true,0,[]]`, `["target",1,true,5,[[1,1,0,0]]]`, `["target",1,true,1,[[1,1,0,4000]]]`, `["target",1,true,1,[[1,1,0,0],[1,1,0,0]]]`} {
		var c OverlayCast
		if json.Unmarshal([]byte(b), &c) == nil {
			t.Fatal(b)
		}
	}
	for base := 0; base <= 14; base++ {
		occupied := map[[2]int]bool{}
		for f := 0; f < 16; f++ {
			for c := 0; c < 2; c++ {
				x, y := OverlayCooldownPoint(base, f, c)
				k := [2]int{x, y}
				if occupied[k] || x < 0 || x > 252 || y < 0 || y > 36 {
					t.Fatal(k)
				}
				occupied[k] = true
			}
		}
	}
}
