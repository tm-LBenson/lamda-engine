package lookup

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"os"
	"strings"
	"testing"
	"time"
)

func TestQueueLuaEncoderPacket(t *testing.T) {
	packet, err := os.ReadFile("testdata/queue-accept.bin")
	if err != nil {
		t.Fatal(err)
	}
	sample := DecodeSample(packet)
	if sample.Error != "" {
		t.Fatal(sample.Error)
	}
	r, err := ParseQueueRequest(sample.Queue, "retail", time.Unix(1800000037, 100000000))
	if err != nil || r.X != 800 || r.Y != 500 || r.Remaining != 3000 || !r.Due {
		t.Fatal(r, err)
	}
	marker, err := os.ReadFile("testdata/queue-accept-marker.bin")
	if err != nil {
		t.Fatal(err)
	}
	want := queueMarker(r.Nonce)
	if string(marker) != string(want[:]) {
		t.Fatal("Lua button marker differs from engine expectation")
	}
}

func queueBody(tick int) []byte {
	return []byte(fmt.Sprintf("1|retail|invitation_1|lfg|1800000000|%d|1920|1080|800|500|115|22|3000|1", tick))
}
func queueNow() time.Time               { return time.Unix(1800000000, 100000000) }
func queueSample(tick int) BridgeSample { return BridgeSample{Window: 42, Queue: queueBody(tick)} }

func TestQueuePacketAndLeaseValidation(t *testing.T) {
	body := queueBody(1)
	packet := make([]byte, BridgeBytes)
	copy(packet, "LUQ1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
	copy(packet[10:], body)
	sample := DecodeSample(packet)
	if sample.Error != "" || sample.Source != "queue" || string(sample.Queue) != string(body) {
		t.Fatal(sample)
	}
	packet[12] ^= 1
	if DecodeSample(packet).Error != "bridge_checksum" {
		t.Fatal("corrupt lease accepted")
	}
	if _, err := ParseQueueRequest(body, "retail", queueNow()); err != nil {
		t.Fatal(err)
	}
	bad := []struct {
		index int
		value string
	}{{0, "2"}, {1, "forever"}, {2, "unsafe;code"}, {3, "trade"}, {4, "1799999997"}, {4, "1800000002"}, {5, "0"}, {5, "-1"}, {6, "200"}, {7, "32769"}, {8, "-1"}, {8, "1900"}, {9, "1070"}, {10, "0"}, {10, "1025"}, {11, "257"}, {12, "-2"}, {12, "300001"}, {13, "2"}, {5, "1.5"}}
	for _, c := range bad {
		f := strings.Split(string(body), "|")
		f[c.index] = c.value
		if _, err := ParseQueueRequest([]byte(strings.Join(f, "|")), "retail", queueNow()); err == nil {
			t.Fatalf("accepted field %d=%s", c.index, c.value)
		}
	}
	for _, b := range [][]byte{nil, []byte("1|retail"), make([]byte, 513), append(body, []byte("|extra")...)} {
		if _, err := ParseQueueRequest(b, "retail", queueNow()); err == nil {
			t.Fatal("malformed lease accepted")
		}
	}
}

func TestQueueGateRequiresAdvancingFreshStableTarget(t *testing.T) {
	now := queueNow()
	var g QueueGate
	if g.Observe(queueSample(1), "retail", now) != nil {
		t.Fatal("one sample authorized")
	}
	if g.Observe(queueSample(1), "retail", now.Add(100*time.Millisecond)) != nil {
		t.Fatal("frozen heartbeat authorized")
	}
	r := g.Observe(queueSample(2), "retail", now.Add(200*time.Millisecond))
	if r == nil {
		t.Fatal("stable advancing heartbeat rejected")
	}
	if !g.consume(*r, now) || g.consume(*r, now) {
		t.Fatal("duplicate invitation consumption")
	}
	if g.Observe(queueSample(3), "retail", now.Add(300*time.Millisecond)) != nil {
		t.Fatal("repeated click authorized")
	}
	for _, kind := range []string{"window", "geometry", "gap", "invalid", "disabled", "expired", "backwards", "client"} {
		t.Run(kind, func(t *testing.T) {
			var gate QueueGate
			gate.Observe(queueSample(1), "retail", now)
			s := queueSample(2)
			at := now.Add(100 * time.Millisecond)
			client := "retail"
			switch kind {
			case "window":
				s.Window = 43
			case "geometry":
				s.Queue = []byte(strings.Replace(string(s.Queue), "|800|", "|801|", 1))
			case "gap":
				at = now.Add(time.Second)
			case "invalid":
				s.Error = "checksum"
			case "disabled":
				s.Queue = []byte(strings.TrimSuffix(string(s.Queue), "|1") + "|0")
			case "expired":
				s.Queue = []byte(strings.Replace(string(s.Queue), "|3000|", "|0|", 1))
			case "backwards":
				at = now.Add(-time.Millisecond)
			case "client":
				client = "forever"
			}
			if gate.Observe(s, client, at) != nil {
				t.Fatal("unstable request authorized")
			}
		})
	}
}

func queueFixture() (QueueRequest, *QueueGate, *queuePlatform, *[]string) {
	now := queueNow()
	r, _ := ParseQueueRequest(queueBody(2), "retail", now)
	gate := &QueueGate{}
	calls := []string{}
	p := &queuePlatform{
		now: func() time.Time { return now }, foreground: func() uintptr { return 42 }, executable: func(uintptr) string { return "game/Wow.exe" }, matchesGame: func(exe, path, client string) bool {
			return exe == "game/Wow.exe" && path == "game" && client == "retail"
		},
		bounds: func(uintptr) (image.Rectangle, bool) { return image.Rect(-1920, 100, 0, 1180), true }, visible: func(uintptr, image.Rectangle) bool { return true }, capture: func() BridgeSample { return queueSample(3) }, held: func() bool { return false }, cursor: func() (image.Point, bool) { return image.Pt(-1000, 200), true },
		click: func(point image.Point) (int, error) {
			calls = append(calls, "click")
			if point != image.Pt(-1063, 611) {
				panic(fmt.Sprint("wrong button center: ", point))
			}
			return 3, nil
		}, release: func() { calls = append(calls, "release") }, restore: func(original, target image.Point) { calls = append(calls, "restore") },
	}
	return *r, gate, p, &calls
}

func TestQueueClickAndDuplicatePrevention(t *testing.T) {
	r, g, p, calls := queueFixture()
	if tried, err := sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p); !tried || err != nil {
		t.Fatal(tried, err)
	}
	if strings.Join(*calls, ",") != "click,restore" {
		t.Fatal(*calls)
	}
	if tried, _ := sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p); tried || len(*calls) != 2 {
		t.Fatal("repeated click", *calls)
	}
}

func TestQueueDefersWhileAnotherEngineInputOperationIsActive(t *testing.T) {
	r, g, p, calls := queueFixture()
	gameInputMu.Lock()
	tried, err := sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p)
	gameInputMu.Unlock()
	if tried || err == nil || len(*calls) != 0 || len(g.attempts) != 0 {
		t.Fatal("queue interleaved with another operation", tried, err, *calls)
	}
	if tried, err = sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p); !tried || err != nil {
		t.Fatal("deferred queue could not resume", tried, err)
	}
}

func TestQueuePreflightNeverSendsInput(t *testing.T) {
	cases := map[string]func(*queuePlatform){
		"wrong foreground":       func(p *queuePlatform) { p.foreground = func() uintptr { return 43 } },
		"wrong executable":       func(p *queuePlatform) { p.executable = func(uintptr) string { return "other" } },
		"covered button":         func(p *queuePlatform) { p.visible = func(uintptr, image.Rectangle) bool { return false } },
		"mouse or modifier held": func(p *queuePlatform) { p.held = func() bool { return true } },
		"cursor unavailable":     func(p *queuePlatform) { p.cursor = func() (image.Point, bool) { return image.Point{}, false } },
		"resized window": func(p *queuePlatform) {
			p.bounds = func(uintptr) (image.Rectangle, bool) { return image.Rect(0, 0, 1280, 720), true }
		},
		"cancelled popup": func(p *queuePlatform) {
			p.capture = func() BridgeSample { return BridgeSample{Error: "queue_missing"} }
		},
		"other window capture": func(p *queuePlatform) {
			p.capture = func() BridgeSample { s := queueSample(3); s.Window = 43; return s }
		},
		"moved button": func(p *queuePlatform) {
			p.capture = func() BridgeSample {
				s := queueSample(3)
				s.Queue = []byte(strings.Replace(string(s.Queue), "|800|", "|801|", 1))
				return s
			}
		},
		"stale request": func(p *queuePlatform) { p.now = func() time.Time { return queueNow().Add(4 * time.Second) } },
		"foreground changes during capture": func(p *queuePlatform) {
			n := 0
			p.foreground = func() uintptr {
				n++
				if n == 1 {
					return 42
				}
				return 43
			}
		},
		"window moves during capture": func(p *queuePlatform) {
			n := 0
			old := p.bounds
			p.bounds = func(w uintptr) (image.Rectangle, bool) {
				b, ok := old(w)
				n++
				if n > 1 {
					b = b.Add(image.Pt(1, 0))
				}
				return b, ok
			}
		},
		"user moves mouse": func(p *queuePlatform) {
			n := 0
			p.cursor = func() (image.Point, bool) { n++; return image.Pt(n, 2), true }
		},
		"lease turned off": func(p *queuePlatform) {
			p.capture = func() BridgeSample {
				s := queueSample(3)
				s.Queue = []byte(strings.TrimSuffix(string(s.Queue), "|1") + "|0")
				return s
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r, g, p, calls := queueFixture()
			change(p)
			tried, err := sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p)
			if tried || err == nil || len(*calls) != 0 || len(g.attempts) != 0 {
				t.Fatal(tried, err, *calls)
			}
		})
	}
	r, g, p, calls := queueFixture()
	ctx, cancel := context.WithCancel(context.Background())
	p.capture = func() BridgeSample { cancel(); return queueSample(3) }
	if tried, err := sendQueueAccept(ctx, 42, "game", "retail", r, g, *p); tried || !errors.Is(err, context.Canceled) || len(*calls) != 0 {
		t.Fatal(tried, err, *calls)
	}
}

func TestQueuePartialInputReleasesOnlyDeliveredPressAndNeverRetries(t *testing.T) {
	for sent := 0; sent < 3; sent++ {
		t.Run(fmt.Sprint(sent), func(t *testing.T) {
			r, g, p, calls := queueFixture()
			p.click = func(image.Point) (int, error) { *calls = append(*calls, "click"); return sent, errors.New("blocked") }
			tried, err := sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p)
			if !tried || err == nil {
				t.Fatal(tried, err)
			}
			want := "click,restore"
			if sent == 2 {
				want = "click,release,restore"
			}
			if strings.Join(*calls, ",") != want {
				t.Fatal(*calls)
			}
			count := len(*calls)
			tried, _ = sendQueueAccept(context.Background(), 42, "game", "retail", r, g, *p)
			if tried || len(*calls) != count {
				t.Fatal("partial click retried")
			}
		})
	}
}
