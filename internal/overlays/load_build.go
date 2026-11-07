package overlays

import "github.com/tm-LBenson/lamda-engine/internal/lookup"

// Load filters initially cover personal build and gear facts. Combat predicates
// belong in IF blocks until their capture and lifecycle contracts are supported.
func validBuildLoad(p map[string]any) bool {
	if p == nil {
		return true
	}
	n := 0
	if !validPredicate(p, 0, &n) {
		return false
	}
	ok := true
	walkPredicate(p, func(q map[string]any) {
		if !one(str(q, "condition"), "group", "talent", "learned", "equipment") {
			ok = false
		}
	})
	return ok
}
func buildLoadMatches(p map[string]any, input *lookup.OverlayInput) bool {
	if p == nil {
		return true
	}
	if str(p, "condition") == "group" {
		if children, ok := predicateChildren(p); ok && len(children) == 0 {
			return true
		}
	}
	return predicate(p, input, "player") == yes
}
