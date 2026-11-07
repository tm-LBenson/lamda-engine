package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestTalentComparisonsUnknownNegationAndPendingWatch(t *testing.T) {
	q := map[string]any{"condition": "talent", "source": "spell", "id": "123", "check": "rank", "operator": "gte", "value": float64(2)}
	w := []TalentWatch{{Kind: "spell", ID: 123}}
	p := &lookup.OverlayInput{TalentsVersion: 1, TalentHash: TalentHash(w), Talents: []lookup.OverlayTalent{{Kind: "spell", ID: 123, Rank: ptr(2)}}}
	if predicate(q, p, "party1") != yes {
		t.Fatal("personal talent followed clone instead of player")
	}
	q["operator"] = "lt"
	if predicate(q, p, "") != no {
		t.Fatal("rank compare")
	}
	q["check"] = "missing"
	p.Talents[0].Rank = nil
	not := map[string]any{"condition": "group", "mode": "not", "tests": []any{q}}
	if predicate(q, p, "") != unknown || predicate(not, p, "") != unknown {
		t.Fatal("unknown became unselected")
	}
	p.Talents[0].Rank = ptr(0)
	if predicate(q, p, "") != yes {
		t.Fatal("known unselected")
	}
	q["check"] = "selected"
	p.Talents[0].Rank = ptr(2)
	r := rule()
	r.Roots[0].Children = []Node{{ID: "known-talent", Type: "if", Props: q}, {ID: "new-talent", Type: "if", Props: map[string]any{"condition": "talent", "source": "entry", "id": "456", "check": "selected"}}}
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{}
	rt.validateTalents(p, rs)
	if rt.problem != "Reload WoW to watch new talents" || predicate(q, p, "") != yes {
		t.Fatal("pending watch broke loaded source", rt.problem)
	}
	p.TalentHash = "wrong"
	rt.validateTalents(p, rs)
	if predicate(q, p, "") != unknown {
		t.Fatal("mismatched watch accepted")
	}
}
func TestTalentDemandValidationAndPublishing(t *testing.T) {
	r := rule()
	r.Roots[0].Children = []Node{{ID: "talent", Type: "if", Props: map[string]any{"condition": "talent", "source": "pvp", "id": "123", "check": "selected"}}, {ID: "known", Type: "if", Props: map[string]any{"condition": "learned", "id": "456", "check": "missing"}}}
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	if len(TalentWatches(rs)) != 2 {
		t.Fatal("missing demand")
	}
	s, _ := Open(t.TempDir())
	s.doc.Rules = rs
	path := filepath.Join(t.TempDir(), "OverlaysData.lua")
	os.WriteFile(path, []byte(Marker+"\nLamdaOverlaysData={}\n"), 0600)
	s.SetGameFile(path)
	b, err := os.ReadFile(path)
	if err != nil || s.publishError != "" || !strings.Contains(string(b), `["talentHash"]=`) || !strings.Contains(string(b), `["kind"]="pvp"`) {
		t.Fatal("not published", err, s.publishError)
	}
	rs[0].Enabled = false
	if len(TalentWatches(rs)) != 0 {
		t.Fatal("disabled rule collected")
	}
	for _, q := range []map[string]any{{"condition": "talent", "source": "bad", "id": "123", "check": "selected"}, {"condition": "talent", "source": "spell", "id": "0", "check": "missing"}, {"condition": "talent", "source": "entry", "id": "123", "check": "rank", "operator": "gte", "value": 1.5}, {"condition": "learned", "id": "123", "check": "rank", "operator": "gte", "value": float64(1)}} {
		if validTalent(q) {
			t.Fatal("invalid accepted", q)
		}
	}
}

func TestTalentRankTextSource(t *testing.T) {
	n := Node{ID: "show", Type: "show", Props: map[string]any{"display": "text", "text": "Rank {rank}: {name}", "visibility": "conditions"}}
	source := displaySource{Kind: "talent", TalentKind: "entry", ID: 123}
	p := &lookup.OverlayInput{TalentsVersion: 1, Talents: []lookup.OverlayTalent{{Kind: "entry", ID: 123, Rank: ptr(2), Spell: 9999999}}}
	v := engine.OverlayDisplay{Text: "Rank {rank}: {name}"}
	resolveDisplay(&v, n, source, p)
	if v.Text != "Rank 2: Spell 9999999" {
		t.Fatal(v.Text)
	}
	p.Talents[0].Rank = nil
	v.Text = "Rank {rank}: {name}"
	resolveDisplay(&v, n, source, p)
	if v.Text != "Rank —: Spell 9999999" {
		t.Fatal(v.Text)
	}
}
