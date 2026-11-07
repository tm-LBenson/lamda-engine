package augmentation

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type hybridEvent struct {
	at                time.Time
	source, owner     string
	amount            int64
	support, rejected bool
	mask              int
}
type hybridState struct {
	view              *lookup.NativeMeter
	viewAt, controlAt time.Time
	hidden            bool
	uiFrame           *lookup.NativeMeterFrame
	uiMetric          int
	uiAt              time.Time
	frame             *lookup.NativeMeter
	seen, start       time.Time
	events            []hybridEvent
	classes           map[string]int
	error             string
	buffs             map[string]int
	calibrations      map[string]*Calibration
	nativeBuckets     map[string]map[int]int64
	soloSession       string
	soloObserved      bool
}
type HybridStatus struct {
	Epoch     string       `json:"epoch"`
	At        time.Time    `json:"at"`
	SessionID int          `json:"sessionID"`
	Input     string       `json:"input"`
	Error     string       `json:"error,omitempty"`
	Meter     *DamageMeter `json:"meter,omitempty"`
}

func (s *Store) hybridState() *hybridState {
	if s.hybrid == nil {
		s.hybrid = &hybridState{classes: map[string]int{}, buffs: map[string]int{}, calibrations: map[string]*Calibration{}, nativeBuckets: map[string]map[int]int64{}}
	}
	return s.hybrid
}
func (s *Store) hybridInput(f []string, amount int64, support, rejected bool) {
	h := s.hybridState()
	owner := ""
	if support && len(f) > 0 {
		owner = f[len(f)-1]
	}
	guid := s.sourceOwner(f[1])
	mask := -1
	if guid != "" {
		mask = h.buffs[s.own+"|"+guid]
	}
	h.events = append(h.events, hybridEvent{at: s.last, source: strings.Clone(f[1]), owner: strings.Clone(owner), amount: amount, support: support, rejected: rejected, mask: mask})
	if !rejected && guid != "" {
		if support {
			s.learnCalibration(guid, owner, h.buffs[owner+"|"+guid], amount, true)
		} else {
			for key, active := range h.buffs {
				if active > 0 && strings.HasSuffix(key, "|"+guid) {
					caster := strings.TrimSuffix(key, "|"+guid)
					s.learnCalibration(guid, caster, active, amount, false)
				}
			}
		}
	}
	if len(h.events) > 100000 {
		clear(h.events[:10000])
		h.events = h.events[10000:]
	}
}

// ObserveNative accepts decoded pixels only. A missing, corrupt or regressing
// reading cannot replace the last good snapshot, nor become a zero counter.
func (s *Store) ObserveNative(n *lookup.NativeMeter, at time.Time, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observeNative(n, at, reason)
}
func (s *Store) observeNative(n *lookup.NativeMeter, at time.Time, reason string) {
	if !s.enabled {
		return
	}
	h := s.hybridState()
	if n != nil && n.ControlOnly && reason == "" {
		h.hidden = n.ControlHidden
		h.error = "native_meter_capture_paused"
		return // Sharing metadata never replaces damage, owner or session clocks.
	}
	if reason == "native_meter_inactive_spec" {
		h.hidden = true
	} else if reason == "native_meter_capture_paused" || n != nil && n.ID != "" && reason == "" {
		h.hidden = false
	}
	if n != nil && n.Frame != nil && n.Border != nil && n.ID != "" && reason == "" {
		value := *n.Frame
		if value.Menu != nil {
			menu := *value.Menu
			value.Menu = &menu
		}
		h.uiFrame = &value
		h.uiMetric = n.Metric
		h.uiAt = at
	}
	if n == nil || !n.Captured || reason != "" {
		h.error = reason
		if h.error == "" && n != nil {
			h.error = n.Error
		}
		if h.error == "" {
			h.error = "Native meter unavailable"
		}
		return
	}
	rootEpoch := func(id string) string {
		i := strings.LastIndexByte(id, ':')
		if i < 0 {
			return id
		}
		return id[:i]
	}
	soloKey := fmt.Sprintf("%s/%s/%d", rootEpoch(n.ID), n.Player, n.SessionID)
	current := n.Frame == nil || n.Frame.Scope == "boss"
	if h.soloSession != soloKey {
		h.soloSession, h.soloObserved = soloKey, false
	}
	if !current || len(n.Members) != 1 {
		h.soloObserved = false
	} else if n.Combat && n.DurationMillis <= 1000 {
		h.soloObserved = true
	}
	if h.frame != nil && rootEpoch(h.frame.ID) == rootEpoch(n.ID) && n.Serial <= h.frame.Serial && !(h.frame.Serial > 16777183 && n.Serial < 32) {
		return
	}
	if h.frame != nil && rootEpoch(h.frame.ID) == rootEpoch(n.ID) && h.frame.SessionID == n.SessionID && h.frame.Player == n.Player && h.frame.Metric == n.Metric && ((h.frame.Frame == nil && n.Frame == nil) || (h.frame.Frame != nil && n.Frame != nil && h.frame.Frame.Scope == n.Frame.Scope)) {
		old := map[string]int64{}
		for _, m := range h.frame.Members {
			old[m.GUID] = m.Total
		}
		for _, m := range n.Members {
			if m.Total < old[m.GUID] {
				h.error = "native_counter_regressed"
				return
			}
		}
	} else {
		if h.frame != nil && (h.frame.SessionID != n.SessionID || h.frame.Player != n.Player) {
			if previous := s.hybridView(h.seen); previous != nil {
				previous.Live = false
				if previous.EndedAt.IsZero() && !previous.Historical {
					previous.EndedAt = h.start.Add(time.Duration(h.frame.DurationMillis) * time.Millisecond)
				}
				s.rememberNative(*previous)
			}
		}
		h.start = at.Add(-time.Duration(n.DurationMillis) * time.Millisecond)
		h.nativeBuckets = map[string]map[int]int64{}
	}
	previous := map[string]lookup.NativeMeterMember{}
	if h.frame != nil && rootEpoch(h.frame.ID) == rootEpoch(n.ID) && h.frame.SessionID == n.SessionID && h.frame.Player == n.Player && h.frame.Metric == n.Metric && ((h.frame.Frame == nil && n.Frame == nil) || (h.frame.Frame != nil && n.Frame != nil && h.frame.Frame.Scope == n.Frame.Scope)) {
		for _, m := range h.frame.Members {
			previous[m.GUID] = m
		}
	}
	for _, m := range n.Members {
		old, exists := previous[m.GUID]
		mask := m.AuraMask
		if m.AuraMode != 15 {
			mask = -1
		}
		if exists && mask == 0 && old.AuraMode == 15 && old.AuraMask > 0 {
			mask = old.AuraMask
		}
		if !exists && (!n.Combat || n.DurationMillis > 1000) {
			mask = -1
		}
		if h.nativeBuckets[m.GUID] == nil {
			h.nativeBuckets[m.GUID] = map[int]int64{}
		}
		h.nativeBuckets[m.GUID][mask] += max(int64(0), m.Total-old.Total)
	}
	copyFrame := *n
	copyFrame.Members = append([]lookup.NativeMeterMember(nil), n.Members...)
	if n.Frame != nil {
		v := *n.Frame
		if v.Menu != nil {
			menu := *v.Menu
			v.Menu = &menu
		}
		copyFrame.Frame = &v
	}
	if n.Border != nil {
		v := *n.Border
		copyFrame.Border = &v
	}
	if n.Auto != nil {
		a := *n.Auto
		copyFrame.Auto = &a
	}
	s.syncAutoScope(n)
	h.frame = &copyFrame
	h.seen = at
	h.error = ""
	if n.Combat {
		candidate := at.Add(-time.Duration(n.DurationMillis) * time.Millisecond)
		if h.start.IsZero() || candidate.Before(h.start) {
			h.start = candidate
		}
	}
	for _, m := range n.Members {
		if m.Class > 0 && (len(h.classes) < 512 || h.classes[m.GUID] > 0) {
			h.classes[m.GUID] = m.Class
		}
	}
	s.own = n.Player
	if n.Spec != 0 && (n.Frame == nil || !strings.HasPrefix(n.Frame.Scope, "native:")) {
		s.specs[n.Player] = n.Spec
	}
	if !n.Combat {
		if completed := s.hybridView(at); completed != nil {
			s.rememberNative(*completed)
		}
	}
	if s.debug != nil && s.debug.Enabled {
		s.debug.write(map[string]any{"kind": "native_meter_input", "receivedAt": at, "native": copyFrame})
	}
}
func (s *Store) HybridView(now time.Time) *DamageMeter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hybridView(now)
}

func (s *Store) nativeAugEligible() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hybrid == nil || !s.hybrid.hidden
}
func (s *Store) nativeFrame(now time.Time) *lookup.NativeMeterFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.hybrid
	if h == nil || h.uiFrame == nil || h.frame == nil || h.frame.Player != s.own || h.error == "native_meter_capture_paused" {
		return nil
	}
	v := *h.uiFrame
	if v.Menu != nil {
		menu := *v.Menu
		v.Menu = &menu
	}
	return &v
}
func nativeHistorical(n *lookup.NativeMeter) bool {
	return n != nil && n.Frame != nil && (n.Frame.Scope == fmt.Sprintf("native:%d", n.SessionID) || n.Frame.Scope == "previous")
}
func (s *Store) hybridView(now time.Time) *DamageMeter {
	h := s.hybrid
	if h == nil || h.frame == nil {
		return nil
	}
	n := h.frame
	if n.Metric == 1 {
		copy := *n
		copy.Metric = 0
		h.frame = &copy
		v := s.hybridView(now)
		h.frame = n
		if v != nil {
			v.Metric = 1
		}
		return v
	}
	historical := nativeHistorical(n)
	if n.Metric != 0 {
		return nativePlainMeter(n, h.start, h.seen, now)
	}
	// A fully matched completed encounter is the strongest available scope
	// binding, including a cached native fight received after Engine restart.
	if !n.Combat || historical {
		var matched *DamageMeter
		legacyCompatible := n.Spec == 1473
		for _, member := range n.Members {
			if member.GUID != n.Player && (member.Class == 0 || member.Class == 13) {
				legacyCompatible = false
			}
		}
		for _, m := range s.damageMeters {
			clockMatches := math.Abs(m.Seconds-float64(n.DurationMillis)/1000) <= 3
			// Native key timers exclude periods present in the wall-clock log.
			// Bind only a unique, nonempty complete roster with exact raw totals.
			if m.Kind == "key" && m.RawTotal > 0 && (historical || n.Frame != nil && n.Frame.Scope == "overall") {
				clockMatches = true
			}
			if m.Owner != n.Player || m.Metric != 0 || (m.AttributionMode != "all" && !legacyCompatible) || m.Live || !m.Valid || len(m.Rows) != len(n.Members) || !clockMatches {
				continue
			}
			amounts := map[string]int64{}
			for _, r := range m.Rows {
				amounts[r.GUID] = r.Raw
			}
			good := true
			for _, r := range n.Members {
				value, ok := amounts[r.GUID]
				if !ok || value != r.Total {
					good = false
					break
				}
			}
			if good {
				if matched != nil {
					matched = nil
					break
				}
				copyMeter := m
				copyMeter.Rows = append([]DamageRow(nil), m.Rows...)
				matched = &copyMeter
			}
		}
		if matched != nil {
			matched.Historical = historical
			matched.DamageBasis = "health+absorbs"
			matched.Seconds = float64(n.DurationMillis) / 1000
			for i := range matched.Rows {
				matched.Rows[i].DPS = float64(matched.Rows[i].Adjusted) / max(1, matched.Seconds)
			}
			matched.InputSource = "native+recorded"
			matched.NativeSessionID = n.SessionID
			matched.NativeSeenAt = h.seen
			if now.Sub(h.seen) > 1500*time.Millisecond {
				matched.Valid = false
				matched.NativeError = "native_feed_stale"
			}
			return matched
		}
	}
	v := &DamageMeter{ID: fmt.Sprintf("native/%s/%d", n.Player, n.SessionID), Kind: "combat", Name: "Current combat", Owner: n.Player,
		DamageBasis: "health+absorbs",
		StartedAt:   h.start, Seconds: float64(n.DurationMillis) / 1000, Live: n.Combat && !historical, Valid: true, Provisional: true,
		InputSource: "native+estimated", NativeSessionID: n.SessionID, NativeSeenAt: h.seen, Rows: []DamageRow{}}
	if !n.Combat {
		v.EndedAt = h.start.Add(time.Duration(n.DurationMillis) * time.Millisecond)
	}
	if historical {
		v.Historical = true
		v.Name = fmt.Sprintf("Combat %d", n.SessionID)
		v.StartedAt, v.EndedAt = time.Time{}, time.Time{}
	}
	if now.Sub(h.seen) > 1500*time.Millisecond {
		v.Valid = false
		v.NativeError = "native_feed_stale"
	}
	raw := map[string]int64{}
	known := map[transferKey]int64{}
	available := map[string]bool{}
	// A fully reconciled solo roster has nobody to transfer support to or
	// receive it from. Its native total is already the adjusted total, even if
	// optional recorded proc data has not arrived.
	maySupport, foreignSupport := n.Spec == 1473 && !h.soloObserved, false
	for _, m := range n.Members {
		available[m.GUID] = true
		if historical && m.Class == 13 {
			maySupport = true // today's specialization cannot identify an old pull
		}
		if !historical && (m.GUID == n.Player && n.Spec == 1473 || m.GUID != n.Player && s.specs[m.GUID] == 1473) {
			v.HasAug = true
		}
		if m.GUID == n.Player && m.Class == 13 && n.Spec == 0 {
			maySupport = true
			v.CalibrationMissing = true
		}
		if m.GUID != n.Player && (s.specs[m.GUID] == 1473 || (m.Class == 13 && s.specs[m.GUID] == 0)) {
			maySupport = true
			foreignSupport = true
		}
	}
	loggedBuckets := map[string]map[int]int64{}
	end := h.start.Add(time.Duration(n.DurationMillis) * time.Millisecond)
	for _, e := range h.events {
		if historical {
			break
		}
		if e.at.Before(h.start.Add(-300*time.Millisecond)) || e.at.After(end.Add(200*time.Millisecond)) {
			continue
		}
		if e.rejected {
			v.Partial = true
			v.RejectedHooks++
			continue
		}
		guid := s.sourceOwner(e.source)
		if guid == "" {
			continue
		}
		if !e.support {
			raw[guid] += e.amount
			if loggedBuckets[guid] == nil {
				loggedBuckets[guid] = map[int]int64{}
			}
			loggedBuckets[guid][e.mask] += e.amount
		} else if e.owner != guid && player(e.owner) {
			known[transferKey{source: guid, owner: e.owner}] += e.amount
			maySupport = true
			if e.owner != n.Player {
				foreignSupport = true
			}
		}
	}
	removed, added := map[string]int64{}, map[string]int64{}
	nativeRaw := map[string]int64{}
	for _, m := range n.Members {
		nativeRaw[m.GUID] = m.Total
	}
	sums := map[string]int64{}
	for key, amount := range known {
		if available[key.owner] && available[key.source] {
			sums[key.source] += amount
		} else {
			v.CalibrationMissing = true
		}
	}
	for key, amount := range known {
		if available[key.owner] && available[key.source] && sums[key.source] <= nativeRaw[key.source] {
			removed[key.source] += amount
			added[key.owner] += amount
			v.RecordedSupport += amount
		} else if available[key.source] && sums[key.source] > nativeRaw[key.source] {
			v.Partial = true
		}
	}
	allMatched := true
	for _, m := range n.Members {
		r := DamageRow{GUID: m.GUID, Name: m.Name, Raw: m.Total, Removed: removed[m.GUID], Added: added[m.GUID]}
		if raw[m.GUID] != m.Total {
			allMatched = false
		}
		pending := max(int64(0), m.Total-raw[m.GUID])
		if foreignSupport && pending > 0 {
			v.CalibrationMissing = true
		}
		if n.Spec == 1473 && m.GUID != n.Player && !historical {
			remaining := pending
			estimate := int64(0)
			for mask := 1; mask <= 15; mask++ {
				knownRaw := int64(0)
				if loggedBuckets[m.GUID] != nil {
					knownRaw = loggedBuckets[m.GUID][mask]
				}
				wanted := max(int64(0), h.nativeBuckets[m.GUID][mask]-knownRaw)
				count := min(remaining, wanted)
				if count <= 0 {
					continue
				}
				remaining -= count
				rate, calibrated := s.calibrationRate(n.Player, m.GUID, m.Class, mask)
				if !calibrated {
					v.CalibrationMissing = true
					continue
				}
				estimate += int64(math.Round(float64(count) * max(0, min(.95, rate))))
			}
			if remaining > 0 && h.nativeBuckets[m.GUID][-1] > 0 {
				v.CalibrationMissing = true
			}
			estimate = min(max(int64(0), m.Total-r.Removed), estimate)
			r.Removed += estimate
			r.Estimated = estimate
			added[n.Player] += estimate
			v.EstimatedSupport += estimate
		}
		v.RawTotal += m.Total
		v.Rows = append(v.Rows, r)
	}
	for i := range v.Rows {
		r := &v.Rows[i]
		r.Added = added[r.GUID]
		if r.GUID == n.Player {
			r.Estimated += v.EstimatedSupport
		}
		r.Adjusted = r.Raw - r.Removed + r.Added
		r.DPS = float64(r.Adjusted) / max(1, v.Seconds)
		v.AdjustedTotal += r.Adjusted
	}
	v.AttributionMode = "all"
	if !maySupport {
		v.Provisional = false
		v.InputSource = "native"
	}
	if allMatched && !n.Combat && !s.last.Before(end.Add(time.Second)) && !v.Partial {
		v.Provisional = false
		v.InputSource = "native+recorded"
		v.CollectionValid = true
	}
	sort.Slice(v.Rows, func(i, j int) bool {
		if v.Rows[i].Adjusted == v.Rows[j].Adjusted {
			return v.Rows[i].GUID < v.Rows[j].GUID
		}
		return v.Rows[i].Adjusted > v.Rows[j].Adjusted
	})
	return v
}
