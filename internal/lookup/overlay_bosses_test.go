package lookup

import (
	"math"
	"os"
	"testing"
)

func TestBossMetadataValidationAndUnknowns(t *testing.T) {
	ptr := func(n float64) *float64 { return &n }
	base := func() *OverlayBosses {
		return &OverlayBosses{Seq: 1, Providers: []OverlayBossProvider{{Name: "dbm", Available: true, Complete: true}}, Timers: []OverlayBossTimer{{Provider: "dbm", ID: 1, Name: "Boss", Remaining: ptr(3), Duration: ptr(10)}}, Events: []OverlayBossEvent{{Seq: 1, Kind: "bossstart", Timer: OverlayBossTimer{Provider: "dbm", ID: 1}}}}
	}
	p := OverlayInput{BossesVersion: 1, Bosses: base()}
	sanitizeOverlayBosses(&p)
	if p.Bosses == nil || p.Bosses.Timers[0].Spell != nil || p.Bosses.Timers[0].Paused != nil {
		t.Fatal("valid unknown fields changed")
	}
	for _, tc := range []struct {
		name string
		edit func(*OverlayBosses)
	}{
		{"duplicate provider", func(b *OverlayBosses) { b.Providers = append(b.Providers, b.Providers[0]) }},
		{"unsupported provider", func(b *OverlayBosses) { b.Providers[0].Name = "other" }},
		{"unavailable timer", func(b *OverlayBosses) { b.Providers[0].Available = false; b.Providers[0].Complete = false }},
		{"duplicate timer", func(b *OverlayBosses) { b.Timers = append(b.Timers, b.Timers[0]) }},
		{"invalid lifetime", func(b *OverlayBosses) { b.Timers[0].ID = 0 }},
		{"invalid name", func(b *OverlayBosses) { b.Timers[0].Name = string([]byte{255}) }},
		{"long name", func(b *OverlayBosses) { b.Timers[0].Name = string(make([]byte, 161)) }},
		{"invalid source", func(b *OverlayBosses) { b.Timers[0].Source = "encounter" }},
		{"invalid duration", func(b *OverlayBosses) { b.Timers[0].Duration = ptr(math.NaN()) }},
		{"negative remaining", func(b *OverlayBosses) { b.Timers[0].Remaining = ptr(-1) }},
		{"invalid count", func(b *OverlayBosses) { b.Timers[0].Count = ptr(math.Inf(1)) }},
		{"invalid stage", func(b *OverlayBosses) { b.Providers[0].Stage = ptr(-1) }},
		{"duplicate event", func(b *OverlayBosses) { b.Events = append(b.Events, b.Events[0]) }},
		{"future event", func(b *OverlayBosses) { b.Events[0].Seq = 2 }},
		{"stale event", func(b *OverlayBosses) { b.Events[0].Age = 3001 }},
		{"missing lifetime", func(b *OverlayBosses) { b.Events[0].Timer.ID = 0 }},
		{"event provider", func(b *OverlayBosses) { b.Events[0].Timer.Provider = "bigwigs" }},
		{"invalid event", func(b *OverlayBosses) { b.Events[0].Kind = "invented" }},
		{"too many timers", func(b *OverlayBosses) { b.Timers = make([]OverlayBossTimer, 17) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p.Bosses = base()
			tc.edit(p.Bosses)
			sanitizeOverlayBosses(&p)
			if p.Bosses != nil {
				t.Fatal("invalid metadata retained")
			}
		})
	}
	for _, kind := range []string{"bosswarning", "bossstage"} {
		p.Bosses = base()
		p.Bosses.Events[0].Kind = kind
		p.Bosses.Events[0].Timer.ID = 0
		sanitizeOverlayBosses(&p)
		if p.Bosses == nil {
			t.Fatal("non-timer event rejected", kind)
		}
	}
	p.Bosses = base()
	p.BossesVersion = 2
	sanitizeOverlayBosses(&p)
	if p.Bosses != nil {
		t.Fatal("unsupported version")
	}
}
func TestBundledLuaBossMetadataInteroperability(t *testing.T) {
	body, err := os.ReadFile("testdata/overlays-bosses-deflate.bin")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := expandOverlayMetadata(body)
	if err != nil || len(raw) <= 6134 {
		t.Fatal("invalid maximum fixture", err, len(raw))
	}
	s := DecodeSample(simPacketFixture("LUO1", SimBridgeBytes, body))
	if s.Error != "" || s.Overlays == nil {
		t.Fatal(s.Error)
	}
	p := s.Overlays
	if p.Bosses == nil || len(p.Bosses.Timers) != 16 || len(p.Bosses.Events) != 8 || len(p.Watches) != 16 || len(p.Casts) != 8 || len(p.Measurements) != 16 || len(p.Identities) != 8 || len(p.Units) != 5 || len(p.Anchors) != 8 || p.Encounter == nil || len(p.Encounter.Events) != 4 {
		t.Fatalf("source lost: %+v", p)
	}
	if p.Bosses.Timers[0].Spell == nil || *p.Bosses.Timers[0].Spell != 9999999 || p.Bosses.Timers[0].Remaining == nil || *p.Bosses.Timers[0].Remaining != 86400 || len(p.Bosses.Timers[0].Name) != 160 {
		t.Fatal("public timer metadata changed")
	}
	if p.Health != nil || p.Watches[0].Active != nil {
		t.Fatal("metadata supplied native readings")
	}
}
