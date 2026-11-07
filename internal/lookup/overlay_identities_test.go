package lookup

import (
	"encoding/json"
	"testing"
)

func TestOverlayCombatPublicIdentityAliasesAndMissingGUID(t *testing.T) {
	var p OverlayInput
	if e := json.Unmarshal([]byte(`{"combatVersion":1,"identityHash":"1234567812345678","logging":true,"identities":[["player","Player-1-A",1],["target","Player-1-A",2],["focus","",4]]}`), &p); e != nil {
		t.Fatal(e)
	}
	sanitizeOverlayIdentities(&p)
	if len(p.Identities) != 3 || p.Logging == nil || p.Identities[2].ID != "" {
		t.Fatal(p.Identities)
	}
	p.Identities = append(p.Identities, p.Identities[0])
	sanitizeOverlayIdentities(&p)
	if len(p.Identities) != 0 || p.Logging != nil {
		t.Fatal("duplicate unit")
	}
	for _, s := range []string{`["target","not-a-guid",1]`, `["target","Player-1-A",0]`, `["nameplate41","Player-1-A",1]`, `["target","Player-1-A",1,2]`} {
		var v OverlayIdentity
		if json.Unmarshal([]byte(s), &v) == nil {
			t.Fatal(s)
		}
	}
}
func TestCompactAuraMetadataRetainsShapeWithoutNativeReadings(t *testing.T) {
	var v OverlayAura
	if e := json.Unmarshal([]byte(`[9999999,"debuff","nameplate40",5,true,true]`), &v); e != nil {
		t.Fatal(e)
	}
	if v.Spell != 9999999 || v.Unit != "nameplate40" || v.Generation != 5 || !v.Available || !v.DurationAvailable || v.Active != nil || v.Stacks != nil {
		t.Fatal(v)
	}
	for _, s := range []string{`[1,"buff","player",1,true]`, `[1,"buff","player",1,true,true,123]`, `[1,"buff","player",1,"true",true]`} {
		if json.Unmarshal([]byte(s), &v) == nil {
			t.Fatal(s)
		}
	}
}
