package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestLiveInputFreshMembershipChunksAndReplay(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", Kind: "roster", At: now, Members: []lookup.Character{member}}, Observations: &lookup.Observations{}}
	m := engine.NewModel()
	input := observationInput{}
	part := lookup.BuildChunk{ID: "1:1", GUID: member.GUID, Game: "12.1.0", At: now.Add(-time.Second), Spec: 264, Part: 2, Total: 2, Data: "2,102,1,381647"}
	sample.Observations.Builds = []lookup.BuildChunk{part}
	input.handle(m, sample, now)
	if len(m.InspectedBuilds) != 0 {
		t.Fatal("partial build accepted")
	}
	part.Part = 1
	part.Data = "1,101,1,108271"
	sample.Observations.Builds = []lookup.BuildChunk{part}
	sample.Observations.Casts = []lookup.LiveCast{{ID: "Cast-one", GUID: member.GUID, Spell: 108271, AgeMillis: 100}}
	input.handle(m, sample, now)
	e := m.Snapshot(now).Cooldowns[0]
	if e.CooldownSeconds != 90 || e.Source != "unit_spellcast" || len(m.InspectedBuilds) != 1 {
		t.Fatal(e)
	}
	// A sampled copy or post-zone resend does not slide the timer forward.
	input.handle(m, sample, now.Add(time.Second))
	if m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal("duplicate cast accepted")
	}
	m.Reset()
	input.handle(m, sample, now.Add(time.Second))
	if len(m.Evidence) != 0 {
		t.Fatal("old cast revived after reset")
	}
	sample.Event.Members = nil
	sample.Observations.Casts[0].ID = "Cast-two"
	input.handle(m, sample, now)
	if len(m.Evidence) != 0 {
		t.Fatal("departed member accepted")
	}
	sample.Event.Members = []lookup.Character{member}
	input.handle(m, sample, now.Add(4*time.Second))
	if len(m.Evidence) != 0 {
		t.Fatal("frozen strip accepted")
	}
}
func TestInspectChunkCorruptionDoesNotOverwriteCompleteBuild(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	member := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	input := observationInput{}
	m := engine.NewModel()
	sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", Kind: "roster", At: now, Members: []lookup.Character{member}}, Observations: &lookup.Observations{Builds: []lookup.BuildChunk{{ID: "1:1", GUID: member.GUID, Game: "12.1.0", At: now, Spec: 264, Part: 1, Total: 1, Data: "1,101,1,108271"}}}}
	input.handle(m, sample, now)
	before := m.InspectedBuilds[member.GUID].Revision
	part := &sample.Observations.Builds[0]
	part.ID = "1:2"
	part.At = now.Add(time.Second)
	part.Data = "1,101,1,108271;1,102,1,381647"
	input.handle(m, sample, now.Add(time.Second))
	if m.InspectedBuilds[member.GUID].Revision != before {
		t.Fatal("corrupt build replaced complete one")
	}
}

func TestLiveKeyCompletionKeepsPartyButDepartureInvalidates(t *testing.T) {
	start := time.Now().Truncate(time.Second)
	bob := lookup.Character{GUID: "Player-1-BOB", Name: "Bob", Realm: "Draenor", Region: "us"}
	self := lookup.Character{GUID: "Player-1-SELF", Name: "Self", Realm: "Draenor", Region: "us"}
	for _, end := range []string{"departure", "challenge_end"} {
		t.Run(end, func(t *testing.T) {
			input := observationInput{}
			m := engine.NewModel()
			sample := lookup.BridgeSample{Event: lookup.Event{ID: "1:1", Kind: "challenge_start", At: start, Deadline: start, Members: []lookup.Character{self, bob}}, Observations: &lookup.Observations{}}
			input.handle(m, sample, start)
			castAt := start.Add(time.Second)
			m.Observe(engine.Cast{GUID: bob.GUID, Spell: 57994, At: castAt}, castAt.Add(49*time.Second))
			now := start.Add(10 * time.Minute)
			if e := m.Snapshot(now).Cooldowns[0]; e.Status != "estimated_ready" {
				t.Fatal("same-key delayed cast did not become estimated ready", e)
			}
			sample.Event.ID, sample.Event.At, sample.Event.Kind = "1:2", now, "roster"
			if end == "departure" {
				sample.Event.Members = []lookup.Character{self}
			} else {
				sample.Event.Kind = "challenge_end"
			}
			input.handle(m, sample, now)
			s := m.Snapshot(now)
			want := "estimated_ready"
			if end == "departure" {
				want = "unknown"
			}
			if e := s.Cooldowns[0]; e.Status != want {
				t.Fatal("party continuity ignored", e)
			}
			if end == "challenge_end" && (!s.PartySince[bob.GUID].Equal(start) || m.FreshKey != nil) {
				t.Fatal("completion restarted party or retained ended key", s.PartySince)
			}
		})
	}
}
func TestLiveTrackingFromActualLuaPackets(t *testing.T) {
	folder := os.Getenv("LAMDA_OBSERVATION_PACKETS")
	if folder == "" {
		t.Skip("set LAMDA_OBSERVATION_PACKETS to actual Lua samples")
	}
	input := observationInput{}
	m := engine.NewModel()
	var first time.Time
	var latest lookup.BridgeSample
	var now time.Time
	for i := 0; i < 8; i++ {
		bytes, err := os.ReadFile(filepath.Join(folder, fmt.Sprintf("%d.bin", i)))
		if err != nil {
			t.Fatal(err)
		}
		sample := lookup.DecodeSample(bytes)
		if sample.Error != "" || sample.Frames == nil || sample.Observations == nil {
			t.Fatal(sample)
		}
		if i == 0 {
			first = sample.Event.At
		}
		now = first.Add(time.Duration(float64(i)*750)*time.Millisecond + 150*time.Millisecond)
		input.handle(m, sample, now)
		latest = sample
	}
	if len(m.InspectedBuilds) != 2 || m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal(m.Diagnostics, len(m.InspectedBuilds))
	}
	snap := m.Snapshot(now)
	if len(snap.Cooldowns) != 1 || snap.Cooldowns[0].Source != "unit_spellcast" || snap.Cooldowns[0].CooldownSeconds != 90 {
		t.Fatal(snap)
	}
	cfg := engine.DefaultConfig()
	cfg.FrameEstimates = true
	view := engine.FrameView(snap, cfg, latest.Frames, now, now, 123)
	found := false
	for _, icon := range view.Icons {
		if icon.GUID == "Player-1-BOB" && icon.Spell == 108271 && icon.Ends > view.At && icon.Source == "unit_spellcast" {
			found = true
		}
	}
	if !found {
		t.Fatal("live cast did not reach teammate frame", view)
	}
	if path := os.Getenv("LAMDA_OBSERVATION_STATE"); path != "" {
		if err := engine.WriteJSON(path, struct {
			PID          int                      `json:"pid"`
			FrameDisplay engine.FrameDisplay      `json:"frameDisplay"`
			Inference    engine.InferenceSnapshot `json:"inference"`
		}{123, view, snap}); err != nil {
			t.Fatal(err)
		}
	}
}
