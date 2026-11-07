package overlays

import (
	"sort"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// A finish animation is visual history, not an active rule or renewed trigger.
// Keep only frames that were actually emitted, with their final layout position.
type finishSnapshot struct {
	Display engine.OverlayDisplay
	Rule    Rule
	Show    Node
	Source  displaySource
	Trigger *triggerContext
	Guards  []guard
	Anchor  *[2]float64
}

func finishAnchor(a *AnchorStyle, p *lookup.OverlayInput, actor string) (*[2]float64, bool) {
	if a == nil {
		return nil, true
	}
	if p == nil || p.AnchorsVersion != 1 {
		return nil, false
	}
	target := a.Target
	if target == "current" {
		target = boundUnit(p, actor)
	}
	for _, v := range p.Anchors {
		if v.Kind == a.Kind && v.Target == target && v.Available {
			px, py := anchorPoint(a.Point)
			return &[2]float64{v.X + v.Width*px, v.Y + v.Height*py}, true
		}
	}
	return nil, false
}
func finishSource(snapshot finishSnapshot, p *lookup.OverlayInput) displaySource {
	source := boundSource(snapshot.Source, p, snapshot.Display.Actor)
	if source.Kind == "castEvent" {
		source.Kind = "none"
		if e := snapshot.Trigger; e != nil && e.Spell != nil {
			source = displaySource{Kind: "castEvent", Unit: e.Unit, ID: *e.Spell}
		}
	}
	return source
}
func (r *Runtime) finishReadable(snapshot finishSnapshot) bool {
	p := r.last
	if p == nil || !snapshot.Rule.Enabled || !loaded(snapshot.Rule.Load, p) {
		return false
	}
	if snapshot.Display.Actor != "" && boundUnit(p, snapshot.Display.Actor) == "" {
		return false
	}
	for _, g := range snapshot.Guards {
		if predicate(g.Node.Props, p, g.Unit, g.Trigger) == unknown {
			return false
		}
	}
	if _, ok := finishAnchor(snapshot.Rule.Style.Anchor, p, snapshot.Display.Actor); !ok {
		return false
	}
	for _, m := range snapshot.Rule.Style.Modifiers {
		if m.Visible != nil && predicate(m.Condition, p, snapshot.Display.Actor, snapshot.Trigger) == unknown {
			return false
		}
	}
	if one(snapshot.Display.Kind, "arc", "ring") && snapshot.Rule.Style.Arc != nil {
		if arc, _ := resolveBoundArc(snapshot.Rule.Style.Arc, snapshot.Show, snapshot.Display, p); arc == nil {
			return false
		}
	}
	check := snapshot.Display
	return resolveDisplayTrigger(&check, snapshot.Show, finishSource(snapshot, p), p, snapshot.Trigger, check.Text)
}
func (r *Runtime) rememberRendered(displays []engine.OverlayDisplay, rules []Rule) {
	next := map[string]finishSnapshot{}
	for _, v := range displays {
		if v.Visual == nil || v.Visual.Motion == nil || v.Visual.Motion.Finish == nil {
			continue
		}
		for _, rule := range rules {
			if rule.ID != v.Rule {
				continue
			}
			parts := strings.Split(strings.TrimSuffix(v.ID, "/"), "/")
			n, source, ok := showSource(rule.Roots, parts[len(parts)-1], displaySource{})
			if !ok {
				break
			}
			anchor, ok := finishAnchor(rule.Style.Anchor, r.last, v.Actor)
			if !ok {
				break
			}
			// Freeze the same untransformed top-left used by the native renderer.
			if v.PixelX == nil {
				w, h := displayGeometry(v)
				x := max(0, min(float64(r.last.Width)-w, v.X*float64(r.last.Width)/100))
				y := max(0, min(float64(r.last.Height)-h, v.Y*float64(r.last.Height)/100))
				v.PixelX = &x
				v.PixelY = &y
			}
			next[v.ID] = finishSnapshot{Display: v, Rule: rule, Show: n, Source: source, Trigger: r.displayTriggers[v.ID], Guards: append([]guard{}, r.displayGuards[v.ID]...), Anchor: anchor}
			break
		}
	}
	r.rendered = next
}
func (r *Runtime) retire(key string, now time.Time) {
	current, exists := r.displays[key]
	snapshot, shown := r.rendered[key]
	if !exists || !shown || current.Instance != snapshot.Display.Instance || !r.finishReadable(snapshot) {
		return
	}
	if old, ok := r.retired[key]; ok && old.Display.Instance == current.Instance {
		return
	}
	v := snapshot.Display
	at := float64(now.UnixMilli()) / 1000
	if v.Visual == nil || v.Visual.Motion == nil || v.Visual.Motion.Finish == nil {
		return
	}
	if v.Fraction == nil && !v.Persistent && one(v.Kind, "bar", "arc", "ring", "shape", "image") {
		fraction := max(0, min(1, (v.Ends-at)/v.Duration))
		v.Fraction = &fraction
	}
	v.Finishing = at
	v.Ends = at + v.Visual.Motion.Finish.Duration
	v.Persistent = true
	snapshot.Display = v
	if len(r.retired) >= 64 {
		oldest := ""
		for id, old := range r.retired {
			if oldest == "" || old.Display.Finishing < r.retired[oldest].Display.Finishing {
				oldest = id
			}
		}
		delete(r.retired, oldest)
	}
	r.retired[key] = snapshot
}
func (r *Runtime) appendRetired(active []engine.OverlayDisplay, now time.Time) []engine.OverlayDisplay {
	keys := make([]string, 0, len(r.retired))
	for key := range r.retired {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := r.retired[keys[i]].Display, r.retired[keys[j]].Display
		if a.Finishing == b.Finishing {
			return keys[i] < keys[j]
		}
		return a.Finishing > b.Finishing
	})
	for _, key := range keys {
		snapshot := r.retired[key]
		v := snapshot.Display
		if v.Ends <= float64(now.UnixMilli())/1000 || !r.finishReadable(snapshot) {
			delete(r.retired, key)
			continue
		}
		if len(active) >= 64 {
			delete(r.retired, key)
			continue
		}
		if snapshot.Anchor != nil {
			point, ok := finishAnchor(snapshot.Rule.Style.Anchor, r.last, v.Actor)
			if !ok {
				delete(r.retired, key)
				continue
			}
			x := *v.PixelX + point[0] - snapshot.Anchor[0]
			y := *v.PixelY + point[1] - snapshot.Anchor[1]
			v.PixelX = &x
			v.PixelY = &y
		}
		active = append(active, v)
	}
	return active
}
