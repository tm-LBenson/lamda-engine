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

type TalentWatch struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}

func (w TalentWatch) Key() string { return w.Kind + ":" + strconv.Itoa(w.ID) }
func talentWatch(p map[string]any) TalentWatch {
	id, _ := strconv.Atoi(str(p, "id"))
	kind := str(p, "source")
	if str(p, "condition") == "learned" {
		kind = "known"
	}
	return TalentWatch{kind, id}
}
func validTalent(p map[string]any) bool {
	w := talentWatch(p)
	if !lookup.ValidTalentWatch(w.Kind, w.ID) {
		return false
	}
	if str(p, "condition") == "talent" && w.Kind == "known" {
		return false
	}
	if one(str(p, "check"), "selected", "missing") {
		return true
	}
	return w.Kind != "known" && str(p, "check") == "rank" && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, 100) && math.Trunc(num(p, "value")) == num(p, "value")
}
func TalentWatches(rules []Rule) []TalentWatch {
	rules = executionRules(rules)
	found := map[string]TalentWatch{}
	add := func(p map[string]any) {
		walkPredicate(p, func(q map[string]any) {
			if one(str(q, "condition"), "talent", "learned") {
				w := talentWatch(q)
				found[w.Key()] = w
			}
		})
	}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == "if" {
				add(n.Props)
			}
			if n.Type == "show" && one(str(n.Props, "dataSource"), "talent", "learned") {
				id, _ := strconv.Atoi(str(n.Props, "dataID"))
				kind := str(n.Props, "dataTalentKind")
				if str(n.Props, "dataSource") == "learned" {
					kind = "known"
				}
				w := TalentWatch{kind, id}
				found[w.Key()] = w
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
	out := make([]TalentWatch, 0, len(found))
	for _, w := range found {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}
func TalentHash(w []TalentWatch) string {
	b, _ := json.Marshal(w)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}
func talentReading(w TalentWatch, p *lookup.OverlayInput) *lookup.OverlayTalent {
	if p.TalentsVersion != 1 {
		return nil
	}
	for i := range p.Talents {
		v := &p.Talents[i]
		if v.Kind == w.Kind && v.ID == w.ID {
			return v
		}
	}
	return nil
}
func talentCondition(q map[string]any, p *lookup.OverlayInput) truth {
	t := talentReading(talentWatch(q), p)
	if t == nil || t.Rank == nil {
		return unknown
	}
	switch str(q, "check") {
	case "selected":
		return truthOf(*t.Rank > 0)
	case "missing":
		return truthOf(*t.Rank == 0)
	case "rank":
		return truthOf(compareNumber(float64(*t.Rank), num(q, "value"), str(q, "operator")))
	}
	return unknown
}
func (r *Runtime) validateTalents(p *lookup.OverlayInput, rules []Rule) {
	expected := TalentWatches(rules)
	if p.TalentsVersion != 1 {
		p.Talents = nil
		if len(expected) > 0 && r.problem == "" {
			r.problem = "Reload WoW for talent tracking"
		}
		return
	}
	captured := make([]TalentWatch, 0, len(p.Talents))
	seen := map[string]bool{}
	valid := true
	for _, t := range p.Talents {
		w := TalentWatch{t.Kind, t.ID}
		if seen[w.Key()] || !lookup.ValidTalentWatch(w.Kind, w.ID) {
			valid = false
		}
		seen[w.Key()] = true
		captured = append(captured, w)
	}
	if !valid || TalentHash(captured) != p.TalentHash {
		p.Talents = nil
		if len(expected) > 0 && r.problem == "" {
			r.problem = "Reload WoW to refresh talent tracking"
		}
		return
	}
	for _, w := range expected {
		if !seen[w.Key()] {
			if r.problem == "" {
				r.problem = "Reload WoW to watch new talents"
			}
			break
		}
	}
}
