package augmentation

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func encounterID(m DamageMeter) string {
	p := strings.Split(m.ID, "/")
	if len(p) > 1 {
		return p[1]
	}
	return ""
}

// Separate retention keeps early wipes when the meter's forty-row history rolls.
func (s *Store) rememberBossAttemptLocked(m DamageMeter) {
	if m.Kind != "boss" || !m.OutcomeKnown || m.Live {
		return
	}
	for i, old := range s.bossAttempts {
		if summaryKey(old) == summaryKey(m) {
			m.Utility = upgradeAwardCounts(m.Utility, old.Utility)
			s.bossAttempts[i] = m
			return
		}
	}
	s.bossAttempts = append([]DamageMeter{m}, s.bossAttempts...)
	sort.SliceStable(s.bossAttempts, func(i, j int) bool { return s.bossAttempts[i].StartedAt.After(s.bossAttempts[j].StartedAt) })
	if len(s.bossAttempts) > 512 {
		s.bossAttempts = s.bossAttempts[:512]
	}
}

func (s *Store) bossProgressionLocked(last DamageMeter) (DamageMeter, []DamageMeter) {
	if last.Kind != "boss" || !last.OutcomeKnown || last.BossVisit == "" {
		return last, nil
	}
	pulls := []DamageMeter{}
	for _, p := range s.bossAttempts {
		if p.Owner != last.Owner || p.BossVisit != last.BossVisit || p.Difficulty != last.Difficulty || encounterID(p) != encounterID(last) || p.StartedAt.After(last.StartedAt) {
			continue
		}
		if p.ID != last.ID && p.Success {
			break
		}
		if last.EndedAt.Sub(p.StartedAt) > 12*time.Hour {
			break
		}
		pulls = append(pulls, p)
	}
	if len(pulls) == 0 {
		return last, nil
	}
	v := last
	v.Rows = nil
	v.RawTotal = 0
	v.AdjustedTotal = 0
	v.Seconds = 0
	counts := &summaryCounts{Known: true, FishKnown: true, Fish: map[string]int64{}, MechanicsKnown: true, Soaks: map[string]int64{}, Avoidable: map[string]int64{}, Dispels: map[string]int64{}, Deaths: map[string]int64{}}
	initAggregateAwardCounts(counts)
	rows := map[string]*DamageRow{}
	for _, p := range pulls {
		v.Seconds += p.Seconds
		v.RawTotal += p.RawTotal
		v.AdjustedTotal += p.AdjustedTotal
		v.Valid = v.Valid && p.Valid
		v.Partial = v.Partial || p.Partial
		v.Provisional = v.Provisional || p.Provisional
		v.HasAug = v.HasAug || s.meterHasAugLocked(p)
		if p.StartedAt.Before(v.StartedAt) {
			v.StartedAt = p.StartedAt
		}
		if p.Utility == nil || !p.Utility.Known {
			counts.Known = false
			counts.AwardVersion, counts.HealingKnown = 0, false
			counts.EncounterAwardVersion = 0
			counts.OverhealRoleVersion = 0
			counts.FishKnown = false
			counts.MechanicsKnown = false
		} else {
			addAwardCounts(counts, p.Utility)
			counts.FishKnown = counts.FishKnown && p.Utility.FishKnown
			for id, n := range p.Utility.Fish {
				counts.Fish[id] += n
			}
			counts.MechanicsKnown = counts.MechanicsKnown && p.Utility.MechanicsKnown
			for id, n := range p.Utility.Soaks {
				counts.Soaks[id] += n
			}
			for id, n := range p.Utility.Avoidable {
				counts.Avoidable[id] += n
			}
			for id, n := range p.Utility.Dispels {
				counts.Dispels[id] += n
			}
			for id, n := range p.Utility.Deaths {
				counts.Deaths[id] += n
			}
		}
		for _, r := range p.Rows {
			if rows[r.GUID] == nil {
				rows[r.GUID] = &DamageRow{GUID: r.GUID, Name: r.Name}
			}
			a := rows[r.GUID]
			a.Raw += r.Raw
			a.Removed += r.Removed
			a.Added += r.Added
			a.Adjusted += r.Adjusted
		}
	}
	for _, r := range rows {
		if v.Seconds > 0 {
			r.DPS = float64(r.Adjusted) / v.Seconds
		}
		v.Rows = append(v.Rows, *r)
	}
	v.Utility = counts
	return v, pulls
}

func progressionAwards(m DamageMeter, pulls []DamageMeter, result ChatSummary) ChatSummary {
	if len(pulls) == 0 && !hasDungeonAwards(m) {
		return result
	}
	const prefix = "[LamdaMeters] "
	if len(pulls) > 0 {
		state := "progress"
		if m.Success {
			state = "kill"
		}
		suffix := fmt.Sprintf(" · %s, %d %s", state, len(pulls), plural(int64(len(pulls)), "pull"))
		result.Lines[0] += suffix
		result.awardsHeader += suffix
	}
	// Keep the header and top-five rankings, then select the configured awards
	// of each kind from the full raid and all of this boss's attempts.
	keep := 1
	if m.Valid && !m.Provisional && m.Seconds > 0 {
		keep += min(5, len(m.Rows))
	}
	result.Lines = result.Lines[:min(keep, len(result.Lines))]
	result.Lines = append(result.Lines, bossAwardLines(m, pulls)...)
	result.Text = strings.Join(result.Lines, "~")
	return result
}

// Legacy histories can recover presence from an exact Aug report for this fight;
// current character specialization never relabels an unrelated historical fight.
func (s *Store) meterHasAugLocked(m DamageMeter) bool {
	if m.AugPresenceKnown {
		return m.HasAug
	}
	if m.HasAug {
		return true
	}
	for _, r := range s.reports {
		if strings.TrimPrefix(r.ID, r.GUID+"/") == m.ID {
			for id, n := range r.Casts {
				if id != 360827 && castSpells[id] && n > 0 {
					return true
				}
			}
		}
	}
	return false
}
