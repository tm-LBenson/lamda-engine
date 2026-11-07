package lookup

import (
	"math"
	"unicode/utf8"
)

// Optional boss-mod observations. Incomplete capture cannot prove a timer absent.
// Provider timer keys stay in the addon; ID identifies one lifetime on the wire.
type OverlayBosses struct {
	Seq       int                   `json:"seq"`
	Providers []OverlayBossProvider `json:"providers"`
	Timers    []OverlayBossTimer    `json:"timers"`
	Events    []OverlayBossEvent    `json:"events"`
	Overflow  bool                  `json:"overflow,omitempty"`
}
type OverlayBossProvider struct {
	Name        string   `json:"name"`
	Available   bool     `json:"available"`
	Complete    bool     `json:"complete"`
	Stage       *float64 `json:"stage,omitempty"`
	StageTotal  *float64 `json:"stageTotal,omitempty"`
	EncounterID int      `json:"encounterID,omitempty"`
}
type OverlayBossTimer struct {
	Provider  string   `json:"provider"`
	Source    string   `json:"source,omitempty"`
	ID        int      `json:"id"`
	Name      string   `json:"name,omitempty"`
	Spell     *int     `json:"spell,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	Count     *float64 `json:"count,omitempty"`
	Duration  *float64 `json:"duration,omitempty"`
	Remaining *float64 `json:"remaining,omitempty"`
	Paused    *bool    `json:"paused,omitempty"`
}
type OverlayBossEvent struct {
	Seq         int              `json:"seq"`
	Kind        string           `json:"kind"`
	Age         int              `json:"age"`
	Timer       OverlayBossTimer `json:"timer"`
	Stage       *float64         `json:"stage,omitempty"`
	StageTotal  *float64         `json:"stageTotal,omitempty"`
	EncounterID int              `json:"encounterID,omitempty"`
}

func ValidBossProvider(s string) bool { return s == "dbm" || s == "bigwigs" || s == "blizzard" }
func ValidBossEvent(s string) bool {
	switch s {
	case "bossstart", "bossupdate", "bosspause", "bossresume", "bossstop", "bossexpire", "bosswarning", "bossstage":
		return true
	}
	return false
}
func sanitizeOverlayBosses(p *OverlayInput) {
	b := p.Bosses
	if p.BossesVersion != 1 {
		p.Bosses = nil
		return
	}
	if b == nil {
		return
	}
	number := func(v *float64, limit float64) bool {
		return v == nil || !math.IsNaN(*v) && !math.IsInf(*v, 0) && *v >= 0 && *v <= limit
	}
	validTimer := func(t OverlayBossTimer, event bool) bool {
		return (t.Source == "" || t.Provider == "blizzard" && (t.Source == "encounter" || t.Source == "script")) && ValidBossProvider(t.Provider) && t.ID >= 0 && (event || t.ID > 0) && t.ID <= 2147483647 && len(t.Name) <= 160 && utf8.ValidString(t.Name) && (t.Spell == nil || *t.Spell > 0 && *t.Spell < 10000000) &&
			(t.Kind == "" || t.Kind == "timer" || t.Kind == "cast" || t.Kind == "pull" || t.Kind == "break") && number(t.Count, 1000000) && number(t.Duration, 86400) && number(t.Remaining, 86400)
	}
	valid := func() bool {
		if b.Seq < 0 || b.Seq > 2147483647 || len(b.Providers) > 3 || len(b.Timers) > 16 || len(b.Events) > 8 {
			return false
		}
		providers := map[string]OverlayBossProvider{}
		ids := map[int]bool{}
		seqs := map[int]bool{}
		for _, v := range b.Providers {
			if _, ok := providers[v.Name]; ok || !ValidBossProvider(v.Name) || !number(v.Stage, 10000) || !number(v.StageTotal, 1000000) || v.EncounterID < 0 || v.EncounterID >= 10000000 || !v.Available && (v.Complete || v.Stage != nil || v.StageTotal != nil) {
				return false
			}
			providers[v.Name] = v
		}
		for _, t := range b.Timers {
			if !validTimer(t, false) || ids[t.ID] || !providers[t.Provider].Available {
				return false
			}
			ids[t.ID] = true
		}
		for _, e := range b.Events {
			if !ValidBossEvent(e.Kind) || !validTimer(e.Timer, true) || !providers[e.Timer.Provider].Available || e.Seq < 1 || e.Seq > b.Seq || seqs[e.Seq] || e.Age < 0 || e.Age > 3000 || !number(e.Stage, 10000) || !number(e.StageTotal, 1000000) || e.EncounterID < 0 || e.EncounterID >= 10000000 {
				return false
			}
			if e.Kind != "bosswarning" && e.Kind != "bossstage" && e.Timer.ID == 0 {
				return false
			}
			seqs[e.Seq] = true
		}
		return true
	}
	if !valid() {
		p.Bosses = nil
	}
}
