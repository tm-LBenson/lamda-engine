package augmentation

import (
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

var namedMeterScope = regexp.MustCompile(`^(native:[1-9][0-9]{0,9}|recorded:[A-Za-z0-9_/:.-]{1,100})$`)

func validMeterScope(scope string) bool {
	return scope == "boss" || scope == "key" || scope == "overall" || scope == "previous" || namedMeterScope.MatchString(scope)
}

// Retain native sessions independently of boss/key boundaries, including trash.
// Compare without capture freshness so an unchanged idle frame cannot rewrite
// reports four times a second. Completed log matches can repair the stored split.
func (s *Store) rememberNative(v DamageMeter) {
	if v.Live || v.NativeSessionID == 0 || !v.Valid || v.RawTotal == 0 {
		return
	}
	for i, old := range s.nativeMeters {
		if old.Owner != v.Owner || old.NativeSessionID != v.NativeSessionID || old.Metric != v.Metric {
			continue
		}
		v.NativeSeenAt = old.NativeSeenAt
		if !reflect.DeepEqual(old, v) {
			s.nativeMeters[i] = v
			s.revision++
		}
		return
	}
	s.nativeMeters = append([]DamageMeter{v}, s.nativeMeters...)
	if len(s.nativeMeters) > 40 {
		s.nativeMeters = s.nativeMeters[:40]
	}
	s.revision++
}

func (s *Store) selectedMeter(scope string, live bool, now time.Time) (*DamageMeter, []engine.MeterChoice) {
	current := s.DamageView("boss")
	var native *DamageMeter
	if live {
		native = s.HybridView(now)
	}
	choices := []engine.MeterChoice{{Scope: "boss", Label: "Current"}, {Scope: "overall", Label: "Overall"}, {Scope: "previous", Label: "Previous"}, {Scope: "key", Label: "Whole key"}}
	s.mu.Lock()
	defer s.mu.Unlock()
	if native != nil && native.Owner == s.own && native.Valid {
		current = native
	}
	if h := s.hybrid; live && h != nil && h.uiFrame != nil && (h.frame == nil || h.frame.Metric != h.uiMetric) {
		// Public controls may switch before the first valid counters arrive.
		// Never put old damage totals beneath a newly selected Healing title.
		current = &DamageMeter{Metric: h.uiMetric, Owner: s.own, InputSource: "native", Name: "Current combat", NativeError: "native_feed_pending"}
	}
	metric := 0
	if current != nil {
		metric = current.Metric
	}
	var sessions []DamageMeter
	for _, old := range s.nativeMeters {
		if old.Owner != s.own || old.Metric != metric || old.RawTotal == 0 || (!live && old.Provisional) {
			continue
		}
		v := old
		// A later completed recorded encounter can replace a provisional saved
		// split only when every raw row and duration match exactly.
		for _, recorded := range s.damageMeters {
			if recorded.Owner != v.Owner || recorded.Metric != metric || !recorded.Valid || recorded.Live || len(recorded.Rows) != len(v.Rows) || absDuration(recorded.Seconds-v.Seconds) > 3 {
				continue
			}
			match := true
			for _, row := range v.Rows {
				found := false
				for _, r := range recorded.Rows {
					if r.GUID == row.GUID && r.Raw == row.Raw {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if match {
				v = recorded
				v.NativeSessionID = old.NativeSessionID
				v.Historical = old.Historical
				v.InputSource = "native+recorded"
				break
			}
		}
		sessions = append(sessions, v)
		label := v.Name
		if label == "Current combat" || label == "" {
			label = "Combat " + strconv.Itoa(v.NativeSessionID)
		}
		if !v.StartedAt.IsZero() {
			label = v.StartedAt.Local().Format("15:04") + " · " + label
		}
		choices = append(choices, engine.MeterChoice{Scope: "native:" + strconv.Itoa(v.NativeSessionID), Label: label})
	}
	if current != nil && current.NativeSessionID != 0 {
		filtered := sessions[:0]
		for _, v := range sessions {
			if v.NativeSessionID != current.NativeSessionID {
				filtered = append(filtered, v)
			}
		}
		sessions = filtered
	}
	for _, v := range s.damageMeters {
		if v.Owner == s.own && v.Metric == metric {
			choices = append(choices, engine.MeterChoice{Scope: "recorded:" + v.ID, Label: v.StartedAt.Local().Format("15:04") + " · " + v.Name})
		}
		if len(choices) >= 64 {
			break
		}
	}
	switch scope {
	case "boss":
		return current, choices
	case "key":
		if metric != 0 {
			return nil, choices
		}
		if seg := s.active["key"]; seg != nil {
			v := s.damageView(seg, s.last, true)
			return &v, choices
		}
		for _, v := range s.damageMeters {
			if v.Owner == s.own && v.Kind == "key" {
				copy := v
				return &copy, choices
			}
		}
	case "previous":
		if len(sessions) > 0 {
			return &sessions[0], choices
		}
		for _, v := range s.damageMeters {
			if v.Owner == s.own && v.Metric == metric && (current == nil || current.ID != v.ID) {
				copy := v
				return &copy, choices
			}
		}
	case "overall":
		// Explicit API history can overlap a whole key and its bosses. Keep
		// those browsable without adding them to automatically captured Overall.
		automatic := sessions[:0]
		for _, v := range sessions {
			if !v.Historical {
				automatic = append(automatic, v)
			}
		}
		sessions = automatic
		if current != nil && !current.Historical && current.NativeSessionID != 0 && (current.Live || current.RawTotal > 0) {
			sessions = append(sessions, *current)
		}
		if len(sessions) > 0 {
			return sumMeterSessions(s.own, nonoverlappingMeterSessions(sessions)), choices
		}
		// Recorded-only mode aggregates nonoverlapping completed encounters.
		// A whole key already contains its bosses and must not count them again.
		var recorded []DamageMeter
		for _, v := range s.damageMeters {
			if v.Owner != s.own || v.Metric != metric {
				continue
			}
			covered := false
			for _, key := range s.damageMeters {
				if key.Owner == s.own && key.Kind == "key" && v.Kind != "key" && !v.StartedAt.Before(key.StartedAt) && !v.EndedAt.After(key.EndedAt) {
					covered = true
					break
				}
			}
			if !covered {
				recorded = append(recorded, v)
			}
		}
		if len(recorded) > 0 {
			v := sumMeterSessions(s.own, recorded)
			v.Name = "Overall (recorded encounters)"
			v.InputSource = "recorded"
			return v, choices
		}
	default:
		if id, err := strconv.Atoi(strings.TrimPrefix(scope, "native:")); err == nil && strings.HasPrefix(scope, "native:") {
			if current != nil && current.NativeSessionID == id {
				return current, choices
			}
			for _, v := range sessions {
				if v.NativeSessionID == id {
					copy := v
					return &copy, choices
				}
			}
		}
		if strings.HasPrefix(scope, "recorded:") {
			for _, v := range s.damageMeters {
				if v.Owner == s.own && v.Metric == metric && "recorded:"+v.ID == scope {
					copy := v
					return &copy, choices
				}
			}
		}
	}
	return nil, choices
}

func absDuration(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Native history can retain the completed whole-key aggregate as well as its
// individual pulls. Once that aggregate is matched to a recorded key, its
// boundaries are known. Keep the pulls selectable but count their damage only
// once in Overall. Unknown historical timestamps are never guessed here.
func nonoverlappingMeterSessions(sessions []DamageMeter) []DamageMeter {
	result := make([]DamageMeter, 0, len(sessions))
	for i, inner := range sessions {
		covered := false
		for j, outer := range sessions {
			if i == j || outer.Kind != "key" || outer.Live || !outer.Valid || outer.Owner != inner.Owner || outer.Metric != inner.Metric || outer.StartedAt.IsZero() || outer.EndedAt.IsZero() || inner.StartedAt.IsZero() || inner.EndedAt.IsZero() {
				continue
			}
			if inner.StartedAt.Before(outer.StartedAt) || inner.EndedAt.After(outer.EndedAt) {
				continue
			}
			if inner.Kind == "key" && inner.StartedAt.Equal(outer.StartedAt) && inner.EndedAt.Equal(outer.EndedAt) && j > i {
				continue // equivalent duplicate keys retain exactly one stable copy
			}
			// Containment alone is insufficient for corrupt/mismatched history.
			// Every source must also fit inside the same owner's key counters.
			amounts := map[string]int64{}
			for _, row := range outer.Rows {
				amounts[row.GUID] += row.Raw
			}
			fits := inner.RawTotal <= outer.RawTotal
			for _, row := range inner.Rows {
				if row.Raw < 0 || row.Raw > amounts[row.GUID] {
					fits = false
					break
				}
			}
			if fits {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, inner)
		}
	}
	return result
}
func sumMeterSessions(owner string, sessions []DamageMeter) *DamageMeter {
	v := &DamageMeter{ID: "overall/" + owner, Name: "Overall (retained combat)", Kind: "overall", Owner: owner, Valid: true, InputSource: "native+recorded"}
	rows := map[string]*DamageRow{}
	for _, m := range sessions {
		v.Metric = m.Metric
		v.HasAug = v.HasAug || m.HasAug
		v.Seconds += m.Seconds
		v.RawTotal += m.RawTotal
		v.AdjustedTotal += m.AdjustedTotal
		v.RecordedSupport += m.RecordedSupport
		v.EstimatedSupport += m.EstimatedSupport
		v.Valid = v.Valid && m.Valid
		v.Provisional = v.Provisional || m.Provisional
		v.Live = v.Live || m.Live
		v.Partial = v.Partial || m.Partial
		v.CalibrationMissing = v.CalibrationMissing || m.CalibrationMissing
		if v.StartedAt.IsZero() || m.StartedAt.Before(v.StartedAt) {
			v.StartedAt = m.StartedAt
		}
		if m.EndedAt.After(v.EndedAt) {
			v.EndedAt = m.EndedAt
		}
		for _, r := range m.Rows {
			if rows[r.GUID] == nil {
				rows[r.GUID] = &DamageRow{GUID: r.GUID, Name: r.Name}
			}
			row := rows[r.GUID]
			row.Raw += r.Raw
			row.Adjusted += r.Adjusted
			row.Removed += r.Removed
			row.Added += r.Added
			row.Estimated += r.Estimated
		}
	}
	if v.Provisional {
		v.InputSource = "native+estimated"
	}
	for _, r := range rows {
		r.DPS = float64(r.Adjusted) / max(1, v.Seconds)
		v.Rows = append(v.Rows, *r)
	}
	sort.Slice(v.Rows, func(i, j int) bool {
		if v.Rows[i].Adjusted == v.Rows[j].Adjusted {
			return v.Rows[i].GUID < v.Rows[j].GUID
		}
		return v.Rows[i].Adjusted > v.Rows[j].Adjusted
	})
	return v
}
