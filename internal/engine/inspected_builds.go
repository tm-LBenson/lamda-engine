package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

const inspectRetention = 5 * time.Minute

// Snapshot builds keep their original inspection time. A retained scan is
// usable only while the snapshot still contains its continuous party context.
func (s InferenceSnapshot) retainsPartyBuild(b BuildSnapshot, now time.Time) bool {
	joined, member := s.PartySince[b.GUID]
	return !Forever && member && !joined.IsZero() && !joined.After(now) &&
		b.Source == "inspect" && b.SpellRanks != nil && !b.At.IsZero() && !b.At.After(now)
}

func (m *Model) retainsPartyBuild(b BuildSnapshot, now time.Time) bool {
	_, member := m.partyMembers[b.GUID]
	p, exists := m.partyBuilds[b.GUID]
	return !Forever && member && exists && b.Source == "inspect" && b.SpellRanks != nil &&
		b.Revision == p.Build.Revision && b.At.Equal(p.Build.At) && !b.At.After(now) &&
		inspectionMatchesLog(b, m.Builds[b.GUID])
}

// Includes retained inspections for a continuously observed party membership.
func (m *Model) HasInspectedBuild(guid string, now time.Time) bool {
	b, ok := m.buildFor(guid, now)
	return ok && b.Source == "inspect"
}

func (m *Model) ObserveInspectedBuild(b BuildSnapshot, now time.Time) bool {
	return m.observeInspectedBuild(b, now, false)
}

// A fresh live addon packet may reassert an older scan for this same party.
// Preserve its actual inspection time; do not pretend a new inspection occurred.
func (m *Model) ObserveRetainedPartyBuild(b BuildSnapshot, now time.Time) bool {
	if _, member := m.partyMembers[b.GUID]; !member || now.Sub(b.At) > 7*24*time.Hour {
		return false
	}
	return m.observeInspectedBuild(b, now, true)
}
func (m *Model) observeInspectedBuild(b BuildSnapshot, now time.Time, retained bool) bool {
	if Forever || b.Source != "inspect" || b.GUID == "" || b.SpecID <= 0 || b.At.After(now.Add(2*time.Second)) || (!retained && now.Sub(b.At) > inspectRetention) || b.SpellRanks == nil {
		return false
	}
	if prev, ok := m.InspectedBuilds[b.GUID]; ok && !b.At.After(prev.At) {
		return false
	}
	encoded, _ := json.Marshal(struct {
		Spec    int
		Game    string
		Talents [][3]int
		Spells  map[int]int
	}{b.SpecID, b.GameBuild, b.Talents, b.SpellRanks})
	b.Revision = fmt.Sprintf("%x", sha256.Sum256(encoded))
	b.ReceivedAt = now
	if len(m.InspectedBuilds) >= 128 {
		for guid, old := range m.InspectedBuilds {
			if now.Sub(old.At) > inspectRetention {
				delete(m.InspectedBuilds, guid)
			}
		}
		if len(m.InspectedBuilds) >= 128 {
			return false
		}
	}
	m.InspectedBuilds[b.GUID] = b
	if retained {
		if m.partyBuilds == nil {
			m.partyBuilds = map[string]partyBuild{}
		}
		if previous, exists := m.partyBuilds[b.GUID]; !exists || b.At.After(previous.Build.At) {
			since := m.partyMembers[b.GUID]
			if exists {
				since = previous.Since
				if previous.Build.Revision != b.Revision {
					since = maxTime(since, b.At)
					m.clearPartySeeds(b.GUID)
				}
			}
			m.partyBuilds[b.GUID] = partyBuild{Build: b, Since: since}
		}
	} else {
		m.rememberPartyBuild(b)
	}
	m.Diagnostics.InspectedBuilds++
	return true
}

func validInspectedBuild(b BuildSnapshot, logged BuildSnapshot, now time.Time) bool {
	if Forever || b.Source != "inspect" || b.SpellRanks == nil || b.At.After(now.Add(2*time.Second)) || now.Sub(b.At) > inspectRetention {
		return false
	}
	return inspectionMatchesLog(b, logged)
}

func inspectionMatchesLog(b, logged BuildSnapshot) bool {
	if logged.At.After(b.At) {
		if logged.SpecID > 0 && logged.SpecID != b.SpecID {
			return false
		}
		if len(logged.Talents) > 0 {
			for _, want := range b.Talents {
				found := false
				for _, have := range logged.Talents {
					if have == want {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
		}
	}
	return true
}

func (m *Model) buildFor(guid string, now time.Time) (BuildSnapshot, bool) {
	logged, exists := m.Builds[guid]
	if b, ok := m.InspectedBuilds[guid]; ok && validInspectedBuild(b, logged, now) {
		return b, true
	}
	if p, ok := m.partyBuilds[guid]; ok && !now.Before(p.Build.At) && inspectionMatchesLog(p.Build, logged) {
		return p.Build, true
	}
	return logged, exists
}
