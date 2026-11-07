package simming

// Enrich older drop results from their original captured catalog without
// changing simulated values or rewriting history. Links always describe the
// captured drop; planned levels remain explicit separate metadata.
func upgradeDisplayResult(s *Snapshot, result *Result) *Result {
	if s == nil || s.Loot == nil || result == nil || len(result.Upgrades) == 0 {
		return result
	}
	profile, err := ParseProfile(s.Profile)
	if err != nil {
		return result
	}
	copy := *result
	copy.Upgrades = append([]Upgrade{}, result.Upgrades...)
	for index, original := range result.Upgrades {
		if original.Link != "" && original.Key != "" && len(original.Items) > 0 {
			continue
		}
		for _, pool := range s.Loot.Pools {
			if pool.SourceID() != original.SourceID {
				continue
			}
			for _, captured := range pool.Items {
				if captured.ID != original.ID {
					continue
				}
				variant := captured
				if original.Level > 0 {
					variant.Level = original.Level
				}
				for _, comparison := range lootComparisons(profile, variant, "display") {
					if len(comparison.Items) == 0 || comparison.Items[0].Slot != original.Slot {
						continue
					}
					metric := Metric{}
					decorateMetric(&metric, comparison)
					item := &copy.Upgrades[index]
					item.Link, item.Key, item.CapturedLevel = captured.Link, comparison.Items[0].Value, captured.Level
					item.Items = metric.Items
					break
				}
				break
			}
		}
	}
	return &copy
}
