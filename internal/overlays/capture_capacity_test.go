package overlays

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"strings"
	"testing"
	"time"
)

func captureRules(count int, source string) []Rule {
	rules := make([]Rule, count)
	for i := range rules {
		r := rule()
		r.ID = fmt.Sprintf("capture-%d", i)
		p := map[string]any{"condition": "buff", "aura": fmt.Sprint(1000 + i), "unit": "player", "check": "active"}
		switch source {
		case "timer":
			p = map[string]any{"condition": "cooldown", "source": "spell", "id": fmt.Sprint(1000 + i), "check": "ready"}
		case "measurement":
			p = map[string]any{"condition": "resource", "unit": fmt.Sprintf("raid%d", i%40+1), "resource": fmt.Sprint(i / 40), "measure": "amount", "operator": "gte", "value": 0.0}
		case "anchor":
			r.Style.Anchor = &AnchorStyle{Kind: "frame", Target: fmt.Sprintf("Frame%d", i), Point: "center", SelfPoint: "center"}
		}
		r.Roots = []Node{{ID: fmt.Sprintf("when-%d", i), Type: "when", Props: map[string]any{"event": "state"}, Children: []Node{{ID: fmt.Sprintf("if-%d", i), Type: "if", Props: p, Children: []Node{{ID: fmt.Sprintf("show-%d", i), Type: "show", Props: map[string]any{"display": "text", "text": "Active", "visibility": "conditions"}}}}}}}
		rules[i] = r
	}
	return rules
}

func TestExpandedCaptureLimitsRemainAtomic(t *testing.T) {
	for _, source := range []string{"aura", "timer", "measurement", "anchor"} {
		t.Run(source, func(t *testing.T) {
			count := lookup.OverlaySourceLimit
			if source == "timer" {
				count = lookup.OverlayTimerLimit
			}
			if source == "anchor" {
				count = lookup.OverlayAnchorLimit
			}
			s, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Save(0, captureRules(count, source)); err != nil {
				t.Fatal(err)
			}
			if err = s.Save(1, captureRules(count+1, source)); err == nil {
				t.Fatal("overflow accepted")
			}
			s, err = Open(s.dir)
			if err != nil || s.doc.Revision != 1 || len(s.doc.Rules) != count {
				t.Fatal("rejected edit replaced saved rules", err)
			}
		})
	}
}

func TestExpandedAuraCaptureRetainsLoadedSourcesUntilReload(t *testing.T) {
	rules := captureRules(64, "aura")
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(2000000000, 0)
	capture := func(count, seq int) lookup.BridgeSample {
		x := sample(rules[:count], at, seq)
		p := x.Overlays
		p.Native = true
		p.Watches = nil
		if count > 16 {
			p.NativeVersion = 2
		}
		for _, w := range Watches(rules[:count]) {
			p.Watches = append(p.Watches, lookup.OverlayAura{Spell: w.Spell, Kind: w.Kind, Unit: w.Unit, Available: true, Active: ptr(true), Generation: 1})
		}
		return x
	}
	if err = s.Save(0, rules[:16]); err != nil {
		t.Fatal(err)
	}
	s.Accept(capture(16, 1), at)
	if n := len(s.Merge(engine.FrameDisplay{}, at).Overlays); n != 16 {
		t.Fatal(n, s.runtime.problem)
	}
	if err = s.Save(1, rules); err != nil {
		t.Fatal(err)
	}
	at = at.Add(100 * time.Millisecond)
	s.Accept(capture(16, 2), at)
	if n := len(s.Merge(engine.FrameDisplay{}, at).Overlays); n != 16 || !strings.Contains(s.runtime.problem, "Reload WoW") {
		t.Fatal(n, s.runtime.problem)
	}
	at = at.Add(100 * time.Millisecond)
	s.Accept(capture(64, 3), at)
	if n := len(s.Merge(engine.FrameDisplay{}, at).Overlays); n != 64 || s.runtime.problem != "" {
		t.Fatal(n, s.runtime.problem)
	}
}
