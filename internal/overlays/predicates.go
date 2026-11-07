package overlays

import (
	"strconv"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Compound conditions preserve unknown values. NOT unknown is still unknown.
type truth int

const (
	unknown truth = iota
	no
	yes
)

func truthOf(v bool) truth {
	if v {
		return yes
	}
	return no
}
func predicateChildren(p map[string]any) ([]map[string]any, bool) {
	raw, ok := p["tests"].([]any)
	if !ok {
		return nil, false
	}
	result := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		child, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		result = append(result, child)
	}
	return result, true
}
func walkPredicate(p map[string]any, visit func(map[string]any)) {
	visit(p)
	if str(p, "condition") == "group" {
		children, _ := predicateChildren(p)
		for _, child := range children {
			walkPredicate(child, visit)
		}
	}
}
func validPredicate(p map[string]any, depth int, count *int) bool {
	*count++
	if depth > 4 || *count > 32 {
		return false
	}
	if str(p, "condition") == "group" {
		children, ok := predicateChildren(p)
		mode := str(p, "mode")
		if !ok || !one(mode, "all", "any", "not") || mode == "not" && len(children) > 1 {
			return false
		}
		for _, child := range children {
			if !validPredicate(child, depth+1, count) {
				return false
			}
		}
		return true
	}
	ok := false
	switch str(p, "condition") {
	case "value":
		ok = validValueCondition(p)
	case "equipment":
		ok = validEquipment(p)
	case "talent", "learned":
		ok = validTalent(p)
	case "bossTimer", "bossStage":
		ok = validBossCondition(p)
	case "encounter":
		ok = validEncounterCondition(p)
	case "pet":
		ok = validPetCondition(p)
	case "eventValue":
		ok = validEventValue(p)
	case "casting":
		ok = validCasting(p)
	case "cooldown":
		ok = validCooldown(p)
	case "role":
		ok = one(str(p, "value"), "tank", "healer", "damage")
	case "location":
		ok = one(str(p, "value"), "world", "dungeon", "raid", "arena", "pvp")
	case "health":
		if !validUnitChoice(str(p, "unit")) {
			return false
		}
		ok = bounded(num(p, "value"), 0, 100) && one(str(p, "operator"), "", "lt", "lte", "eq", "ne", "gt", "gte")
	case "resource":
		if !validUnitChoice(str(p, "unit")) {
			return false
		}
		power, err := strconv.Atoi(str(p, "resource"))
		ok = (str(p, "resource") == "primary" || (err == nil && power >= 0 && power < 32)) && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte", "cap") && one(str(p, "measure"), "amount", "percent")
		if str(p, "operator") != "cap" {
			limit := float64(0xfffffe)
			if str(p, "measure") == "percent" {
				limit = 100
			}
			ok = ok && bounded(num(p, "value"), 0, limit)
		}
	case "buff", "debuff":
		spell, e := strconv.Atoi(str(p, "aura"))
		ok = e == nil && spell > 0 && spell < 10000000 && validUnitChoice(str(p, "unit")) && one(str(p, "check"), "active", "missing", "stacks", "remaining")
		if one(str(p, "check"), "stacks", "remaining") {
			ok = ok && one(str(p, "operator"), "eq", "ne", "gt", "gte", "lt", "lte") && bounded(num(p, "value"), 0, 86400)
		}
	}
	return ok
}
func condition(n Node, p *lookup.OverlayInput, unit string, context ...*triggerContext) bool {
	return predicate(n.Props, p, unit, context...) == yes
}
func predicate(q map[string]any, p *lookup.OverlayInput, unit string, context ...*triggerContext) truth {
	if str(q, "condition") == "value" {
		var c *triggerContext
		if len(context) > 0 {
			c = context[0]
		}
		return valueCondition(q, p, unit, c)
	}
	if str(q, "condition") == "equipment" {
		return equipmentCondition(q, p)
	}
	if one(str(q, "condition"), "talent", "learned") {
		return talentCondition(q, p)
	}
	if one(str(q, "condition"), "bossTimer", "bossStage") {
		return bossCondition(q, p)
	}
	if str(q, "condition") == "encounter" {
		return encounterCondition(q, p)
	}
	if str(q, "condition") == "pet" {
		return petCondition(q, p)
	}
	if str(q, "condition") == "eventValue" {
		if len(context) == 0 || context[0] == nil || context[0].Combat == nil {
			return unknown
		}
		v, ok := context[0].Combat.Values[str(q, "field")]
		if !ok {
			return unknown
		}
		return truthOf(compareNumber(v, num(q, "value"), str(q, "operator")))
	}
	if str(q, "condition") == "casting" {
		return castCondition(q, p, unit)
	}
	if str(q, "condition") == "group" {
		children, ok := predicateChildren(q)
		if !ok || len(children) == 0 {
			return unknown
		}
		mode := str(q, "mode")
		hasUnknown := false
		for _, child := range children {
			v := predicate(child, p, unit, context...)
			if mode == "not" {
				if len(children) != 1 || v == unknown {
					return unknown
				}
				return truthOf(v == no)
			}
			if mode == "all" && v == no {
				return no
			}
			if mode == "any" && v == yes {
				return yes
			}
			if v == unknown {
				hasUnknown = true
			}
		}
		if hasUnknown {
			return unknown
		}
		return truthOf(mode == "all")
	}
	if !predicateKnown(q, p, unit) {
		return unknown
	}
	return truthOf(conditionLeaf(Node{Props: q}, p, unit))
}
func predicateKnown(q map[string]any, p *lookup.OverlayInput, unit string) bool {
	unit = boundUnit(p, unit)
	finite := func(n *float64) bool { return n != nil && bounded(*n, 0, 16777214) }
	switch str(q, "condition") {
	case "role":
		if unit == "player" {
			return one(p.Role, "tank", "healer", "damage")
		}
		m := unitMember(p, unit)
		return m != nil && one(m.Role, "tank", "healer", "damage")
	case "location":
		return one(p.Location, "world", "dungeon", "raid", "arena", "pvp")
	case "health":
		return unitHealth(p, selectedUnit(unit, str(q, "unit"))) != nil
	case "resource":
		_, _, ok := unitResource(p, selectedUnit(unit, str(q, "unit")), str(q, "resource"))
		return ok
	case "buff", "debuff":
		id, _ := strconv.Atoi(str(q, "aura"))
		u := str(q, "unit")
		if u == "" {
			u = "player"
		}
		if u == "current" {
			u = unit
		}
		for _, a := range p.Watches {
			if a.Spell != id || a.Kind != str(q, "condition") || a.Unit != u {
				continue
			}
			if !a.Available || a.Active == nil {
				return false
			}
			if one(str(q, "check"), "active", "missing") || !*a.Active {
				return true
			}
			if str(q, "check") == "remaining" {
				return !a.Permanent && finite(a.Remaining)
			}
			return finite(a.Stacks)
		}
	case "cooldown":
		id, _ := strconv.Atoi(str(q, "id"))
		check := str(q, "check")
		for _, c := range p.Cooldowns {
			if c.ID != id || c.Kind != str(q, "source") {
				continue
			}
			if !c.Available {
				return false
			}
			if one(check, "ready", "cooldown", "usable", "unusable") {
				if c.Enabled == nil {
					return false
				}
				if !*c.Enabled {
					return true
				}
				if !finite(c.Remaining) || !finite(c.MaxCharges) || *c.MaxCharges > 0 && !finite(c.Charges) {
					return false
				}
				return one(check, "ready", "cooldown") || c.Usable != nil
			}
			if check == "remaining" {
				return finite(c.Remaining)
			}
			if !finite(c.MaxCharges) {
				return false
			}
			if *c.MaxCharges == 0 {
				return false
			}
			if check == "recharge" {
				return finite(c.Recharge)
			}
			return finite(c.Charges)
		}
	}
	return false
}
