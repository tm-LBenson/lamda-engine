package framewire

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var npcGUID = regexp.MustCompile(`^(Creature|Vehicle)-[0-9]+-[0-9]+-[0-9]+-[0-9]+-[0-9]+-[A-Fa-f0-9]+$`)

// IsNPCGUID recognizes bounded public NPC identities from older party packets.
// NPCs retain their frame/log tracking but never become player lookup targets.
func IsNPCGUID(guid string) bool {
	return len(guid) <= 108 && npcGUID.MatchString(guid)
}

type KickMember struct {
	GUID, Pet, Name string
	Class, Spec     int
	Frame           *Rect
}

// KickSnapshot is a separate module contract. Mode zero immediately hides it.
// Roster identities and frame identities are refreshed together in one packet.
type KickSnapshot struct {
	ID, Player                                                                                                  string
	At                                                                                                          time.Time
	Width, Height, Mode, X, Y, BarWidth, BarHeight, Size, Opacity, Anchor, Gap, OffsetX, OffsetY, Sort, Omitted int
	Self, Preview, Counts                                                                                       bool
	// Positive: a native player cooldown owns this spell's row; -1: no selected
	// learned interrupt. Zero retains compatibility with older addon packets.
	OwnSpell int
	Members  []KickMember
}

func ParseKick(body string) (*KickSnapshot, error) {
	bad := fmt.Errorf("kick_bridge_invalid")
	lines := strings.Split(body, "\n")
	h := strings.Split(lines[0], "|")
	if len(body) > 900 || (len(h) != 20 && len(h) != 21 && len(h) != 22) || len(lines) > 6 || !packetID.MatchString(h[0]) || !strings.HasPrefix(h[4], "Player-") || !identity.MatchString(h[4]) {
		return nil, bad
	}
	s := &KickSnapshot{ID: h[0], Player: h[4]}
	if len(h) >= 21 {
		if h[20] != "0" && h[20] != "1" {
			return nil, bad
		}
		s.Counts = h[20] == "1"
	}
	if len(h) == 22 {
		v, err := strconv.Atoi(h[21])
		if err != nil || v < -1 || v > 10000000 {
			return nil, bad
		}
		s.OwnSpell = v
	}
	stamp, err := strconv.ParseInt(h[1], 10, 64)
	if err != nil || stamp < 1 {
		return nil, bad
	}
	s.At = time.Unix(stamp, 0)
	fields := []struct {
		index  int
		p      *int
		lo, hi int
	}{
		{2, &s.Width, 128, 16000}, {3, &s.Height, 16, 16000}, {5, &s.Mode, 0, 3},
		{7, &s.X, 0, 16000}, {8, &s.Y, 0, 16000}, {9, &s.BarWidth, 80, 1600}, {10, &s.BarHeight, 10, 192},
		{11, &s.Size, 8, 256}, {12, &s.Opacity, 20, 100}, {13, &s.Anchor, 1, 4}, {14, &s.Gap, 0, 320},
		{15, &s.OffsetX, -800, 800}, {16, &s.OffsetY, -800, 800}, {17, &s.Sort, 1, 2}, {19, &s.Omitted, 0, 5},
	}
	for _, f := range fields {
		n, e := strconv.Atoi(h[f.index])
		if e != nil || n < f.lo || n > f.hi {
			return nil, bad
		}
		*f.p = n
	}
	for i, p := range map[int]*bool{6: &s.Self, 18: &s.Preview} {
		if h[i] != "0" && h[i] != "1" {
			return nil, bad
		}
		*p = h[i] == "1"
	}
	if s.X+s.BarWidth > s.Width || s.Y+s.BarHeight > s.Height {
		return nil, bad
	}
	seen, pets := map[string]bool{}, map[string]bool{}
	for _, line := range lines[1:] {
		p := strings.Split(line, "|")
		if len(p) != 5 || (!(strings.HasPrefix(p[0], "Player-") && identity.MatchString(p[0])) && !IsNPCGUID(p[0])) || seen[p[0]] {
			return nil, bad
		}
		seen[p[0]] = true
		m := KickMember{GUID: p[0], Pet: p[3]}
		m.Class, err = strconv.Atoi(p[1])
		if err != nil || m.Class < 0 || m.Class > 13 {
			return nil, bad
		}
		m.Spec, err = strconv.Atoi(p[2])
		if err != nil || m.Spec < 0 || m.Spec > 10000 {
			return nil, bad
		}
		if m.Pet != "" {
			// UnitGUID(partypet) also returns Creature IDs for temporary summons
			// such as elementals. They must not invalidate unrelated party CDs.
			if (!strings.HasPrefix(m.Pet, "Pet-") && !strings.HasPrefix(m.Pet, "Creature-")) || !identity.MatchString(m.Pet) || pets[m.Pet] {
				return nil, bad
			}
			pets[m.Pet] = true
		}
		if p[4] != "" {
			v := strings.Split(p[4], ",")
			if len(v) != 4 {
				return nil, bad
			}
			r := &Rect{GUID: m.GUID}
			for i, out := range []*int{&r.X, &r.Y, &r.Width, &r.Height} {
				n, e := strconv.Atoi(v[i])
				if e != nil || n < 0 || n > 16000 {
					return nil, bad
				}
				*out = n
			}
			if r.Width <= 0 || r.Height <= 0 || r.X+r.Width > s.Width || r.Y+r.Height > s.Height {
				return nil, bad
			}
			m.Frame = r
		}
		// Old addons included companions in party slots. Validate their row,
		// then leave them out of player interrupt ownership.
		if !IsNPCGUID(m.GUID) {
			s.Members = append(s.Members, m)
		}
	}
	if s.Mode == 0 && len(lines) > 1 {
		return nil, bad
	}
	return s, nil
}
