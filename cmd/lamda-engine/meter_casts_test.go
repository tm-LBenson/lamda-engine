package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestMeterIncreaseStartsTeammateCooldownAndOneSuccessfulUse(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	guid := "Player-1-BOB"
	m := engine.NewModel()
	in := meterInput{}
	s := &lookup.ScreenCasts{ID: "1:1", Version: 3, Captured: true, Meter: []lookup.MeterOwner{{GUID: guid, Serial: 1, Amount: 7}}}
	k := &framewire.KickSnapshot{Members: []framewire.KickMember{{GUID: guid, Class: 8, Spec: 64}}}
	tick := func(ms int, serial, amount int) {
		at := now.Add(time.Duration(ms) * time.Millisecond)
		k.At = at
		s.Meter[0].Serial = serial
		s.Meter[0].Amount = amount
		in.handle(m, s, k, at, map[string]bool{guid: true}, map[string]string{guid: "Bob"}, "")
	}
	tick(0, 1, 7)
	tick(250, 2, 8)
	if len(m.Snapshot(now).Cooldowns) != 0 {
		t.Fatal("history or single read became a kick")
	}
	tick(500, 3, 8)
	tick(750, 4, 8)
	state := m.Snapshot(now.Add(time.Second))
	if m.Diagnostics.MeterConfirmed != 1 || len(state.Cooldowns) != 1 || state.InterruptCounts[guid] != 1 || state.InterruptAttempts[guid] != 1 {
		t.Fatal(m.Diagnostics, state)
	}
	e := state.Cooldowns[0]
	if e.Source != "damage_meter" || e.Spell != 2139 || !e.EventAt.Equal(now.Add(250*time.Millisecond)) || e.EstimatedEnd <= float64(now.Unix())+10 {
		t.Fatal(e)
	}
	tick(1000, 5, 10)
	tick(1250, 6, 10)
	if m.Diagnostics.MeterConfirmed != 1 {
		t.Fatal("nearby multi-target result became another use")
	}
}
func TestMeterRebasesHistoryAcrossGapsResetsAndRosterChanges(t *testing.T) {
	for _, kind := range []string{"focus", "gap", "generation", "owner_missing", "roster", "decrease", "uncaptured", "frozen"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Now()
			guid := "Player-1-BOB"
			m := engine.NewModel()
			in := meterInput{}
			s := &lookup.ScreenCasts{ID: "1:1", Version: 3, Captured: true, Meter: []lookup.MeterOwner{{GUID: guid, Serial: 1, Amount: 7}}}
			k := &framewire.KickSnapshot{At: now, Members: []framewire.KickMember{{GUID: guid, Class: 8, Spec: 64}}}
			roster := map[string]bool{guid: true}
			in.handle(m, s, k, now, roster, nil, "")
			switch kind {
			case "focus":
				in.handle(m, nil, k, now.Add(time.Millisecond), roster, nil, "")
			case "gap":
				now = now.Add(2 * time.Second)
			case "generation":
				s.ID = "1:2"
			case "owner_missing":
				empty := *s
				empty.Meter = nil
				in.handle(m, &empty, k, now.Add(time.Millisecond), roster, nil, "")
			case "roster":
				in.handle(m, s, k, now.Add(time.Millisecond), map[string]bool{}, nil, "")
			case "decrease":
				s.Meter[0].Amount = 0
			case "uncaptured":
				s.Captured = false
				in.handle(m, s, k, now.Add(time.Millisecond), roster, nil, "")
				s.Captured = true
			}
			if kind != "decrease" {
				s.Meter[0].Amount = 8
			}
			for i := 1; i <= 3; i++ {
				at := now.Add(time.Duration(i) * 250 * time.Millisecond)
				k.At = at
				if kind != "frozen" {
					s.Meter[0].Serial = i + 1
				}
				in.handle(m, s, k, at, roster, nil, "")
			}
			if m.Diagnostics.MeterConfirmed != 0 || len(m.Snapshot(now).Cooldowns) > 0 {
				t.Fatal(kind, m.Diagnostics)
			}
		})
	}
}
func TestMeterDoesNotGuessMissingOrAmbiguousAbility(t *testing.T) {
	for _, kind := range []string{"missing", "stale", "preview", "shield", "beam"} {
		t.Run(kind, func(t *testing.T) {
			m := engine.NewModel()
			in := meterInput{}
			now := time.Now()
			guid := "Player-1-A"
			s := &lookup.ScreenCasts{Version: 3, ID: "1:1", Captured: true, Meter: []lookup.MeterOwner{{GUID: guid, Serial: 1}}}
			member := framewire.KickMember{GUID: guid, Class: 8, Spec: 64}
			if kind == "shield" {
				member.Class = 2
				member.Spec = 66
			}
			if kind == "beam" {
				member.Class = 11
				member.Spec = 102
			}
			k := &framewire.KickSnapshot{At: now, Members: []framewire.KickMember{member}, Preview: kind == "preview"}
			if kind == "missing" {
				k = nil
			}
			if kind == "stale" {
				k.At = now.Add(-5 * time.Second)
			}
			for i := 0; i < 3; i++ {
				s.Meter[0].Serial = i + 1
				if i > 0 {
					s.Meter[0].Amount = 1
				}
				in.handle(m, s, k, now.Add(time.Duration(i)*250*time.Millisecond), map[string]bool{guid: true}, nil, "")
			}
			if m.Diagnostics.MeterConfirmed != 0 {
				t.Fatal(kind)
			}
		})
	}
}
