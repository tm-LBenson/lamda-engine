package lookup

import (
	"encoding/json"
	"fmt"
)

// Large packets compact public aura metadata; readings still come exclusively
// from native tiles. Older object-form packets remain supported.
func (v *OverlayAura) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || b[0] != '[' {
		type plain OverlayAura
		return json.Unmarshal(b, (*plain)(v))
	}
	var a []json.RawMessage
	if json.Unmarshal(b, &a) != nil || len(a) != 6 {
		return fmt.Errorf("invalid compact aura")
	}
	for i, target := range []any{&v.Spell, &v.Kind, &v.Unit, &v.Generation, &v.Available, &v.DurationAvailable} {
		if err := json.Unmarshal(a[i], target); err != nil {
			return err
		}
	}
	return nil
}
