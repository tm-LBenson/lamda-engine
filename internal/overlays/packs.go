package overlays

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

// ID and Member are portable author identities. Instance identifies one local
// copy; importing a pack twice must never link their settings or saved history.
type Pack struct {
	ID       string            `json:"id"`
	Instance string            `json:"instance"`
	Member   string            `json:"member"`
	Members  []string          `json:"members"`
	Name     string            `json:"name"`
	Version  int               `json:"version"`
	Nodes    map[string]string `json:"nodes"`
}

func (p *Pack) UnmarshalJSON(b []byte) error {
	type plain Pack
	var value plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	*p = Pack(value)
	return nil
}
func (p Pack) signature() string {
	p.Member = ""
	p.Nodes = nil
	p.Members = append([]string(nil), p.Members...)
	sort.Strings(p.Members)
	b, _ := json.Marshal(p)
	return string(b)
}
func validPacks(rules []Rule) bool {
	groups := map[string][]Rule{}
	for _, r := range rules {
		if r.Pack == nil {
			if r.Logic != nil {
				for _, o := range r.Logic.Options {
					if o.Shared {
						return false
					}
				}
			}
			continue
		}
		p := r.Pack
		if !validID.MatchString(p.ID) || !validID.MatchString(p.Instance) || !validID.MatchString(p.Member) || !validLogicLabel(p.Name) || strings.IndexFunc(p.Name, unicode.IsControl) >= 0 || p.Version < 1 || p.Version > 1000000 || len(p.Members) == 0 || len(p.Members) > MaxRules {
			return false
		}
		seen := map[string]bool{}
		for _, id := range p.Members {
			if !validID.MatchString(id) || seen[id] {
				return false
			}
			seen[id] = true
		}
		if !seen[p.Member] {
			return false
		}
		if !validPackNodes(r) {
			return false
		}
		groups[p.Instance] = append(groups[p.Instance], r)
	}
	for _, rs := range groups {
		p := rs[0].Pack
		if len(rs) != len(p.Members) {
			return false
		}
		members := map[string]bool{}
		shared := map[string]string{}
		for _, r := range rs {
			if r.Pack.signature() != p.signature() || members[r.Pack.Member] {
				return false
			}
			members[r.Pack.Member] = true
			if r.Logic != nil {
				for _, o := range r.Logic.Options {
					if o.Shared {
						b, _ := json.Marshal(o)
						if old, ok := shared[o.Key]; ok && old != string(b) {
							return false
						}
						shared[o.Key] = string(b)
					}
				}
			}
		}
		for _, r := range rs {
			for key, definition := range shared {
				o := r.Logic.option(key)
				if o == nil || !o.Shared {
					return false
				}
				b, _ := json.Marshal(o)
				if string(b) != definition {
					return false
				}
			}
		}
	}
	return true
}

func validPackNodes(r Rule) bool {
	if r.Pack == nil || r.Pack.Nodes == nil || len(r.Pack.Nodes) > 80 {
		return false
	}
	wanted := map[string]bool{}
	var walk func([]Node) bool
	walk = func(ns []Node) bool {
		for _, n := range ns {
			if wanted[n.ID] || len(wanted) >= 80 {
				return false
			}
			wanted[n.ID] = true
			if !walk(n.Children) {
				return false
			}
		}
		return true
	}
	if !walk(r.Roots) || len(wanted) != len(r.Pack.Nodes) {
		return false
	}
	seen := map[string]bool{}
	for source, local := range r.Pack.Nodes {
		if !validID.MatchString(source) || !wanted[local] || seen[local] {
			return false
		}
		seen[local] = true
	}
	return true
}
