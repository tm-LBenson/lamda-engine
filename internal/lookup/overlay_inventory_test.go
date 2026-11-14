package lookup

import "testing"

func TestPublicInventoryMetadataPreservesFalseAndClearsNativeValues(t *testing.T) {
	for _, carried := range []bool{false, true} {
		zero := 0.0
		yes := true
		p := OverlayInput{Epoch: "inventory", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, CooldownsVersion: 1, CooldownHash: "1234567812345678", Cooldowns: []OverlayCooldown{{Kind: "item", ID: 5511, Available: true, Carried: &carried, Remaining: &zero, Enabled: &yes}, {Kind: "spell", ID: 686, Carried: &carried}}}
		s := DecodeOverlayPixels(cooldownPixels(p), 0, 32)
		if s.Error != "" {
			t.Fatal(s.Error)
		}
		c := s.Overlays.Cooldowns[0]
		if c.Carried == nil || *c.Carried != carried {
			t.Fatal("bag state lost", carried)
		}
		if !c.Available {
			t.Fatal("public absence erased cooldown capture availability")
		}
		if c.Remaining != nil || c.Enabled != nil {
			t.Fatal("metadata supplied native measurements")
		}
		if s.Overlays.Cooldowns[1].Carried != nil {
			t.Fatal("non-item gained bag presence")
		}
	}
}
