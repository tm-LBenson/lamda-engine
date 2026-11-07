// Package framewire describes public frame geometry and native widget ownership,
// never aura/cooldown values.
package framewire

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Rect struct {
	GUID         string         `json:"guid"`
	X            int            `json:"x"`
	Y            int            `json:"y"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	NativeSpells []int          `json:"nativeSpells,omitempty"`
	SelfSlots    []CooldownSlot `json:"selfSlots,omitempty"`
}

// A non-nil SelfSlots slice fixes own engine fallbacks to the native row's
// selected learned spells and physical slots. Empty means no eligible slots.
type CooldownSlot struct {
	X, Y, Size, Opacity int
	Spells              []int
}

type Snapshot struct {
	PartyLayout                                                            *Layout
	Persistent                                                             bool
	Growth                                                                 int
	ID                                                                     string    `json:"id"`
	At                                                                     time.Time `json:"at"`
	Width                                                                  int       `json:"width"`
	Height                                                                 int       `json:"height"`
	Player                                                                 string    `json:"player"`
	Size, Spacing, PerRow, Maximum, Opacity, Anchor, Gap, OffsetX, OffsetY int
	Self, Party, Ready, Unknown                                            bool
	Omitted                                                                int    `json:"omitted"`
	Frames                                                                 []Rect `json:"frames"`
}

// Lanes anchor their full configured capacity. Growth 0 preserves old packets;
// 1 is automatic (outward on the left), 2 left-to-right, 3 right-to-left.
type Layout struct {
	Size, Spacing, PerRow, Maximum, Opacity, Anchor, Gap, OffsetX, OffsetY int
	Growth                                                                 int
}

var identity = regexp.MustCompile(`^(Player|Creature|Pet)-[A-Za-z0-9-]{1,100}$`)
var packetID = regexp.MustCompile(`^[0-9:]{1,80}$`)

// Parse accepts complete, bounded snapshots. A malformed member invalidates the
// snapshot so the renderer cannot retain a plausible but incorrectly bound icon.
func Parse(body string) (*Snapshot, error) {
	bad := fmt.Errorf("frame_bridge_invalid")
	if len(body) > 1526 {
		return nil, bad
	}
	lines := strings.Split(body, "\n")
	h := strings.Split(lines[0], "|")
	if (len(h) < 19 || len(h) > 22) || len(lines) > 81 || !packetID.MatchString(h[0]) || !identity.MatchString(h[4]) {
		return nil, bad
	}
	s := &Snapshot{ID: h[0], Player: h[4], Frames: []Rect{}}
	if len(h) >= 20 {
		if h[19] != "0" && h[19] != "1" {
			return nil, bad
		}
		s.Persistent = h[19] == "1"
	}
	if len(h) == 22 {
		growth, err := strconv.Atoi(h[21])
		if err != nil || growth < 1 || growth > 3 {
			return nil, bad
		}
		s.Growth = growth
	}
	if len(h) == 21 || (len(h) == 22 && h[20] != "") {
		parts := strings.Split(h[20], ",")
		if len(parts) != 9 && len(parts) != 10 {
			return nil, bad
		}
		layout := &Layout{}
		fields := []struct {
			p      *int
			lo, hi int
		}{{&layout.Size, 8, 256}, {&layout.Spacing, 0, 64}, {&layout.PerRow, 1, 12}, {&layout.Maximum, 1, 12}, {&layout.Opacity, 10, 100}, {&layout.Anchor, 1, 4}, {&layout.Gap, 0, 160}, {&layout.OffsetX, -800, 800}, {&layout.OffsetY, -800, 800}}
		for i, f := range fields {
			n, e := strconv.Atoi(parts[i])
			if e != nil || n < f.lo || n > f.hi {
				return nil, bad
			}
			*f.p = n
		}
		if len(parts) == 10 {
			growth, err := strconv.Atoi(parts[9])
			if err != nil || growth < 0 || growth > 3 {
				return nil, bad
			}
			layout.Growth = growth
		}
		s.PartyLayout = layout
	}
	at, err := strconv.ParseInt(h[1], 10, 64)
	if err != nil || at < 1 {
		return nil, bad
	}
	s.At = time.Unix(at, 0)
	fields := []struct {
		index  int
		p      *int
		lo, hi int
	}{
		{2, &s.Width, 128, 16000}, {3, &s.Height, 16, 16000},
		{5, &s.Size, 8, 256}, {6, &s.Spacing, 0, 64}, {7, &s.PerRow, 1, 12},
		{8, &s.Maximum, 1, 12}, {9, &s.Opacity, 10, 100}, {10, &s.Anchor, 1, 4},
		{11, &s.Gap, 0, 160}, {12, &s.OffsetX, -800, 800}, {13, &s.OffsetY, -800, 800},
		{18, &s.Omitted, 0, 160},
	}
	for _, f := range fields {
		n, e := strconv.Atoi(h[f.index])
		if e != nil || n < f.lo || n > f.hi {
			return nil, bad
		}
		*f.p = n
	}
	for i, p := range []*bool{&s.Self, &s.Party, &s.Ready, &s.Unknown} {
		if h[14+i] != "0" && h[14+i] != "1" {
			return nil, bad
		}
		*p = h[14+i] == "1"
	}
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		f := strings.Split(line, "|")
		if (len(f) < 5 || len(f) > 7) || !identity.MatchString(f[0]) {
			return nil, bad
		}
		key := strings.Join(f[:5], "|")
		if seen[key] {
			return nil, bad
		}
		r := Rect{GUID: f[0]}
		seen[key] = true
		if len(f) >= 6 {
			// Only the local player's native cooldown widgets are supported.
			if r.GUID != s.Player {
				return nil, bad
			}
			ids := strings.Split(f[5], ",")
			if len(f) == 7 && f[5] == "" {
				ids = nil
			}
			// Up to 128 selected bases plus 12 displayed overrides.
			if len(ids) > 140 {
				return nil, bad
			}
			previous := 0
			for _, value := range ids {
				id, err := strconv.Atoi(value)
				if err != nil || id <= previous || id > 10000000 || strconv.Itoa(id) != value {
					return nil, bad
				}
				r.NativeSpells = append(r.NativeSpells, id)
				previous = id
			}
		}
		if len(f) == 7 {
			var err error
			r.SelfSlots, err = parseCooldownSlots(f[6], s.Width, s.Height)
			if err != nil {
				return nil, bad
			}
		}
		for i, p := range []*int{&r.X, &r.Y, &r.Width, &r.Height} {
			n, e := strconv.Atoi(f[i+1])
			if e != nil {
				return nil, bad
			}
			*p = n
		}
		if r.X < 0 || r.Y < 0 || r.X >= s.Width || r.Y >= s.Height || r.Width < 1 || r.Height < 1 || r.Width > s.Width-r.X || r.Height > s.Height-r.Y {
			return nil, bad
		}
		s.Frames = append(s.Frames, r)
	}
	return s, nil
}
