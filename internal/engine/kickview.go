package engine

import (
	"sort"
	"strconv"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

type KickBar struct {
	FrameIcon
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Player   string `json:"player"`
	Color    string `json:"color"`
	Count    *int   `json:"count,omitempty"`
	Attempts *int   `json:"attempts,omitempty"`
	Native   bool   `json:"native,omitempty"`
}
type KickPanel struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Width   int  `json:"width"`
	Opacity int  `json:"opacity"`
	Preview bool `json:"preview"`
	Omitted int  `json:"omitted"`
}

var classColors = map[int]string{1: "#C69B6D", 2: "#F48CBA", 3: "#AAD372", 4: "#FFF468", 5: "#FFFFFF", 6: "#C41E3A", 7: "#0070DD", 8: "#3FC7EB", 9: "#8788EE", 10: "#00FF98", 11: "#FF7C0A", 12: "#A330C9", 13: "#33937F"}

// A zero entry is a known no-interrupt spec in Retail's current rules.
// Forever deliberately uses its own observed spell families instead.
var retailSpecInterrupts = map[int]int{
	71: 6552, 72: 6552, 73: 6552, 65: 0, 66: 96231, 70: 96231,
	253: 147362, 254: 147362, 255: 187707, 259: 1766, 260: 1766, 261: 1766,
	256: 0, 257: 0, 258: 15487, 250: 47528, 251: 47528, 252: 47528,
	262: 57994, 263: 57994, 264: 57994, 62: 2139, 63: 2139, 64: 2139,
	265: 19647, 266: 89766, 267: 19647, 268: 116705, 269: 116705, 270: 0,
	102: 78675, 103: 106839, 104: 106839, 105: 0, 577: 183752, 581: 183752,
	1467: 351338, 1468: 0, 1473: 351338,
}

func kickCandidate(member framewire.KickMember) int {
	if Forever {
		return 0
	} // No Retail class/spec suggestions for an unobserved beta ability.
	if spell, known := retailSpecInterrupts[member.Spec]; known {
		return spell
	}
	switch member.Class {
	case 1:
		return 6552
	case 2:
		if member.Spec == 65 {
			return 0
		}
		return 96231
	case 3:
		if member.Spec == 255 {
			return 187707
		}
		if member.Spec == 253 || member.Spec == 254 {
			return 147362
		}
	case 4:
		return 1766
	case 5:
		if member.Spec == 258 {
			return 15487
		}
	case 6:
		return 47528
	case 7:
		return 57994
	case 8:
		return 2139
	case 9:
		if member.Spec == 266 {
			return 89766
		}
		if member.Spec == 265 || member.Spec == 267 {
			return 19647
		}
	case 10:
		if member.Spec == 270 {
			return 0
		}
		return 116705
	case 11:
		if member.Spec == 102 {
			return 78675
		}
		if member.Spec == 103 || member.Spec == 104 {
			return 106839
		}
	case 12:
		return 183752
	case 13:
		if member.Spec == 1467 || member.Spec == 1473 {
			return 351338
		}
	}
	return 0
}

// The meter identifies an interrupter, not the interrupt ability. Use this
// fallback only for builds whose standard kick can be distinguished from
// alternate/persistent interrupts. Real spell/log observations remain available
// for excluded builds; never turn an Avenger's Shield hit into Rebuke.
func (m *Model) MeterInterruptSpell(member framewire.KickMember, now time.Time) int {
	if Forever {
		return 0
	}
	build, inspected := m.InspectedBuilds[member.GUID]
	inspected = inspected && build.SpellRanks != nil && now.Sub(build.At) >= 0 && now.Sub(build.At) <= 5*time.Minute
	if inspected {
		member.Spec = build.SpecID
	}
	switch member.Class {
	case 1:
		if member.Spec == 0 || (member.Spec == 73 && (!inspected || build.SpellRanks[386071] > 0)) {
			return 0
		}
	case 2:
		if member.Spec != 70 {
			return 0
		}
	case 3:
		if member.Spec != 253 && member.Spec != 255 {
			return 0
		}
	case 11:
		if member.Spec != 103 && member.Spec != 104 {
			return 0
		}
	case 12:
		if member.Spec != 577 || !inspected || build.SpellRanks[202137] > 0 {
			return 0
		}
	}
	return kickEvidence(m.Snapshot(now), member, now).Spell
}

func kickEvidence(s InferenceSnapshot, member framewire.KickMember, now time.Time) CooldownEvidence {
	// Observations and completed party warm-up beat a generic class suggestion.
	var found CooldownEvidence
	for _, e := range s.Cooldowns {
		if e.GUID == member.GUID && IsInterrupt(e.Spell) && e.EventAt.After(found.EventAt) {
			found = e
		}
	}
	var build *BuildSnapshot
	for i := range s.Builds {
		b := &s.Builds[i]
		if !Forever && b.GUID == member.GUID && b.Source == "inspect" && b.SpellRanks != nil && now.Sub(b.At) >= 0 && (now.Sub(b.At) <= inspectRetention || s.retainsPartyBuild(*b, now)) {
			build = b
			member.Spec = b.SpecID
			break
		}
	}
	id := kickCandidate(member)
	if build != nil {
		for _, candidate := range []int{1766, 2139, 6552, 15487, 19647, 47528, 57994, 78675, 89766, 96231, 106839, 116705, 147362, 183752, 187707, 351338} {
			if build.SpellRanks[candidate] > 0 {
				id = candidate
				break
			}
		}
		if id != 0 && !referenceRules.NonTalentSpells[id] && build.SpellRanks[id] == 0 {
			id = 0
		}
		// A spec/talent change invalidates older timing for the removed interrupt.
		if found.Spell != 0 && build.At.After(found.EventAt) && found.Spell != id {
			found = CooldownEvidence{}
		}
	}
	joined, inParty := s.PartySince[member.GUID]
	oldReady := inParty && found.EventAt.Before(joined) && found.EstimatedEnd <= float64(now.UnixMilli())/1000
	if found.Spell != 0 && found.Status != "unknown" && !oldReady {
		return found
	}
	// A fresh, complete inspection with no selected interrupt is different
	// from a failed inspection. Do not turn confirmed absence into a '?' row.
	if !Forever && id == 0 && (retailNoInterrupt(member) ||
		(build != nil && specializationDefensives[member.Spec] != nil)) {
		return CooldownEvidence{GUID: member.GUID, Status: "unavailable", Source: "inspect"}
	}
	spell := InterruptCatalog[id]
	if id == 0 {
		spell.Name = "Interrupt"
	}
	// A normal cooldown spent with this member is the requested initial-ready
	// assumption, even while inspection is still pending. A real use above wins.
	if !Forever && inParty && id != 0 && !now.Before(joined) {
		baseline := spell.Cooldown
		if seconds := referenceRules.SpecializationBaselines[member.Spec][id]; seconds > 0 {
			baseline = seconds
		}
		e := CooldownEvidence{GUID: member.GUID, Player: member.Name, Spell: id, Name: spell.Name,
			Status: "calibrating", Source: "party_calibration", EventAt: joined,
			BaselineSeconds: baseline, CooldownSeconds: baseline,
			EstimatedEnd: float64(joined.UnixMilli())/1000 + baseline,
			Reasons:      []string{"party_cooldown_calibration", "current_cooldown_not_queried"}}
		if build != nil {
			applyBuildRules(&e, *build)
		}
		// Invalidated evidence may still contain an unfinished longer timer.
		e.EstimatedEnd = max(e.EstimatedEnd, found.EstimatedEnd)
		if float64(now.UnixMilli())/1000 >= e.EstimatedEnd {
			e.Status = "estimated_ready"
			e.Reasons = []string{"full_cooldown_in_party", "current_cooldown_not_queried"}
		}
		return e
	}
	if found.Spell != 0 {
		return found
	}
	return CooldownEvidence{GUID: member.GUID, Player: member.Name, Spell: id, Name: spell.Name, Status: "unknown", Source: "interrupt_candidate", Reasons: []string{"cooldown_not_observed", "ability_not_verified"}}
}

func retailNoInterrupt(member framewire.KickMember) bool {
	// Spec IDs come from the fresh in-game inspection cache, never party role.
	return (member.Class == 2 && member.Spec == 65) ||
		(member.Class == 5 && (member.Spec == 256 || member.Spec == 257)) ||
		(member.Class == 10 && member.Spec == 270) ||
		(member.Class == 11 && member.Spec == 105) ||
		(member.Class == 13 && member.Spec == 1468)
}

func KickView(s InferenceSnapshot, c Config, k *framewire.KickSnapshot, received, now time.Time, window uintptr) FrameDisplay {
	d := FrameDisplay{Status: "kick_disabled", At: float64(now.UnixMilli()) / 1000, Icons: []FrameIcon{}, KickBars: []KickBar{}}
	if !c.KickEnabled || k == nil || k.Mode == 0 || (c.KickSpells != nil && len(c.KickSpells) == 0) {
		return d
	}
	if now.Before(received) || now.Sub(received) > FrameLifetime || now.Sub(k.At) > FrameTimestampSkew || k.At.Sub(now) > FrameTimestampSkew {
		d.Status = "kick_data_stale"
		return d
	}
	d.Enabled = true
	d.Status = "receiving"
	d.Window = strconv.FormatUint(uint64(window), 10)
	d.Width = k.Width
	d.Height = k.Height
	d.Expires = float64(received.Add(FrameLifetime).UnixMilli()) / 1000
	d.GeometryAge = now.Sub(received).Seconds()
	selected := map[int]bool{}
	for _, id := range c.KickSpells {
		selected[InterruptSpell(id)] = true
	}
	members := k.Members
	if k.Preview && k.Mode == 2 {
		members = []framewire.KickMember{{GUID: "Preview-1", Name: "Warrior", Class: 1}, {GUID: "Preview-2", Name: "Shaman", Class: 7}, {GUID: "Preview-3", Name: "Mage", Class: 8}, {GUID: "Preview-4", Name: "Rogue", Class: 4}, {GUID: "Preview-5", Name: "Monk", Class: 10}}
	}
	for i, member := range members {
		if member.GUID == k.Player && !k.Self {
			continue
		}
		e := kickEvidence(s, member, now)
		native := !k.Preview && member.GUID == k.Player && k.OwnSpell > 0
		if !k.Preview && member.GUID == k.Player && k.OwnSpell == -1 {
			continue
		}
		if native {
			e = CooldownEvidence{GUID: member.GUID, Spell: k.OwnSpell, Name: InterruptCatalog[InterruptSpell(k.OwnSpell)].Name, Status: "native", Source: "player_cooldown"}
		}
		if !k.Preview && e.Status == "unavailable" {
			continue
		}
		if !k.Preview && c.KickSpells != nil && e.Spell != 0 && !selected[e.Spell] {
			continue
		}
		if k.Preview {
			e = CooldownEvidence{GUID: member.GUID, Player: member.Name, Spell: kickCandidate(member), Status: "estimated_cooldown", Source: "preview", CooldownSeconds: 15, EstimatedEnd: d.At + float64(5+i*2), Reasons: []string{"preview"}}
			e.Name = InterruptCatalog[e.Spell].Name
			if e.Name == "" {
				e.Name = "Interrupt"
			}
			if i%3 == 1 {
				e.Status = "estimated_ready"
			}
			if i%3 == 2 {
				e.Status = "unknown"
			}
		}
		icon := FrameIcon{GUID: member.GUID, Spell: e.Spell, Name: e.Name, Label: member.Name, Size: k.Size, Opacity: k.Opacity, Status: e.Status, Ends: e.EstimatedEnd, Duration: e.CooldownSeconds, Source: e.Source, DeliverySeconds: e.DeliverySeconds, Reasons: e.Reasons}
		icon.Kick = true
		if k.Mode == 1 || k.Mode == 3 {
			if f := member.Frame; f != nil && !native {
				icon.X = f.X
				icon.Y = f.Y + f.Height + k.Gap
				switch k.Anchor {
				case 2:
					icon.Y = f.Y - k.Size - k.Gap
				case 3:
					icon.X = f.X - k.Size - k.Gap
					icon.Y = f.Y + (f.Height-k.Size)/2
				case 4:
					icon.X = f.X + f.Width + k.Gap
					icon.Y = f.Y + (f.Height-k.Size)/2
				}
				icon.X += k.OffsetX
				icon.Y -= k.OffsetY
				if icon.X >= 0 && icon.Y >= 0 && icon.X+k.Size <= k.Width && icon.Y+k.Size <= k.Height {
					d.Icons = append(d.Icons, icon)
				}
			}
		}
		if k.Mode == 2 || k.Mode == 3 {
			color := classColors[member.Class]
			if color == "" {
				color = "#8995A5"
			}
			bar := KickBar{FrameIcon: icon, Width: k.BarWidth, Height: k.BarHeight, Player: member.Name, Color: color, Native: native}
			if k.Counts {
				count := s.InterruptCounts[member.GUID]
				attempts := max(count, s.InterruptAttempts[member.GUID])
				if k.Preview {
					count = 2 + i*3
					attempts = count + 1 + i
				}
				bar.Count = &count
				bar.Attempts = &attempts
			}
			d.KickBars = append(d.KickBars, bar)
		}
	}
	if k.Sort == 2 {
		sort.SliceStable(d.KickBars, func(i, j int) bool {
			rank := func(b KickBar) int {
				switch b.Status {
				case "native":
					return -1
				case "estimated_ready":
					return 0
				case "estimated_cooldown":
					return 1
				}
				return 2
			}
			a, b := d.KickBars[i], d.KickBars[j]
			if rank(a) != rank(b) {
				return rank(a) < rank(b)
			}
			if a.Status == "estimated_cooldown" {
				return a.Ends < b.Ends
			}
			return false
		})
	}
	// The addon can place its own native row without reading teammate timers.
	// Keep it at the same first position for both bar sorting options.
	sort.SliceStable(d.KickBars, func(i, j int) bool { return d.KickBars[i].Native && !d.KickBars[j].Native })
	for i := range d.KickBars {
		b := &d.KickBars[i]
		b.X = k.X
		if k.Counts {
			b.X = min(max(28, b.X), k.Width-k.BarWidth)
		}
		b.Y = k.Y + 24 + i*(k.BarHeight+3)
	}
	if k.Mode == 2 || k.Mode == 3 {
		x := k.X
		if k.Counts {
			x = min(max(28, x), k.Width-k.BarWidth)
		}
		d.KickPanel = &KickPanel{X: x, Y: k.Y, Width: k.BarWidth, Opacity: k.Opacity, Preview: k.Preview, Omitted: k.Omitted}
	}
	return d
}

func MergeKickDisplay(cd, kick FrameDisplay) FrameDisplay {
	if !kick.Enabled {
		return cd
	}
	if !cd.Enabled || cd.Window != kick.Window || cd.Width != kick.Width || cd.Height != kick.Height {
		return kick
	}
	cd.Expires = min(cd.Expires, kick.Expires)
	cd.Icons = append(cd.Icons, kick.Icons...)
	cd.KickBars = kick.KickBars
	cd.KickPanel = kick.KickPanel
	return cd
}
