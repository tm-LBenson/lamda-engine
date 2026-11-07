package lookup

import (
	"context"
	"encoding/binary"
	"hash/crc32"
	"os"
	"strings"
	"testing"
	"time"
)

func controlFixture(body string) []byte {
	p := make([]byte, 384)
	copy(p, "LMC1")
	binary.BigEndian.PutUint16(p[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(p[6:10], crc32.ChecksumIEEE([]byte(body)))
	copy(p[10:], body)
	return p
}

func TestCompactHistoryScopesStayBoundedAndUseModernPreviewLease(t *testing.T) {
	valid := []string{"history:1:100:all", "history:25:25:key", "history:2:200:boss", "saved:" + strings.Repeat("a", 32), "savedu:" + strings.Repeat("b", 32)}
	for _, scope := range valid {
		body := "4|history_1|Player-1-Aug|1800000030|SELF|" + scope + "|0|1|request|1|0|0|0|0"
		n, err := parseMeterControl(controlFixture(body))
		if err != nil || n.ShareRequest.Scope != scope || n.Captured {
			t.Fatal(scope, err, n)
		}
		if _, err := parseMeterControl(controlFixture(strings.Replace(body, "4|", "3|", 1))); err == nil {
			t.Fatal("legacy receiver accepted history scope")
		}
	}
	for _, scope := range []string{"history:0:100:all", "history:26:100:all", "history:1:999:all", "history:1:26:all", "history:1:100:raid", "saved:" + strings.Repeat("a", 33), "saved:" + strings.Repeat("A", 32), "saved:../state.json", "/reload"} {
		body := "4|history_1|Player-1-Aug|1800000030|SELF|" + scope + "|0|1|request|1|0|0|0|0"
		if _, err := parseMeterControl(controlFixture(body)); err == nil {
			t.Fatal(scope)
		}
	}
}
func TestControlPacketContainsRequestButNeverCapturedDamage(t *testing.T) {
	body := "1|unique_1|Player-1-Aug|1800000030|PARTY|boss-uptime|4294967295"
	n, err := parseMeterControl(controlFixture(body))
	if err != nil || !n.ControlOnly || n.Captured || len(n.Members) != 0 || n.ShareRequest.Scope != "boss-uptime" || n.ShareRequest.Roster != 4294967295 {
		t.Fatal(n, err)
	}
	for _, v := range []string{strings.Replace(body, "unique_1", "bad token", 1), strings.Replace(body, "PARTY", "SAY", 1), strings.Replace(body, "boss-uptime", "/reload", 1), strings.Replace(body, "4294967295", "4294967296", 1), strings.Replace(body, "Player-1-Aug", "Pet-1", 1), strings.Replace(body, "1800000030", "0", 1)} {
		if _, err := parseMeterControl(controlFixture(v)); err == nil {
			t.Fatal("bad request accepted", v)
		}
	}
	damaged := controlFixture(body)
	damaged[12] ^= 1
	if _, err := parseMeterControl(damaged); err == nil {
		t.Fatal("corrupt CRC accepted")
	}
}
func TestControlPointerHasExactSmallDimensions(t *testing.T) {
	p := MeterBorderPointer{X: 130, Y: 24, Width: 64, Height: 8, ControlOnly: true}
	got, err := ParseMeterBorderPointer(p.Encode())
	if err != nil || got != p {
		t.Fatal(got, err)
	}
	p.Width = 128
	if _, err := ParseMeterBorderPointer(p.Encode()); err == nil {
		t.Fatal("unbounded control dimensions")
	}
	p = MeterBorderPointer{Width: 396, Height: 236, ShareCapable: true}
	got, err = ParseMeterBorderPointer(p.Encode())
	if err != nil || got != p {
		t.Fatal(got, err)
	}
	p.ShareOpenVersion = 2
	got, err = ParseMeterBorderPointer(p.Encode())
	if err != nil || got != p {
		t.Fatal(got, err)
	}
}
func TestSummaryTransportWorksWithoutNumericCapture(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, open := range []bool{false, true} {
		n, _ := parseMeterControl(controlFixture("1|unique_1|Player-1-Aug|1800000030|PARTY|key|42"))
		message := "[LamdaMeters] 1. Aug 100K~[LamdaMeters] 2. Ally 90K"
		if open {
			n = &NativeMeter{ControlOnly: true, ShareCapable: true}
			message = ""
		}
		sample := BridgeSample{Window: 10, NativeMeter: n}
		calls := 0
		p := meterSharePlatform{now: func() time.Time { return now }, foreground: func() uintptr { return 10 }, executable: func(uintptr) string { return "Wow.exe" }, matchesGame: func(string, string, string) bool { return true }, capture: func() BridgeSample { return sample }, keyDown: func(uint16) bool { return false }, open: func() error { calls++; return nil }, text: func(text string) error {
			calls++
			if !strings.HasSuffix(text, "[LM!unique_1]") {
				t.Fatal(text)
			}
			return nil
		}}
		if err := sendMeterSummary(context.Background(), sample, "WoW", "retail", message, p); err != nil || calls != 1 {
			t.Fatal(err, calls)
		}
		// Ordinary unavailable meter samples still cannot authorize input.
		n.ControlOnly = false
		if sendMeterSummary(context.Background(), sample, "WoW", "retail", message, p) == nil || calls != 1 {
			t.Fatal("uncaptured non-control source authorized input")
		}
	}
}

func TestMeterControlActualLuaFixture(t *testing.T) {
	path := os.Getenv("LAMDA_CONTROL_FIXTURE")
	if path == "" {
		t.Skip("actual Lua fixture not supplied")
	}
	packet, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err := parseMeterControl(packet)
	if err != nil || n.Player != "Player-1-ALICE" || n.ShareRequest.Token != "nonce_1" || n.Captured || len(n.Members) > 0 {
		t.Fatal(n, err)
	}
}

func TestControlV2PublishesFocusProgressAndReceiptWithoutCounters(t *testing.T) {
	base := "2|unique_1|Player-1-Aug|1800000030|SELF|key|42|1|request|1|0"
	n, err := parseMeterControl(controlFixture(base))
	if err != nil || n.ShareRequest.TransferVersion != 2 || n.ShareRequest.FocusBlocked || !n.ControlHidden || n.Captured {
		t.Fatal(n, err)
	}
	for _, phase := range []string{"ready", "error"} {
		packet := strings.Replace(base, "|1|request|1|0", "|0|"+phase+"|1|100", 1)
		n, err := parseMeterControl(controlFixture(packet))
		if err != nil || n.ShareAck != "unique_1" || n.ShareRequest != nil {
			t.Fatal(n, err)
		}
	}
	for _, suffix := range []string{"|2|request|1|0", "|1|posting|1|0", "|1|request|2|0", "|1|request|1|13001", "|1|request|1|-1"} {
		if _, err := parseMeterControl(controlFixture(strings.TrimSuffix(base, "|1|request|1|0") + suffix)); err == nil {
			t.Fatal(suffix)
		}
	}
}

func TestControlV3ReservesFunctionKeysWithoutEditFocus(t *testing.T) {
	body := "3|unique_1|Player-1-Aug|1800000030|RAID|boss|42|1|request|1|123"
	n, err := parseMeterControl(controlFixture(body))
	if err != nil || n.ShareRequest.TransferVersion != 3 || n.ShareRequest.FocusBlocked || n.ShareRequest.ReceivedBytes != 123 {
		t.Fatal(n, err)
	}
	ready := strings.Replace(body, "|1|request|1|123", "|0|ready|1|1000", 1)
	n, err = parseMeterControl(controlFixture(ready))
	if err != nil || n.ShareAck != "unique_1" || n.ShareRequest != nil {
		t.Fatal(n, err)
	}
}
