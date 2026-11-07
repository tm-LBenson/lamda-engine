//go:build windows

package lookup

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

func DeliverMeterSummary(ctx context.Context, sample BridgeSample, wow, client, text string) error {
	return deliverMeterText(ctx, sample, wow, client, text, false)
}
func deliverMeterText(ctx context.Context, sample BridgeSample, wow, client, text string, navigation bool) error {
	gameInputMu.Lock()
	defer gameInputMu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if setDPI.Find() == nil {
		old, _, _ := setDPI.Call(^uintptr(3))
		if old != 0 {
			defer setDPI.Call(old)
		}
	}
	c := captureSession{surfaceLimit: 8}
	defer c.Close()
	return deliverMeterTextCapture(ctx, sample, wow, client, text, navigation, &c)
}
func deliverMeterTextCapture(ctx context.Context, sample BridgeSample, wow, client, text string, navigation bool, c *captureSession, combatAwards ...bool) error {
	awardsCommand := navigation && len(combatAwards) == 1 && combatAwards[0]
	send := func(inputs []reloadInput) (int, error) {
		n, _, err := user32.NewProc("SendInput").Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
		if int(n) != len(inputs) {
			return int(n), fmt.Errorf("summary preview input incomplete: %d/%d (%v)", n, len(inputs), err)
		}
		return int(n), nil
	}
	opened := false
	captureFresh := func() BridgeSample {
		slot := 0
		if navigation && text == "" && !opened {
			slot = meterCaptureSlot(sample)
		}
		s, _ := c.captureMeterBorderSlot(sample.LocatorBase, slot, wow, client)
		if navigation && text == "" && !opened && sample.NativeMeter.Frame != nil {
			s = meterSampleForWindow(s, sample.NativeMeter.Frame.WindowID)
		}
		if navigation && text == "" && !opened && s.NativeMeter != nil && s.NativeMeter.NavigationCapable {
			v := *s.NativeMeter
			v.ShareCapable = true
			v.ShareOpenVersion = 3
			s.NativeMeter = &v
		}
		return s
	}
	platform := meterSharePlatform{
		awardsCommand: awardsCommand,
		now:           time.Now,
		foreground:    func() uintptr { h, _, _ := foreground.Call(); return h },
		executable:    windowExecutable, matchesGame: func(exe, path, client string) bool { return gameExecutable(exe, path, client) },
		capture: captureFresh,
		keyDown: func(k uint16) bool {
			v, _, _ := user32.NewProc("GetAsyncKeyState").Call(uintptr(k))
			return v&0x8000 != 0
		},
		open: func() error {
			return sendSummaryOpen(ctx, sample.NativeMeter.ShareOpenVersion, send)
		},
		opened: func(ctx context.Context) error {
			opened = true
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if err := ctx.Err(); err != nil {
					return err
				}
				fresh := captureFresh()
				if fresh.Window == sample.Window && fresh.Error == "" && fresh.NativeMeter != nil && (!fresh.NativeMeter.Combat || awardsCommand) && (fresh.NativeMeter.ShareRequest != nil || fresh.NativeMeter.ShareAck != "") && (!navigation || fresh.NativeMeter.ShareRequest != nil && fresh.NativeMeter.ShareRequest.TransferVersion == 5) {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(50 * time.Millisecond):
				}
			}
			return fmt.Errorf("native summary preview did not open; use the in-game meter's chat button")
		},
		ack: func(ctx context.Context, owner, token string) error {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				fresh := captureFresh()
				if fresh.Error == "" && fresh.NativeMeter != nil && fresh.NativeMeter.Player == owner && fresh.NativeMeter.ShareAck == token {
					if navigation && fresh.NativeMeter.NavigationFailed {
						return fmt.Errorf("native meter selection changed before the action")
					}
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(50 * time.Millisecond):
				}
			}
			return fmt.Errorf("native preview did not acknowledge the report")
		},
		text: func(text string) error {
			// Reports contain text, never action-bar key presses. Even if the
			// native edit box loses focus after this last optical check, Unicode
			// packets cannot turn the encoded digits into ordinary number keys.
			// V3 instead uses the native lease's verified unbound F13-F22 keys.
			var inputs []reloadInput
			var err error
			if sample.NativeMeter.ShareRequest.TransferVersion == 3 {
				inputs, err = summaryFunctionInputs(text)
			} else {
				inputs, err = summaryTextInputs(text)
			}
			if err != nil {
				return err
			}
			r := sample.NativeMeter.ShareRequest
			if r != nil && r.TransferVersion == 2 {
				for {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					h, _, _ := foreground.Call()
					if h != sample.Window {
						if time.Now().Unix() >= r.Expires {
							return fmt.Errorf("summary expired while waiting for WoW focus")
						}
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(25 * time.Millisecond):
						}
						continue
					}
					heldKeys := false
					for _, key := range simReloadHeldKeys {
						held, _, _ := user32.NewProc("GetAsyncKeyState").Call(uintptr(key))
						if held&0x8000 != 0 {
							heldKeys = true
						}
					}
					fresh := captureFresh()
					ready, done, reason, err := summaryTransferState(sample, *r, fresh, time.Now())
					if err != nil {
						return fmt.Errorf("%w (%s)", err, reason)
					}
					if done {
						return nil
					}
					if ready && !heldKeys {
						break
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(25 * time.Millisecond):
					}
				}
			}
			// V3/V4 already verified the receiver lease in sendMeterSummary.
			// The native handler checks combat eligibility, cancellation and stale tokens;
			// a second full-window capture adds latency without focus safety.
			h, _, _ := foreground.Call()
			if h != sample.Window || ctx.Err() != nil || time.Now().Unix() >= r.Expires {
				return fmt.Errorf("summary focus or lease changed before transfer")
			}
			if sent, err := send(inputs); err != nil {
				// Release only a key-down actually accepted by SendInput. A
				// blanket replay of key-ups could itself invoke click bindings.
				if ups := summaryInputCleanup(inputs, sent); len(ups) > 0 {
					_, _ = send(ups)
				}
				return err
			}
			return nil
		},
	}
	return sendMeterSummary(ctx, sample, wow, client, text, platform)
}

// Only the explicitly advertised, addon-reserved preview opener uses bindable
// keys. Report data always follows its separate text-only V4 receiver.
func summaryOpenInputs(version int) ([]reloadInput, error) {
	keys := []uint16{0x87} // Legacy F24 receivers; never infer the new chord.
	if version == 2 {
		keys = []uint16{0x11, 0x10, 0x79} // CTRL-SHIFT-F10, verified unbound by addon.
	} else if version == 3 {
		keys = []uint16{0x11, 0x10, 0x78} // explicitly reserved CTRL-SHIFT-F9, navigation only
	} else if version != 0 {
		return nil, fmt.Errorf("unknown summary opener")
	}
	inputs := make([]reloadInput, 0, len(keys)*2)
	for _, key := range keys {
		inputs = append(inputs, reloadInput{Type: 1, Keyboard: reloadKeyboardInput{Key: key}})
	}
	for i := len(keys) - 1; i >= 0; i-- {
		inputs = append(inputs, reloadInput{Type: 1, Keyboard: reloadKeyboardInput{Key: keys[i], Flags: 2}})
	}
	return inputs, nil
}

func DeliverMeterNavigation(ctx context.Context, sample BridgeSample, wow, client string, command MeterCommand) error {
	gameInputMu.Lock()
	defer gameInputMu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if setDPI.Find() == nil {
		old, _, _ := setDPI.Call(^uintptr(3))
		if old != 0 {
			defer setDPI.Call(old)
		}
	}
	n := sample.NativeMeter
	if !meterCommandMatches(n, command) {
		return fmt.Errorf("meter selection expired")
	}
	c := captureSession{surfaceLimit: 8}
	defer c.Close()
	fresh, err := awaitMeterNavigationFrame(ctx, sample, command, func() BridgeSample {
		s, _ := c.captureMeterBorderSlot(sample.LocatorBase, meterCaptureSlot(sample), wow, client)
		return meterSampleForWindow(s, command.WindowID)
	})
	if err != nil {
		return err
	}
	sample, n = fresh, fresh.NativeMeter
	copy := *n
	copy.ShareCapable = true
	copy.ShareOpenVersion = 3
	sample.NativeMeter = &copy
	if err := deliverMeterTextCapture(ctx, sample, wow, client, "", true, &c, command.CombatAwards()); err != nil {
		return err
	}
	lease, found := c.captureMeterBorder(sample.LocatorBase, wow, client)
	if !found || lease.Error != "" || lease.NativeMeter == nil || lease.NativeMeter.ShareRequest == nil || lease.NativeMeter.ShareRequest.TransferVersion != 5 || lease.NativeMeter.Player != n.Player {
		return fmt.Errorf("native meter navigation receiver unavailable")
	}
	lease.LocatorBase = sample.LocatorBase
	text, err := command.Text()
	if err != nil {
		return err
	}
	return deliverMeterTextCapture(ctx, lease, wow, client, text, true, &c, command.CombatAwards())
}

func sendSummaryOpen(ctx context.Context, version int, send func([]reloadInput) (int, error)) error {
	inputs, err := summaryOpenInputs(version)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	down := len(inputs) / 2
	sent, err := send(inputs[:down])
	if err != nil {
		if release := summaryInputCleanup(inputs[:down], sent); len(release) > 0 {
			_, _ = send(release)
		}
		return err
	}
	// A bounded hold gives WoW a frame to observe the actual opener. Always
	// release accepted keys, including when the request is cancelled.
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}
	released, releaseErr := send(inputs[down:])
	if releaseErr != nil {
		if cleanup := summaryInputCleanup(inputs, down+released); len(cleanup) > 0 {
			_, _ = send(cleanup)
		}
		return releaseErr
	}
	return err
}

// KEYEVENTF_UNICODE produces text packets instead of the digit/letter virtual
// keys used by action bindings. There is deliberately no keyboard-layout or
// ordinary-key fallback when the receiver cannot accept these packets.
func summaryTextInputs(text string) ([]reloadInput, error) {
	if text == "" || len(text) > 13000 || !utf8.ValidString(text) || strings.ContainsFunc(text, unicode.IsControl) {
		return nil, fmt.Errorf("invalid report text transfer")
	}
	units := utf16.Encode([]rune(text))
	inputs := make([]reloadInput, 0, len(units)*2)
	for _, unit := range units {
		inputs = append(inputs,
			reloadInput{Type: 1, Keyboard: reloadKeyboardInput{Scan: unit, Flags: 4}},
			reloadInput{Type: 1, Keyboard: reloadKeyboardInput{Scan: unit, Flags: 6}},
		)
	}
	return inputs, nil
}

// The native v3 lease reserves F13-F22 only after checking their plain and
// modified bindings. No action-bar digit keys or modifiers are ever emitted.
func summaryFunctionInputs(text string) ([]reloadInput, error) {
	if text == "" || len(text) > 13000 || strings.ContainsFunc(text, func(r rune) bool { return r < '0' || r > '9' }) {
		return nil, fmt.Errorf("invalid reserved-key report transfer")
	}
	inputs := make([]reloadInput, 0, len(text)*2)
	for _, digit := range text {
		key := uint16(0x7c + digit - '0')
		inputs = append(inputs, reloadInput{Type: 1, Keyboard: reloadKeyboardInput{Key: key}}, reloadInput{Type: 1, Keyboard: reloadKeyboardInput{Key: key, Flags: 2}})
	}
	return inputs, nil
}

func summaryInputCleanup(inputs []reloadInput, sent int) []reloadInput {
	held := map[reloadKeyboardInput]bool{}
	for _, input := range inputs[:max(0, min(sent, len(inputs)))] {
		key := input.Keyboard
		up := key.Flags&2 != 0
		key.Flags &^= 2
		if up {
			delete(held, key)
		} else {
			held[key] = true
		}
	}
	releases := []reloadInput{}
	for i := max(0, min(sent, len(inputs))) - 1; i >= 0; i-- {
		key := inputs[i].Keyboard
		if held[key] {
			delete(held, key)
			key.Flags |= 2
			releases = append(releases, reloadInput{Type: 1, Keyboard: key})
		}
	}
	return releases
}
