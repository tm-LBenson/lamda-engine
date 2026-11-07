package engine

import "time"

// Local diagnostic records contain ability evidence, never raw chat/log lines.
// The sink must buffer synchronously: the model and its caller own one goroutine.
type TrackingEvent struct {
	Kind       string            `json:"kind"`
	ReceivedAt time.Time         `json:"receivedAt"`
	EventAt    time.Time         `json:"eventAt"`
	GUID       string            `json:"guid,omitempty"`
	Player     string            `json:"player,omitempty"`
	Spell      int               `json:"spell,omitempty"`
	Canonical  int               `json:"canonicalSpell,omitempty"`
	Source     string            `json:"source,omitempty"`
	LivePath   string            `json:"livePath,omitempty"`
	Outcome    string            `json:"outcome,omitempty"`
	Applied    bool              `json:"applied"`
	Landed     int               `json:"landed"`
	Attempts   int               `json:"attempts"`
	Evidence   *CooldownEvidence `json:"currentCooldown,omitempty"`
}

func (m *Model) trackingEvent(kind string, c Cast, now time.Time) TrackingEvent {
	source := c.Source
	if source == "" {
		source = "combat_log"
	}
	return TrackingEvent{Kind: kind, ReceivedAt: now, EventAt: c.At, GUID: c.GUID,
		Player: c.Name, Spell: c.Spell, Canonical: CooldownSpell(c.Spell), Source: source, LivePath: c.LivePath,
		Landed: m.interruptCounts.Totals[c.GUID], Attempts: m.interruptCounts.Attempts[c.GUID]}
}
