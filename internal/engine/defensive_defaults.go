package engine

import "slices"

// WoW writes old settings before loading new Lua on /reload. Mirror the addon's
// exact-starter migration in memory so one reload suffices. Never write the
// game's SavedVariables or change an empty/custom/already-versioned selection.
func upgradeDefensiveDefaults(ids []int, version string) []int {
	legacy := []int{48707, 51052, 49028, 48792, 49039, 55233, 198589, 196718, 204021, 187827,
		22812, 102342, 61336, 363916, 363534, 357170, 374227, 374348, 186265, 109304, 264735, 110959,
		45438, 55342, 115203, 116849, 122470, 31850, 1022, 6940, 498, 642, 86659, 633, 19236,
		47585, 47788, 33206, 62618, 31224, 5277, 108271, 98008, 108416, 104773, 118038, 184364, 97462, 871}
	withoutBlaze := append(append([]int{}, legacy[:17]...), legacy[18:]...)
	if version == "" && (slices.Equal(ids, legacy) || slices.Equal(ids, withoutBlaze)) {
		ids = append(append([]int{}, ids...), 2565, 12975, 23920, 202168, 190456)
	}
	current := append(append([]int{}, legacy...), 2565, 12975, 23920, 202168, 190456)
	currentWithoutBlaze := append(append([]int{}, withoutBlaze...), 2565, 12975, 23920, 202168, 190456)
	if slices.Equal(ids, current) || slices.Equal(ids, currentWithoutBlaze) {
		return append(append([]int{}, ids...), 342245)
	}
	return ids
}
