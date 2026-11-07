package lookup

import (
	"sort"
	"testing"
)

func TestExpandedRosterValidationAliasesAndUnknownIdentity(t *testing.T) {
	tokens := []string{}
	for _, s := range []string{"party", "raid", "boss", "arena", "nameplates"} {
		tokens = append(tokens, OverlayScopeTokens(s)...)
	}
	sort.Strings(tokens)
	fixture := func() *OverlayInput {
		p := &OverlayInput{UnitsVersion: 2, UnitTokens: append([]string{}, tokens...), UnitHash: OverlayUnitHash(tokens)}
		for _, u := range tokens {
			p.Units = append(p.Units, OverlayUnit{Unit: u, Generation: 7, ID: "Slot-" + u + "-7"})
		}
		return p
	}
	p := fixture()
	sanitizeOverlayUnits(p)
	if len(p.Units) != 98 || len(p.UnitTokens) != 98 {
		t.Fatal("full roster lost")
	}
	p = &OverlayInput{UnitsVersion: 2, UnitTokens: []string{"party1", "player", "raid1"}, Units: []OverlayUnit{{Unit: "party1", ID: "Player-1-A", Generation: 1}, {Unit: "player", ID: "Player-1-A", Generation: 1}, {Unit: "raid1", ID: "Player-1-A", Generation: 1}}}
	p.UnitHash = OverlayUnitHash(p.UnitTokens)
	sanitizeOverlayUnits(p)
	if len(p.Units) != 3 {
		t.Fatal("cross-token aliases discarded", p.Units)
	}
	for _, bad := range []func(*OverlayInput){func(p *OverlayInput) { p.UnitHash = "wrong" }, func(p *OverlayInput) { p.UnitTokens[0] = "target"; p.UnitHash = OverlayUnitHash(p.UnitTokens) }, func(p *OverlayInput) { p.UnitTokens[1] = p.UnitTokens[0]; p.UnitHash = OverlayUnitHash(p.UnitTokens) }, func(p *OverlayInput) {
		p.UnitTokens[0], p.UnitTokens[1] = p.UnitTokens[1], p.UnitTokens[0]
		p.UnitHash = OverlayUnitHash(p.UnitTokens)
	}, func(p *OverlayInput) { p.UnitsVersion = 3 }} {
		p = fixture()
		bad(p)
		sanitizeOverlayUnits(p)
		if p.UnitsVersion != 0 || len(p.Units) != 0 {
			t.Fatal("bad config accepted")
		}
	}
	p = fixture()
	p.Units[0].ID = "Slot-arena1-6"
	p.Units[1].ID = "Slot-arena1-7"
	p.Units[2].ID = "GameObject-1-X"
	p.Units[3].Generation = 0
	sanitizeOverlayUnits(p)
	if len(p.Units) != 94 {
		t.Fatal("invalid identity trusted", len(p.Units))
	}
	for scope, count := range map[string]int{"party": 5, "group": 45, "raid": 40, "boss": 8, "arena": 5, "nameplates": 40} {
		if n := len(OverlayScopeTokens(scope)); n != count {
			t.Fatal(scope, n)
		}
	}
	if OverlayScopeContains("group", "party1", "raid") || !OverlayScopeContains("group", "raid40", "raid") || OverlayScopeContains("raid", "raid41", "raid") {
		t.Fatal("group/token bounds")
	}
	if OverlayUnitHash(nil) != OverlayUnitHash([]string{}) {
		t.Fatal("empty config hash")
	}
}
