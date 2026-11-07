package lookup

import (
	"context"
	"strings"
	"testing"
)

const sharedWindow = "@2|overlay:alias:1|17|500|200|400|240|1|0|0|0|0|45|20|11|600|percent|0|0"

func TestSharedCaptureKeepsIndependentWindowSettingsAndNavigation(t *testing.T) {
	p, _ := meterViewFixtureFont(5, true, true, sharedWindow)
	s := DecodeMeterBorder(p)
	n := s.NativeMeter
	if n == nil || !n.Captured || len(n.Aliases) != 1 {
		t.Fatalf("%+v %+v", s, n)
	}
	s.MeterSlot = 0
	values := expandMeterAliases(s)
	if len(values) != 2 {
		t.Fatal(values)
	}
	a := values[1]
	v := a.NativeMeter
	if v.Frame.WindowID != 2 || v.Frame.X != 500 || v.Frame.Values != "percent" || v.Frame.ClassColors || v.Frame.Opacity != 600 || v.ID != "overlay:alias:1" || v.Serial != 17 || a.MeterSlot != 1 || meterCaptureSlot(a) != 0 || v.Members[0].Total != 418240 || v.View.Rows[0].Total != 418240 {
		t.Fatalf("%+v %+v", a, v)
	}
	v.Members[0].Total = 123
	v.View.Rows[0].Total = 456
	if n.Members[0].Total != 418240 || n.View.Rows[0].Total != 418240 {
		t.Fatal("window mutable data crossed")
	}
	fresh := meterSampleForWindow(s, 2)
	command := MeterCommand{WindowID: 2, Serial: 17, Epoch: v.ID, Action: "actor", Value: "Player-1-Aug"}
	if !meterCommandMatches(fresh.NativeMeter, command) || meterCaptureSlot(fresh) != 0 {
		t.Fatal("alias navigation lost physical source")
	}
	ready, e := awaitMeterNavigationFrame(context.Background(), fresh, command, func() BridgeSample { return meterSampleForWindow(s, 2) })
	if e != nil || ready.MeterCaptureSlot != 1 || ready.MeterSlot != 1 {
		t.Fatal(ready, e)
	}
}

func TestSharedMetadataRejectsDuplicateWindowsAndInvalidStyles(t *testing.T) {
	for _, extra := range [][]string{{strings.Replace(sharedWindow, "@2", "@1", 1)}, {sharedWindow, sharedWindow}, {strings.Replace(sharedWindow, "|11|600|", "|999|600|", 1)}} {
		p, _ := meterViewFixtureFont(1, true, true, extra...)
		n := DecodeMeterBorder(p).NativeMeter
		if n != nil && n.Captured {
			t.Fatal(extra)
		}
	}
	p, _ := meterViewFixtureFont(1, false, true, sharedWindow)
	n := DecodeMeterBorder(p).NativeMeter
	if n != nil && n.Captured {
		t.Fatal("breakdown row bindings were shared")
	}
}

func TestSharedWindowCannotRegressWhenItGetsItsOwnCapture(t *testing.T) {
	last := map[int]nativeWindowRevision{}
	n := &NativeMeter{Captured: true, ID: "overlay:100:10:w2:1", Serial: 10, Player: "Player-1-Aug", Frame: &NativeMeterFrame{WindowID: 2}}
	s := BridgeSample{NativeMeter: n, MeterCaptureSlot: 1, MeterSlot: 1}
	if !acceptNativeWindowRevision(last, s) {
		t.Fatal("initial shared view")
	}
	n.ID = "overlay:100:10:w2:2"
	n.Serial = 11
	s.MeterCaptureSlot = 0
	if !acceptNativeWindowRevision(last, s) {
		t.Fatal("independent metric change")
	}
	n.ID = "overlay:100:10:w2:1"
	n.Serial = 10
	s.MeterCaptureSlot = 1
	if acceptNativeWindowRevision(last, s) {
		t.Fatal("late shared metric restored")
	}
	n.ID = "overlay:101:10:w2:1"
	n.Serial = 1
	if !acceptNativeWindowRevision(last, s) {
		t.Fatal("reload did not reset capture sequence")
	}
	n.Serial = 16777215
	last[2] = nativeWindowRevision{n.Player, "overlay:101:10:w2", 16777214}
	if !acceptNativeWindowRevision(last, s) {
		t.Fatal("last serial")
	}
	n.Serial = 1
	if !acceptNativeWindowRevision(last, s) {
		t.Fatal("serial wrap")
	}
}
