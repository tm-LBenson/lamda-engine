package lookup

import (
	"strings"
	"testing"
)

func TestScreenCastMetadataRejectsBadOwnerAndUnboundedRaster(t *testing.T) {
	valid := "T|1|123:1|24|0,1;1,2|7000"
	s, e := parseScreenCasts(strings.Split(valid, "|"))
	if e != nil || s.Pitch != 7 || len(s.Members) != 2 {
		t.Fatal(s, e)
	}
	for _, line := range []string{
		"T|2|123:1|24|0,1|7000", "T|1|123:1|23|0,1|7000",
		"T|1|123:1|24|0,1;0,2|7000", "T|1|123:1|24|0,1;1,1|7000",
		"T|1|123:1|24|5,1|7000", "T|1|123:1|24|0,6|7000",
		"T|1|123:1|24|0,1|0", "T|1|123:1|24|0,1|99999",
	} {
		if _, e := parseScreenCasts(strings.Split(line, "|")); e == nil {
			t.Fatal("accepted", line)
		}
	}
	if _, e := parseObservations(valid + "\n" + valid); e == nil {
		t.Fatal("duplicate mapping")
	}
	DecodeScreenCasts(s, Pixels{}, 0)
	if s.Captured {
		t.Fatal("missing image accepted")
	}
}

func TestScreenDigitsRejectEqualCandidates(t *testing.T) {
	calibration := make([]glyph, 10)
	if _, ok := digits([]glyph{{}}, calibration); ok {
		t.Fatal("ambiguous glyph accepted")
	}
}

func TestScreenStartupFailureIsReadableWithoutNativeText(t *testing.T) {
	o, e := parseObservations("S|failed|-1|0|1|Cast font unavailable\nD|0|0|0|0|0")
	if e != nil || o.ScreenStatus == nil || o.ScreenStatus.State != "failed" || o.ScreenStatus.Error != "Cast font unavailable" || o.Screen != nil {
		t.Fatal(o, e)
	}
	for _, body := range []string{"S|active|NaN|0|0|", "S|active|70000|-1|0|", "S|invented|70000|0|0|", "S|active|70000|0|0|\nS|idle|-1|0|0|"} {
		if _, e := parseObservations(body); e == nil {
			t.Fatal("accepted invalid status", body)
		}
	}
}
