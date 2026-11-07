package overlays

import (
	"strings"
	"unicode/utf8"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func validPetCondition(p map[string]any) bool {
	check := str(p, "check")
	if one(check, "present", "missing", "alive", "dead", "unavailable") {
		return true
	}
	v, ok := p["value"].(string)
	return one(check, "family", "specialization", "creatureType") && ok && strings.TrimSpace(v) != "" && len(v) <= 160 && utf8.ValidString(v) && strings.IndexFunc(v, func(r rune) bool { return r < 32 }) < 0
}

func petCondition(q map[string]any, p *lookup.OverlayInput) truth {
	pet := p.Pet
	if p.PetVersion != 1 || pet == nil || pet.Exists == nil {
		return unknown
	}
	check := str(q, "check")
	if pet.Revivable != nil && *pet.Revivable && one(check, "present", "missing", "alive", "dead", "unavailable") {
		return truthOf(one(check, "dead", "unavailable") || check == "present" && *pet.Exists)
	}
	if !*pet.Exists {
		return truthOf(one(check, "missing", "unavailable"))
	}
	switch check {
	case "present":
		return yes
	case "missing":
		return no
	case "alive", "dead", "unavailable":
		if pet.Alive == nil {
			return unknown
		}
		return truthOf(*pet.Alive == (check == "alive"))
	case "family", "specialization", "creatureType":
		v := map[string]string{"family": pet.Family, "specialization": pet.Specialization, "creatureType": pet.CreatureType}[check]
		if strings.TrimSpace(v) == "" {
			return unknown
		}
		return truthOf(strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(str(q, "value"))))
	}
	return unknown
}

func needsPet(rules []Rule) bool {
	needed := false
	pred := func(p map[string]any) {
		walkPredicate(p, func(q map[string]any) { needed = needed || str(q, "condition") == "pet" })
	}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == "if" {
				pred(n.Props)
			}
			walk(n.Children)
		}
	}
	for _, r := range executionRules(rules) {
		if r.Enabled {
			walk(r.Roots)
			walk(valueInputNodes(r))
			for _, m := range r.Style.Modifiers {
				pred(m.Condition)
			}
		}
	}
	return needed
}
