package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"strings"
)

// Static dependencies of one prepared rule. Captured values and remembered
// character state remain live; only the authoring graph is cached.
type rulePlan struct {
	inputs     []Node
	remembered []NamedValue
}

func logicContext(c *triggerContext, l *Logic) *triggerContext {
	if l == nil {
		return c
	}
	out := triggerContext{}
	if c != nil {
		out = *c
	}
	out.Logic = l
	return &out
}
func logicFrom(c *triggerContext) *Logic {
	if c != nil {
		return c.Logic
	}
	return nil
}
func valueCondition(q map[string]any, p *lookup.OverlayInput, actor string, c *triggerContext) truth {
	e := newValueEvaluation(logicFrom(c), p, actor, c)
	v := e.named(str(q, "key"))
	if v.kind == "" {
		return unknown
	}
	op := str(q, "operator")
	if one(op, "true", "false") {
		if v.kind != "boolean" {
			return unknown
		}
		return truthOf(v.boolean == (op == "true"))
	}
	rhs := logicLiteral(q["value"])
	switch str(q, "compareTo") {
	case "value":
		rhs = e.named(str(q, "other"))
	case "option":
		rhs = e.option(str(q, "other"))
	}
	result := applyValueOp(op, []logicValue{v, rhs})
	if result.kind != "boolean" {
		return unknown
	}
	return truthOf(result.boolean)
}
func validValueCondition(q map[string]any) bool {
	if !logicKey.MatchString(str(q, "key")) || !one(str(q, "operator"), "true", "false", "eq", "ne", "lt", "lte", "gt", "gte") {
		return false
	}
	if one(str(q, "operator"), "true", "false") {
		return true
	}
	switch str(q, "compareTo") {
	case "", "literal":
		return logicLiteral(q["value"]).kind != ""
	case "value", "option":
		return logicKey.MatchString(str(q, "other"))
	}
	return false
}
func ruleValueRefs(r Rule) (map[string]bool, map[string]bool) {
	values, options := map[string]bool{}, map[string]bool{}
	text := func(s string) {
		for _, m := range namedValueToken.FindAllStringSubmatch(s, -1) {
			values[m[1]] = true
		}
	}
	check := func(p map[string]any) {
		walkPredicate(p, func(q map[string]any) {
			if str(q, "condition") == "value" {
				values[str(q, "key")] = true
				if !one(str(q, "operator"), "true", "false") {
					switch str(q, "compareTo") {
					case "value":
						values[str(q, "other")] = true
					case "option":
						options[str(q, "other")] = true
					}
				}
			}
		})
	}
	var walk func([]Node)
	walk = func(ns []Node) {
		for _, n := range ns {
			if n.Type == "if" {
				check(n.Props)
			}
			if n.Type == "show" {
				text(str(n.Props, "text"))
				if str(n.Props, "dataSource") == "value" {
					values[str(n.Props, "dataValue")] = true
					if key := str(n.Props, "dataMaxValue"); key != "" {
						values[key] = true
					}
				}
			}
			walk(n.Children)
		}
	}
	walk(r.Roots)
	for _, m := range r.Style.Modifiers {
		check(m.Condition)
		if m.Text != nil {
			text(*m.Text)
		}
	}
	return values, options
}
func validRuleLogic(r Rule) bool {
	if !validLogic(r.Logic) {
		return false
	}
	values, options := ruleValueRefs(r)
	for k := range values {
		if r.Logic.named(k) == nil {
			return false
		}
	}
	for k := range options {
		if r.Logic.option(k) == nil {
			return false
		}
	}
	return true
}

// Feed dependencies through the existing watch collectors, so formulas use the
// same native capture and reload continuity as ordinary IF/SHOW blocks.
func valueInputNodes(r Rule) []Node {
	if r.plan != nil {
		return r.plan.inputs
	}
	if r.Logic == nil {
		return nil
	}
	refs, _ := ruleValueRefs(r)
	visited := map[string]bool{}
	out := []Node{}
	var expr func(ValueExpr)
	var visit func(string)
	visit = func(key string) {
		if visited[key] {
			return
		}
		visited[key] = true
		if v := r.Logic.named(key); v != nil {
			if v.Remember != nil {
				out = append(out, Node{Type: "when", Props: v.Remember.When})
				if v.Remember.Reset == "encounter" {
					out = append(out, Node{Type: "when", Props: map[string]any{"event": "encounterstart"}})
				}
				if len(v.Remember.Check) > 0 {
					out = append(out, Node{Type: "if", Props: v.Remember.Check})
				}
			}
			expr(v.Expr)
		}
	}
	expr = func(e ValueExpr) {
		switch e.Kind {
		case "value":
			visit(e.Key)
		case "predicate":
			out = append(out, Node{Type: "if", Props: e.Check})
		case "data":
			p := map[string]any{"display": "text", "text": "{" + e.Field + "}"}
			for k, v := range e.Source {
				p[k] = v
			}
			out = append(out, Node{Type: "show", Props: p})
		}
		for _, a := range e.Args {
			expr(a)
		}
	}
	for key := range refs {
		visit(key)
	}
	return out
}
func valueText(template string, e *valueEvaluation) string {
	return namedValueToken.ReplaceAllStringFunc(template, func(token string) string {
		return e.named(strings.TrimSuffix(strings.TrimPrefix(token, "{value:"), "}")).display()
	})
}
