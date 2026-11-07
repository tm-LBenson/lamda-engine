package augmentation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Raw damage is counted once. Support hooks transfer part of it between rows;
// they never increase the group's total. Each hook credits its recorded owner.
type DamageRow struct {
	Estimated int64   `json:"estimated,omitempty"`
	GUID      string  `json:"guid"`
	Name      string  `json:"name"`
	Raw       int64   `json:"raw"`
	Removed   int64   `json:"removed"`
	Added     int64   `json:"added"`
	Adjusted  int64   `json:"adjusted"`
	DPS       float64 `json:"dps"`
}
type DamageMeter struct {
	AugPresenceKnown     bool                        `json:"augPresenceKnown,omitempty"`
	HasAug               bool                        `json:"hasAug,omitempty"`
	BossVisit            string                      `json:"bossVisit,omitempty"`
	Difficulty           string                      `json:"difficulty,omitempty"`
	OutcomeKnown         bool                        `json:"outcomeKnown,omitempty"`
	Success              bool                        `json:"success,omitempty"`
	Utility              *summaryCounts              `json:"utility,omitempty"`
	DamageBasis          string                      `json:"damageBasis,omitempty"`
	Historical           bool                        `json:"historical,omitempty"`
	Metric               int                         `json:"metric,omitempty"`
	AttributionMode      string                      `json:"attributionMode,omitempty"`
	PendingSupportOwners map[string]map[string]int64 `json:"pendingSupportOwners,omitempty"`
	Provisional          bool                        `json:"provisional,omitempty"`
	CalibrationMissing   bool                        `json:"calibrationMissing,omitempty"`
	InputSource          string                      `json:"inputSource,omitempty"`
	NativeSessionID      int                         `json:"nativeSessionID,omitempty"`
	NativeSeenAt         time.Time                   `json:"nativeSeenAt,omitempty"`
	NativeError          string                      `json:"nativeError,omitempty"`
	EstimatedSupport     int64                       `json:"estimatedSupport,omitempty"`
	RecordedSupport      int64                       `json:"recordedSupport,omitempty"`
	PendingRaw           map[string]int64            `json:"pendingRaw,omitempty"`
	PendingSupport       map[string]int64            `json:"pendingSupport,omitempty"`
	CollectionValid      bool                        `json:"collectionValid,omitempty"`
	ID                   string                      `json:"id"`
	Name                 string                      `json:"name"`
	Kind                 string                      `json:"kind"`
	Owner                string                      `json:"owner"`
	StartedAt            time.Time                   `json:"startedAt"`
	EndedAt              time.Time                   `json:"endedAt"`
	Seconds              float64                     `json:"seconds"`
	Live                 bool                        `json:"live"`
	Valid                bool                        `json:"valid"`
	Partial              bool                        `json:"partial"`
	RawTotal             int64                       `json:"rawTotal"`
	AdjustedTotal        int64                       `json:"adjustedTotal"`
	UnassignedPetDamage  int64                       `json:"unassignedPetDamage"`
	RejectedHooks        int                         `json:"rejectedHooks"`
	Rows                 []DamageRow                 `json:"rows"`
	Note                 string                      `json:"note"`
}
type transferKey struct{ source, owner string }
type damageSession struct {
	encounterState                                  *encounterAwardState
	voidscarState                                   *voidscarAwardState
	details                                         map[damageDetailKey]*DamageDetailEntry
	detailsTruncated                                bool
	augSeen                                         bool
	healingRoles                                    map[string]string
	healTargets                                     map[healingTarget]healingAmounts
	unknownMechanics                                map[string]bool
	mechanicTotals                                  map[string]map[string]int64
	bombCarriers                                    map[string]time.Time
	bombDrops                                       map[string]bombDrop
	bombDetonation                                  time.Time
	dropBlocked                                     map[string]time.Time
	mutilated                                       map[string]time.Time
	healing, overhealing, interrupts, resurrections map[string]int64
	firstDeaths, avoidableHits, mathFails           map[string]int64
	mathFailed                                      map[string]bool
	firstDeathAt                                    time.Time
	healingInvalid                                  bool
	fish                                            map[string]int64
	lastFish                                        map[string]time.Time
	soaks, avoidable                                map[string]int64
	lastSoak                                        map[string]time.Time
	mechanicsInvalid                                bool
	players                                         map[string]bool
	raw                                             map[string]int64
	transfers                                       map[transferKey]int64
	rejected                                        int
	dispels                                         map[string]int64
	deaths                                          map[string]int64
}

func newDamageSession() *damageSession {
	return &damageSession{healing: map[string]int64{}, overhealing: map[string]int64{}, interrupts: map[string]int64{}, resurrections: map[string]int64{}, firstDeaths: map[string]int64{}, avoidableHits: map[string]int64{}, mathFails: map[string]int64{}, mathFailed: map[string]bool{}, fish: map[string]int64{}, lastFish: map[string]time.Time{}, soaks: map[string]int64{}, avoidable: map[string]int64{}, lastSoak: map[string]time.Time{}, players: map[string]bool{}, raw: map[string]int64{}, transfers: map[transferKey]int64{}, dispels: map[string]int64{}, deaths: map[string]int64{}}
}

func flags(value string) uint64       { n, _ := strconv.ParseUint(value, 0, 64); return n }
func friendlyGroup(value string) bool { return flags(value)&0x7 != 0 && flags(value)&0x10 != 0 }
func player(guid string) bool         { return strings.HasPrefix(guid, "Player-") }
func damagePayload(f []string) (base int, support bool, ok bool) {
	if len(f) == 0 {
		return
	}
	switch f[0] {
	case "SWING_DAMAGE":
		return 9, false, true
	case "SWING_MISSED":
		return 9, false, true
	case "SPELL_MISSED", "SPELL_PERIODIC_MISSED", "SPELL_BUILDING_MISSED", "RANGE_MISSED", "DAMAGE_SHIELD_MISSED":
		return 12, false, true
	case "SPELL_DAMAGE", "SPELL_PERIODIC_DAMAGE", "RANGE_DAMAGE", "DAMAGE_SHIELD", "DAMAGE_SPLIT", "SPELL_BUILDING_DAMAGE":
		return 12, false, true
	case "SPELL_DAMAGE_SUPPORT", "SPELL_PERIODIC_DAMAGE_SUPPORT", "SWING_DAMAGE_LANDED_SUPPORT", "RANGE_DAMAGE_SUPPORT", "DAMAGE_SHIELD_SUPPORT", "DAMAGE_SPLIT_SUPPORT":
		return 12, true, true
	}
	return
}

// Blizzard and Details include damage to absorbs. Fully absorbed attacks use
// the short MISSED payload; partial absorbs use the advanced damage payload.
func recordedDamageAmount(f []string, base int) (int64, string) {
	if strings.HasSuffix(f[0], "_MISSED") {
		if len(f) <= base {
			return 0, "short_damage_payload"
		}
		if f[base] != "ABSORB" {
			return 0, "not_damage"
		}
		if len(f) < base+3 {
			return 0, "short_damage_payload"
		}
		n, err := strconv.ParseInt(f[base+2], 10, 64)
		if err != nil || n < 0 || n > 1e12 {
			return 0, "invalid_absorbed_amount"
		}
		return n, ""
	}
	if len(f) < base+29 {
		return 0, "short_damage_payload"
	}
	amount, err := strconv.ParseInt(f[base+19], 10, 64)
	overkill, overErr := strconv.ParseInt(f[base+21], 10, 64)
	if err != nil || overErr != nil || amount < 0 || amount > 1e12 {
		return 0, "invalid_amount_or_overkill"
	}
	absorbed, absErr := strconv.ParseInt(f[base+25], 10, 64)
	if absErr != nil || absorbed < 0 || absorbed > 1e12 {
		return 0, "invalid_absorbed_amount"
	}
	return max(int64(0), amount-max(int64(0), overkill)) + absorbed, ""
}
func (s *Store) sourceOwner(guid string) string {
	for i := 0; i < 8; i++ {
		if player(guid) {
			return guid
		}
		next := s.pets[guid]
		if next == "" || next == guid {
			return ""
		}
		guid = next
	}
	return ""
}

func (s *Store) trackDamage(f []string) {
	if len(f) < 9 {
		return
	}
	s.observeDetailHit(f)
	if player(f[1]) && friendlyGroup(f[3]) {
		s.party[f[1]] = f[2]
		for _, seg := range s.active {
			seg.damage.players[f[1]] = true
		}
		if flags(f[3])&1 != 0 {
			s.own = f[1]
		}
	}
	// Count utility while the original stream is already being ingested. A
	// completed report must not rescan a whole log before its first share.
	for _, seg := range s.active {
		s.trackBossMechanics(seg, f)
		s.trackAwardStats(seg, f)
		if (f[0] == "SPELL_DISPEL" || f[0] == "SPELL_STOLEN") && friendlyGroup(f[3]) {
			seg.damage.dispels[f[1]]++
		}
		if f[0] == "UNIT_DIED" && player(f[5]) && friendlyGroup(f[7]) {
			seg.damage.deaths[f[5]]++
			seg.damage.players[f[5]] = true
			s.party[f[5]] = f[6]
		}
	}
	if (f[0] == "SPELL_SUMMON" || f[0] == "SPELL_CREATE") && friendlyGroup(f[3]) && f[1] != f[5] {
		s.petOwner(f[5], f[1], "summon")
	}
	info := 12
	if strings.HasPrefix(f[0], "SWING_") {
		info = 9
	}
	if !player(f[1]) && len(f) >= info+19 && f[info] == f[1] && player(f[info+1]) {
		s.petOwner(f[1], f[info+1], "advanced_unit_data")
	}
	base, support, ok := damagePayload(f)
	if !ok {
		return
	}
	if !friendlyGroup(f[3]) || flags(f[7])&0x10 != 0 {
		if friendlyGroup(f[3]) {
			s.debugDecision("skipped", "friendly_fire", f, 0, support)
		}
		return
	}
	amount, reason := recordedDamageAmount(f, base)
	if reason == "not_damage" {
		return
	}
	if reason != "" {
		s.hybridInput(f, 0, support, true)
		s.debugDecision("skipped", reason, f, 0, support)
		for _, seg := range s.active {
			seg.damage.rejected++
		}
		return
	}
	// Advanced unit data can identify a pet owner even if its summon preceded
	// attachment. Never mistake the target's advanced data for source ownership.
	if !strings.HasSuffix(f[0], "_MISSED") && f[base] == f[1] && player(f[base+1]) {
		s.petOwner(f[1], f[base+1], "advanced_damage_data")
	}
	if !friendlyGroup(f[3]) || flags(f[7])&0x10 != 0 {
		return
	} // no enemy or friendly-fire damage
	if support {
		if len(f) != base+30 || !player(f[base+29]) {
			s.hybridInput(f, 0, true, true)
			s.debugDecision("skipped", "invalid_support_owner_or_format", f, amount, true)
			for _, seg := range s.active {
				seg.damage.rejected++
			}
			return
		}
		s.debugDecision("support_recorded", "", f, amount, true)
		s.hybridInput(f, amount, true, false)
		for _, seg := range s.active {
			seg.damage.transfers[transferKey{f[1], f[base+29]}] += amount
			s.trackMeterDetail(seg, f, amount, true, base)
		}
	} else {
		s.debugDecision("raw_damage_recorded", "", f, amount, false)
		s.hybridInput(f, amount, false, false)
		for _, seg := range s.active {
			seg.damage.raw[f[1]] += amount
			s.trackMeterDetail(seg, f, amount, false, base)
		}
	}
}

func (s *Store) damageView(seg *segment, at time.Time, live bool) DamageMeter {
	v := DamageMeter{ID: fmt.Sprintf("%s/%s/%d", seg.kind, seg.id, seg.start.UnixMilli()), Name: seg.name, Kind: seg.kind,
		Owner: s.own, StartedAt: seg.start, EndedAt: at, Seconds: max(0, at.Sub(seg.start).Seconds()), Live: live, Valid: !seg.invalid,
		Rows: []DamageRow{}, RejectedHooks: seg.damage.rejected,
		AugPresenceKnown: true, HasAug: seg.damage.augSeen, AttributionMode: "all", Note: "Recorded support is credited to each supporting player's GUID. Missing or incorrect Blizzard hooks can affect attribution. DPS uses the whole segment duration."}
	v.DamageBasis = "health+absorbs"
	v.CollectionValid = !seg.invalid && s.own != ""
	if !live && !seg.invalid {
		c := &summaryCounts{Known: true, Dispels: map[string]int64{}, Deaths: map[string]int64{}}
		s.bossCountView(seg, c)
		s.awardCountView(seg, c)
		for source, count := range seg.damage.dispels {
			owner := s.sourceOwner(source)
			if owner == "" {
				c.Known = false
			} else {
				c.Dispels[owner] += count
			}
		}
		for guid, count := range seg.damage.deaths {
			c.Deaths[guid] = count
		}
		v.Utility = c
	}
	rows := map[string]*DamageRow{}
	row := func(guid string) *DamageRow {
		r := rows[guid]
		if r == nil {
			name := s.party[guid]
			if name == "" {
				name = s.names[guid]
			}
			if name == "" {
				name = guid
			}
			r = &DamageRow{GUID: guid, Name: name}
			rows[guid] = r
		}
		return r
	}
	if v.Utility != nil {
		for _, counts := range []map[string]int64{v.Utility.Healing, v.Utility.Interrupts, v.Utility.Resurrections, v.Utility.MathFails, v.Utility.Fish} {
			for guid := range counts {
				row(guid)
			}
		}
		for guid := range v.Utility.Dispels {
			row(guid)
		}
		for guid := range v.Utility.Deaths {
			row(guid)
		}
	}
	for guid := range seg.damage.players {
		row(guid)
	}
	for source, amount := range seg.damage.raw {
		owner := s.sourceOwner(source)
		if owner == "" {
			v.UnassignedPetDamage += amount
			if v.PendingRaw == nil {
				v.PendingRaw = map[string]int64{}
			}
			v.PendingRaw[source] += amount
			continue
		}
		row(owner).Raw += amount
	}
	// Every support owner receives their own recorded contribution, including
	// when this client is playing a healer or ordinary damage specialization.
	for key, amount := range seg.damage.transfers {
		beneficiary := s.sourceOwner(key.source)
		if beneficiary == "" {
			v.Valid = false
			if v.PendingSupportOwners == nil {
				v.PendingSupportOwners = map[string]map[string]int64{}
			}
			if v.PendingSupportOwners[key.owner] == nil {
				v.PendingSupportOwners[key.owner] = map[string]int64{}
			}
			v.PendingSupportOwners[key.owner][key.source] += amount
			continue
		}
		if beneficiary == key.owner {
			continue
		} // self-buffs already belong to the Aug
		row(beneficiary).Removed += amount
		row(key.owner).Added += amount
	}
	for _, r := range rows {
		r.Adjusted = r.Raw - r.Removed + r.Added
		if r.Adjusted < 0 {
			v.Valid = false
		}
		if v.Seconds > 0 {
			r.DPS = float64(r.Adjusted) / v.Seconds
		}
		v.RawTotal += r.Raw
		v.AdjustedTotal += r.Adjusted
		v.Rows = append(v.Rows, *r)
	}
	v.Partial = v.RejectedHooks > 0 || v.UnassignedPetDamage > 0 || len(v.PendingSupportOwners) > 0
	if v.UnassignedPetDamage > 0 || s.own == "" {
		v.Valid = false
	}
	sort.Slice(v.Rows, func(i, j int) bool {
		if v.Rows[i].Adjusted != v.Rows[j].Adjusted {
			return v.Rows[i].Adjusted > v.Rows[j].Adjusted
		}
		return v.Rows[i].GUID < v.Rows[j].GUID
	})
	return v
}

// Ownership can arrive just after ENCOUNTER_END (for example a final guardian
// cast). Preserve unresolved source totals so completed meters can recover as
// soon as that authoritative record arrives, including after an engine restart.
func (s *Store) petOwner(pet, owner, method string) {
	if pet == "" || owner == "" || pet == owner || s.pets[pet] == owner {
		return
	}
	s.debugOwnership(pet, owner, method)
	s.pets[pet] = owner
	s.repairMeterDetailOwners()
	repaired := false
	for i, old := range s.damageMeters {
		if len(old.PendingRaw) == 0 && len(old.PendingSupport) == 0 && len(old.PendingSupportOwners) == 0 {
			continue
		}
		v := old
		rows := map[string]*DamageRow{}
		for _, r := range old.Rows {
			r := r
			rows[r.GUID] = &r
		}
		row := func(guid string) *DamageRow {
			if r := rows[guid]; r != nil {
				return r
			}
			name := s.party[guid]
			if name == "" {
				name = s.names[guid]
			}
			if name == "" {
				name = guid
			}
			r := &DamageRow{GUID: guid, Name: name}
			rows[guid] = r
			return r
		}
		// Allocate new maps/slices: HTTP snapshots may still be reading old ones.
		v.PendingRaw = map[string]int64{}
		v.PendingSupport = map[string]int64{}
		v.PendingSupportOwners = map[string]map[string]int64{}
		changed := false
		for guid, amount := range old.PendingRaw {
			resolved := s.sourceOwner(guid)
			if resolved == "" {
				v.PendingRaw[guid] = amount
				continue
			}
			row(resolved).Raw += amount
			v.UnassignedPetDamage -= amount
			changed = true
		}
		for guid, amount := range old.PendingSupport {
			resolved := s.sourceOwner(guid)
			if resolved == "" {
				v.PendingSupport[guid] = amount
				continue
			}
			if resolved != v.Owner {
				row(resolved).Removed += amount
				row(v.Owner).Added += amount
			}
			changed = true
		}
		for caster, pending := range old.PendingSupportOwners {
			for guid, amount := range pending {
				resolved := s.sourceOwner(guid)
				if resolved == "" {
					if v.PendingSupportOwners[caster] == nil {
						v.PendingSupportOwners[caster] = map[string]int64{}
					}
					v.PendingSupportOwners[caster][guid] = amount
					continue
				}
				if resolved != caster {
					row(resolved).Removed += amount
					row(caster).Added += amount
				}
				changed = true
			}
		}
		if !changed {
			continue
		}
		v.Rows = nil
		v.RawTotal = 0
		v.AdjustedTotal = 0
		v.Valid = v.CollectionValid && v.UnassignedPetDamage == 0 && len(v.PendingSupport) == 0 && len(v.PendingSupportOwners) == 0
		for _, r := range rows {
			r.Adjusted = r.Raw - r.Removed + r.Added
			if r.Adjusted < 0 {
				v.Valid = false
			}
			if v.Seconds > 0 {
				r.DPS = float64(r.Adjusted) / v.Seconds
			}
			v.RawTotal += r.Raw
			v.AdjustedTotal += r.Adjusted
			v.Rows = append(v.Rows, *r)
		}
		v.Partial = v.RejectedHooks > 0 || v.UnassignedPetDamage > 0 || len(v.PendingSupport) > 0 || len(v.PendingSupportOwners) > 0
		sort.Slice(v.Rows, func(a, b int) bool {
			if v.Rows[a].Adjusted != v.Rows[b].Adjusted {
				return v.Rows[a].Adjusted > v.Rows[b].Adjusted
			}
			return v.Rows[a].GUID < v.Rows[b].GUID
		})
		s.damageMeters[i] = v
		repaired = true
		s.prepareSummaryLocked(v)
		s.revision++
		s.debugMeter(v, "ownership_resolved", s.received, true)
	}
	if repaired {
		for _, v := range s.damageMeters {
			if v.Kind == "boss" && v.OutcomeKnown {
				s.prepareSummaryLocked(v)
			}
		}
	}
}

// Current boss or whole key, independent of Details' data and settings.
func (s *Store) DamageView(scope string) *DamageMeter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled || s.own == "" {
		return nil
	}
	if scope != "key" {
		if seg := s.active["boss"]; seg != nil {
			v := s.damageView(seg, s.last, true)
			return &v
		}
	}
	if seg := s.active["key"]; seg != nil {
		v := s.damageView(seg, s.last, true)
		return &v
	}
	for _, v := range s.damageMeters {
		if v.Owner == s.own && (scope != "key" || v.Kind == "key") {
			copy := v
			return &copy
		}
	}
	return nil
}
