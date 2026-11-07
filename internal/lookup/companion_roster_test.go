package lookup

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const companionGUID = "Creature-0-1-2-3-209057-0000000001"

func TestOlderCompanionRosterPreservesPlayerBindings(t *testing.T) {
	parts := []string{
		"1:1|1789400000|roster|0\n" + companionGUID + "|us|Area 52|NPC Companion\nPlayer-1-ALICE|us|Area 52|Alice",
		"1:1|1789400000|1600|1000|Player-1-ALICE|32|2|6|6|95|2|8|0|0|1|1|0|1|0\nPlayer-1-ALICE|100|400|200|40\n" + companionGUID + "|100|500|200|40",
		"A|1|2:1|16|1,1,6,31\nC|Cast-1|Player-1-ALICE|108271|200",
		"1:1|1789400000|1600|1000|Player-1-ALICE|2|1|400|200|240|28|26|95|4|4|0|0|1|0|0\n" + companionGUID + "|1|0||\nPlayer-1-ALICE|9|266|Pet-1-DEMON|",
	}
	for _, magic := range []string{"LUI4", "LUF4", "LCD1", "FCD1", "LKI1", "FKI1"} {
		t.Run(magic, func(t *testing.T) {
			p := append([]string(nil), parts...)
			if magic == "LCD1" || magic == "FCD1" {
				p[3] = ""
			}
			if magic == "LKI1" || magic == "FKI1" {
				p[1], p[2] = "", "C|Cast-1|Player-1-ALICE|19647|200"
			}
			s := DecodeSample(simPacketFixture(magic, BridgeBytes, []byte(strings.Join(p, "\n\n"))))
			if s.Error != "" || len(s.Event.Members) != 1 || s.Event.Members[0] != alice {
				t.Fatalf("player roster lost: %+v", s)
			}
			if p[1] != "" && (s.Frames == nil || len(s.Frames.Frames) != 2 || s.Frames.Frames[1].GUID != companionGUID) {
				t.Fatal("companion frame lost")
			}
			if p[3] != "" && (s.Kick == nil || len(s.Kick.Members) != 1 || s.Kick.Members[0].Name != "Alice" || s.Kick.Members[0].Pet != "Pet-1-DEMON") {
				t.Fatal("player or pet ownership lost")
			}
			if a := s.Observations.Auras; a != nil && a.Members[0].GUID != alice.GUID {
				t.Fatal("aura bound to NPC")
			}
			if len(s.Observations.Casts) != 1 || s.Observations.Casts[0].GUID != alice.GUID {
				t.Fatal("player cast lost")
			}
		})
	}
}

func TestCompanionRosterNeverQueuesNPCProfile(t *testing.T) {
	q, f := testQueue(t)
	now := time.Now()
	e, err := ParseEvent(fmt.Sprintf("1:1|%d|roster|0\n%s|us|Area 52|NPC Companion\nPlayer-1-ALICE|us|Area 52|Alice", now.Unix(), companionGUID))
	if err != nil {
		t.Fatal(err)
	}
	q.Handle(e, now)
	q.Advance(now)
	call := takeCall(t, f)
	if call.Character != alice {
		t.Fatal("NPC lookup", call.Character)
	}
	finish(t, q, call, nil)
	q.Advance(now.Add(3 * time.Second))
	if len(q.pending) != 0 || len(f.Calls) != 0 || q.stats.Requests != 1 {
		t.Fatal("extra lookup", q.Status())
	}
}

func TestCompanionCompatibilityKeepsMalformedMembersRejected(t *testing.T) {
	for _, guid := range []string{"Creature-invalid/guid", "Creature-", "Pet-1-PET", "Vehicle-nope", "Other-1-NPC"} {
		if _, err := ParseEvent("1:1|1789400000|roster|0\n" + guid + "|us|Area 52|NPC"); err == nil {
			t.Fatal("invalid member accepted", guid)
		}
	}
	for _, row := range []string{"Player-1-A|us||Alice", "Player-1-A|xx|Area 52|Alice", companionGUID + "|us|Area 52|NPC|extra"} {
		if _, err := ParseEvent("1:1|1789400000|roster|0\n" + row); err == nil {
			t.Fatal("malformed member accepted", row)
		}
	}
}

func TestLocalCompanionCapture(t *testing.T) {
	path := os.Getenv("LAMDA_COMPANION_PACKET")
	if path == "" {
		t.Skip("local capture not requested")
	}
	packet, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := DecodeSample(packet)
	if s.Error != "" || s.Frames == nil || len(s.Event.Members) == 0 {
		t.Fatal("live companion packet rejected:", s.Error)
	}
	for _, c := range s.Event.Members {
		if !c.Valid() {
			t.Fatal("nonplayer retained")
		}
	}
}
