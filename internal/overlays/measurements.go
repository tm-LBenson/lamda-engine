package overlays

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"reflect"
	"sort"
	"strconv"
)

type MeasurementWatch struct {
	Unit     string `json:"unit"`
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
}

func (w MeasurementWatch) Key() string { return w.Unit + ":" + w.Kind + ":" + w.Resource }
func MeasurementHash(w []MeasurementWatch) string {
	if w == nil {
		w = []MeasurementWatch{}
	}
	b, _ := json.Marshal(w)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}
func MeasurementWatches(rules []Rule) []MeasurementWatch {
	rules = executionRules(rules)
	seen := map[string]MeasurementWatch{}
	add := func(unit, kind, resource string, repeated cloneScope) {
		if kind == "health" {
			resource = ""
		}
		if kind == "resource" && resource == "" {
			resource = "primary"
		}
		for _, u := range cloneUnits(repeated, unit) {
			if lookup.ValidMeasurement(u, kind, resource) {
				w := MeasurementWatch{u, kind, resource}
				seen[w.Key()] = w
			}
		}
	}
	predicate := func(p map[string]any, repeated cloneScope) {
		walkPredicate(p, func(q map[string]any) {
			kind := str(q, "condition")
			if !one(kind, "health", "resource") {
				return
			}
			unit := str(q, "unit")
			if unit == "" {
				unit = "current"
			}
			add(unit, kind, str(q, "resource"), repeated)
		})
	}
	var walk func([]Node, cloneScope)
	walk = func(ns []Node, repeated cloneScope) {
		for _, n := range ns {
			if n.Type == "if" {
				predicate(n.Props, repeated)
			}
			if n.Type == "show" {
				s := explicitSource(n.Props)
				if one(s.Kind, "health", "resource") {
					add(s.Unit, s.Kind, s.Resource, repeated)
				}
			}
			walk(n.Children, childScope(n, repeated))
		}
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		walk(r.Roots, 0)
		walk(valueInputNodes(r), ruleScopes(r.Roots)|playerScope)
		repeated := ruleScopes(r.Roots)
		for _, m := range r.Style.Modifiers {
			predicate(m.Condition, repeated)
		}
		a := r.Style.Arc
		if a != nil && hasArcs(r.Roots) && one(a.Fill, "health", "resource") {
			add(a.FillUnit, a.Fill, a.Resource, repeated)
		}
	}
	out := make([]MeasurementWatch, 0, len(seen))
	for _, w := range seen {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}
func selectedUnit(contextUnit, unit string) string {
	if unit == "" || unit == "current" {
		return contextUnit
	}
	return unit
}
func unitHealth(p *lookup.OverlayInput, unit string) *float64 {
	if unit == "player" {
		if p.Health != nil && bounded(*p.Health, 0, 100) {
			return p.Health
		}
		return nil
	}
	if p.MeasurementsVersion != 1 {
		return nil
	}
	for _, m := range p.Measurements {
		if m.Unit == unit && m.Kind == "health" && m.Available && m.Current != nil && m.Max != nil && *m.Max == 100 && bounded(*m.Current, 0, 100) {
			return m.Current
		}
	}
	return nil
}
func unitResource(p *lookup.OverlayInput, unit, key string) (lookup.OverlayResource, int, bool) {
	if key == "" {
		key = "primary"
	}
	if unit == "player" {
		if key == "primary" {
			key = strconv.Itoa(p.PrimaryPower)
		}
		power, _ := strconv.Atoi(key)
		v, ok := p.Resources[key]
		return v, power, ok && bounded(v.Max, 1, 0xfffffe) && bounded(v.Current, 0, v.Max)
	}
	if p.MeasurementsVersion == 1 {
		for _, m := range p.Measurements {
			if m.Unit == unit && m.Kind == "resource" && m.Resource == key && m.Available && m.Current != nil && m.Max != nil && bounded(*m.Max, 1, 0xfffffe) && bounded(*m.Current, 0, *m.Max) {
				return lookup.OverlayResource{Current: *m.Current, Max: *m.Max}, m.Power, true
			}
		}
	}
	return lookup.OverlayResource{}, -1, false
}

func validUnitChoice(unit string) bool {
	return unit == "" || unit == "current" || lookup.OverlayUnitToken(unit)
}
func measurementChanged(p, old *lookup.OverlayInput, kind string) bool {
	if old == nil {
		return true
	}
	selectKind := func(p *lookup.OverlayInput) []lookup.OverlayMeasurement {
		out := []lookup.OverlayMeasurement{}
		for _, m := range p.Measurements {
			if m.Kind == kind {
				out = append(out, m)
			}
		}
		return out
	}
	return !reflect.DeepEqual(selectKind(p), selectKind(old))
}
func (r *Runtime) validateMeasurements(p *lookup.OverlayInput, rules []Rule) {
	wanted := MeasurementWatches(rules)
	known := map[string]bool{}
	captured := []MeasurementWatch{}
	valid := p.MeasurementsVersion == 1
	for _, m := range p.Measurements {
		if known[m.Key()] || !lookup.ValidMeasurement(m.Unit, m.Kind, m.Resource) {
			valid = false
		}
		known[m.Key()] = true
		captured = append(captured, MeasurementWatch{m.Unit, m.Kind, m.Resource})
	}
	if !valid || MeasurementHash(captured) != p.MeasurementHash {
		p.Measurements = nil
		known = map[string]bool{}
	}
	for _, w := range wanted {
		if !known[w.Key()] {
			if r.problem == "" {
				r.problem = "Reload WoW for new unit readings"
			}
			break
		}
	}
	if !p.Native {
		p.Measurements = nil
		if len(wanted) > 0 && r.problem == "" {
			r.problem = "Native unit capture unavailable"
		}
	}
}

// Explicit token references must not retain a delayed/timed display when the
// token starts referring to a different unit. GUID-bound current clones keep
// their own identity and therefore survive harmless party slot changes.
func staticUnitDependencies(rule Rule) map[string]bool {
	out := map[string]bool{}
	add := func(unit string, repeated bool) {
		if unit == "current" {
			if repeated {
				return
			}
			unit = "player"
		}
		if unit != "" {
			out[unit] = true
		}
	}
	pred := func(p map[string]any, repeated bool) {
		walkPredicate(p, func(q map[string]any) {
			kind := str(q, "condition")
			if !one(kind, "health", "resource", "buff", "debuff", "casting") {
				return
			}
			u := str(q, "unit")
			if u == "" {
				u = "player"
				if one(kind, "health", "resource") {
					u = "current"
				}
			}
			add(u, repeated)
		})
	}
	var walk func([]Node, bool)
	walk = func(ns []Node, repeated bool) {
		for _, n := range ns {
			if n.Type == "when" && (lookup.ValidCastEvent(str(n.Props, "event")) || str(n.Props, "event") == "casting") {
				add(str(n.Props, "unit"), repeated)
			}
			if n.Type == "when" && str(n.Props, "event") == "combatlog" {
				for _, k := range []string{"sourceUnit", "targetUnit"} {
					if u := str(n.Props, k); !one(u, "", "any") {
						add(u, repeated)
					}
				}
			}
			if n.Type == "if" {
				pred(n.Props, repeated)
			}
			if n.Type == "show" {
				s := explicitSource(n.Props)
				if one(s.Kind, "health", "resource", "buff", "debuff", "casting") {
					add(s.Unit, repeated)
				}
			}
			walk(n.Children, repeated || n.Type == "each")
		}
	}
	walk(rule.Roots, false)
	walk(valueInputNodes(rule), hasEach(rule.Roots))
	repeated := hasEach(rule.Roots)
	for _, m := range rule.Style.Modifiers {
		pred(m.Condition, repeated)
	}
	if a := rule.Style.Arc; a != nil && hasArcs(rule.Roots) {
		if a.Repeat == "stacks" {
			add(a.Unit, repeated)
		}
		if one(a.Fill, "health", "resource") {
			add(a.FillUnit, repeated)
		}
	}
	return out
}
func (r *Runtime) clearChangedUnits(rules []Rule, p, old *lookup.OverlayInput) {
	if old == nil {
		return
	}
	generations := func(p *lookup.OverlayInput) map[string]int {
		out := map[string]int{}
		for _, a := range p.Watches {
			if a.Generation > 0 {
				out[a.Unit] = a.Generation
			}
		}
		for _, m := range p.Measurements {
			if m.Generation > 0 {
				out[m.Unit] = m.Generation
			}
		}
		return out
	}
	previous, current := generations(old), generations(p)
	for _, v := range old.Identities {
		previous[v.Unit] = v.Generation
	}
	for _, v := range p.Identities {
		current[v.Unit] = v.Generation
	}
	for _, c := range old.Casts {
		previous[c.Unit] = c.Generation
	}
	for _, c := range p.Casts {
		current[c.Unit] = c.Generation
	}
	changed := map[string]bool{}
	for u, g := range current {
		if previous[u] > 0 && previous[u] != g {
			changed[u] = true
		}
	}
	if len(changed) == 0 {
		return
	}
	for _, rule := range rules {
		for unit := range staticUnitDependencies(rule) {
			if changed[unit] {
				r.clearRule(rule.ID)
				break
			}
		}
	}
}
