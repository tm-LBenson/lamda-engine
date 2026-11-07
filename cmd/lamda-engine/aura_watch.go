package main

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"time"
)

type auraTileState struct {
	initialized, active, pending bool
	since                        time.Time
}
type auraInput struct {
	key   string
	last  time.Time
	tiles [40]auraTileState
}

func (in *auraInput) handle(m *engine.Model, sample lookup.BridgeSample, now time.Time, names map[string]string, own string) {
	var a *lookup.AuraWatch
	if sample.Observations != nil {
		a = sample.Observations.Auras
	}
	if engine.Forever || m.DisabledDefensives || a == nil || !a.Captured {
		m.Diagnostics.AuraWatchedAbilities = 0
		if a != nil && !a.Captured {
			m.Diagnostics.AuraCaptureFailures++
		}
		*in = auraInput{}
		return
	}
	m.Diagnostics.AuraSamples++
	m.Diagnostics.AuraWatchedAbilities = 0
	key := fmt.Sprint(a.ID, "/", sample.Window, "/", a.Offset, "/", m.Diagnostics.SessionResets, "/", a.Members)
	if in.key != key || now.Before(in.last) || now.Sub(in.last) > time.Second {
		*in = auraInput{key: key, last: now}
	}
	in.last = now
	for _, member := range a.Members {
		for index, spell := range lookup.AuraWatchSpells[member.Class] {
			if member.Mask&(1<<index) == 0 {
				continue
			}
			m.Diagnostics.AuraWatchedAbilities++
			slot := member.Slot*8 + index
			state := &in.tiles[slot]
			active := a.Active[slot]
			if state.since.IsZero() || state.pending != active {
				state.pending = active
				state.since = now
				continue
			}
			if now.Sub(state.since) < 150*time.Millisecond {
				continue
			}
			if !state.initialized {
				state.initialized = true
				state.active = active
				if active {
					m.RecordGap("aura_onset_not_observed", now, engine.Cast{GUID: member.GUID, Spell: spell})
				}
				continue
			}
			if state.active == active {
				continue
			}
			state.active = active
			if active {
				m.Observe(engine.Cast{At: state.since, GUID: member.GUID, Name: names[member.GUID], Spell: spell,
					Self: member.GUID == own, Source: "aura_activation", RosterMatched: true}, now)
			}
		}
	}
}
