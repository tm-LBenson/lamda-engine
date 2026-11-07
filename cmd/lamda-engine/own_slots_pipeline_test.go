package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestOwnCooldownSlotsFromActualLuaPackets(t *testing.T) {
	folder := os.Getenv("LAMDA_OWN_SLOT_PACKETS")
	if folder == "" {
		t.Skip("set LAMDA_OWN_SLOT_PACKETS to Lua packet output")
	}
	for _, phase := range []string{"native", "fallback"} {
		data, err := os.ReadFile(filepath.Join(folder, phase+".bin"))
		if err != nil {
			t.Fatal(err)
		}
		sample := lookup.DecodeSample(data)
		if sample.Error != "" || sample.Frames == nil {
			t.Fatal(sample)
		}
		g := sample.Frames
		now := g.At
		model := engine.NewModel()
		for _, spell := range []int{363916, 374227} {
			model.Observe(engine.Cast{At: now, GUID: g.Player, Spell: spell, Self: true}, now)
		}
		s := model.Snapshot(now)
		s.PublicBuilds = []lookup.Profile{{Character: lookup.Character{GUID: g.Player, Name: "Alice", Realm: "Realm", Region: "us"},
			FetchedAt: now, Talents: []lookup.Talent{{SpellID: 357170, Rank: 1}, {SpellID: 363534, Rank: 1}}}}
		cfg := engine.DefaultConfig()
		cfg.FrameEstimates = true
		view := engine.FrameView(s, cfg, g, now, now, 123)
		if phase == "native" {
			if len(view.Icons) != 0 || view.NativeHidden != 2 {
				t.Fatal("upper row survived", view)
			}
		} else {
			if len(view.Icons) != 1 {
				t.Fatal(view)
			}
			icon := view.Icons[0]
			if icon.Spell != 363916 || icon.X != 756 || icon.Y != 710 || icon.Size != 41 || icon.Opacity != 100 || icon.Ends <= view.At {
				t.Fatal("fallback was not in the exact native slot", icon)
			}
			if path := os.Getenv("LAMDA_OWN_SLOT_STATE"); path != "" {
				if err := engine.WriteJSON(path, struct {
					PID          int                 `json:"pid"`
					FrameDisplay engine.FrameDisplay `json:"frameDisplay"`
				}{123, view}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
