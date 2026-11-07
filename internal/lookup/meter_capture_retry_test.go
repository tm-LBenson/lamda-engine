package lookup

import (
	"context"
	"testing"
	"time"
)

func TestSummaryOpenerRetriesRepaintAndStillRejectsChangedWindow(t *testing.T) {
	for _, changed := range []bool{false, true} {
		n := &NativeMeter{Player: "Player-1-Aug", ID: "native:1", Captured: true, ShareCapable: true}
		sample := BridgeSample{Window: 10, NativeMeter: n}
		reads, opens := 0, 0
		p := meterSharePlatform{now: func() time.Time { return time.Unix(1800000000, 0) }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, keyDown: func(uint16) bool { return false }, open: func() error { opens++; return nil }, opened: func(context.Context) error { return nil }, capture: func() BridgeSample {
			reads++
			if reads == 1 {
				return BridgeSample{Error: "meter_border_changed_during_capture"}
			}
			s := sample
			if changed {
				s.Window = 20
			}
			return s
		}}
		err := sendMeterSummary(context.Background(), sample, "WoW", "retail", "", p)
		if reads != 2 || changed && (err == nil || opens != 0) || !changed && (err != nil || opens != 1) {
			t.Fatal(changed, reads, opens, err)
		}
	}
	reads := 0
	s, err := stableMeterCapture(context.Background(), func() BridgeSample { reads++; return BridgeSample{Error: "bridge_strip_occluded"} })
	if err != nil || reads != 1 || s.Error != "bridge_strip_occluded" {
		t.Fatal(s, reads, err)
	}
}
