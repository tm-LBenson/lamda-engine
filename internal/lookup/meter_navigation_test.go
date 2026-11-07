package lookup

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNavigationUsesItsOwnReceiverLeaseAndDoesNotBecomeReportText(t *testing.T) {
	now := time.Unix(1800000000, 0)
	r := &MeterShareRequest{Token: "ui_1", Expires: now.Unix() + 6, Channel: "SELF", Scope: "navigation", TransferVersion: 5}
	n := &NativeMeter{Player: "Player-1-Aug", ControlOnly: true, ShareRequest: r}
	sample := BridgeSample{Window: 10, NativeMeter: n}
	writes := 0
	p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return sample }, keyDown: func(uint16) bool { return false }, text: func(wire string) error {
		writes++
		if wire != encodeSummaryDigits("[LMUI] 1,4,actor,Player-1-Ally[LM!ui_1]") {
			t.Fatal(wire)
		}
		return nil
	}, ack: func(context.Context, string, string) error { return nil }}
	if err := sendMeterSummary(context.Background(), sample, "WoW", "retail", "[LMUI] 1,4,actor,Player-1-Ally", p); err != nil || writes != 1 {
		t.Fatal(err, writes)
	}
	for _, bad := range []string{"/reload", "[LMUI] 1,4,cast,FireBreath", "[LamdaMeters] Group report", "[LMUI] 1,4,actor,Player-1-Ally~text", "[LMUI] 9,4,actor,Player-1-Ally"} {
		before := writes
		if sendMeterSummary(context.Background(), sample, "WoW", "retail", bad, p) == nil || writes != before {
			t.Fatal("non-navigation input accepted", bad)
		}
	}
	r.TransferVersion = 4
	r.Scope = "key"
	if sendMeterSummary(context.Background(), sample, "WoW", "retail", "[LMUI] 1,4,actor,Player-1-Ally", p) == nil {
		t.Fatal("report receiver accepted a UI action")
	}
}

func TestNavigationControlAndCommandValidation(t *testing.T) {
	body := "5|ui_1|Player-1-Aug|1800000006|SELF|navigation|0|1|request|0|0|0|0|0"
	n, err := parseMeterControl(controlFixture(body))
	if err != nil || n.ShareRequest.TransferVersion != 5 || n.ShareRequest.Scope != "navigation" {
		t.Fatal(n, err)
	}
	for _, bad := range []string{strings.Replace(body, "SELF", "PARTY", 1), strings.Replace(body, "navigation", "boss", 1), strings.Replace(body, "|0|1|request", "|1|1|request", 1), strings.Replace(body, "5|", "4|", 1)} {
		if _, err := parseMeterControl(controlFixture(bad)); err == nil {
			t.Fatal(bad)
		}
	}
	failed := strings.Replace(body, "request", "error", 1)
	n, err = parseMeterControl(controlFixture(failed))
	if err != nil || n.ShareAck != "ui_1" || !n.NavigationFailed {
		t.Fatal(n, err)
	}
	for _, command := range []MeterCommand{{WindowID: 1, Serial: 4, Epoch: "native:1:2", Action: "actor", Value: "Player-1-Aug"}, {WindowID: 8, Serial: 16777215, Epoch: "native:1:2", Action: "session", Value: "Overall"}} {
		if !command.Valid() {
			t.Fatal(command)
		}
		if _, err := command.Text(); err != nil {
			t.Fatal(err)
		}
	}
	if (MeterCommand{WindowID: 1, Serial: 4, Epoch: "native:1:2", Action: "cast", Value: "FireBreath"}).Valid() {
		t.Fatal("gameplay action accepted")
	}
}

func TestNextNavigationWaitsForAcknowledgmentWithoutLosingWindowRoute(t *testing.T) {
	command := MeterCommand{WindowID: 2, Serial: 40, Epoch: "native:1:2", Action: "metric", Value: "Dispels"}
	n := &NativeMeter{Captured: true, NavigationCapable: true, ID: command.Epoch, Player: "Player-1-Aug", Serial: 42, Frame: &NativeMeterFrame{WindowID: 2}}
	expected := BridgeSample{Window: 10, NativeMeter: n, LocatorBase: BridgePosition{X: 100, Y: 200}, MeterSlot: 1}
	reads := 0
	fresh, err := awaitMeterNavigationFrame(context.Background(), expected, command, func() BridgeSample {
		reads++
		if reads < 3 {
			return BridgeSample{Window: 10, NativeMeter: &NativeMeter{Player: n.Player, ControlOnly: true, ShareAck: "previous"}}
		}
		return BridgeSample{Window: 10, NativeMeter: n}
	})
	if err != nil || reads != 3 || fresh.LocatorBase != expected.LocatorBase || fresh.MeterSlot != 1 {
		t.Fatal(reads, fresh, err)
	}
	for _, bad := range []NativeMeter{
		{Player: n.Player, ControlOnly: true, ShareRequest: &MeterShareRequest{Token: "active_report"}},
		{Player: "Player-Other", Captured: true},
		{Player: n.Player, Captured: true, NavigationCapable: true, ID: "another:epoch", Serial: 42, Frame: n.Frame},
		{Player: n.Player, Captured: true, NavigationCapable: true, ID: n.ID, Serial: 80, Frame: n.Frame},
	} {
		if _, err := awaitMeterNavigationFrame(context.Background(), expected, command, func() BridgeSample { return BridgeSample{Window: 10, NativeMeter: &bad} }); err == nil {
			t.Fatal("changed selection or an open report was accepted", bad)
		}
	}
}
