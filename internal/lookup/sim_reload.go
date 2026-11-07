package lookup

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// This protocol authorizes only the addon's temporary Ctrl+Shift+F12 reload
// binding. It carries no key names, chat, commands, URLs, or executable code.
type ReloadRequest struct {
	Client, RequestID, GUID, Nonce string
	Spec                           int
	At, Expires                    int64
}

var reloadID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)
var reloadToken = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func ParseSimReloadRequest(body []byte, client string, now time.Time) (*ReloadRequest, error) {
	if len(body) == 0 || len(body) > 512 || (client != "retail" && client != "forever") {
		return nil, fmt.Errorf("invalid reload request size or client")
	}
	f := strings.Split(string(body), "|")
	if len(f) != 8 || f[0] != "2" || f[1] != client || !reloadID.MatchString(f[2]) || !reloadToken.MatchString(f[3]) || !reloadToken.MatchString(f[5]) {
		return nil, fmt.Errorf("invalid reload request identity")
	}
	spec, e1 := strconv.Atoi(f[4])
	at, e2 := strconv.ParseInt(f[6], 10, 64)
	expires, e3 := strconv.ParseInt(f[7], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || spec < 0 || spec > 10000 || at <= 0 || at > now.Unix() || now.Sub(time.Unix(at, 0)) > 5*time.Second || expires <= at || expires-at > 15 || now.Unix() >= expires {
		return nil, fmt.Errorf("reload request expired or invalid")
	}
	return &ReloadRequest{f[1], f[2], f[3], f[5], spec, at, expires}, nil
}

const (
	simReloadControlKey uint16 = 0x11 // VK_CONTROL
	simReloadShiftKey   uint16 = 0x10 // VK_SHIFT
	simReloadKey        uint16 = 0x7B // VK_F12
)

type simKeyEvent struct {
	Key uint16
	Up  bool
}

var simReloadHeldKeys = []uint16{0x10, 0x11, 0x12, 0x5B, 0x5C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, simReloadKey}
var simReloadSequence = []simKeyEvent{
	{Key: simReloadControlKey},
	{Key: simReloadShiftKey},
	{Key: simReloadKey},
	{Key: simReloadKey, Up: true},
	{Key: simReloadShiftKey, Up: true},
	{Key: simReloadControlKey, Up: true},
}
var simReloadCleanup = []simKeyEvent{
	{Key: simReloadKey, Up: true},
	{Key: simReloadShiftKey, Up: true},
	{Key: simReloadControlKey, Up: true},
}

// All operating-system actions are injected here so tests never touch input or
// focus. The production implementation provides exactly these bounded actions.
type simReloadPlatform struct {
	now         func() time.Time
	foreground  func() uintptr
	executable  func(uintptr) string
	matchesGame func(string, string, string) bool
	capture     func() BridgeSample
	keyDown     func(uint16) bool
	send        func([]simKeyEvent) (int, error)
}

func sendSimReload(ctx context.Context, window uintptr, wow, client string, expected []byte, platform simReloadPlatform) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if window == 0 || wow == "" {
		return fmt.Errorf("reload requires a configured game window")
	}
	if _, err := ParseSimReloadRequest(expected, client, platform.now()); err != nil {
		return err
	}
	if platform.foreground() != window || !platform.matchesGame(platform.executable(window), wow, client) {
		return fmt.Errorf("reload game is not foreground")
	}
	sample := platform.capture()
	if sample.Error != "" || sample.Window != window || !bytes.Equal(sample.Reload, expected) {
		return fmt.Errorf("reload request changed during capture")
	}
	for _, key := range simReloadHeldKeys {
		if platform.keyDown(key) {
			return fmt.Errorf("reload deferred while a modifier or F12 is held")
		}
	}
	if _, err := ParseSimReloadRequest(expected, client, platform.now()); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Check both executable and foreground again after capture/key checks. Never
	// focus or activate a window to make this succeed.
	if !platform.matchesGame(platform.executable(window), wow, client) || platform.foreground() != window {
		return fmt.Errorf("reload foreground changed before input")
	}
	sent, err := platform.send(simReloadSequence)
	if sent > 0 && sent < len(simReloadSequence) {
		// A partial SendInput may have left the chord held. Release only its
		// allowlisted keys; never retry key-down or this nonce.
		_, _ = platform.send(simReloadCleanup)
	}
	if err != nil || sent != len(simReloadSequence) {
		return fmt.Errorf("reload key chord was not fully delivered: %d events (%v)", sent, err)
	}
	return nil
}
