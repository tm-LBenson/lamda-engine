package lookup

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSummaryPreviewTransportChecksFocusNonceAndFixedActions(t *testing.T) {
	for _, kind := range []string{"open", "fill", "foreground", "combat", "expired", "request changed", "held key", "occluded", "text", "error text", "unicode", "multiple lines", "bad line", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Unix(1800000000, 0)
			r := &MeterShareRequest{Token: "unique_1", Expires: now.Unix() + 20, Channel: "PARTY", Scope: "key", Roster: 42}
			n := &NativeMeter{ID: "123:45", Player: "Player-1-A", Captured: true, ShareCapable: true, ShareRequest: r}
			s := BridgeSample{Window: 10, NativeMeter: n}
			fresh := s
			copy := *n
			fresh.NativeMeter = &copy
			calls := 0
			text := "[LamdaMeters] Fixture; Damage MVP: Aug 1M"
			fg := uintptr(10)
			held := false
			ctx := context.Background()
			switch kind {
			case "open":
				text = ""
			case "foreground":
				fg = 11
			case "combat":
				copy.Combat = true
			case "expired":
				r.Expires = now.Unix()
			case "request changed":
				rr := *r
				rr.Token = "different"
				copy.ShareRequest = &rr
			case "held key":
				held = true
			case "occluded":
				fresh.Error = "occluded"
			case "text":
				text = "/reload"
			case "error text":
				text = "[LM?] No completed key is ready"
			case "unicode":
				text = "[LamdaMeters] 界 Я"
			case "multiple lines":
				text = "[LamdaMeters] Boss~[LamdaMeters] 1. Aug 100K"
			case "bad line":
				text = "[LamdaMeters] Boss~/reload"
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return fg }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return fresh }, keyDown: func(uint16) bool { return held }, open: func() error { calls++; return nil }, text: func(v string) error {
				calls++
				if !strings.HasSuffix(v, "[LM!unique_1]") || strings.ContainsAny(v, "\r\n") {
					t.Fatal(v)
				}
				return nil
			}}
			e := sendMeterSummary(ctx, s, "WoW", "retail", text, p)
			good := kind == "open" || kind == "fill" || kind == "error text" || kind == "unicode" || kind == "multiple lines"
			if good && (e != nil || calls != 1) {
				t.Fatal(e, calls)
			}
			if !good && (e == nil || calls != 0) {
				t.Fatal("input allowed", e, calls)
			}
		})
	}
}
func TestNativeShareMetadataV5(t *testing.T) {
	body := "5|123:45|7|68|Player-1-Aug|1473|0|6000|32|12345|100|100|380|220|boss|0|0|0|0|0|0|1|unique_1|1800000030|PARTY|key|4294967295\n1|Player-1-Aug|13|Aug|0"
	s := parseNativeMeter([]byte(body))
	if s.Error != "" || s.NativeMeter.ShareRequest == nil || !s.NativeMeter.ShareCapable || s.NativeMeter.ShareRequest.Roster != 4294967295 {
		t.Fatal(s)
	}
	for _, scope := range []string{"boss", "boss-uptime"} {
		if parseNativeMeter([]byte(strings.Replace(body, "|key|", "|"+scope+"|", 1))).Error != "" {
			t.Fatal(scope)
		}
	}
	for _, change := range [][2]string{{"unique_1", "bad token"}, {"|PARTY|", "|SAY|"}, {"4294967295", "4294967296"}, {"|key|", "|/reload|"}} {
		if parseNativeMeter([]byte(strings.Replace(body, change[0], change[1], 1))).Error == "" {
			t.Fatal(change)
		}
	}
}

func TestNativeShareOpenerMustBeExplicitAndRemainUnchanged(t *testing.T) {
	body := "6|123:45|7|68|Player-1-Aug|1473|0|6000|32|12345|100|100|380|220|boss|0|0|0|0|0|0|2|-|0|-|-|0|1800000000:123:1|overall|1|1\n1|Player-1-Aug|13|Aug|0"
	s := parseNativeMeter([]byte(body))
	if s.Error != "" || !s.NativeMeter.ShareCapable || s.NativeMeter.ShareOpenVersion != 2 {
		t.Fatal(s)
	}
	if parseNativeMeter([]byte(strings.Replace(body, "|2|-|", "|3|-|", 1))).Error == "" {
		t.Fatal("unknown opener accepted")
	}
	s.NativeMeter.Captured = true
	s.Window = 10
	fresh := s
	copy := *s.NativeMeter
	copy.ShareOpenVersion = 0
	fresh.NativeMeter = &copy
	calls := 0
	p := meterSharePlatform{now: time.Now, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return fresh }, keyDown: func(uint16) bool { return false }, open: func() error { calls++; return nil }}
	if sendMeterSummary(context.Background(), s, "WoW", "retail", "", p) == nil || calls != 0 {
		t.Fatal("changed opener sent keys")
	}
	fresh.NativeMeter = s.NativeMeter
	p.opened = func(context.Context) error { return fmt.Errorf("no native receipt") }
	if sendMeterSummary(context.Background(), s, "WoW", "retail", "", p) == nil || calls != 1 {
		t.Fatal("opener without native receipt reported success")
	}
}

func TestNativeAutoScopeMetadataV6(t *testing.T) {
	body := "6|123:45|7|68|Player-1-Aug|1473|0|6000|32|12345|100|100|380|220|boss|0|0|0|0|0|0|1|-|0|-|-|0|1800000000:123:1|overall|1|1\n1|Player-1-Aug|13|Aug|0"
	s := parseNativeMeter([]byte(body))
	if s.Error != "" || s.NativeMeter.Auto == nil || s.NativeMeter.Auto.Scope != "overall" || !s.NativeMeter.Auto.AfterKey || !s.NativeMeter.Auto.OnCombat {
		t.Fatal(s)
	}
	for _, v := range []string{strings.Replace(body, "|overall|", "|native:77|", 1), strings.Replace(body, "|1|1\n", "|2|1\n", 1), strings.Replace(body, "1800000000:123:1", "bad token", 1)} {
		if parseNativeMeter([]byte(v)).Error == "" {
			t.Fatal("invalid auto request accepted")
		}
	}
}

func TestDecimalTransferRequiresReceiptAndPreservesUTF8(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, kind := range []string{"success", "focus", "partial", "no receipt", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			r := &MeterShareRequest{Token: "unique_1", Expires: now.Unix() + 20, Channel: "PARTY", Scope: "key", Roster: 42, TransferVersion: 2}
			n := &NativeMeter{ControlOnly: true, Player: "Player-1-Aug", ID: "preview", ShareRequest: r}
			sample := BridgeSample{Window: 10, NativeMeter: n}
			text := "[LamdaMeters] 1. 界Я 110K DPS"
			writes := 0
			acks := 0
			p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return sample }, keyDown: func(uint16) bool { return false }, text: func(v string) error {
				writes++
				if v != encodeSummaryDigits(text+"[LM!unique_1]") {
					t.Fatal(v)
				}
				for _, r := range v {
					if r < '0' || r > '9' {
						t.Fatal(r)
					}
				}
				return nil
			}, ack: func(ctx context.Context, owner, token string) error {
				acks++
				if owner != n.Player || token != r.Token {
					t.Fatal(owner, token)
				}
				if kind == "no receipt" {
					return fmt.Errorf("no receipt")
				}
				return nil
			}}
			ctx := context.Background()
			switch kind {
			case "focus":
				r.FocusBlocked = true
			case "partial":
				r.ReceivedBytes = 12
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := sendMeterSummary(ctx, sample, "WoW", "retail", text, p)
			if kind == "success" {
				if err != nil || writes != 1 || acks != 1 {
					t.Fatal(err, writes, acks)
				}
			} else if kind == "no receipt" {
				if err == nil || writes != 1 || acks != 1 {
					t.Fatal(err, writes, acks)
				}
			} else if err == nil || writes != 0 || acks != 0 {
				t.Fatal(err, writes, acks)
			}
		})
	}
}

func TestPreparedSummaryPreflightFollowsPreviewLeaseAcrossPostPullChanges(t *testing.T) {
	for _, kind := range []string{"session", "control to native", "native to control", "header only", "crc retry", "focus retry", "metadata retry", "window pending", "owner", "window", "combat", "token", "channel", "scope", "roster", "expiry changed", "partial", "acknowledged", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Unix(1800000000, 0)
			r := MeterShareRequest{Token: "same_preview_1", Expires: now.Unix() + 20, TransferVersion: 2, Channel: "RAID", Scope: "boss", Roster: 42}
			n := &NativeMeter{ID: "current:old", Player: "Player-1-Aug", Captured: true, ShareCapable: true, ShareRequest: &r}
			expected := BridgeSample{Window: 10, NativeMeter: n}
			copy := *n
			rr := r
			copy.ID = "current:new"
			copy.ShareRequest = &rr
			fresh := BridgeSample{Window: 10, NativeMeter: &copy}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "control to native":
				n.Captured = false
				n.ControlOnly = true
			case "native to control":
				copy.Captured = false
				copy.ControlOnly = true
			case "header only":
				copy.Captured = false
				copy.HeaderOnly = true
			case "owner":
				copy.Player = "Player-Other"
			case "window":
				fresh.Window = 11
			case "combat":
				copy.Combat = true
			case "token":
				rr.Token = "another_preview"
			case "channel":
				rr.Channel = "PARTY"
			case "scope":
				rr.Scope = "key"
			case "roster":
				rr.Roster++
			case "expiry changed":
				rr.Expires++
			case "partial":
				rr.ReceivedBytes = 12
			case "acknowledged":
				copy.ShareRequest = nil
				copy.ShareAck = r.Token
			}
			reads, writes, acks := 0, 0, 0
			p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, keyDown: func(uint16) bool { return false }, capture: func() BridgeSample {
				reads++
				v := fresh
				if reads == 1 {
					switch kind {
					case "crc retry":
						v.Error = "meter_control_crc"
					case "focus retry":
						c := copy
						request := rr
						request.FocusBlocked = true
						c.ShareRequest = &request
						v.NativeMeter = &c
					case "metadata retry":
						v.NativeMeter = nil
					case "window pending":
						v.Window = 0
					case "cancel":
						cancel()
					}
				}
				return v
			}, text: func(v string) error {
				writes++
				if v != encodeSummaryDigits("[LamdaMeters] Boss recap[LM!same_preview_1]") {
					t.Fatal(v)
				}
				return nil
			}, ack: func(context.Context, string, string) error { acks++; return nil }}
			err := sendMeterSummary(ctx, expected, "WoW", "retail", "[LamdaMeters] Boss recap", p)
			good := kind == "session" || kind == "control to native" || kind == "native to control" || kind == "header only" || strings.HasSuffix(kind, "retry") || kind == "window pending"
			if good && (err != nil || writes != 1 || acks != 1) {
				t.Fatal(kind, err, reads, writes, acks)
			}
			if kind == "acknowledged" {
				if err != nil || writes != 0 || acks != 0 {
					t.Fatal(kind, err, writes, acks)
				}
			} else if !good && (err == nil || writes != 0 || acks != 0) {
				t.Fatal("unsafe preview input", kind, err, writes, acks)
			}
		})
	}
}
