package overlays

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestPreparedRulesRefreshDependenciesAfterSave(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := valueRule()
	r.Roots[0].Props["event"] = "state"
	rules := []Rule{r}
	at := time.Unix(2000000000, 0)
	if err = s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	s.Accept(resourceSample(rules, at, 1, 15), at)
	if ds := s.Merge(engine.FrameDisplay{}, at).Overlays; len(ds) != 1 || ds[0].Text != "Mana 15" {
		t.Fatal(ds)
	}
	// A newly referenced buff must request reload, then use the new capture.
	r.Logic.Values[0].Expr = ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "buff", "dataID": "123", "dataUnit": "player"}, Field: "stacks"}
	if err = s.Save(1, rules); err != nil {
		t.Fatal(err)
	}
	at = at.Add(100 * time.Millisecond)
	old := resourceSample(nil, at, 2, 15)
	s.Accept(old, at)
	if !strings.Contains(s.runtime.problem, "Reload WoW") {
		t.Fatal("new dependency missing", s.runtime.problem)
	}
	at = at.Add(100 * time.Millisecond)
	fresh := resourceSample(rules, at, 3, 15)
	fresh.Overlays.Watches = []lookup.OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true, Active: ptr(true), Stacks: ptr(3.0)}}
	s.Accept(fresh, at)
	if ds := s.Merge(engine.FrameDisplay{}, at).Overlays; len(ds) != 1 || ds[0].Text != "Mana 3" {
		t.Fatal(ds, s.runtime.problem)
	}
	// Removing that source also removes the reload requirement.
	r.Logic.Values[0].Expr = val(7)
	if err = s.Save(2, rules); err != nil {
		t.Fatal(err)
	}
	at = at.Add(100 * time.Millisecond)
	s.Accept(resourceSample(rules, at, 4, 15), at)
	if ds := s.Merge(engine.FrameDisplay{}, at).Overlays; len(ds) != 1 || ds[0].Text != "Mana 7" || s.runtime.problem != "" {
		t.Fatal(ds, s.runtime.problem)
	}
	if s.doc.Rules[0].plan != nil || s.doc.Rules[0].execution {
		t.Fatal("runtime preparation changed editable rules")
	}
	b, _ := json.Marshal(s.doc.Rules)
	var reread []Rule
	if json.Unmarshal(b, &reread) != nil || Validate(reread) != nil {
		t.Fatal("prepared rules cannot round trip")
	}
}
