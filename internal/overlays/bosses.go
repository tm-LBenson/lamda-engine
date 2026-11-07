package overlays

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"github.com/tm-LBenson/lamda-engine/internal/spellbook"
)

type bossFilter struct {
	Provider, Text, Kind, Source string
	Spell                        int
	Check, Operator              string
	Value                        float64
}

func bossQuery(p map[string]any) bossFilter {
	id, _ := strconv.Atoi(str(p, "spell"))
	q := bossFilter{Provider: str(p, "provider"), Text: str(p, "textFilter"), Kind: str(p, "timerKind"), Source: str(p, "timelineSource"), Spell: id, Check: str(p, "check"), Operator: str(p, "operator"), Value: num(p, "value")}
	if q.Provider == "" {
		q.Provider = "auto"
	}
	if q.Source == "" {
		q.Source = "any"
	}
	if q.Kind == "" {
		q.Kind = "any"
	}
	if !bounded(q.Value, 0, 1000000) {
		q.Value = 0
	}
	return q
}
func validBossFields(p map[string]any) bool {
	for _, key := range []string{"provider", "spell", "textFilter", "timerKind", "encounterID", "timelineSource"} {
		if v, ok := p[key]; ok {
			if _, ok := v.(string); !ok {
				return false
			}
		}
	}
	return one(str(p, "timelineSource"), "", "any", "encounter", "script") && one(str(p, "provider"), "", "auto", "dbm", "bigwigs", "blizzard") && validCastSpell(str(p, "spell")) && len([]rune(str(p, "textFilter"))) <= 80 && one(str(p, "timerKind"), "", "any", "timer", "cast", "pull", "break") && validCastSpell(str(p, "encounterID"))
}
func validBossWhen(p map[string]any) bool {
	return lookup.ValidBossEvent(str(p, "event")) && validBossFields(p)
}
func validBossCondition(p map[string]any) bool {
	if !validBossFields(p) {
		return false
	}
	if str(p, "condition") == "bossStage" {
		return one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, 10000)
	}
	return one(str(p, "check"), "active", "missing", "paused", "running") || one(str(p, "check"), "remaining", "count") && one(str(p, "operator"), "lt", "lte", "eq", "ne", "gt", "gte") && bounded(num(p, "value"), 0, func() float64 {
		if str(p, "check") == "count" {
			return 1000000
		}
		return 86400
	}())
}
func bossProvidersWanted(rules []Rule) []string {
	rules = executionRules(rules)
	names := map[string]bool{}
	add := func(name string) {
		if name == "" {
			name = "auto"
		}
		names[name] = true
	}
	pred := func(p map[string]any) {
		walkPredicate(p, func(q map[string]any) {
			if one(str(q, "condition"), "bossTimer", "bossStage") {
				add(str(q, "provider"))
			}
		})
	}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == "if" {
				pred(n.Props)
			}
			if n.Type == "when" && lookup.ValidBossEvent(str(n.Props, "event")) {
				add(str(n.Props, "provider"))
			}
			if n.Type == "show" && one(str(n.Props, "dataSource"), "bossTimer", "bossStage") {
				add(str(n.Props, "dataProvider"))
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
	out := []string{}
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
func needsBosses(rules []Rule) bool { return len(bossProvidersWanted(rules)) > 0 }
func (r *Runtime) validateBosses(p *lookup.OverlayInput, rules []Rule) {
	wanted := bossProvidersWanted(rules)
	if len(wanted) == 0 || r.problem != "" {
		return
	}
	if p.BossesVersion != 1 || p.Bosses == nil {
		r.problem = "Reload WoW for boss tracking"
		return
	}
	if p.Bosses.Overflow {
		r.problem = "Boss timer limit reached"
		return
	}
	for _, name := range wanted {
		v := bossProvider(p, name)
		if v == nil || !v.Available {
			label := map[string]string{"auto": "Boss mod", "dbm": "DBM", "bigwigs": "BigWigs", "blizzard": "Blizzard boss data"}[name]
			r.problem = label + " unavailable"
			return
		}
	}
}
func bossProvider(p *lookup.OverlayInput, name string) *lookup.OverlayBossProvider {
	if p.BossesVersion != 1 || p.Bosses == nil {
		return nil
	}
	if one(name, "", "auto") {
		for _, preferred := range []string{"dbm", "bigwigs", "blizzard"} {
			if v := bossProvider(p, preferred); v != nil && v.Available {
				return v
			}
		}
		return nil
	}
	for i := range p.Bosses.Providers {
		if p.Bosses.Providers[i].Name == name {
			return &p.Bosses.Providers[i]
		}
	}
	return nil
}
func bossTimerMatches(q bossFilter, t lookup.OverlayBossTimer) truth {
	result := yes
	if !one(q.Source, "", "any") {
		if t.Source == "" {
			result = unknown
		} else if t.Source != q.Source {
			return no
		}
	}
	if q.Spell > 0 {
		if t.Spell == nil {
			result = unknown
		} else if *t.Spell != q.Spell {
			return no
		}
	}
	if q.Text != "" {
		if t.Name == "" {
			result = unknown
		} else if !strings.Contains(strings.ToLower(t.Name), strings.ToLower(q.Text)) {
			return no
		}
	}
	if !one(q.Kind, "", "any") {
		if t.Kind == "" {
			result = unknown
		} else if t.Kind != q.Kind {
			return no
		}
	}
	return result
}
func bossCondition(q map[string]any, p *lookup.OverlayInput) truth {
	provider := bossProvider(p, str(q, "provider"))
	if provider == nil || !provider.Available {
		return unknown
	}
	if str(q, "condition") == "bossStage" {
		if id, _ := strconv.Atoi(str(q, "encounterID")); id > 0 {
			if provider.EncounterID == 0 {
				return unknown
			}
			if provider.EncounterID != id {
				return no
			}
		}
		if provider.Stage == nil {
			return unknown
		}
		return truthOf(compareNumber(*provider.Stage, num(q, "value"), str(q, "operator")))
	}
	check := str(q, "check")
	uncertain := !provider.Complete
	for _, t := range p.Bosses.Timers {
		if t.Provider != provider.Name {
			continue
		}
		match := bossTimerMatches(bossQuery(q), t)
		if match == no {
			continue
		}
		if match == unknown {
			uncertain = true
			continue
		}
		if check == "missing" {
			return no
		}
		if check == "active" {
			return yes
		}
		if one(check, "paused", "running") {
			if t.Paused == nil {
				uncertain = true
			} else if *t.Paused == (check == "paused") {
				return yes
			}
			continue
		}
		v := t.Remaining
		if check == "count" {
			v = t.Count
		}
		if v == nil {
			uncertain = true
		} else if compareNumber(*v, num(q, "value"), str(q, "operator")) {
			return yes
		}
	}
	if uncertain {
		return unknown
	}
	return truthOf(check == "missing")
}
func bossEventMatches(q map[string]any, p *lookup.OverlayInput, e *lookup.OverlayBossEvent) bool {
	if e == nil || e.Kind != str(q, "event") {
		return false
	}
	provider := bossProvider(p, str(q, "provider"))
	if provider == nil || !provider.Available || provider.Name != e.Timer.Provider {
		return false
	}
	if id, _ := strconv.Atoi(str(q, "encounterID")); id > 0 && e.EncounterID != id {
		return false
	}
	return bossTimerMatches(bossQuery(q), e.Timer) == yes
}
func bossTimerSource(q bossFilter, p *lookup.OverlayInput) *lookup.OverlayBossTimer {
	provider := bossProvider(p, q.Provider)
	if provider == nil || !provider.Available {
		return nil
	}
	var selected *lookup.OverlayBossTimer
	for i := range p.Bosses.Timers {
		t := &p.Bosses.Timers[i]
		if t.Provider != provider.Name || bossTimerMatches(q, *t) != yes || bossTimerCheck(q, *t) != yes {
			continue
		}
		if selected == nil || t.Remaining != nil && (selected.Remaining == nil || *t.Remaining < *selected.Remaining) || ((t.Remaining == nil && selected.Remaining == nil || t.Remaining != nil && selected.Remaining != nil && *t.Remaining == *selected.Remaining) && t.ID < selected.ID) {
			selected = t
		}
	}
	return selected
}
func (r *Runtime) evaluateBosses(rules []Rule, p *lookup.OverlayInput, now time.Time, fresh bool) {
	b := p.Bosses
	if p.BossesVersion != 1 || b == nil {
		r.bossCursor = -1
		return
	}
	if fresh || r.bossCursor < 0 || b.Seq < r.bossCursor {
		r.bossCursor = b.Seq
		return
	}
	events := append([]lookup.OverlayBossEvent(nil), b.Events...)
	sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })
	if b.Seq-r.bossCursor > 8 && r.problem == "" {
		r.problem = "Some boss events were missed"
	}
	for _, event := range events {
		if event.Seq <= r.bossCursor {
			continue
		}
		r.currentTrigger = &triggerContext{Boss: &event}
		for _, rule := range rules {
			if rule.Enabled && loaded(rule.Load, p) {
				r.evaluate(rule, event.Kind, p, now.Add(-time.Duration(event.Age)*time.Millisecond))
			}
		}
	}
	r.currentTrigger = nil
	r.bossCursor = b.Seq
}

func bossTimerCheck(q bossFilter, t lookup.OverlayBossTimer) truth {
	switch q.Check {
	case "", "active":
		return yes
	case "missing":
		return no
	case "paused", "running":
		if t.Paused == nil {
			return unknown
		}
		return truthOf(*t.Paused == (q.Check == "paused"))
	case "remaining", "count":
		v := t.Remaining
		if q.Check == "count" {
			v = t.Count
		}
		if v == nil {
			return unknown
		}
		return truthOf(compareNumber(*v, q.Value, q.Operator))
	}
	return unknown
}
func bossValues(data *displayValues, t *lookup.OverlayBossTimer) {
	if t == nil {
		return
	}
	data.Text["name"] = t.Name
	data.Text["provider"] = map[string]string{"dbm": "DBM", "bigwigs": "BigWigs", "blizzard": "Blizzard"}[t.Provider]
	if t.ID > 0 {
		data.Text["timerID"] = strconv.Itoa(t.ID)
	}
	if t.Spell != nil {
		data.Spell = *t.Spell
		data.Text["spellID"] = strconv.Itoa(*t.Spell)
		if data.Text["name"] == "" {
			data.Text["name"] = spellbook.Name(*t.Spell)
		}
	}
	for k, v := range map[string]*float64{"remaining": t.Remaining, "duration": t.Duration, "count": t.Count} {
		if v != nil {
			data.setNumber(k, v)
		}
	}
	if t.Remaining != nil && t.Duration != nil {
		if *t.Duration > 0 && *t.Remaining <= *t.Duration {
			v := *t.Remaining / *t.Duration
			data.Fraction = &v
		} else if *t.Remaining == 0 && *t.Duration == 0 {
			v := 0.0
			data.Fraction = &v
		}
	}
	if data.Fraction != nil {
		percent := *data.Fraction * 100
		data.setNumber("percent", &percent)
	}
}
func bossTriggerValues(data *displayValues, s displaySource, p *lookup.OverlayInput, e *lookup.OverlayBossEvent) {
	if e == nil || one(s.Kind, "bossTimer", "bossStage") {
		return
	}
	data.Text["provider"] = map[string]string{"dbm": "DBM", "bigwigs": "BigWigs", "blizzard": "Blizzard"}[e.Timer.Provider]
	data.Text["event"] = e.Kind
	if e.Timer.ID > 0 {
		data.Text["timerID"] = strconv.Itoa(e.Timer.ID)
	}
	if e.Timer.Count != nil {
		data.setNumber("count", e.Timer.Count)
	}
	if e.Stage != nil {
		data.setNumber("stage", e.Stage)
	}
	if e.StageTotal != nil {
		data.setNumber("stageTotal", e.StageTotal)
	}
	if !one(s.Kind, "bossEvent", "trigger") {
		return
	}
	timer := e.Timer
	if s.Kind == "bossEvent" && timer.ID > 0 {
		// Automatic follows this lifetime, including pauses and timer corrections.
		// An incomplete provider snapshot cannot prove this timer has stopped.
		timer.Remaining = nil
		if provider := bossProvider(p, timer.Provider); provider != nil && provider.Available {
			if provider.Complete {
				v := 0.0
				timer.Remaining = &v
			}
			for _, current := range p.Bosses.Timers {
				if current.ID == timer.ID && current.Provider == timer.Provider {
					timer = current
					break
				}
			}
		}
	}
	bossValues(data, &timer)
}
