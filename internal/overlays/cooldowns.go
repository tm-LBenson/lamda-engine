package overlays

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strconv"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type CooldownWatch struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}

func (w CooldownWatch) Key() string { return w.Kind + ":" + strconv.Itoa(w.ID) }

func validCooldown(p map[string]any) bool {
	id, err := strconv.Atoi(str(p, "id"))
	kind, check := str(p, "source"), str(p, "check")
	if err != nil || !lookup.ValidCooldownWatch(kind, id) {
		return false
	}
	if one(check, "ready", "cooldown", "usable", "unusable") {
		return true
	}
	if one(check, "charges", "full", "recharge") && kind != "spell" {
		return false
	}
	if check == "full" {
		return true
	}
	limit := 86400.0
	if check == "charges" {
		limit = 999
	}
	return one(check, "remaining", "charges", "recharge") && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, limit) && (check != "charges" || math.Trunc(num(p, "value")) == num(p, "value"))
}

func CooldownWatches(rules []Rule) []CooldownWatch {
	rules = executionRules(rules)
	byKey := map[string]CooldownWatch{}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == "show" {
				s := explicitSource(n.Props)
				if lookup.ValidCooldownWatch(s.Kind, s.ID) {
					w := CooldownWatch{s.Kind, s.ID}
					byKey[w.Key()] = w
				}
			}
			if n.Type == "if" {
				walkPredicate(n.Props, func(p map[string]any) {
					if str(p, "condition") == "cooldown" {
						id, _ := strconv.Atoi(str(p, "id"))
						w := CooldownWatch{str(p, "source"), id}
						byKey[w.Key()] = w
					}
				})
			}
			walk(n.Children)
		}
	}
	for _, r := range rules {
		if r.Enabled {
			walk(r.Roots)
			walk(valueInputNodes(r))
			for _, m := range r.Style.Modifiers {
				walk([]Node{{Type: "if", Props: m.Condition}})
			}
		}
	}
	result := make([]CooldownWatch, 0, len(byKey))
	for _, w := range byKey {
		result = append(result, w)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result
}

func CooldownHash(w []CooldownWatch) string {
	b, _ := json.Marshal(w)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// A charged spell is ready with any charge, even while another recharges.
// Unknown fields never satisfy a negative or numeric comparison.
func cooldownCondition(p map[string]any, input *lookup.OverlayInput) bool {
	id, _ := strconv.Atoi(str(p, "id"))
	for _, c := range input.Cooldowns {
		if c.ID != id || c.Kind != str(p, "source") || !c.Available {
			continue
		}
		check := str(p, "check")
		if one(check, "ready", "cooldown", "usable", "unusable") {
			if c.Enabled == nil || !*c.Enabled || c.Remaining == nil || c.MaxCharges == nil {
				return false
			}
			ready := *c.Remaining == 0
			if *c.MaxCharges > 0 {
				if c.Charges == nil {
					return false
				}
				ready = *c.Charges > 0 && *c.Remaining == 0
			}
			switch check {
			case "ready":
				return ready
			case "cooldown":
				return !ready
			default:
				if c.Usable == nil {
					return false
				}
				usable := ready && *c.Usable
				if check == "unusable" {
					return !usable
				}
				return usable
			}
		}
		if check == "full" {
			return c.Charges != nil && c.MaxCharges != nil && *c.MaxCharges > 0 && *c.Charges == *c.MaxCharges
		}
		v := c.Remaining
		if check == "charges" {
			v = c.Charges
			if c.MaxCharges == nil || *c.MaxCharges == 0 {
				return false
			}
		}
		if check == "recharge" {
			v = c.Recharge
			if c.MaxCharges == nil || *c.MaxCharges == 0 {
				return false
			}
		}
		if v == nil {
			return false
		}
		return compareNumber(*v, num(p, "value"), str(p, "operator"))
	}
	return false
}

func compareNumber(a, b float64, op string) bool {
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	switch op {
	case "lt":
		return a < b
	case "lte":
		return a <= b
	case "eq":
		return a == b
	case "ne":
		return a != b
	case "gt":
		return a > b
	case "gte":
		return a >= b
	}
	return false
}

func (r *Runtime) validateCooldowns(p *lookup.OverlayInput, rules []Rule) {
	expected := CooldownWatches(rules)
	if p.CooldownsVersion != 1 {
		p.Cooldowns = nil
		if len(expected) > 0 {
			r.problem = "Reload WoW for cooldown tracking"
		}
		return
	}
	captured := make([]CooldownWatch, 0, len(p.Cooldowns))
	seen := map[string]bool{}
	valid := true
	for _, c := range p.Cooldowns {
		w := CooldownWatch{c.Kind, c.ID}
		if seen[w.Key()] || !lookup.ValidCooldownWatch(c.Kind, c.ID) {
			valid = false
		}
		seen[w.Key()] = true
		captured = append(captured, w)
	}
	if !valid || CooldownHash(captured) != p.CooldownHash {
		p.Cooldowns = nil
		if len(expected) > 0 {
			r.problem = "Reload WoW to refresh cooldown tracking"
		}
		return
	}
	for _, w := range expected {
		if !seen[w.Key()] {
			r.problem = "Reload WoW to watch new cooldowns"
			break
		}
	}
}
