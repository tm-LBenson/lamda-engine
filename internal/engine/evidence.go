package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Gap counts affected observations (or failed input checks), never render ticks.
// Only a bounded, structured example is retained; no raw log lines or chat.
type Gap struct {
	Count       uint64    `json:"count"`
	FirstAt     time.Time `json:"firstAt"`
	LastAt      time.Time `json:"lastAt"`
	LastEventAt time.Time `json:"lastEventAt,omitempty"`
	LastGUID    string    `json:"lastGuid,omitempty"`
	LastSpell   int       `json:"lastSpell,omitempty"`
}

type Diagnostics struct {
	ScreenStatus          *lookup.ScreenStatus `json:"screenCastStatus,omitempty"`
	ScreenSamples         uint64               `json:"screenCastSamples"`
	ScreenCaptureFailures uint64               `json:"screenCastCaptureFailures"`
	ScreenCaptureError    string               `json:"screenCastCaptureError,omitempty"`
	ScreenFormat          int                  `json:"screenCastFormat,omitempty"`
	ScreenRejected        uint64               `json:"screenCastRejected"`
	ScreenConfirmed       uint64               `json:"screenCastConfirmed"`
	MeterSamples          uint64               `json:"interruptMeterSamples"`
	MeterConfirmed        uint64               `json:"interruptMeterConfirmed"`
	MeterRebases          uint64               `json:"interruptMeterRebases"`
	AuraSamples           uint64               `json:"auraSamples"`
	AuraCaptureFailures   uint64               `json:"auraCaptureFailures"`
	AuraWatchedAbilities  int                  `json:"auraWatchedAbilities"`
	AuraActivations       uint64               `json:"auraActivations"`
	ReconciledAuraCasts   uint64               `json:"reconciledAuraCasts"`
	LiveCasts             uint64               `json:"liveCasts"`
	ReconciledLogCasts    uint64               `json:"reconciledLogCasts"`
	InspectedBuilds       uint64               `json:"inspectedBuilds"`
	AddonObservations     []uint64             `json:"addonObservations,omitempty"`
	Lines                 uint64               `json:"lines"`
	EligibleCasts         uint64               `json:"eligibleCasts"`
	SelfCasts             uint64               `json:"selfCasts"`
	UnsupportedCasts      uint64               `json:"unsupportedCasts"`
	AcceptedCasts         uint64               `json:"acceptedCasts"`
	BuildRecords          uint64               `json:"buildRecords"`
	SessionResets         uint64               `json:"sessionResets"`
	Duplicates            uint64               `json:"duplicates"`
	OutOfOrder            uint64               `json:"outOfOrder"`
	MaxDeliverySeconds    float64              `json:"maxDeliverySeconds"`
	Gaps                  map[string]Gap       `json:"gaps"`
}

type CooldownEvidence struct {
	GUID             string    `json:"guid"`
	Player           string    `json:"player"`
	Spell            int       `json:"spell"`
	DisplaySpell     int       `json:"displaySpell,omitempty"`
	PvP              *bool     `json:"pvp,omitempty"`
	Name             string    `json:"name"`
	Self             bool      `json:"self"`
	Source           string    `json:"source"`
	Kind             string    `json:"kind"`
	EventAt          time.Time `json:"eventAt"`
	ReceivedAt       time.Time `json:"receivedAt"`
	DeliverySeconds  float64   `json:"deliverySeconds"`
	AgeSeconds       float64   `json:"ageSeconds"`
	CooldownSeconds  float64   `json:"cooldownSeconds"`
	AppliedTalents   []int     `json:"appliedTalents,omitempty"`
	RechargeEnds     []float64 `json:"rechargeEnds,omitempty"`
	BaselineSeconds  float64   `json:"baselineSeconds"`
	EstimatedEnd     float64   `json:"estimatedEnd"`
	Status           string    `json:"status"`
	ReadyInvalidated bool      `json:"readyInvalidated,omitempty"`
	MayHaveCharges   bool      `json:"mayHaveCharges"`
	AvailableCharges *int      `json:"availableCharges"`
	MaxCharges       *int      `json:"maxCharges"`
	BuildRevision    string    `json:"buildRevision,omitempty"`
	BuildSource      string    `json:"buildSource,omitempty"`
	Reasons          []string  `json:"reasons"`
}

type InferenceSnapshot struct {
	Schema            int                  `json:"schema"`
	At                time.Time            `json:"at"`
	LogVersion        int                  `json:"logVersion"`
	GameBuild         string               `json:"gameBuild"`
	Cooldowns         []CooldownEvidence   `json:"cooldowns"`
	Builds            []BuildSnapshot      `json:"builds"`
	PublicBuilds      []lookup.Profile     `json:"publicBuilds"`
	Diagnostics       Diagnostics          `json:"diagnostics"`
	InterruptCounts   map[string]int       `json:"interruptCounts,omitempty"`
	InterruptAttempts map[string]int       `json:"interruptAttempts,omitempty"`
	PartySince        map[string]time.Time `json:"partySince,omitempty"`
}

func (m *Model) RecordGap(code string, now time.Time, c Cast) {
	if len(m.Diagnostics.Gaps) >= 64 {
		if _, exists := m.Diagnostics.Gaps[code]; !exists {
			code = "other"
		}
	}
	g := m.Diagnostics.Gaps[code]
	if g.Count == 0 {
		g.FirstAt = now
	}
	g.Count++
	g.LastAt, g.LastEventAt, g.LastGUID, g.LastSpell = now, c.At, c.GUID, c.Spell
	m.Diagnostics.Gaps[code] = g
}

// Reset drops estimates and build context across session/input discontinuities,
// while keeping diagnostics for the entire engine run or replay.
func (m *Model) Reset() {
	audit := m.TrackingAudit
	disabledCD, disabledKick := m.DisabledDefensives, m.DisabledInterrupts
	diagnostics := m.Diagnostics
	version, game := m.LogVersion, m.GameBuild
	public := m.PublicBuilds
	inspected := m.InspectedBuilds
	lastKey := m.lastFreshKeyAt
	context := m.context
	diagnostics.SessionResets++
	*m = *NewModel()
	m.DisabledDefensives, m.DisabledInterrupts = disabledCD, disabledKick
	m.Diagnostics = diagnostics
	m.LogVersion, m.GameBuild = version, game
	m.PublicBuilds = public
	m.InspectedBuilds = inspected
	m.lastFreshKeyAt = lastKey
	m.context = context
	m.TrackingAudit = audit
	if audit != nil {
		audit(TrackingEvent{Kind: "model_reset"})
	}
}

// RecordHeader avoids parsing thousands of unrelated damage/aura CSV payloads.
func RecordHeader(line string) (time.Time, string, error) {
	match := stamp.FindStringSubmatch(line)
	if match == nil {
		return time.Time{}, "", fmt.Errorf("timestamp")
	}
	event, _, _ := strings.Cut(match[3], ",")
	at, _, err := parseRecord(match[1] + match[2] + "  " + event)
	return at, event, err
}

func (m *Model) Ingest(line string, now time.Time, companions bool) bool {
	return m.IngestWithRoster(line, now, companions, nil)
}

// IngestWithRoster additionally accepts observed player casts matched by exact
// GUID to a fresh in-game party roster. Callers must expire unavailable rosters.
func (m *Model) IngestWithRoster(line string, now time.Time, companions bool, roster map[string]bool) bool {
	m.Diagnostics.Lines++
	at, event, err := RecordHeader(line)
	if err != nil {
		m.RecordGap("malformed_record", now, Cast{})
		return false
	}
	switch event {
	case "COMBAT_LOG_VERSION":
		m.readLogVersion(line, now)
	case "COMBATANT_INFO":
		m.readBuild(line, now)
	case "SPELL_INTERRUPT":
		c, err := parsePartySpell(line, roster, event)
		if err != nil || c.Companion {
			return false
		}
		_, fields, err := parseRecord(line)
		if err != nil {
			return false
		}
		return m.countInterrupt(c, fields, now)
	case "SPELL_CAST_SUCCESS":
		// Valid outsider casts are ordinary filtering, not missing-data errors.
		_, fields, err := parseRecord(line)
		if err != nil || len(fields) < 12 {
			m.RecordGap("malformed_cast", now, Cast{At: at})
			return false
		}
		c, err := parseCast(line, roster)
		if err != nil {
			if err.Error() != "not a supported party member or companion" {
				m.RecordGap("malformed_cast", now, Cast{At: at})
			}
			return false
		}
		m.Diagnostics.EligibleCasts++
		if c.Self {
			m.Diagnostics.SelfCasts++
		}
		if c.Companion && !companions {
			return false
		}
		if _, known := trackedSpell(c.Spell); !known && !c.Companion && c.Spell != 235219 {
			// Rotation spells are not a separate warning for every spell ID.
			m.Diagnostics.UnsupportedCasts++
			return false
		}
		return m.Observe(c, now)
	}
	return false
}

func (m *Model) observePlayer(c Cast, now time.Time) bool {
	spellID := CooldownSpell(c.Spell)
	reset := !Forever && spellID == 235219
	if reset {
		spellID = 45438
	}
	s, known := trackedSpell(spellID)
	if !known {
		return false
	}
	if !Forever && m.context.arena && c.At.Before(m.context.since.Add(-2*time.Second)) {
		m.RecordGap("cast_before_arena_round", now, c)
		return false
	}
	delay := now.Sub(c.At)
	if delay < -2*time.Second {
		m.RecordGap("future_event", now, c)
		return false
	}
	if delay >= observationRetention || (reset && delay >= 10*time.Minute) {
		m.RecordGap("event_too_old", now, c)
		return false
	}
	key := fmt.Sprintf("%s/%d", c.GUID, spellID)
	corroboratedAura := false
	var activationPrevious *CooldownEvidence
	if prev, ok := m.Evidence[key]; ok {
		activeEnd := prev.EstimatedEnd
		for _, end := range prev.RechargeEnds {
			if end > float64(now.UnixMilli())/1000 {
				activeEnd = end
				break
			}
		}
		newMageCharge := spellID == 45438 && prev.MaxCharges != nil && *prev.MaxCharges > 1 && math.Abs(c.At.Sub(prev.EventAt).Seconds()) > 2
		if c.Source == "aura_activation" && prev.Kind == "cast" && !newMageCharge &&
			(activeEnd > float64(now.UnixMilli())/1000 || math.Abs(c.At.Sub(prev.EventAt).Seconds()) <= 2) {
			// An actual cast already accounts for this activation/charge. The
			// buff display cannot replace a stronger timer or spend another charge.
			return false
		}
		if prev.Kind == "activation" && c.Source != "aura_activation" && math.Abs(c.At.Sub(prev.EventAt).Seconds()) <= 2 {
			// Replace the provisional onset with the actual cast timestamp. Do
			// not keep a provisional recharge in the charge queue as a second use.
			if spellID == 45438 && len(prev.RechargeEnds) > 0 {
				previous := prev
				previous.RechargeEnds = append([]float64{}, prev.RechargeEnds[:len(prev.RechargeEnds)-1]...)
				activationPrevious = &previous
			}
			delete(m.Evidence, key)
			delete(m.Seen, key)
			m.Diagnostics.ReconciledAuraCasts++
			corroboratedAura = true
		}
	}
	if prev, ok := m.Evidence[key]; ok && prev.Source == "combat_log" && c.Source == "unit_spellcast" && prev.Kind == "cast" && !reset && math.Abs(c.At.Sub(prev.EventAt).Seconds()) <= 1.5 {
		prev.Source = "unit_spellcast"
		prev.Reasons = append(prev.Reasons, "live_cast_corroborated")
		m.Evidence[key] = prev
		m.Diagnostics.LiveCasts++
		return false
	}
	if prev, ok := m.Evidence[key]; ok && prev.Source == "unit_spellcast" && c.Source == "" && prev.Kind == "cast" && !reset && math.Abs(c.At.Sub(prev.EventAt).Seconds()) <= 1.5 {
		m.Diagnostics.ReconciledLogCasts++
		m.Diagnostics.MaxDeliverySeconds = max(m.Diagnostics.MaxDeliverySeconds, delay.Seconds())
		return false
	}
	if prev, ok := m.Evidence[key]; ok && prev.Kind == "cast" && IsInterrupt(spellID) && math.Abs(c.At.Sub(prev.EventAt).Seconds()) <= 1.5 {
		m.Diagnostics.Duplicates++
		return false
	}
	if prev, exists := m.Seen[key]; exists && !c.At.After(prev) {
		if c.At.Equal(prev) {
			m.Diagnostics.Duplicates++
		} else {
			m.Diagnostics.OutOfOrder++
		}
		return false
	}
	e := CooldownEvidence{GUID: c.GUID, Player: c.Name, Spell: spellID, Name: s.Name,
		Self: c.Self, Source: "combat_log", Kind: "cast", EventAt: c.At, ReceivedAt: now,
		DeliverySeconds: delay.Seconds(), BaselineSeconds: s.Cooldown, CooldownSeconds: s.Cooldown,
		EstimatedEnd: float64(c.At.UnixMilli())/1000 + s.Cooldown, MayHaveCharges: s.Charges,
		Reasons: []string{"reference_baseline"}}
	e.PvP = m.pvpContext(c.At)
	if !Forever && (c.Spell == 414658 || (spellID == 45438 && m.Evidence[key].DisplaySpell == 414658)) {
		e.DisplaySpell, e.Name = 414658, "Ice Cold"
	}
	if Forever {
		e.Reasons = append(e.Reasons, "beta_public_metadata", "beta_talent_modifiers_unmodeled")
	}
	if c.Source == "unit_spellcast" {
		e.Source = c.Source
		e.Reasons = append(e.Reasons, "live_cast_receipt_estimate")
		m.Diagnostics.LiveCasts++
	}
	if c.Source == "damage_meter" {
		e.Source = c.Source
		e.Reasons = append(e.Reasons, "landed_interrupt_counter", "interrupt_spell_inferred", "meter_receipt_estimate")
	}
	if c.Source == "aura_activation" {
		e.Source = c.Source
		e.Kind = "activation"
		e.Reasons = append(e.Reasons, "buff_onset_estimate", "activation_not_cast_confirmed")
		m.Diagnostics.AuraActivations++
	}
	if corroboratedAura {
		e.Reasons = append(e.Reasons, "buff_start_corroborated")
	}
	if c.RosterMatched {
		e.Reasons = append(e.Reasons, "group_roster_match")
	}
	addGap := func(code string) { e.Reasons = append(e.Reasons, code); m.RecordGap(code, now, c) }
	if b, exists := m.buildFor(c.GUID, now); !exists {
		if p, known := m.PublicBuilds[c.GUID]; known && now.Sub(p.FetchedAt) < 24*time.Hour && !p.FetchedAt.After(now) {
			if p.SourceUpdatedAt != nil && p.SourceUpdatedAt.After(c.At) {
				addGap("public_build_newer_than_cast")
			} else {
				e.BuildRevision, e.BuildSource = p.Revision, p.Source
			}
			addGap("public_build_provisional")
			if len(p.Missing) > 0 {
				addGap("partial_public_build")
			}
			if p.SourceUpdatedAt == nil {
				addGap("public_build_age_unknown")
			} else if c.At.Sub(*p.SourceUpdatedAt) > 24*time.Hour {
				addGap("public_build_stale")
			}
			addGap("unmapped_build_modifiers")
		} else {
			addGap("missing_build")
		}
	} else if b.At.After(c.At) {
		addGap("build_newer_than_cast")
	} else if c.At.Sub(b.At) > observationRetention && !m.retainsPartyBuild(b, now) {
		addGap("stale_build")
	} else {
		e.BuildRevision = b.Revision
		e.BuildSource = b.Source
		if len(b.Missing) > 0 {
			addGap("partial_build")
		}
		// Capturing a talent tuple does not establish its cooldown effect.
		addGap("unmapped_build_modifiers")
	}
	if b, ok := m.buildFor(c.GUID, now); ok && !b.At.After(c.At) {
		applyBuildRules(&e, b)
	}
	if s.Charges {
		addGap("unknown_charge_count")
	}
	if delay > 2*time.Second {
		addGap("late_delivery")
	}
	if !reset && e.CooldownSeconds > 0 && float64(now.UnixMilli())/1000 >= e.EstimatedEnd {
		addGap("arrived_after_estimated_end")
	}
	if previous, exists := m.Evidence[key]; exists && !reset && previous.Kind == "cast" && c.At.Sub(previous.EventAt).Seconds() < s.Cooldown {
		// A short reuse can be a charge, reset or modifier. Never learn the
		// cooldown by simply replacing it with the player's reuse interval.
		addGap("reuse_before_baseline")
	}
	if reset {
		e.Kind = "reset"
		e.EstimatedEnd = float64(c.At.UnixMilli()) / 1000
	}
	if s.Cooldown <= 0 {
		e.EstimatedEnd = 0
		if Forever {
			e.Reasons = append(e.Reasons, "beta_baseline_unavailable")
		} else {
			e.Reasons = append(e.Reasons, "resource_based_ability")
		}
	}
	if previous, ok := m.Evidence[key]; ok {
		e.rechargeAfter(previous)
	} else if activationPrevious != nil {
		e.rechargeAfter(*activationPrevious)
	} else {
		e.rechargeAfter(CooldownEvidence{})
	}
	m.remember(key, c.At)
	m.Evidence[key] = e
	if c.Source != "aura_activation" {
		m.Diagnostics.AcceptedCasts++
	}
	m.Diagnostics.MaxDeliverySeconds = max(m.Diagnostics.MaxDeliverySeconds, delay.Seconds())
	delete(m.Rows, key)
	if reset || s.Cooldown <= 0 || e.EstimatedEnd <= float64(now.UnixMilli())/1000 {
		return false
	}
	m.Rows[key] = Row{GUID: c.GUID, Player: c.Name, Spell: spellID, Name: s.Name,
		Ends: e.EstimatedEnd, Charges: s.Charges, Delay: delay.Seconds(), Duration: e.CooldownSeconds}
	return true
}

func (m *Model) Snapshot(now time.Time) InferenceSnapshot {
	m.Active(now) // Shared retention with legacy display; never train on preview rows.
	m.seedFreshKey(now)
	m.seedPartyReady(now)
	result := InferenceSnapshot{Schema: 1, At: now, LogVersion: m.LogVersion,
		GameBuild: m.GameBuild, Cooldowns: []CooldownEvidence{}, Builds: []BuildSnapshot{}, Diagnostics: m.Diagnostics}
	result.InterruptCounts = map[string]int{}
	result.InterruptAttempts = map[string]int{}
	result.PartySince = map[string]time.Time{}
	for guid, joined := range m.partyMembers {
		if b, ok := m.partyBuilds[guid]; ok {
			joined = maxTime(joined, b.Since)
		}
		result.PartySince[guid] = joined
	}
	for guid, count := range m.interruptCounts.Totals {
		result.InterruptCounts[guid] = count
	}
	for guid, count := range m.interruptCounts.Attempts {
		result.InterruptAttempts[guid] = count
	}
	// Copy the map: a caller may retain this snapshot while ingestion continues.
	result.Diagnostics.Gaps = make(map[string]Gap, len(m.Diagnostics.Gaps))
	for code, gap := range m.Diagnostics.Gaps {
		result.Diagnostics.Gaps[code] = gap
	}
	for key, e := range m.Evidence {
		e.Reasons = append([]string{}, e.Reasons...)
		keepKey := m.keepKeyEvidence(e, now)
		keepReady := keepKey || m.keepPartyEvidence(e, now)
		e.AgeSeconds = max(0, now.Sub(e.EventAt).Seconds())
		if n := len(e.RechargeEnds); n > 0 {
			e.EstimatedEnd = e.RechargeEnds[n-1]
		}
		for _, end := range e.RechargeEnds {
			if end > float64(now.UnixMilli())/1000 {
				e.EstimatedEnd = end
				break
			}
		}
		e.Status = "estimated_cooldown"
		if e.MayHaveCharges {
			e.Status = "estimated_recharge"
		}
		if e.BaselineSeconds <= 0 {
			e.Status = "unknown"
		} else if e.EstimatedEnd <= float64(now.UnixMilli())/1000 {
			e.Status = "estimated_ready"
			if e.Kind == "activation" && !keepReady {
				e.Status = "unknown"
			}
			// A buffered interrupt that first arrives after its whole cooldown
			// tells us about a past cast, not current availability. Keep the
			// evidence. A confirmed key or continuously observed party opts into
			// assuming no unseen reuse; outside those contexts it stays unknown.
			if !keepReady && IsInterrupt(e.Spell) && e.Kind == "cast" && !e.ReceivedAt.IsZero() && float64(e.ReceivedAt.UnixMilli())/1000 >= e.EstimatedEnd {
				e.Status = "unknown"
			}
			if !keepReady && e.MayHaveCharges && e.Kind != "reset" && e.Kind != "key_start" {
				e.Status = "unknown"
			}
			if keepReady {
				reason := "same_party_ready_estimate"
				if keepKey {
					reason = "same_key_ready_estimate"
				}
				e.Reasons = append(e.Reasons, reason)
			} else if float64(now.UnixMilli())/1000-e.EstimatedEnd > 60 {
				e.Status = "unknown"
				e.Reasons = append(e.Reasons, "observation_stale")
			}
		}
		if b, exists := m.buildFor(e.GUID, now); exists && b.At.After(e.EventAt) && b.Revision != e.BuildRevision {
			if e.Kind == "key_start" || (keepReady && e.BuildRevision != "") {
				// A known build change must not silently become Ready again
				// when the short-lived inspection cache expires. A new observed
				// use creates fresh evidence and clears this invalidation.
				e.ReadyInvalidated = true
				stored := m.Evidence[key]
				stored.ReadyInvalidated = true
				m.Evidence[key] = stored
			}
			if e.BuildRevision == "" {
				e.Reasons = append(e.Reasons, "build_observed_after_cast")
			} else {
				e.Reasons = append(e.Reasons, "build_changed_since_cast")
			}
		}
		if e.ReadyInvalidated && e.EstimatedEnd <= float64(now.UnixMilli())/1000 {
			e.Status = "unknown"
			e.Reasons = append(e.Reasons, "ready_invalidated_by_build_change")
		}
		result.Cooldowns = append(result.Cooldowns, e)
	}
	builds := map[string]BuildSnapshot{}
	for guid, b := range m.Builds {
		builds[guid] = b
	}
	for guid, b := range m.InspectedBuilds {
		if validInspectedBuild(b, m.Builds[guid], now) {
			builds[guid] = b
		} else if now.Sub(b.At) > inspectRetention {
			delete(m.InspectedBuilds, guid)
		}
	}
	// The scan policy may intentionally never refresh this party. Present its
	// verified retained inspection without pretending it was recently scanned.
	for guid, p := range m.partyBuilds {
		if result.retainsPartyBuild(p.Build, now) && inspectionMatchesLog(p.Build, m.Builds[guid]) {
			builds[guid] = p.Build
		}
	}
	for guid, b := range builds {
		if now.Sub(b.At) > observationRetention && !result.retainsPartyBuild(b, now) {
			delete(m.Builds, guid)
			continue
		}
		result.Builds = append(result.Builds, b)
	}
	result.PublicBuilds = []lookup.Profile{}
	for guid, p := range m.PublicBuilds {
		if now.Sub(p.FetchedAt) > 24*time.Hour {
			delete(m.PublicBuilds, guid)
			continue
		}
		result.PublicBuilds = append(result.PublicBuilds, p)
	}
	sort.Slice(result.PublicBuilds, func(i, j int) bool {
		return result.PublicBuilds[i].Character.GUID < result.PublicBuilds[j].Character.GUID
	})
	sort.Slice(result.Cooldowns, func(i, j int) bool {
		a, b := result.Cooldowns[i], result.Cooldowns[j]
		if a.GUID == b.GUID {
			return a.Spell < b.Spell
		}
		return a.GUID < b.GUID
	})
	sort.Slice(result.Builds, func(i, j int) bool { return result.Builds[i].GUID < result.Builds[j].GUID })
	return result
}

// Public snapshots supplement local evidence. They never replace a combat-log
// build, use HTTP receipt time as build time, or rewrite past cooldown estimates.
func (m *Model) ObservePublicBuild(p lookup.Profile) {
	if !p.Character.Valid() || p.SpecID <= 0 || len(p.Talents) == 0 {
		return
	}
	if old, ok := m.PublicBuilds[p.Character.GUID]; ok && !p.FetchedAt.After(old.FetchedAt) {
		return
	}
	if old, ok := m.PublicBuilds[p.Character.GUID]; ok && old.SourceUpdatedAt != nil && p.SourceUpdatedAt != nil && p.SourceUpdatedAt.Before(*old.SourceUpdatedAt) {
		m.RecordGap("public_build_regressed", p.FetchedAt, Cast{GUID: p.Character.GUID})
		return
	}
	if len(m.PublicBuilds) >= 128 {
		var oldest string
		for guid, b := range m.PublicBuilds {
			if oldest == "" || b.FetchedAt.Before(m.PublicBuilds[oldest].FetchedAt) {
				oldest = guid
			}
		}
		delete(m.PublicBuilds, oldest)
	}
	m.PublicBuilds[p.Character.GUID] = p
}
