// Package spellbook provides an offline, pinned Retail spell lookup. It never
// reads game memory or treats catalog entries as learned/active player spells.
package spellbook

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

//go:embed retail.json.gz NOTICE.md LICENSE.txt
var assets embed.FS

type Class struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var Classes = []Class{{"WARRIOR", "Warrior"}, {"PALADIN", "Paladin"}, {"HUNTER", "Hunter"}, {"ROGUE", "Rogue"}, {"PRIEST", "Priest"}, {"DEATHKNIGHT", "Death Knight"}, {"SHAMAN", "Shaman"}, {"MAGE", "Mage"}, {"WARLOCK", "Warlock"}, {"MONK", "Monk"}, {"DRUID", "Druid"}, {"DEMONHUNTER", "Demon Hunter"}, {"EVOKER", "Evoker"}}

type Spell struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Aura        bool     `json:"aura"`
	Description string   `json:"description,omitempty"`
	Classes     []string `json:"classes"`
	Duration    float64  `json:"duration"`
	Stacks      int      `json:"stacks"`
}
type entry struct {
	Spell
	Mask              int  `json:"mask"`
	Tooltip           bool `json:"tooltip"`
	name, description string
}
type Result struct {
	Spells  []Spell `json:"spells"`
	Total   int     `json:"total"`
	Offset  int     `json:"offset"`
	Build   string  `json:"build"`
	Classes []Class `json:"classes"`
}
type Options struct {
	Query, Class, Prefer, Kind string
	Offset                     int
}

var once sync.Once
var byID = map[int]Spell{}
var loadError error
var catalog struct {
	Build  string  `json:"build"`
	Spells []entry `json:"spells"`
}

func normalize(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) }), " ")
}
func classMask(id string) int {
	for i, c := range Classes {
		if id == c.ID {
			return 1 << i
		}
	}
	return 0
}
func load() {
	b, err := assets.ReadFile("retail.json.gz")
	if err != nil {
		loadError = err
		return
	}
	z, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		loadError = err
		return
	}
	defer z.Close()
	if err = json.NewDecoder(z).Decode(&catalog); err != nil {
		loadError = err
		return
	}
	for i := range catalog.Spells {
		e := &catalog.Spells[i]
		e.name = normalize(e.Name)
		e.description = normalize(e.Description)
		e.Classes = []string{}
		for j, c := range Classes {
			if e.Mask&(1<<j) != 0 {
				e.Classes = append(e.Classes, c.Name)
			}
		}
		byID[e.ID] = e.Spell
	}
}

func Name(id int) string {
	once.Do(load)
	if s, ok := byID[id]; ok {
		return s.Name
	}
	if id > 0 {
		return fmt.Sprintf("Spell %d", id)
	}
	return ""
}

func Search(o Options) (Result, error) {
	if len(o.Query) > 120 || o.Offset < 0 || o.Offset > 50000 || (o.Kind != "" && o.Kind != "all" && o.Kind != "auras") || (o.Class != "" && classMask(o.Class) == 0) || (o.Prefer != "" && classMask(o.Prefer) == 0) {
		return Result{}, fmt.Errorf("Invalid spell search")
	}
	once.Do(load)
	if loadError != nil {
		return Result{}, fmt.Errorf("Spell index unavailable: %w", loadError)
	}
	out := Result{Spells: []Spell{}, Offset: o.Offset, Build: catalog.Build, Classes: Classes}
	q := normalize(o.Query)
	if q == "" {
		return out, nil
	}
	id, numeric := strconv.Atoi(strings.TrimSpace(o.Query))
	exactID := numeric == nil && id > 0 && id < 10000000
	if !exactID && len([]rune(q)) < 2 {
		return out, nil
	}
	mask, preferred := classMask(o.Class), classMask(o.Prefer)
	tokens := strings.Fields(q)
	all := func(s string) bool {
		for _, t := range tokens {
			if !strings.Contains(s, t) {
				return false
			}
		}
		return true
	}
	type match struct{ index, score int }
	var matches []match
	for i, e := range catalog.Spells {
		score := 0
		if exactID {
			if e.ID != id {
				continue
			}
		} else {
			if mask != 0 && e.Mask&mask == 0 || o.Kind == "auras" && !e.Aura {
				continue
			}
			switch {
			case e.name == q:
				score = 0
			case strings.HasPrefix(e.name, q):
				score = 100
			case all(e.name):
				score = 200
			case all(e.name + " " + e.description):
				score = 400
			default:
				continue
			}
			if preferred != 0 && e.Mask&preferred == 0 {
				if e.Mask == 0 {
					score += 10
				} else {
					score += 20
				}
			}
			if !e.Tooltip {
				score += 2
			}
		}
		matches = append(matches, match{i, score})
	}
	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.score != b.score {
			return a.score < b.score
		}
		x, y := catalog.Spells[a.index], catalog.Spells[b.index]
		if x.name != y.name {
			return x.name < y.name
		}
		return x.ID < y.ID
	})
	out.Total = len(matches)
	for i := o.Offset; i < len(matches) && i < o.Offset+30; i++ {
		out.Spells = append(out.Spells, catalog.Spells[matches[i].index].Spell)
	}
	return out, nil
}

func Attribution() string {
	n, _ := assets.ReadFile("NOTICE.md")
	l, _ := assets.ReadFile("LICENSE.txt")
	return string(n) + "\n" + string(l)
}
