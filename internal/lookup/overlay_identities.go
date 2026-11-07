package lookup

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Public GUIDs bind combat-log actors to selected units. They are separate from
// the party-only clone list: aliases may share a GUID and names never identify.
type OverlayIdentity struct {
	Unit       string `json:"unit"`
	ID         string `json:"id"`
	Generation int    `json:"g"`
}

var overlayGUID = regexp.MustCompile(`^(Player|Creature|Pet|Vehicle|GameObject)-[A-Za-z0-9-]{1,110}$`)

func ValidOverlayGUID(id string) bool { return overlayGUID.MatchString(id) }
func (v *OverlayIdentity) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		var a []json.RawMessage
		if json.Unmarshal(b, &a) != nil || len(a) != 3 {
			return fmt.Errorf("invalid unit identity")
		}
		if json.Unmarshal(a[0], &v.Unit) != nil || json.Unmarshal(a[1], &v.ID) != nil || json.Unmarshal(a[2], &v.Generation) != nil {
			return fmt.Errorf("invalid unit identity")
		}
	} else {
		type plain OverlayIdentity
		if e := json.Unmarshal(b, (*plain)(v)); e != nil {
			return e
		}
	}
	if !OverlayUnitToken(v.Unit) || v.ID != "" && !ValidOverlayGUID(v.ID) || v.Generation < 1 || v.Generation > 2147483647 {
		return fmt.Errorf("invalid unit identity")
	}
	return nil
}
func sanitizeOverlayIdentities(p *OverlayInput) {
	if p.CombatVersion != 1 || len(p.Identities) > 8 || len(p.IdentityHash) != 16 {
		p.Identities = nil
		p.Logging = nil
		return
	}
	seen := map[string]bool{}
	for _, v := range p.Identities {
		if seen[v.Unit] || !OverlayUnitToken(v.Unit) || v.ID != "" && !ValidOverlayGUID(v.ID) || v.Generation < 1 {
			p.Identities = nil
			p.Logging = nil
			return
		}
		seen[v.Unit] = true
	}
}
