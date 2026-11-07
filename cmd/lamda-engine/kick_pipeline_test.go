package main

import (
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLuaKickPacketToDisplay(t *testing.T) {
	root := os.Getenv("LAMDA_KICK_PACKETS")
	if root == "" {
		t.Skip("set LAMDA_KICK_PACKETS to actual Lua fixture folder")
	}
	for _, file := range []string{"both.bin", "bars-only.bin"} {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, file))
			if err != nil {
				t.Fatal(err)
			}
			sample := lookup.DecodeSample(data)
			if sample.Error != "" || sample.Kick == nil || len(sample.Kick.Members) != 5 || sample.Observations == nil {
				t.Fatal(sample)
			}
			k := sample.Kick
			now := k.At
			input := &observationInput{}
			model := engine.NewModel()
			input.handle(model, sample, now)
			input.handle(model, sample, now.Add(250*time.Millisecond))
			c := engine.DefaultConfig()
			c.KickEnabled = true
			display := engine.KickView(model.Snapshot(now), c, k, now, now, 123)
			if k.OwnSpell != -1 || !display.Enabled || len(display.KickBars) != 4 {
				t.Fatal(display)
			}
			found := false
			for _, bar := range display.KickBars {
				if bar.GUID == "Player-1-BOB" {
					found = true
					// The Lua fixture supplies a complete Restoration inspection;
					// its pinned baseline is 30s, not the uninspected 12s fallback.
					if bar.Player != "Bob" || bar.Spell != 57994 || bar.Status != "estimated_cooldown" || bar.Ends-display.At != 30 {
						t.Fatal(bar)
					}
				}
			}
			if !found || model.Diagnostics.LiveCasts != 1 || model.Snapshot(now).InterruptAttempts["Player-1-BOB"] != 1 {
				t.Fatal("missing or duplicated live cast")
			}
			if file == "both.bin" && len(display.Icons) != 4 {
				t.Fatal("frame icons missing", display)
			}
			if file == "bars-only.bin" && (sample.Frames != nil || len(display.Icons) != 0) {
				t.Fatal("bars still depend on LamdaCD", display)
			}
			if out := os.Getenv("LAMDA_KICK_RENDER"); out != "" && file == "both.bin" {
				// Include a ready bar alongside observed and unknown states.
				display.KickBars[0].Status = "estimated_ready"
				v := struct {
					PID          int                 `json:"pid"`
					FrameDisplay engine.FrameDisplay `json:"frameDisplay"`
				}{12345, display}
				data, err = json.MarshalIndent(v, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(out, append(data, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestLuaOverlayTogglePreservesTracking(t *testing.T) {
	root := os.Getenv("LAMDA_KICK_PACKETS")
	if root == "" {
		t.Skip("set LAMDA_KICK_PACKETS to actual Lua fixture folder")
	}
	model := engine.NewModel()
	input := &observationInput{}
	config := engine.DefaultConfig()
	config.TrackingEnabled, config.FrameEstimates, config.LookupBridge, config.KickEnabled = true, true, true, true
	for _, name := range []string{"overlay-hidden.bin", "overlay-restored.bin"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sample := lookup.DecodeSample(data)
		if sample.Error != "" || sample.Frames == nil || sample.Kick == nil || sample.Observations == nil {
			t.Fatal(name, sample)
		}
		if len(sample.Kick.Members) != 2 || sample.Kick.Members[1].Pet != "Pet-1-DEMON" {
			t.Fatal("hiding must retain member and pet identities", sample.Kick)
		}
		now := sample.Kick.At
		input.handle(model, sample, now)
		if model.Diagnostics.LiveCasts != 2 {
			t.Fatal("hidden casts lost or repeated on restore", model.Diagnostics.LiveCasts)
		}
		snapshot := model.Snapshot(now)
		cd := engine.FrameView(snapshot, config, sample.Frames, now, now, 123)
		kick := engine.KickView(snapshot, config, sample.Kick, now, now, 123)
		display := engine.MergeKickDisplay(cd, kick)
		if name == "overlay-hidden.bin" {
			if len(display.Icons) != 0 || len(display.KickBars) != 0 || display.KickPanel != nil {
				t.Fatal("minimized overlay still draws content", display)
			}
			continue
		}
		if sample.Kick.OwnSpell != -1 || len(cd.Icons) == 0 || len(kick.Icons) != 1 || len(kick.KickBars) != 1 || kick.KickPanel == nil {
			t.Fatal("restore did not restore both modules", display)
		}
		found := false
		for _, bar := range kick.KickBars {
			if bar.GUID == "Player-1-BOB" {
				found = true
				if bar.Spell != 57994 || bar.Status != "estimated_cooldown" || bar.Ends-kick.At != 7 {
					t.Fatal("restore restarted or paused the countdown", bar)
				}
			}
		}
		if !found {
			t.Fatal("missing teammate after restore")
		}
	}
}

func TestLuaNativeOwnKickDoesNotReceiveEngineEstimate(t *testing.T) {
	root := os.Getenv("LAMDA_KICK_PACKETS")
	if root == "" {
		t.Skip("set LAMDA_KICK_PACKETS to actual Lua fixture folder")
	}
	data, err := os.ReadFile(filepath.Join(root, "own-native.bin"))
	if err != nil {
		t.Fatal(err)
	}
	sample := lookup.DecodeSample(data)
	if sample.Error != "" || sample.Kick == nil || sample.Kick.OwnSpell != 57994 {
		t.Fatal(sample)
	}
	now := sample.Kick.At
	model := engine.NewModel()
	input := &observationInput{}
	input.handle(model, sample, now)
	c := engine.DefaultConfig()
	c.KickEnabled = true
	d := engine.KickView(model.Snapshot(now), c, sample.Kick, now, now, 123)
	if len(d.KickBars) != 2 || !d.KickBars[0].Native || d.KickBars[0].GUID != sample.Kick.Player || len(d.Icons) != 1 {
		t.Fatal(d)
	}
	if d.Icons[0].GUID == sample.Kick.Player || model.Diagnostics.LiveCasts != 0 || len(model.Snapshot(now).InterruptAttempts) != 0 {
		t.Fatal("native timing became inferred evidence", d)
	}
}
