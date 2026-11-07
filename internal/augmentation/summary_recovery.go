package augmentation

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

// A manual historical review can repair an earlier collector's lost boundary
// from the original continuous log. Replay is isolated: no live counters,
// identity, open segments or calibration are replaced.
func (s *Store) recoverSummary(ctx context.Context, path, scope, owner string, old *DamageMeter) (*DamageMeter, error) {
	if path == "" || (scope != "key" && scope != "boss") {
		return old, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return old, nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return old, nil
	}
	offset := max(int64(0), info.Size()-(4<<20))
	latest, err := latestSummaryEnd(ctx, io.NewSectionReader(f, offset, info.Size()-offset), scope, offset > 0)
	if err != nil {
		return nil, err
	}
	// The last key may precede more than four megabytes of later raid/trash
	// events. This fallback belongs to background repair, never the Share read.
	if latest.IsZero() && offset > 0 {
		latest, err = latestSummaryEnd(ctx, io.NewSectionReader(f, 0, info.Size()), scope, false)
		if err != nil {
			return nil, err
		}
	}
	// Upgrade earlier award counters once in background, including when a
	// different boss is now the latest. Share still only reads prepared text.
	awardBackfill := false
	if scope == "boss" && old != nil {
		s.mu.Lock()
		for _, m := range s.bossAttempts {
			if m.Owner == owner && m.BossVisit == old.BossVisit && (needsAwardCountUpgrade(m.Utility) || encounterID(m) == "3497" && !m.Utility.FishKnown) {
				awardBackfill = true
				break
			}
		}
		s.mu.Unlock()
	}
	if latest.IsZero() || old != nil && !latest.After(old.EndedAt) && (scope != "boss" || old.OutcomeKnown) && !awardBackfill {
		return old, nil
	}
	replay := New("")
	replay.SetEnabled(true)
	scan := bufio.NewScanner(io.NewSectionReader(f, 0, info.Size()))
	scan.Buffer(make([]byte, 64<<10), 2<<20)
	for scan.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		replay.Ingest(scan.Text(), latest)
	}
	if err = scan.Err(); err != nil {
		return nil, err
	}
	var found *DamageMeter
	for _, v := range replay.damageMeters {
		if v.Owner == owner && v.Kind == scope && v.EndedAt.Equal(latest) {
			copy := v
			found = &copy
			break
		}
	}
	if found == nil {
		// A historical preview of another character remains valid. Never borrow
		// the current log owner's report to replace its selected character.
		if replay.own != "" && replay.own != owner {
			return old, nil
		}
		return nil, fmt.Errorf("%w: the latest %s is missing its full start/end record", errSummaryPending, scope)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// An original continuous log can contain this player's character switch.
	// Enrich other owners only for exact attempts already retained locally;
	// never borrow another character's latest report or replace live identity.
	upgradeExisting := map[string]bool{}
	for _, m := range s.bossAttempts {
		if needsAwardCountUpgrade(m.Utility) {
			upgradeExisting[summaryKey(m)] = true
		}
	}
	awardCounts := map[string]*summaryCounts{}
	for _, attempt := range replay.bossAttempts {
		if attempt.Owner == owner || upgradeExisting[summaryKey(attempt)] {
			s.rememberBossAttemptLocked(attempt)
			if attempt.Utility != nil {
				awardCounts[summaryKey(attempt)] = attempt.Utility
			}
		}
	}
	// Keep the original damage/ownership history; enrich its mechanic counters
	// and refresh cached previews for older selected sessions as well.
	for i, m := range s.damageMeters {
		if counts := awardCounts[summaryKey(m)]; counts != nil {
			if m.Utility == nil {
				m.Utility = counts
			} else {
				m.Utility = upgradeAwardCounts(m.Utility, counts)
			}
			s.damageMeters[i] = m
			s.prepareSummaryLocked(m)
			s.revision++
		}
	}
	for i, v := range s.damageMeters {
		if v.ID == found.ID && v.Owner == owner {
			if scope == "boss" {
				s.damageMeters[i] = *found
				s.prepareSummaryLocked(*found)
				s.revision++
			}
			return found, nil
		}
	}
	s.damageMeters = append([]DamageMeter{*found}, s.damageMeters...)
	sort.SliceStable(s.damageMeters, func(i, j int) bool { return s.damageMeters[i].EndedAt.After(s.damageMeters[j].EndedAt) })
	if len(s.damageMeters) > 40 {
		s.damageMeters = s.damageMeters[:40]
	}
	for _, r := range replay.reports {
		if strings.TrimPrefix(r.ID, owner+"/") == found.ID && r.GUID == owner {
			s.reports = append([]Report{r}, s.reports...)
			break
		}
	}
	if len(s.reports) > 100 {
		s.reports = s.reports[:100]
	}
	s.prepareSummaryLocked(*found)
	s.revision++
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "history_recovered", "segment": found.ID, "owner": owner, "sourceFile": path})
	}
	return found, nil
}

func latestSummaryEnd(ctx context.Context, input io.Reader, scope string, skipFirst bool) (time.Time, error) {
	scan := bufio.NewScanner(input)
	scan.Buffer(make([]byte, 64<<10), 2<<20)
	if skipFirst {
		scan.Scan()
	}
	latest := time.Time{}
	for scan.Scan() {
		if ctx.Err() != nil {
			return time.Time{}, ctx.Err()
		}
		line := scan.Text()
		if !strings.Contains(line, "  CHALLENGE_MODE_END,") && !strings.Contains(line, "  ENCOUNTER_END,") {
			continue
		}
		at, fields, err := engine.RecordFields(line)
		if err != nil || len(fields) < 3 {
			continue
		}
		match := scope == "key" && fields[0] == "CHALLENGE_MODE_END" && fields[2] == "1" || scope == "boss" && fields[0] == "ENCOUNTER_END"
		if match && at.After(latest) {
			latest = at
		}
	}
	return latest, scan.Err()
}
