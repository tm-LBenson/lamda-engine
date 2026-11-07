package lookup

import (
	"strings"
	"unicode/utf8"
)

// OverlayPet is public context for the active main pet only. Nil is unknown,
// never a missing pet. Labels are localized strings supplied by the game.
type OverlayPet struct {
	Exists *bool `json:"exists,omitempty"`
	Alive  *bool `json:"alive,omitempty"`
	// Revivable preserves a confirmed hunter pet death after its unit/corpse
	// disappears. It is not inferred from a missing pet or missing health data.
	Revivable      *bool  `json:"revivable,omitempty"`
	Name           string `json:"name,omitempty"`
	Family         string `json:"family,omitempty"`
	CreatureType   string `json:"creatureType,omitempty"`
	Specialization string `json:"specialization,omitempty"`
}

func sanitizeOverlayPet(p *OverlayInput) {
	pet := p.Pet
	if p.PetVersion != 1 || pet == nil {
		p.Pet = nil
		return
	}
	if p.Class != "HUNTER" || pet.Exists == nil || pet.Alive != nil && *pet.Alive || pet.Revivable != nil && !*pet.Revivable {
		pet.Revivable = nil
	}
	for _, s := range []string{pet.Name, pet.Family, pet.CreatureType, pet.Specialization} {
		if len(s) > 160 || !utf8.ValidString(s) || strings.IndexFunc(s, func(r rune) bool { return r < 32 }) >= 0 {
			p.Pet = nil
			return
		}
	}
	if pet.Exists == nil || !*pet.Exists {
		// Do not retain a previous pet's labels while dismissed or unknown.
		p.Pet = &OverlayPet{Exists: pet.Exists, Revivable: pet.Revivable}
	}
}
