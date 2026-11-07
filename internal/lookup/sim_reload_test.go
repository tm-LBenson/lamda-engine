package lookup

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func reloadPlatformFixture() ([]byte, *simReloadPlatform, *[][]simKeyEvent) {
	body := []byte("2|retail|req_1|Player-1-ABC|1473|nonce_1|1800000000|1800000015")
	calls := [][]simKeyEvent{}
	platform := &simReloadPlatform{
		now:         func() time.Time { return time.Unix(1800000000, 0) },
		foreground:  func() uintptr { return 42 },
		executable:  func(uintptr) string { return "game/Wow.exe" },
		matchesGame: func(exe, path, client string) bool { return exe == path+"/Wow.exe" && client == "retail" },
		capture:     func() BridgeSample { return BridgeSample{Window: 42, Reload: append([]byte(nil), body...)} },
		keyDown:     func(uint16) bool { return false },
		send: func(events []simKeyEvent) (int, error) {
			calls = append(calls, append([]simKeyEvent(nil), events...))
			return len(events), nil
		},
	}
	return body, platform, &calls
}

func TestSimReloadSendsOnlyFixedChordAfterFreshExactCapture(t *testing.T) {
	body, platform, calls := reloadPlatformFixture()
	if err := sendSimReload(context.Background(), 42, "game", "retail", body, *platform); err != nil {
		t.Fatal(err)
	}
	expected := [][]simKeyEvent{simReloadSequence}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("wrong key events: %#v", *calls)
	}
}

func TestSimReloadGuardsNeverSend(t *testing.T) {
	cases := map[string]func(*simReloadPlatform, []byte){
		"foreign foreground": func(p *simReloadPlatform, _ []byte) { p.foreground = func() uintptr { return 43 } },
		"foreign executable": func(p *simReloadPlatform, _ []byte) { p.executable = func(uintptr) string { return "another/Wow.exe" } },
		"capture error": func(p *simReloadPlatform, _ []byte) {
			p.capture = func() BridgeSample { return BridgeSample{Error: "bridge_checksum"} }
		},
		"capture foreign window": func(p *simReloadPlatform, b []byte) {
			p.capture = func() BridgeSample { return BridgeSample{Window: 43, Reload: b} }
		},
		"capture changed nonce": func(p *simReloadPlatform, b []byte) {
			p.capture = func() BridgeSample {
				return BridgeSample{Window: 42, Reload: append(append([]byte(nil), b...), byte('x'))}
			}
		},
		"sim instead of reload": func(p *simReloadPlatform, b []byte) {
			p.capture = func() BridgeSample { return BridgeSample{Window: 42, Sim: b} }
		},
		"lease expired during capture": func(p *simReloadPlatform, b []byte) {
			now := time.Unix(1800000000, 0)
			p.now = func() time.Time { return now }
			p.capture = func() BridgeSample {
				now = now.Add(6 * time.Second)
				return BridgeSample{Window: 42, Reload: b}
			}
		},
		"foreground changed after capture": func(p *simReloadPlatform, _ []byte) {
			n := 0
			p.foreground = func() uintptr {
				n++
				if n == 1 {
					return 42
				}
				return 43
			}
		},
		"executable changed after capture": func(p *simReloadPlatform, _ []byte) {
			n := 0
			p.executable = func(uintptr) string {
				n++
				if n == 1 {
					return "game/Wow.exe"
				}
				return "other"
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			body, p, calls := reloadPlatformFixture()
			change(p, body)
			if err := sendSimReload(context.Background(), 42, "game", "retail", body, *p); err == nil {
				t.Fatal("guard accepted")
			}
			if len(*calls) != 0 {
				t.Fatal("input sent despite guard")
			}
		})
	}
	for _, held := range simReloadHeldKeys {
		body, p, calls := reloadPlatformFixture()
		p.keyDown = func(key uint16) bool { return key == held }
		if err := sendSimReload(context.Background(), 42, "game", "retail", body, *p); err == nil || len(*calls) != 0 {
			t.Fatalf("held key %#x not blocked", held)
		}
	}
	body, p, calls := reloadPlatformFixture()
	ctx, cancel := context.WithCancel(context.Background())
	p.capture = func() BridgeSample { cancel(); return BridgeSample{Window: 42, Reload: body} }
	if err := sendSimReload(ctx, 42, "game", "retail", body, *p); !errors.Is(err, context.Canceled) || len(*calls) != 0 {
		t.Fatal("cancelled context sent input", err)
	}
}

func TestSimReloadPartialInputOnlyReleasesChord(t *testing.T) {
	for count := 0; count < len(simReloadSequence); count++ {
		body, p, calls := reloadPlatformFixture()
		p.send = func(events []simKeyEvent) (int, error) {
			*calls = append(*calls, append([]simKeyEvent(nil), events...))
			return count, errors.New("blocked")
		}
		if err := sendSimReload(context.Background(), 42, "game", "retail", body, *p); err == nil {
			t.Fatal("incomplete send treated as success")
		}
		expected := 1
		if count > 0 {
			expected = 2
		}
		if len(*calls) != expected {
			t.Fatal("unexpected retry count")
		}
		if count > 0 && !reflect.DeepEqual((*calls)[1], simReloadCleanup) {
			t.Fatal("partial send retried down or arbitrary input")
		}
	}
}

func TestReloadBridgeStaysSeparateFromSimAndTracking(t *testing.T) {
	body, _, _ := reloadPlatformFixture()
	p := simPacketFixture("LUR2", SimBridgeBytes, body)
	sample := DecodeSample(p)
	if sample.Error != "" || string(sample.Reload) != string(body) || len(sample.Sim) != 0 || sample.Event.ID != "" || sample.Frames != nil || sample.Kick != nil {
		t.Fatal("reload crossed channels", sample.Error)
	}
	p[20] ^= 1
	if DecodeSample(p).Error != "bridge_checksum" {
		t.Fatal("corrupt reload accepted")
	}
	if DecodeSample(simPacketFixture("LUR2", BridgeBytes, body)).Error == "" {
		t.Fatal("narrow reload accepted")
	}
	oversized := make([]byte, 513)
	copy(oversized, "2|")
	if DecodeSample(simPacketFixture("LUR2", SimBridgeBytes, oversized)).Error == "" {
		t.Fatal("unbounded reload body accepted")
	}
	if DecodeSample(simPacketFixture("LUR2", SimBridgeBytes, []byte("1|bad"))).Error == "" {
		t.Fatal("wrong reload version accepted")
	}
}
