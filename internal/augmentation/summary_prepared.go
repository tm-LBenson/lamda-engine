package augmentation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type preparedSummary struct {
	variants [8][2]ChatSummary
}

func summaryKey(m DamageMeter) string { return m.Owner + "/" + m.ID }

func (s *Store) missingSummaryLocked(kind string, at time.Time) {
	if s.summaryMissing == nil {
		s.summaryMissing = map[string]time.Time{}
	}
	s.summaryMissing[kind] = at
	s.summaryRepairEpoch++
}

// Called when collection finishes, after persisted history loads, or when late
// ownership changes a completed meter. Share only reads the resulting text.
func (s *Store) prepareSummaryLocked(m DamageMeter) {
	// Older window history may predate outcome/visit fields even after its
	// exact attempt has been recovered in the progression archive. Restore
	// that verified record before preparing; never downgrade it with the old
	// window copy or leave a saved boss using single-pull legacy awards.
	if m.Kind == "boss" && !m.OutcomeKnown {
		for _, saved := range s.bossAttempts {
			if saved.OutcomeKnown && summaryKey(saved) == summaryKey(m) {
				m = saved
				for i, old := range s.damageMeters {
					if summaryKey(old) == summaryKey(m) {
						s.damageMeters[i] = m
						s.revision++
						break
					}
				}
				break
			}
		}
	}
	if m.Live || m.Owner == "" || len(m.Rows) == 0 {
		return
	}
	m.HasAug = s.meterHasAugLocked(m)
	key := summaryKey(m)
	s.rememberBossAttemptLocked(m)
	m, pulls := s.bossProgressionLocked(m)
	counts := summaryCounts{}
	if m.Utility != nil && m.Utility.Known {
		counts = *m.Utility
	} else if cached, ok := s.summaryUtilities[summaryKey(m)]; ok {
		counts = cached
	}
	if (!m.Valid || m.Provisional) && !counts.Known {
		delete(s.preparedSummaries, summaryKey(m))
		return
	}
	if m.AttributionMode != "all" || m.DamageBasis != "health+absorbs" {
		m.Partial = true
	}
	standard := makeChatSummary(m, counts)
	standard = progressionAwards(m, pulls, standard)
	uptime := standard
	uptime.Lines = append([]string{}, standard.Lines...)
	ids := map[string]bool{m.ID: true}
	for _, p := range pulls {
		ids[p.ID] = true
	}
	var ebon, seconds float64
	playerName := ""
	for _, r := range s.reports {
		if r.GUID == m.Owner && ids[strings.TrimPrefix(r.ID, m.Owner+"/")] && r.Seconds > 0 {
			ebon += r.EbonSeconds
			seconds += r.Seconds
			playerName = r.Player
		}
	}
	if seconds > 0 {
		line := fmt.Sprintf("[LamdaMeters] %s Ebon Might uptime: %.1f%% (recorded)", summaryText(strings.Split(playerName, "-")[0], 36), 100*ebon/seconds)
		uptime.Lines = append(uptime.Lines, line)
		uptime.Text = strings.Join(uptime.Lines, "~")
	}
	if s.preparedSummaries == nil {
		s.preparedSummaries = map[string]preparedSummary{}
	}
	prepared := preparedSummary{}
	for option := uint8(0); option < 8; option++ {
		prepared.variants[option][0] = summaryWithOptions(standard, option)
		prepared.variants[option][1] = summaryWithOptions(uptime, option)
	}
	s.preparedSummaries[key] = prepared
	s.archiveReportLocked(m, pulls, standard, uptime)
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "summary_prepared", "segment": m.ID, "owner": m.Owner, "lines": len(standard.Lines), "metricsKnown": counts.Known})
	}
	if len(s.preparedSummaries) > 80 {
		keep := map[string]preparedSummary{}
		for _, saved := range s.damageMeters {
			if v, ok := s.preparedSummaries[summaryKey(saved)]; ok {
				keep[summaryKey(saved)] = v
			}
		}
		s.preparedSummaries = keep
	}
}

func (s *Store) latestSummaryMeterLocked(scope, owner string) *DamageMeter {
	var latest *DamageMeter
	for _, meter := range s.damageMeters {
		if meter.Owner != owner || meter.Live || meter.Metric != 0 {
			continue
		}
		if !(scope == "key" && meter.Kind == "key" || scope == "boss" && meter.Kind == "boss" || scope == "recorded:"+meter.ID) {
			continue
		}
		if latest == nil || meter.EndedAt.After(latest.EndedAt) {
			copy := meter
			latest = &copy
		}
	}
	return latest
}

// No file reads, replay, award scans, formatting, timers, or chat sends occur
// here. A missing final log record remains pending rather than using an old key.
func (s *Store) PreparedSummaryForPlayer(scope, owner string, options ...uint8) (ChatSummary, error) {
	if !player(owner) {
		return ChatSummary{}, fmt.Errorf("The summary player is unavailable")
	}
	if strings.HasPrefix(scope, "history:") || strings.HasPrefix(scope, "saved:") || strings.HasPrefix(scope, "savedu:") {
		return s.reportHistorySummary(scope, owner, options)
	}
	if id, err := strconv.Atoi(strings.TrimPrefix(scope, "window:")); strings.HasPrefix(scope, "window:") && err == nil && id >= 1 && id <= 8 {
		if len(options) != 1 || options[0] != 4 {
			return ChatSummary{}, fmt.Errorf("selected windows share rankings only")
		}
		for _, window := range s.windowMeters(time.Now()) {
			if window.frame.WindowID != id {
				continue
			}
			return s.selectedWindowSummary(window, owner)
		}
		return ChatSummary{}, fmt.Errorf("The selected meter capture is unavailable")
	}
	withUptime := scope == "boss-uptime"
	if withUptime {
		scope = "boss"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	awardsOnly := len(options) == 1 && options[0]&1 != 0 && options[0]&4 == 0
	if !awardsOnly && (scope == "key" || scope == "boss") && s.active[scope] != nil {
		return ChatSummary{}, fmt.Errorf("%w: waiting for the %s's final log records", errSummaryPending, scope)
	}
	meter := s.latestSummaryMeterLocked(scope, owner)
	if meter == nil {
		return ChatSummary{}, fmt.Errorf("%w: no completed recorded %s is available", errSummaryPending, scope)
	}
	if missing := s.summaryMissing[meter.Kind]; missing.After(meter.EndedAt) {
		return ChatSummary{}, fmt.Errorf("%w: the latest %s needs its missing boundary records", errSummaryPending, meter.Kind)
	}
	prepared, ok := s.preparedSummaries[summaryKey(*meter)]
	if !ok {
		return ChatSummary{}, fmt.Errorf("%w: the latest report is incomplete", errSummaryPending)
	}
	option := uint8(0)
	if len(options) > 0 {
		option = options[0]
	}
	if option > 7 || option&5 == 5 {
		return ChatSummary{}, fmt.Errorf("invalid report options")
	}
	uptimeIndex := 0
	if withUptime {
		uptimeIndex = 1
	}
	result := prepared.variants[option][uptimeIndex]
	if option&4 != 0 && result.rankingRows == 0 {
		return ChatSummary{}, fmt.Errorf("%w: recorded meter rankings are not available", errSummaryPending)
	}
	result.Lines = append([]string{}, result.Lines...)
	return result, nil
}

// Startup repair and older utility counts are background work. Newly collected
// segments already prepare in finish; the Share request never invokes repair.
func (s *Store) RunSummaryPreparation(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	seen := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		_ = s.flushMeterDetails()
		_ = s.flushReportHistory()
		s.mu.Lock()
		path, owner := "", s.own
		if s.debug != nil {
			path = s.debug.SourceFile
		}
		if owner == "" && len(s.damageMeters) > 0 {
			owner = s.damageMeters[0].Owner
		}
		idle := s.enabled && len(s.active) == 0 && path != "" && player(owner)
		epoch := s.summaryRepairEpoch
		s.mu.Unlock()
		key := fmt.Sprintf("%s|%s|%d", path, owner, epoch)
		if !idle || seen[key] {
			continue
		}
		if len(seen) >= 32 {
			seen = map[string]bool{}
		}
		seen[key] = true
		s.prepareSummaryHistory(ctx, path, owner)
	}
}

func (s *Store) prepareSummaryHistory(ctx context.Context, path, owner string) {
	for _, scope := range []string{"key", "boss"} {
		if ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		meter := s.latestSummaryMeterLocked(scope, owner)
		s.mu.Unlock()
		meter, err := s.recoverSummary(ctx, path, scope, owner, meter)
		if err != nil || meter == nil {
			continue
		}
		counts := meter.Utility
		if counts == nil || !counts.Known {
			value := readSummaryCounts(ctx, summaryLogPath(path, *meter), *meter)
			if !value.Known {
				continue
			}
			counts = &value
		}
		s.mu.Lock()
		for i, existing := range s.damageMeters {
			if summaryKey(existing) == summaryKey(*meter) {
				existing.Utility = counts
				s.damageMeters[i] = existing
				if !s.summaryMissing[scope].After(existing.EndedAt) {
					delete(s.summaryMissing, scope)
				}
				s.prepareSummaryLocked(existing)
				s.revision++
				break
			}
		}
		s.mu.Unlock()
	}
}
