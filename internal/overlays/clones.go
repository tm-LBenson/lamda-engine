package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"sort"
	"strings"
)

var partyTokens = lookup.OverlayScopeTokens("party")

type cloneScope uint8

const playerScope cloneScope = 128

var scopeNames = []string{"party", "group", "raid", "boss", "arena", "nameplates"}

func scopeMask(name string) cloneScope {
	if name == "" {
		name = "party"
	}
	for i, s := range scopeNames {
		if s == name {
			return 1 << i
		}
	}
	return 0
}
func childScope(n Node, inherited cloneScope) cloneScope {
	if n.Type == "each" {
		return scopeMask(str(n.Props, "scope"))
	}
	return inherited
}
func hasEach(ns []Node) bool {
	for _, n := range ns {
		if n.Type == "each" || hasEach(n.Children) {
			return true
		}
	}
	return false
}
func ruleScopes(ns []Node) cloneScope {
	var result cloneScope
	var walk func([]Node, cloneScope)
	walk = func(ns []Node, current cloneScope) {
		for _, n := range ns {
			if n.Type == "each" {
				result |= scopeMask(str(n.Props, "scope"))
			}
			if current == 0 && one(n.Type, "show", "glow", "sound", "hide") {
				result |= playerScope
			}
			walk(n.Children, childScope(n, current))
		}
	}
	walk(ns, 0)
	return result
}
func UnitTokens(rs []Rule) []string {
	seen := map[string]bool{}
	for _, r := range executionRules(rs) {
		if r.Enabled {
			scope := ruleScopes(r.Roots) &^ playerScope
			for _, u := range cloneUnits(scope, "current") {
				if scope != 0 {
					seen[u] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}
func usesClones(rs []Rule) bool { return len(UnitTokens(rs)) > 0 }
func cloneUnits(scopes cloneScope, unit string) []string {
	if unit != "current" {
		if unit == "" {
			unit = "player"
		}
		return []string{unit}
	}
	if scopes == 0 {
		return []string{"player"}
	}
	out := []string{}
	seen := map[string]bool{}
	add := func(u string) {
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	if scopes&playerScope != 0 {
		add("player")
	}
	for i, s := range scopeNames {
		if scopes&(1<<i) != 0 {
			for _, u := range lookup.OverlayScopeTokens(s) {
				add(u)
			}
		}
	}
	return out
}
func cloneActor(scope string, u lookup.OverlayUnit) string {
	if scope == "" {
		scope = "party"
	}
	if scope == "party" && strings.HasPrefix(u.ID, "Player-") {
		return u.ID
	}
	return scope + "@" + u.ID
}
func boundUnit(p *lookup.OverlayInput, actor string) string {
	if actor == "" {
		return "player"
	}
	if lookup.OverlayUnitToken(actor) {
		return actor
	}
	scope, id := "party", actor
	if i := strings.IndexByte(actor, '@'); i >= 0 {
		scope, id = actor[:i], actor[i+1:]
		if !lookup.OverlayCloneScope(scope) || scope == "" {
			return ""
		}
	}
	if p.UnitsVersion == 1 || p.UnitsVersion == 2 {
		for _, u := range p.Units {
			if u.ID == id && lookup.OverlayScopeContains(scope, u.Unit, p.Group) {
				return u.Unit
			}
		}
	}
	return ""
}
func unitMember(p *lookup.OverlayInput, actor string) *lookup.OverlayUnit {
	token := boundUnit(p, actor)
	if token == "" {
		return nil
	}
	if p.UnitsVersion == 1 || p.UnitsVersion == 2 {
		for i, u := range p.Units {
			if u.Unit == token {
				return &p.Units[i]
			}
		}
	}
	return nil
}
func cloneMembers(p *lookup.OverlayInput, scope string) []lookup.OverlayUnit {
	if p.UnitsVersion != 1 && p.UnitsVersion != 2 {
		return nil
	}
	actual := scope
	if actual == "group" {
		if p.Group == "raid" {
			actual = "raid"
		} else {
			actual = "party"
		}
	}
	out := []lookup.OverlayUnit{}
	seen := map[string]bool{}
	for _, token := range lookup.OverlayScopeTokens(actual) {
		for _, u := range p.Units {
			if u.Unit == token && !seen[u.ID] {
				out = append(out, u)
				seen[u.ID] = true
				break
			}
		}
	}
	return out
}
func unitsCaptured(p *lookup.OverlayInput, rs []Rule) bool {
	wanted := UnitTokens(rs)
	if len(wanted) == 0 {
		return true
	}
	tokens := p.UnitTokens
	if p.UnitsVersion == 1 {
		tokens = partyTokens
	} else if p.UnitsVersion != 2 {
		return false
	}
	have := map[string]bool{}
	for _, u := range tokens {
		have[u] = true
	}
	for _, u := range wanted {
		if !have[u] {
			return false
		}
	}
	return true
}
func boundSource(s displaySource, p *lookup.OverlayInput, actor string) displaySource {
	if s.Unit == "current" {
		s.Unit = boundUnit(p, actor)
		if s.Unit == "" {
			s.Unit = "unavailable"
		}
	}
	return s
}
