package engine

import (
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"strconv"
	"strings"
	"time"
)

// Retail 12.1.0.69814 spell/category cooldowns extracted from SimC's generated
// game data. Keep these separate: LamdaCD must never inherit interrupt icons.
var InterruptCatalog = map[int]Spell{
	1766:   {Name: "Kick", Cooldown: 15},
	2139:   {Name: "Counterspell", Cooldown: 25},
	6552:   {Name: "Pummel", Cooldown: 15},
	15487:  {Name: "Silence", Cooldown: 30},
	19647:  {Name: "Spell Lock", Cooldown: 24},
	47528:  {Name: "Mind Freeze", Cooldown: 15},
	57994:  {Name: "Wind Shear", Cooldown: 12},
	78675:  {Name: "Solar Beam", Cooldown: 60},
	89766:  {Name: "Axe Toss", Cooldown: 30},
	96231:  {Name: "Rebuke", Cooldown: 15},
	106839: {Name: "Skull Bash", Cooldown: 15},
	116705: {Name: "Spear Hand Strike", Cooldown: 15},
	147362: {Name: "Counter Shot", Cooldown: 24},
	183752: {Name: "Disrupt", Cooldown: 15},
	187707: {Name: "Muzzle", Cooldown: 15},
	351338: {Name: "Quell", Cooldown: 20},
}

func InterruptSpell(id int) int {
	if Forever {
		if root, ok := foreverAliases[id]; ok {
			return root
		}
		return id
	}
	switch id {
	// Retail combat logs report the hit under these effect IDs. Skull Bash
	// also emits an effect cast alongside the player's main cast.
	case 93985:
		return 106839
	case 220543:
		return 15487
	case 119910:
		return 19647
	case 119914, 347008:
		return 89766
	case 195439:
		return 183752
	}
	return id
}
func IsInterrupt(id int) bool { _, ok := InterruptCatalog[InterruptSpell(id)]; return ok }
func trackedSpell(id int) (Spell, bool) {
	if s, ok := Catalog[CooldownSpell(id)]; ok {
		if !Forever && id == 414658 {
			s.Name = "Ice Cold"
		}
		return s, true
	}
	s, ok := InterruptCatalog[InterruptSpell(id)]
	return s, ok
}

// The current public pet GUID establishes ownership; pet names and friendly
// flags do not. Never apply a new pet-owner mapping to a delayed old cast.
func (m *Model) IngestTeam(line string, now time.Time, companions bool, roster map[string]bool, kick *framewire.KickSnapshot, received time.Time) bool {
	if (strings.Contains(line, "SPELL_CAST_SUCCESS,Pet-") || strings.Contains(line, "SPELL_INTERRUPT,Pet-")) && kick != nil && kick.Mode > 0 && now.Sub(received) >= 0 && now.Sub(received) <= FrameLifetime && now.Sub(kick.At) >= -2*time.Second && now.Sub(kick.At) <= 3*time.Second {
		at, fields, err := parseRecord(line)
		if err == nil && len(fields) >= 12 && (fields[0] == "SPELL_CAST_SUCCESS" || fields[0] == "SPELL_INTERRUPT") && strings.HasPrefix(fields[1], "Pet-") {
			for _, member := range kick.Members {
				if member.Class != 9 || member.Pet != fields[1] || (member.GUID != kick.Player && !roster[member.GUID]) || at.Before(kick.At.Add(-2*time.Second)) {
					continue
				}
				id := 0
				// Parse the ID without accepting an arbitrary owner-derived spell.
				for _, candidate := range []int{19647, 119910, 89766, 119914, 347008} {
					if fields[9] == strconv.Itoa(candidate) {
						id = candidate
						break
					}
				}
				if id != 0 {
					m.Diagnostics.Lines++
					if fields[0] == "SPELL_INTERRUPT" {
						return m.countInterrupt(Cast{At: at, GUID: member.GUID, Spell: id}, fields, now)
					}
					return m.Observe(Cast{At: at, GUID: member.GUID, Name: member.Name, Spell: id, Self: member.GUID == kick.Player, RosterMatched: true}, now)
				}
			}
		}
	}
	return m.IngestWithRoster(line, now, companions, roster)
}
