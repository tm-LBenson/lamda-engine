package lookup

import (
	"context"
	"fmt"
	"hash/crc32"
	"image"
	"strconv"
	"strings"
	"time"
)

// The edge itself can contain LamdaOverlays or meter data. This reserved inset
// matches Queue.lua; a queue packet must not overwrite those existing bands.
const QueueStripBottomInset = 48

func queueBridgePosition(height int) BridgePosition {
	return BridgePosition{X: 0, Y: height - QueueStripBottomInset - BridgeHeight}
}

func queueMarker(nonce string) [12]byte {
	c := crc32.ChecksumIEEE([]byte(nonce))
	r, g, b := byte(c>>16), byte(c>>8), byte(c)
	return [12]byte{255, 0, 255, r, g, b, 0, 255, 255, 255 - r, 255 - g, 255 - b}
}

// A queue lease is emitted only by the enabled LamdaQueue module. Coordinates
// describe the visible Blizzard acceptance button, never a configurable action.
type QueueRequest struct {
	Client, Nonce, Kind                                       string
	At                                                        int64
	Tick                                                      uint64
	Width, Height, X, Y, ButtonWidth, ButtonHeight, Remaining int
	Due                                                       bool
}

func ParseQueueRequest(body []byte, client string, now time.Time) (*QueueRequest, error) {
	if len(body) == 0 || len(body) > 512 || (client != "retail" && client != "forever") {
		return nil, fmt.Errorf("invalid queue lease size or client")
	}
	f := strings.Split(string(body), "|")
	if len(f) != 14 || f[0] != "1" || f[1] != client || !reloadToken.MatchString(f[2]) || (f[3] != "lfg" && f[3] != "pvp") {
		return nil, fmt.Errorf("invalid queue lease identity")
	}
	values := make([]int64, 10)
	for i := range values {
		n, err := strconv.ParseInt(f[i+4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid queue lease number")
		}
		values[i] = n
	}
	if values[0] <= 0 || values[0] > now.Unix()+1 || now.Sub(time.Unix(values[0], 0)) > 2500*time.Millisecond || values[1] < 1 || values[1] > 1<<53 {
		return nil, fmt.Errorf("stale queue lease")
	}
	for _, n := range values[2:8] {
		if n < 0 || n > 32768 {
			return nil, fmt.Errorf("invalid queue geometry")
		}
	}
	if values[8] < -1 || values[8] > 300000 || (values[9] != 0 && values[9] != 1) {
		return nil, fmt.Errorf("invalid queue timing")
	}
	r := &QueueRequest{Client: f[1], Nonce: f[2], Kind: f[3], At: values[0], Tick: uint64(values[1]), Width: int(values[2]), Height: int(values[3]), X: int(values[4]), Y: int(values[5]), ButtonWidth: int(values[6]), ButtonHeight: int(values[7]), Remaining: int(values[8]), Due: values[9] == 1}
	if r.Width < 320 || r.Height < 200 || r.ButtonWidth < 8 || r.ButtonWidth > 1024 || r.ButtonHeight < 8 || r.ButtonHeight > 256 || r.X < 0 || r.Y < 0 || r.X+r.ButtonWidth > r.Width || r.Y+r.ButtonHeight > r.Height || r.Remaining < -1 || r.Remaining > 300000 || (values[9] != 0 && values[9] != 1) {
		return nil, fmt.Errorf("invalid queue geometry or timing")
	}
	return r, nil
}

func (r QueueRequest) sameTarget(other QueueRequest) bool {
	return r.Client == other.Client && r.Nonce == other.Nonce && r.Kind == other.Kind && r.Width == other.Width && r.Height == other.Height && r.X == other.X && r.Y == other.Y && r.ButtonWidth == other.ButtonWidth && r.ButtonHeight == other.ButtonHeight
}

// Two advancing heartbeats must agree on the target. A frozen/repainted packet
// alone cannot authorize input. Each invitation is attempted at most once.
type QueueGate struct {
	previous *QueueRequest
	window   uintptr
	seen     time.Time
	attempts map[string]time.Time
}

func (g *QueueGate) Observe(sample BridgeSample, client string, now time.Time) *QueueRequest {
	r, err := ParseQueueRequest(sample.Queue, client, now)
	if sample.Error != "" || sample.Window == 0 || err != nil || !r.Due || r.Remaining == 0 {
		g.previous = nil
		return nil
	}
	if g.attempts == nil {
		g.attempts = make(map[string]time.Time)
	}
	for key, at := range g.attempts {
		if now.Sub(at) > 10*time.Minute {
			delete(g.attempts, key)
		}
	}
	if _, done := g.attempts[r.Nonce]; done || len(g.attempts) >= 256 {
		g.previous = nil
		return nil
	}
	previous, seen, window := g.previous, g.seen, g.window
	g.previous, g.seen, g.window = r, now, sample.Window
	if previous == nil || window != sample.Window || now.Before(seen) || now.Sub(seen) > 400*time.Millisecond || r.Tick <= previous.Tick || !r.sameTarget(*previous) {
		return nil
	}
	return r
}

func (g *QueueGate) consume(r QueueRequest, now time.Time) bool {
	if g.attempts == nil {
		g.attempts = make(map[string]time.Time)
	}
	if _, exists := g.attempts[r.Nonce]; exists || len(g.attempts) >= 256 {
		return false
	}
	g.attempts[r.Nonce] = now
	g.previous = nil
	return true
}

type queuePlatform struct {
	now         func() time.Time
	foreground  func() uintptr
	executable  func(uintptr) string
	matchesGame func(string, string, string) bool
	bounds      func(uintptr) (image.Rectangle, bool)
	visible     func(uintptr, image.Rectangle) bool
	capture     func() BridgeSample
	held        func() bool
	cursor      func() (image.Point, bool)
	click       func(image.Point) (int, error)
	release     func()
	restore     func(image.Point, image.Point)
}

// Returns attempted=true only after consuming the invitation, immediately
// before OS input. A preflight deferral remains eligible while the popup lasts.
func sendQueueAccept(ctx context.Context, window uintptr, wow, client string, expected QueueRequest, g *QueueGate, p queuePlatform) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !gameInputMu.TryLock() {
		return false, fmt.Errorf("another engine input operation is active")
	}
	defer gameInputMu.Unlock()
	if window == 0 || wow == "" || p.foreground() != window || !p.matchesGame(p.executable(window), wow, client) {
		return false, fmt.Errorf("queue game is not foreground")
	}
	bounds, ok := p.bounds(window)
	if !ok || bounds.Dx() != expected.Width || bounds.Dy() != expected.Height {
		return false, fmt.Errorf("queue window geometry changed")
	}
	area := image.Rect(expected.X, expected.Y, expected.X+expected.ButtonWidth, expected.Y+expected.ButtonHeight).Add(bounds.Min)
	if !p.visible(window, area) || p.held() {
		return false, fmt.Errorf("queue button covered or input held")
	}
	original, ok := p.cursor()
	if !ok {
		return false, fmt.Errorf("queue cursor unavailable")
	}
	fresh := p.capture()
	r, err := ParseQueueRequest(fresh.Queue, client, p.now())
	if err != nil || fresh.Error != "" || fresh.Window != window || !r.Due || r.Remaining == 0 || r.Tick < expected.Tick || !r.sameTarget(expected) {
		return false, fmt.Errorf("queue request changed before input")
	}
	current, valid := p.bounds(window)
	cursor, cursorOK := p.cursor()
	if !valid || current != bounds || !cursorOK || cursor != original || p.held() || !p.visible(window, area) || !p.matchesGame(p.executable(window), wow, client) || p.foreground() != window {
		return false, fmt.Errorf("queue context changed before input")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !g.consume(*r, p.now()) {
		return false, fmt.Errorf("queue invitation already attempted")
	}
	target := area.Min.Add(image.Pt(area.Dx()/2, area.Dy()/2))
	// One move, one press, one release. Partial input never retries the press.
	sent, err := p.click(target)
	if sent == 2 {
		p.release()
	}
	p.restore(original, target)
	if err != nil || sent != 3 {
		return true, fmt.Errorf("queue click incomplete: %d/3 (%v)", sent, err)
	}
	return true, nil
}
