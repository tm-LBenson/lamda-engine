package lookup

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestIndependentPacketIdentityAndPayload(t *testing.T) {
	event := "1:1|1789400000|roster|0\nPlayer-1-ALICE|us|Area 52|Alice"
	frames := "1:1|1789400000|1600|1000|Player-1-ALICE|32|2|6|6|95|2|8|0|0|1|1|0|1|0\nPlayer-1-ALICE|100|400|200|40"
	kick := "1:1|1789400000|1600|1000|Player-1-ALICE|2|1|400|200|240|28|26|95|4|4|0|0|1|0|0\nPlayer-1-ALICE|6|250||"
	for _, tc := range []struct {
		magic, source, geometry, kicks string
		forever                        bool
	}{
		{"LCD1", "cd", frames, "", false}, {"FCD1", "cd", frames, "", true},
		{"LKI1", "kick", "", kick, false}, {"FKI1", "kick", "", kick, true},
	} {
		body := strings.Join([]string{event, tc.geometry, "", tc.kicks}, "\n\n")
		sample := DecodeSample(simPacketFixture(tc.magic, BridgeBytes, []byte(body)))
		if sample.Error != "" || sample.Source != tc.source || sample.Forever != tc.forever {
			t.Fatalf("%s: %+v", tc.magic, sample)
		}
		contaminated := strings.Join([]string{event, frames, "", kick}, "\n\n")
		bad := DecodeSample(simPacketFixture(tc.magic, BridgeBytes, []byte(contaminated)))
		if bad.Error != "bridge_module_payload" {
			t.Fatalf("cross-module payload accepted: %+v", bad)
		}
	}
}

func TestReadersKeepModuleAndPositionThroughFailureAndOverlap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputs := map[string]chan BridgeSample{"cd": make(chan BridgeSample, 2), "kick": make(chan BridgeSample, 2)}
	targets := []BridgeTarget{{Source: "cd", Position: BridgePosition{30, 80}}, {Source: "kick", Position: BridgePosition{1200, 700}}}
	out := readModuleBridges(ctx, targets, func(_ context.Context, target BridgeTarget) <-chan BridgeSample { return inputs[target.Source] })
	inputs["cd"] <- BridgeSample{Error: "bridge_not_visible"}
	inputs["kick"] <- BridgeSample{Source: "kick", Window: 41, Event: Event{ID: "still-running"}}
	got := map[string]BridgeSample{}
	for len(got) < 2 {
		select {
		case s := <-out:
			got[s.Source] = s
		case <-time.After(time.Second):
			t.Fatal("reader stalled")
		}
	}
	if got["cd"].Error != "bridge_not_visible" || got["kick"].Error != "" || got["kick"].Event.ID != "still-running" || got["kick"].Position != targets[1].Position {
		t.Fatal(got)
	}
	inputs["cd"] <- BridgeSample{Source: "kick", Window: 41, Event: Event{ID: "overlapping-strip"}}
	select {
	case s := <-out:
		if s.Source != "cd" || s.Error != "bridge_wrong_module" || s.Event.ID != "" {
			t.Fatal(s)
		}
	case <-time.After(time.Second):
		t.Fatal("reader stalled")
	}
}

func TestFirstReloadProbePreservesNewIdentity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := make(chan BridgeSample, 1)
	input <- BridgeSample{Source: "cd"}
	out := readModuleBridges(ctx, []BridgeTarget{{}}, func(context.Context, BridgeTarget) <-chan BridgeSample { return input })
	select {
	case s := <-out:
		if s.Source != "cd" || s.Error != "" {
			t.Fatal(s)
		}
	case <-time.After(time.Second):
		t.Fatal("reader stalled")
	}
}

func TestShareControlKeepsDecodedPositionThroughMultiplexer(t *testing.T) {
	for _, reason := range []string{"", "bridge_strip_occluded"} {
		t.Run(reason, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := make(chan BridgeSample, 1)
			actual := BridgePosition{130, 24}
			legacy := BridgePosition{288, 224}
			input <- BridgeSample{Source: "augmentation", Position: actual, Error: reason, NativeMeter: &NativeMeter{ControlOnly: true, ControlWidth: 64, ControlHeight: 8}}
			out := readModuleBridges(ctx, []BridgeTarget{{Source: "augmentation", Position: legacy}}, func(context.Context, BridgeTarget) <-chan BridgeSample { return input })
			select {
			case s := <-out:
				if s.Position != actual || s.Error != reason || s.NativeMeter.ControlWidth != 64 {
					t.Fatal("share metadata geometry replaced by legacy strip origin", s)
				}
			case <-time.After(time.Second):
				t.Fatal("reader stalled")
			}
		})
	}
}
