package engine

import "time"

type interruptKey struct {
	GUID  string
	Spell int
}
type interruptLiveSpell struct {
	Spell int
	Path  string
}
type interruptUse struct {
	At, LogAt, LiveAt, HitAt time.Time
	MeterAt                  time.Time
	LogSpells                map[int]time.Time
	LiveSpells               map[interruptLiveSpell]time.Time
}
type interruptCounts struct {
	Since    time.Time
	Uses     map[interruptKey][]*interruptUse
	Totals   map[string]int // Uses which landed at least one interrupt.
	Attempts map[string]int
	Size     int
}

const interruptMatchWindow = 1500 * time.Millisecond

func (u *interruptUse) latest() time.Time {
	return maxTime(maxTime(u.LogAt, u.LiveAt), u.HitAt)
}

// Count uses, not interrupted targets. Live casts and buffered log records
// share the existing 1.5s clock tolerance, independent of cooldown inference.
// Distinct casts of the same spell from one source stay distinct after a fast
// reset. Command/pet aliases may describe the same use from that source.
// Public packets and screen capture are separate live paths: the same cast can
// arrive through both with slightly different receipt-derived timestamps.
func (m *Model) countInterruptUse(c Cast, now time.Time, landed bool) (applied bool) {
	outcome := "duplicate_or_already_landed"
	if m.TrackingAudit != nil && (landed || IsInterrupt(c.Spell)) {
		defer func() {
			kind := "interrupt_attempt"
			if landed {
				kind = "interrupt_landed"
			}
			e := m.trackingEvent(kind, c, now)
			e.Applied, e.Outcome = applied, outcome
			m.TrackingAudit(e)
		}()
	}
	if m.DisabledInterrupts || c.Companion || c.GUID == "" || !IsInterrupt(c.Spell) || c.At.After(now.Add(2*time.Second)) || now.Sub(c.At) >= observationRetention || c.At.Before(m.interruptCounts.Since) {
		switch {
		case m.DisabledInterrupts:
			outcome = "disabled"
		case c.Companion || c.GUID == "":
			outcome = "unverified_owner"
		case !IsInterrupt(c.Spell):
			outcome = "unsupported_interrupt_id"
		case c.At.After(now.Add(2 * time.Second)):
			outcome = "future_event"
		case c.At.Before(m.interruptCounts.Since):
			outcome = "before_count_reset"
		default:
			outcome = "event_too_old"
		}
		return false
	}
	if !landed && c.Source != "" && c.Source != "combat_log" && c.Source != "unit_spellcast" {
		outcome = "not_a_cast_source"
		return false
	}
	counts := &m.interruptCounts
	if counts.Uses == nil {
		counts.Uses = map[interruptKey][]*interruptUse{}
		counts.Totals = map[string]int{}
		counts.Attempts = map[string]int{}
	}
	key := interruptKey{c.GUID, InterruptSpell(c.Spell)}
	var use *interruptUse
	best := 2500*time.Millisecond + 1
	for _, candidate := range counts.Uses[key] {
		if !landed {
			sameSource := candidate.LogSpells[c.Spell]
			if c.Source == "unit_spellcast" {
				sameSource = candidate.LiveSpells[interruptLiveSpell{c.Spell, c.LivePath}]
			}
			if !sameSource.IsZero() {
				if sameSource.Equal(c.At) {
					return false
				}
				continue
			}
		}
		distance := c.At.Sub(candidate.At).Abs()
		window := interruptMatchWindow
		if c.Source == "damage_meter" || !candidate.MeterAt.IsZero() {
			window = 2500 * time.Millisecond
		}
		if distance <= window && distance < best {
			best, use = distance, candidate
		}
	}
	if use == nil {
		// Keep lifetime totals; only the input-age window needs matching.
		if counts.Size >= 8192 {
			for key, uses := range counts.Uses {
				keep := uses[:0]
				for _, old := range uses {
					if now.Sub(old.latest()) < observationRetention {
						keep = append(keep, old)
					} else {
						counts.Size--
					}
				}
				if len(keep) == 0 {
					delete(counts.Uses, key)
				} else {
					counts.Uses[key] = keep
				}
			}
		}
		_, known := counts.Attempts[c.GUID]
		if counts.Size >= 8192 || (!known && len(counts.Attempts) >= 128) {
			outcome = "capacity"
			m.RecordGap("interrupt_count_capacity", now, c)
			return false
		}
		use = &interruptUse{At: c.At}
		counts.Uses[key] = append(counts.Uses[key], use)
		counts.Size++
		counts.Attempts[c.GUID]++
	}
	if landed {
		if c.Source == "damage_meter" {
			use.MeterAt = c.At
		}
		if !use.HitAt.IsZero() {
			return false
		}
		use.HitAt = c.At
		counts.Totals[c.GUID]++
	} else if c.Source == "unit_spellcast" {
		if use.LiveSpells == nil {
			use.LiveSpells = map[interruptLiveSpell]time.Time{}
			use.LiveAt = c.At
		}
		use.LiveSpells[interruptLiveSpell{c.Spell, c.LivePath}] = c.At
	} else {
		if use.LogSpells == nil {
			use.LogSpells = map[int]time.Time{}
			use.LogAt = c.At
		}
		use.LogSpells[c.Spell] = c.At
	}
	// Prefer the authoritative cast clock over a live receipt or hit-only seed.
	if !use.LogAt.IsZero() {
		use.At = use.LogAt
	} else if !use.LiveAt.IsZero() {
		use.At = use.LiveAt
	}
	outcome = "matched_use"
	return true
}

// Meter deltas prove a landed interrupt, but their timestamp is the first
// observed increase. Log/cast records later reconcile against the same use.
func (m *Model) ObserveMeterInterrupt(c Cast, now time.Time) bool {
	if c.Source != "damage_meter" || !c.RosterMatched || !m.countInterruptUse(c, now, true) {
		return false
	}
	m.Observe(c, now)
	return true
}

func (m *Model) countInterrupt(c Cast, fields []string, now time.Time) bool {
	if len(fields) < 15 {
		return false
	}
	// A successful interrupt proves a use even without a cast record.
	return m.countInterruptUse(c, now, true)
}

// A delayed new-instance/key boundary preserves newer uses and their results.
func (m *Model) ResetInterruptCounts(at time.Time) {
	if !at.After(m.interruptCounts.Since) {
		return
	}
	old := m.interruptCounts.Uses
	m.interruptCounts = interruptCounts{Since: at, Uses: map[interruptKey][]*interruptUse{}, Totals: map[string]int{}, Attempts: map[string]int{}}
	for key, uses := range old {
		for _, use := range uses {
			if !use.At.Before(at) {
				m.interruptCounts.Uses[key] = append(m.interruptCounts.Uses[key], use)
				m.interruptCounts.Size++
				m.interruptCounts.Attempts[key.GUID]++
				if !use.HitAt.IsZero() {
					m.interruptCounts.Totals[key.GUID]++
				}
			}
		}
	}
	if m.TrackingAudit != nil {
		m.TrackingAudit(TrackingEvent{Kind: "interrupt_count_reset", EventAt: at})
	}
}
