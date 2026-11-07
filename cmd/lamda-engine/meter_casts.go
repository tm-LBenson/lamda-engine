package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"time"
)

type meterChange struct {
	amount    int
	first, at time.Time
}
type meterBaseline struct {
	serial, amount int
	last           time.Time
	pending        *meterChange
}
type meterInput struct {
	id     string
	owners map[string]*meterBaseline
}

func (in *meterInput) handle(m *engine.Model, s *lookup.ScreenCasts, kick *framewire.KickSnapshot, now time.Time, roster map[string]bool, names map[string]string, own string) {
	if engine.Forever || s == nil || !s.Captured || s.Version != 3 {
		*in = meterInput{}
		return
	}
	if in.id != s.ID {
		*in = meterInput{id: s.ID, owners: map[string]*meterBaseline{}}
	}
	members := map[string]framewire.KickMember{}
	if kick != nil && !kick.Preview && now.Sub(kick.At) <= 2*time.Second && !kick.At.After(now.Add(addonClockTolerance)) {
		for _, member := range kick.Members {
			members[member.GUID] = member
		}
	}
	seen := map[string]bool{}
	for _, owner := range s.Meter {
		if !roster[owner.GUID] || owner.Serial <= 0 || owner.AgeMillis < 0 || owner.AgeMillis > 1500 || owner.Amount < 0 {
			continue
		}
		seen[owner.GUID] = true
		b := in.owners[owner.GUID]
		if b == nil || now.Sub(b.last) > 1500*time.Millisecond || now.Before(b.last) || owner.Amount < b.amount {
			// First reads, focus gaps, resets and roster changes are history, not kicks.
			in.owners[owner.GUID] = &meterBaseline{serial: owner.Serial, last: now, amount: owner.Amount}
			m.Diagnostics.MeterRebases++
			continue
		}
		if b.serial == owner.Serial {
			continue
		}
		b.serial = owner.Serial
		b.last = now
		m.Diagnostics.MeterSamples++
		if owner.Amount <= b.amount {
			b.pending = nil
			continue
		}
		p := b.pending
		if p == nil || owner.Amount < p.amount {
			b.pending = &meterChange{amount: owner.Amount, first: now, at: now.Add(-time.Duration(owner.AgeMillis) * time.Millisecond)}
			continue
		}
		if now.Sub(p.first) < 150*time.Millisecond {
			continue
		}
		b.amount = owner.Amount
		b.pending = nil
		member, ok := members[owner.GUID]
		if !ok {
			m.RecordGap("meter_member_metadata_missing", now, engine.Cast{GUID: owner.GUID})
			continue
		}
		id := m.MeterInterruptSpell(member, now)
		if id == 0 {
			m.RecordGap("meter_interrupt_ability_ambiguous", now, engine.Cast{GUID: owner.GUID})
			continue
		}
		c := engine.Cast{GUID: owner.GUID, Name: names[owner.GUID], Spell: id, At: p.at, Source: "damage_meter", RosterMatched: true, Self: owner.GUID == own}
		if m.ObserveMeterInterrupt(c, now) {
			m.Diagnostics.MeterConfirmed++
		}
	}
	for guid := range in.owners {
		if !seen[guid] {
			delete(in.owners, guid)
		}
	}
}
