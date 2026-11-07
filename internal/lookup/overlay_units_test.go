package lookup

import (
	"strings"
	"testing"
)

func TestOverlayUnitsValidation(t *testing.T) {
	p := &OverlayInput{UnitsVersion: 1, Units: []OverlayUnit{{Unit: "player", ID: "Player-1-A", Name: "Alice", Role: "healer"}, {Unit: "party1", ID: "Player-1-A"}, {Unit: "party2", ID: "Creature-1-B"}, {Unit: "party3", ID: "Player-1-C", Name: strings.Repeat("x", 121), Role: "unsupported"}, {Unit: "target", ID: "Player-1-D"}}}
	sanitizeOverlayUnits(p)
	if len(p.Units) != 2 || p.Units[1].Name != "" || p.Units[1].Role != "" {
		t.Fatal(p.Units)
	}
	p.UnitsVersion = 0
	sanitizeOverlayUnits(p)
	if len(p.Units) != 0 {
		t.Fatal("unknown version accepted")
	}
	p.UnitsVersion = 1
	p.Units = make([]OverlayUnit, 6)
	sanitizeOverlayUnits(p)
	if len(p.Units) != 0 {
		t.Fatal("oversized packet accepted")
	}
}
