package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestFocusRetentionIsOptInAndNeverRevivesStaleOrBrokenInput(t *testing.T) {
	now := time.Unix(1789400100, 0)
	f := displayFocus{lastVisible: now}
	if f.pause("wow_not_foreground", now) {
		t.Fatal("default retained overlay")
	}
	f.allow = true
	if f.pause("wow_not_foreground", now) {
		t.Fatal("missing layout revived")
	}
	f.lastVisible = now.Add(-2 * time.Second)
	if f.pause("wow_not_foreground", now) {
		t.Fatal("stale layout revived")
	}
	f.lastVisible = now
	if !f.pause("wow_not_foreground", now) || !f.pause("wow_not_foreground", now.Add(time.Hour)) {
		t.Fatal("opt-in expired on focus loss")
	}
	for _, reason := range []string{"bridge_checksum", "bridge_not_visible", "wrong_client_packet", "bridge_closed", "bridge_disabled", "bridge_moved"} {
		f.holding = true
		if f.pause(reason, now) || f.pause("wow_not_foreground", now) {
			t.Fatal("bad input survived", reason)
		}
	}
	f.allow = true
	f.holding = true
	f.observe(lookup.BridgeSample{}) // Old packets and an explicit opt-out both disable it.
	if f.allow || f.holding {
		t.Fatal("old packet enabled background")
	}
}

func TestBackgroundDisplayFromActualLuaDoesNotFabricateCapture(t *testing.T) {
	root := os.Getenv("LAMDA_KICK_PACKETS")
	if root == "" {
		t.Skip("set LAMDA_KICK_PACKETS to real Lua fixtures")
	}
	packet, err := os.ReadFile(filepath.Join(root, "background.bin"))
	if err != nil {
		t.Fatal(err)
	}
	sample := lookup.DecodeSample(packet)
	if sample.Error != "" || !sample.Event.KeepUIWhenUnfocused || sample.Kick == nil || sample.Frames == nil {
		t.Fatal(sample)
	}
	sample.Window = 123
	sample.WindowPID = 42
	now := sample.Kick.At
	c := engine.DefaultConfig()
	c.KickEnabled = true
	c.FrameEstimates = true
	m := engine.NewModel()
	input := observationInput{}
	input.handle(m, sample, now)
	guid := sample.Kick.Members[1].GUID
	m.Observe(engine.Cast{GUID: guid, Spell: 57994, At: now}, now)
	f := displayFocus{}
	f.observe(sample)
	view := func(at time.Time) engine.FrameDisplay {
		return f.view(m.Snapshot(at), c, sample.Frames, now, 123, sample.Kick, now, 123, at)
	}
	if d := view(now); !d.Enabled || d.Background {
		t.Fatal(d)
	}
	if !f.pause("wow_not_foreground", now.Add(time.Second)) {
		t.Fatal("fresh layout lost")
	}
	for _, seconds := range []int{2, 40, 300} {
		at := now.Add(time.Duration(seconds) * time.Second)
		d := view(at)
		if !d.Enabled || !d.Background || d.Status != "background_layout" || d.GeometryAge != float64(seconds) || d.Expires <= d.At || d.WindowPID != 42 {
			t.Fatal(d)
		}
		if !d.KickBars[0].Native || d.KickBars[1].Status == "unknown" {
			t.Fatal(d.KickBars)
		}
		if seconds == 2 && d.KickBars[1].Status != "estimated_cooldown" {
			t.Fatal(d.KickBars[1])
		}
		if seconds >= 40 && d.KickBars[1].Status != "estimated_ready" {
			t.Fatal(d.KickBars[1])
		}
	}
	if !sample.Kick.At.Equal(now) || !sample.Frames.At.Equal(now) || m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("retained layout changed input")
	}
	// Foreground recovery must require fresh geometry again, not extend the hold.
	f.observe(sample)
	if view(now.Add(301 * time.Second)).Enabled {
		t.Fatal("stale foreground data kept visible")
	}
	sample.Event.KeepUIWhenUnfocused = false
	f.observe(sample)
	if f.pause("wow_not_foreground", now) {
		t.Fatal("opt-out with same roster ID ignored")
	}
}
