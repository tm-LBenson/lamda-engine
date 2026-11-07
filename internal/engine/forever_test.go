package engine

import (
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestForeverPublicBaselineAndIsolation(t *testing.T) {
	if err := ConfigureClient("forever"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ConfigureClient("retail") })
	now := time.Now()
	m := NewModel()
	cast := Cast{GUID: "Player-1-TEST", Name: "Test", Spell: 1766, At: now, Source: "unit_spellcast"}
	m.Observe(cast, now)
	if got := m.Snapshot(now).Cooldowns[0]; got.Status != "unknown" || got.CooldownSeconds != 0 {
		t.Fatal(got)
	}
	if _, ok := Catalog[48707]; ok {
		t.Fatal("Retail defensive leaked")
	}
	if kickCandidate(framewire.KickMember{Class: 7}) != 0 {
		t.Fatal("Retail class suggestion leaked")
	}
	if m.ObserveForeverSpell(lookup.SpellMetadata{Build: "12.1.0.69814", Root: 1766, Spell: 1766, Millis: 15000, Name: "Kick"}) {
		t.Fatal("accepted Retail metadata")
	}
	// A synthetic public beta baseline validates transport behavior, not game balance.
	if !m.ObserveForeverSpell(lookup.SpellMetadata{Build: "1.60.1.69913", Root: 1766, Spell: 1766, Millis: 10000, Name: "Kick"}) {
		t.Fatal("metadata rejected")
	}
	if len(m.Evidence) != 0 {
		t.Fatal("old unknown cast gained a countdown")
	}
	if !m.Observe(cast, now) {
		t.Fatal("cast not tracked")
	}
	if got := m.Snapshot(now.Add(11 * time.Second)).Cooldowns[0]; got.Status != "estimated_ready" || got.CooldownSeconds != 10 {
		t.Fatal(got)
	}
	m.ObserveForeverSpell(lookup.SpellMetadata{Build: "1.60.1.69914", Root: 1766, Spell: 1766, Millis: 0, Name: "Kick"})
	if len(m.Evidence) != 0 || InterruptCatalog[1766].Cooldown != 0 {
		t.Fatal("build change retained old timers")
	}
	cfg := LimitForeverConfig(DefaultConfig())
	if cfg.PublicLookups || cfg.Companions || cfg.Updates || cfg.Notify {
		t.Fatal("Retail providers enabled")
	}
	fresh := DefaultClientConfig()
	if fresh.KickEnabled || fresh.FrameEstimates || fresh.Enabled || fresh.TrackingEnabled || fresh.ArenaTracking || fresh.PublicLookups {
		t.Fatal("first-login modules must remain disabled", fresh)
	}
	if m.ObserveInspectedBuild(BuildSnapshot{Source: "inspect"}, now) {
		t.Fatal("Retail inspect accepted")
	}
}

func TestForeverRanksDeduplicateAndUnknownNeverReady(t *testing.T) {
	_ = ConfigureClient("forever")
	t.Cleanup(func() { _ = ConfigureClient("retail") })
	now := time.Now()
	m := NewModel()
	m.ObserveForeverSpell(lookup.SpellMetadata{Build: "1.60.1.69913", Root: 17, Spell: 592, Name: "Power Word: Shield"})
	m.Observe(Cast{GUID: "Player-1-A", Spell: 592, At: now, Source: "unit_spellcast"}, now)
	m.Observe(Cast{GUID: "Player-1-A", Spell: 592, At: now.Add(time.Millisecond)}, now.Add(time.Second))
	snapshot := m.Snapshot(now.Add(time.Minute))
	if len(snapshot.Cooldowns) != 1 || snapshot.Cooldowns[0].Spell != 17 || snapshot.Cooldowns[0].Status != "unknown" {
		t.Fatal(snapshot.Cooldowns)
	}
	if InterruptSpell(119910) != 119910 {
		t.Fatal("Retail pet alias leaked")
	}
	if !IsInterrupt(8042) || IsInterrupt(57994) {
		t.Fatal("wrong interrupt families")
	}
}
