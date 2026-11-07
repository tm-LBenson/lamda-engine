package overlays

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type EquipmentSetWatch struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func equipmentSetKey(name string) string {
	h := sha256.Sum256([]byte(name))
	return hex.EncodeToString(h[:])[:16]
}
func validEquipment(p map[string]any) bool {
	source, check := str(p, "source"), str(p, "check")
	slot := num(p, "slot")
	if math.IsNaN(slot) {
		slot = 0
	}
	if !bounded(slot, 0, 19) || math.Trunc(slot) != slot {
		return false
	}
	id, err := strconv.Atoi(str(p, "id"))
	comparison := one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte")
	switch source {
	case "item":
		return err == nil && id > 0 && id < 10000000 && one(check, "equipped", "missing")
	case "slot":
		return slot > 0 && (one(check, "occupied", "empty") || check == "level" && comparison && bounded(num(p, "value"), 0, 10000))
	case "set":
		return err == nil && id > 0 && id < 10000000 && check == "count" && comparison && bounded(num(p, "value"), 0, 19) && math.Trunc(num(p, "value")) == num(p, "value")
	case "saved":
		n := str(p, "name")
		return len(n) <= 120 && strings.TrimSpace(n) != "" && utf8.ValidString(n) && !strings.ContainsAny(n, "\r\n\x00") && one(check, "equipped", "missing")
	case "type":
		return bounded(num(p, "itemClass"), 0, 100) && math.Trunc(num(p, "itemClass")) == num(p, "itemClass") && bounded(num(p, "itemSubclass"), -1, 100) && math.Trunc(num(p, "itemSubclass")) == num(p, "itemSubclass") && one(check, "equipped", "missing")
	}
	return false
}
func equipmentPredicates(rules []Rule, visit func(map[string]any)) {
	rules = executionRules(rules)
	add := func(p map[string]any) {
		walkPredicate(p, func(q map[string]any) {
			if str(q, "condition") == "equipment" {
				visit(q)
			}
		})
	}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == "if" {
				add(n.Props)
			}
			walk(n.Children)
		}
	}
	for _, r := range rules {
		if r.Enabled {
			add(r.Load.Conditions)
			walk(r.Roots)
			walk(valueInputNodes(r))
			for _, m := range r.Style.Modifiers {
				add(m.Condition)
			}
		}
	}
}
func needsEquipment(rules []Rule) bool {
	found := needsLoadoutChange(rules, "equipment")
	equipmentPredicates(rules, func(map[string]any) { found = true })
	return found
}
func EquipmentSets(rules []Rule) []EquipmentSetWatch {
	seen := map[string]EquipmentSetWatch{}
	equipmentPredicates(rules, func(p map[string]any) {
		if str(p, "source") == "saved" {
			n := str(p, "name")
			k := equipmentSetKey(n)
			seen[k] = EquipmentSetWatch{k, n}
		}
	})
	out := make([]EquipmentSetWatch, 0, len(seen))
	for _, w := range seen {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
func equipmentCondition(q map[string]any, p *lookup.OverlayInput) truth {
	if p.EquipmentVersion != 1 || p.Equipment == nil {
		return unknown
	}
	e := p.Equipment
	check := str(q, "check")
	source := str(q, "source")
	if source == "saved" {
		key := equipmentSetKey(str(q, "name"))
		for _, s := range e.Sets {
			if s.Key == key && s.Equipped != nil {
				return truthOf(*s.Equipped == (check == "equipped"))
			}
		}
		return unknown
	}
	slots := map[int]lookup.OverlayEquipmentSlot{}
	for _, s := range e.Slots {
		slots[s.Slot] = s
	}
	start, end := 1, 19
	if slot := num(q, "slot"); !math.IsNaN(slot) && slot > 0 && source != "set" {
		start = int(slot)
		end = start
	}
	id, _ := strconv.Atoi(str(q, "id"))
	count := 0
	uncertain := false
	for i := start; i <= end; i++ {
		s, ok := slots[i]
		if !ok || s.Item == nil {
			uncertain = true
			continue
		}
		if source == "slot" {
			switch check {
			case "occupied":
				return truthOf(*s.Item > 0)
			case "empty":
				return truthOf(*s.Item == 0)
			case "level":
				if *s.Item == 0 {
					return no
				}
				if s.Level == nil {
					return unknown
				}
				return truthOf(compareNumber(*s.Level, num(q, "value"), str(q, "operator")))
			}
		}
		if *s.Item == 0 {
			continue
		}
		match := false
		switch source {
		case "item":
			match = *s.Item == id
		case "set":
			if s.Set == nil {
				uncertain = true
				continue
			}
			match = *s.Set == id
		case "type":
			if s.Class == nil {
				uncertain = true
				continue
			}
			if *s.Class != int(num(q, "itemClass")) {
				continue
			}
			if num(q, "itemSubclass") >= 0 {
				if s.Subclass == nil {
					uncertain = true
					continue
				}
				match = *s.Subclass == int(num(q, "itemSubclass"))
			} else {
				match = true
			}
		}
		if match {
			count++
			if source != "set" {
				return truthOf(check == "equipped")
			}
		}
	}
	if uncertain {
		return unknown
	}
	if source == "set" {
		return truthOf(compareNumber(float64(count), num(q, "value"), str(q, "operator")))
	}
	return truthOf(check == "missing")
}
func (r *Runtime) validateEquipment(p *lookup.OverlayInput, rules []Rule) {
	if !needsEquipment(rules) {
		return
	}
	if p.EquipmentVersion != 1 || p.Equipment == nil {
		if r.problem == "" {
			r.problem = "Reload WoW for equipment tracking"
		}
		return
	}
	for _, w := range EquipmentSets(rules) {
		found := false
		for _, s := range p.Equipment.Sets {
			if s.Key == w.Key {
				found = true
				break
			}
		}
		if !found && r.problem == "" {
			r.problem = "Reload WoW to watch new equipment sets"
		}
	}
}
