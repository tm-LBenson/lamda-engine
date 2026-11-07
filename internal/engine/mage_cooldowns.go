package engine

import (
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Ice Cold replaces Ice Block and spends the same recharge category. Keep one
// evidence/charge queue while retaining the observed variant for presentation.
func CooldownSpell(id int) int {
	if !Forever && id == 414658 {
		return 45438
	}
	return InterruptSpell(id)
}

type partyContext struct {
	epoch                 string
	arena                 bool
	pvp                   bool
	since, seen, sampleAt time.Time
}

// Only fresh public addon context can establish PvP rules or a new arena round.
// A delayed combat-log zone marker cannot reset a newer live observation.
func (m *Model) ObservePartyContext(c lookup.PartyContext, now time.Time) {
	if Forever || now.Sub(c.At) > FrameTimestampSkew || c.At.After(now.Add(FrameTimestampSkew)) || c.At.Before(m.context.sampleAt) {
		return
	}
	if m.context.epoch != c.Epoch || m.context.arena != c.Arena || m.context.pvp != (c.PvP || c.Arena) {
		// Cast ages are anchored to local receipt. A server clock ahead of this
		// machine must not make a valid current cast look older than the round.
		boundary := c.At
		if boundary.After(now) {
			boundary = now
		}
		m.context = partyContext{epoch: c.Epoch, arena: c.Arena, pvp: c.PvP || c.Arena, since: boundary}
		if c.Arena {
			for key, e := range m.Evidence {
				if e.EventAt.Before(boundary) {
					delete(m.Evidence, key)
					delete(m.Rows, key)
					delete(m.Seen, key)
				}
			}
			for guid, b := range m.partyBuilds {
				b.Since = maxTime(b.Since, boundary)
				m.partyBuilds[guid] = b
			}
		}
	}
	m.context.seen, m.context.sampleAt = now, c.At
}

func (m *Model) pvpContext(at time.Time) *bool {
	if m.context.seen.IsZero() || at.Before(m.context.since.Add(-2*time.Second)) || at.After(m.context.seen.Add(3*time.Second)) {
		return nil
	}
	value := m.context.pvp
	return &value
}

// Reviewed against the pinned 12.1.5.70391 spell dump, separately from the
// older all-class rules. Do not enable that entire older ruleset for 12.1.5.
func applyMageBuildRules(e *CooldownEvidence, b BuildSnapshot) bool {
	if Forever || b.Source != "inspect" || b.SpellRanks == nil || b.GameBuild != "12.1.5" ||
		(e.Spell != 45438 && e.Spell != 342245) {
		return false
	}
	seconds, capacity := 0.0, 1
	apply := func(id, maximum int, amount float64) {
		if rank := b.SpellRanks[id]; rank > 0 && rank <= maximum {
			seconds += amount * float64(rank)
			e.AppliedTalents = append(e.AppliedTalents, id)
		}
	}
	if e.Spell == 45438 {
		if e.PvP != nil {
			amount := -30.0
			if *e.PvP {
				amount = -15
			} // Winter's Protection has a 0.5 PvP coefficient.
			apply(382424, 2, amount)
		} else {
			e.Reasons = append(e.Reasons, "pvp_context_unknown")
		}
		apply(1244110, 1, -30)
		if b.SpellRanks[1244110] == 1 {
			capacity++
		}
		apply(1265517, 1, -30)
	} else {
		apply(342249, 2, -5)
		apply(1255166, 1, -10)
	}
	e.MaxCharges = &capacity
	e.MayHaveCharges = capacity > 1
	e.CooldownSeconds = e.BaselineSeconds + seconds
	e.EstimatedEnd = float64(e.EventAt.UnixMilli())/1000 + e.CooldownSeconds
	if len(e.AppliedTalents) > 0 {
		e.Reasons = append(e.Reasons, "selected_talent_modifiers")
	}
	e.Reasons = append(e.Reasons, "dynamic_modifiers_unmodeled", "pvp_talents_unmodeled")
	filtered := e.Reasons[:0]
	for _, reason := range e.Reasons {
		if reason != "unmapped_build_modifiers" {
			filtered = append(filtered, reason)
		}
	}
	e.Reasons = filtered
	return true
}
