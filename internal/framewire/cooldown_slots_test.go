package framewire

import (
	"strings"
	"testing"
)

func TestOwnCooldownSlotsValidateIdentityAliasesAndBounds(t *testing.T) {
	withSlots := func(value string) string {
		return strings.Replace(fixture, "|100|400|200|40", "|100|400|200|40||"+value, 1)
	}
	s, err := Parse(withSlots("56,410,41,95:363916;13,410,41,95:374227,999999"))
	if err != nil || len(s.Frames[0].SelfSlots) != 2 || s.Frames[0].SelfSlots[1].Spells[1] != 999999 {
		t.Fatal(s, err)
	}
	empty, err := Parse(withSlots(""))
	if err != nil || empty.Frames[0].SelfSlots == nil || len(empty.Frames[0].SelfSlots) != 0 || empty.Frames[1].SelfSlots != nil {
		t.Fatal("empty learned selection became unknown", empty, err)
	}
	for _, value := range []string{
		"0,0,41,100:", "0,0,41,100:0", "0,0,41,100:01", "0,0,41,100:2,1", "0,0,41,100:1,1",
		"-1,0,41,100:1", "1590,0,41,100:1", "0,990,41,100:1", "0,0,257,100:1", "0,0,41,0:1",
		"0,0,41,100:1;0,50,41,100:1", "0,0,41,100:10000001", "secret,0,41,100:1", "0,0,41,100:1;",
		"0,0,41,100:1:2", "0,0,41,100,0:1",
	} {
		if _, err := Parse(withSlots(value)); err == nil {
			t.Fatal("invalid slots accepted", value)
		}
	}
	if _, err := Parse(fixture + "||0,0,41,100:374227"); err == nil {
		t.Fatal("teammate claimed own spellbook slots")
	}
}
