package framewire

import (
	"strings"
	"testing"
)

const kickPacket = "1:1|1789500000|1600|1000|Player-1-SELF|3|1|400|200|240|28|26|95|4|4|0|0|1|0|0\nPlayer-1-SELF|1|73||100,300,200,40\nPlayer-1-FRIEND|9|266|Pet-1-DEMON|100,350,200,40"

func TestKickWireRejectsMalformedIdentityGeometryAndBounds(t *testing.T) {
	s, err := ParseKick(kickPacket)
	if err != nil || len(s.Members) != 2 || s.Members[1].Pet != "Pet-1-DEMON" {
		t.Fatal(s, err)
	}
	if s, err = ParseKick(strings.Replace(kickPacket, "Pet-1-DEMON", "Creature-0-1-2-3-61029-0000000001", 1)); err != nil || len(s.Members) != 2 {
		t.Fatal("temporary summon rejected", s, err)
	}
	for _, bad := range []string{
		strings.Replace(kickPacket, "|3|1|400|", "|4|1|400|", 1),
		strings.Replace(kickPacket, "Player-1-FRIEND", "Player-1-SELF", 1),
		strings.Replace(kickPacket, "Pet-1-DEMON", "Player-1-DEMON", 1),
		strings.Replace(kickPacket, "Pet-1-DEMON", "Vehicle-1-DEMON", 1),
		strings.Replace(kickPacket, "Pet-1-DEMON", "Creature-invalid/guid", 1),
		strings.Replace(strings.Replace(kickPacket, "Pet-1-DEMON", "Creature-1-DEMON", 1), "73||", "73|Creature-1-DEMON|", 1),
		strings.Replace(kickPacket, "100,350,200,40", "1599,350,200,40", 1),
		strings.Replace(kickPacket, "|3|1|400|", "|0|1|400|", 1),
		strings.Replace(kickPacket, "|9|266|", "|99|266|", 1),
		strings.Replace(kickPacket, "|240|28|", "|16000|28|", 1),
		strings.Replace(kickPacket, "|0|0|1|0|0", "|0|0|1|true|0", 1),
	} {
		if _, err := ParseKick(bad); err == nil {
			t.Fatal("accepted malformed kick snapshot", bad)
		}
	}
}

func TestKickCountWireOptionalAndStrict(t *testing.T) {
	for _, suffix := range []string{"", "|0", "|1"} {
		body := strings.Replace(kickPacket, "\n", suffix+"\n", 1)
		k, err := ParseKick(body)
		if err != nil || k.Counts != (suffix == "|1") {
			t.Fatal(k, err)
		}
	}
	if _, err := ParseKick(strings.Replace(kickPacket, "\n", "|true\n", 1)); err == nil {
		t.Fatal("invalid count flag accepted")
	}
}

func TestNativeOwnKickWire(t *testing.T) {
	for _, value := range []string{"-1", "0", "351338"} {
		k, err := ParseKick(strings.Replace(kickPacket, "\n", "|1|"+value+"\n", 1))
		if err != nil || !k.Counts || (value == "351338" && k.OwnSpell != 351338) {
			t.Fatal(k, err)
		}
	}
	for _, value := range []string{"-2", "10000001", "true", "1.5", "351338|0"} {
		if _, err := ParseKick(strings.Replace(kickPacket, "\n", "|1|"+value+"\n", 1)); err == nil {
			t.Fatal(value)
		}
	}
}

func TestNPCPartyMembersDoNotOwnInterruptRows(t *testing.T) {
	for _, prefix := range []string{"Creature", "Vehicle"} {
		guid := prefix + "-0-1-2-3-209057-0000000001"
		row := guid + "|1|0||100,400,200,40"
		s, err := ParseKick(kickPacket + "\n" + row)
		if err != nil || len(s.Members) != 2 || s.Members[1].Pet != "Pet-1-DEMON" {
			t.Fatal("companion blocked player/pet interrupts", s, err)
		}
		for _, bad := range []string{
			kickPacket + "\n" + row + "\n" + row,
			kickPacket + "\n" + strings.Replace(row, "|1|0|", "|99|0|", 1),
			kickPacket + "\n" + strings.Replace(row, "100,400,200,40", "1599,400,200,40", 1),
			kickPacket + "\n" + strings.Replace(row, guid, prefix+"-invalid/guid", 1),
			strings.Replace(strings.Split(kickPacket, "\n")[0], "|3|1|400|", "|0|1|400|", 1) + "\n" + row,
		} {
			if _, err := ParseKick(bad); err == nil {
				t.Fatal("malformed NPC packet accepted")
			}
		}
	}
}
