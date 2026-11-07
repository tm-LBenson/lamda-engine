package augmentation

import (
	"fmt"
	"strings"
	"testing"
)

func TestDeathAwardsFollowSavedContentAndNeverLeakRaidTitlesIntoDungeon(t *testing.T) {
	for _, tc := range []struct{ kind, difficulty, id, forbidden string }{{"key", "", "2923", "Raid"}, {"boss", "8", "3285", "Raid"}, {"boss", "23", "123", "Raid"}, {"boss", "2", "123", "Keystone"}, {"boss", "15", "3470", "Dungeon"}, {"native", "", "123", "Raid"}} {
		seen := map[string]bool{}
		for i := 0; i < 1000; i++ {
			m := DamageMeter{Kind: tc.kind, Difficulty: tc.difficulty, ID: fmt.Sprintf("%s/%s/%d", tc.kind, tc.id, i)}
			label := deathAwardLabelFor(m)
			seen[label] = true
			if strings.Contains(label, tc.forbidden) {
				t.Fatal("wrong content title", m, label)
			}
			if label != deathAwardLabelFor(m) {
				t.Fatal("refresh changed title")
			}
		}
		if tc.difficulty == "15" && !seen["Raid Carpet Consultant"] {
			t.Fatal("raid-only title unavailable on raid")
		}
		if tc.kind == "key" && !seen["Keystone Carpet Consultant"] {
			t.Fatal("key title unavailable on key")
		}
	}
}
