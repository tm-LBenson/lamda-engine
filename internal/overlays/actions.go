package overlays

import (
	"fmt"
	"hash/fnv"
)

const expandedActionLimit = 256

func actionID(rule, path, source string) string {
	h := fnv.New64a()
	h.Write([]byte(rule + "|" + path + "|" + source))
	return fmt.Sprintf("a%016x", h.Sum64())
}

// Actions are declarations. Calls expand into private sequence containers with
// stable IDs per call site, so repeated calls cannot share display lifetimes.
func compileActions(rule Rule) (Rule, error) {
	if rule.execution {
		return rule, nil
	}
	defs := map[string]Node{}
	for _, n := range rule.Roots {
		if n.Type == "action" {
			defs[n.ID] = n
		}
	}
	var hasCalls func([]Node) bool
	hasCalls = func(ns []Node) bool {
		for _, n := range ns {
			if n.Type == "call" || hasCalls(n.Children) {
				return true
			}
		}
		return false
	}
	if len(defs) == 0 && !hasCalls(rule.Roots) {
		return rule, nil
	}
	if len(defs) > 16 {
		return Rule{}, fmt.Errorf("Keep at most 16 actions per overlay")
	}
	var expand func([]Node, string, map[string]bool, int, *int, bool) ([]Node, error)
	expand = func(ns []Node, path string, visiting map[string]bool, depth int, count *int, inEach bool) ([]Node, error) {
		out := make([]Node, 0, len(ns))
		for _, source := range ns {
			*count++
			if *count > expandedActionLimit || depth > 8 {
				return nil, fmt.Errorf("Action calls exceed the block or nesting limit")
			}
			if source.Type == "action" || source.Type == "sequence" {
				return nil, fmt.Errorf("Actions must be declared at the top level")
			}
			if source.Type == "each" && inEach {
				return nil, fmt.Errorf("Calls cannot nest FOR EACH blocks")
			}
			n := source
			if path != "" {
				n.ID = actionID(rule.ID, path, source.ID)
			}
			if n.Type == "call" {
				key := str(n.Props, "action")
				def, ok := defs[key]
				if !ok {
					return nil, fmt.Errorf("Choose an existing action")
				}
				if visiting[key] {
					return nil, fmt.Errorf("An action cannot call itself, directly or indirectly")
				}
				visiting[key] = true
				n.Type = "sequence"
				n.Props = map[string]any{}
				var err error
				n.Children, err = expand(def.Children, path+"/"+source.ID+"/"+key, visiting, depth+1, count, inEach)
				delete(visiting, key)
				if err != nil {
					return nil, err
				}
			} else {
				var err error
				n.Children, err = expand(source.Children, path, visiting, depth+1, count, inEach || n.Type == "each")
				if err != nil {
					return nil, err
				}
			}
			out = append(out, n)
		}
		return out, nil
	}
	// Unused declarations still need valid references and an acyclic graph.
	for key, def := range defs {
		count := 0
		if _, err := expand(def.Children, "/"+key, map[string]bool{key: true}, 1, &count, false); err != nil {
			return Rule{}, err
		}
	}
	roots := make([]Node, 0, len(rule.Roots))
	for _, n := range rule.Roots {
		if n.Type != "action" {
			roots = append(roots, n)
		}
	}
	count := 0
	nodes, err := expand(roots, "", map[string]bool{}, 0, &count, false)
	if err != nil {
		return Rule{}, err
	}
	rule.Roots = nodes
	rule.execution = true
	return rule, nil
}

func executionRule(rule Rule) Rule {
	out, err := compileActions(rule)
	if err != nil {
		rule.Roots = nil
		rule.execution = true
		return rule
	}
	return out
}

func executionRules(rules []Rule) []Rule {
	var out []Rule
	for i, r := range rules {
		compiled := executionRule(r)
		if compiled.execution && !r.execution && out == nil {
			out = append([]Rule{}, rules...)
		}
		if out != nil {
			out[i] = compiled
		}
	}
	if out == nil {
		return rules
	}
	return out
}

func (s *Store) rulesForExecution() []Rule {
	if s.execution == nil || len(s.executionSource) != len(s.doc.Rules) || len(s.doc.Rules) > 0 && &s.executionSource[0] != &s.doc.Rules[0] {
		// Store rules are immutable until Save, which invalidates this slice.
		// Copy even rules without ACTIONs so runtime-only preparation never leaks
		// into the editable document or exported packs.
		s.execution = append([]Rule{}, executionRules(s.doc.Rules)...)
		for i := range s.execution {
			r := &s.execution[i]
			r.execution = true
			r.plan = nil
			r.plan = &rulePlan{inputs: valueInputNodes(*r), remembered: rememberedValues(*r)}
		}
		s.executionSource = s.doc.Rules
		s.combatRequested = len(CombatWatches(s.execution)) > 0
	}
	return s.execution
}
