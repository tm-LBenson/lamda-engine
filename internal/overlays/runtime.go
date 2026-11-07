package overlays

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type scope struct {
	Trigger *triggerContext
	Event   string
	Started time.Time
	Path    string
}
type guard struct {
	Trigger *triggerContext
	Path    string
	Node    Node
	Unit    string
}
type pending struct {
	Trigger *triggerContext
	Actor   string
	Guards  []guard
	Rule    string
	Node    Node
	At      time.Time
	Style   Style
	Key     string
}
type Runtime struct {
	savedMemory     *savedMemory
	memory          map[string]*rememberState
	rendered        map[string]finishSnapshot
	retired         map[string]finishSnapshot
	displayGuards   map[string][]guard
	bossCursor      int
	encounterCursor int
	combatProblem   string
	connected       time.Time
	castCursors     map[string]castCursor
	currentTrigger  *triggerContext
	displayTriggers map[string]*triggerContext
	initialized     bool
	last            *lookup.OverlayInput
	seen            time.Time
	window          uintptr
	pid             uint32
	position        lookup.BridgePosition
	scopes          map[string]scope
	latches         map[string]bool
	soundLatches    map[string]bool
	lifecycle       map[string]bool
	cloneRoots      map[string]string
	queue           []pending
	held            map[string][]guard
	displays        map[string]engine.OverlayDisplay
	sounds          []engine.OverlaySound
	serial          uint64
	problem         string
}

func NewRuntime() *Runtime { r := &Runtime{savedMemory: newSavedMemory()}; r.Reset(); return r }
func (r *Runtime) Reset() {
	r.memory = map[string]*rememberState{}
	r.rendered = map[string]finishSnapshot{}
	r.retired = map[string]finishSnapshot{}
	r.displayGuards = map[string][]guard{}
	r.initialized = false
	r.encounterCursor = -1
	r.bossCursor = -1
	r.combatProblem = ""
	r.castCursors = map[string]castCursor{}
	r.displayTriggers = map[string]*triggerContext{}
	r.currentTrigger = nil
	r.scopes = map[string]scope{}
	r.latches = map[string]bool{}
	r.soundLatches = map[string]bool{}
	r.lifecycle = map[string]bool{}
	r.cloneRoots = map[string]string{}
	r.queue = nil
	r.held = map[string][]guard{}
	r.displays = map[string]engine.OverlayDisplay{}
	r.sounds = nil
}

// A capture interruption is not a new condition edge. Drop all output and
// pending actions, but remember sounds already played until a confirmed false
// reading, rule unload, or actual context change rearms them.
func (r *Runtime) resetCapture() {
	sounds := r.soundLatches
	r.Reset()
	r.soundLatches = sounds
}
func (r *Runtime) Status(now time.Time) any {
	state := "Waiting for WoW"
	if r.last != nil && now.Sub(r.seen) <= engine.FrameLifetime {
		state = "Connected"
	}
	if state == "Connected" && r.combatProblem != "" {
		state = r.combatProblem
	}
	if r.problem != "" {
		state = r.problem
	}
	return struct {
		Status    string               `json:"status"`
		Character *lookup.OverlayInput `json:"character,omitempty"`
	}{state, r.last}
}
func (s *Store) Accept(sample lookup.BridgeSample, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.captureDisabled {
		return
	}
	s.runtime.Accept(s.rulesForExecution(), sample, now)
	s.updateCombatStatus()
}
func (r *Runtime) Accept(rules []Rule, s lookup.BridgeSample, now time.Time) {
	rules = executionRules(rules)
	p := s.Overlays
	if s.Error != "" || p == nil || now.Unix()-p.At > 3 || p.At-now.Unix() > 3 {
		return
	}
	if r.last != nil && p.Epoch == r.last.Epoch && p.Seq <= r.last.Seq {
		return
	} // A frozen packet cannot renew the display lease.
	// Runtime validation must not erase readings in the shared capture sample.
	input := *p
	input.Identities = append([]lookup.OverlayIdentity(nil), p.Identities...)
	input.Casts = append([]lookup.OverlayCast(nil), p.Casts...)
	input.Watches = append([]lookup.OverlayAura(nil), p.Watches...)
	input.Cooldowns = append([]lookup.OverlayCooldown(nil), p.Cooldowns...)
	input.Measurements = append([]lookup.OverlayMeasurement(nil), p.Measurements...)
	p = &input

	old := r.last
	newSession := old == nil || old.Epoch != p.Epoch || old.Character != p.Character || old.Realm != p.Realm
	fresh := !r.initialized || newSession || now.Sub(r.seen) > engine.FrameLifetime || r.window != s.Window || r.pid != s.WindowPID
	if old != nil && (old.Width != p.Width || old.Height != p.Height) {
		r.rendered = map[string]finishSnapshot{}
		r.retired = map[string]finishSnapshot{}
	}
	loadoutFresh := fresh
	if fresh || old.Spec != p.Spec {
		if !newSession && old.Spec == p.Spec && old.Class == p.Class && r.window == s.Window && r.pid == s.WindowPID {
			r.resetCapture()
		} else {
			r.Reset()
		}
		fresh = true
	}
	r.last = p
	r.seen = now
	r.window = s.Window
	r.pid = s.WindowPID
	r.position = s.Position
	r.problem = ""
	// Each packet describes the IDs currently loaded by WoW, in pixel order.
	// Validate against that list, not the latest editor revision: adding another
	// watch must not discard healthy readings for already-loaded auras.
	captured := make([]Watch, 0, len(p.Watches))
	known := map[string]bool{}
	mapping := true
	for _, w := range p.Watches {
		if known[w.Key()] {
			mapping = false
		}
		known[w.Key()] = true
		captured = append(captured, Watch{w.Spell, w.Kind, w.Unit})
	}
	currentRules := []Rule{}
	for _, rule := range rules {
		if baseLoaded(rule.Load, p) {
			currentRules = append(currentRules, rule)
		}
	}
	expected := Watches(currentRules)
	if !mapping || p.Hash != WatchHash(captured) {
		r.problem = "Reload WoW to refresh aura tracking"
		p.Watches = nil
	} else {
		for _, w := range expected {
			if !known[w.Key()] {
				r.problem = "Reload WoW to watch new auras"
				break
			}
		}
	}
	if !p.Native && len(expected) > 0 {
		r.problem = "Native aura capture unavailable"
	}
	if p.CaptureError != "" {
		r.problem = "Aura capture: " + p.CaptureError
		p.Watches = nil // Failed setup is unknown, never proof of an absent buff.
	}
	for _, rule := range rules {
		var usesResources func([]Node) bool
		usesResources = func(nodes []Node) bool {
			for _, n := range nodes {
				needed := false
				if n.Type == "if" {
					walkPredicate(n.Props, func(p map[string]any) {
						if str(p, "condition") == "resource" {
							needed = true
						}
					})
				}
				if needed || usesResources(n.Children) {
					return true
				}
			}
			return false
		}
		modifierResource := false
		for _, m := range rule.Style.Modifiers {
			walkPredicate(m.Condition, func(q map[string]any) {
				if str(q, "condition") == "resource" {
					modifierResource = true
				}
			})
		}
		if rule.Enabled && loaded(rule.Load, p) && (modifierResource || usesResources(rule.Roots) || usesResources(valueInputNodes(rule)) || rule.Style.Arc != nil && rule.Style.Arc.Fill == "resource" && hasArcs(rule.Roots)) {
			if p.ResourcesVersion != 1 {
				r.problem = "Reload WoW for resource tracking"
			} else if !p.Native {
				r.problem = "Native resource capture unavailable"
			}
			break
		}
	}
	r.validateCooldowns(p, currentRules)
	r.validateCasts(p, currentRules)
	r.validateCombat(p, currentRules)
	if needsPet(currentRules) && (p.PetVersion != 1 || p.Pet == nil) && r.problem == "" {
		r.problem = "Reload WoW for pet tracking"
	}
	if needsEncounters(currentRules) && (p.EncountersVersion != 1 || p.Encounter == nil) && r.problem == "" {
		r.problem = "Reload WoW for encounter tracking"
	}
	r.validateBosses(p, currentRules)
	r.validateTalents(p, currentRules)
	r.validateEquipment(p, currentRules)
	r.validateLoadoutChanges(p, currentRules)
	r.validateMeasurements(p, currentRules)
	if r.problem == "" {
		for _, rule := range currentRules {
			if rule.Enabled {
				if problem := displayDataProblem(rule, p); problem != "" {
					r.problem = problem
					break
				}
			}
		}
	}
	if r.problem == "" {
		for _, w := range Anchors(currentRules) {
			found := false
			for _, a := range p.Anchors {
				if w.Kind == a.Kind && w.Target == a.Target {
					found = true
					break
				}
			}
			if p.AnchorsVersion != 1 || !found {
				r.problem = "Reload WoW for new attachments"
				break
			}
		}
	}
	if !unitsCaptured(p, currentRules) && r.problem == "" {
		r.problem = "Reload WoW for unit tracking"
	}
	for path, actor := range r.cloneRoots {
		if boundUnit(p, actor) == "" {
			r.remove(path)
			for key := range r.displays {
				if strings.HasPrefix(key, path) {
					delete(r.displays, key)
					delete(r.displayTriggers, key)
					delete(r.displayGuards, key)
				}
			}
			delete(r.cloneRoots, path)
		}
	}
	r.clearChangedUnits(rules, p, old)
	r.refreshConditions(p, now)
	events := []string{}
	if newSession {
		events = append(events, "login")
		if p.Location != "world" {
			events = append(events, "enter")
		}
	} else {
		if p.Location != old.Location || p.Zone != old.Zone {
			r.Reset()
			fresh = true
			if old.Location != "world" {
				events = append(events, "leave")
			}
			if p.Location != "world" {
				events = append(events, "enter")
			}
		}
		if p.Combat != old.Combat {
			if p.Combat {
				events = append(events, "combat")
			} else {
				events = append(events, "combatend")
			}
		}
		if p.Key != old.Key {
			if p.Key {
				events = append(events, "key")
			} else {
				events = append(events, "keyend")
				for k, v := range r.scopes {
					if v.Event == "key" {
						r.removeAt(k, now, true)
					}
				}
			}
		}
		if p.CastSeq != old.CastSeq {
			events = append(events, "ability")
		}
	}
	if fresh || !reflect.DeepEqual(p.Health, old.Health) || measurementChanged(p, old, "health") {
		events = append(events, "health")
	}
	if fresh || !reflect.DeepEqual(p.Resources, old.Resources) || p.PrimaryPower != old.PrimaryPower || measurementChanged(p, old, "resource") {
		events = append(events, "resource")
	}
	// Native remaining time crosses thresholds between UNIT_AURA events.
	events = append(events, "aura", "cooldown", "casting", "state")
	for _, rule := range rules {
		if !rule.Enabled || !loaded(rule.Load, p) {
			r.clearRule(rule.ID)
			continue
		}
		for _, event := range events {
			r.evaluate(rule, event, p, now)
		}
	}
	r.evaluateCasts(rules, p, now, fresh)
	r.evaluateEncounters(rules, p, now, fresh)
	r.evaluateBosses(rules, p, now, fresh)
	r.evaluateLoadoutChanges(rules, p, old, now, loadoutFresh)
	r.advance(now)
	r.appearanceSounds(rules, p, now)
	if fresh {
		r.connected = now
	}
	r.initialized = true
}
func loaded(l Load, p *lookup.OverlayInput) bool {
	return baseLoaded(l, p) && buildLoadMatches(l.Conditions, p)
}
func baseLoaded(l Load, p *lookup.OverlayInput) bool {
	for _, v := range [][2]string{{l.Character, p.Character}, {l.Realm, p.Realm}, {l.Class, p.Class}, {l.Race, p.Race}, {l.Faction, p.Faction}, {l.Role, p.Role}, {l.Location, p.Location}, {l.Group, p.Group}, {l.Zone, p.Zone}} {
		if v[0] != "" && !strings.EqualFold(strings.TrimSpace(v[0]), v[1]) {
			return false
		}
	}
	for _, v := range []struct {
		s string
		b bool
	}{{l.Combat, p.Combat}, {l.Mounted, p.Mounted}, {l.Alive, p.Alive}} {
		if v.s != "" && ((v.s == "yes") != v.b) {
			return false
		}
	}
	return (l.Spec == 0 || l.Spec == p.Spec) && (l.MinLevel == 0 || p.Level >= l.MinLevel)
}
func compare(value, threshold float64, operator string) bool {
	switch operator {
	case "", "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	case "ne":
		return value != threshold
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	}
	return false
}
func conditionLeaf(n Node, p *lookup.OverlayInput, unit string) bool {
	q := n.Props
	unit = boundUnit(p, unit)
	switch str(q, "condition") {
	case "cooldown":
		return cooldownCondition(q, p)
	case "role":
		if unit == "player" {
			return p.Role == str(q, "value")
		}
		m := unitMember(p, unit)
		return m != nil && m.Role == str(q, "value")
	case "location":
		return p.Location == str(q, "value")
	case "health":
		h := unitHealth(p, selectedUnit(unit, str(q, "unit")))
		return h != nil && compare(*h, num(q, "value"), str(q, "operator"))
	case "resource":
		v, _, ok := unitResource(p, selectedUnit(unit, str(q, "unit")), str(q, "resource"))
		if !ok {
			return false
		}
		if str(q, "operator") == "cap" {
			return v.Current >= v.Max
		}
		amount := v.Current
		if str(q, "measure") == "percent" {
			amount = 100 * v.Current / v.Max
		}
		return compare(amount, num(q, "value"), str(q, "operator"))
	case "buff", "debuff":
		spell, _ := strconv.Atoi(str(q, "aura"))
		u := str(q, "unit")
		if u == "" {
			u = "player"
		}
		if u == "current" {
			u = unit
		}
		for _, a := range p.Watches {
			if a.Spell != spell || a.Kind != str(q, "condition") || a.Unit != u {
				continue
			}
			if a.Active == nil {
				return false
			}
			switch str(q, "check") {
			case "active":
				return *a.Active
			case "missing":
				return !*a.Active
			}
			if !*a.Active {
				return false
			}
			v := a.Stacks
			if str(q, "check") == "remaining" {
				if a.Permanent {
					return false
				}
				v = a.Remaining
			}
			if v == nil {
				return false
			}
			t := num(q, "value")
			switch str(q, "operator") {
			case "eq":
				return *v == t
			case "ne":
				return *v != t
			case "lt":
				return *v < t
			case "lte":
				return *v <= t
			case "gt":
				return *v > t
			case "gte":
				return *v >= t
			}
			return false
		}
	}
	return false
}
func hasWhen(ns []Node) bool {
	for _, n := range ns {
		if n.Type == "when" || hasWhen(n.Children) {
			return true
		}
	}
	return false
}

// Evaluate held displays and delayed actions on every fresh capture, even when
// the event that originally started their WHEN has not happened again.
func (r *Runtime) refreshConditions(p *lookup.OverlayInput, now time.Time) {
	invalid := map[string]guard{}
	check := func(guards []guard) {
		for _, g := range guards {
			value := predicate(g.Node.Props, p, g.Unit, g.Trigger)
			if value != yes {
				invalid[g.Path] = g
			}
		}
	}
	for _, guards := range r.held {
		check(guards)
	}
	for _, action := range r.queue {
		check(action.Guards)
	}
	for path, g := range invalid {
		r.removeCondition(path, g.Node, now, predicate(g.Node.Props, p, g.Unit, g.Trigger))
	}
	for key, guards := range r.held {
		confirmed := true
		for _, g := range guards {
			if predicate(g.Node.Props, p, g.Unit, g.Trigger) != yes {
				confirmed = false
				break
			}
		}
		v, ok := r.displays[key]
		if ok && confirmed {
			v.Ends = float64(now.Add(engine.FrameLifetime).UnixMilli()) / 1000
			r.displays[key] = v
		}
	}
}
func (r *Runtime) remove(path string) { r.removeAt(path, time.Time{}, false) }
func (r *Runtime) removeAt(path string, now time.Time, finish bool) {
	r.removeBranch(path, now, finish, false, false)
}
func (r *Runtime) removeCondition(path string, n Node, now time.Time, value truth) {
	if value != unknown {
		r.removeAt(path, now, true)
		return
	}
	// Paired cooldown pixels can be unreadable for one repaint. Keep a ready
	// cue only for the remainder of its last confirmed lease, never extending it
	// from unknown data. Other predicates retain their existing fail-closed UI.
	ready := str(n.Props, "condition") == "cooldown" && str(n.Props, "check") == "ready"
	r.removeBranch(path, now, false, true, ready)
}
func (r *Runtime) removeBranch(path string, now time.Time, finish, preserveSounds, retainReady bool) {
	retained := map[string]bool{}
	if retainReady {
		for key := range r.held {
			if v, ok := r.displays[key]; ok && strings.HasPrefix(key, path) && v.Ends > float64(now.UnixMilli())/1000 {
				retained[key] = true
			}
		}
	}
	if !finish {
		for key := range r.retired {
			if strings.HasPrefix(key, path) {
				delete(r.retired, key)
			}
		}
	}
	for prefix := range r.cloneRoots {
		if strings.HasPrefix(prefix, path) {
			delete(r.cloneRoots, prefix)
		}
	}
	for key := range r.held {
		if strings.HasPrefix(key, path) && !retained[key] {
			if finish {
				r.retire(key, now)
			}
			delete(r.held, key)
			delete(r.displays, key)
			delete(r.displayTriggers, key)
			delete(r.displayGuards, key)
		}
	}
	for key := range r.rendered {
		if strings.HasPrefix(key, path) && !retained[key] {
			delete(r.rendered, key)
		}
	}
	q := r.queue[:0]
	for _, a := range r.queue {
		if !strings.HasPrefix(a.Key, path) {
			q = append(q, a)
		}
	}
	r.queue = q
	for k := range r.scopes {
		if strings.HasPrefix(k, path) {
			delete(r.scopes, k)
		}
	}
	for k := range r.latches {
		if strings.HasPrefix(k, path) && !retained[k] {
			delete(r.latches, k)
		}
	}
	if !preserveSounds {
		for k := range r.soundLatches {
			if strings.HasPrefix(k, path) {
				delete(r.soundLatches, k)
			}
		}
	}
}
func (r *Runtime) clearRule(id string) {
	delete(r.memory, id)
	r.remove(id + "/")
	for k, v := range r.displays {
		if v.Rule == id {
			delete(r.displays, k)
			delete(r.displayTriggers, k)
			delete(r.displayGuards, k)
		}
	}
	q := r.queue[:0]
	for _, a := range r.queue {
		if a.Rule != id {
			q = append(q, a)
		}
	}
	r.queue = q
}
func (r *Runtime) evaluate(rule Rule, event string, p *lookup.OverlayInput, now time.Time) {
	rule = executionRule(rule)
	r.rememberEvent(rule, event, p)
	var run func([]Node, string, string, time.Time, time.Duration, bool, []guard, *triggerContext) time.Duration
	run = func(ns []Node, path, unit string, since time.Time, delay time.Duration, execute bool, guards []guard, context *triggerContext) time.Duration {
		for _, n := range ns {
			key := path + n.ID + "/"
			switch n.Type {
			case "sequence":
				delay = run(n.Children, key, unit, since, delay, execute, guards, context)
			case "when":
				sc, active := r.scopes[key]
				fires := !execute && (!active || r.currentTrigger != nil) && str(n.Props, "event") == event && r.triggerMatches(n, p, unit) && !now.Before(since.Add(delay)) && (strings.Count(path, "/") == 1 || now.After(since) || r.currentTrigger != nil)
				if fires {
					if r.currentTrigger != nil {
						r.remove(key)
						context = r.ruleContext(rule, r.currentTrigger)
					}
					if hasWhen(n.Children) {
						r.scopes[key] = scope{Event: event, Started: now, Path: key, Trigger: context}
					}
					run(n.Children, key, unit, now, 0, true, guards, context)
				} else if active {
					run(n.Children, key, unit, sc.Started, 0, false, guards, sc.Trigger)
				}
			case "if":
				value := predicate(n.Props, p, unit, context)
				if value == yes {
					next := append(append([]guard{}, guards...), guard{Path: key, Node: n, Unit: unit, Trigger: context})
					delay = run(n.Children, key, unit, since, delay, execute, next, context)
				} else {
					r.removeCondition(key, n, now, value)
					q := r.queue[:0]
					for _, a := range r.queue {
						if !strings.HasPrefix(a.Key, key) {
							q = append(q, a)
						}
					}
					r.queue = q
				}
			case "each":
				endDelay := delay
				if p.UnitsVersion == 1 || p.UnitsVersion == 2 {
					for _, u := range cloneMembers(p, str(n.Props, "scope")) {
						actor := cloneActor(str(n.Props, "scope"), u)
						prefix := key + actor + "/"
						r.cloneRoots[prefix] = actor
						if end := run(n.Children, prefix, actor, since, delay, execute, guards, context); end > endDelay {
							endDelay = end
						}
					}
				}
				delay = endDelay
			case "wait":
				delay += time.Duration(num(n.Props, "seconds") * float64(time.Second))
			default:
				if execute {
					edge := one(event, "state", "aura", "health", "resource", "cooldown", "casting")
					latched := r.latches[key] || n.Type == "sound" && r.soundLatches[key]
					if (!edge || !latched) && len(r.queue) < 256 {
						r.queue = append(r.queue, pending{Trigger: context, Actor: func() string {
							if unit != "" && !lookup.OverlayUnitToken(unit) {
								return unit
							}
							return ""
						}(), Rule: rule.ID, Node: n, At: now.Add(delay), Style: rule.Style, Key: key, Guards: guards})
						r.latches[key] = true
					}
				}
			}
		}
		return delay
	}
	run(rule.Roots, rule.ID+"/", "player", now, 0, false, nil, r.ruleContext(rule, nil))
}
func (r *Runtime) advance(now time.Time) {
	sort.SliceStable(r.queue, func(i, j int) bool { return r.queue[i].At.Before(r.queue[j].At) })
	q := r.queue[:0]
	for _, a := range r.queue {
		if a.At.After(now) {
			q = append(q, a)
			continue
		}
		p := a.Node.Props
		switch a.Node.Type {
		case "show":
			if _, exists := r.displays[a.Key]; !exists && len(r.displays) >= 64 {
				r.problem = "Display limit reached"
				continue
			}
			r.serial++
			duration := num(p, "duration")
			persistent := str(p, "visibility") == "conditions"
			if persistent {
				duration = engine.FrameLifetime.Seconds()
				r.held[a.Key] = a.Guards
			} else {
				delete(r.held, a.Key)
			}
			delete(r.retired, a.Key)
			r.displayGuards[a.Key] = append([]guard{}, a.Guards...)
			r.displayTriggers[a.Key] = a.Trigger
			r.displays[a.Key] = engine.OverlayDisplay{Started: float64(now.UnixMilli()) / 1000, Instance: fmt.Sprintf("%d-%d", now.UnixNano(), r.serial), Actor: a.Actor, ID: a.Key, Rule: a.Rule, Kind: str(p, "display"), Text: str(p, "text"), X: a.Style.X, Y: a.Style.Y, Size: a.Style.Size, Font: a.Style.Font, Opacity: a.Style.Opacity, Color: colorValue(a.Style.Color), Transparent: a.Style.Transparent, Persistent: persistent, Duration: duration, Ends: float64(now.UnixMilli())/1000 + duration}
		case "hide":
			for k, v := range r.displays {
				if v.Rule == a.Rule && (a.Actor == "" || v.Actor == a.Actor) {
					r.retire(k, now)
					delete(r.held, k)
					delete(r.displays, k)
					delete(r.displayTriggers, k)
					delete(r.displayGuards, k)
				}
			}
		case "glow":
			for k, v := range r.displays {
				if v.Rule == a.Rule && (a.Actor == "" || v.Actor == a.Actor) {
					v.Glow = true
					v.Color = colorValue(str(p, "color"))
					r.displays[k] = v
				}
			}
		case "sound":
			if len(r.sounds) >= 4 {
				continue
			}
			r.serial++
			r.soundLatches[a.Key] = true
			r.sounds = append(r.sounds, engine.OverlaySound{ID: fmt.Sprintf("%d-%d", now.UnixNano(), r.serial), Name: str(p, "sound"), At: float64(now.UnixMilli()) / 1000})
		}
	}
	r.queue = q
	for k, v := range r.displays {
		if v.Ends <= float64(now.UnixMilli())/1000 {
			if !v.Persistent {
				r.retire(k, now)
			}
			delete(r.held, k)
			delete(r.displays, k)
			delete(r.displayTriggers, k)
			delete(r.displayGuards, k)
		}
	}
	sounds := r.sounds[:0]
	for _, a := range r.sounds {
		if float64(now.UnixMilli())/1000-a.At < 1 {
			sounds = append(sounds, a)
		}
	}
	r.sounds = sounds
}
func (s *Store) Merge(d engine.FrameDisplay, now time.Time) engine.FrameDisplay {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.captureDisabled {
		return d
	}
	r := s.runtime
	if r.last == nil || now.Before(r.seen) || now.Sub(r.seen) > engine.FrameLifetime {
		r.resetCapture()
		return d
	}
	r.advance(now)

	window := strconv.FormatUint(uint64(r.window), 10)
	if d.Enabled && (d.Window != window || d.Width != r.last.Width || d.Height != r.last.Height) {
		return d
	}
	d.Enabled = true
	d.OverlaysActive = true
	d.Status = "receiving"
	d.Window = window
	d.WindowPID = r.pid
	d.Width = r.last.Width
	d.Height = r.last.Height
	d.At = float64(now.UnixMilli()) / 1000
	d.Expires = float64(r.seen.Add(engine.FrameLifetime).UnixMilli()) / 1000
	keys := []string{}
	for k := range r.displays {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := r.displays[k]
		visible, readable := true, true
		for _, rule := range s.rulesForExecution() {
			if rule.ID != v.Rule {
				continue
			}
			path := strings.Split(strings.TrimSuffix(v.ID, "/"), "/")
			n, source, ok := showSource(rule.Roots, path[len(path)-1], displaySource{})
			if !ok {
				visible = false
				break
			}
			if one(v.Kind, "arc", "ring") && rule.Style.Arc != nil {
				v.Arc, visible = resolveBoundArc(rule.Style.Arc, n, v, r.last)
				readable = v.Arc != nil
			}
			source = boundSource(source, r.last, v.Actor)
			if source.Kind == "castEvent" {
				source.Kind = "none"
				if e := r.displayTriggers[k]; e != nil && e.Spell != nil {
					source = displaySource{Kind: "castEvent", Unit: e.Unit, ID: *e.Spell}
				}
			}
			visible = applyAppearance(&v, rule.Style.Modifiers, r.last, r.displayTriggers[k]) && visible
			v.Visual = resolveVisual(rule.Style.Visual)
			if v.Kind == "image" {
				if v.Visual != nil {
					v.ImagePath = s.mediaPathLocked(v.Visual.Image, "image")
					asset := s.mediaAssetLocked(v.Visual.Image)
					readable = readable && spriteFits(v.Visual.Sprite, asset.Width, asset.Height)
				}
				readable = readable && v.ImagePath != ""
			}
			readable = resolveDisplayTrigger(&v, n, source, r.last, r.displayTriggers[k], v.Text) && readable
			visible = visible && readable
			if one(source.Kind, "", "none") && !v.Persistent {
				remaining := max(0, v.Ends-float64(now.UnixMilli())/1000)
				v.SortRemaining = &remaining
			}
			if v.Spell > 0 {
				v.IconPath = s.icons.Request(v.Spell)
			}
			break
		}
		if one(v.Kind, "arc", "ring") && (v.Fraction != nil && *v.Fraction <= 0 || v.Arc != nil && (v.Arc.Count == 0 || !v.Arc.Timed && v.Arc.Fraction <= 0)) {
			visible = false
		}
		if visible {
			delete(r.retired, k)
			d.Overlays = append(d.Overlays, v)
		} else if readable {
			r.retire(k, now)
		} else {
			delete(r.retired, k)
		}
	}
	layoutGroups(d.Overlays, s.rulesForExecution(), d.Width, d.Height)
	d.Overlays = placeAnchors(d.Overlays, s.rulesForExecution(), r.last)
	r.lifecycleSounds(s.rulesForExecution(), d.Overlays, now)
	r.rememberRendered(d.Overlays, s.rulesForExecution())
	d.Overlays = r.appendRetired(d.Overlays, now)
	// Finished visuals do not participate in active layout or lifecycle edges.
	sort.SliceStable(d.Overlays, func(i, j int) bool { return visualLayer(d.Overlays[i]) < visualLayer(d.Overlays[j]) })
	for _, sound := range r.sounds {
		if strings.HasPrefix(sound.Name, "media:") {
			sound.Path = s.mediaPathLocked(strings.TrimPrefix(sound.Name, "media:"), "sound")
			if sound.Path == "" {
				continue
			}
		}
		d.OverlaySounds = append(d.OverlaySounds, sound)
	}
	if lookup.ValidOverlayWidth(r.last.StripWidth) {
		width, height := r.last.StripWidth, 18432/r.last.StripWidth
		x := max(0, min(r.position.X, d.Width-width))
		y := max(0, min(r.position.Y, d.Height-height))
		engine.ReserveCaptureArea(&d, x, y, width, height)
	} else {
		x := max(0, min(r.position.X, d.Width-256))
		y := max(0, min(r.position.Y, d.Height-32))
		engine.ReserveCaptureArea(&d, x, y+min(0, r.last.Offset), 256, 72)
	}
	return d
}
