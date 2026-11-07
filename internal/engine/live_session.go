package engine

import "time"

// Log sessions are not party sessions. Keep continuously observed party state
// across zoning, log rotation and key completion. Outside that roster, only
// live observations newer than the boundary survive.
func (m *Model) ResetLog(at time.Time) {
	freshKey := m.FreshKey
	counts := m.interruptCounts
	partyBuilds := m.partyBuilds
	party := m.partyMembers
	builds := map[string]BuildSnapshot{}
	for guid, b := range m.Builds {
		if _, member := party[guid]; member {
			builds[guid] = b
		}
	}
	preserveKey := freshKey != nil && (at.IsZero() || !at.After(freshKey.At))
	retained := map[string]CooldownEvidence{}
	rows := map[string]Row{}
	for key, e := range m.Evidence {
		keepParty := m.keepPartyEvidence(e, e.EventAt)
		if ((e.Source == "unit_spellcast" || e.Source == "aura_activation" || e.Source == "damage_meter") && (at.IsZero() || !e.EventAt.Before(at))) ||
			(preserveKey && (e.Kind == "key_start" || !e.EventAt.Before(freshKey.At) || e.EstimatedEnd > float64(freshKey.At.UnixMilli())/1000)) || keepParty {
			retained[key] = e
			if row, ok := m.Rows[key]; ok {
				rows[key] = row
			}
		}
	}
	m.Reset()
	m.interruptCounts = counts
	m.partyMembers = party
	m.partyBuilds = partyBuilds
	m.Builds = builds
	if preserveKey {
		m.FreshKey = freshKey
	}
	for key, e := range retained {
		m.Evidence[key] = e
		if e.Kind != "key_start" && e.Kind != "party_ready" {
			m.Seen[key] = e.EventAt
		}
	}
	for key, row := range rows {
		m.Rows[key] = row
	}
}
