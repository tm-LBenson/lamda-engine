package lookup

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Arena identities are scoped to one addon session/round and native arena token.
// GUID is optional corroboration for logs, never a name/spec-based guess.
type ArenaActor struct {
	Slot, Spec, X, Y, Size int
	GUID                   string
}
type ArenaSnapshot struct {
	Epoch                                  string
	Sequence                               int
	At                                     time.Time
	Width, Height, Flags, Maximum, Opacity int
	Hidden                                 bool
	Actors                                 []ArenaActor
}

func (a *ArenaSnapshot) Identity(slot int) string { return fmt.Sprintf("Arena:%s:%d", a.Epoch, slot) }

func parseArena(body string) BridgeSample {
	bad := BridgeSample{Error: "arena_bridge_invalid"}
	parts := strings.Split(body, "\n\n")
	if len(parts) != 2 {
		return bad
	}
	lines := strings.Split(parts[0], "\n")
	if len(lines) != 2 {
		return bad
	}
	h := strings.Split(lines[0], "|")
	if len(h) != 11 || h[0] != "1" || len(h[1]) > 64 || !observationID.MatchString(h[1]) {
		return bad
	}
	valid := true
	integer := func(s string, lo, hi int) int {
		v, e := strconv.Atoi(s)
		if e != nil || v < lo || v > hi || strconv.Itoa(v) != s {
			valid = false
		}
		return v
	}
	a := &ArenaSnapshot{Epoch: h[1], Sequence: integer(h[2], 1, 2147483647)}
	a.At = time.Unix(int64(integer(h[3], 1, 4102444800)), 0)
	a.Width = integer(h[4], 128, 16000)
	a.Height = integer(h[5], 200, 16000)
	a.Flags = integer(h[6], 0, 31)
	a.Maximum = integer(h[7], 1, 8)
	a.Opacity = integer(h[8], 20, 100)
	background := integer(h[9], 0, 1) == 1
	a.Hidden = integer(h[10], 0, 1) == 1
	seen, guids := map[int]bool{}, map[string]bool{}
	if lines[1] != "" {
		for _, line := range strings.Split(lines[1], ";") {
			p := strings.Split(line, ",")
			if len(p) != 6 {
				return bad
			}
			actor := ArenaActor{Slot: integer(p[0], 1, 5), Spec: integer(p[1], 0, 2000), X: integer(p[2], -1000, a.Width+1000), Y: integer(p[3], -1000, a.Height+1000), Size: integer(p[4], 8, 192), GUID: p[5]}
			if seen[actor.Slot] || len(a.Actors) >= 5 {
				return bad
			}
			seen[actor.Slot] = true
			if actor.GUID == "-" {
				actor.GUID = ""
			} else if !observationGUID.MatchString(actor.GUID) || guids[actor.GUID] {
				return bad
			} else {
				guids[actor.GUID] = true
			}
			a.Actors = append(a.Actors, actor)
		}
	}
	if !valid {
		return bad
	}
	obs, err := parseObservations(parts[1])
	if err != nil {
		return bad
	}
	// Arena v1 permits only the bound native cast panel, not party build/cast data.
	if len(obs.Casts) > 0 || len(obs.Builds) > 0 || len(obs.Metadata) > 0 || obs.Auras != nil || obs.ScreenStatus != nil || obs.Context != nil || len(obs.Diagnostics) > 0 {
		return bad
	}
	if s := obs.Screen; s != nil {
		if len(s.Members) != len(a.Actors) {
			return bad
		}
		for i := range s.Members {
			m := &s.Members[i]
			if !seen[m.Member] || m.Slot != m.Member-1 {
				return bad
			}
			m.GUID = a.Identity(m.Member)
		}
	}
	return BridgeSample{Arena: a, Observations: obs, Event: Event{ID: fmt.Sprintf("Arena:%s:%d", a.Epoch, a.Sequence), At: a.At, KeepUIWhenUnfocused: background}}
}
