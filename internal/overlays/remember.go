package overlays

import (
	"bytes"
	"encoding/json"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// RememberValue samples Expr when an event matches. Updates in the same event
// all read the previous snapshot, so authoring order cannot change a counter.
type RememberValue struct {
	Initial any            `json:"initial"`
	When    map[string]any `json:"when"`
	Check   map[string]any `json:"check,omitempty"`
	Reset   string         `json:"reset"`
}

func (m *RememberValue) UnmarshalJSON(b []byte) error {
	type plain RememberValue
	var v plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	*m = RememberValue(v)
	return nil
}

func validWhen(p map[string]any) bool {
	return one(str(p, "event"), "talents", "equipment", "enter", "leave", "key", "keyend", "aura", "health", "resource", "cooldown", "combat", "combatend", "login", "state", "ability") || validCastWhen(p) || validEncounterWhen(p) || validBossWhen(p) || str(p, "event") == "combatlog" && validCombatWhen(p)
}

func validRemember(m *RememberValue) bool {
	if m == nil {
		return true
	}
	if logicLiteral(m.Initial).kind == "" || !one(m.Reset, "loaded", "combat", "encounter", "saved") || !validWhen(m.When) {
		return false
	}
	for key := range m.When {
		if !one(key, "event", "unit", "spell", "combatEvent", "sourceUnit", "targetUnit", "extraSpell", "school", "critical", "periodic", "auraKind", "miss", "provider", "timelineSource", "textFilter", "timerKind", "encounterID", "difficulty", "outcome") {
			return false
		}
	}
	if len(m.Check) > 0 {
		count := 0
		if !validValueExpr(ValueExpr{Kind: "predicate", Check: m.Check}, 1, &count) {
			return false
		}
	}
	return true
}

type rememberState struct {
	Profile string
	Values  map[string]logicValue
	Latched map[string]bool
}

func (r *Runtime) ruleContext(rule Rule, original *triggerContext) *triggerContext {
	c := logicContext(original, rule.Logic)
	if c != nil {
		c.Memory = r.memory[rule.ID]
	}
	return c
}

// Only reachable definitions capture events or consume native watch slots.
func rememberedValues(rule Rule) []NamedValue {
	if rule.plan != nil {
		return rule.plan.remembered
	}
	if rule.Logic == nil {
		return nil
	}
	refs, _ := ruleValueRefs(rule)
	seen := map[string]bool{}
	var visit func(string)
	var expr func(ValueExpr)
	expr = func(e ValueExpr) {
		if e.Kind == "value" {
			visit(e.Key)
		}
		for _, a := range e.Args {
			expr(a)
		}
	}
	visit = func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		if v := rule.Logic.named(key); v != nil {
			expr(v.Expr)
		}
	}
	for key := range refs {
		visit(key)
	}
	var out []NamedValue
	for _, v := range rule.Logic.Values {
		if v.Remember != nil && seen[v.Key] {
			out = append(out, v)
		}
	}
	return out
}

func rememberContinuous(event string) bool {
	return one(event, "state", "aura", "cooldown", "casting")
}

func (r *Runtime) rememberEvent(rule Rule, event string, input *lookup.OverlayInput) {
	values := rememberedValues(rule)
	if len(values) == 0 {
		return
	}
	m := r.memory[rule.ID]
	profile := ""
	if input != nil {
		profile = memoryProfile(input.Character, input.Realm)
	}
	if m == nil || m.Profile != profile {
		m = &rememberState{Profile: profile, Values: map[string]logicValue{}, Latched: map[string]bool{}}
		r.memory[rule.ID] = m
	}
	for _, v := range values {
		if v.Remember.Reset == "saved" && r.savedMemory != nil && profile != "" {
			if _, loaded := m.Values[v.Key]; !loaded {
				if stored, exists := r.savedMemory.records[memoryKey(profile, rule.ID, v.Key)]; exists && logicLiteral(stored.Value).kind == logicLiteral(v.Remember.Initial).kind {
					m.Values[v.Key] = logicLiteral(stored.Value)
				}
			}
		}
		if v.Remember.Reset == "combat" && event == "combat" || v.Remember.Reset == "encounter" && event == "encounterstart" {
			delete(m.Values, v.Key)
			delete(m.Latched, v.Key)
		}
	}
	context := r.ruleContext(rule, r.currentTrigger)
	writes := map[string]logicValue{}
	for _, v := range values {
		config := v.Remember
		if str(config.When, "event") != event {
			continue
		}
		matched := r.triggerMatches(Node{Type: "when", Props: config.When}, input, "player")
		if len(config.Check) > 0 && predicate(config.Check, input, "player", context) != yes {
			matched = false
		}
		if !matched {
			delete(m.Latched, v.Key)
			continue
		}
		if rememberContinuous(event) && m.Latched[v.Key] {
			continue
		}
		// Use a fresh bounded evaluator for each update. Its named references
		// still read the same pre-commit snapshot for all remembered values.
		evaluation := newValueEvaluation(rule.Logic, input, "player", context)
		result := evaluation.eval(v.Expr, 0)
		if result.kind == "" || result.kind != logicLiteral(config.Initial).kind {
			continue
		}
		writes[v.Key] = result
		m.Latched[v.Key] = true
	}
	for key, value := range writes {
		if rule.Logic.named(key).Remember.Reset == "saved" && r.savedMemory != nil {
			if input == nil || !r.savedMemory.capture(input.Character, input.Realm, rule.ID, key, value) {
				delete(m.Latched, key)
				continue
			}
		}
		m.Values[key] = value
	}
}
