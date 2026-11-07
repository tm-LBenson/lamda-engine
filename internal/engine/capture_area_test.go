package engine

import "testing"

func TestCaptureAreaCannotBeCoveredByOurOwnOverlays(t *testing.T) {
	d := FrameDisplay{Icons: []FrameIcon{{X: 20, Y: 30, Size: 20}, {X: 140, Y: 30, Size: 20}}, KickPanel: &KickPanel{}, KickBars: []KickBar{{FrameIcon: FrameIcon{X: 10, Y: 80}, Width: 200, Height: 28}}}
	ReserveCaptureArea(&d, 0, 24, 128, 176)
	if len(d.Icons) != 1 || d.Icons[0].X != 140 || len(d.KickBars) != 0 || d.KickPanel != nil || d.Omitted != 2 {
		t.Fatal(d)
	}
	d = FrameDisplay{KickPanel: &KickPanel{X: 0, Y: 180, Width: 200}, KickBars: []KickBar{{FrameIcon: FrameIcon{X: 0, Y: 206}, Width: 200, Height: 28}}}
	ReserveCaptureArea(&d, 0, 24, 128, 176)
	if d.KickPanel != nil || len(d.KickBars) != 0 {
		t.Fatal("panel header obscures capture")
	}
}

func TestCustomOverlaysKeepPlacementWhileCapturePixelsAreReserved(t *testing.T) {
	for _, kind := range []string{"text", "bar", "icon", "ring", "arc"} {
		v := OverlayDisplay{Kind: kind, Text: "Alert", X: 30, Y: 30, Size: 60, Font: 24}
		d := FrameDisplay{Width: 640, Height: 480, Overlays: []OverlayDisplay{v}, OverlaySounds: []OverlaySound{{ID: "sound"}}}
		ReserveCaptureArea(&d, 190, 140, 128, 32)
		if len(d.Overlays) != 1 || d.Overlays[0] != v || len(d.OverlaySounds) != 1 || d.Omitted != 0 {
			t.Fatal("reservation changed an alert", d)
		}
		if len(d.CaptureAreas) != 1 || d.CaptureAreas[0] != (CaptureArea{190, 140, 128, 32}) {
			t.Fatal("renderer did not receive the protected rectangle", d.CaptureAreas)
		}
		ReserveCaptureArea(&d, 190, 140, 128, 32)
		if len(d.CaptureAreas) != 1 {
			t.Fatal("duplicate reservations", d.CaptureAreas)
		}
		ReserveCaptureArea(&d, 620, 470, 128, 32)
		if len(d.CaptureAreas) != 2 || d.CaptureAreas[1] != (CaptureArea{620, 470, 20, 10}) {
			t.Fatal("reservation not clamped to screen", d.CaptureAreas)
		}
		ReserveCaptureArea(&d, 640, 480, 128, 32)
		ReserveCaptureArea(&d, 100, 100, 0, 32)
		if len(d.CaptureAreas) != 2 {
			t.Fatal("empty or offscreen reservation", d.CaptureAreas)
		}
	}
}
