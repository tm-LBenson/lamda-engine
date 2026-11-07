package overlays

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Later matching modifiers override earlier ones. Omitted fields retain the
// base appearance; false/zero/empty are explicit overrides, not missing values.
type AppearanceModifier struct {
	Condition map[string]any `json:"condition"`
	Color     string         `json:"color,omitempty"`
	Size      *float64       `json:"size,omitempty"`
	Font      *float64       `json:"font,omitempty"`
	Opacity   *float64       `json:"opacity,omitempty"`
	Text      *string        `json:"text,omitempty"`
	Glow      *bool          `json:"glow,omitempty"`
	Visible   *bool          `json:"visible,omitempty"`
	Sound     string         `json:"sound,omitempty"`
}

var hexColor = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)

func colorValue(s string) string {
	if c := colors[s]; c != "" {
		return c
	}
	if hexColor.MatchString(s) {
		return strings.ToUpper(s)
	}
	return ""
}
func validModifiers(ms []AppearanceModifier) bool {
	if len(ms) > 16 {
		return false
	}
	for _, m := range ms {
		count := 0
		if !validPredicate(m.Condition, 0, &count) || m.Color != "" && colorValue(m.Color) == "" || !validSound(m.Sound, true) {
			return false
		}
		for _, v := range []struct {
			n      *float64
			lo, hi float64
		}{{m.Size, 16, 400}, {m.Font, 10, 72}, {m.Opacity, 0, 100}} {
			if v.n != nil && !bounded(*v.n, v.lo, v.hi) {
				return false
			}
		}
		if m.Text != nil && len(*m.Text) > 360 {
			return false
		}
	}
	return true
}
func applyAppearance(v *engine.OverlayDisplay, ms []AppearanceModifier, p *lookup.OverlayInput, context ...*triggerContext) bool {
	visible := true
	for _, m := range ms {
		if predicate(m.Condition, p, v.Actor, context...) != yes {
			continue
		}
		if m.Color != "" {
			v.Color = colorValue(m.Color)
		}
		if m.Size != nil {
			v.Size = *m.Size
		}
		if m.Font != nil {
			v.Font = *m.Font
		}
		if m.Opacity != nil {
			v.Opacity = *m.Opacity
		}
		if m.Text != nil {
			v.Text = *m.Text
		}
		if m.Glow != nil {
			v.Glow = *m.Glow
		}
		if m.Visible != nil {
			visible = *m.Visible
		}
	}
	return visible
}
func (r *Runtime) appearanceSounds(rules []Rule, p *lookup.OverlayInput, now time.Time) {
	active := map[string]map[string][]*triggerContext{}
	for key, d := range r.displays {
		if active[d.Rule] == nil {
			active[d.Rule] = map[string][]*triggerContext{}
		}
		active[d.Rule][d.Actor] = append(active[d.Rule][d.Actor], r.displayTriggers[key])
	}
	next := map[string]bool{}
	for _, rule := range rules {
		for i, m := range rule.Style.Modifiers {
			for actor, contexts := range active[rule.ID] {
				key := fmt.Sprintf("%s/appearance/%d/%s/", rule.ID, i, actor)
				matched := false
				if rule.Enabled && loaded(rule.Load, p) {
					for _, context := range contexts {
						if predicate(m.Condition, p, actor, context) == yes {
							matched = true
							break
						}
					}
				}
				if matched && !r.latches[key] && m.Sound != "" && len(r.sounds) < 4 {
					r.serial++
					r.sounds = append(r.sounds, engine.OverlaySound{ID: fmt.Sprintf("%d-%d", now.UnixNano(), r.serial), Name: m.Sound, At: float64(now.UnixMilli()) / 1000})
				}
				next[key] = matched
			}
		}
	}
	for key := range r.latches {
		if strings.Contains(key, "/appearance/") {
			delete(r.latches, key)
		}
	}
	for key, matched := range next {
		r.latches[key] = matched
	}
}

// Visibility edges are observed after conditions, unknown inputs and modifiers
// have resolved. Multiple SHOWs produce one transition for their rule.
func (r *Runtime) lifecycleSounds(rules []Rule, displays []engine.OverlayDisplay, now time.Time) {
	active := map[string]bool{}
	for _, d := range displays {
		active[d.Rule] = true
	}
	for _, rule := range rules {
		was, is := r.lifecycle[rule.ID], active[rule.ID]
		sound := ""
		if is && !was {
			sound = rule.Style.OnShow
		}
		if was && !is {
			sound = rule.Style.OnHide
		}
		if sound != "" && len(r.sounds) < 4 {
			r.serial++
			r.sounds = append(r.sounds, engine.OverlaySound{ID: fmt.Sprintf("%d-%d", now.UnixNano(), r.serial), Name: sound, At: float64(now.UnixMilli()) / 1000})
		}
		r.lifecycle[rule.ID] = is
	}
}
