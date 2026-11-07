package engine

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

const outdoorAstral = `9/15/2026 23:33:10.792-4  SPELL_CAST_SUCCESS,Player-1-00000001,"Shaman-Realm",0x10548,0x80000000,0000000000000000,nil,0x80000000,0x80000000,108271,"Astral Shift",0x1`
const outdoorLink = `9/15/2026 23:33:25.875-4  SPELL_CAST_SUCCESS,Player-1-00000001,"Shaman-Realm",0x10548,0x80000000,0000000000000000,nil,0x80000000,0x80000000,98008,"Spirit Link Totem",0x8`

func TestRosterMatchedShamanCastsReachFrameCountdowns(t *testing.T) {
	roster := map[string]bool{"Player-1-00000001": true}
	now, _, _ := RecordHeader(outdoorLink)
	now = now.Add(time.Second)
	m := NewModel()
	for _, line := range []string{outdoorAstral, outdoorLink} {
		if _, err := Parse(line); err == nil {
			t.Fatal("unconfirmed outsider was accepted")
		}
		if !m.IngestWithRoster(line, now, false, roster) {
			t.Fatal("confirmed party cast missing", line)
		}
	}
	g := &framewire.Snapshot{At: now, Player: "Player-1-SELF", Width: 1920, Height: 1080,
		Size: 41, Spacing: 2, PerRow: 6, Maximum: 6, Opacity: 100, Anchor: 1,
		Party: true, Unknown: true, Persistent: true,
		Frames: []framewire.Rect{{GUID: "Player-1-00000001", X: 680, Y: 400, Width: 150, Height: 64}}}
	cfg := DefaultConfig()
	cfg.FrameEstimates = true
	view := FrameView(m.Snapshot(now), cfg, g, now, now, 123)
	if len(view.Icons) != 2 || m.Diagnostics.AcceptedCasts != 2 {
		t.Fatal(view, m.Diagnostics)
	}
	for _, icon := range view.Icons {
		if (icon.Status != "estimated_cooldown" && icon.Status != "estimated_recharge") || icon.Source != "combat_log" || icon.Ends <= view.At ||
			!slices.Contains(icon.Reasons, "group_roster_match") || !slices.Contains(icon.Reasons, "reference_baseline") {
			t.Fatal(icon)
		}
		if icon.Spell == 108271 && (icon.Ends-view.At < 103 || icon.Ends-view.At > 104) {
			t.Fatal("Astral Shift started at receipt instead of cast", icon)
		}
		if icon.Spell == 98008 && icon.Ends-view.At != 179 {
			t.Fatal("Spirit Link countdown incorrect", icon)
		}
	}
}

func TestRosterFallbackDoesNotAcceptUnrelatedSourcesOrInventTimers(t *testing.T) {
	at, _, _ := RecordHeader(outdoorAstral)
	for name, roster := range map[string]map[string]bool{
		"unavailable": nil, "ungrouped": {}, "different player": {"Player-1-OTHER": true},
		"false membership": {"Player-1-00000001": false},
	} {
		t.Run(name, func(t *testing.T) {
			m := NewModel()
			if m.IngestWithRoster(outdoorAstral, at, true, roster) || len(m.Snapshot(at).Cooldowns) != 0 {
				t.Fatal("outsider became party evidence")
			}
		})
	}
	roster := map[string]bool{"Player-1-00000001": true}
	for _, line := range []string{
		strings.Replace(outdoorAstral, "Player-1-00000001", "Player-104-DIFFERENT", 1),
		strings.Replace(outdoorAstral, "SPELL_CAST_SUCCESS", "SPELL_AURA_APPLIED", 1),
		strings.Replace(outdoorAstral, "0x10548", "invalid", 1),
	} {
		if NewModel().IngestWithRoster(line, at, true, roster) {
			t.Fatal("invalid/noncast source accepted", line)
		}
	}
	pet := strings.Replace(outdoorAstral, "Player-1-00000001", "Pet-104-0C71695B", 1)
	if NewModel().IngestWithRoster(pet, at, true, map[string]bool{"Pet-104-0C71695B": true}) {
		t.Fatal("roster broadened NPC/companion filtering")
	}
	m := NewModel()
	now := at.Add(154 * time.Second)
	if m.IngestWithRoster(outdoorAstral, now, false, roster) {
		t.Fatal("late cast became an active countdown")
	}
	e := m.Snapshot(now).Cooldowns[0]
	if e.EstimatedEnd != float64(at.UnixMilli())/1000+120 || e.DeliverySeconds != 154 ||
		!slices.Contains(e.Reasons, "arrived_after_estimated_end") || e.Status == "estimated_cooldown" {
		t.Fatal(e)
	}
	if m.IngestWithRoster(outdoorAstral, now, false, roster) || m.Diagnostics.Duplicates != 1 {
		t.Fatal("duplicate cast restarted timer")
	}
	// Existing friendly log evidence needs no external roster or lookup service.
	if !NewModel().IngestWithRoster(strings.Replace(outdoorAstral, "0x10548", "0x512", 1), at, false, nil) {
		t.Fatal("ordinary party tracking regressed")
	}
}
