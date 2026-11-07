package overlays

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packRules() []Rule {
	a, b := memoryRule(), memoryRule()
	a.ID = "pack-a"
	b.ID = "pack-b"
	b.Roots[0].ID = "when-b"
	b.Roots[0].Children[0].ID = "show-b"
	for _, r := range []*Rule{&a, &b} {
		r.Pack = &Pack{ID: "author-pack", Instance: "local-pack", Member: r.ID, Members: []string{"pack-a", "pack-b"}, Name: "My pack", Version: 1}
		r.Pack.Nodes = map[string]string{r.Roots[0].ID: r.Roots[0].ID, r.Roots[0].Children[0].ID: r.Roots[0].Children[0].ID}
		r.Logic.Options = []LogicOption{{Key: "limit", Name: "Threshold", Type: "number", Default: 20.0, Shared: true}}
	}
	return []Rule{a, b}
}
func TestPackValidationAndPortableIdentities(t *testing.T) {
	rs := packRules()
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	if err := Validate(rs[:1]); err == nil {
		t.Fatal("incomplete pack accepted")
	}
	rs[1].Pack.Members = []string{"pack-b", "pack-a"}
	if err := Validate(rs); err != nil {
		t.Fatal("member order matters", err)
	}
	for _, change := range []func([]Rule){
		func(r []Rule) { r[1].Pack.Member = "pack-a" }, func(r []Rule) { r[1].Pack.ID = "different" }, func(r []Rule) { r[1].Pack.Name = "different" },
		func(r []Rule) { r[1].Pack.Version = 2 }, func(r []Rule) { r[0].Pack.Version = 0 }, func(r []Rule) { r[0].Pack.Members = []string{"pack-a", "pack-a"} },
		func(r []Rule) { r[1].Pack.Instance = "other" }, func(r []Rule) { r[0].Pack.Member = "outside" }, func(r []Rule) { r[0].Pack.Name = strings.Repeat("x", 61) },
		func(r []Rule) { r[1].Logic.Options[0].Value = 30.0 }, func(r []Rule) { r[1].Logic.Options[0].Shared = false }, func(r []Rule) { r[1].Logic.Options = nil },
		func(r []Rule) { r[1].Pack = nil }, func(r []Rule) { r[1].Logic = nil },
		func(r []Rule) { r[1].Pack.Nodes = nil }, func(r []Rule) { r[1].Pack.Nodes["outside"] = "missing" },
	} {
		rs = packRules()
		change(rs)
		if err := Validate(rs); err == nil {
			t.Fatal("invalid pack accepted", rs)
		}
	}
	var p Pack
	if json.Unmarshal([]byte(`{"id":"x","unknown":true}`), &p) == nil {
		t.Fatal("unknown metadata accepted")
	}
}
func TestPackSharedSettingsUseExistingEvaluator(t *testing.T) {
	rs := packRules()
	for i := range rs {
		rs[i].Logic.Values[0].Remember = nil
		rs[i].Logic.Values[0].Expr = ValueExpr{Kind: "option", Key: "limit"}
		rs[i].Logic.Options[0].Value = 42.0
	}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if got := newValueEvaluation(r.Logic, nil, "player", nil).named("count"); got.number != 42 {
			t.Fatal("shared value not resolved", got)
		}
	}
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	s, err = Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.doc.Rules) != 2 || s.doc.Rules[1].Pack.ID != "author-pack" || !s.doc.Rules[1].Logic.Options[0].Shared {
		t.Fatal("pack was lost on reload")
	}
	// Separate local copies may use different values without coupling.
	copy := packRules()
	for i := range copy {
		copy[i].ID += "copy"
		copy[i].Roots[0].ID += "copy"
		copy[i].Roots[0].Children[0].ID += "copy"
		for source, local := range copy[i].Pack.Nodes {
			copy[i].Pack.Nodes[source] = local + "copy"
		}
		copy[i].Pack.Instance = "copy"
		copy[i].Logic.Options[0].Value = 7.0
	}
	if err = Validate(append(rs, copy...)); err != nil {
		t.Fatal(err)
	}
}

func TestPackOversizedDocumentCannotBreakRestart(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(0, packRules()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, "rules.json")
	before, _ := os.ReadFile(path)
	var many []Rule
	for i := 0; i < 80; i++ {
		r := memoryRule()
		r.ID = fmt.Sprint("large-", i)
		r.Roots[0].ID = fmt.Sprint("when-", i)
		r.Roots[0].Children[0].ID = fmt.Sprint("show-", i)
		r.Roots[0].Children[0].Props["text"] = "Ready"
		r.Logic = &Logic{}
		for j := 0; j < 16; j++ {
			e := ValueExpr{Kind: "op", Op: "concat"}
			for k := 0; k < 8; k++ {
				e.Args = append(e.Args, val(strings.Repeat("x", 512)))
			}
			r.Logic.Values = append(r.Logic.Values, NamedValue{Key: fmt.Sprint("value", j), Name: "Text", Expr: e})
		}
		many = append(many, r)
	}
	if err = Validate(many); err != nil {
		t.Fatal("size fixture invalid", err)
	}
	if err = s.Save(1, many); err == nil || !strings.Contains(err.Error(), "4 MB") {
		t.Fatal("oversized document accepted", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) || s.doc.Revision != 1 {
		t.Fatal("rejected save changed current data")
	}
	if _, err = Open(s.dir); err != nil {
		t.Fatal("store no longer opens", err)
	}
}
