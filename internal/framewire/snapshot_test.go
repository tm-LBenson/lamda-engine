package framewire

import (
	"strconv"
	"strings"
	"testing"
)

const fixture = "1789400100:100:1|1789400100|1600|1000|Player-1-ALICE|32|2|6|6|95|2|8|0|0|1|1|0|1|0\nPlayer-1-ALICE|100|400|200|40\nPlayer-1-BOB|100|500|200|40"

func TestGrowthModesAndLegacyPackets(t *testing.T) {
	for _, suffix := range []string{"", "|1", "|1|41,2,6,6,100,3,0,83,0"} {
		s, err := Parse(strings.Replace(fixture, "\n", suffix+"\n", 1))
		if err != nil || s.Growth != 0 || (s.PartyLayout != nil && s.PartyLayout.Growth != 0) {
			t.Fatal(s, err)
		}
	}
	for _, custom := range []string{"1", "2", "3"} {
		for _, party := range []string{"", "41,2,6,6,100,3,0,-3,0,0", "41,2,6,6,100,3,0,-3,0,1", "41,2,6,6,100,3,0,-3,0,3"} {
			s, err := Parse(strings.Replace(fixture, "\n", "|1|"+party+"|"+custom+"\n", 1))
			if err != nil || strconv.Itoa(s.Growth) != custom || (s.PartyLayout == nil) != (party == "") {
				t.Fatal(s, err)
			}
			if party != "" && strconv.Itoa(s.PartyLayout.Growth) != party[len(party)-1:] {
				t.Fatal(s.PartyLayout)
			}
		}
	}
	for _, suffix := range []string{"|1||0", "|1||4", "|1||-1", "|1||NaN", "|1||", "|1||1|2", "|1|41,2,6,6,100,3,0,-3,0,4|1", "|1|41,2,6,6,100,3,0,-3,0,-1|1", "|1|41,2,6,6,100,3,0,-3,0,NaN|1", "|1|41,2,6,6,100,3,0,-3,0,1,2|1"} {
		if _, err := Parse(strings.Replace(fixture, "\n", suffix+"\n", 1)); err == nil {
			t.Fatal("accepted invalid growth", suffix)
		}
	}
}

func TestPersistentRowFlagIsOptionalAndValidated(t *testing.T) {
	legacy, err := Parse(fixture)
	if err != nil || legacy.Persistent {
		t.Fatal(legacy, err)
	}
	for _, flag := range []string{"0", "1", "2", "-1", "true", ""} {
		s, err := Parse(strings.Replace(fixture, "\n", "|"+flag+"\n", 1))
		if flag == "0" || flag == "1" {
			if err != nil || s.Persistent != (flag == "1") {
				t.Fatal(s, err)
			}
		} else if err == nil {
			t.Fatal("invalid row mode accepted", flag)
		}
	}
}

func TestMatchingPartyLayoutIsBounded(t *testing.T) {
	body := strings.Replace(fixture, "\n", "|1|40,2,6,6,100,3,4,83,-10\n", 1)
	s, err := Parse(body)
	if err != nil || s.PartyLayout == nil || s.PartyLayout.Size != 40 || s.PartyLayout.OffsetX != 83 || s.PartyLayout.OffsetY != -10 {
		t.Fatal(s, err)
	}
	for _, fields := range []string{"", "40,2", "0,2,6,6,100,3,4,83,-10", "40,2,6,0,100,3,4,83,-10", "40,2,6,6,100,3,4,801,-10", "40,2,6,6,100,3,4,NaN,-10", "40,2,6,6,100,3,4,83,-10,4"} {
		if _, err := Parse(strings.Replace(body, "40,2,6,6,100,3,4,83,-10", fields, 1)); err == nil {
			t.Fatal("invalid party layout accepted", fields)
		}
	}
}

func TestSnapshotValidatesCompletePublicGeometry(t *testing.T) {
	s, err := Parse(fixture)
	if err != nil || len(s.Frames) != 2 || s.Frames[1].GUID != "Player-1-BOB" || s.Anchor != 2 || !s.Unknown {
		t.Fatal(s, err)
	}
	for _, bad := range []string{
		strings.Replace(fixture, "|32|2|6", "|NaN|2|6", 1),
		strings.Replace(fixture, "|100|400|200|40", "|-1|400|200|40", 1),
		strings.Replace(fixture, "|100|400|200|40", "|1500|400|200|40", 1),
		strings.Replace(fixture, "|100|400|200|40", "|100|400|0|40", 1),
		strings.Replace(fixture, "|100|400|200|40", "|100|400|9223372036854775807|40", 1),
		strings.Replace(fixture, "|1|1|0|1|0", "|2|1|0|1|0", 1),
		strings.Replace(fixture, "Player-1-BOB", "NameWithoutGUID", 1),
		fixture + "\nPlayer-1-ALICE|100|400|200|40",
		fixture + "\nPlayer-1-BOB|100|500|200", strings.Repeat("x", 1527),
	} {
		if _, err := Parse(bad); err == nil {
			t.Fatalf("accepted invalid snapshot %q", bad)
		}
	}
}

func TestNativeOwnershipIsBoundedAndCannotChangeFrameIdentity(t *testing.T) {
	withNative := strings.Replace(fixture, "|100|400|200|40", "|100|400|200|40|48707,374227", 1)
	s, err := Parse(withNative)
	if err != nil || len(s.Frames[0].NativeSpells) != 2 || len(s.Frames[1].NativeSpells) != 0 {
		t.Fatal(s, err)
	}
	for _, ids := range []string{"", "0", "-1", "+1", "01", "10000001", "374227,374227", "374227,48707", "48707,", "48707,,374227", "secret", "1.5", "9223372036854775807"} {
		if _, err := Parse(strings.Replace(withNative, "48707,374227", ids, 1)); err == nil {
			t.Fatal("accepted native IDs", ids)
		}
	}
	for _, bad := range []string{
		fixture + "|374227", // teammate cannot claim native ownership
		withNative + "\nPlayer-1-ALICE|100|400|200|40|374227", // same rect, conflicting metadata
	} {
		if _, err := Parse(bad); err == nil {
			t.Fatal("accepted conflicting ownership", bad)
		}
	}
	ids := []string{}
	for i := 1; i <= 141; i++ {
		ids = append(ids, strconv.Itoa(i))
	}
	if _, err := Parse(strings.Replace(withNative, "48707,374227", strings.Join(ids, ","), 1)); err == nil {
		t.Fatal("unbounded ownership list")
	}
}
