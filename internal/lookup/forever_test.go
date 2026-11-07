package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"strings"
	"testing"
)

func TestForeverPacketMetadata(t *testing.T) {
	body := "test:1|1789400000|roster|0\nPlayer-1-A|us|Area 52|Alice\n\n\n\nM|1.60.1.69913|1766|1766|10000|Kick\nC|cast:1|Player-1-A|1766|0\n\n"
	packet := make([]byte, BridgeBytes)
	copy(packet, "LUF4")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE([]byte(body)))
	copy(packet[10:], body)
	sample := DecodeSample(packet)
	if sample.Error != "" || !sample.Forever || len(sample.Observations.Metadata) != 1 || len(sample.Observations.Casts) != 1 {
		t.Fatal(sample)
	}
	packet[20] ^= 1
	if DecodeSample(packet).Error == "" {
		t.Fatal("corrupted public metadata accepted")
	}
	for _, bad := range []string{"M|12.1.0.69814|1766|1766|15000|Kick", "M|1.60.1.69913|1766|1766|-1|Kick", "M|1.60.1.69913|1766|1766|7200001|Kick", "M|1.60.1.69913|1766|1766|10000|", strings.Repeat("M|1.60.1.69913|1766|1766|10000|Kick\n", 4)} {
		if _, err := parseObservations(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
