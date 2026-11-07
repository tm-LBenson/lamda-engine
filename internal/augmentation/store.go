// Package augmentation reviews execution without inventing full Aug attribution.
package augmentation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/atomicfile"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

const Note = "Buff uptime follows recorded aura intervals; missing application or removal records can distort it. The damage meter transfers all recorded damage-support hooks. Missing or invalid hooks can affect attribution; no Warcraft Logs percentile is calculated."

var castSpells = map[int]bool{395152: true, 409311: true, 360827: true, 403631: true, 395160: true, 396286: true}

type Report struct {
	ID                      string      `json:"id"`
	GUID                    string      `json:"guid"`
	Player                  string      `json:"player"`
	Name                    string      `json:"name"`
	Kind                    string      `json:"kind"`
	StartedAt               time.Time   `json:"startedAt"`
	EndedAt                 time.Time   `json:"endedAt"`
	Seconds                 float64     `json:"seconds"`
	Success                 bool        `json:"success"`
	Casts                   map[int]int `json:"casts"`
	Deaths                  int         `json:"deaths"`
	EbonSeconds             float64     `json:"ebonSeconds"`
	PrescienceTargetSeconds float64     `json:"prescienceTargetSeconds"`
	ScalesTargetSeconds     float64     `json:"scalesTargetSeconds"`
	RecordedSupportDamage   int64       `json:"recordedSupportDamage"`
	UnknownSupportEvents    int         `json:"unknownSupportEvents"`
	Note                    string      `json:"note"`
}

type ShareStatus struct {
	At    time.Time `json:"at"`
	Phase string    `json:"phase"`
	Token string    `json:"token,omitempty"`
	Error string    `json:"error,omitempty"`
}

type State struct {
	MeterDetailsError string          `json:"meterDetailsError,omitempty"`
	BossAttempts      []DamageMeter   `json:"bossAttempts,omitempty"`
	Share             *ShareStatus    `json:"share,omitempty"`
	ActiveSegments    []ActiveSegment `json:"activeSegments,omitempty"`
	NativeMeters      []DamageMeter   `json:"nativeMeters,omitempty"`
	Calibrations      []Calibration   `json:"calibrations,omitempty"`
	NativeClasses     map[string]int  `json:"nativeClasses,omitempty"`
	Hybrid            *HybridStatus   `json:"hybrid,omitempty"`
	LogInput          *LogProgress    `json:"logInput,omitempty"`
	Schema            int             `json:"schema"`
	Client            string          `json:"client"`
	Enabled           bool            `json:"enabled"`
	Status            string          `json:"status"`
	LastEventAt       time.Time       `json:"lastEventAt"`
	ReceivedAt        time.Time       `json:"receivedAt"`
	DelaySeconds      float64         `json:"delaySeconds"`
	Reports           []Report        `json:"reports"`
	Live              []Report        `json:"live"`
	DamageMeters      []DamageMeter   `json:"damageMeters"`
	Debug             DebugStatus     `json:"debug"`
}
type ActiveSegment struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
}

type auraKey struct {
	owner, target string
	spell         int
}
type segment struct {
	visit, difficulty string
	id, name, kind    string
	start             time.Time
	players           map[string]*Report
	auras             map[auraKey]time.Time
	invalid           bool
	damage            *damageSession
}

type Store struct {
	reportHistory                 []savedReport
	reportHistoryLimit            int
	historyRevision, historySaved uint64
	historyIO                     sync.Mutex
	nativeWindows                 map[int]*hybridState
	meterOverlayHidden            bool
	meterDetails                  map[string]meterDetailRecord
	detailRevision, detailSaved   uint64
	detailHits                    map[detailHitKey]detailHit
	detailIO                      sync.Mutex
	detailError                   string
	bossAttempts                  []DamageMeter
	bossVisit                     string
	preparedSummaries             map[string]preparedSummary
	summaryMissing                map[string]time.Time
	summaryRepairEpoch            uint64
	summaryUtilities              map[string]summaryCounts
	shareStatus                   *ShareStatus
	autoScopeToken                string
	meterImages                   map[string]string
	hybrid                        *hybridState
	logInput                      *LogProgress
	mu                            sync.Mutex
	path                          string
	enabled, supported            bool
	status                        string
	last, received                time.Time
	reports                       []Report
	active                        map[string]*segment
	auras                         map[auraKey]time.Time
	specs                         map[string]int
	names                         map[string]string
	zone                          string
	own                           string
	party                         map[string]string
	pets                          map[string]string
	damageMeters                  []DamageMeter
	nativeMeters                  []DamageMeter
	revision, saved               uint64
	debug                         *debugRecorder
}

func New(path string) *Store {
	s := &Store{path: path, status: "Collection is off", meterDetails: map[string]meterDetailRecord{}}
	if path != "" {
		s.debug = newDebug(filepath.Join(filepath.Dir(path), "debug", "ledger.jsonl"))
	}
	s.reset()
	if path != "" {
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			var state State
			if json.NewDecoder(io.LimitReader(f, 16<<20)).Decode(&state) == nil && state.Schema == 1 && state.Client == "retail" {
				for _, v := range state.NativeMeters {
					if len(s.nativeMeters) < 40 && player(v.Owner) && v.NativeSessionID > 0 && len(v.Rows) <= 40 && v.RawTotal >= 0 && v.RawTotal == v.AdjustedTotal && v.Seconds >= 0 && v.Seconds <= 604800 {
						s.nativeMeters = append(s.nativeMeters, v)
					}
				}
				if len(state.Calibrations) <= 4096 {
					for _, c := range state.Calibrations {
						if player(c.Owner) && player(c.GUID) && c.Mask >= 1 && c.Mask <= 15 && c.Raw >= 0 && c.Support >= 0 {
							value := c
							s.hybridState().calibrations[calibrationKey(c.Owner, c.GUID, c.Mask)] = &value
						}
					}
				}
				if len(state.NativeClasses) <= 512 {
					for guid, class := range state.NativeClasses {
						if player(guid) && class >= 1 && class <= 13 {
							s.hybridState().classes[guid] = class
						}
					}
				}
				s.damageMeters = state.DamageMeters
				if len(state.BossAttempts) <= 512 {
					s.bossAttempts = state.BossAttempts
				}
				if len(s.damageMeters) > 40 {
					s.damageMeters = s.damageMeters[:40]
				}
				for _, r := range state.Reports {
					if r.GUID != "" && r.Seconds > 0 {
						s.reports = append(s.reports, r)
					}
					if len(s.reports) == 100 {
						break
					}
				}
			}
		}
	}
	if s.path != "" && (s.hybrid == nil || len(s.hybrid.calibrations) == 0) {
		seedPath := filepath.Join(filepath.Dir(s.path), "calibration.json")
		if f, err := os.Open(seedPath); err == nil {
			var state State
			err = json.NewDecoder(io.LimitReader(f, 8<<20)).Decode(&state)
			f.Close()
			if err == nil && state.Schema == 1 && state.Client == "retail" && len(state.Calibrations) <= 4096 && len(state.NativeClasses) <= 512 {
				for _, c := range state.Calibrations {
					if player(c.Owner) && player(c.GUID) && c.Mask >= 1 && c.Mask <= 15 && c.Raw >= 0 && c.Support >= 0 {
						value := c
						s.hybridState().calibrations[calibrationKey(c.Owner, c.GUID, c.Mask)] = &value
					}
				}
				for guid, class := range state.NativeClasses {
					if player(guid) && class >= 1 && class <= 13 {
						s.hybridState().classes[guid] = class
					}
				}
			}
		}
	}
	s.loadReportHistory()
	// Backfill from already retained summaries, never by scanning combat logs.
	meters := append(append([]DamageMeter{}, s.bossAttempts...), s.damageMeters...)
	for _, meter := range meters {
		s.prepareSummaryLocked(meter)
	}
	s.loadMeterDetails()
	return s
}

func (s *Store) reset() {
	s.detailHits = nil
	if s.hybrid != nil {
		s.hybrid.events = nil
		s.hybrid.buffs = map[string]int{}
	}
	s.own = ""
	s.party = map[string]string{}
	s.pets = map[string]string{}
	s.active = map[string]*segment{}
	s.auras = map[auraKey]time.Time{}
	s.specs = map[string]int{}
	s.names = map[string]string{}
	s.zone = ""
	s.bossVisit = ""
}

func (s *Store) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled == enabled {
		return
	}
	s.enabled = enabled
	s.reset()
	s.supported = false
	if enabled {
		s.status = "Waiting for supported combat log metadata and a new boss/key"
	} else {
		s.status = "Collection is off"
	}
}

// ResetInput drops unfinished segments rather than labeling a mid-fight tail complete.
// Seed only the file header; never replay history into a live segment on attachment.
func (s *Store) ResetInput(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaryRepairEpoch++
	if s.debug != nil {
		s.debug.SourceFile = path
		s.debug.write(map[string]any{"kind": "source_reset", "discardedOpenSegments": len(s.active), "awaitingNewBoundary": true})
	}
	s.reset()
	s.supported = false
	s.last = time.Time{}
	s.received = time.Time{}
	s.status = "Waiting for a new boss/key; previous partial input discarded"
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scan := bufio.NewScanner(io.LimitReader(f, 8192))
	scan.Buffer(make([]byte, 8192), 8192)
	for scan.Scan() {
		_, fields, err := engine.RecordFields(scan.Text())
		if err == nil && len(fields) > 0 && fields[0] == "COMBAT_LOG_VERSION" {
			s.metadata(fields)
			return
		}
	}
}

func (s *Store) metadata(f []string) {
	supported := len(f) > 5 && f[1] == "22" && f[2] == "ADVANCED_LOG_ENABLED" && f[3] == "1" && f[4] == "BUILD_VERSION" && strings.HasPrefix(f[5], "12.1.")
	// /reload can append the same metadata to an uninterrupted log. It is
	// not a new file or encounter boundary. ResetInput still drops partial
	// segments when the actual source changes.
	if !s.supported || !supported {
		s.reset()
	}
	s.supported = supported
	if s.supported {
		s.status = "Waiting for a new boss/key"
	} else {
		s.status = "Unsupported log: requires Retail 12.1 advanced combat log version 22"
	}
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "log_metadata", "supported": s.supported})
	}
}

func (s *Store) begin(kind, id, name string, at time.Time) {
	seg := &segment{id: id, name: name, kind: kind, start: at, players: map[string]*Report{}, auras: map[auraKey]time.Time{}, damage: newDamageSession()}
	for key := range s.auras {
		seg.auras[key] = at
	}
	for key := range s.auras {
		s.player(seg, key.owner)
	}
	s.active[kind] = seg
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "segment_start", "id": fmt.Sprintf("%s/%s/%d", kind, id, at.UnixMilli()), "name": name, "segmentKind": kind, "eventAt": at, "receivedAt": s.received})
	}
	s.status = "Receiving combat log; damage attribution follows recorded hooks"
}

func (s *Store) player(seg *segment, guid string) *Report {
	if !strings.HasPrefix(guid, "Player-") || s.specs[guid] != 1473 {
		return nil
	}
	r := seg.players[guid]
	if r == nil {
		r = &Report{GUID: guid, Player: s.names[guid], Name: seg.name, Kind: seg.kind, StartedAt: seg.start, Casts: map[int]int{}, Note: Note}
		seg.players[guid] = r
	}
	return r
}

func addAura(r *Report, key auraKey, seconds float64) {
	if seconds <= 0 {
		return
	}
	switch {
	case key.spell == 395296 && key.owner == key.target:
		r.EbonSeconds += seconds
	case key.spell == 410089:
		r.PrescienceTargetSeconds += seconds
	case key.spell == 360827:
		r.ScalesTargetSeconds += seconds
	}
}

func (s *Store) copyReport(seg *segment, r *Report, at time.Time) Report {
	copy := *r
	copy.Casts = map[int]int{}
	for id, n := range r.Casts {
		copy.Casts[id] = n
	}
	copy.Player = s.names[r.GUID]
	copy.EndedAt = at
	copy.Seconds = at.Sub(seg.start).Seconds()
	for key, start := range seg.auras {
		if key.owner == r.GUID {
			addAura(&copy, key, at.Sub(start).Seconds())
		}
	}
	copy.EbonSeconds = min(copy.Seconds, copy.EbonSeconds)
	copy.ID = fmt.Sprintf("%s/%s/%s/%d", r.GUID, seg.kind, seg.id, seg.start.UnixMilli())
	return copy
}

func (s *Store) finish(kind, id string, at time.Time, success bool) {
	seg := s.active[kind]
	if seg == nil || (id != "" && seg.id != id) {
		s.missingSummaryLocked(kind, at)
		return
	}
	delete(s.active, kind)
	if seg.invalid || !at.After(seg.start) {
		s.missingSummaryLocked(kind, at)
		s.status = "Incomplete segment discarded"
		return
	}
	s.finishEncounterAwards(seg, at, success)
	s.finishVoidscarAwards(seg, at, success)
	s.finishTempleAwards(seg, at, success)
	view := s.damageView(seg, at, false)
	view.BossVisit, view.Difficulty = seg.visit, seg.difficulty
	view.OutcomeKnown, view.Success = true, success
	s.debugMeter(view, "segment_final", s.received, true)
	if s.own != "" {
		s.rememberMeterDetails(seg, view)
		s.damageMeters = append([]DamageMeter{view}, s.damageMeters...)
		if len(s.damageMeters) > 40 {
			s.damageMeters = s.damageMeters[:40]
		}
		s.revision++
	}
	guids := []string{}
	for guid := range seg.players {
		guids = append(guids, guid)
	}
	sort.Strings(guids)
	for _, guid := range guids {
		r := s.copyReport(seg, seg.players[guid], at)
		r.Success = success
		duplicate := false
		for _, old := range s.reports {
			if old.ID == r.ID {
				duplicate = true
				break
			}
		}
		if !duplicate {
			s.reports = append([]Report{r}, s.reports...)
			s.revision++
		}
	}
	if len(s.reports) > 100 {
		s.reports = s.reports[:100]
	}
	if s.own != "" {
		delete(s.summaryMissing, kind)
		s.prepareSummaryLocked(view)
	}
	s.status = "Completed log review; waiting for the next boss/key"
}

func (s *Store) Ingest(line string, received time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return
	}
	s.debugInput(line)
	at, f, err := engine.RecordFields(line)
	if err != nil || len(f) == 0 {
		if s.debug != nil {
			s.debug.write(map[string]any{"kind": "parse_error", "receivedAt": received, "normalizedLineSHA256": s.debug.lineHash})
		}
		for _, seg := range s.active {
			seg.invalid = true
		}
		return
	}
	// Reload headers use wall time and can precede an older buffered batch.
	// They describe the stream, not combat time: advancing last here would
	// misclassify that intact batch as out of order and erase its open key.
	if f[0] == "COMBAT_LOG_VERSION" {
		s.metadata(f)
		s.received = received
		return
	}
	if !s.last.IsZero() && at.Before(s.last.Add(-time.Second)) {
		s.reset()
		s.status = "Out-of-order log input; waiting for a new boss/key"
	}
	s.last = at
	s.received = received
	kind := f[0]
	if !s.supported {
		return
	}
	s.trackDamage(f)
	s.hybridAura(f)
	switch kind {
	case "ZONE_CHANGE":
		if len(f) > 3 {
			zone := f[1] + "/" + f[3]
			if s.zone != zone {
				own := s.own
				s.reset()
				s.own = own // completed reports remain identifiable after leaving a dungeon
				s.zone = zone
				s.bossVisit = fmt.Sprintf("%s/%d", zone, at.UnixMilli())
				s.status = "Zone changed; waiting for a new boss/key"
			}
		}
		return
	case "CHALLENGE_MODE_START":
		if len(f) > 4 {
			// Starting from EOF may miss the preceding ZONE_CHANGE. The key
			// boundary supplies its zone, so a later room change cannot erase it.
			s.zone = f[2] + "/8"
			s.bossVisit = fmt.Sprintf("%s/%d", s.zone, at.UnixMilli())
			s.begin("key", f[2], f[1]+" +"+f[4], at)
		}
		return
	case "CHALLENGE_MODE_END":
		if len(f) > 2 {
			if f[2] == "1" {
				s.finish("key", f[1], at, true)
			} else if seg := s.active["key"]; seg != nil && seg.id == f[1] {
				delete(s.active, "key")
			}
		}
		return
	case "ENCOUNTER_START":
		if len(f) > 4 {
			if len(f) > 5 {
				s.zone = f[5] + "/" + f[3]
			}
			s.begin("boss", f[1], f[2], at)
			seg := s.active["boss"]
			seg.difficulty = f[3]
			if s.bossVisit == "" {
				s.bossVisit = fmt.Sprintf("%s/%d", s.zone, at.UnixMilli())
				for _, old := range s.bossAttempts {
					if old.Owner == s.own && old.Difficulty == f[3] && strings.HasPrefix(old.BossVisit, s.zone+"/") && at.Sub(old.EndedAt) >= 0 && at.Sub(old.EndedAt) < 2*time.Hour {
						s.bossVisit = old.BossVisit
						break
					}
				}
			}
			seg.visit = s.bossVisit
		}
		return
	case "ENCOUNTER_END":
		if len(f) > 5 {
			s.finish("boss", f[1], at, f[5] == "1")
		}
		return
	case "COMBATANT_INFO":
		if len(f) > 25 {
			spec, _ := strconv.Atoi(f[25])
			s.specs[f[1]] = spec
			for _, seg := range s.active {
				if spec == 1473 && player(f[1]) {
					seg.damage.augSeen = true
				}
				if player(f[1]) {
					seg.damage.players[f[1]] = true
					if seg.damage.healingRoles == nil {
						seg.damage.healingRoles = map[string]string{}
					}
					role := healingAwardRole(spec)
					if old, ok := seg.damage.healingRoles[f[1]]; ok && old != role {
						role = "MIXED"
					}
					seg.damage.healingRoles[f[1]] = role
				}
				s.player(seg, f[1])
			}
		}
		return
	case "UNIT_DIED":
		if len(f) > 5 {
			for _, seg := range s.active {
				if r := s.player(seg, f[5]); r != nil {
					r.Deaths++
				}
			}
		}
		return
	}
	if len(f) < 12 || !strings.HasPrefix(kind, "SPELL_") {
		return
	}
	id, err := strconv.Atoi(f[9])
	if err != nil {
		return
	}
	owner, target := f[1], f[5]
	if strings.HasPrefix(owner, "Player-") {
		s.names[owner] = f[2]
	}
	// Bombardments also occurs for non-Aug Evokers. It never establishes spec.
	if kind == "SPELL_CAST_SUCCESS" && castSpells[id] {
		// Ebon Might, Prescience, Eruption, Upheaval and Eons identify Aug.
		if id != 360827 {
			s.specs[owner] = 1473
			for _, seg := range s.active {
				seg.damage.augSeen = seg.damage.augSeen || player(owner)
			}
		}
		for _, seg := range s.active {
			if r := s.player(seg, owner); r != nil {
				r.Casts[id]++
			}
		}
	}
	if (id == 395296 || id == 410089 || id == 360827) && strings.HasPrefix(kind, "SPELL_AURA_") {
		key := auraKey{owner: owner, target: target, spell: id}
		if id == 395296 || id == 410089 {
			s.specs[owner] = 1473
			for _, seg := range s.active {
				seg.damage.augSeen = seg.damage.augSeen || player(owner)
			}
		}
		switch kind {
		case "SPELL_AURA_APPLIED", "SPELL_AURA_REFRESH":
			if _, exists := s.auras[key]; !exists {
				s.auras[key] = at
			}
			for _, seg := range s.active {
				s.player(seg, owner)
				if _, exists := seg.auras[key]; !exists {
					seg.auras[key] = at
				}
			}
		case "SPELL_AURA_REMOVED":
			delete(s.auras, key)
			for _, seg := range s.active {
				if start, ok := seg.auras[key]; ok {
					if r := s.player(seg, owner); r != nil {
						addAura(r, key, at.Sub(start).Seconds())
					}
					delete(seg.auras, key)
				}
			}
		}
	}
	if kind == "SPELL_DAMAGE_SUPPORT" || kind == "SPELL_PERIODIC_DAMAGE_SUPPORT" {
		// Version 22 advanced SPELL payload: 12 common + 19 advanced fields,
		// then ten damage fields and the supporting player's GUID. Ordinary
		// SPELL_DAMAGE is deliberately ignored; the SUPPORT record mirrors it.
		if len(f) != 42 || !strings.HasPrefix(f[41], "Player-") {
			for _, seg := range s.active {
				seg.invalid = true
			}
			return
		}
		amount, e := strconv.ParseInt(f[31], 10, 64)
		overkill, oe := strconv.ParseInt(f[33], 10, 64)
		if e != nil || oe != nil || amount < 0 || amount > 1e12 {
			return
		}
		amount = max(int64(0), amount-max(int64(0), overkill))
		for _, seg := range s.active {
			if r := s.player(seg, f[41]); r != nil {
				if id == 434481 {
					r.RecordedSupportDamage += amount
				} else {
					r.UnknownSupportEvents++
				}
			}
		}
	}
}

func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := State{MeterDetailsError: s.detailError, Share: s.shareStatus, Schema: 1, Client: "retail", Enabled: s.enabled, Status: s.status, LastEventAt: s.last, ReceivedAt: s.received, Reports: append([]Report{}, s.reports...), Live: []Report{}}
	state.BossAttempts = append([]DamageMeter{}, s.bossAttempts...)
	if h := s.hybrid; h != nil {
		state.Hybrid = &HybridStatus{At: h.seen, Input: "native", Error: h.error}
		if h.frame != nil {
			state.Hybrid.Epoch = h.frame.ID
			state.Hybrid.SessionID = h.frame.SessionID
			state.Hybrid.Meter = s.hybridView(time.Now())
		}
	}
	if s.hybrid != nil {
		state.NativeClasses = map[string]int{}
		for guid, class := range s.hybrid.classes {
			state.NativeClasses[guid] = class
		}
	}
	state.Calibrations = s.calibrationSnapshot()
	if s.logInput != nil {
		value := *s.logInput
		state.LogInput = &value
	}
	if s.debug != nil {
		state.Debug = s.debug.DebugStatus
	}
	state.DamageMeters = append([]DamageMeter{}, s.damageMeters...)
	state.NativeMeters = append([]DamageMeter{}, s.nativeMeters...)
	for _, kind := range []string{"key", "boss"} {
		if seg := s.active[kind]; seg != nil {
			state.ActiveSegments = append(state.ActiveSegments, ActiveSegment{Kind: kind, Name: seg.name, StartedAt: seg.start})
			state.DamageMeters = append([]DamageMeter{s.damageView(seg, s.last, true)}, state.DamageMeters...)
		}
	}
	if !s.last.IsZero() {
		state.DelaySeconds = max(0, s.received.Sub(s.last).Seconds())
	}
	for _, seg := range s.active {
		for _, r := range seg.players {
			if !seg.invalid {
				state.Live = append(state.Live, s.copyReport(seg, r, s.last))
			}
		}
	}
	sort.Slice(state.Live, func(i, j int) bool { return state.Live[i].ID < state.Live[j].ID })
	return state
}

// Flush persists only completed reports and publishes only an explicitly marked
// module-owned results file. It cannot create an installed module or force reload.
func (s *Store) Flush(addons string) error {
	s.mu.Lock()
	if s.debug != nil {
		s.debug.flush()
	}
	rev := s.revision
	needed := rev != s.saved
	path := s.path
	enabled := s.enabled
	s.mu.Unlock()
	if !needed {
		return nil
	}
	state := s.Snapshot()
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := engine.WriteJSON(path, state); err != nil {
			return err
		}
	}
	if enabled && addons != "" {
		dir := filepath.Join(addons, "lamdaUI")
		toc, err := os.ReadFile(filepath.Join(dir, "lamdaUI.toc"))
		if err != nil {
			return err
		}
		if !strings.Contains(string(toc), "## X-Lamda-Augmentation-Client: retail") {
			return fmt.Errorf("Augmentation results target is not marked Retail")
		}
		// JSON strings are not Lua strings for control characters. Use a
		// serializer with decimal escapes and no executable user expressions.
		file, err := os.CreateTemp(dir, ".augmentation-*.tmp")
		if err != nil {
			return err
		}
		tmp := file.Name()
		defer os.Remove(tmp)
		_, writeErr := file.WriteString(luaResults(state.Reports))
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := atomicfile.Replace(tmp, filepath.Join(dir, "AugmentationResults.lua")); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.saved = rev
	s.mu.Unlock()
	return nil
}

func luaString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, v := range []byte(value) {
		if v == '"' || v == '\\' {
			b.WriteByte('\\')
			b.WriteByte(v)
		} else if v < 32 || v == 127 {
			fmt.Fprintf(&b, "\\%03d", v)
		} else {
			b.WriteByte(v)
		}
	}
	b.WriteByte('"')
	return b.String()
}
func luaResults(reports []Report) string {
	var b strings.Builder
	b.WriteString("-- Local Aug execution review; full support DPS unavailable.\nLamdaAugResults={schema=1,client=\"retail\",reports={\n")
	for _, r := range reports {
		fmt.Fprintf(&b, "{guid=%s,name=%s,endedAt=%s,seconds=%.3f,ebonSeconds=%.3f,prescienceTargetSeconds=%.3f,recordedSupportDamage=%d,note=%s},\n", luaString(r.GUID), luaString(r.Name), luaString(r.EndedAt.Format(time.RFC3339)), r.Seconds, r.EbonSeconds, r.PrescienceTargetSeconds, r.RecordedSupportDamage, luaString(r.Note))
	}
	b.WriteString("}}\n")
	return b.String()
}

// Share diagnostics describe the preview transfer, never a chat-post result.
func (s *Store) NoteShare(phase, token string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message := ""
	if err != nil {
		message = err.Error()
	}
	if old := s.shareStatus; old != nil && old.Phase == phase && old.Token == token && old.Error == message {
		return
	}
	v := &ShareStatus{At: time.Now(), Phase: phase, Token: token, Error: message}
	s.shareStatus = v
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "share_status", "share": v})
	}
}
