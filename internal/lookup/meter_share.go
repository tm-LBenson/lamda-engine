package lookup

import (
	"context"
	"fmt"
	"hash/crc32"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// This transport can only open our native preview or fill its nonce-bound empty
// edit box. It has no Enter, slash commands, clipboard, mouse or focus actions.
type meterSharePlatform struct {
	// Set only by DeliverMeterNavigation after validating a share-awards action.
	awardsCommand bool
	now           func() time.Time
	foreground    func() uintptr
	executable    func(uintptr) string
	matchesGame   func(string, string, string) bool
	capture       func() BridgeSample
	keyDown       func(uint16) bool
	open          func() error
	opened        func(context.Context) error
	text          func(string) error
	ack           func(context.Context, string, string) error
}

func sendMeterSummary(ctx context.Context, expected BridgeSample, wow, client, text string, p meterSharePlatform) error {
	n := expected.NativeMeter
	combatAllowed := combatAwardsInput(n, text, p.awardsCommand)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if expected.Error != "" || n == nil || (!n.Captured && !n.ControlOnly) || (n.Combat && !combatAllowed) || expected.Window == 0 || client != "retail" {
		return fmt.Errorf("summary needs a visible, idle Retail meter")
	}
	if text == "" {
		if !n.ShareCapable {
			return fmt.Errorf("reload the addon to enable its summary button (the preview shortcut must be unbound)")
		}
	} else {
		r := n.ShareRequest
		if r == nil || r.FocusBlocked || r.ReceivedBytes != 0 || r.Expires <= p.now().Unix() || r.Expires-p.now().Unix() > 30 || !reloadToken.MatchString(r.Token) {
			return fmt.Errorf("summary preview expired")
		}
		navigation := r.TransferVersion == 5 && r.Scope == "navigation" && r.Channel == "SELF" && r.Roster == 0
		if navigation && !meterCommandText.MatchString(text) {
			return fmt.Errorf("invalid native meter action")
		}
		if !utf8.ValidString(text) || len(text) > 4096 || (!navigation && !strings.HasPrefix(text, "[LamdaMeters] ") && !strings.HasPrefix(text, "[LM?] ")) || strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || r == '|' }) {
			return fmt.Errorf("invalid summary text")
		}

		if !navigation && !strings.HasPrefix(text, "[LM?] ") {
			lines := strings.Split(text, "~")
			if len(lines) > 50 {
				return fmt.Errorf("too many summary lines")
			}
			for _, line := range lines {
				if len(line) > 255 || !strings.HasPrefix(line, "[LamdaMeters] ") {
					return fmt.Errorf("invalid summary line")
				}
			}
		}
	}
	validWindow := func() bool {
		return p.foreground() == expected.Window && p.matchesGame(p.executable(expected.Window), wow, client)
	}
	if !validWindow() {
		return fmt.Errorf("keep WoW visible and focused to open the group summary")
	}
	fresh, captureErr := stableMeterCapture(ctx, p.capture)
	if captureErr != nil {
		return captureErr
	}
	f := fresh.NativeMeter
	if text != "" && n.ShareRequest.TransferVersion >= 2 {
		// Report preparation may span the boss ending or a display repaint.
		// The CRC-bound preview lease identifies the receiver; the changing
		// damage-session ID and optical surface type do not identify it.
		// Brief torn reads/focus changes pause within that same bounded lease.
		deadline := time.Now().Add(time.Duration(n.ShareRequest.Expires-p.now().Unix()) * time.Second)
		for {
			if !validWindow() {
				return fmt.Errorf("summary focus changed before input")
			}
			ready, done, reason, err := summaryTransferState(expected, *n.ShareRequest, fresh, p.now(), combatAllowed)
			if err != nil {
				return fmt.Errorf("summary input: %w (%s)", err, reason)
			}
			if done {
				return nil
			}
			if ready && fresh.Window == expected.Window {
				if fresh.NativeMeter.ShareRequest.ReceivedBytes != 0 {
					return fmt.Errorf("summary receiver already contains an incomplete transfer")
				}
				break
			}
			if !time.Now().Before(deadline) {
				return fmt.Errorf("summary receiver lease expired while waiting for %s", reason)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(25 * time.Millisecond):
			}
			fresh = p.capture()
		}
	} else {
		if fresh.Error != "" || fresh.Window != expected.Window || f == nil || (!f.Captured && !f.ControlOnly) || f.ControlOnly != n.ControlOnly || (f.Combat && !combatAllowed) || f.Player != n.Player || f.ID != n.ID {
			return fmt.Errorf("meter changed before summary input")
		}
		if text == "" {
			if !f.ShareCapable || f.ShareOpenVersion != n.ShareOpenVersion {
				return fmt.Errorf("summary receiver unavailable")
			}
		} else if f.ShareRequest == nil || *f.ShareRequest != *n.ShareRequest {
			return fmt.Errorf("summary preview changed")
		}
	}
	for _, key := range append(append([]uint16{}, simReloadHeldKeys...), 0x87, 0x79, 0x78) {
		if p.keyDown(key) {
			return fmt.Errorf("release modifier keys before sharing")
		}
	}
	if !validWindow() || ctx.Err() != nil {
		return fmt.Errorf("summary focus changed")
	}
	if text == "" {
		if err := p.open(); err != nil {
			return err
		}
		if p.opened != nil {
			return p.opened(ctx)
		}
		return nil
	}
	if n.ShareRequest.Expires <= p.now().Unix() {
		return fmt.Errorf("summary expired")
	}
	if n.ShareRequest.TransferVersion >= 3 && n.ShareRequest.Compact {
		text = compactSummaryText(text)
	}
	message := text + "[LM!" + n.ShareRequest.Token + "]"
	if n.ShareRequest.TransferVersion >= 2 {
		message = encodeSummaryDigits(message)
	}
	if err := p.text(message); err != nil {
		return err
	}
	if n.ShareRequest.TransferVersion >= 2 {
		if p.ack == nil {
			return fmt.Errorf("native preview acknowledgement unavailable")
		}
		return p.ack(ctx, n.Player, n.ShareRequest.Token)
	}
	return nil
}

// Only modern text-only awards receivers can stay available in combat. Older
// transports and ordinary meter/navigation actions retain their combat guard.
func combatAwardsLease(r *MeterShareRequest) bool {
	return r != nil && r.TransferVersion == 4 && r.Options <= 3 && r.Options&1 != 0 &&
		(r.Scope == "key" || r.Scope == "boss" || r.Scope == "boss-uptime" || meterHistoryScope.MatchString(r.Scope))
}

func combatAwardsInput(n *NativeMeter, text string, command bool) bool {
	if n == nil {
		return false
	}
	if text != "" && combatAwardsLease(n.ShareRequest) {
		return true
	}
	if !command {
		return false
	}
	if text == "" {
		return n.NavigationCapable && n.ShareOpenVersion == 3
	}
	r := n.ShareRequest
	return r != nil && r.TransferVersion == 5 && r.Scope == "navigation" && r.Channel == "SELF" && r.Roster == 0 && meterAwardsCommandText.MatchString(text)
}

// Decimal framing preserves UTF-8 names in the native receiver. Windows sends
// it as Unicode packets for v2/v4, or reserved unbound function keys for v3.
// Neither path emits ordinary digit keys or modifiers.
func encodeSummaryDigits(message string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "9%06d%010d", len(message), crc32.ChecksumIEEE([]byte(message)))
	for _, v := range []byte(message) {
		fmt.Fprintf(&b, "%03d", v)
	}
	return b.String()
}

func sameSummaryLease(a, b MeterShareRequest) bool {
	return a.Token == b.Token && a.Expires == b.Expires && a.Channel == b.Channel && a.Scope == b.Scope && a.Roster == b.Roster && a.TransferVersion == b.TransferVersion && a.Options == b.Options && a.Compact == b.Compact && a.EndedAt == b.EndedAt && a.EncounterID == b.EncounterID
}

// Repeated chat prefixes are reconstructed by the v3 native receiver. The report
// body, nonce and CRC remain verified; fewer key events need to cross the bridge.
func compactSummaryText(text string) string {
	if !strings.HasPrefix(text, "[LamdaMeters] ") {
		return text
	}
	lines := strings.Split(text, "~")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "[LamdaMeters] ")
	}
	return "[LM3] " + strings.Join(lines, "~")
}
