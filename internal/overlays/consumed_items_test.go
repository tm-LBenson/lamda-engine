package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
)

func TestConsumedItemCooldownReady(t *testing.T) {
	q := Node{Type: "if", Props: map[string]any{"condition": "cooldown", "source": "item", "id": "5511", "check": "cooldownReady"}}
	p := &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{{Kind: "item", ID: 5511, Available: true, Carried: ptr(false), Remaining: ptr(90.0), Enabled: ptr(false)}}}
	if condition(q, p, "player") {
		t.Fatal("consumed item cooldown ignored")
	}
	p.Cooldowns[0].Remaining = ptr(0.1)
	if condition(q, p, "player") {
		t.Fatal("positive fraction became ready")
	}
	p.Cooldowns[0].Remaining = ptr(0.0)
	if !condition(q, p, "player") {
		t.Fatal("finished cooldown required carried item")
	}
	q.Props["check"] = "ready"
	if condition(q, p, "player") {
		t.Fatal("normal ready matched missing item")
	}
	q.Props["check"] = "cooldownReady"
	p.Cooldowns[0].Remaining = nil
	if condition(q, p, "player") || condition(Node{Type: "if", Props: group("not", q.Props)}, p, "player") {
		t.Fatal("unknown cooldown became known")
	}
	for _, kind := range []string{"spell", "slot", "gcd"} {
		r := cooldownRule()
		r.Roots[0].Children[0].Props = map[string]any{"condition": "cooldown", "source": kind, "id": "5511", "check": "cooldownReady"}
		if Validate([]Rule{r}) == nil {
			t.Fatal("non-item cooldownReady accepted")
		}
	}
}
