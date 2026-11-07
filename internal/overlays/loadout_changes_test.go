package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestLoadoutEventsSeedDeduplicateReconnectAndSpecChanges(t *testing.T) {
	r := rule()
	r.Roots = []Node{{ID: "root", Type: "when", Props: map[string]any{"event": "talents"}, Children: []Node{{ID: "sound", Type: "sound", Props: map[string]any{"sound": "chime"}}}}}
	rs := []Rule{r}
	if e := Validate(rs); e != nil {
		t.Fatal(e)
	}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	next := func(packet, seq, age, spec int, at time.Time) {
		s := sample(rs, at, packet)
		s.Overlays.Watches = nil
		s.Overlays.Spec = spec
		s.Overlays.LoadoutChangesVersion = 1
		s.Overlays.LoadoutChanges = []lookup.OverlayLoadoutChange{{Kind: "talents", Seq: seq}}
		if age >= 0 {
			s.Overlays.LoadoutChanges[0].Age = ptr(age)
		}
		rt.Accept(rs, s, at)
	}
	next(1, 5, 0, 1468, now)
	if len(rt.sounds) != 0 {
		t.Fatal("initial event replayed")
	}
	next(2, 6, 0, 1468, now.Add(200*time.Millisecond))
	if len(rt.sounds) != 1 {
		t.Fatal("change missing")
	}
	next(3, 6, 200, 1468, now.Add(400*time.Millisecond))
	if len(rt.sounds) != 1 {
		t.Fatal("duplicate event")
	}
	next(4, 7, -1, 1468, now.Add(600*time.Millisecond))
	if len(rt.sounds) != 1 {
		t.Fatal("stale event replayed")
	}
	next(5, 8, 0, 1467, now.Add(800*time.Millisecond))
	if len(rt.sounds) != 1 {
		t.Fatal("spec-reset change missing")
	}
	next(6, 9, 0, 1467, now.Add(10*time.Second))
	if len(rt.sounds) != 0 {
		t.Fatal("reconnect replayed")
	}
	next(7, 0, -1, 1467, now.Add(10200*time.Millisecond))
	next(8, 1, 0, 1467, now.Add(10400*time.Millisecond))
	if len(rt.sounds) != 1 {
		t.Fatal("native reseed did not rearm")
	}
}
func TestLoadoutEventScopesRearmAndGearDemand(t *testing.T) {
	r := rule()
	r.Roots = []Node{{ID: "root", Type: "when", Props: map[string]any{"event": "equipment"}, Children: []Node{{ID: "child", Type: "when", Props: map[string]any{"event": "talents"}, Children: []Node{{ID: "sound", Type: "sound", Props: map[string]any{"sound": "chime"}}}}}}}
	rs := []Rule{r}
	if !needsEquipment(rs) || !needsLoadoutChange(rs, "talents") {
		t.Fatal("WHEN demand missing")
	}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	next := func(seq, gear, build int) {
		s := sample(rs, now, seq)
		s.Overlays.Watches = nil
		s.Overlays.EquipmentVersion = 1
		s.Overlays.Equipment = gearInput().Equipment
		s.Overlays.LoadoutChangesVersion = 1
		s.Overlays.LoadoutChanges = []lookup.OverlayLoadoutChange{{Kind: "equipment", Seq: gear}, {Kind: "talents", Seq: build}}
		for i := range s.Overlays.LoadoutChanges {
			if s.Overlays.LoadoutChanges[i].Seq > 0 {
				s.Overlays.LoadoutChanges[i].Age = ptr(0)
			}
		}
		rt.Accept(rs, s, now.Add(time.Duration(seq)*100*time.Millisecond))
	}
	next(1, 0, 0)
	next(2, 1, 0)
	next(3, 1, 1)
	if len(rt.sounds) != 1 {
		t.Fatal("nested build event missing")
	}
	next(4, 2, 1)
	next(5, 2, 2)
	if len(rt.sounds) != 2 {
		t.Fatal("scope did not rearm")
	}
}
