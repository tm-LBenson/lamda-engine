package engine

import (
	"fmt"
	"time"
)

// These are possible abilities for a specialization, NOT a learned spellbook.
// No level, talent, resource, charge or readiness claim can follow from this list.
// It supplies dim '?' placeholders only when the player's public build cannot
// identify selected defensive spells. Observations always take priority.
var specializationDefensives = map[int][]int{
	250: {48707, 51052, 49028, 48792, 49039, 55233},
	251: {48707, 51052, 48792, 49039},
	252: {48707, 51052, 48792, 49039},
	577: {198589, 196718},
	581: {204021, 187827, 196718},
	102: {22812}, 103: {22812, 61336}, 104: {22812, 61336}, 105: {22812, 102342},
	1467: {363916, 374227}, 1468: {363916, 363534, 357170, 374227}, 1473: {363916, 374227},
	253: {186265, 109304, 264735}, 254: {186265, 109304, 264735}, 255: {186265, 109304, 264735},
	62: {110959, 45438, 55342, 342245}, 63: {110959, 45438, 55342, 342245}, 64: {110959, 45438, 55342, 342245},
	268: {115203}, 269: {115203, 122470}, 270: {115203, 116849},
	65: {1022, 6940, 498, 642, 633}, 66: {31850, 1022, 6940, 642, 86659, 633}, 70: {1022, 6940, 498, 642, 633},
	256: {19236, 33206, 62618}, 257: {19236, 47788}, 258: {19236, 47585},
	259: {31224, 5277}, 260: {31224, 5277}, 261: {31224, 5277},
	262: {108271}, 263: {108271}, 264: {108271, 98008},
	265: {108416, 104773}, 266: {108416, 104773}, 267: {108416, 104773},
	71: {118038, 97462, 23920, 202168}, 72: {184364, 97462, 23920, 202168},
	73: {871, 2565, 12975, 23920, 190456, 97462, 202168},
}

func candidateCooldowns(s InferenceSnapshot, now time.Time, own string) []CooldownEvidence {
	seen := map[string]bool{}
	for _, e := range s.Cooldowns {
		seen[fmt.Sprintf("%s/%d", e.GUID, e.Spell)] = true
	}
	for _, e := range knownCooldowns(s, now) {
		seen[fmt.Sprintf("%s/%d", e.GUID, e.Spell)] = true
	}
	result := []CooldownEvidence{}
	for _, b := range s.Builds {
		if b.GUID == "" || b.GUID == own || b.At.IsZero() || b.At.After(now) || (now.Sub(b.At) >= observationRetention && !s.retainsPartyBuild(b, now)) {
			continue
		}
		verifiedProfile := false
		for _, p := range s.PublicBuilds {
			if p.Character.GUID != b.GUID || !p.Character.Valid() || p.SpecID != b.SpecID || p.FetchedAt.IsZero() || p.FetchedAt.After(now) || now.Sub(p.FetchedAt) > 24*time.Hour {
				continue
			}
			// Match actual selected local entries. A stale public loadout alone
			// must neither suppress the fallback nor resurrect removed talents.
			for _, t := range p.Talents {
				if _, ok := Catalog[t.SpellID]; !ok || t.Rank <= 0 {
					continue
				}
				for _, entry := range b.Talents {
					if entry[0] == t.NodeID && entry[1] == t.EntryID && entry[2] > 0 {
						verifiedProfile = true
					}
				}
			}
		}
		if verifiedProfile && b.Source != "inspect" {
			continue
		}
		for _, id := range specializationDefensives[b.SpecID] {
			// Inspection establishes talent choices, not a teammate spellbook.
			// Keep possible baseline class abilities that have no talent node,
			// while suppressing unselected optional defenses from old profiles.
			if b.Source == "inspect" && b.SpellRanks != nil && !referenceRules.NonTalentSpells[id] {
				continue
			}
			key := fmt.Sprintf("%s/%d", b.GUID, id)
			spell, ok := Catalog[id]
			if !ok || seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, CooldownEvidence{GUID: b.GUID, Spell: id, Name: spell.Name,
				Source: "spec_candidate", Kind: "ability", Status: "unknown", BuildRevision: b.Revision,
				BuildSource: b.Source, Reasons: []string{"ability_not_verified", "cooldown_not_observed"}})
		}
	}
	return result
}
