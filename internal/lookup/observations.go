package lookup

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type LiveCast struct {
	ID, GUID         string
	Spell, AgeMillis int
}
type BuildChunk struct {
	ID, GUID, Game    string
	At                time.Time
	Spec, Part, Total int
	Data              string
}
type SpellMetadata struct {
	Build               string
	Root, Spell, Millis int
	Name                string
}

type Observations struct {
	Context      *PartyContext
	ScreenStatus *ScreenStatus
	Screen       *ScreenCasts
	Auras        *AuraWatch
	Metadata     []SpellMetadata
	Casts        []LiveCast
	Builds       []BuildChunk
	Diagnostics  []uint64
}

type PartyContext struct {
	Arena bool
	PvP   bool
	At    time.Time
	Epoch string
}

var observationGUID = regexp.MustCompile(`^Player-[A-Za-z0-9-]{1,100}$`)
var observationID = regexp.MustCompile(`^[A-Za-z0-9:-]{1,128}$`)
var gameVersion = regexp.MustCompile(`^[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2}$`)

func parseObservations(body string) (*Observations, error) {
	bad := fmt.Errorf("observation_bridge_invalid")
	if len(body) > 640 {
		return nil, bad
	}
	o := &Observations{}
	if body == "" {
		return o, nil
	}
	lines := strings.Split(body, "\n")
	if len(lines) > 8 {
		return nil, bad
	}
	integer := func(s string, lo, hi int) (int, bool) {
		n, e := strconv.Atoi(s)
		return n, e == nil && n >= lo && n <= hi && strconv.Itoa(n) == s
	}
	for _, line := range lines {
		p := strings.Split(line, "|")
		switch p[0] {
		case "P":
			if len(p) != 5 || p[1] != "1" || o.Context != nil || (p[2] != "0" && p[2] != "1" && p[2] != "2") || !observationID.MatchString(p[4]) {
				return nil, bad
			}
			stamp, err := strconv.ParseInt(p[3], 10, 64)
			if err != nil || stamp < 1 || strconv.FormatInt(stamp, 10) != p[3] {
				return nil, bad
			}
			o.Context = &PartyContext{Arena: p[2] == "1", PvP: p[2] != "0", At: time.Unix(stamp, 0), Epoch: p[4]}
		case "S":
			if len(p) != 6 || o.ScreenStatus != nil || len(p[5]) > 140 || !utf8.ValidString(p[5]) || strings.ContainsAny(p[5], "\x00\r") {
				return nil, bad
			}
			switch p[1] {
			case "idle", "unsupported", "disabled", "failed", "font_pending", "active", "moving", "raid", "preview":
			default:
				return nil, bad
			}
			width, ok := integer(p[2], -1, 1000000)
			if !ok {
				return nil, bad
			}
			displayed, ok := integer(p[3], 0, 2147483647)
			if !ok {
				return nil, bad
			}
			errors, ok := integer(p[4], 0, 2147483647)
			if !ok {
				return nil, bad
			}
			o.ScreenStatus = &ScreenStatus{State: p[1], FontWidth: float64(width) / 1000, Displayed: displayed, Errors: errors, Error: p[5]}
		case "T":
			if o.Screen != nil {
				return nil, bad
			}
			s, err := parseScreenCasts(p)
			if err != nil {
				return nil, err
			}
			o.Screen = s
		case "A":
			if o.Auras != nil {
				return nil, bad
			}
			a, err := parseAuraWatch(p)
			if err != nil {
				return nil, err
			}
			o.Auras = a
		case "M":
			if len(p) != 6 || len(o.Metadata) >= 3 || !regexp.MustCompile(`^1\.60\.[0-9]{1,2}\.[0-9]{4,7}$`).MatchString(p[1]) || !utf8.ValidString(p[5]) || len(p[5]) < 1 || len(p[5]) > 128 || strings.ContainsAny(p[5], "\x00\r") {
				return nil, bad
			}
			root, ok := integer(p[2], 1, 10000000)
			if !ok {
				return nil, bad
			}
			spell, ok := integer(p[3], 1, 10000000)
			if !ok {
				return nil, bad
			}
			ms, ok := integer(p[4], 0, 7200000)
			if !ok {
				return nil, bad
			}
			o.Metadata = append(o.Metadata, SpellMetadata{p[1], root, spell, ms, p[5]})
		case "C":
			if len(p) != 5 || !observationID.MatchString(p[1]) || !observationGUID.MatchString(p[2]) || len(o.Casts) >= 4 {
				return nil, bad
			}
			spell, ok := integer(p[3], 1, 10000000)
			if !ok {
				return nil, bad
			}
			age, ok := integer(p[4], 0, 30000)
			if !ok {
				return nil, bad
			}
			o.Casts = append(o.Casts, LiveCast{p[1], p[2], spell, age})
		case "B":
			if len(p) != 9 || len(o.Builds) >= 1 || !observationID.MatchString(p[1]) || !observationGUID.MatchString(p[2]) || !gameVersion.MatchString(p[5]) || len(p[8]) > 180 {
				return nil, bad
			}
			stamp, e := strconv.ParseInt(p[3], 10, 64)
			if e != nil || stamp < 1 {
				return nil, bad
			}
			spec, ok := integer(p[4], 1, 100000)
			if !ok {
				return nil, bad
			}
			part, ok := integer(p[6], 1, 24)
			if !ok {
				return nil, bad
			}
			total, ok := integer(p[7], part, 24)
			if !ok {
				return nil, bad
			}
			o.Builds = append(o.Builds, BuildChunk{p[1], p[2], p[5], time.Unix(stamp, 0), spec, part, total, p[8]})
		case "D":
			if len(p) != 6 || o.Diagnostics != nil {
				return nil, bad
			}
			for _, s := range p[1:] {
				n, ok := integer(s, 0, 2147483647)
				if !ok {
					return nil, bad
				}
				o.Diagnostics = append(o.Diagnostics, uint64(n))
			}
		default:
			return nil, bad
		}
	}
	return o, nil
}
