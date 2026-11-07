package augmentation

const overhealRoleVersion = 1

type healingTarget struct{ source, target string }
type healingAmounts struct{ effective, over int64 }

func healingAwardRole(spec int) string {
	if tankSpec(spec) {
		return "TANK"
	}
	switch spec {
	case 65, 105, 256, 257, 264, 270, 1468:
		return "HEALER"
	}
	return "OTHER_OR_UNKNOWN"
}

func needsAwardCountUpgrade(c *summaryCounts) bool {
	return c == nil || c.AwardVersion < awardStatsVersion || c.OverhealRoleVersion < overhealRoleVersion || c.EncounterAwardVersion < encounterAwardVersion
}

func (s *Store) healingRoleCountView(seg *segment, c *summaryCounts) {
	c.OverhealRoleVersion = overhealRoleVersion
	c.HealingRoles = map[string]string{}
	c.HealingOthers, c.OverhealingOthers = map[string]int64{}, map[string]int64{}
	// Only encounter COMBATANT_INFO establishes eligibility. A later respec,
	// current group role or healing amount cannot supply a past pull's role.
	for guid := range seg.damage.players {
		role := seg.damage.healingRoles[guid]
		if role == "" {
			role = "OTHER_OR_UNKNOWN"
		}
		c.HealingRoles[guid] = role
	}
	for target, n := range seg.damage.healTargets {
		owner := s.sourceOwner(target.source)
		if owner == "" || owner == target.target {
			continue
		}
		c.HealingOthers[owner] += n.effective
		c.OverhealingOthers[owner] += n.over
	}
}
