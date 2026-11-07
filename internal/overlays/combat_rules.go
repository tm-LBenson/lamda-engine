package overlays

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

var combatKinds = []string{"damage", "heal", "interrupt", "dispel", "dispelfailed", "auraapply", "auraremove", "aurarefresh", "aurastacks", "logcaststart", "logcastsuccess", "logcastfailed", "energize", "miss", "death", "despawn", "summon", "resurrect"}
var combatValues = []string{"amount", "effective", "baseAmount", "absorbed", "overheal", "overkill", "blocked", "resisted", "stacks", "wasted", "power", "critical"}

func validCombatWhen(p map[string]any) bool {
	for _, k := range []string{"combatEvent", "sourceUnit", "targetUnit", "spell", "extraSpell", "school", "critical", "periodic", "auraKind", "miss"} {
		if v, ok := p[k]; ok {
			if _, ok := v.(string); !ok {
				return false
			}
		}
	}
	unit := func(u string) bool { return one(u, "", "any") || validUnitChoice(u) }
	if !one(str(p, "combatEvent"), combatKinds...) || !unit(str(p, "sourceUnit")) || !unit(str(p, "targetUnit")) || !validCastSpell(str(p, "spell")) || !validCastSpell(str(p, "extraSpell")) {
		return false
	}
	if !one(str(p, "critical"), "", "any", "yes", "no") || !one(str(p, "periodic"), "", "any", "yes", "no") || !one(str(p, "auraKind"), "", "any", "buff", "debuff") || !one(str(p, "miss"), "", "any", "ABSORB", "BLOCK", "DEFLECT", "DODGE", "EVADE", "IMMUNE", "MISS", "PARRY", "REFLECT", "RESIST") {
		return false
	}
	if s := str(p, "school"); s != "" && s != "any" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 || n > 127 {
			return false
		}
	}
	return true
}
func validEventValue(p map[string]any) bool {
	return one(str(p, "field"), combatValues...) && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, 1e12)
}
func CombatWatches(rules []Rule) []string {
	rules = executionRules(rules)
	seen := map[string]bool{}
	var walk func([]Node, cloneScope)
	walk = func(nodes []Node, repeated cloneScope) {
		for _, n := range nodes {
			if n.Type == "when" && str(n.Props, "event") == "combatlog" {
				seen["player"] = true // The logger must be this captured character.
				for _, key := range []string{"sourceUnit", "targetUnit"} {
					if u := str(n.Props, key); !one(u, "", "any") {
						for _, u := range cloneUnits(repeated, u) {
							if lookup.OverlayUnitToken(u) {
								seen[u] = true
							}
						}
					}
				}
			}
			walk(n.Children, childScope(n, repeated))
		}
	}
	for _, r := range rules {
		if r.Enabled {
			walk(r.Roots, 0)
			walk(valueInputNodes(r), ruleScopes(r.Roots)|playerScope)
		}
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}
func identityFor(p *lookup.OverlayInput, unit string) string {
	for _, v := range p.Identities {
		if v.Unit == unit {
			return v.ID
		}
	}
	return ""
}
func (r *Runtime) validateCombat(p *lookup.OverlayInput, rules []Rule) {
	units := []string{}
	known := map[string]bool{}
	valid := p.CombatVersion == 1
	for _, v := range p.Identities {
		if known[v.Unit] || !lookup.OverlayUnitToken(v.Unit) {
			valid = false
		}
		known[v.Unit] = true
		units = append(units, v.Unit)
	}
	if !valid || CastHash(units) != p.IdentityHash {
		p.Identities = nil
		known = map[string]bool{}
	}
	wanted := CombatWatches(rules)
	for _, u := range wanted {
		if !known[u] && r.problem == "" {
			r.problem = "Reload WoW for combat events"
		}
	}
	if len(wanted) > 0 && r.problem == "" && p.Logging != nil && !*p.Logging {
		r.problem = "Combat logging is off"
	}
}
func combatMatches(q map[string]any, p *lookup.OverlayInput, actor string, e *combatEvent) bool {
	if e == nil || str(q, "combatEvent") != e.Kind {
		return false
	}
	for key, id := range map[string]string{"sourceUnit": e.Source.GUID, "targetUnit": e.Target.GUID} {
		if u := str(q, key); !one(u, "", "any") {
			guid := identityFor(p, selectedUnit(boundUnit(p, actor), u))
			if guid == "" || guid != id {
				return false
			}
		}
	}
	for key, id := range map[string]int{"spell": e.Spell, "extraSpell": e.ExtraSpell} {
		if wanted, _ := strconv.Atoi(str(q, key)); wanted > 0 && wanted != id {
			return false
		}
	}
	if s, _ := strconv.Atoi(str(q, "school")); s > 0 && (e.School&s) != s {
		return false
	}
	if c := str(q, "critical"); !one(c, "", "any") && (e.Critical == nil || *e.Critical != (c == "yes")) {
		return false
	}
	if periodic := str(q, "periodic"); !one(periodic, "", "any") && strings.Contains(e.RawKind, "_PERIODIC_") != (periodic == "yes") {
		return false
	}
	if kind := str(q, "auraKind"); !one(kind, "", "any") && kind != e.AuraKind {
		return false
	}
	if miss := str(q, "miss"); !one(miss, "", "any") && miss != e.Miss {
		return false
	}
	return true
}
func (r *Runtime) triggerMatches(n Node, p *lookup.OverlayInput, unit string) bool {
	if lookup.ValidLoadoutChange(str(n.Props, "event")) {
		return r.currentTrigger != nil && r.currentTrigger.Loadout == str(n.Props, "event")
	}
	if lookup.ValidBossEvent(str(n.Props, "event")) {
		return r.currentTrigger != nil && bossEventMatches(n.Props, p, r.currentTrigger.Boss)
	}
	if lookup.ValidEncounterEvent(str(n.Props, "event")) {
		return r.currentTrigger != nil && encounterMatches(n.Props, r.currentTrigger.Encounter)
	}
	if str(n.Props, "event") == "combatlog" {
		return r.currentTrigger != nil && combatMatches(n.Props, p, unit, r.currentTrigger.Combat)
	}
	return r.castMatches(n, p, unit)
}
func (r *Runtime) evaluateCombat(rules []Rule, p *lookup.OverlayInput, event *combatEvent, now time.Time) {
	r.currentTrigger = &triggerContext{Combat: event}
	defer func() { r.currentTrigger = nil }()
	for _, rule := range rules {
		if rule.Enabled && loaded(rule.Load, p) {
			r.evaluate(rule, "combatlog", p, now)
		}
	}
}
