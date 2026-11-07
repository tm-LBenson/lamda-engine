package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func TestBuildLoadUnknownsDemandAndReload(t *testing.T) {
	r := rule()
	r.Load.Conditions = map[string]any{"condition": "group", "mode": "all", "tests": []any{map[string]any{"condition": "talent", "source": "entry", "id": "123", "check": "selected"}, map[string]any{"condition": "equipment", "source": "saved", "name": "Healing É", "check": "equipped"}}}
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	if len(TalentWatches(rs)) != 1 || len(EquipmentSets(rs)) != 1 || !needsEquipment(rs) {
		t.Fatal("load demand lost")
	}
	p := gearInput()
	p.TalentsVersion = 1
	p.Talents = []lookup.OverlayTalent{{Kind: "entry", ID: 123, Rank: ptr(1)}}
	p.TalentHash = TalentHash(TalentWatches(rs))
	if !loaded(r.Load, p) {
		t.Fatal("valid build did not load")
	}
	p.Talents[0].Rank = nil
	if loaded(r.Load, p) {
		t.Fatal("unknown build loaded")
	}
	p.Talents[0].Rank = ptr(0)
	if loaded(r.Load, p) {
		t.Fatal("unselected build loaded")
	}
	now := time.Unix(2000000000, 0)
	s := sample(rs, now, 1)
	s.Overlays.Native = true
	s.Overlays.TalentsVersion = 1
	s.Overlays.TalentHash = TalentHash([]TalentWatch{})
	rt := NewRuntime()
	rt.Accept(rs, s, now)
	if rt.problem != "Reload WoW to watch new talents" {
		t.Fatal("unknown Load hid missing-watch warning", rt.problem)
	}
	r.Load.Conditions = map[string]any{"condition": "health", "value": 35.0}
	if Validate([]Rule{r}) == nil {
		t.Fatal("unsupported Load predicate accepted")
	}
}
func TestBuildLoadCancelsTimedAndDelayedActions(t *testing.T) {
	r := rule()
	r.Load.Conditions = map[string]any{"condition": "equipment", "source": "item", "id": "123", "check": "equipped"}
	r.Roots = []Node{{ID: "root", Type: "when", Props: map[string]any{"event": "state"}, Children: []Node{{ID: "show", Type: "show", Props: map[string]any{"display": "text", "text": "Equipped", "duration": float64(30)}}, {ID: "wait", Type: "wait", Props: map[string]any{"seconds": float64(2)}}, {ID: "sound", Type: "sound", Props: map[string]any{"sound": "chime"}}}}}
	rs := []Rule{r}
	rt := NewRuntime()
	store := &Store{runtime: rt, doc: Document{Rules: rs}}
	now := time.Unix(2000000000, 0)
	makeSample := func(seq int, known bool) lookup.BridgeSample {
		s := sample(rs, now, seq)
		s.Overlays.Watches = nil
		s.Overlays.EquipmentVersion = 1
		s.Overlays.Equipment = gearInput().Equipment
		if !known {
			s.Overlays.Equipment.Slots[0].Item = nil
		}
		return s
	}
	rt.Accept(rs, makeSample(1, true), now)
	if len(store.Merge(engine.FrameDisplay{}, now).Overlays) != 1 || len(rt.queue) != 1 {
		t.Fatal("rule never started")
	}
	rt.Accept(rs, makeSample(2, false), now.Add(time.Second))
	if len(rt.displays) != 0 || len(rt.queue) != 0 {
		t.Fatal("unknown Load failed to cancel")
	}
	rt.advance(now.Add(3 * time.Second))
	if len(rt.sounds) != 0 {
		t.Fatal("unloaded WAIT still played")
	}
	rt.Accept(rs, makeSample(3, true), now.Add(2*time.Second))
	if len(rt.displays) != 1 {
		t.Fatal("recovered gear did not re-arm rule")
	}
}
