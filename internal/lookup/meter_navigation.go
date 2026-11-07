package lookup

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

type MeterCommand struct {
	WindowID int    `json:"windowID"`
	Serial   int    `json:"serial"`
	Epoch    string `json:"epoch"`
	Action   string `json:"action"`
	Value    string `json:"value"`
}

func meterCommandMatches(n *NativeMeter, command MeterCommand) bool {
	return command.Valid() && n != nil && n.Captured && n.NavigationCapable && n.Frame != nil && n.Frame.WindowID == command.WindowID && n.ID == command.Epoch && command.Serial <= n.Serial && n.Serial-command.Serial <= 20
}

// A completed command advertises its acknowledgment briefly. Wait for that
// acknowledgment to clear before opening the next receiver; it is not a
// changed owner/session and must not silently discard the next click.
func awaitMeterNavigationFrame(ctx context.Context, expected BridgeSample, command MeterCommand, capture func() BridgeSample) (BridgeSample, error) {
	if expected.NativeMeter == nil {
		return BridgeSample{}, fmt.Errorf("meter selection unavailable")
	}
	for {
		if err := ctx.Err(); err != nil {
			return BridgeSample{}, err
		}
		fresh, err := stableMeterCapture(ctx, capture)
		if err != nil {
			return fresh, err
		}
		fresh.LocatorBase, fresh.MeterSlot = expected.LocatorBase, expected.MeterSlot
		fresh.MeterCaptureSlot = expected.MeterCaptureSlot
		n := fresh.NativeMeter
		if fresh.Error != "" || fresh.Window != expected.Window || n == nil || (n.Combat && !command.CombatAwards()) || n.Player != expected.NativeMeter.Player {
			return fresh, fmt.Errorf("meter changed before navigation")
		}
		if !n.ControlOnly || n.ShareAck == "" {
			if !meterCommandMatches(n, command) {
				return fresh, fmt.Errorf("meter selection expired")
			}
			return fresh, nil
		}
		select {
		case <-ctx.Done():
			return fresh, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

var meterWindowScope = regexp.MustCompile(`^window:[1-8]$`)
var meterCommandValue = regexp.MustCompile(`^[A-Za-z0-9:_-]{1,160}$`)
var meterCommandText = regexp.MustCompile(`^\[LMUI\] [1-8],[1-9][0-9]{0,7},(actor|row|back|metric|session|scroll|share),[A-Za-z0-9:_-]{1,160}$`)
var meterAwardsCommandText = regexp.MustCompile(`^\[LMUI\] [1-8],[1-9][0-9]{0,7},share,awards$`)

func (r MeterCommand) CombatAwards() bool {
	return r.Valid() && r.Action == "share" && r.Value == "awards"
}

func (r MeterCommand) Valid() bool {
	if r.WindowID < 1 || r.WindowID > 8 || r.Serial < 1 || r.Serial > 16777215 || !observationID.MatchString(r.Epoch) || !meterCommandValue.MatchString(r.Value) {
		return false
	}
	return r.Action == "share" || r.Action == "row" || r.Action == "actor" || r.Action == "back" || r.Action == "metric" || r.Action == "session" || r.Action == "scroll"
}
func (r MeterCommand) Text() (string, error) {
	if !r.Valid() {
		return "", fmt.Errorf("invalid meter navigation")
	}
	return fmt.Sprintf("[LMUI] %d,%d,%s,%s", r.WindowID, r.Serial, r.Action, r.Value), nil
}
