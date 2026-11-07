package lookup

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestAuraTilesRejectOcclusionAndDoNotConfuseSlots(t *testing.T) {
	a := &AuraWatch{Members: []AuraWatchMember{{Slot: 4, Class: 6, Mask: 31}}}
	pixels := make([]byte, 768)
	for i := 0; i < 5; i++ {
		slot := 32 + i
		for cell := 0; cell < 4; cell++ {
			c := AuraWatchColor(slot, cell, i == 2)
			off := ((slot/32*2+cell/2)*64 + (slot%32)*2 + cell%2) * 3
			copy(pixels[off:], c[:])
		}
	}
	if !DecodeAuraWatch(a, pixels) || !a.Active[34] || a.Active[33] {
		t.Fatal(a)
	}
	pixels[390] ^= 1
	a.Captured = false
	if DecodeAuraWatch(a, pixels) || a.Captured {
		t.Fatal("occluded colors accepted")
	}
}

func TestAuraMappingRequiresRosterAndValidMasks(t *testing.T) {
	base := "1:1|1789400100|roster|0\nPlayer-1-BOB|us|Draenor|Bob\n\n\n\nA|1|2:1|16|1,1,6,31\n\n"
	pack := func(body string) []byte {
		p := make([]byte, BridgeBytes)
		copy(p, "LUI4")
		copy(p[10:], body)
		binary.BigEndian.PutUint16(p[4:6], uint16(len(body)))
		binary.BigEndian.PutUint32(p[6:10], crc32.ChecksumIEEE([]byte(body)))
		return p
	}
	s := DecodeSample(pack(base))
	if s.Error != "" || s.Observations.Auras.Members[0].GUID != "Player-1-BOB" {
		t.Fatal(s)
	}
	for _, line := range []string{"A|1|x|16|5,1,6,1", "A|1|x|16|1,1,6,32", "A|1|x|0|1,1,6,1", "A|1|x|16|1,1,6,1;1,2,6,1", "A|1|x|16|1,1,6,1;2,1,6,1"} {
		if _, e := parseObservations(line); e == nil {
			t.Fatal("accepted", line)
		}
	}
	if s = DecodeSample(pack("1:1|1789400100|roster|0\n\n\n\nA|1|2:1|16|1,1,6,1\n\n")); s.Error != "aura_member_not_in_roster" {
		t.Fatal(s)
	}
}

func TestNativeLuaAuraFixture(t *testing.T) {
	path := os.Getenv("LAMDA_AURA_PACKETS")
	if path == "" {
		t.Skip("Lua fixture not requested")
	}
	packet, e := os.ReadFile(filepath.Join(path, "packet.bin"))
	if e != nil {
		t.Fatal(e)
	}
	s := DecodeSample(packet)
	if s.Error != "" || s.Observations.Auras == nil {
		t.Fatal(s)
	}
	for _, state := range []string{"off", "on"} {
		raw, e := os.ReadFile(filepath.Join(path, state+".bin"))
		if e != nil {
			t.Fatal(e)
		}
		s = DecodeSample(packet)
		if !DecodeAuraWatch(s.Observations.Auras, raw) {
			t.Fatal("native color/layout mismatch", state)
		}
		for _, m := range s.Observations.Auras.Members {
			for i := range AuraWatchSpells[m.Class] {
				if m.Mask&(1<<i) != 0 && s.Observations.Auras.Active[m.Slot*8+i] != (state == "on") {
					t.Fatal("native slot mismatch")
				}
			}
		}
	}
}
