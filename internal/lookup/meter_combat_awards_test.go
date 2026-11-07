package lookup

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCombatAwardsTransportKeepsLeaseAndInputGuards(t *testing.T) {
	for _, kind := range []string{"awards", "without damage award", "boss", "saved", "history", "combat starts", "legacy", "function keys", "meter", "combined", "window", "focus", "foreground", "owner", "nonce", "expiry", "audience", "held key", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Unix(1800000000, 0)
			r := MeterShareRequest{Token: "awards_1", Expires: now.Unix() + 20, Channel: "RAID", Scope: "key", Roster: 42, TransferVersion: 4, Options: 1}
			n := NativeMeter{ControlOnly: true, Combat: true, Player: "Player-1-Aug", ShareRequest: &r}
			sample := BridgeSample{Window: 10, NativeMeter: &n}
			fg, held := uintptr(10), false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "without damage award":
				r.Options = 3
			case "boss":
				r.Scope = "boss-uptime"
			case "saved":
				r.Scope = "saved:" + strings.Repeat("a", 32)
			case "history":
				r.Scope = "history:1:100:all"
			case "combat starts":
				n.Combat = false
			case "legacy":
				r.TransferVersion = 2
			case "function keys":
				r.TransferVersion = 3
			case "meter":
				r.Options = 4
			case "combined":
				r.Options = 0
			case "window":
				r.Scope = "window:1"
			case "focus":
				r.FocusBlocked = true
			case "foreground":
				fg = 11
			case "held key":
				held = true
			case "cancel":
				cancel()
			}
			fresh, rr := n, r
			fresh.Combat, fresh.ShareRequest = true, &rr
			switch kind {
			case "owner":
				fresh.Player = "Player-Other"
			case "nonce":
				rr.Token = "other_1"
			case "expiry":
				rr.Expires++
			case "audience":
				rr.Roster++
			}
			writes := 0
			text := "[LamdaMeters] Last completed fight awards"
			p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return fg }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return BridgeSample{Window: 10, NativeMeter: &fresh} }, keyDown: func(uint16) bool { return held }, text: func(wire string) error {
				writes++
				if wire != encodeSummaryDigits(text+"[LM!awards_1]") {
					t.Fatal(wire)
				}
				return nil
			}, ack: func(context.Context, string, string) error { return nil }}
			err := sendMeterSummary(ctx, sample, "WoW", "retail", text, p)
			good := kind == "awards" || kind == "without damage award" || kind == "boss" || kind == "saved" || kind == "history" || kind == "combat starts"
			if good && (err != nil || writes != 1) || !good && (err == nil || writes != 0) {
				t.Fatal("unexpected combat input result", err, writes)
			}
		})
	}
}

func TestCombatNavigationOnlyAllowsValidatedAwardsClick(t *testing.T) {
	for _, action := range []string{"share", "actor", "metric", "session"} {
		for _, value := range []string{"awards", "meter"} {
			command := MeterCommand{WindowID: 1, Serial: 4, Epoch: "native:1:2", Action: action, Value: value}
			n := NativeMeter{Combat: true, Captured: true, NavigationCapable: true, ID: command.Epoch, Player: "Player-1-Aug", Serial: 4, Frame: &NativeMeterFrame{WindowID: 1}}
			sample := BridgeSample{Window: 10, NativeMeter: &n}
			_, err := awaitMeterNavigationFrame(context.Background(), sample, command, func() BridgeSample { return sample })
			allowed := action == "share" && value == "awards"
			if (err == nil) != allowed {
				t.Fatal(action, value, err)
			}
		}
	}
	for _, kind := range []string{"open awards", "fill awards", "unvalidated", "meter", "actor", "ordinary opener"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Unix(1800000000, 0)
			r := MeterShareRequest{Token: "ui_1", Expires: now.Unix() + 6, Channel: "SELF", Scope: "navigation", TransferVersion: 5}
			n := NativeMeter{Combat: true, ControlOnly: true, NavigationCapable: true, ShareCapable: true, ShareOpenVersion: 3, Player: "Player-1-Aug", ShareRequest: &r}
			sample := BridgeSample{Window: 10, NativeMeter: &n}
			text := "[LMUI] 1,4,share,awards"
			calls := 0
			p := meterSharePlatform{awardsCommand: kind != "unvalidated", now: func() time.Time { return now }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return sample }, keyDown: func(uint16) bool { return false }, open: func() error { calls++; return nil }, text: func(string) error { calls++; return nil }, ack: func(context.Context, string, string) error { return nil }}
			switch kind {
			case "open awards":
				text = ""
			case "ordinary opener":
				text, n.ShareOpenVersion = "", 2
			case "meter":
				text = "[LMUI] 1,4,share,meter"
			case "actor":
				text = "[LMUI] 1,4,actor,Player-1-Aug"
			}
			err := sendMeterSummary(context.Background(), sample, "WoW", "retail", text, p)
			good := kind == "open awards" || kind == "fill awards"
			if good && (err != nil || calls != 1) || !good && (err == nil || calls != 0) {
				t.Fatal(err, calls)
			}
		})
	}
}
