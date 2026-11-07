package engine

import (
	"testing"
	"time"
)

func TestInterruptPublicAndScreenPathsCountOneUseInAnyOrder(t *testing.T) {
	// The Sept 29 Kings' Rest run delivered each own Quell through both live
	// paths, then the meter and buffered log. Previously three uses became 6/6.
	var orders func([]string, []string)
	orders = func(prefix, remaining []string) {
		if len(remaining) != 0 {
			for i, path := range remaining {
				next := append([]string{}, remaining[:i]...)
				next = append(next, remaining[i+1:]...)
				orders(append(append([]string{}, prefix...), path), next)
			}
			return
		}
		t.Run(prefix[0]+"/"+prefix[1]+"/"+prefix[2]+"/"+prefix[3], func(t *testing.T) {
			m := NewModel()
			at := time.Unix(1790701778, 0)
			guid := "Player-1-00000003"
			for n := 0; n < 3; n++ {
				castAt := at.Add(time.Duration(n) * time.Minute)
				for i, path := range prefix {
					now := castAt.Add(time.Duration(i+1) * 10 * time.Second)
					c := Cast{GUID: guid, Spell: 351338, At: castAt.Add(70 * time.Millisecond), Source: "unit_spellcast", RosterMatched: true, Self: true, LivePath: path}
					switch path {
					case "screen_capture":
						c.At = castAt.Add(140 * time.Millisecond)
						m.Observe(c, now)
					case "public_packet":
						m.Observe(c, now)
					case "meter":
						c.Source, c.LivePath, c.At = "damage_meter", "", castAt.Add(400*time.Millisecond)
						m.ObserveMeterInterrupt(c, now)
					case "log":
						m.Ingest(interruptLine(castAt, "SPELL_CAST_SUCCESS", guid, "Creature-1-A", c.Spell), now, false)
						m.Ingest(interruptLine(castAt.Add(2*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", c.Spell), now, false)
					}
					// Repeated delivery on either live path is also a duplicate.
					if path == "screen_capture" || path == "public_packet" {
						m.Observe(c, now.Add(time.Millisecond))
					}
				}
				requireInterruptFraction(t, m, castAt.Add(50*time.Second), guid, n+1, n+1)
			}
		})
	}
	orders(nil, []string{"public_packet", "screen_capture", "meter", "log"})
}

func TestInterruptPathsPreserveDistinctCastsAndOwners(t *testing.T) {
	at := time.Unix(1790701778, 0)
	m := NewModel()
	for _, guid := range []string{"Player-1-A", "Player-1-B"} {
		// A fast reset gives two distinct casts on each path; only the second
		// landed. A different owner's events must never match these uses.
		for _, offset := range []time.Duration{0, 700 * time.Millisecond} {
			for _, path := range []string{"public_packet", "screen_capture"} {
				c := Cast{GUID: guid, Spell: 351338, At: at.Add(offset), Source: "unit_spellcast", LivePath: path}
				if path == "screen_capture" {
					c.At = c.At.Add(70 * time.Millisecond)
				}
				m.Observe(c, at.Add(time.Second))
			}
		}
		m.Ingest(interruptLine(at.Add(702*time.Millisecond), "SPELL_INTERRUPT", guid, "Creature-1-A", 351338), at.Add(30*time.Second), false)
		requireInterruptFraction(t, m, at.Add(30*time.Second), guid, 1, 2)
	}
}
