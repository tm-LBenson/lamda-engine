//go:build !windows

package lookup

import (
	"context"
	"fmt"
)

func DeliverMeterSummary(context.Context, BridgeSample, string, string, string) error {
	return fmt.Errorf("native summary preview requires Windows")
}
func DeliverMeterNavigation(context.Context, BridgeSample, string, string, MeterCommand) error {
	return fmt.Errorf("native meter navigation requires Windows")
}
