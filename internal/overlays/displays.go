package overlays

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"github.com/tm-LBenson/lamda-engine/internal/spellbook"
)

// Display bindings are per SHOW. Auto follows the nearest enclosing data IF,
// skipping unrelated role/location checks. No synthetic game readings are used.
type displaySource struct {
	Value, MaxValue                  string
	Maximum                          float64
	Boss                             bossFilter
	Kind, Unit, Resource, TalentKind string
	ID                               int
}

func explicitSource(p map[string]any) displaySource {
	id, _ := strconv.Atoi(str(p, "dataID"))
	u := str(p, "dataUnit")
	if u == "" {
		u = "player"
	}
	resource := str(p, "dataResource")
	if resource == "" {
		resource = "primary"
	}
	s := displaySource{Kind: str(p, "dataSource"), Unit: u, Resource: resource, ID: id}
	if s.Kind == "value" {
		s.Value = str(p, "dataValue")
		s.MaxValue = str(p, "dataMaxValue")
		s.Maximum = 100
		if _, ok := p["dataMaximum"]; ok {
			s.Maximum = num(p, "dataMaximum")
		}
	}
	if one(s.Kind, "talent", "learned") {
		s.TalentKind = str(p, "dataTalentKind")
		if s.Kind == "learned" {
			s.TalentKind = "known"
		}
	}
	if one(s.Kind, "bossTimer", "bossStage") {
		s.Boss = bossFilter{Provider: str(p, "dataProvider"), Text: str(p, "dataText"), Kind: str(p, "dataKind"), Source: str(p, "dataTimelineSource"), Spell: id}
	}
	return s
}
func conditionSource(p map[string]any) displaySource {
	var source displaySource
	ambiguous := false
	walkPredicate(p, func(q map[string]any) {
		s := conditionLeafSource(q)
		if s.Kind == "" {
			return
		}
		if source.Kind != "" && source != s {
			ambiguous = true
		}
		source = s
	})
	if ambiguous {
		return displaySource{}
	}
	return source
}
func conditionLeafSource(p map[string]any) displaySource {
	s := displaySource{Kind: str(p, "condition"), Unit: str(p, "unit"), Resource: str(p, "resource")}
	if s.Kind == "value" {
		s.Value = str(p, "key")
		s.Maximum = 100
	}
	if s.Unit == "" {
		s.Unit = "player"
	}
	if one(s.Kind, "talent", "learned") {
		w := talentWatch(p)
		s.ID = w.ID
		s.TalentKind = w.Kind
	}
	if one(s.Kind, "buff", "debuff") {
		s.ID, _ = strconv.Atoi(str(p, "aura"))
	}
	if s.Kind == "cooldown" {
		s.Kind = str(p, "source")
		s.ID, _ = strconv.Atoi(str(p, "id"))
	}
	if one(s.Kind, "health", "resource") {
		if str(p, "unit") == "" {
			s.Unit = "current"
		}
	} else if !one(s.Kind, "buff", "debuff", "casting") {
		s.Unit = "player"
	}
	if one(s.Kind, "bossTimer", "bossStage") {
		s.Boss = bossQuery(p)
		if s.Kind == "bossStage" {
			s.Boss.Check = ""
			s.Boss.Operator = ""
			s.Boss.Value = 0
		}
	}
	if s.Kind != "resource" {
		s.Resource = ""
	}
	if one(s.Kind, "value", "health", "resource", "buff", "debuff", "spell", "item", "slot", "gcd", "casting", "encounter", "bossTimer", "bossStage", "talent", "learned") {
		return s
	}
	return displaySource{}
}
func showSource(nodes []Node, id string, inherited displaySource) (Node, displaySource, bool) {
	return scopedShowSource(nodes, id, inherited, false)
}
func scopedShowSource(nodes []Node, id string, inherited displaySource, repeated bool) (Node, displaySource, bool) {
	for _, n := range nodes {
		s := inherited
		if n.Type == "when" && one(s.Kind, "", "castEvent", "combatEvent", "encounterEvent", "bossEvent") && lookup.ValidCastEvent(str(n.Props, "event")) {
			s = displaySource{Kind: "castEvent", Unit: str(n.Props, "unit")}
		}
		if n.Type == "when" && one(s.Kind, "", "castEvent", "combatEvent", "encounterEvent", "bossEvent") && str(n.Props, "event") == "combatlog" {
			s = displaySource{Kind: "combatEvent"}
		}
		if n.Type == "when" && one(s.Kind, "", "castEvent", "combatEvent", "encounterEvent", "bossEvent") && lookup.ValidEncounterEvent(str(n.Props, "event")) {
			s = displaySource{Kind: "encounterEvent"}
		}
		if n.Type == "when" && one(s.Kind, "", "castEvent", "combatEvent", "encounterEvent", "bossEvent") && lookup.ValidBossEvent(str(n.Props, "event")) {
			s = displaySource{Kind: "bossEvent"}
		}
		if n.Type == "if" {
			if c := conditionSource(n.Props); c.Kind != "" {
				s = c
				if !repeated && s.Unit == "current" {
					s.Unit = "player"
				}
			}
		}
		if n.ID == id {
			if explicit := explicitSource(n.Props); !one(explicit.Kind, "", "auto") {
				s = explicit
			}
			return n, s, true
		}
		if child, source, ok := scopedShowSource(n.Children, id, s, repeated || n.Type == "each"); ok {
			return child, source, true
		}
	}
	return Node{}, displaySource{}, false
}
func validShowData(p map[string]any) bool {
	for _, key := range []string{"dataSource", "dataValue", "dataMaxValue", "dataID", "dataUnit", "dataResource", "dataProvider", "dataText", "dataKind", "dataTimelineSource", "dataTalentKind", "icon", "fill"} {
		if value, ok := p[key]; ok {
			if _, ok := value.(string); !ok {
				return false
			}
		}
	}
	s := explicitSource(p)
	if !one(str(p, "fill"), "", "timer", "full", "source", "charges", "recharge") {
		return false
	}
	icon := str(p, "icon")
	if !one(icon, "", "shield", "auto") {
		id, e := strconv.Atoi(icon)
		if e != nil || id < 1 || id >= 10000000 {
			return false
		}
	}
	switch s.Kind {
	case "value":
		return logicKey.MatchString(s.Value) && (s.MaxValue == "" || logicKey.MatchString(s.MaxValue)) && bounded(s.Maximum, .000000001, logicLimit)
	case "talent", "learned":
		return lookup.ValidTalentWatch(s.TalentKind, s.ID) && (s.Kind != "talent" || s.TalentKind != "known")
	case "bossTimer", "bossStage":
		return validBossFields(map[string]any{"provider": s.Boss.Provider, "textFilter": s.Boss.Text, "timerKind": s.Boss.Kind, "timelineSource": s.Boss.Source, "spell": str(p, "dataID")})
	case "", "auto", "none", "trigger", "affected", "encounter":
		return true
	case "health", "casting":
		return validUnitChoice(s.Unit)
	case "resource":
		v, e := strconv.Atoi(s.Resource)
		return validUnitChoice(s.Unit) && (s.Resource == "primary" || e == nil && v >= 0 && v < 32)
	case "buff", "debuff":
		return s.ID > 0 && s.ID < 10000000 && validUnitChoice(s.Unit)
	default:
		return lookup.ValidCooldownWatch(s.Kind, s.ID)
	}
}

func displayDataProblem(rule Rule, p *lookup.OverlayInput) string {
	var walk func([]Node) string
	durationText := func(text string) bool {
		return strings.Contains(text, "{duration}") || strings.Contains(text, "{percent}")
	}
	modifierDuration := false
	for _, m := range rule.Style.Modifiers {
		if m.Text != nil && durationText(*m.Text) {
			modifierDuration = true
		}
	}
	walk = func(nodes []Node) string {
		for _, n := range nodes {
			if n.Type == "show" {
				_, s, _ := showSource(rule.Roots, n.ID, displaySource{})
				if n.ID == "" {
					s = explicitSource(n.Props)
				}
				if s.Kind == "resource" && p.ResourcesVersion != 1 {
					return "Reload WoW for resource tracking"
				}
				if one(s.Kind, "buff", "debuff") && (one(str(n.Props, "display"), "bar", "arc", "ring", "shape", "image") && str(n.Props, "fill") == "source" || one(str(n.Props, "display"), "bar", "text") && (modifierDuration || durationText(str(n.Props, "text")))) {
					if p.AuraDurationsVersion != 1 {
						return "Reload WoW for aura durations"
					}
					for _, w := range p.Watches {
						if w.Spell == s.ID && w.Kind == s.Kind && w.Unit == s.Unit && !w.DurationAvailable {
							return "Aura duration capture unavailable"
						}
					}
				}
			}
			if problem := walk(n.Children); problem != "" {
				return problem
			}
		}
		return ""
	}
	if problem := walk(rule.Roots); problem != "" {
		return problem
	}
	return walk(valueInputNodes(rule))
}

var displayToken = regexp.MustCompile(`\{(name|remaining|duration|stacks|charges|maxCharges|recharge|current|max|percent|character|realm|unit|unitName|role|sourceName|targetName|amount|effective|baseAmount|absorbed|overheal|overkill|blocked|resisted|wasted|power|critical|extraName|spellID|extraSpellID|event|miss|encounterName|encounterID|difficulty|groupSize|outcome|elapsed|provider|timerID|count|stage|stageTotal|rank)\}`)
var powerNames = []string{"Mana", "Rage", "Focus", "Energy", "Combo points", "Runes", "Runic power", "Soul shards", "Astral power", "Holy power", "Alternate power", "Maelstrom", "Chi", "Insanity", "Burning embers", "Demonic fury", "Arcane charges", "Fury", "Pain", "Essence", "Blood runes", "Frost runes", "Unholy runes", "Quest power", "Encounter power", "Vigor / mount power", "Balance", "Happiness", "Shadow orbs", "Chromatic runes"}

type displayValues struct {
	Raw                         map[string]float64
	Text                        map[string]string
	Fraction, Charges, Recharge *float64
	Spell                       int
}

// Preserve captured precision for calculations; text formatting is separate.
func (v *displayValues) setNumber(key string, n *float64) {
	if n == nil {
		return
	}
	v.Text[key] = displayNumber(*n)
	if bounded(*n, -logicLimit, logicLimit) {
		if v.Raw == nil {
			v.Raw = map[string]float64{}
		}
		v.Raw[key] = *n
	}
}
func displayNumber(v float64) string {
	if !bounded(v, 0, 16777214) {
		return "—"
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}
func sourceValues(s displaySource, p *lookup.OverlayInput) displayValues {
	if s.Unit == "" {
		s.Unit = "player"
	}
	v := displayValues{Text: map[string]string{"character": p.Character, "realm": p.Realm, "unit": s.Unit}}
	set := v.setNumber
	ratio := func(a, b *float64) *float64 {
		if a == nil || b == nil || !bounded(*b, 0.00001, 16777214) || !bounded(*a, 0, *b) {
			return nil
		}
		f := *a / *b
		return &f
	}
	if one(s.Kind, "buff", "debuff", "spell", "gcd", "castEvent") {
		v.Spell = s.ID
		v.Text["name"] = spellbook.Name(s.ID)
	}
	switch s.Kind {
	case "talent", "learned":
		if t := talentReading(TalentWatch{s.TalentKind, s.ID}, p); t != nil {
			if t.Rank != nil {
				v.Text["rank"] = strconv.Itoa(*t.Rank)
				if one(t.Kind, "hero", "pvp", "known") {
					f := float64(*t.Rank)
					v.Fraction = &f
				}
			}
			if t.Spell > 0 {
				v.Spell = t.Spell
				v.Text["name"] = spellbook.Name(t.Spell)
				v.Text["spellID"] = strconv.Itoa(t.Spell)
			}
		}
	case "bossTimer":
		bossValues(&v, bossTimerSource(s.Boss, p))
	case "bossStage":
		if provider := bossProvider(p, s.Boss.Provider); provider != nil && provider.Available {
			v.Text["name"] = "Stage"
			v.Text["provider"] = map[string]string{"dbm": "DBM", "bigwigs": "BigWigs", "blizzard": "Blizzard"}[provider.Name]
			if provider.Stage != nil {
				v.setNumber("stage", provider.Stage)
			}
			if provider.StageTotal != nil {
				v.setNumber("stageTotal", provider.StageTotal)
			}
		}
	case "encounter":
		if e := p.Encounter; p.EncountersVersion == 1 && e != nil && e.Active != nil && *e.Active {
			encounterValues(&v, e.ID, e.Name, e.Difficulty, e.GroupSize, e.Elapsed)
			v.Text["name"] = e.Name
		}
	case "casting":
		if c := castFor(p, s.Unit); c != nil && c.Available && c.Mode != nil && *c.Mode > 0 && c.Spell != nil {
			v.Spell = *c.Spell
			v.Text["name"] = spellbook.Name(*c.Spell)
			set("remaining", c.Remaining)
			set("duration", c.Duration)
			v.Fraction = ratio(c.Remaining, c.Duration)
		}
	case "health":
		v.Text["name"] = "Health"
		h := unitHealth(p, s.Unit)
		set("percent", h)
		if h != nil {
			f := *h / 100
			v.Fraction = &f
		}
	case "resource":
		r, power, ok := unitResource(p, s.Unit, s.Resource)
		if power >= 0 && power < len(powerNames) {
			v.Text["name"] = powerNames[power]
		}
		if ok {
			set("current", &r.Current)
			set("max", &r.Max)
			v.Fraction = ratio(&r.Current, &r.Max)
		}
	case "buff", "debuff":
		for _, a := range p.Watches {
			if a.Spell != s.ID || a.Kind != s.Kind || a.Unit != s.Unit || !a.Available || a.Active == nil {
				continue
			}
			if !*a.Active {
				zero := 0.0
				set("stacks", &zero)
				set("remaining", &zero)
				v.Fraction = &zero
				break
			}
			set("stacks", a.Stacks)
			set("remaining", a.Remaining)
			set("duration", a.Duration)
			if a.Permanent {
				delete(v.Raw, "remaining")
				delete(v.Raw, "duration")
				v.Text["remaining"] = "∞"
				v.Text["duration"] = "∞"
				full := 1.0
				v.Fraction = &full
			} else {
				v.Fraction = ratio(a.Remaining, a.Duration)
			}
			break
		}
	case "spell", "item", "slot", "gcd":
		if s.Kind == "item" {
			v.Text["name"] = fmt.Sprintf("Item %d", s.ID)
		}
		if s.Kind == "slot" {
			v.Text["name"] = fmt.Sprintf("Slot %d", s.ID)
		}
		if s.Kind == "gcd" {
			v.Text["name"] = "Global cooldown"
		}
		for _, c := range p.Cooldowns {
			if c.Kind != s.Kind || c.ID != s.ID || !c.Available {
				continue
			}
			set("remaining", c.Remaining)
			set("duration", c.Duration)
			v.Fraction = ratio(c.Remaining, c.Duration)
			if c.Remaining != nil && *c.Remaining == 0 && c.Duration != nil && *c.Duration == 0 {
				f := 0.0
				v.Fraction = &f
			}
			if c.MaxCharges != nil && *c.MaxCharges > 0 {
				set("charges", c.Charges)
				set("maxCharges", c.MaxCharges)
				set("recharge", c.Recharge)
				v.Charges = ratio(c.Charges, c.MaxCharges)
				v.Recharge = ratio(c.Recharge, c.RechargeDuration)
				if c.Charges != nil && *c.Charges == *c.MaxCharges {
					f := 0.0
					v.Recharge = &f
					v.setNumber("recharge", &f)
				}
			}
			break
		}
	}
	if v.Fraction != nil {
		percent := *v.Fraction * 100
		v.setNumber("percent", &percent)
	}
	return v
}
func resolveDisplay(v *engine.OverlayDisplay, n Node, s displaySource, p *lookup.OverlayInput, templates ...string) bool {
	return resolveDisplayTrigger(v, n, s, p, nil, templates...)
}
func resolvedDisplayValues(s displaySource, p *lookup.OverlayInput, trigger *triggerContext, actor string) displayValues {
	data := sourceValues(s, p)
	if s.Kind == "value" {
		data = calculatedDisplayValues(s, p, actor, trigger)
	}
	if trigger != nil {
		bossTriggerValues(&data, s, p, trigger.Boss)
	}
	if trigger != nil && trigger.Encounter != nil && s.Kind != "encounter" {
		e := trigger.Encounter
		encounterValues(&data, e.ID, e.Name, e.Difficulty, e.GroupSize, nil)
		data.Text["event"] = e.Kind
		if e.Success != nil {
			if *e.Success {
				data.Text["outcome"] = "Victory"
			} else {
				data.Text["outcome"] = "Wipe"
			}
		}
		if one(s.Kind, "trigger", "encounterEvent") {
			data.Text["name"] = e.Name
			if e.Elapsed != nil {
				data.setNumber("elapsed", e.Elapsed)
			}
		}
	}
	if trigger != nil && trigger.Combat != nil {
		e := trigger.Combat
		for k, value := range map[string]string{"sourceName": e.Source.Name, "targetName": e.Target.Name, "extraName": e.ExtraName, "event": e.Kind, "miss": e.Miss} {
			data.Text[k] = value
		}
		for k, value := range e.Values {
			if k != "stacks" || one(s.Kind, "combatEvent", "trigger", "affected") {
				data.setNumber(k, &value)
			}
		}
		if e.Spell > 0 {
			data.Text["spellID"] = strconv.Itoa(e.Spell)
		}
		if e.ExtraSpell > 0 {
			data.Text["extraSpellID"] = strconv.Itoa(e.ExtraSpell)
		}
		if one(s.Kind, "combatEvent", "trigger") {
			data.Spell = e.Spell
			data.Text["name"] = e.Name
		}
		if s.Kind == "affected" {
			data.Spell = e.ExtraSpell
			data.Text["name"] = e.ExtraName
		}
	} else if trigger != nil && s.Kind == "trigger" && trigger.Spell != nil {
		data.Spell = *trigger.Spell
		data.Text["name"] = spellbook.Name(*trigger.Spell)
	}
	token := s.Unit
	if actor != "" {
		token = boundUnit(p, actor)
	}
	if token == "" {
		token = "player"
	}
	data.Text["unit"] = token
	if member := unitMember(p, token); member != nil {
		data.Text["unitName"] = member.Name
		data.Text["role"] = member.Role
	}
	if token == "player" {
		if data.Text["unitName"] == "" {
			data.Text["unitName"] = p.Character
		}
		data.Text["role"] = p.Role
	}
	return data
}
func resolveDisplayTrigger(v *engine.OverlayDisplay, n Node, s displaySource, p *lookup.OverlayInput, trigger *triggerContext, templates ...string) bool {
	data := resolvedDisplayValues(s, p, trigger, v.Actor)
	for _, entry := range []struct {
		key    string
		target **float64
	}{{"remaining", &v.SortRemaining}, {"stacks", &v.SortStacks}} {
		if f, err := strconv.ParseFloat(data.Text[entry.key], 64); err == nil && bounded(f, 0, 16777214) {
			*entry.target = &f
		}
	}
	if v.SortStacks == nil {
		if f, err := strconv.ParseFloat(data.Text["charges"], 64); err == nil && bounded(f, 0, 999) {
			v.SortStacks = &f
		}
	}
	template := str(n.Props, "text")
	if len(templates) > 0 {
		template = templates[0]
	}
	v.Text = displayToken.ReplaceAllStringFunc(template, func(token string) string {
		key := strings.Trim(token, "{}")
		if value, ok := data.Text[key]; ok && (value != "" || s.Kind == "value" && key == "current") {
			return value
		}
		return "—"
	})
	if strings.Contains(v.Text, "{value:") {
		v.Text = valueText(v.Text, newValueEvaluation(logicFrom(trigger), p, v.Actor, trigger))
	}
	if v.Kind == "icon" || v.Kind == "bar" && v.Visual != nil && !one(v.Visual.BarIcon, "", "none") {
		icon := str(n.Props, "icon")
		v.IconRequested = !one(icon, "", "shield")
		if icon == "auto" {
			v.Spell = data.Spell
		} else if !one(icon, "", "shield") {
			v.Spell, _ = strconv.Atoi(icon)
		}
	}
	fill := str(n.Props, "fill")
	if one(v.Kind, "bar", "arc", "ring", "shape", "image") && !one(fill, "", "timer") {
		f := data.Fraction
		switch fill {
		case "full":
			full := 1.0
			f = &full
		case "charges":
			f = data.Charges
		case "recharge":
			f = data.Recharge
		}
		if f == nil {
			return false
		}
		v.Fraction = f
		if v.Arc != nil {
			v.Arc.Fraction = *f
			v.Arc.Timed = false
		}
	}
	return true
}
