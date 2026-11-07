package overlays

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

func libraryRules(count, loaded int) []Rule {
	rs := make([]Rule, count)
	for i := range rs {
		r := valueRule()
		r.ID = fmt.Sprintf("library-%d", i)
		var rename func([]Node)
		rename = func(ns []Node) {
			for j := range ns {
				ns[j].ID = fmt.Sprintf("r%d-%s", i, ns[j].ID)
				rename(ns[j].Children)
			}
		}
		rename(r.Roots)
		if i >= loaded {
			r.Load.Character = "SomeoneElse"
		}
		rs[i] = r
	}
	return rs
}

func TestExpandedLibrarySaveRestartAndAtomicLimit(t *testing.T) {
	rules := libraryRules(MaxRules, 40)
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(0, rules); err != nil {
		t.Fatal(err)
	}
	s, err = Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.doc.Rules) != MaxRules {
		t.Fatal("library truncated")
	}
	at := time.Unix(2000000000, 0)
	s.Accept(resourceSample(rules, at, 1, 15), at)
	if got := len(s.Merge(engine.FrameDisplay{}, at).Overlays); got != 40 {
		t.Fatal("character filtering failed", got)
	}
	if err = s.Save(1, libraryRules(MaxRules+1, 40)); err == nil {
		t.Fatal("library limit ignored")
	}
	if s.doc.Revision != 1 || len(s.doc.Rules) != MaxRules {
		t.Fatal("rejected save changed library")
	}
	// Complete large packs use the same limit and remain restorable after restart.
	memberIDs := make([]string, len(rules))
	for i := range rules {
		memberIDs[i] = rules[i].ID
	}
	for i := range rules {
		r := &rules[i]
		r.Pack = &Pack{ID: "large-author", Instance: "large-copy", Member: r.ID, Members: memberIDs, Name: "Large pack", Version: 1, Nodes: map[string]string{}}
		var nodes func([]Node)
		nodes = func(ns []Node) {
			for _, n := range ns {
				r.Pack.Nodes[n.ID] = n.ID
				nodes(n.Children)
			}
		}
		nodes(r.Roots)
	}
	if err = s.Save(1, rules); err != nil {
		t.Fatal(err)
	}
	if err = s.SavePackUpdates(2, packVersion(rules, 2), []string{"large-copy"}); err != nil {
		t.Fatal(err)
	}
	s, err = Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	h := s.PackHistory()
	if h.Error != "" || len(h.Entries) != 1 || h.Entries[0].Members != MaxRules {
		t.Fatal(h)
	}
}

func BenchmarkPreparedLibrary(b *testing.B) {
	for _, tc := range []struct{ total, loaded int }{{40, 40}, {200, 40}, {200, 200}} {
		b.Run(fmt.Sprintf("%d-total-%d-loaded", tc.total, tc.loaded), func(b *testing.B) {
			rules := libraryRules(tc.total, tc.loaded)
			if err := Validate(rules); err != nil {
				b.Fatal(err)
			}
			s, err := Open(b.TempDir())
			if err != nil {
				b.Fatal(err)
			}
			s.doc.Rules = rules
			at := time.Unix(2000000000, 0)
			p := resourceSample(rules, at, 1, 15)
			s.Accept(p, at)
			s.Merge(engine.FrameDisplay{}, at)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				now := at.Add(time.Duration(i+1) * 100 * time.Millisecond)
				input := *p.Overlays
				input.Seq = i + 2
				input.At = now.Unix()
				p.Overlays = &input
				s.Accept(p, now)
				if n := len(s.Merge(engine.FrameDisplay{}, now).Overlays); n != min(64, tc.loaded) {
					b.Fatal(n)
				}
			}
		})
	}
}

func BenchmarkLibraryState(b *testing.B) {
	s, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	s.doc.Rules = libraryRules(MaxRules, 40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(s.State()); err != nil {
			b.Fatal(err)
		}
	}
}
