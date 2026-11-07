package overlays

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func CastHash(w []string) string {
	if w == nil {
		w = []string{}
	}
	b, _ := json.Marshal(w)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}
func CastWatches(rules []Rule) []string {
	rules = executionRules(rules)
	seen := map[string]bool{}
	add := func(unit string, repeated cloneScope) {
		for _, u := range cloneUnits(repeated, unit) {
			if lookup.OverlayUnitToken(u) {
				seen[u] = true
			}
		}
	}
	pred := func(p map[string]any, repeated cloneScope) {
		walkPredicate(p, func(q map[string]any) {
			if str(q, "condition") == "casting" {
				add(str(q, "unit"), repeated)
			}
		})
	}
	var walk func([]Node, cloneScope)
	walk = func(ns []Node, repeated cloneScope) {
		for _, n := range ns {
			if n.Type == "if" {
				pred(n.Props, repeated)
			}
			if n.Type == "when" && (lookup.ValidCastEvent(str(n.Props, "event")) || str(n.Props, "event") == "casting") {
				add(str(n.Props, "unit"), repeated)
			}
			if n.Type == "show" && str(n.Props, "dataSource") == "casting" {
				add(str(n.Props, "dataUnit"), repeated)
			}
			walk(n.Children, childScope(n, repeated))
		}
	}
	for _, r := range rules {
		if r.Enabled {
			walk(r.Roots, 0)
			walk(valueInputNodes(r), ruleScopes(r.Roots)|playerScope)
			for _, m := range r.Style.Modifiers {
				pred(m.Condition, ruleScopes(r.Roots))
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
func validCastSpell(value string) bool {
	if value == "" {
		return true
	}
	id, e := strconv.Atoi(value)
	return e == nil && id > 0 && id < 10000000
}
func validCastFields(p map[string]any) bool {
	for _, k := range []string{"unit", "spell", "check", "operator"} {
		if v, ok := p[k]; ok {
			if _, ok := v.(string); !ok {
				return false
			}
		}
	}
	return true
}
func validCastWhen(p map[string]any) bool {
	return validCastFields(p) && (lookup.ValidCastEvent(str(p, "event")) || str(p, "event") == "casting") && validUnitChoice(str(p, "unit")) && validCastSpell(str(p, "spell"))
}
func validCasting(p map[string]any) bool {
	if !validCastFields(p) || !validUnitChoice(str(p, "unit")) || !validCastSpell(str(p, "spell")) {
		return false
	}
	if one(str(p, "check"), "active", "idle", "cast", "channel", "empower", "interruptible", "protected") {
		return true
	}
	return str(p, "check") == "remaining" && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, 86400)
}
func castFor(p *lookup.OverlayInput, unit string) *lookup.OverlayCast {
	for i := range p.Casts {
		if p.Casts[i].Unit == unit {
			return &p.Casts[i]
		}
	}
	return nil
}
func castUnit(p *lookup.OverlayInput, actor, unit string) string {
	if unit == "" {
		return "player"
	}
	return selectedUnit(boundUnit(p, actor), unit)
}
func castCondition(p map[string]any, input *lookup.OverlayInput, unit string) truth {
	c := castFor(input, castUnit(input, unit, str(p, "unit")))
	if c == nil || !c.Available || c.Mode == nil || c.Spell == nil {
		return unknown
	}
	check := str(p, "check")
	if *c.Mode == 0 {
		return truthOf(check == "idle")
	}
	if check == "idle" {
		id, _ := strconv.Atoi(str(p, "spell"))
		return truthOf(id > 0 && *c.Spell != id)
	}
	if id, _ := strconv.Atoi(str(p, "spell")); id > 0 && *c.Spell != id {
		return no
	}
	switch check {
	case "active":
		return yes
	case "cast":
		return truthOf(*c.Mode == 1)
	case "channel":
		return truthOf(*c.Mode == 2)
	case "empower":
		return truthOf(*c.Mode == 3)
	case "interruptible", "protected":
		if c.Interruptible == nil {
			return unknown
		}
		return truthOf(*c.Interruptible == (check == "interruptible"))
	case "remaining":
		if c.Remaining == nil {
			return unknown
		}
		return truthOf(compareNumber(*c.Remaining, num(p, "value"), str(p, "operator")))
	}
	return unknown
}
func (r *Runtime) validateCasts(p *lookup.OverlayInput, rules []Rule) {
	captured := []string{}
	known := map[string]bool{}
	valid := p.CastsVersion == 1 && p.Native
	for _, c := range p.Casts {
		if known[c.Unit] || !lookup.OverlayUnitToken(c.Unit) {
			valid = false
		}
		known[c.Unit] = true
		captured = append(captured, c.Unit)
	}
	if !valid || CastHash(captured) != p.CastHash {
		p.Casts = nil
		known = map[string]bool{}
	}
	for _, u := range CastWatches(rules) {
		if !known[u] && r.problem == "" {
			r.problem = "Reload WoW for cast tracking"
		}
	}
}

type triggerContext struct {
	Memory     *rememberState
	Logic      *Logic
	Loadout    string
	Boss       *lookup.OverlayBossEvent
	Encounter  *lookup.OverlayEncounterEvent
	Combat     *combatEvent
	Unit       string
	Generation int
	lookup.OverlayCastEvent
}
type castCursor struct{ Generation, Seq int }

func (r *Runtime) castEvents(p *lookup.OverlayInput, fresh bool) []triggerContext {
	out := []triggerContext{}
	for _, c := range p.Casts {
		cursor, known := r.castCursors[c.Unit]
		if fresh || !known || cursor.Generation != c.Generation {
			r.castCursors[c.Unit] = castCursor{c.Generation, c.Seq}
			continue
		}
		entries := append([]lookup.OverlayCastEvent(nil), c.Events...)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
		if c.Seq-cursor.Seq > 4 {
			cursor.Seq = c.Seq - 4
			if r.problem == "" {
				r.problem = "Some cast events were missed"
			}
		}
		for _, e := range entries {
			if e.Seq <= cursor.Seq {
				continue
			}
			if e.Spell == nil {
				break
			} // Retry a torn native tile while it is retained.
			out = append(out, triggerContext{Unit: c.Unit, Generation: c.Generation, OverlayCastEvent: e})
			cursor.Seq = e.Seq
		}
		if len(entries) == 0 {
			cursor.Seq = c.Seq
		}
		r.castCursors[c.Unit] = cursor
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Age != out[j].Age {
			return out[i].Age > out[j].Age
		}
		if out[i].Unit == out[j].Unit {
			return out[i].Seq < out[j].Seq
		}
		return out[i].Unit < out[j].Unit
	})
	return out
}
func (r *Runtime) castMatches(n Node, p *lookup.OverlayInput, unit string) bool {
	if !lookup.ValidCastEvent(str(n.Props, "event")) {
		return true
	}
	e := r.currentTrigger
	if e == nil || e.Unit != castUnit(p, unit, str(n.Props, "unit")) {
		return false
	}
	id, _ := strconv.Atoi(str(n.Props, "spell"))
	return id == 0 || e.Spell != nil && *e.Spell == id
}
func (r *Runtime) evaluateCasts(rules []Rule, p *lookup.OverlayInput, now time.Time, fresh bool) {
	for _, event := range r.castEvents(p, fresh) {
		r.currentTrigger = &event
		for _, rule := range rules {
			if rule.Enabled && loaded(rule.Load, p) {
				r.evaluate(rule, event.Kind, p, now.Add(-time.Duration(event.Age)*time.Millisecond))
			}
		}
	}
	r.currentTrigger = nil
}
