package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"strings"
	"testing"
)

func TestObservationPacketValidation(t *testing.T) {
	body := "1:1|1789400100|roster|0\nPlayer-1-BOB|us|Draenor|Bob\n\n\n\nC|Cast-1|Player-1-BOB|108271|200\nB|1:1|Player-1-BOB|1789400090|264|12.1.0|1|1|1,101,1,108271\nD|1|2|0|1|0"
	packet := make([]byte, BridgeBytes)
	copy(packet, "LUI3")
	copy(packet[10:], body)
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE([]byte(body)))
	sample := DecodeSample(packet)
	if sample.Error != "" || sample.Observations == nil || len(sample.Observations.Casts) != 1 || sample.Observations.Casts[0].AgeMillis != 200 || len(sample.Observations.Builds) != 1 || sample.Frames != nil {
		t.Fatal(sample)
	}
	for _, body := range []string{
		"C|Cast|Player-1-BOB|108271|-1", "C|Cast|Creature-1-X|108271|1", "C|Cast|Player-1-BOB|108271|30001",
		"C|Cast|Player-1-BOB|108271|1\nBAD", "B|1:1|Player-1-BOB|1789400090|264|12.1.0|2|1|",
		"B|1:1|Player-1-BOB|1789400090|264|unknown|1|1|", "D|0|0|0|0|0\nD|0|0|0|0|0", strings.Repeat("x", 641),
	} {
		if _, err := parseObservations(body); err == nil {
			t.Fatal("accepted", body)
		}
	}
	packet[20] ^= 1
	if DecodeSample(packet).Error != "bridge_checksum" {
		t.Fatal("unchecked corruption")
	}
}
