package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"time"
)

func needsLoadoutChange(rules []Rule, kind string) bool {
	rules = executionRules(rules)
	var walk func([]Node) bool
	walk = func(nodes []Node) bool {
		for _, n := range nodes {
			if n.Type == "when" && str(n.Props, "event") == kind || walk(n.Children) {
				return true
			}
		}
		return false
	}
	for _, r := range rules {
		if r.Enabled && (walk(r.Roots) || walk(valueInputNodes(r))) {
			return true
		}
	}
	return false
}
func loadoutChange(p *lookup.OverlayInput, kind string) *lookup.OverlayLoadoutChange {
	if p == nil || p.LoadoutChangesVersion != 1 {
		return nil
	}
	for i := range p.LoadoutChanges {
		if p.LoadoutChanges[i].Kind == kind {
			return &p.LoadoutChanges[i]
		}
	}
	return nil
}
func (r *Runtime) validateLoadoutChanges(p *lookup.OverlayInput, rules []Rule) {
	for _, kind := range []string{"talents", "equipment"} {
		if needsLoadoutChange(rules, kind) && loadoutChange(p, kind) == nil && r.problem == "" {
			r.problem = "Reload WoW for build and gear change events"
		}
	}
}
func (r *Runtime) evaluateLoadoutChanges(rules []Rule, p, old *lookup.OverlayInput, now time.Time, fresh bool) {
	if fresh {
		return
	}
	for _, kind := range []string{"talents", "equipment"} {
		current, prior := loadoutChange(p, kind), loadoutChange(old, kind)
		if current == nil || prior == nil || current.Seq <= prior.Seq || current.Age == nil {
			continue
		}
		r.currentTrigger = &triggerContext{Loadout: kind}
		for _, rule := range rules {
			if rule.Enabled && loaded(rule.Load, p) {
				r.evaluate(rule, kind, p, now.Add(-time.Duration(*current.Age)*time.Millisecond))
			}
		}
	}
	r.currentTrigger = nil
}
