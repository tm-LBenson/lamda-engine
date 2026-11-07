package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestAuraOnsetStartsDuringEffectAndLogConfirmsWithoutRestart(t *testing.T) {
	now := time.Unix(1790000000, 0)
	m := engine.NewModel()
	in := observationInput{}
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Realm", Region: "us"}
	a := &lookup.AuraWatch{ID: "1:1", Captured: true, Members: []lookup.AuraWatchMember{{Slot: 1, Member: 1, Class: 6, Mask: 1, GUID: member.GUID}}}
	s := lookup.BridgeSample{Window: 123, Event: lookup.Event{ID: "1:1", Kind: "roster", Members: []lookup.Character{member}}, Observations: &lookup.Observations{Auras: a}}
	sample := func(ms int, active bool) {
		at := now.Add(time.Duration(ms) * time.Millisecond)
		s.Event.At = at
		a.Active[8] = active
		in.handle(m, s, at)
	}
	sample(0, false)
	sample(200, false)
	sample(400, true)
	sample(600, true)
	key := member.GUID + "/48707"
	e := m.Evidence[key]
	if e.Source != "aura_activation" || e.EstimatedEnd != float64(now.Unix())+60.4 || m.Diagnostics.AuraActivations != 1 {
		t.Fatal(e, m.Diagnostics)
	}
	sample(800, true)
	sample(1000, true)
	if m.Diagnostics.AuraActivations != 1 {
		t.Fatal("active ticks restart timer")
	}
	// Buff disappearance cannot reset or start the ability's recharge.
	sample(1200, false)
	sample(1400, false)
	if m.Evidence[key].EstimatedEnd != e.EstimatedEnd {
		t.Fatal("end-of-buff timer restart")
	}
	m.Observe(engine.Cast{GUID: member.GUID, Spell: 48707, At: now.Add(300 * time.Millisecond)}, now.Add(20*time.Second))
	e = m.Evidence[key]
	if e.Source != "combat_log" || e.EstimatedEnd != float64(now.Unix())+60.3 || m.Diagnostics.ReconciledAuraCasts != 1 || m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal(e, m.Diagnostics)
	}
}

func TestAuraMidBuffReconnectRosterChangeAndCorruptionDoNotInventCasts(t *testing.T) {
	now := time.Unix(1790000000, 0)
	for _, interruption := range []string{"gap", "mapping", "invalid", "session"} {
		m := engine.NewModel()
		in := auraInput{}
		a := &lookup.AuraWatch{ID: "1:1", Captured: true, Members: []lookup.AuraWatchMember{{Slot: 0, Class: 6, Mask: 1, GUID: "Player-1-BOB"}}}
		s := lookup.BridgeSample{Window: 123, Observations: &lookup.Observations{Auras: a}}
		at := now
		feed := func(active bool) {
			a.Active[0] = active
			in.handle(m, s, at, map[string]string{}, "")
			at = at.Add(200 * time.Millisecond)
		}
		feed(true)
		feed(true)
		feed(true)
		if len(m.Evidence) != 0 {
			t.Fatal("already-active buff invented cast")
		}
		feed(false)
		feed(false)
		switch interruption {
		case "gap":
			at = at.Add(2 * time.Second)
		case "mapping":
			a.ID = "1:2"
		case "invalid":
			a.Captured = false
			feed(false)
			a.Captured = true
		case "session":
			m.Reset()
		}
		feed(true)
		feed(true)
		if len(m.Evidence) != 0 {
			t.Fatal("discontinuous observation invented cast", interruption)
		}
	}
}

func TestAuraDoesNotSpendSecondChargeOrOverrideRealCast(t *testing.T) {
	now := time.Unix(1790000000, 0)
	m := engine.NewModel()
	guid := "Player-1-BOB"
	m.Observe(engine.Cast{GUID: guid, Spell: 363916, At: now, Source: "unit_spellcast"}, now)
	m.Observe(engine.Cast{GUID: guid, Spell: 363916, At: now.Add(time.Second), Source: "aura_activation"}, now.Add(time.Second))
	if m.Diagnostics.AcceptedCasts != 1 || m.Evidence[guid+"/363916"].Source != "unit_spellcast" {
		t.Fatal(m.Diagnostics)
	}
	// Keep a known multi-charge queue even after its first recharge ends.
	e := m.Evidence[guid+"/363916"]
	e.EstimatedEnd = float64(now.Unix()) + 90
	e.RechargeEnds = []float64{float64(now.Unix()) + 90, float64(now.Unix()) + 180}
	m.Evidence[guid+"/363916"] = e
	m.Observe(engine.Cast{GUID: guid, Spell: 363916, At: now.Add(100 * time.Second), Source: "aura_activation"}, now.Add(100*time.Second))
	if m.Evidence[guid+"/363916"].Source != "unit_spellcast" || len(m.Evidence[guid+"/363916"].RechargeEnds) != 2 {
		t.Fatal("buff onset discarded a known charge queue")
	}
	m = engine.NewModel()
	m.Observe(engine.Cast{GUID: guid, Spell: 48707, At: now, Source: "aura_activation"}, now)
	m.ResetLog(now.Add(-time.Second))
	if len(m.Evidence) != 1 {
		t.Fatal("delayed log boundary erased live buff onset")
	}
	if m.Snapshot(now.Add(time.Minute)).Cooldowns[0].Status != "unknown" {
		t.Fatal("unconfirmed buff implies ready")
	}
}
