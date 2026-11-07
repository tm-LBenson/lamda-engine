package framewire

import (
	"fmt"
	"strconv"
	"strings"
)

func parseCooldownSlots(value string, width, height int) ([]CooldownSlot, error) {
	result := []CooldownSlot{}
	if value == "" {
		return result, nil
	}
	bad := fmt.Errorf("invalid cooldown slots")
	parts := strings.Split(value, ";")
	if len(parts) > 12 {
		return nil, bad
	}
	seen := map[int]bool{}
	for _, part := range parts {
		position, aliases, ok := strings.Cut(part, ":")
		fields := strings.Split(position, ",")
		if !ok || len(fields) != 4 {
			return nil, bad
		}
		slot := CooldownSlot{}
		for i, p := range []*int{&slot.X, &slot.Y, &slot.Size, &slot.Opacity} {
			n, err := strconv.Atoi(fields[i])
			if err != nil || strconv.Itoa(n) != fields[i] {
				return nil, bad
			}
			*p = n
		}
		if slot.X < 0 || slot.Y < 0 || slot.X >= width || slot.Y >= height || slot.Size < 8 || slot.Size > 256 ||
			slot.Size > width-slot.X || slot.Size > height-slot.Y || slot.Opacity < 10 || slot.Opacity > 100 {
			return nil, bad
		}
		previous := 0
		for _, alias := range strings.Split(aliases, ",") {
			id, err := strconv.Atoi(alias)
			if err != nil || id <= previous || id > 10000000 || strconv.Itoa(id) != alias || seen[id] || len(seen) >= 140 {
				return nil, bad
			}
			seen[id] = true
			previous = id
			slot.Spells = append(slot.Spells, id)
		}
		result = append(result, slot)
	}
	return result, nil
}
