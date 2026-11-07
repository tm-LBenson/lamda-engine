package augmentation

import (
	"fmt"
	"strings"
)

// Boss mechanics lead the pool; every candidate needs a recorded result.
// Fewer relevant results means fewer awards, never invented fillers.
const bossAwardLimit = 10

func bossAwardLines(m DamageMeter, pulls []DamageMeter) []string {
	good, banter := []string{}, []string{}
	add := func(pool *[]string, label, detail string, values map[string]int64) {
		name, n := awardNames(m.Rows, func(r DamageRow) int64 { return values[r.GUID] })
		if n > 0 {
			text := fmt.Sprintf(detail, n)
			if n == 1 {
				text = strings.NewReplacer("dispels.", "dispel.", "soaks.", "soak.", "interrupts.", "interrupt.", "resurrections.", "resurrection.", "deaths.", "death.", "first deaths across", "first death across", "puzzles with", "puzzle with").Replace(text)
			}
			*pool = append(*pool, fmt.Sprintf("%s: %s - %s", label, name, text))
		}
	}
	c := m.Utility
	mechanicGood, mechanicBanter, hasNamedSoaks, hasNamedAvoidable, hasNamedDispels, hasNamedInterrupts := detailedBossAwards(m)
	if encounterID(m) == "3497" && c != nil && c.FishKnown {
		add(&good, "Seeker", "%d fish handled. The guild's designated fish finder.", c.Fish)
	}
	good = append(good, mechanicGood...)
	if m.Valid && !m.Provisional {
		name, n := awardNames(m.Rows, func(r DamageRow) int64 { return r.Adjusted })
		if n > 0 {
			label := "Damage MVP"
			if m.Partial {
				label = "Recorded damage MVP"
			}
			good = append(good, fmt.Sprintf("%s: %s %s", label, name, compact(n)))
		}
	} else {
		good = append(good, "Damage award pending: incomplete log")
	}
	if c != nil && c.Known {
		if c.AwardVersion >= awardStatsVersion && c.HealingKnown {
			name, n := awardNames(m.Rows, func(r DamageRow) int64 { return c.Healing[r.GUID] })
			if n > 0 {
				good = append(good, fmt.Sprintf("Lifeguard: %s - %s effective healing (logged heals).", name, compact(n)))
			}
		}
		if !hasNamedDispels {
			add(&good, "Clean-up crew", "%d dispels. Someone brought the mop.", c.Dispels)
		}
		if c.MechanicsKnown && !hasNamedSoaks {
			add(&good, "Team Player", "%d good soaks. Actually understood the assignment.", c.Soaks)
		}
		if c.AwardVersion >= awardStatsVersion {
			if !hasNamedInterrupts {
				add(&good, "Quiet Please", "%d interrupts. That cast had enough screen time.", c.Interrupts)
			}
			add(&good, "Second Chance", "%d combat resurrections. Back to work.", c.Resurrections)
		}
		if len(c.Deaths) == 0 {
			line := "Deathless. Floors left uninspected!"
			if m.Success && len(pulls) == 1 && encounterID(m) == "3470" {
				line = "Souls Denied: one-shot, zero deaths. Nek'zali's collection stays empty."
			}
			good = append(good, line)
		}
		if encounterID(m) == "3445" && c.AwardVersion >= awardStatsVersion {
			add(&banter, "Bad Math", "%d Helical Toxins puzzles with a merge above four. Calculators confiscated.", c.MathFails)
		}
		banter = append(banter, mechanicBanter...)
		allSame := len(m.Rows) > 1
		var first int64 = -1
		for _, r := range m.Rows {
			n := c.Deaths[r.GUID]
			if first < 0 {
				first = n
			}
			if n == 0 || n != first {
				allSame = false
			}
		}
		if !allSame {
			add(&banter, deathAwardLabelFor(m), "%d deaths. Excellent floor coverage.", c.Deaths)
		}
		if c.MechanicsKnown && !hasNamedAvoidable {
			name, n := awardNames(m.Rows, func(r DamageRow) int64 { return c.Avoidable[r.GUID] })
			if n > 0 {
				banter = append(banter, fmt.Sprintf("Healer's Problem: %s - %s avoidable damage. The floor sends its regards.", name, compact(n)))
			}
		}
		if c.AwardVersion >= awardStatsVersion {
			add(&banter, "First to the Afterparty", "%d first deaths across these pulls. Saved everyone a seat.", c.FirstDeaths)
			if c.MechanicsKnown {
				many := map[string]int64{}
				for id, n := range c.AvoidableHits {
					if n >= 3 {
						many[id] = n
					}
				}
				add(&banter, "Repeat Customer", "%d avoidable damage hits/ticks. Loyalty card fully stamped.", many)
			}
			if c.HealingKnown && c.OverhealRoleVersion >= overhealRoleVersion {
				name, n := awardNames(m.Rows, func(r DamageRow) int64 {
					if c.HealingRoles[r.GUID] != "HEALER" {
						return 0
					}
					over := c.OverhealingOthers[r.GUID]
					if over >= 1000000 && over > c.HealingOthers[r.GUID] {
						return over
					}
					return 0
				})
				if n > 0 {
					banter = append(banter, fmt.Sprintf("Just In Case: %s - %s overhealing on others. Everyone was VERY topped.", name, compact(n)))
				}
			}
		}
	}
	if m.Success && m.Kind == "boss" {
		if len(pulls) == 1 {
			banter = append(banter, "First-Try Finish: one pull, one kill.")
		} else {
			banter = append(banter, fmt.Sprintf("First Try, Obviously: kill on pull %d. The others were rehearsals.", len(pulls)))
		}
	}
	if c == nil || !c.Known {
		good = append(good, "Utility awards unavailable for this saved report")
	}
	lines := []string{}
	for _, pool := range [][]string{good, banter} {
		for _, line := range pool[:min(bossAwardLimit, len(pool))] {
			lines = append(lines, "[LamdaMeters] "+line)
		}
	}
	// The transport uses ~ between independently bounded, complete chat lines.
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return lines
}
