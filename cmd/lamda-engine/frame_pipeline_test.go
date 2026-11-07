package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Uses the real addon packet after converting the user's six-slot left layout
// (41px, +83px compensation, four learned spells). Engine rows of different
// lengths must share the native row's near edge rather than its empty far slots.
func TestCooldownGrowthFromActualLuaPacket(t *testing.T) {
	path := os.Getenv("LAMDA_GROWTH_TEST_PACKET")
	if path == "" {
		t.Skip("set LAMDA_GROWTH_TEST_PACKET to the addon test output")
	}
	packet, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sample := lookup.DecodeSample(packet)
	if sample.Error != "" || sample.Frames == nil {
		t.Fatal(sample)
	}
	g := sample.Frames
	now := g.At
	if g.PartyLayout == nil || g.PartyLayout.Growth != 1 || g.PartyLayout.OffsetX != -3 {
		t.Fatal(g)
	}
	c := engine.DefaultConfig()
	c.FrameEstimates = true
	c.FrameSpells = nil
	for _, count := range []int{1, 2, 4} {
		s := engine.InferenceSnapshot{}
		for i := 0; i < count; i++ {
			s.Cooldowns = append(s.Cooldowns, engine.CooldownEvidence{GUID: "Player-1-BOB", Spell: 100 + i, Status: "unknown"})
		}
		s.Cooldowns = append(s.Cooldowns, engine.CooldownEvidence{GUID: g.Player, Spell: 374227, Status: "unknown"})
		view := engine.FrameView(s, c, g, now, now, 123)
		if len(view.Icons) != count || view.NativeHidden != 1 {
			t.Fatal(view)
		}
		for i, icon := range view.Icons {
			if icon.GUID != "Player-1-BOB" || icon.X != 756-i*43 || icon.Y != 500 {
				t.Fatal(i, icon)
			}
		}
	}
}

// LAMDA_NATIVE_TEST_PACKETS optionally supplies packets produced by the actual
// Lua native widgets/exporter in test_frame_bridge.py, including recovery.
func TestNativeCooldownPriorityAcrossPixelPacketAndEngine(t *testing.T) {
	m := engine.NewModel()
	cfg := engine.DefaultConfig()
	cfg.FrameEstimates = true
	var now time.Time
	for i, phase := range []string{"native", "pending_refresh", "delayed_refresh", "fallback", "recovered"} {
		owned := "|374227"
		if phase == "fallback" {
			owned = ""
		}
		body := []byte("1:1|1789400100|roster|0\n\n1:2|1789400100|1600|1000|Player-1-ALICE|32|2|6|6|95|2|8|0|0|1|1|0|1|0\nPlayer-1-ALICE|100|400|200|40" + owned + "\nPlayer-1-BOB|100|500|200|40")
		packet := make([]byte, lookup.BridgeBytes)
		copy(packet, "LUI2")
		binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
		binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
		copy(packet[10:], body)
		if folder := os.Getenv("LAMDA_NATIVE_TEST_PACKETS"); folder != "" {
			var err error
			packet, err = os.ReadFile(filepath.Join(folder, phase+".bin"))
			if err != nil {
				t.Fatal(err)
			}
		}
		sample := lookup.DecodeSample(packet)
		if sample.Error != "" || sample.Frames == nil {
			t.Fatal(phase, sample)
		}
		now = sample.Frames.At
		if i == 0 {
			for _, guid := range []string{sample.Frames.Player, "Player-1-BOB"} {
				m.Observe(engine.Cast{At: now, GUID: guid, Spell: 374227, Self: guid == sample.Frames.Player}, now)
			}
		}
		view := engine.FrameView(m.Snapshot(now), cfg, sample.Frames, now, now, 123)
		expected := 1
		if phase == "fallback" {
			expected = 2
		}
		if !view.Enabled || len(view.Icons) != expected || m.Diagnostics.AcceptedCasts != 2 {
			t.Fatal(phase, view, m.Diagnostics)
		}
		if phase != "fallback" && view.Icons[0].GUID != "Player-1-BOB" {
			t.Fatal("suppressed teammate", view)
		}
	}
}

// Optionally consumes the real Lua-generated packet. The same decoded public
// GUIDs join a synthetic combat log cast to output consumed by the WPF renderer.
func TestFramePipelinePacketLogAndVisibleState(t *testing.T) {
	body := []byte("1:1|1789400100|roster|0\nPlayer-1-ALICE|us|Area 52|Alice\n\n1:2|1789400100|1600|1000|Player-1-ALICE|32|2|6|6|95|2|8|0|0|1|1|0|1|0\nPlayer-1-ALICE|100|400|200|40\nPlayer-1-BOB|100|500|200|40")
	packet := make([]byte, lookup.BridgeBytes)
	copy(packet, "LUI2")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
	copy(packet[10:], body)
	if path := os.Getenv("LAMDA_FRAME_TEST_PACKET"); path != "" {
		var err error
		packet, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	sample := lookup.DecodeSample(packet)
	if sample.Error != "" || sample.Frames == nil || len(sample.Frames.Frames) < 2 {
		t.Fatal(sample)
	}
	now := sample.Frames.At.Add(200 * time.Millisecond)
	model := engine.NewModel()
	cfg := engine.DefaultConfig()
	cfg.FrameEstimates = true
	for _, cast := range []struct {
		guid, flags string
		spell       int
		name        string
	}{{sample.Frames.Player, "0x511", 55342, "Mirror Image"}, {"Player-1-BOB", "0x512", 363916, "Obsidian Scales"}} {
		line := fmt.Sprintf("%s  SPELL_CAST_SUCCESS,%s,\"Test\",%s,0x0,0000000000000000,nil,0x0,0x0,%d,\"%s\",0x1", now.Add(-time.Second).Format("1/2/2006 15:04:05.000"), cast.guid, cast.flags, cast.spell, cast.name)
		model.Ingest(line, now, false)
	}
	view := engine.FrameView(model.Snapshot(now), cfg, sample.Frames, now, now, 123)
	if !view.Enabled || len(view.Icons) != 2 || model.Diagnostics.AcceptedCasts != 2 {
		t.Fatal(view, model.Diagnostics)
	}
	for _, icon := range view.Icons {
		var matched bool
		for _, frame := range sample.Frames.Frames {
			if icon.GUID == frame.GUID && icon.X == frame.X && icon.Y < frame.Y {
				matched = true
				break
			}
		}
		if !matched || icon.Source != "combat_log" || icon.Ends <= view.At {
			t.Fatal("incorrect observation/frame output", icon)
		}
	}
	if path := os.Getenv("LAMDA_FRAME_TEST_STATE"); path != "" {
		if err := engine.WriteJSON(path, struct {
			PID          int                 `json:"pid"`
			FrameDisplay engine.FrameDisplay `json:"frameDisplay"`
		}{123, view}); err != nil {
			t.Fatal(err)
		}
	}
	packet[20] ^= 1
	if broken := lookup.DecodeSample(packet); broken.Error == "" || broken.Frames != nil {
		t.Fatal("corrupt source retained frame state", broken)
	}
}
