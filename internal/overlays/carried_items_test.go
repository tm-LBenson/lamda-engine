package overlays

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Reproduce the live Forever packet: a confirmed bag item with unreadable
// native timer pixels. Presence must not require a timer, enabled or usable.
func TestCarriedItemWithUnreadableTimer(t *testing.T) {
	r := cooldownRule()
	q := &r.Roots[0].Children[0]
	q.Props = map[string]any{"condition": "cooldown", "source": "item", "id": "5232", "check": "carried"}
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	p := &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{{Kind: "item", ID: 5232, Available: true}}}
	if !condition(*q, p, "player") {
		t.Fatal("confirmed carried item required timer pixels")
	}
	p.Cooldowns[0].Enabled = ptr(false)
	p.Cooldowns[0].Usable = ptr(false)
	p.Cooldowns[0].Remaining = ptr(900.0)
	if !condition(*q, p, "player") {
		t.Fatal("cooldown or unusability erased bag presence")
	}
	p.Cooldowns[0].Available = false
	if condition(*q, p, "player") {
		t.Fatal("unavailable item matched")
	}
	not := Node{Type: "if", Props: group("not", q.Props)}
	if condition(not, p, "player") {
		t.Fatal("unavailable item became confirmed missing")
	}
	for _, kind := range []string{"spell", "slot", "gcd"} {
		q.Props["source"] = kind
		if Validate([]Rule{r}) == nil {
			t.Fatal("carried accepted non-item source", kind)
		}
	}
}

func TestCarriedSoulstoneRuntimeTransitions(t *testing.T) {
	r := cooldownRule()
	r.Roots[0].Props["event"] = "state"
	r.Roots[0].Children[0].Props = group("all",
		map[string]any{"condition": "cooldown", "source": "item", "id": "5232", "check": "carried"},
		map[string]any{"condition": "buff", "unit": "player", "aura": "20707", "check": "missing"})
	r.Roots[0].Children[0].Children = r.Roots[0].Children[0].Children[:1]
	rules := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	s := sample(rules, now, 1)
	p := s.Overlays
	p.CooldownsVersion = 1
	p.CooldownHash = CooldownHash([]CooldownWatch{{"item", 5232}})
	p.Cooldowns = []lookup.OverlayCooldown{{Kind: "item", ID: 5232, Available: true}}
	p.Watches = []lookup.OverlayAura{{Spell: 20707, Kind: "buff", Unit: "player", Available: true, Active: ptr(false)}}
	send := func(want int) {
		rt.Accept(rules, s, now)
		if len(rt.displays) != want {
			t.Fatalf("seq %d: got %d displays, want %d", p.Seq, len(rt.displays), want)
		}
		p.Seq++
		now = now.Add(200 * time.Millisecond)
		p.At = now.Unix()
	}
	send(1)
	p.Watches[0].Active = ptr(true)
	send(0)
	p.Watches[0].Active = ptr(false)
	send(1)
	p.Cooldowns[0].Available = false
	send(0)
	p.Cooldowns[0].Available = true
	p.Watches[0].Active = nil
	send(0)
	p.Watches[0].Active = ptr(false)
	send(1)
	rt.advance(now.Add(4 * time.Second))
	if len(rt.displays) != 0 {
		t.Fatal("stale presence kept reminder visible")
	}
}
