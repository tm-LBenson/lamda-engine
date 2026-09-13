package engine

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// NPC/spell pairs observed in local follower-dungeon and delve recordings.
// No player cooldown values are assigned to NPC variants.
var CompanionSpells = map[int]map[int]string{
	209057: {425435: "Ardent Defender", 283630: "Lay on Hands", 283627: "Divine Shield", 420090: "Rebuke"},
	209059: {295238: "Blazing Barrier", 413889: "Ice Block", 429812: "Counterspell"},
	214390: {292158: "Astral Shift", 420320: "Wind Shear", 426214: "Healing Stream Totem"},
	209065: {284341: "Exhilaration", 426240: "Counter Shot", 455641: "Tranquilizing Shot"},
	248567: {1248172: "Dagger to the Throat", 1248109: "Blood Contract", 1248595: "No Witnesses", 1251050: "Your First Mistake", 1248339: "Assassinate", 1251054: "Poison Cloud"},
}

func CompanionSpell(guid string, spell int) (string, bool) {
	parts := strings.Split(guid, "-")
	if len(parts) != 7 || (parts[0] != "Creature" && parts[0] != "Vehicle") {
		return "", false
	}
	npc, e := strconv.Atoi(parts[5])
	if e != nil {
		return "", false
	}
	name, ok := CompanionSpells[npc][spell]
	return name, ok
}

type companionHistory struct {
	Last               time.Time
	Interval, Estimate float64
}

func (m *Model) observeCompanion(c Cast, now time.Time) bool {
	name, ok := CompanionSpell(c.GUID, c.Spell)
	if !ok {
		return false
	}
	delay := now.Sub(c.At).Seconds()
	if delay < -2 || delay > 30 {
		return false
	}
	key := fmt.Sprintf("%s/%d", c.GUID, c.Spell)
	h := m.CompanionHistory[key]
	if !h.Last.IsZero() && !c.At.After(h.Last) {
		return false
	}
	interval := c.At.Sub(h.Last).Seconds()
	if !h.Last.IsZero() && interval >= 2 && interval <= 600 {
		if h.Estimate > 0 && interval < h.Estimate*.85 {
			h.Estimate = 0
		}
		if h.Interval >= 2 && math.Abs(interval-h.Interval) <= math.Max(1.5, math.Min(interval, h.Interval)*.1) {
			candidate := math.Min(interval, h.Interval)
			if h.Estimate == 0 || candidate < h.Estimate {
				h.Estimate = candidate
			}
		}
		h.Interval = interval
	} else {
		h.Interval = 0
		h.Estimate = 0
	}
	h.Last = c.At
	// Bound history independently of visible rows; expired pulses must not erase learning.
	if _, exists := m.CompanionHistory[key]; !exists && len(m.CompanionHistory) >= 256 {
		for k, v := range m.CompanionHistory {
			if now.Sub(v.Last) > 20*time.Minute {
				delete(m.CompanionHistory, k)
			}
		}
		if len(m.CompanionHistory) >= 256 {
			var oldest string
			var at time.Time
			for k, v := range m.CompanionHistory {
				if at.IsZero() || v.Last.Before(at) {
					oldest = k
					at = v.Last
				}
			}
			delete(m.CompanionHistory, oldest)
		}
	}
	m.CompanionHistory[key] = h
	ends := float64(now.UnixMilli())/1000 + 8
	observedOnly := true
	if h.Estimate > 0 && h.Estimate > delay {
		ends = float64(c.At.UnixMilli())/1000 + h.Estimate
		observedOnly = false
	}
	m.Rows[key] = Row{GUID: c.GUID, Player: c.Name, Spell: c.Spell, Name: name, Ends: ends, Delay: delay, Companion: true, ObservedOnly: observedOnly, Duration: h.Estimate}
	return true
}
func PreviewRows(now time.Time) []Row {
	return []Row{
		{GUID: "preview-1", Player: "Companion", Name: "Barrier", Duration: 40, Ends: float64(now.Unix()) + 25},
		{GUID: "preview-2", Player: "Teammate", Name: "Defensive", Duration: 120, Ends: float64(now.Unix()) + 90, Charges: true},
		{GUID: "preview-3", Player: "Companion", Name: "Interrupt", Ends: float64(now.Unix()) + 8, ObservedOnly: true},
	}
}
