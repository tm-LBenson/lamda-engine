package lookup

import (
	"strings"
	"testing"
)

func TestNativeWindowMetadataCarriesIndependentScaledControlsAndStyle(t *testing.T) {
	head := "7|123:45:1|7|68|Player-1-Aug|1473|0|6000|32|10000|100|200|255|160|overall|1|0|0|0|0|2|2|-|0|-|-|0|-|-|1|1|2|45|18|9|880|percent|1|0|Overall"
	row := "\n1|Player-1-Aug|13|Aug|0"
	n := parseNativeMeter([]byte(head + row)).NativeMeter
	if n == nil || n.Frame.WindowID != 2 || n.Frame.Width != 255 || n.Frame.HeaderHeight != 45 || n.Frame.RowHeight != 18 || n.Frame.FontSize != 9 || n.Frame.Opacity != 880 || n.Frame.Values != "percent" || !n.Frame.ClassColors || n.Frame.Label != "Overall" {
		t.Fatal(n)
	}
	for index, value := range map[int]string{31: "0", 32: "0", 33: "7", 34: "NaN", 35: "1001", 36: "invalid", 37: "2", 38: "2", 39: ""} {
		p := strings.Split(head, "|")
		p[index] = value
		if parseNativeMeter([]byte(strings.Join(p, "|")+row)).NativeMeter != nil {
			t.Fatal("invalid window metadata accepted", index, value)
		}
	}
	p := strings.Split(head, "|")
	p[0] = "6"
	if parseNativeMeter([]byte(strings.Join(p[:31], "|")+row)).NativeMeter != nil {
		t.Fatal("legacy geometry rules weakened")
	}
}

func TestWindowHiddenLocatorDoesNotImplyGlobalHidden(t *testing.T) {
	p := MeterBorderPointer{Width: 832, Height: 24, WindowHidden: true}
	v, err := ParseMeterBorderPointer(p.Encode())
	if err != nil || !v.WindowHidden || v.Inactive || v.Paused || v.ControlOnly {
		t.Fatal(v, err)
	}
}
