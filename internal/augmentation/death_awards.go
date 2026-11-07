package augmentation

import "hash/crc32"

// Stable for a saved encounter/review, varied across encounters. The title
// describes the death count only; it never invents a cause for those deaths.
var deathAwardTitles = []string{
	"Gravity Tester", "Afterlife VIP", "Professional Floor Hugger",
	"Respawn Rewards Member", "Ghost Mode Enthusiast", "Spirit Healer Pen Pal",
	"Corpse Run Captain", "Release Button Enthusiast", "Frequent Dier Miles",
	"Horizontal Gameplay", "Afterlife Season Pass", "Premium Spectator",
	"Nap Time Champion", "Spectral Correspondent", "Floor Sample Collector",
	"Floor Cover Consultant", "Ghost Tour Guide", "Resurrection Subscription",
}

var raidDeathAwardTitles = append(append([]string{}, deathAwardTitles...), "Raid Carpet Consultant", "Raid Nap Coordinator", "Boss Room Doormat", "Progression Tour Guide")
var dungeonDeathAwardTitles = append(append([]string{}, deathAwardTitles...), "Dungeon Carpet Consultant", "Dungeon Nap Coordinator", "Respawn Route Planner", "Dungeon Floor Auditor")
var keyDeathAwardTitles = append(append([]string{}, deathAwardTitles...), "Keystone Carpet Consultant", "Keystone Nap Coordinator", "Respawn Route Planner", "Dungeon Floor Auditor")

func deathAwardLabel(id string) string {
	return deathAwardTitles[crc32.ChecksumIEEE([]byte(id))%uint32(len(deathAwardTitles))]
}

// Classify the saved report, never the player's current group. Historical
// dungeon reports can be shared after zoning into a raid (and vice versa).
// Unknown context uses the neutral pool, which contains no raid/key claims.
func deathAwardLabelFor(m DamageMeter) string {
	pool := deathAwardTitles
	switch {
	case m.Kind == "key" || m.Difficulty == "8":
		pool = keyDeathAwardTitles
	case m.Difficulty == "1" || m.Difficulty == "2" || m.Difficulty == "23" || m.Difficulty == "24" || voidscarBoss(encounterID(m)) || templeBoss(encounterID(m)):
		pool = dungeonDeathAwardTitles
	case m.Difficulty == "3" || m.Difficulty == "4" || m.Difficulty == "5" || m.Difficulty == "6" || m.Difficulty == "7" || m.Difficulty == "9" || m.Difficulty == "14" || m.Difficulty == "15" || m.Difficulty == "16" || m.Difficulty == "17":
		pool = raidDeathAwardTitles
	}
	return pool[crc32.ChecksumIEEE([]byte(m.ID))%uint32(len(pool))]
}
