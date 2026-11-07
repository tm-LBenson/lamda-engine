package engine

import (
	"fmt"
	"time"
)

// FreshKey records an actual challenge-start event and its original roster.
// Inserting a stone, countdowns, zoning and starting the engine mid-key do not
// establish this boundary. It is an initial estimate, not a cooldown query.
type FreshKey struct {
	At      time.Time
	Members map[string]bool
}

func (m *Model) BeginFreshKey(at, now time.Time, roster map[string]bool) bool {
	if at.IsZero() || at.After(now.Add(2*time.Second)) || now.Sub(at) > 30*time.Second || len(roster) == 0 || len(roster) > 5 {
		return false
	}
	if !m.lastFreshKeyAt.IsZero() && !at.After(m.lastFreshKeyAt.Add(2*time.Second)) {
		return false
	}
	members := map[string]bool{}
	for guid, present := range roster {
		if present {
			members[guid] = true
		}
	}
	if len(members) == 0 {
		return false
	}
	m.EndFreshKey()
	m.lastFreshKeyAt = at
	m.ResetInterruptCounts(at)
	m.FreshKey = &FreshKey{At: at, Members: members}
	return true
}

func (m *Model) EndFreshKey() {
	m.FreshKey = nil
	for key, e := range m.Evidence {
		if e.Kind == "key_start" {
			delete(m.Evidence, key)
		}
	}
}

// KeepKeyRoster only removes original members. A replacement or someone who
// leaves and rejoins must not inherit the run's ready assumption.
func (m *Model) KeepKeyRoster(roster map[string]bool) {
	if m.FreshKey == nil || roster == nil {
		return
	}
	for guid := range m.FreshKey.Members {
		if !roster[guid] {
			delete(m.FreshKey.Members, guid)
			for key, e := range m.Evidence {
				if e.GUID == guid && e.Kind == "key_start" {
					delete(m.Evidence, key)
				}
			}
		}
	}
	if len(m.FreshKey.Members) == 0 {
		m.EndFreshKey()
	}
}

// During a confirmed Retail key, retain our last observed state until another
// use or a context boundary. This is an estimate, never a current cooldown query
// or a claim that every intervening use was observable.
func (m *Model) keepKeyEvidence(e CooldownEvidence, now time.Time) bool {
	k := m.FreshKey
	if Forever || k == nil || !k.Members[e.GUID] || now.Before(k.At) || e.EventAt.After(now) || e.BaselineSeconds <= 0 {
		return false
	}
	if e.Kind != "cast" && e.Kind != "activation" && e.Kind != "reset" && e.Kind != "key_start" {
		return false
	}
	end := e.EstimatedEnd
	for _, recharge := range e.RechargeEnds {
		end = max(end, recharge)
	}
	// Also retain a cooldown already in progress when the key started. Old
	// expired evidence from before this key is not promoted into this run.
	return !e.EventAt.Before(k.At) || end > float64(k.At.UnixMilli())/1000
}

func (m *Model) seedFreshKey(now time.Time) {
	k := m.FreshKey
	if k == nil || now.Before(k.At) || now.Sub(k.At) > 45*time.Second {
		return
	}
	for guid := range k.Members {
		b, ok := m.buildFor(guid, now)
		// Inspection may finish just after the start; an old web profile or a
		// class/spec suggestion alone must never turn an icon into ready.
		if !ok || b.Source != "inspect" || b.SpellRanks == nil || b.At.Before(k.At.Add(-60*time.Second)) || b.At.After(now) {
			continue
		}
		eligible := map[int]int{}
		for id, rank := range b.SpellRanks {
			eligible[id] = rank
		}
		// In a confirmed Retail keystone, baseline class spells follow the
		// inspected specialization and the pinned game data. Optional talents
		// still require a selected rank; generic spec candidates never qualify.
		if !Forever && b.GameBuild == referenceRules.Game {
			for _, id := range specializationDefensives[b.SpecID] {
				if referenceRules.NonTalentSpells[id] {
					eligible[id] = 1
				}
			}
			if id := retailSpecInterrupts[b.SpecID]; id != 0 && referenceRules.NonTalentSpells[id] {
				eligible[id] = 1
			}
		}
		for rawID, rank := range eligible {
			id := CooldownSpell(rawID)
			if !Forever && rawID == 45438 && b.SpellRanks[414658] > 0 {
				continue
			}
			if rank <= 0 || (IsInterrupt(id) && m.DisabledInterrupts) || (!IsInterrupt(id) && m.DisabledDefensives) {
				continue
			}
			spell, known := trackedSpell(id)
			if !known || spell.Cooldown <= 0 {
				continue
			}
			key := fmt.Sprintf("%s/%d", guid, id)
			// Preserve casts before or after the boundary, including short CDs
			// which need not reset at key start. Never erase conflicting evidence.
			if _, exists := m.Evidence[key]; exists {
				continue
			}
			if cast, exists := m.Seen[key]; exists && !cast.Before(k.At) {
				continue
			}
			reason := "inspected_talent_selected"
			if b.SpellRanks[id] <= 0 {
				reason = "inspected_spec_baseline"
			}
			baseline := spell.Cooldown
			if !Forever && b.GameBuild == referenceRules.Game {
				if specBaseline := referenceRules.SpecializationBaselines[b.SpecID][id]; specBaseline > 0 {
					baseline = specBaseline
				}
			}
			m.Evidence[key] = CooldownEvidence{GUID: guid, Player: b.Player, Spell: id, Name: spell.Name,
				Source: "fresh_key", Kind: "key_start", EventAt: k.At, ReceivedAt: now,
				BaselineSeconds: baseline, CooldownSeconds: baseline,
				EstimatedEnd: float64(k.At.UnixMilli()) / 1000, MayHaveCharges: spell.Charges,
				BuildRevision: b.Revision, BuildSource: b.Source,
				Reasons: []string{reason, "fresh_key_ready_estimate", "current_cooldown_not_queried"}}
			if !Forever && rawID == 414658 {
				e := m.Evidence[key]
				e.DisplaySpell, e.Name = 414658, "Ice Cold"
				m.Evidence[key] = e
			}
		}
	}
}
