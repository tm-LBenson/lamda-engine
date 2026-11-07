package lookup

import (
	"context"
	"hash/crc32"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExtendedReportControlCarriesOptionsCompactCapabilityAndBoundary(t *testing.T) {
	for _, version := range []string{"3", "4"} {
		body := version + "|unique_1|Player-1-Aug|1800000030|RAID|boss|42|1|request|1|0|3|1800000000|3497"
		n, err := parseMeterControl(controlFixture(body))
		if err != nil || n.ShareRequest.Options != 3 || !n.ShareRequest.Compact || n.ShareRequest.EndedAt != 1800000000 || n.ShareRequest.EncounterID != "3497" {
			t.Fatal(n, err)
		}
		for _, change := range []string{"5|1800000000|3497", "8|1800000000|3497", "03|1800000000|3497", "3|-1|3497", "3|bad|3497", "3|1800000000|bad"} {
			if _, err := parseMeterControl(controlFixture(strings.Replace(body, "3|1800000000|3497", change, 1))); err == nil {
				t.Fatal(change)
			}
		}
		r := *n.ShareRequest
		for _, change := range []string{"option", "boundary", "encounter", "capability"} {
			other := r
			switch change {
			case "option":
				other.Options = 0
			case "boundary":
				other.EndedAt++
			case "encounter":
				other.EncounterID = "123"
			case "capability":
				other.Compact = false
			}
			if sameSummaryLease(r, other) {
				t.Fatal("changed lease accepted", change)
			}
		}
	}
}

func TestRankingsOnlyControlRemainsBoundToThePreviewLease(t *testing.T) {
	body := "4|unique_1|Player-1-Aug|1800000030|RAID|boss|42|1|request|1|0|4|1800000000|3497"
	n, err := parseMeterControl(controlFixture(body))
	if err != nil || n.ShareRequest.Options != 4 {
		t.Fatal(n, err)
	}
	changed := *n.ShareRequest
	changed.Options = 0
	if sameSummaryLease(*n.ShareRequest, changed) {
		t.Fatal("rankings-only request reused a combined report lease")
	}
}

func TestCompactTransferRequiresCapabilityAndPreservesExactUTF8Report(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, version := range []int{3, 4} {
		for _, compact := range []bool{false, true} {
			r := &MeterShareRequest{Token: "unique_1", Expires: now.Unix() + 20, TransferVersion: version, Compact: compact, Scope: "boss", Channel: "RAID"}
			n := &NativeMeter{ControlOnly: true, Player: "Player-1-Aug", ShareRequest: r}
			sample := BridgeSample{Window: 10, NativeMeter: n}
			text := "[LamdaMeters] Boss - DPS~[LamdaMeters] 1. 界Я 110K DPS"
			p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return sample }, keyDown: func(uint16) bool { return false }, ack: func(context.Context, string, string) error { return nil }, text: func(wire string) error {
				size, _ := strconv.Atoi(wire[1:7])
				checksum, _ := strconv.ParseUint(wire[7:17], 10, 32)
				if len(wire) != 17+size*3 {
					t.Fatal("bad frame length")
				}
				var body strings.Builder
				for i := 17; i < len(wire); i += 3 {
					v, e := strconv.Atoi(wire[i : i+3])
					if e != nil || v > 255 {
						t.Fatal(wire)
					}
					body.WriteByte(byte(v))
				}
				if crc32.ChecksumIEEE([]byte(body.String())) != uint32(checksum) {
					t.Fatal("bad checksum")
				}
				want := text + "[LM!unique_1]"
				if compact {
					want = "[LM3] Boss - DPS~1. 界Я 110K DPS[LM!unique_1]"
				}
				if body.String() != want {
					t.Fatal(body.String())
				}
				return nil
			}}
			if err := sendMeterSummary(context.Background(), sample, "WoW", "retail", text, p); err != nil {
				t.Fatal(err)
			}
		}
	}
}
