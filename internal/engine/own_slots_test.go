package engine

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestOwnSlotsPreventPublicBuildExtraRowAndReplaceOnlyMissingNative(t *testing.T) {
	now := time.Now()
	g := frameFixture(now)
	g.Persistent = true
	self := &g.Frames[1]
	self.SelfSlots = []framewire.CooldownSlot{
		{X: 56, Y: 410, Size: 41, Opacity: 100, Spells: []int{363916}},
		{X: 13, Y: 410, Size: 41, Opacity: 100, Spells: []int{374227}},
	}
	self.NativeSpells = []int{363916, 374227}
	s := knownFixture(now)
	public := s.PublicBuilds[0]
	public.Character.GUID = g.Player
	public.Talents = append(public.Talents, lookup.Talent{NodeID: 5, EntryID: 50, SpellID: 357170, Rank: 1}, lookup.Talent{NodeID: 6, EntryID: 60, SpellID: 363534, Rank: 1})
	s.PublicBuilds = append(s.PublicBuilds, public)
	cfg := DefaultConfig()
	cfg.FrameEstimates = true
	view := FrameView(s, cfg, g, now, now, 123)
	if len(view.Icons) != 2 || view.NativeHidden != 2 {
		t.Fatal(view)
	}
	for _, icon := range view.Icons {
		if icon.GUID == g.Player {
			t.Fatal("cached build added an own row", icon)
		}
	}
	self.NativeSpells = []int{374227}
	s.Cooldowns = []CooldownEvidence{{GUID: g.Player, Spell: 363916, Status: "estimated_recharge", Source: "combat_log", EstimatedEnd: float64(now.Unix()) + 60, BaselineSeconds: 90}}
	view = FrameView(s, cfg, g, now, now, 123)
	if len(view.Icons) != 3 {
		t.Fatal(view)
	}
	own := view.Icons[2]
	if own.GUID != g.Player || own.Spell != 363916 || own.X != 56 || own.Y != 410 || own.Size != 41 || own.Opacity != 100 {
		t.Fatal("fallback moved out of the missing native slot", own)
	}
	self.SelfSlots = []framewire.CooldownSlot{}
	if view = FrameView(s, cfg, g, now, now, 123); len(view.Icons) != 2 {
		t.Fatal("empty selection retained own icons", view)
	}
}

func TestOwnSlotsAliasesUnknownAndIndependentVisibility(t *testing.T) {
	now := time.Now()
	g := frameFixture(now)
	g.Persistent = true
	g.Frames = g.Frames[1:]
	g.Frames[0].SelfSlots = []framewire.CooldownSlot{{X: 56, Y: 410, Size: 41, Opacity: 90, Spells: []int{363916, 999999}}}
	cfg := DefaultConfig()
	cfg.FrameEstimates = true
	cfg.FrameSpells = []int{999999}
	s := InferenceSnapshot{}
	view := FrameView(s, cfg, g, now, now, 123)
	if len(view.Icons) != 1 || view.Icons[0].Source != "player_spellbook" || view.Icons[0].Status != "unknown" || view.Icons[0].Ends != 0 {
		t.Fatal(view)
	}
	s.Cooldowns = []CooldownEvidence{
		{GUID: g.Player, Spell: 999999, Source: "combat_log", Status: "estimated_cooldown", EstimatedEnd: float64(now.Unix()) + 10},
		{GUID: g.Player, Spell: 363916, Status: "unknown"},
	}
	view = FrameView(s, cfg, g, now, now, 123)
	if len(view.Icons) != 1 || view.Icons[0].Spell != 999999 {
		t.Fatal("alias duplicated or displaced the active slot", view)
	}
	g.Self = false
	if view = FrameView(s, cfg, g, now, now, 123); len(view.Icons) != 0 {
		t.Fatal(view)
	}
	g.Self = true
	cfg.FrameSpells = []int{}
	if view = FrameView(s, cfg, g, now, now, 123); len(view.Icons) != 0 {
		t.Fatal(view)
	}
	cfg.FrameSpells = nil
	g.Unknown = false
	if view = FrameView(InferenceSnapshot{}, cfg, g, now, now, 123); len(view.Icons) != 0 {
		t.Fatal(view)
	}
}
