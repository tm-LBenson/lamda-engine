package lookup

import (
	"math"
	"unicode/utf8"
)

// Encounter lifecycle is public game context, independent of combat log writes.
// A reload during a fight can know Active while ID and elapsed time stay unknown.
type OverlayEncounter struct {
	Active     *bool                   `json:"active,omitempty"`
	ID         int                     `json:"id"`
	Name       string                  `json:"name,omitempty"`
	Difficulty int                     `json:"difficulty"`
	GroupSize  int                     `json:"groupSize"`
	Elapsed    *float64                `json:"elapsed,omitempty"`
	Seq        int                     `json:"seq"`
	Events     []OverlayEncounterEvent `json:"events,omitempty"`
}
type OverlayEncounterEvent struct {
	Seq        int      `json:"seq"`
	Kind       string   `json:"kind"`
	ID         int      `json:"id"`
	Name       string   `json:"name,omitempty"`
	Difficulty int      `json:"difficulty"`
	GroupSize  int      `json:"groupSize"`
	Elapsed    *float64 `json:"elapsed,omitempty"`
	Success    *bool    `json:"success,omitempty"`
	Age        int      `json:"age"`
}

func ValidEncounterEvent(kind string) bool {
	return kind == "encounterstart" || kind == "encounterend" || kind == "bosskill"
}
func sanitizeOverlayEncounter(p *OverlayInput) {
	e := p.Encounter
	if p.EncountersVersion != 1 {
		p.Encounter = nil
		return
	}
	if e == nil {
		return
	}
	valid := func(id int, name string, difficulty, size int) bool {
		return id >= 0 && id < 10000000 && len(name) <= 160 && utf8.ValidString(name) && difficulty >= 0 && difficulty <= 1000 && size >= 0 && size <= 100
	}
	if !valid(e.ID, e.Name, e.Difficulty, e.GroupSize) || e.Seq < 0 || e.Seq > 2147483647 || len(e.Events) > 4 || e.Elapsed != nil && (math.IsNaN(*e.Elapsed) || math.IsInf(*e.Elapsed, 0) || *e.Elapsed < 0 || *e.Elapsed > 86400) {
		p.Encounter = nil
		return
	}
	seen := map[int]bool{}
	for _, v := range e.Events {
		if !valid(v.ID, v.Name, v.Difficulty, v.GroupSize) || !ValidEncounterEvent(v.Kind) || v.Seq < 1 || v.Seq > e.Seq || seen[v.Seq] || v.Age < 0 || v.Age > 3000 || v.Kind != "encounterend" && v.Success != nil || v.Elapsed != nil && (math.IsNaN(*v.Elapsed) || math.IsInf(*v.Elapsed, 0) || *v.Elapsed < 0 || *v.Elapsed > 86400) {
			p.Encounter = nil
			return
		}
		seen[v.Seq] = true
	}
}
