package engine

import (
	"fmt"
	"time"
)

// Builds establish provisional ability selection, never cooldown readiness.
// These presentation-only entries do not enter the observation model.
func knownCooldowns(s InferenceSnapshot, now time.Time) []CooldownEvidence {
	seen := map[string]bool{}
	for _, e := range s.Cooldowns {
		seen[fmt.Sprintf("%s/%d", e.GUID, e.Spell)] = true
	}
	local := map[string]BuildSnapshot{}
	for _, b := range s.Builds {
		if !b.At.After(now) && (now.Sub(b.At) <= observationRetention || s.retainsPartyBuild(b, now)) {
			local[b.GUID] = b
		}
	}
	result := []CooldownEvidence{}
	for guid, b := range local {
		if b.Source != "inspect" || b.SpellRanks == nil {
			continue
		}
		for id, rank := range b.SpellRanks {
			if !Forever && id == 45438 && b.SpellRanks[414658] > 0 {
				continue
			}
			spell, ok := trackedSpell(id)
			if IsInterrupt(id) {
				continue
			}
			canonical := CooldownSpell(id)
			key := fmt.Sprintf("%s/%d", guid, canonical)
			if !ok || rank <= 0 || seen[key] {
				continue
			}
			seen[key] = true
			display := 0
			if canonical != id {
				display = id
			}
			result = append(result, CooldownEvidence{GUID: guid, Player: b.Player, Spell: canonical, DisplaySpell: display, Name: spell.Name, Source: "inspect", Kind: "ability", Status: "unknown", BuildRevision: b.Revision, BuildSource: b.Source, Reasons: []string{"inspected_talent_selected", "cooldown_not_observed"}})
		}
	}
	for _, p := range s.PublicBuilds {
		if !p.Character.Valid() || p.FetchedAt.After(now) || now.Sub(p.FetchedAt) > 24*time.Hour {
			continue
		}
		b, hasLocal := local[p.Character.GUID]
		if hasLocal && b.Source == "inspect" && b.SpellRanks != nil {
			continue
		}
		for _, talent := range p.Talents {
			spell, ok := trackedSpell(talent.SpellID)
			if IsInterrupt(talent.SpellID) {
				continue
			}
			canonical := CooldownSpell(talent.SpellID)
			key := fmt.Sprintf("%s/%d", p.Character.GUID, canonical)
			if !ok || talent.Rank <= 0 || seen[key] {
				continue
			}
			if hasLocal {
				// Public data supplies the entry-to-spell mapping. Confirm the
				// entry against the newer local build, even across spec swaps.
				matched := false
				for _, entry := range b.Talents {
					if entry[0] == talent.NodeID && entry[1] == talent.EntryID && entry[2] > 0 {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			seen[key] = true
			result = append(result, CooldownEvidence{GUID: p.Character.GUID, Player: p.Character.Name,
				Spell: canonical, Name: spell.Name, Source: "public_build", Kind: "ability",
				Status: "unknown", BuildRevision: p.Revision, BuildSource: p.Source,
				Reasons: []string{"public_build_provisional", "cooldown_not_observed"}})
		}
	}
	return result
}
