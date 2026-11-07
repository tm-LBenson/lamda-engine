package augmentation

import (
	"strings"
	"time"
)

// Reviewed against the installed DBM-Raids-Midnight/TheVenomousAbyss
// definitions (revisions 20261023/20261026). Names match logged impact spells,
// not merely cast-warning IDs. Intentional Frost/Fire Patch clearing on Lost
// Explorers and the uncertain Toxic Incubation soak are deliberately excluded.
type bossMechanicRules struct {
	good, bad map[string]bool
	tankSoaks map[string]bool
}

func names(values ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range values {
		m[v] = true
	}
	return m
}

var bossMechanics = map[string]bossMechanicRules{
	"3470": {good: names("Hungering Pyre"), bad: names("1288554")},
	"3445": {good: names("Unstable Miasma", "Toxic Droplets"), bad: names("1284210")},
	"3421": {good: names("Ravenous Feast"), tankSoaks: names("Stone Breaker"), bad: names("1292552", "1309471", "1294605")},
	"3497": {good: names("Mighty Thud"), bad: names("1297650")},
	"3429": {good: names("Guillotine", "Grim Guillotine"), bad: names("1283290", "1285017", "1298591")},
	"3420": {good: names("Mutilate"), bad: names("1296667", "1305998", "1287083")},
	"3492": {good: names("Spectral Coils", "Serpent's Bite")},
	"3455": {good: names("Malignant Catalyst"), bad: names("1291461")},
}

func tankSpec(spec int) bool {
	switch spec {
	case 66, 73, 104, 250, 268, 581:
		return true
	}
	return false
}

func (s *Store) trackBossMechanics(seg *segment, f []string) {
	// The player-attributed successful use is distinct from the fish NPC's
	// spawn aura (1292490) and the duplicate boss aura. Verified in both real
	// Lost Explorers pulls on 2026-10-27; failures do not complete the mechanic.
	if seg.kind == "boss" && seg.id == "3497" && len(f) >= 12 && f[0] == "SPELL_CAST_SUCCESS" && f[9] == "1296535" && player(f[1]) && friendlyGroup(f[3]) {
		p := strings.Split(f[5], "-")
		if len(p) > 5 && (p[5] == "261835" || p[5] == "261843" || p[5] == "261848") {
			last := seg.damage.lastFish[f[1]]
			if last.IsZero() || s.last.Sub(last) >= 2*time.Second {
				seg.damage.fish[f[1]]++
				seg.damage.lastFish[f[1]] = s.last
			}
			seg.damage.players[f[1]] = true
			s.party[f[1]] = f[2]
		}
	}
	scope := mechanicScope(seg)
	if scope == "" || len(f) < 12 || !player(f[5]) || !friendlyGroup(f[7]) || friendlyGroup(f[3]) {
		return
	}
	rules, known := bossMechanics[scope]
	if !known {
		return
	}
	base, support, ok := damagePayload(f)
	if !ok || support {
		return
	}
	spell := f[10]
	good := rules.good[spell] || rules.tankSoaks[spell] && tankSpec(s.specs[f[5]])
	// Exact ground-effect IDs avoid conflating a required debuff with a
	// different damage effect that happens to have the same display name.
	bad := rules.bad[f[9]] && !rules.good[spell] && !rules.tankSoaks[spell]
	if !good && !bad {
		return
	}
	amount, reason := recordedDamageAmount(f, base)
	if reason != "" {
		if reason != "not_damage" {
			seg.damage.mechanicsInvalid = true
			if seg.damage.unknownMechanics == nil {
				seg.damage.unknownMechanics = map[string]bool{}
			}
			if good {
				seg.damage.unknownMechanics["soak/"+spell] = true
			}
			if bad {
				seg.damage.unknownMechanics["avoid/"+f[9]] = true
			}
			if seg.id == "3420" && spell == "Mutilate" {
				seg.damage.unknownMechanics["fail/double_mutilate"] = true
			}
			if s.debug != nil {
				s.debug.write(map[string]any{"kind": "mechanic_rejected", "boss": seg.id, "spell": f[9], "effect": spell, "reason": reason, "eventAt": s.last})
			}
		}
		return
	}
	seg.damage.players[f[5]] = true
	s.party[f[5]] = f[6]
	if good && f[0] != "SPELL_PERIODIC_DAMAGE" {
		if seg.id == "3421" && spell == "Ravenous Feast" && (seg.difficulty == "15" || seg.difficulty == "16") {
			feasted := seg.damage.lastSoak["aura/1310096/"+f[5]]
			if !feasted.IsZero() && s.last.Sub(feasted) > 250*time.Millisecond {
				key := "fail/feast/" + f[5]
				last := seg.damage.lastSoak[key]
				if last.IsZero() || s.last.Sub(last) >= time.Second {
					seg.damage.addMechanic("fail/repeat_feast", f[5], 1)
					seg.damage.lastSoak[key] = s.last
				}
				return
			}
		}
		key := f[5] + "/" + spell
		last := seg.damage.lastSoak[key]
		if last.IsZero() || s.last.Sub(last) >= 8*time.Second {
			priorGash := seg.damage.mutilated[f[5]]
			if seg.id == "3420" && spell == "Mutilate" && priorGash.Before(s.last.Add(-time.Second)) && !priorGash.IsZero() && (seg.difficulty == "15" || seg.difficulty == "16") {
				seg.damage.addMechanic("fail/double_mutilate", f[5], 1)
			} else {
				seg.damage.soaks[f[5]]++
				seg.damage.addMechanic("soak/"+spell, f[5], 1)
			}
			seg.damage.lastSoak[key] = s.last
		}
	}
	if bad {
		seg.damage.addMechanic("avoid/"+f[9], f[5], amount)
		seg.damage.avoidable[f[5]] += amount
		seg.damage.avoidableHits[f[5]]++
	}
}

func (s *Store) bossCountView(seg *segment, c *summaryCounts) {
	if seg.id == "3497" {
		c.FishKnown = true
		c.Fish = map[string]int64{}
		for id, n := range seg.damage.fish {
			c.Fish[id] = n
		}
	}
	_, c.MechanicsKnown = bossMechanics[mechanicScope(seg)]
	c.MechanicsKnown = c.MechanicsKnown && !seg.damage.mechanicsInvalid
	c.Soaks = map[string]int64{}
	c.Avoidable = map[string]int64{}
	for id, n := range seg.damage.soaks {
		c.Soaks[id] = n
	}
	for id, n := range seg.damage.avoidable {
		c.Avoidable[id] = n
	}
}
