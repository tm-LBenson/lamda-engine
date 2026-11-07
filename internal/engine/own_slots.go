package engine

import (
	"slices"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

func ownSlotIndex(slots []framewire.CooldownSlot, spell int) int {
	for i, slot := range slots {
		if slices.Contains(slot.Spells, spell) {
			return i
		}
	}
	return -1
}

// Learned, selected own spells may supply an unknown fallback slot without a
// public profile. Slot metadata says nothing about timing, charges or readiness.
func ownSlotCooldowns(evidence []CooldownEvidence, geometry *framewire.Snapshot) []CooldownEvidence {
	seen := map[int]bool{}
	for _, e := range evidence {
		if e.GUID == geometry.Player {
			seen[e.Spell] = true
		}
	}
	result := []CooldownEvidence{}
	for _, frame := range geometry.Frames {
		if frame.GUID != geometry.Player {
			continue
		}
		for _, slot := range frame.SelfSlots {
			known := false
			for _, id := range slot.Spells {
				known = known || seen[id] || slices.Contains(frame.NativeSpells, id)
			}
			if known {
				continue
			}
			for _, id := range slot.Spells {
				if spell, ok := Catalog[id]; ok {
					result = append(result, CooldownEvidence{GUID: geometry.Player, Spell: id, Name: spell.Name,
						Self: true, Source: "player_spellbook", Kind: "ability", Status: "unknown",
						Reasons: []string{"cooldown_not_observed"}})
					for _, alias := range slot.Spells {
						seen[alias] = true
					}
					break
				}
			}
		}
	}
	return result
}
