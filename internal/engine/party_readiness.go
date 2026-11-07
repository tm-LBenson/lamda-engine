package engine

import (
	"fmt"
	"time"
)

type partyBuild struct {
	Build BuildSnapshot
	Since time.Time
}

// A current addon roster starts a bounded observation period. Once a known
// ability's full cooldown elapses, assume Ready unless a later use was observed.
func (m *Model) TrackPartyRoster(roster map[string]bool, at time.Time) {
	if Forever || roster == nil || at.IsZero() || len(roster) > 5 {
		return
	}
	if m.partyMembers == nil {
		m.partyMembers = map[string]time.Time{}
	}
	for guid := range m.partyMembers {
		if !roster[guid] {
			delete(m.partyMembers, guid)
			delete(m.partyBuilds, guid)
			m.clearPartySeeds(guid)
		}
	}
	for guid, present := range roster {
		if _, exists := m.partyMembers[guid]; present && !exists {
			m.partyMembers[guid] = at
			if b, ok := m.InspectedBuilds[guid]; ok && validInspectedBuild(b, m.Builds[guid], at) {
				m.rememberPartyBuild(b)
			}
		}
	}
}

// Like same-key estimates, this assumes no unobserved reuse. It never invents
// charge counts; Ready means at least one use should be available.
func (m *Model) keepPartyEvidence(e CooldownEvidence, now time.Time) bool {
	joined, present := m.partyMembers[e.GUID]
	if Forever || !present || e.EventAt.Before(joined) || e.EventAt.After(now) || e.BaselineSeconds <= 0 {
		return false
	}
	return e.Kind == "cast" || e.Kind == "activation" || e.Kind == "reset" || e.Kind == "party_ready"
}

func (m *Model) clearPartySeeds(guid string) {
	for key, e := range m.Evidence {
		if e.GUID == guid && e.Kind == "party_ready" {
			delete(m.Evidence, key)
		}
	}
}

// Cache verified ability ownership for the party, independently of the short
// inspection refresh TTL. Repeated inspection of the same build cannot slide
// the warm-up clock. A changed build must establish readiness again.
func (m *Model) rememberPartyBuild(b BuildSnapshot) {
	joined, member := m.partyMembers[b.GUID]
	if Forever || !member || b.Source != "inspect" || b.SpellRanks == nil || b.At.Before(joined.Add(-60*time.Second)) {
		return
	}
	if m.partyBuilds == nil {
		m.partyBuilds = map[string]partyBuild{}
	}
	p, exists := m.partyBuilds[b.GUID]
	if exists && b.At.Before(p.Build.At) {
		return
	}
	since := joined
	if exists {
		since = p.Since
		if p.Build.Revision != b.Revision {
			since = maxTime(joined, b.At)
			m.clearPartySeeds(b.GUID)
		}
	}
	m.partyBuilds[b.GUID] = partyBuild{Build: b, Since: since}
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (m *Model) seedPartyReady(now time.Time) {
	if Forever {
		return
	}
	for guid, p := range m.partyBuilds {
		b := p.Build
		if now.Before(p.Since) || now.Before(b.At) {
			continue
		}
		if !inspectionMatchesLog(b, m.Builds[guid]) {
			m.clearPartySeeds(guid)
			delete(m.partyBuilds, guid)
			continue
		}
		eligible := map[int]int{}
		for id, rank := range b.SpellRanks {
			eligible[id] = rank
		}
		if b.GameBuild == referenceRules.Game {
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
			if old, exists := m.Evidence[key]; exists {
				// A real in-party use always wins. Older evidence may still be
				// cooling down or contain a longer charge queue; preserve it.
				end := old.EstimatedEnd
				for _, recharge := range old.RechargeEnds {
					end = max(end, recharge)
				}
				if !old.EventAt.Before(p.Since) || end > float64(now.UnixMilli())/1000 {
					continue
				}
			}
			e := CooldownEvidence{GUID: guid, Player: b.Player, Spell: id, Name: spell.Name,
				Source: "party_observation", Kind: "party_ready", EventAt: p.Since, ReceivedAt: now,
				BaselineSeconds: spell.Cooldown, CooldownSeconds: spell.Cooldown,
				EstimatedEnd: float64(p.Since.UnixMilli())/1000 + spell.Cooldown, MayHaveCharges: spell.Charges,
				BuildRevision: b.Revision, BuildSource: b.Source,
				Reasons: []string{"full_cooldown_in_party", "current_cooldown_not_queried"}}
			e.PvP = m.pvpContext(now)
			if rawID == 414658 && !Forever {
				e.DisplaySpell, e.Name = 414658, "Ice Cold"
			}
			applyBuildRules(&e, b)
			if float64(now.UnixMilli())/1000 < e.EstimatedEnd {
				continue
			}
			m.Evidence[key] = e // No fabricated cast, Seen timestamp or charge count.
		}
	}
}
