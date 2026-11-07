package lookup

import (
	"fmt"
	"time"
)

// Only a fresh matching lease permits input. A torn optical read or a temporary
// keyboard-focus loss pauses the transfer; it does not discard received bytes.
func summaryTransferState(expected BridgeSample, r MeterShareRequest, fresh BridgeSample, now time.Time, combatAwards ...bool) (ready, done bool, reason string, err error) {
	if now.Unix() >= r.Expires {
		return false, false, "expired", fmt.Errorf("summary receiver lease expired")
	}
	if fresh.Error != "" {
		return false, false, "capture/" + fresh.Error, nil
	}
	n := fresh.NativeMeter
	if n == nil {
		return false, false, "metadata_pending", nil
	}
	if fresh.Window != 0 && fresh.Window != expected.Window {
		return false, false, "window_changed", fmt.Errorf("summary window changed during transfer")
	}
	if n.Player == expected.NativeMeter.Player && n.ShareAck == r.Token {
		return false, true, "acknowledged", nil
	}
	if n.ShareCapable && n.ShareRequest == nil {
		return false, false, "cancelled", fmt.Errorf("summary preview was closed during transfer")
	}
	if n.Player != expected.NativeMeter.Player {
		return false, false, "owner_changed", fmt.Errorf("summary player changed during transfer")
	}
	if n.Combat && !combatAwardsLease(&r) && !(len(combatAwards) == 1 && combatAwards[0]) {
		return false, false, "combat", fmt.Errorf("summary paused because combat started")
	}
	if n.ShareRequest == nil {
		return false, false, "request_pending", nil
	}
	if !sameSummaryLease(r, *n.ShareRequest) {
		return false, false, "lease_changed", fmt.Errorf("summary request changed during transfer")
	}
	if n.ShareRequest.FocusBlocked {
		return false, false, "waiting_for_receiver_focus", nil
	}
	return true, false, "ready", nil
}
