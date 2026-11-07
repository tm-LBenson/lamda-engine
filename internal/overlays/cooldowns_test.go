package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cooldownRule() Rule {
	r := rule()
	r.Roots[0].Props["event"] = "cooldown"
	r.Roots[0].Children[0].Props = map[string]any{"condition": "cooldown", "source": "spell", "id": "363916", "check": "ready", "operator": "gte", "value": float64(1)}
	r.Roots[0].Children[0].Children[0].Props["visibility"] = "conditions"
	return r
}
func cdValue() lookup.OverlayCooldown {
	return lookup.OverlayCooldown{Kind: "spell", ID: 363916, Available: true, Remaining: ptr(0.0), Duration: ptr(0.0), Charges: ptr(1.0), MaxCharges: ptr(2.0), Recharge: ptr(10.0), RechargeDuration: ptr(30.0), Enabled: ptr(true), Usable: ptr(true)}
}

func TestCooldownConditions(t *testing.T) {
	for _, tc := range []struct {
		check, op string
		value     float64
		change    func(*lookup.OverlayCooldown)
		want      bool
	}{
		{"ready", "", 0, func(c *lookup.OverlayCooldown) {}, true},
		{"ready", "", 0, func(c *lookup.OverlayCooldown) { c.Charges = ptr(0.0); c.Remaining = ptr(10.0) }, false},
		{"cooldown", "", 0, func(c *lookup.OverlayCooldown) { c.Charges = ptr(0.0); c.Remaining = ptr(10.0) }, true},
		{"remaining", "lt", 5, func(c *lookup.OverlayCooldown) { c.Remaining = ptr(4.9) }, true},
		{"remaining", "eq", 0, func(c *lookup.OverlayCooldown) { c.Remaining = ptr(.1) }, false},
		{"charges", "eq", 1, func(c *lookup.OverlayCooldown) {}, true},
		{"charges", "ne", 1, func(c *lookup.OverlayCooldown) {}, false},
		{"charges", "gte", 2, func(c *lookup.OverlayCooldown) { c.Charges = ptr(2.0) }, true},
		{"full", "", 0, func(c *lookup.OverlayCooldown) {}, false},
		{"full", "", 0, func(c *lookup.OverlayCooldown) { c.Charges = ptr(2.0) }, true},
		{"recharge", "lte", 10, func(c *lookup.OverlayCooldown) {}, true},
		{"usable", "", 0, func(c *lookup.OverlayCooldown) {}, true},
		{"usable", "", 0, func(c *lookup.OverlayCooldown) { c.Usable = ptr(false) }, false},
		{"unusable", "", 0, func(c *lookup.OverlayCooldown) { c.Usable = ptr(false) }, true},
	} {
		t.Run(tc.check+tc.op, func(t *testing.T) {
			r := cooldownRule()
			n := r.Roots[0].Children[0]
			n.Props["check"] = tc.check
			n.Props["operator"] = tc.op
			n.Props["value"] = tc.value
			c := cdValue()
			tc.change(&c)
			if got := condition(n, &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{c}}, "player"); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			c.Available = false
			if condition(n, &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{c}}, "player") {
				t.Fatal("unavailable matched")
			}
			if condition(n, &lookup.OverlayInput{}, "player") {
				t.Fatal("missing matched")
			}
		})
	}
	for _, check := range []string{"ready", "cooldown", "usable", "unusable", "remaining", "charges", "full", "recharge"} {
		n := cooldownRule().Roots[0].Children[0]
		n.Props["check"] = check
		n.Props["operator"] = "ne"
		if condition(n, &lookup.OverlayInput{Cooldowns: []lookup.OverlayCooldown{{Kind: "spell", ID: 363916, Available: true}}}, "player") {
			t.Fatal("unknown matched", check)
		}
	}
}

func TestCooldownWatchSaveIsolationAndEdges(t *testing.T) {
	r := cooldownRule()
	now := time.Unix(2000000000, 0)
	rt := NewRuntime()
	rules := []Rule{r}
	send := func(seq int, c lookup.OverlayCooldown) {
		s := sample(rules, now, seq)
		s.Overlays.Watches = nil
		s.Overlays.CooldownsVersion = 1
		s.Overlays.Cooldowns = []lookup.OverlayCooldown{c}
		s.Overlays.CooldownHash = CooldownHash([]CooldownWatch{{"spell", 363916}})
		rt.Accept(rules, s, now.Add(time.Duration(seq)*100*time.Millisecond))
	}
	send(1, cdValue())
	send(2, cdValue())
	if len(rt.displays) != 1 || len(rt.sounds) != 1 {
		t.Fatal("no ready display or sound repeated")
	}
	other := cooldownRule()
	other.ID = "other"
	other.Load.Class = "MAGE"
	other.Roots[0].Children[0].Props["id"] = "116"
	rules = append(rules, other)
	send(3, cdValue())
	if rt.problem != "" || len(rt.displays) != 1 {
		t.Fatal("other class interrupted loaded watch", rt.problem)
	}
	rules[1].Load.Class = "EVOKER"
	send(4, cdValue())
	if rt.problem != "Reload WoW to watch new cooldowns" || len(rt.displays) != 1 {
		t.Fatal("new unloaded watch interrupted old one", rt.problem)
	}
	c := cdValue()
	c.Remaining = ptr(10.0)
	c.Charges = ptr(0.0)
	send(5, c)
	if len(rt.displays) != 0 {
		t.Fatal("held ready display stayed visible on cooldown")
	}
	send(6, cdValue())
	if len(rt.displays) != 1 || len(rt.sounds) != 2 {
		t.Fatal("ready transition did not rearm")
	}
	c = cdValue()
	c.Remaining = nil
	send(7, c)
	if len(rt.displays) != 1 {
		t.Fatal("brief unreadable ready sample blinked the held display")
	}
	// Corrupt mappings cannot erase the caller's sample or block other inputs.
	s := sample(rules, now, 8)
	s.Overlays.CooldownsVersion = 1
	s.Overlays.Cooldowns = []lookup.OverlayCooldown{cdValue()}
	s.Overlays.CooldownHash = "bad"
	rt.Accept(rules, s, now.Add(time.Second))
	if len(rt.last.Cooldowns) != 0 || len(s.Overlays.Cooldowns) != 1 {
		t.Fatal("bad mapping accepted or sample mutated")
	}
}

func TestCooldownPublicationAndValidation(t *testing.T) {
	r := cooldownRule()
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	for _, check := range []string{"charges", "recharge", "full"} {
		r.Roots[0].Children[0].Props["source"] = "item"
		r.Roots[0].Children[0].Props["check"] = check
		if Validate([]Rule{r}) == nil {
			t.Fatal("item charges accepted")
		}
	}
	r = cooldownRule()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "OverlaysData.lua")
	os.WriteFile(file, []byte(Marker+"\n"), 0600)
	s.SetGameFile(file)
	if err = s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(file)
	if !strings.Contains(string(b), "[\"id\"]=363916") || !strings.Contains(string(b), CooldownHash(CooldownWatches([]Rule{r}))) {
		t.Fatal(string(b))
	}
	loaded, err := Open(dir)
	if err != nil || len(CooldownWatches(loaded.doc.Rules)) != 1 {
		t.Fatal("saved cooldown lost", err)
	}
}
