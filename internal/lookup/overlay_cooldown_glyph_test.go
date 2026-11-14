package lookup

import "testing"

func TestCompactCooldownLowerRowGlyph(t *testing.T) {
	for _, remaining := range []int{0, 1, 9000} {
		p := Pixels{Width: 256, Height: 40, BGRA: make([]byte, 256*40*4)}
		input := OverlayInput{NativeVersion: 2, CooldownsVersion: 1, Cooldowns: []OverlayCooldown{{Kind: "item", ID: 5232, Available: true}}}
		values := []int{remaining, 18000, 0, 0, 0, 0, 1, 1}
		paint := func(field, copy, color int) {
			x, y := overlayTimerPoint(0, field, copy, 2)
			paintDense(&p, x, y, color)
			for dx := 0; dx < 2; dx++ {
				i := (y*p.Width + x + dx) * 4
				for b := 0; b < 3; b++ {
					p.BGRA[i+b] = 0
				}
			}
		}
		for f, v := range values {
			for copy := 0; copy < 2; copy++ {
				paint(f, copy, 0x100000+v)
			}
		}
		decodeOverlayCooldowns(p, 0, &input)
		v := input.Cooldowns[0]
		if v.Remaining == nil || *v.Remaining != float64(remaining)/10 || v.Enabled == nil || !*v.Enabled || v.Usable == nil || !*v.Usable {
			t.Fatalf("live glyph lost timer: %+v", v)
		}
		// A single positive tenth remains a cooldown, never rounded to ready.
		for _, bad := range []int{0, 0xffffff, 0x100000 + remaining + 1, 0x1fffff} {
			paint(0, 1, bad)
			decodeOverlayCooldowns(p, 0, &input)
			if input.Cooldowns[0].Remaining != nil {
				t.Fatalf("mismatched/invalid glyph accepted: %x", bad)
			}
		}
		paint(0, 0, 0)
		paint(0, 1, 0)
		decodeOverlayCooldowns(p, 0, &input)
		if input.Cooldowns[0].Remaining != nil {
			t.Fatal("black obstruction became ready")
		}
		for copy := 0; copy < 2; copy++ {
			paint(0, copy, 0x100000)
			x, y := overlayTimerPoint(0, 0, copy, 2)
			i := (y*p.Width + x) * 4
			p.BGRA[i] = 255
		}
		decodeOverlayCooldowns(p, 0, &input)
		if input.Cooldowns[0].Remaining != nil {
			t.Fatal("arbitrary mixed tile became ready")
		}
	}
}
