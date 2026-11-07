// Package overlays owns user-authored rules. No rule can execute code or game input.
package overlays

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type Node struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props"`
	Children []Node         `json:"children,omitempty"`
}
type Style struct {
	Visual      *VisualStyle         `json:"visual,omitempty"`
	Anchor      *AnchorStyle         `json:"anchor,omitempty"`
	Group       *DisplayGroup        `json:"group,omitempty"`
	OnShow      string               `json:"onShow,omitempty"`
	OnHide      string               `json:"onHide,omitempty"`
	Modifiers   []AppearanceModifier `json:"modifiers,omitempty"`
	Arc         *ArcStyle            `json:"arc,omitempty"`
	Size        float64              `json:"size"`
	Opacity     float64              `json:"opacity"`
	Color       string               `json:"color"`
	Font        float64              `json:"font"`
	Transparent bool                 `json:"transparent"`
	X           float64              `json:"x"`
	Y           float64              `json:"y"`
}
type Load struct {
	Conditions map[string]any `json:"conditions,omitempty"`
	Character  string         `json:"character,omitempty"`
	Realm      string         `json:"realm,omitempty"`
	Class      string         `json:"class,omitempty"`
	Spec       int            `json:"spec,omitempty"`
	Race       string         `json:"race,omitempty"`
	Faction    string         `json:"faction,omitempty"`
	Role       string         `json:"role,omitempty"`
	Location   string         `json:"location,omitempty"`
	Combat     string         `json:"combat,omitempty"`
	Group      string         `json:"group,omitempty"`
	Mounted    string         `json:"mounted,omitempty"`
	Alive      string         `json:"alive,omitempty"`
	MinLevel   int            `json:"minLevel,omitempty"`
	Zone       string         `json:"zone,omitempty"`
}
type Rule struct {
	execution bool
	plan      *rulePlan
	Pack      *Pack  `json:"pack,omitempty"`
	Logic     *Logic `json:"logic,omitempty"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Roots     []Node `json:"roots"`
	Style     Style  `json:"style"`
	Load      Load   `json:"load"`
}
type Watch struct {
	Spell int    `json:"spell"`
	Kind  string `json:"kind"`
	Unit  string `json:"unit"`
}

func (w Watch) Key() string                 { return w.Unit + ":" + w.Kind + ":" + strconv.Itoa(w.Spell) }
func str(p map[string]any, k string) string { v, _ := p[k].(string); return v }
func num(p map[string]any, k string) float64 {
	switch v := p[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	}
	return math.NaN()
}
func one(s string, choices ...string) bool {
	for _, v := range choices {
		if v == s {
			return true
		}
	}
	return false
}
func bounded(n, lo, hi float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= lo && n <= hi
}

const MaxRules = 200

var validID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)
var colors = map[string]string{"gold": "#F4C879", "teal": "#65DDC7", "violet": "#C6A5EE", "red": "#EF9B9B"}

func Validate(rules []Rule) error {
	if !validPacks(rules) {
		return fmt.Errorf("Invalid pack membership or shared settings")
	}
	if !validGroups(rules) {
		return fmt.Errorf("Invalid display group")
	}
	if len(Anchors(rules)) > lookup.OverlayAnchorLimit {
		return fmt.Errorf("Keep at most %d attachments", lookup.OverlayAnchorLimit)
	}
	if len(rules) > MaxRules {
		return fmt.Errorf("Keep at most %d overlays", MaxRules)
	}
	ids := map[string]bool{}
	id := func(s string) bool {
		if !validID.MatchString(s) || ids[s] {
			return false
		}
		ids[s] = true
		return true
	}
	for _, r := range rules {
		if _, err := compileActions(r); err != nil {
			return err
		}
		if !id(r.ID) || strings.TrimSpace(r.Name) == "" || len(r.Name) > 240 {
			return fmt.Errorf("Invalid overlay name or ID")
		}
		if !validRuleLogic(r) {
			return logicError(r.Name)
		}
		s := r.Style
		if !validVisual(s.Visual) {
			return fmt.Errorf("Invalid visual settings")
		}
		if !bounded(s.Size, 16, 400) || !bounded(s.Opacity, 25, 100) || colorValue(s.Color) == "" || !bounded(s.Font, 10, 72) || !bounded(s.X, 0, 100) || !bounded(s.Y, 0, 100) || !validAnchor(s.Anchor) || !validModifiers(s.Modifiers) || !validSound(s.OnShow, true) || !validSound(s.OnHide, true) {
			return fmt.Errorf("Invalid appearance")
		}
		if s.Arc != nil && (!s.Arc.valid() || r.Enabled && s.Arc.Repeat == "stacks" && s.Arc.Aura == 0 && hasArcs(r.Roots)) {
			return fmt.Errorf("Invalid arch appearance")
		}
		l := r.Load
		if !validBuildLoad(l.Conditions) {
			return fmt.Errorf("Invalid build or gear load condition")
		}
		for _, s := range []string{l.Character, l.Realm, l.Class, l.Race, l.Faction, l.Role, l.Location, l.Zone} {
			if len(s) > 120 || strings.ContainsAny(s, "\r\n\x00") {
				return fmt.Errorf("Invalid load condition")
			}
		}
		if l.Spec < 0 || l.Spec > 10000 || l.MinLevel < 0 || l.MinLevel > 1000 {
			return fmt.Errorf("Invalid spec or level")
		}
		for _, s := range []string{l.Combat, l.Mounted, l.Alive} {
			if !one(s, "", "yes", "no") {
				return fmt.Errorf("Invalid load state")
			}
		}
		if !one(l.Group, "", "solo", "party", "raid") {
			return fmt.Errorf("Invalid group")
		}
		count := 0
		var check func([]Node, int, bool) error
		check = func(nodes []Node, depth int, inEach bool) error {
			for _, n := range nodes {
				count++
				if count > 80 || depth > 8 || !id(n.ID) || (depth == 0 && !one(n.Type, "when", "action")) {
					return fmt.Errorf("Invalid block nesting or ID")
				}
				p := n.Props
				ok := false
				switch n.Type {
				case "action":
					ok = depth == 0 && validLogicLabel(str(p, "name")) && len(p) == 1
				case "call":
					ok = validID.MatchString(str(p, "action")) && len(p) == 1
				case "when":
					ok = validWhen(p)
				case "each":
					ok = !inEach && lookup.OverlayCloneScope(str(p, "scope"))
				case "if":
					count := 0
					ok = validPredicate(p, 0, &count)
				case "show":
					ok = one(str(p, "display"), "text", "bar", "icon", "arc", "ring", "shape", "image") && len(str(p, "text")) <= 360 && one(str(p, "visibility"), "", "timed", "conditions") && (str(p, "visibility") == "conditions" || bounded(num(p, "duration"), 1, 120))
					ok = ok && validShowData(p)
				case "sound":
					ok = validSound(str(p, "sound"), false)
				case "glow":
					ok = colorValue(str(p, "color")) != ""
				case "wait":
					ok = bounded(num(p, "seconds"), 0, 15)
				case "hide":
					ok = true
				}
				if !ok {
					return fmt.Errorf("Invalid %s block", n.Type)
				}
				if one(n.Type, "when", "if", "each", "action") {
					if err := check(n.Children, depth+1, inEach || n.Type == "each"); err != nil {
						return err
					}
				} else if len(n.Children) > 0 {
					return fmt.Errorf("Actions cannot contain blocks")
				}
			}
			return nil
		}
		if err := check(r.Roots, 0, false); err != nil {
			return err
		}
	}
	if len(CombatWatches(rules)) > 8 {
		return fmt.Errorf("Keep at most 8 combat units, including Player")
	}
	if len(MeasurementWatches(rules)) > lookup.OverlaySourceLimit {
		return fmt.Errorf("Keep at most %d unit health/resource sources", lookup.OverlaySourceLimit)
	}
	if len(EquipmentSets(rules)) > 8 {
		return fmt.Errorf("Keep at most 8 saved equipment sets")
	}
	if len(TalentWatches(rules)) > 16 {
		return fmt.Errorf("Keep at most 16 talent and learned-spell sources")
	}
	if len(CooldownWatches(rules))+2*len(CastWatches(rules)) > lookup.OverlayTimerLimit {
		return fmt.Errorf("Keep at most %d timer slots (one per cooldown, two per cast unit)", lookup.OverlayTimerLimit)
	}
	if len(Watches(rules)) > lookup.OverlaySourceLimit {
		return fmt.Errorf("Keep at most %d aura sources; reuse the same spell and unit where possible", lookup.OverlaySourceLimit)
	}
	return nil
}
func Watches(rules []Rule) []Watch {
	rules = executionRules(rules)
	all := map[string]Watch{}
	add := func(spell int, kind, unit string, repeated cloneScope) {
		if spell > 0 {
			for _, u := range cloneUnits(repeated, unit) {
				w := Watch{spell, kind, u}
				all[w.Key()] = w
			}
		}
	}
	var walk func([]Node, cloneScope)
	walk = func(ns []Node, repeated cloneScope) {
		for _, n := range ns {
			if n.Type == "show" {
				s := explicitSource(n.Props)
				if one(s.Kind, "buff", "debuff") {
					add(s.ID, s.Kind, s.Unit, repeated)
				}
			}
			if n.Type == "if" {
				walkPredicate(n.Props, func(p map[string]any) {
					if one(str(p, "condition"), "buff", "debuff") {
						id, _ := strconv.Atoi(str(p, "aura"))
						add(id, str(p, "condition"), str(p, "unit"), repeated)
					}
				})
			}
			walk(n.Children, childScope(n, repeated))
		}
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		repeated := ruleScopes(r.Roots)
		walk(r.Roots, 0)
		walk(valueInputNodes(r), repeated|playerScope)
		for _, m := range r.Style.Modifiers {
			walk([]Node{{Type: "if", Props: m.Condition}}, repeated)
		}
		if a := r.Style.Arc; a != nil && a.Repeat == "stacks" && hasArcs(r.Roots) {
			add(a.Aura, a.Kind, a.Unit, repeated)
		}
	}
	out := []Watch{}
	for _, w := range all {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// Containers retain empty openings across save/reload; actions have no children.
func (n Node) MarshalJSON() ([]byte, error) {
	if one(n.Type, "when", "if", "each", "action", "sequence") {
		children := n.Children
		if children == nil {
			children = []Node{}
		}
		return json.Marshal(struct {
			ID       string         `json:"id"`
			Type     string         `json:"type"`
			Props    map[string]any `json:"props"`
			Children []Node         `json:"children"`
		}{n.ID, n.Type, n.Props, children})
	}
	return json.Marshal(struct {
		ID    string         `json:"id"`
		Type  string         `json:"type"`
		Props map[string]any `json:"props"`
	}{n.ID, n.Type, n.Props})
}
