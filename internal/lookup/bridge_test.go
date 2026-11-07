package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"strings"
	"testing"
)

func TestVisibilityFlagIsOptionalAndStrict(t *testing.T) {
	for _, suffix := range []string{"", "|0", "|1"} {
		e, err := ParseEvent("boot:1|1789400000|roster|0" + suffix)
		if err != nil || e.KeepUIWhenUnfocused != (suffix == "|1") {
			t.Fatal(e, err)
		}
	}
	for _, suffix := range []string{"|true", "|2", "|", "|1|0"} {
		if _, err := ParseEvent("boot:1|1789400000|roster|0" + suffix); err == nil {
			t.Fatal(suffix)
		}
	}
}

func TestTemporaryPetKeepsDefensiveAndKickPacket(t *testing.T) {
	// Public partypet GUIDs include a shaman's summoned elemental. Rejecting
	// its Creature prefix used to discard the entire checksummed LUI4 packet.
	parts := []string{
		"1:1|1789400000|roster|0\nPlayer-1-ALICE|us|Area 52|Alice\nPlayer-1-BOB|us|Area 52|Bob",
		"1:1|1789400000|1600|1000|Player-1-ALICE|32|2|6|6|95|2|8|0|0|1|1|0|1|0\nPlayer-1-ALICE|100|400|200|40\nPlayer-1-BOB|100|500|200|40",
		"C|Cast-1|Player-1-BOB|108271|200",
		"1:1|1789400000|1600|1000|Player-1-ALICE|2|1|400|200|240|28|26|95|4|4|0|0|1|0|0\nPlayer-1-ALICE|6|250||\nPlayer-1-BOB|7|262|Creature-0-1-2-3-61029-0000000001|",
	}
	for _, magic := range []string{"LUI4", "LUF4"} {
		s := DecodeSample(simPacketFixture(magic, BridgeBytes, []byte(strings.Join(parts, "\n\n"))))
		if s.Error != "" || s.Frames == nil || s.Observations == nil || s.Kick == nil || len(s.Kick.Members) != 2 || s.Kick.Members[1].Name != "Bob" {
			t.Fatalf("temporary pet hid party data: %+v", s)
		}
	}
}

func TestLocalCapturedTrackingPacket(t *testing.T) {
	path := os.Getenv("LAMDA_PACKET_CHECK")
	if path == "" {
		t.Skip("local packet check not requested")
	}
	packet, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := DecodeSample(packet)
	if s.Error != "" || s.Frames == nil || s.Kick == nil {
		t.Fatal("captured party packet rejected:", s.Error)
	}
}

func TestBridgeChecksPacketBeforeUsingIdentity(t *testing.T) {
	body := []byte("boot:1|1789400000|key_start|1789400010\nPlayer-1-ALICE|us|Area 52|Alice")
	packet := make([]byte, BridgeBytes)
	copy(packet, "LUI1")
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
	copy(packet[10:], body)
	e, err := DecodePacket(packet)
	if err != nil || e.Kind != "key_start" || e.Members[0] != alice || e.Deadline.Sub(e.At).Seconds() != 10 {
		t.Fatal(e, err)
	}
	packet[14] ^= 1
	if _, err = DecodePacket(packet); err == nil {
		t.Fatal("corrupt pixel data accepted")
	}
	if _, err = DecodePacket(packet[:len(packet)-1]); err == nil {
		t.Fatal("partial frame accepted")
	}
}

func simPacketFixture(magic string, size int, body []byte) []byte {
	packet := make([]byte, size)
	copy(packet, magic)
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE(body))
	copy(packet[10:], body)
	return packet
}

func TestSimPacketGeometryAndVersionAreBoundTogether(t *testing.T) {
	for _, tc := range []struct {
		magic   string
		size    int
		version byte
	}{{"LUS1", BridgeBytes, '1'}, {"LUS2", SimBridgeBytes, '2'}} {
		body := make([]byte, tc.size-10)
		body[0], body[1] = tc.version, '|'
		for i := 2; i < len(body); i++ {
			body[i] = byte(i % 251)
		}
		packet := simPacketFixture(tc.magic, tc.size, body)
		sample := DecodeSample(packet)
		if sample.Error != "" || string(sample.Sim) != string(body) || sample.Event.ID != "" || sample.Frames != nil {
			t.Fatal("wrong sim result", sample.Error)
		}
		packet[len(packet)-1] ^= 1
		if DecodeSample(packet).Error != "bridge_checksum" {
			t.Fatal("corrupt final pixel accepted")
		}
		body[0] = '3'
		if DecodeSample(simPacketFixture(tc.magic, tc.size, body)).Error != "sim_bridge_version" {
			t.Fatal("mismatched header version accepted")
		}
	}
	for _, tc := range []struct {
		magic string
		size  int
	}{{"LUS2", BridgeBytes}, {"LUS1", SimBridgeBytes}, {"LUI1", SimBridgeBytes}, {"LUI4", SimBridgeBytes}, {"LUF4", SimBridgeBytes}, {"LUS3", SimBridgeBytes}} {
		if DecodeSample(simPacketFixture(tc.magic, tc.size, []byte("1|example"))).Error == "" {
			t.Fatal("protocol silently widened", tc)
		}
	}
	for n := 0; n < 10; n++ {
		if DecodeSample(make([]byte, n)).Error == "" {
			t.Fatal("short packet accepted")
		}
	}
	p, ok := (BridgePosition{16000, 16000}).clampSize(640, 480, SimBridgeWidth, SimBridgeHeight)
	if !ok || p != (BridgePosition{384, 448}) {
		t.Fatal("wide clamp", p, ok)
	}
	if _, ok = (BridgePosition{}).clampSize(255, 32, SimBridgeWidth, SimBridgeHeight); ok {
		t.Fatal("wide capture exceeds small window")
	}
}
