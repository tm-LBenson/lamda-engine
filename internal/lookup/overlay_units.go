package lookup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type OverlayUnit struct {
	Generation int    `json:"g,omitempty"`
	Unit       string `json:"unit"`
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Role       string `json:"role,omitempty"`
}

var overlayUnitID = regexp.MustCompile(`^Player-[A-Za-z0-9-]{1,100}$`)
var overlayRosterID = regexp.MustCompile(`^(Player|Creature|Pet|Vehicle)-[A-Za-z0-9-]{1,114}$`)

const OverlayRosterLimit = 98

func OverlayCloneScope(scope string) bool {
	switch scope {
	case "", "party", "group", "raid", "boss", "arena", "nameplates":
		return true
	}
	return false
}
func OverlayScopeTokens(scope string) []string {
	if scope == "" || scope == "party" {
		return []string{"player", "party1", "party2", "party3", "party4"}
	}
	if scope == "group" {
		return append(OverlayScopeTokens("party"), OverlayScopeTokens("raid")...)
	}
	count := map[string]int{"raid": 40, "boss": 8, "arena": 5, "nameplates": 40}[scope]
	prefix := scope
	if prefix == "nameplates" {
		prefix = "nameplate"
	}
	out := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		out = append(out, fmt.Sprintf("%s%d", prefix, i))
	}
	return out
}
func OverlayScopeContains(scope, unit, group string) bool {
	if scope == "group" {
		if group == "raid" {
			scope = "raid"
		} else {
			scope = "party"
		}
	}
	if scope == "" || scope == "party" {
		return PartyUnit(unit)
	}
	prefix := scope
	if scope == "nameplates" {
		prefix = "nameplate"
	}
	return OverlayCloneScope(scope) && OverlayUnitToken(unit) && strings.HasPrefix(unit, prefix)
}
func OverlayRosterUnit(unit string) bool {
	return PartyUnit(unit) || OverlayUnitToken(unit) && (strings.HasPrefix(unit, "raid") || strings.HasPrefix(unit, "boss") || strings.HasPrefix(unit, "arena") || strings.HasPrefix(unit, "nameplate"))
}
func OverlayUnitHash(tokens []string) string {
	if tokens == nil {
		tokens = []string{}
	}
	b, _ := json.Marshal(tokens)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func PartyUnit(unit string) bool {
	return unit == "player" || unit == "party1" || unit == "party2" || unit == "party3" || unit == "party4"
}
func sanitizeOverlayUnits(p *OverlayInput) {
	if p.UnitsVersion != 1 && p.UnitsVersion != 2 || len(p.Units) > OverlayRosterLimit || p.UnitsVersion == 1 && len(p.Units) > 5 {
		p.Units = nil
		p.UnitTokens = nil
		p.UnitsVersion = 0
		return
	}
	loaded := map[string]bool{}
	if p.UnitsVersion == 2 {
		valid := len(p.UnitTokens) <= OverlayRosterLimit && sort.StringsAreSorted(p.UnitTokens) && p.UnitHash == OverlayUnitHash(p.UnitTokens)
		for _, unit := range p.UnitTokens {
			if !OverlayRosterUnit(unit) || loaded[unit] {
				valid = false
			}
			loaded[unit] = true
		}
		if !valid {
			p.Units = nil
			p.UnitTokens = nil
			p.UnitsVersion = 0
			return
		}
	}
	seenUnits, seenIDs := map[string]bool{}, map[string]bool{}
	out := p.Units[:0]
	for _, u := range p.Units {
		valid := PartyUnit(u.Unit) && overlayUnitID.MatchString(u.ID) && !seenIDs[u.ID]
		if p.UnitsVersion == 2 {
			valid = loaded[u.Unit] && len(u.ID) <= 122 && u.Generation >= 1 && u.Generation <= 2147483647 && (overlayRosterID.MatchString(u.ID) || u.ID == fmt.Sprintf("Slot-%s-%d", u.Unit, u.Generation))
		}
		if !valid || seenUnits[u.Unit] {
			continue
		}
		if len(u.Name) > 120 {
			u.Name = ""
		}
		if u.Role != "tank" && u.Role != "healer" && u.Role != "damage" {
			u.Role = ""
		}
		out = append(out, u)
		seenUnits[u.Unit] = true
		seenIDs[u.ID] = true
	}
	p.Units = out
}
