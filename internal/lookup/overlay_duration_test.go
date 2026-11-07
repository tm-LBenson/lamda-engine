package lookup

import "testing"

func TestAuraTotalsRequireMarkedPairAndCapability(t *testing.T) {
	input := OverlayInput{Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, Watches: []OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true, DurationAvailable: true}}}
	for _, tc := range []struct {
		a, b int
		cap  bool
		want float64
	}{{0x1000c8, 0x1000c8, true, 20}, {0x1000c8, 0x1000c9, true, -1}, {0, 0, true, -1}, {0xffffff, 0xffffff, true, -1}, {0x1000c8, 0x1000c8, false, -1}} {
		input.Watches[0].DurationAvailable = tc.cap
		p := cooldownPixels(input)
		paintOverlay(&p, 12, 40, tc.a)
		paintOverlay(&p, 28, 40, tc.b)
		decoded := DecodeOverlayPixels(p, 0, 32)
		if decoded.Error != "" {
			t.Fatal(decoded.Error)
		}
		a := decoded.Overlays.Watches[0]
		if tc.want < 0 && a.Duration != nil || tc.want >= 0 && (a.Duration == nil || *a.Duration != tc.want) {
			t.Fatalf("%+v => %+v", tc, a)
		}
		if a.Stacks == nil || *a.Stacks != 3 || a.Remaining == nil || *a.Remaining != 4.9 {
			t.Fatal("total duration interfered with existing measurements", a)
		}
	}
}
