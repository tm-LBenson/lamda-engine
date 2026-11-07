package overlays

import (
	"sort"
	"strconv"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func validEncounterFields(p map[string]any) bool {
	for _, key := range []string{"encounterID", "difficulty", "outcome", "check", "operator"} {
		if v, ok := p[key]; ok {
			if _, ok := v.(string); !ok {
				return false
			}
		}
	}
	if !validCastSpell(str(p, "encounterID")) {
		return false
	}
	if d := str(p, "difficulty"); d != "" {
		v, e := strconv.Atoi(d)
		if e != nil || v < 1 || v > 1000 {
			return false
		}
	}
	return true
}
func validEncounterWhen(p map[string]any) bool {
	return lookup.ValidEncounterEvent(str(p, "event")) && validEncounterFields(p) && one(str(p, "outcome"), "", "any", "victory", "wipe") && (str(p, "event") == "encounterend" || one(str(p, "outcome"), "", "any"))
}
func validEncounterCondition(p map[string]any) bool {
	return validEncounterFields(p) && (one(str(p, "check"), "active", "idle") || str(p, "check") == "elapsed" && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, 86400))
}
func needsEncounters(rules []Rule) bool {
	rules = executionRules(rules)
	needed := false
	pred := func(p map[string]any) {
		walkPredicate(p, func(q map[string]any) {
			if str(q, "condition") == "encounter" {
				needed = true
			}
		})
	}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == "if" {
				pred(n.Props)
			}
			if n.Type == "when" && lookup.ValidEncounterEvent(str(n.Props, "event")) || n.Type == "show" && str(n.Props, "dataSource") == "encounter" {
				needed = true
			}
			walk(n.Children)
		}
	}
	for _, r := range rules {
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
func encounterCondition(p map[string]any, input *lookup.OverlayInput) truth {
	e := input.Encounter
	if input.EncountersVersion != 1 || e == nil || e.Active == nil {
		return unknown
	}
	check := str(p, "check")
	if !*e.Active {
		return truthOf(check == "idle")
	}
	if id, _ := strconv.Atoi(str(p, "encounterID")); id > 0 {
		if e.ID == 0 {
			return unknown
		}
		if id != e.ID {
			return truthOf(check == "idle")
		}
	}
	if d, _ := strconv.Atoi(str(p, "difficulty")); d > 0 {
		if e.Difficulty == 0 {
			return unknown
		}
		if d != e.Difficulty {
			return truthOf(check == "idle")
		}
	}
	if check == "idle" {
		return no
	}
	if check == "active" {
		return yes
	}
	if check == "elapsed" && e.Elapsed != nil {
		return truthOf(compareNumber(*e.Elapsed, num(p, "value"), str(p, "operator")))
	}
	return unknown
}
func encounterMatches(p map[string]any, e *lookup.OverlayEncounterEvent) bool {
	if e == nil || e.Kind != str(p, "event") {
		return false
	}
	if id, _ := strconv.Atoi(str(p, "encounterID")); id > 0 && id != e.ID {
		return false
	}
	if d, _ := strconv.Atoi(str(p, "difficulty")); d > 0 && d != e.Difficulty {
		return false
	}
	if result := str(p, "outcome"); !one(result, "", "any") && (e.Success == nil || *e.Success != (result == "victory")) {
		return false
	}
	return true
}
func (r *Runtime) evaluateEncounters(rules []Rule, p *lookup.OverlayInput, now time.Time, fresh bool) {
	e := p.Encounter
	if p.EncountersVersion != 1 || e == nil {
		r.encounterCursor = -1
		return
	}
	if fresh || r.encounterCursor < 0 || e.Seq < r.encounterCursor {
		r.encounterCursor = e.Seq
		return
	}
	entries := append([]lookup.OverlayEncounterEvent(nil), e.Events...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
	if e.Seq-r.encounterCursor > 4 && r.problem == "" {
		r.problem = "Some encounter events were missed"
	}
	for _, entry := range entries {
		if entry.Seq <= r.encounterCursor {
			continue
		}
		r.currentTrigger = &triggerContext{Encounter: &entry}
		for _, rule := range rules {
			if rule.Enabled && loaded(rule.Load, p) {
				r.evaluate(rule, entry.Kind, p, now.Add(-time.Duration(entry.Age)*time.Millisecond))
			}
		}
	}
	r.currentTrigger = nil
	r.encounterCursor = e.Seq
}

func encounterValues(data *displayValues, id int, name string, difficulty, size int, elapsed *float64) {
	text := data.Text
	text["encounterName"] = name
	for k, v := range map[string]int{"encounterID": id, "difficulty": difficulty, "groupSize": size} {
		if v > 0 {
			text[k] = strconv.Itoa(v)
		}
	}
	if elapsed != nil {
		data.setNumber("elapsed", elapsed)
	}
}
