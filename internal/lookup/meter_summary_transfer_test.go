package lookup

import (
	"testing"
	"time"
)

func TestSummaryTransferPausesForTransientReadsAndFocusWithoutLosingLease(t *testing.T) {
	r := MeterShareRequest{Token: "nonce_1", Expires: 1800000030, TransferVersion: 2, Scope: "boss", Channel: "RAID", Roster: 42}
	expected := BridgeSample{Window: 10, NativeMeter: &NativeMeter{Player: "Player-1", ControlOnly: true, ShareRequest: &r}}
	for _, phase := range []string{"ready", "crc", "focus", "progress", "ack", "cancel", "owner", "lease", "combat", "expired"} {
		n := *expected.NativeMeter
		request := r
		n.ShareRequest = &request
		fresh := BridgeSample{Window: 10, NativeMeter: &n}
		now := time.Unix(1800000000, 0)
		switch phase {
		case "crc":
			fresh.Error = "meter_control_crc"
		case "focus":
			request.FocusBlocked = true
		case "progress":
			request.ReceivedBytes = 500
		case "ack":
			n.ShareRequest = nil
			n.ShareAck = r.Token
		case "cancel":
			n.ShareRequest = nil
			n.ShareCapable = true
		case "owner":
			n.Player = "Player-Other"
		case "lease":
			request.Token = "different"
		case "combat":
			n.Combat = true
		case "expired":
			now = time.Unix(r.Expires, 0)
		}
		ready, done, _, err := summaryTransferState(expected, r, fresh, now)
		wantReady := phase == "ready" || phase == "progress"
		wantDone := phase == "ack"
		wantError := phase == "cancel" || phase == "owner" || phase == "lease" || phase == "combat" || phase == "expired"
		if ready != wantReady || done != wantDone || (err != nil) != wantError {
			t.Fatal(phase, ready, done, err)
		}
	}
}
