package overlays

import (
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestReadyCueSurvivesBriefUnknownWithoutSoundOrAnimationRestart(t *testing.T) {
	r := cooldownRule()
	r.Roots[0].Children[0].Props["id"] = "19574"
	r.Roots[0].Children[0].Children[0].Props["display"] = "arc"
	r.Style.Arc = &ArcStyle{Pair: "sides", Start: -130, Sweep: 80, Thickness: 10, Gap: 6, Fill: "full", Resource: "primary", Repeat: "single", Layout: "nested", Kind: "buff", Unit: "player", Limit: 1}
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	if err := s.Save(0, rs); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	seq := 0
	send := func(ms int, change func(*lookup.OverlayInput)) engine.FrameDisplay {
		seq++
		at := now.Add(time.Duration(ms) * time.Millisecond)
		p := sample(rs, at, seq)
		c := cdValue()
		c.ID = 19574
		c.MaxCharges = ptr(0.0)
		c.Charges = ptr(0.0)
		p.Overlays.CooldownsVersion = 1
		p.Overlays.Cooldowns = []lookup.OverlayCooldown{c}
		p.Overlays.CooldownHash = CooldownHash([]CooldownWatch{{"spell", 19574}})
		if change != nil {
			change(p.Overlays)
		}
		s.Accept(p, at)
		return s.Merge(engine.FrameDisplay{}, at)
	}
	first := send(0, nil)
	if len(first.Overlays) != 1 || len(first.OverlaySounds) != 1 {
		t.Fatal("fixture", first)
	}
	instance := first.Overlays[0].Instance
	unknown := func(p *lookup.OverlayInput) { p.Cooldowns[0].Remaining = nil }
	for _, ms := range []int{200, 400, 600} {
		d := send(ms, unknown)
		if len(d.Overlays) != 1 || d.Overlays[0].Instance != instance {
			t.Fatal("transient read blinked/restarted cue", ms, d.Overlays)
		}
	}
	d := send(800, nil)
	if len(d.Overlays) != 1 || d.Overlays[0].Instance != instance || len(d.OverlaySounds) != 1 {
		t.Fatal("recovery replayed ready cue", d)
	}
	// Unknown readings cannot keep the last confirmed visual alive indefinitely.
	for _, ms := range []int{1000, 1200, 1400, 1600, 1800, 2100} {
		d = send(ms, unknown)
	}
	if len(d.Overlays) != 0 || len(d.OverlaySounds) != 0 {
		t.Fatal("unknown renewed display/audio", d)
	}
	d = send(2300, nil)
	if len(d.Overlays) != 1 || len(d.OverlaySounds) != 0 {
		t.Fatal("unknown recovery dinged again", d)
	}
	d = send(2500, func(p *lookup.OverlayInput) { p.Cooldowns[0].Remaining = ptr(30.0) })
	if len(d.Overlays) != 0 {
		t.Fatal("confirmed cooldown was held")
	}
	d = send(2700, nil)
	if len(d.Overlays) != 1 || len(d.OverlaySounds) != 1 {
		t.Fatal("real ready edge did not rearm", d)
	}
	// Capture expiry removes output, but is not evidence of another ready edge.
	if d = s.Merge(engine.FrameDisplay{}, now.Add(4*time.Second)); len(d.Overlays) != 0 {
		t.Fatal("frozen capture retained cue")
	}
	d = send(4200, nil)
	if len(d.Overlays) != 1 || len(d.OverlaySounds) != 0 {
		t.Fatal("capture recovery replayed sound", d)
	}
	// A new game epoch is a new context and may announce readiness again.
	d = send(4400, func(p *lookup.OverlayInput) { p.Epoch = "next" })
	if len(d.Overlays) != 1 || len(d.OverlaySounds) != 1 {
		t.Fatal("new session retained old latch", d)
	}
}

func TestUnknownCannotStartReadyCueOrPlayCancelledDelayedSound(t *testing.T) {
	r := cooldownRule()
	children := r.Roots[0].Children[0].Children
	r.Roots[0].Children[0].Children = []Node{children[0], {ID: "delay", Type: "wait", Props: map[string]any{"seconds": 0.4}}, children[1]}
	rs := []Rule{r}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	seq := 0
	send := func(ms int, known bool) {
		seq++
		at := now.Add(time.Duration(ms) * time.Millisecond)
		s := sample(rs, at, seq)
		s.Overlays.CooldownsVersion = 1
		s.Overlays.CooldownHash = CooldownHash([]CooldownWatch{{"spell", 363916}})
		c := cdValue()
		if !known {
			c.Remaining = nil
		}
		s.Overlays.Cooldowns = []lookup.OverlayCooldown{c}
		rt.Accept(rs, s, at)
	}
	send(0, false)
	if len(rt.displays) != 0 || len(rt.queue) != 0 || len(rt.sounds) != 0 {
		t.Fatal("unknown started cue")
	}
	send(200, true)
	send(400, false) // Cancel the not-yet-played sound, without losing the ready visual.
	send(600, true)
	if len(rt.sounds) != 0 {
		t.Fatal("cancelled sound fired early")
	}
	send(1000, true)
	if len(rt.sounds) != 1 {
		t.Fatal("cancelled sound was incorrectly latched forever")
	}
}
