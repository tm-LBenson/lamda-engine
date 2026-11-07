package engine

import (
	"fmt"
	"sort"
	"time"
)

// EstimatePanel is a presentation of existing evidence. Rendering, previews,
// filters and timer expiry never create casts or confirmed readiness.
type EstimatePanel struct {
	Enabled bool          `json:"enabled"`
	Demo    bool          `json:"demo"`
	Heading string        `json:"heading"`
	Status  string        `json:"status"`
	Empty   string        `json:"empty"`
	Hidden  int           `json:"hidden"`
	Rows    []EstimateRow `json:"rows"`
}

type EstimateRow struct {
	GUID   string  `json:"guid"`
	Spell  int     `json:"spell"`
	Player string  `json:"player"`
	Name   string  `json:"name"`
	Self   bool    `json:"self"`
	Status string  `json:"status"`
	Ends   float64 `json:"ends"`
	Detail string  `json:"detail"`
}

func PanelFor(s InferenceSnapshot, c Config, input string, lastInput time.Time, roster map[string]bool, now time.Time) EstimatePanel {
	p := EstimatePanel{Enabled: c.EstimatePanel, Heading: "Cooldown estimates", Rows: []EstimateRow{}}
	p.Empty = "Waiting for a supported defensive cast."
	p.Status = "Log-based estimates; talents and resets may change timers."
	switch {
	case !c.TrackingEnabled:
		p.Status = "Tracking is off."
		p.Empty = "Enable engine tracking in LamdaCD."
		return p
	case input == "read_error":
		p.Status = "Cannot read the combat log. Existing estimates may be stale."
	case input == "waiting_for_log":
		p.Status = "Waiting for a combat log."
		p.Empty = "Start recording in LamdaCD or use /combatlog."
	case !lastInput.IsZero():
		p.Status = fmt.Sprintf("Last log data %.0fs ago; delivery can be delayed.", max(0, now.Sub(lastInput).Seconds()))
	default:
		p.Status = "Waiting for new log data; recording may be idle."
	}
	for _, e := range s.Cooldowns {
		if (e.Self && !c.EstimateSelf) || (!e.Self && !c.EstimateParty) {
			continue
		}
		if !e.Self && roster != nil && !roster[e.GUID] {
			continue
		}
		if e.Status == "unknown" || (e.Status == "estimated_ready" && !c.EstimateReady) {
			continue
		}
		detail := "Reference timer"
		if e.MayHaveCharges {
			detail = "Recharge estimate; charges unknown"
		}
		if e.Kind == "reset" {
			detail = "Reset observed; availability estimated"
		}
		if e.DeliverySeconds > 2 {
			detail += fmt.Sprintf(" · log delay %.0fs", e.DeliverySeconds)
		}
		if e.BuildSource == "raider_io" {
			detail += " · public build"
		}
		p.Rows = append(p.Rows, EstimateRow{GUID: e.GUID, Spell: e.Spell, Player: e.Player, Name: e.Name, Self: e.Self, Status: e.Status, Ends: e.EstimatedEnd, Detail: detail})
	}
	sort.Slice(p.Rows, func(i, j int) bool {
		a, b := p.Rows[i], p.Rows[j]
		if a.Self != b.Self {
			return a.Self
		}
		if a.Status == "estimated_ready" && b.Status != "estimated_ready" {
			return false
		}
		if a.Status != "estimated_ready" && b.Status == "estimated_ready" {
			return true
		}
		if a.Ends != b.Ends {
			return a.Ends < b.Ends
		}
		if a.GUID != b.GUID {
			return a.GUID < b.GUID
		}
		return a.Spell < b.Spell
	})
	limit := max(1, min(12, c.EstimateRows))
	if len(p.Rows) > limit {
		p.Hidden = len(p.Rows) - limit
		p.Rows = p.Rows[:limit]
	}
	if !c.EstimateSelf && !c.EstimateParty {
		p.Empty = "Choose My cooldowns or Party cooldowns in LamdaCD."
	}
	return p
}
