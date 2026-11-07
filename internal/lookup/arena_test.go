package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"strings"
	"testing"
)

func arenaPacket(body string) []byte {
	b := make([]byte, BridgeBytes)
	copy(b, "LUA1")
	binary.BigEndian.PutUint16(b[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(b[6:10], crc32.ChecksumIEEE([]byte(body)))
	copy(b[10:], body)
	return b
}

const arenaBody = "1|1790618400:100000:1|1|1790618400|1920|1080|31|6|100|0|0\n1,62,700,300,24,Player-1-enemy;2,62,700,400,24,-\n\nT|1|panel:1|24|0,1;1,2|7000"

func TestArenaPacketNeverBecomesPartyRoster(t *testing.T) {
	s := DecodeSample(arenaPacket(arenaBody))
	if s.Error != "" || s.Arena == nil {
		t.Fatalf("decode %+v", s)
	}
	if len(s.Event.Members) != 0 || s.Frames != nil || s.Kick != nil {
		t.Fatal("arena packet contaminated party")
	}
	for i, m := range s.Observations.Screen.Members {
		if m.GUID != s.Arena.Identity(i+1) {
			t.Fatal("wrong native slot binding")
		}
	}
}
func TestArenaRejectsCrossActorAndMalformedMetadata(t *testing.T) {
	for _, c := range [][2]string{{"0,1;1,2", "1,1;0,2"}, {"0,1;1,2", "0,1;1,3"}, {"|31|", "|32|"}, {"|100|0|0", "|100|2|0"}, {"2,62,700", "1,62,700"}, {"24,-", "24,Player-1-enemy"}, {"24,-", "24,Pet-1-enemy"}, {"|7000", "|0"}, {"panel:1", "bad/panel"}} {
		s := DecodeSample(arenaPacket(strings.Replace(arenaBody, c[0], c[1], 1)))
		if s.Error == "" {
			t.Fatalf("accepted %q => %q", c[0], c[1])
		}
	}
	b := arenaPacket(arenaBody)
	b[30] ^= 1
	if DecodeSample(b).Error != "bridge_checksum" {
		t.Fatal("torn packet accepted")
	}
	// A party cast/build block cannot be smuggled into the arena channel.
	s := DecodeSample(arenaPacket(arenaBody + "\nC|Player-1-enemy|642|event:1|0"))
	if s.Error == "" {
		t.Fatal("party record accepted")
	}
}
func TestArenaActualLuaPacket(t *testing.T) {
	p := os.Getenv("LAMDA_ARENA_PACKET")
	if p == "" {
		t.Skip("Lua fixture path not supplied")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := DecodeSample(data)
	if s.Error != "" || s.Arena == nil || len(s.Arena.Actors) != 2 || s.Observations.Screen == nil {
		t.Fatalf("Lua packet %+v", s)
	}
	if s.Arena.Actors[0].Size != 39 || s.Arena.Actors[0].GUID != "" {
		t.Fatal("physical scale/opaque GUID contract")
	}
}
