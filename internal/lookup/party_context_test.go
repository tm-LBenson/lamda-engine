package lookup

import "testing"

func TestPartyContextRequiresOneBoundedPublicRecord(t *testing.T) {
	o, err := parseObservations("P|1|1|1793573400|session:round:1")
	if err != nil || o.Context == nil || !o.Context.Arena || o.Context.Epoch != "session:round:1" {
		t.Fatal(o, err)
	}
	for _, bad := range []string{"P|1|3|1793573400|x", "P|2|1|1793573400|x", "P|1|1|0|x", "P|1|1|+1793573400|x", "P|1|1|1793573400|", "P|1|1|1793573400|x\nP|1|0|1793573400|x"} {
		if _, err := parseObservations(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
	if s := DecodeSample(arenaPacket(arenaBody + "\nP|1|1|1793573400|x")); s.Error == "" {
		t.Fatal("party context entered opponent channel")
	}
}
