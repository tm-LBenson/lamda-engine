package overlays

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"strings"
	"testing"
	"time"
)

func scopedRule(scope string) Rule {
	r := cloneRule()
	r.Roots[0].Children[0].Props["scope"] = scope
	return r
}
func scopedSample(rs []Rule, at time.Time, seq int, units []lookup.OverlayUnit) lookup.BridgeSample {
	s := sample(rs, at, seq)
	p := s.Overlays
	p.Native = true
	p.NativeVersion = 2
	p.UnitsVersion = 2
	p.UnitTokens = UnitTokens(rs)
	p.UnitHash = lookup.OverlayUnitHash(p.UnitTokens)
	p.Units = units
	p.Group = "raid"
	p.Watches = nil
	for _, w := range Watches(rs) {
		available := false
		for _, u := range units {
			if u.Unit == w.Unit {
				available = true
			}
		}
		p.Watches = append(p.Watches, lookup.OverlayAura{Spell: w.Spell, Kind: w.Kind, Unit: w.Unit, Generation: 1, Available: available, Active: ptr(true), Stacks: ptr(2.0)})
	}
	return s
}
func TestCloneScopesFullRostersSourceExpansionAndAttachments(t *testing.T) {
	for scope, count := range map[string]int{"party": 5, "group": 45, "raid": 40, "boss": 8, "arena": 5, "nameplates": 40} {
		t.Run(scope, func(t *testing.T) {
			r := scopedRule(scope)
			r.Style.Anchor = &AnchorStyle{Kind: "unit", Target: "current", Point: "center", SelfPoint: "center"}
			rs := []Rule{r}
			if err := Validate(rs); err != nil {
				t.Fatal(err)
			}
			if len(UnitTokens(rs)) != count || len(Watches(rs)) != count || len(Anchors(rs)) != count {
				t.Fatal("wrong scope expansion", len(UnitTokens(rs)), len(Watches(rs)), len(Anchors(rs)))
			}
			r.Style.Anchor = nil
			rs = []Rule{r}
			actual := scope
			if scope == "group" {
				actual = "raid"
			}
			us := []lookup.OverlayUnit{}
			for i, u := range lookup.OverlayScopeTokens(actual) {
				us = append(us, lookup.OverlayUnit{Unit: u, ID: fmt.Sprintf("Player-1-%d", i), Generation: 1, Name: fmt.Sprint("Member", i)})
			}
			s, _ := Open(t.TempDir())
			s.doc.Rules = rs
			at := time.Unix(2000000000, 0)
			s.Accept(scopedSample(rs, at, 1, us), at)
			ds := s.Merge(engine.FrameDisplay{}, at).Overlays
			if len(ds) != len(us) {
				t.Fatal("full roster did not render", len(ds), len(us), s.runtime.problem)
			}
			for _, d := range ds {
				if !strings.Contains(d.Text, "Member") {
					t.Fatal("unit name missing", d.Text)
				}
			}
		})
	}
	// Separate branches retain their own source scope; rule-wide styling spans both.
	r := scopedRule("raid")
	other := r.Roots[0].Children[0]
	other.ID = "boss-each"
	other.Props = map[string]any{"scope": "boss"}
	other.Children = []Node{{ID: "boss-if", Type: "if", Props: map[string]any{"condition": "buff", "unit": "current", "aura": "456", "check": "active"}}}
	r.Roots[0].Children = append(r.Roots[0].Children, other)
	ws := Watches([]Rule{r})
	if len(ws) != 48 {
		t.Fatal("branch scopes leaked", len(ws))
	}
	for _, w := range ws {
		if w.Spell == 123 && !strings.HasPrefix(w.Unit, "raid") || w.Spell == 456 && !strings.HasPrefix(w.Unit, "boss") {
			t.Fatal(w)
		}
	}
	r.Roots[0].Children = append(r.Roots[0].Children, Node{ID: "outside", Type: "show", Props: map[string]any{"display": "text", "text": "Player", "visibility": "conditions"}})
	r.Style.Anchor = &AnchorStyle{Kind: "unit", Target: "current", Point: "center", SelfPoint: "center"}
	if len(Anchors([]Rule{r})) != 49 {
		t.Fatal("ordinary output lost player anchor")
	}
}

func TestScopedIdentitiesKeepAliasesAndExpireOnScopeExit(t *testing.T) {
	p := &lookup.OverlayInput{UnitsVersion: 2, Group: "raid", Units: []lookup.OverlayUnit{{Unit: "player", ID: "Player-1-A"}, {Unit: "party1", ID: "Player-1-B"}, {Unit: "raid1", ID: "Player-1-A"}, {Unit: "raid2", ID: "Player-1-B"}, {Unit: "boss1", ID: "Creature-1-C"}, {Unit: "nameplate1", ID: "Creature-1-C"}, {Unit: "nameplate2", ID: "Creature-1-C"}}}
	for actor, want := range map[string]string{"Player-1-B": "party1", "raid@Player-1-A": "raid1", "group@Player-1-B": "raid2", "boss@Creature-1-C": "boss1", "nameplates@Creature-1-C": "nameplate1"} {
		if got := boundUnit(p, actor); got != want {
			t.Fatal(actor, got, want)
		}
	}
	if len(cloneMembers(p, "nameplates")) != 1 {
		t.Fatal("alias rendered twice within one scope")
	}
	p.Units = p.Units[2:]
	if boundUnit(p, "Player-1-B") != "" {
		t.Fatal("departed party member retained via raid alias")
	}
	p.Group = "party"
	if boundUnit(p, "group@Player-1-B") != "" {
		t.Fatal("group retained stale raid member")
	}
	if boundUnit(p, "unknown@Creature-1-C") != "" {
		t.Fatal("unknown actor scope")
	}
}

func TestScopedCloneSlotReuseCancelsDelayedOutput(t *testing.T) {
	r := scopedRule("nameplates")
	c := &r.Roots[0].Children[0].Children[0]
	c.Children = append([]Node{{ID: "pause", Type: "wait", Props: map[string]any{"seconds": 2.0}}}, c.Children...)
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	s.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	old := lookup.OverlayUnit{Unit: "nameplate1", ID: "Slot-nameplate1-1", Generation: 1, Name: "Old"}
	s.Accept(scopedSample(rs, at, 1, []lookup.OverlayUnit{old}), at)
	if len(s.runtime.queue) != 1 || s.runtime.queue[0].Actor != "nameplates@Slot-nameplate1-1" {
		t.Fatal("delayed identity lost", s.runtime.queue)
	}
	fresh := lookup.OverlayUnit{Unit: "nameplate1", ID: "Slot-nameplate1-2", Generation: 2, Name: "New"}
	at = at.Add(time.Second)
	s.Accept(scopedSample(rs, at, 2, []lookup.OverlayUnit{fresh}), at)
	for _, q := range s.runtime.queue {
		if q.Actor != "nameplates@Slot-nameplate1-2" {
			t.Fatal("recycled slot retained old action")
		}
	}
	for seq := 3; seq <= 13; seq++ {
		at = at.Add(200 * time.Millisecond)
		s.Accept(scopedSample(rs, at, seq, []lookup.OverlayUnit{fresh}), at)
		if seq < 12 && len(s.Merge(engine.FrameDisplay{}, at).Overlays) != 0 {
			t.Fatal("old action fired on replacement")
		}
	}
	ds := s.Merge(engine.FrameDisplay{}, at).Overlays
	if len(ds) != 1 || ds[0].Text != "New: 2" {
		t.Fatal("replacement failed", ds, s.runtime.problem)
	}
}

func TestNewCloneScopeWaitsForReloadWithoutStoppingParty(t *testing.T) {
	r := cloneRule()
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(2000000000, 0)
	s.Accept(cloneSample(rs, at, 1), at)
	newRule := scopedRule("boss")
	newRule.ID = "boss-rule"
	var rename func([]Node)
	rename = func(ns []Node) {
		for i := range ns {
			ns[i].ID = "boss-" + ns[i].ID
			rename(ns[i].Children)
		}
	}
	rename(newRule.Roots)
	rs = append(rs, newRule)
	if err := s.Save(1, rs); err != nil {
		t.Fatal(err)
	}
	at = at.Add(100 * time.Millisecond)
	s.Accept(cloneSample(rs[:1], at, 2), at)
	if ds := s.Merge(engine.FrameDisplay{}, at).Overlays; len(ds) != 2 || !strings.Contains(s.runtime.problem, "Reload WoW") {
		t.Fatal(ds, s.runtime.problem)
	}
}

func BenchmarkExpandedRaidClones(b *testing.B) {
	r := scopedRule("raid")
	rules := []Rule{r}
	units := []lookup.OverlayUnit{}
	for i, u := range lookup.OverlayScopeTokens("raid") {
		units = append(units, lookup.OverlayUnit{Unit: u, ID: fmt.Sprintf("Player-1-%d", i), Name: fmt.Sprint("Member", i), Generation: 1})
	}
	s, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	s.doc.Rules = rules
	at := time.Unix(2000000000, 0)
	input := scopedSample(rules, at, 1, units)
	s.Accept(input, at)
	s.Merge(engine.FrameDisplay{}, at)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now := at.Add(time.Duration(i+1) * 100 * time.Millisecond)
		p := *input.Overlays
		p.Seq = i + 2
		p.At = now.Unix()
		input.Overlays = &p
		s.Accept(input, now)
		if ds := s.Merge(engine.FrameDisplay{}, now).Overlays; len(ds) != 40 {
			b.Fatal(len(ds), s.runtime.problem)
		}
	}
}

func TestScopedValueInputsIncludePlayerAndCalledActions(t *testing.T) {
	r := scopedRule("raid")
	r.Logic = &Logic{Values: []NamedValue{{Key: "stacks", Name: "Stacks", Expr: ValueExpr{Kind: "data", Source: map[string]any{"dataSource": "buff", "dataID": "123", "dataUnit": "current"}, Field: "stacks"}}}}
	c := &r.Roots[0].Children[0].Children[0]
	c.Props = map[string]any{"condition": "value", "key": "stacks", "operator": "gt", "value": 0.0}
	c.Children[0].Props["text"] = "{value:stacks}"
	if ws := Watches([]Rule{r}); len(ws) != 41 {
		t.Fatal("rule-level value lost player or raid source", len(ws))
	}
	def := Node{ID: "shared-action", Type: "action", Props: map[string]any{"name": "Alert"}, Children: r.Roots[0].Children[0].Children}
	r.Roots[0].Children[0].Children = []Node{{ID: "call-alert", Type: "call", Props: map[string]any{"action": "shared-action"}}}
	r.Roots = append(r.Roots, def)
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	if len(Watches([]Rule{r})) != 41 || len(UnitTokens([]Rule{r})) != 40 {
		t.Fatal("called action lost caller scope")
	}
}
