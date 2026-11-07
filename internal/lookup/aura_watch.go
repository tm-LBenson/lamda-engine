package lookup

import (
	"fmt"
	"strconv"
	"strings"
)

// Fixed personal-buff slots, mirrored by Observations.lua and checked by its
// cross-language fixture. No externals, group buffs, instant heals or Meta:
// those do not identify the caster (or can come from a different cooldown).
var AuraWatchSpells = map[int][]int{
	1: {871, 12975, 118038, 184364, 23920},
	2: {498, 642, 31850}, 3: {186265, 264735}, 4: {31224, 5277},
	5: {19236, 47585}, 6: {48707, 48792, 49028, 49039, 55233},
	7: {108271}, 8: {45438, 110959, 55342, 414658, 342245}, 9: {108416, 104773},
	10: {122470}, 11: {22812, 61336}, 12: {198589}, 13: {363916},
}

type AuraWatchMember struct {
	Slot, Member, Class, Mask int
	GUID                      string
}
type AuraWatch struct {
	ID       string
	Offset   int
	Members  []AuraWatchMember
	Active   [40]bool
	Captured bool
}

func parseAuraWatch(p []string) (*AuraWatch, error) {
	bad := fmt.Errorf("aura_watch_invalid")
	if len(p) != 5 || p[1] != "1" || !observationID.MatchString(p[2]) {
		return nil, bad
	}
	offset, err := strconv.Atoi(p[3])
	if err != nil || (offset != 16 && offset != -8) {
		return nil, bad
	}
	a := &AuraWatch{ID: p[2], Offset: offset}
	seenSlots, seenMembers := map[int]bool{}, map[int]bool{}
	for _, member := range strings.Split(p[4], ";") {
		f := strings.Split(member, ",")
		if len(f) != 4 {
			return nil, bad
		}
		slot, e1 := strconv.Atoi(f[0])
		class, e2 := strconv.Atoi(f[2])
		mask, e3 := strconv.Atoi(f[3])
		member, e4 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || member < 1 || member > 5 || slot < 0 || slot >= 5 || len(AuraWatchSpells[class]) == 0 || mask < 1 || mask >= (1<<len(AuraWatchSpells[class])) || seenSlots[slot] || seenMembers[member] {
			return nil, bad
		}
		seenSlots[slot], seenMembers[member] = true, true
		a.Members = append(a.Members, AuraWatchMember{Slot: slot, Class: class, Mask: mask, Member: member})
	}
	return a, nil
}

// Each 4x4 tile has four 2x2 color cells. Three identify its slot; the first
// is the native aura button's on/off color. Reject occlusion or color changes.
func AuraWatchColor(slot, cell int, active bool) [3]byte {
	if cell == 0 {
		if active {
			return [3]byte{24, 224, 96}
		}
		return [3]byte{160, 32, 96}
	}
	return [3]byte{byte(24 + (slot*37+cell*53)%208), byte(24 + (slot*61+cell*29)%208), byte(24 + (slot*43+cell*71)%208)}
}

// pixels are RGB cells sampled at the center of each 2x2 physical block,
// exactly like the existing packet. Only mapped tiles are consumed.
func DecodeAuraWatch(a *AuraWatch, pixels []byte) bool {
	if a == nil || len(pixels) != 128*8/4*3 {
		return false
	}
	for _, m := range a.Members {
		for i := range AuraWatchSpells[m.Class] {
			if m.Mask&(1<<i) == 0 {
				continue
			}
			slot := m.Slot*8 + i
			x, y := (slot%32)*2, (slot/32)*2
			for cell := 0; cell < 4; cell++ {
				off := ((y+cell/2)*64 + x + cell%2) * 3
				got := [3]byte{pixels[off], pixels[off+1], pixels[off+2]}
				if cell == 0 {
					if got == AuraWatchColor(slot, 0, true) {
						a.Active[slot] = true
					} else if got != AuraWatchColor(slot, 0, false) {
						return false
					}
				} else if got != AuraWatchColor(slot, cell, false) {
					return false
				}
			}
		}
	}
	a.Captured = true
	return true
}
