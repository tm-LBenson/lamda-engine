package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
)

func TestInventoryAbsenceIsNotUnknownAvailability(t *testing.T) {
	q := Node{Type: "if", Props: map[string]any{"condition": "cooldown", "source": "item", "id": "5511", "check": "missing"}}
	for _, tc := range []struct {
		name             string
		item             lookup.OverlayCooldown
		missing, carried bool
	}{
		{"absent", lookup.OverlayCooldown{Carried: ptr(false)}, true, false},
		{"present without cooldown API", lookup.OverlayCooldown{Carried: ptr(true)}, false, true},
		{"unknown", lookup.OverlayCooldown{}, false, false},
		{"legacy confirmed present", lookup.OverlayCooldown{Available: true}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.item.Kind = "item"
			tc.item.ID = 5511
			p := &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{tc.item}}
			q.Props["check"] = "missing"
			if condition(q, p, "player") != tc.missing {
				t.Fatal("wrong missing state")
			}
			q.Props["check"] = "carried"
			if condition(q, p, "player") != tc.carried {
				t.Fatal("wrong carried state")
			}
			if tc.name == "unknown" && condition(Node{Type: "if", Props: group("not", q.Props)}, p, "player") {
				t.Fatal("NOT unknown became missing")
			}
		})
	}
}
