package lookup

import (
	"context"
	"time"
)

// A repaint can fall between the header and numeric bands. Retry incomplete
// optical reads before input; owner/session/foreground checks still run against
// the eventual valid frame. Occlusion and changed windows remain failures.
func stableMeterCapture(ctx context.Context, capture func() BridgeSample) (BridgeSample, error) {
	deadline := time.Now().Add(1500 * time.Millisecond)
	for {
		if err := ctx.Err(); err != nil {
			return BridgeSample{}, err
		}
		s := capture()
		code := s.Error
		if code == "" && s.NativeMeter != nil {
			code = s.NativeMeter.Error
		}
		pending := false
		switch code {
		case "meter_border_changed_during_capture", "meter_border_bands_pending", "meter_control_invalid", "meter_view_metadata_checksum", "meter_view_row_serial", "meter_view_duplicate_mismatch", "meter_view_calibration", "meter_view_aura_mismatch", "meter_view_aura_unreadable", "meter_view_duration":
			pending = true
		}
		if !pending || !time.Now().Before(deadline) {
			return s, nil
		}
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
