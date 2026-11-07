package overlays

import (
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestPetConditionsUnknownPresenceLifeAndTypes(t *testing.T) {
	states := []struct {
		pet  *lookup.OverlayPet
		want []truth
	}{
		{nil, []truth{unknown, unknown, unknown, unknown, unknown}},
		{&lookup.OverlayPet{}, []truth{unknown, unknown, unknown, unknown, unknown}},
		{&lookup.OverlayPet{Exists: ptr(false)}, []truth{no, yes, no, no, yes}},
		{&lookup.OverlayPet{Exists: ptr(true)}, []truth{yes, no, unknown, unknown, unknown}},
		{&lookup.OverlayPet{Exists: ptr(true), Alive: ptr(true)}, []truth{yes, no, yes, no, no}},
		{&lookup.OverlayPet{Exists: ptr(true), Alive: ptr(false)}, []truth{yes, no, no, yes, yes}},
		{&lookup.OverlayPet{Exists: ptr(false), Revivable: ptr(true)}, []truth{no, no, no, yes, yes}},
	}
	for _, state := range states {
		p := &lookup.OverlayInput{PetVersion: 1, Pet: state.pet}
		for i, check := range []string{"present", "missing", "alive", "dead", "unavailable"} {
			q := map[string]any{"condition": "pet", "check": check}
			count := 0
			if !validPredicate(q, 0, &count) {
				t.Fatal("invalid", check)
			}
			if got := predicate(q, p, ""); got != state.want[i] {
				t.Fatalf("%s %+v got %v want %v", check, state.pet, got, state.want[i])
			}
		}
	}
	p := &lookup.OverlayInput{PetVersion: 1, Pet: &lookup.OverlayPet{Exists: ptr(true), Family: "Spirit Beast", Specialization: "Tenacity", CreatureType: "Beast"}}
	for check, value := range map[string]string{"family": " spirit BEAST ", "specialization": "tenacity", "creatureType": "beast"} {
		q := map[string]any{"condition": "pet", "check": check, "value": value}
		if predicate(q, p, "") != yes {
			t.Fatal(check)
		}
		q["value"] = "wrong"
		if predicate(q, p, "") != no {
			t.Fatal(check)
		}
	}
	p.Pet.Specialization = ""
	q := map[string]any{"condition": "pet", "check": "specialization", "value": "Ferocity"}
	not := map[string]any{"condition": "group", "mode": "not", "tests": []any{q}}
	if predicate(not, p, "") != unknown {
		t.Fatal("NOT unknown matched")
	}
	p.PetVersion = 0
	q["check"] = "missing"
	if predicate(q, p, "") != unknown {
		t.Fatal("old addon inferred absent")
	}
	for _, v := range []any{"", strings.Repeat("x", 161), "Wolf\n", float64(5)} {
		q["check"] = "family"
		q["value"] = v
		if validPetCondition(q) {
			t.Fatal("invalid value accepted", v)
		}
	}
}

func TestHunterReviveWarningDoesNotBecomeSummonWhenCorpseDisappears(t *testing.T) {
	makeRule := func(id, check string) Rule {
		r := rule()
		r.ID = id
		r.Roots[0].Props["event"] = "state"
		r.Roots[0].Children[0].Props = map[string]any{"condition": "pet", "check": check}
		r.Roots[0].Children[0].Children = r.Roots[0].Children[0].Children[:1]
		r.Roots[0].Children[0].Children[0].Props = map[string]any{"display": "text", "text": id, "visibility": "conditions"}
		return r
	}
	rs := []Rule{makeRule("summon", "missing"), makeRule("revive", "dead")}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	for i, state := range []struct {
		pet  *lookup.OverlayPet
		want string
	}{
		{&lookup.OverlayPet{Exists: ptr(true), Alive: ptr(false), Revivable: ptr(true)}, "revive"},
		{&lookup.OverlayPet{Exists: ptr(false), Revivable: ptr(true)}, "revive"},
		{&lookup.OverlayPet{Exists: ptr(true), Alive: ptr(true)}, ""},
		{&lookup.OverlayPet{Exists: ptr(false)}, "summon"},
		{&lookup.OverlayPet{}, ""},
	} {
		p := sample(rs, now, i+1)
		p.Overlays.PetVersion = 1
		p.Overlays.Pet = state.pet
		rt.Accept(rs, p, now.Add(time.Duration(i)*200*time.Millisecond))
		if state.want == "" {
			if len(rt.displays) != 0 {
				t.Fatal(i, rt.displays)
			}
			continue
		}
		if len(rt.displays) != 1 {
			t.Fatal(i, rt.displays)
		}
		for _, d := range rt.displays {
			if d.Text != state.want {
				t.Fatal(i, d.Text, state.want)
			}
		}
	}
}

func TestPetWarningRuntimeTransitionsAndLoadFilters(t *testing.T) {
	r := rule()
	r.Name = "NO PET"
	r.Roots[0].Props = map[string]any{"event": "state"}
	r.Roots[0].Children[0].Props = map[string]any{"condition": "pet", "check": "unavailable"}
	r.Roots[0].Children[0].Children = r.Roots[0].Children[0].Children[:1]
	r.Roots[0].Children[0].Children[0].Props = map[string]any{"display": "text", "text": "NO PET", "visibility": "conditions"}
	r.Load = Load{Character: "Lamdabow", Realm: "Gurubashi", Class: "HUNTER", Spec: 253, Mounted: "no", Alive: "yes"}
	rs := []Rule{r}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	rt := NewRuntime()
	now := time.Unix(2000000000, 0)
	for i, state := range []struct {
		pet                      *lookup.OverlayPet
		mounted, dead, wrongSpec bool
		want                     int
	}{
		{want: 0},
		{pet: &lookup.OverlayPet{Exists: ptr(false)}, want: 1},
		{pet: &lookup.OverlayPet{Exists: ptr(true), Alive: ptr(true)}, want: 0},
		{pet: &lookup.OverlayPet{Exists: ptr(true), Alive: ptr(false)}, want: 1},
		{pet: &lookup.OverlayPet{Exists: ptr(false)}, mounted: true, want: 0},
		{pet: &lookup.OverlayPet{Exists: ptr(false)}, dead: true, want: 0},
		{pet: &lookup.OverlayPet{Exists: ptr(false)}, wrongSpec: true, want: 0},
		{pet: &lookup.OverlayPet{Exists: ptr(false)}, want: 1},
		{pet: &lookup.OverlayPet{}, want: 0},
	} {
		s := sample(rs, now, i+1)
		p := s.Overlays
		p.Character = "Lamdabow"
		p.Class = "HUNTER"
		p.Spec = 253
		p.PetVersion = 1
		p.Pet = state.pet
		p.Mounted = state.mounted
		p.Alive = !state.dead
		if state.wrongSpec {
			p.Spec = 254
		}
		rt.Accept(rs, s, now.Add(time.Duration(i)*200*time.Millisecond))
		if len(rt.displays) != state.want {
			t.Fatalf("step %d displays %d want %d, %s", i, len(rt.displays), state.want, rt.problem)
		}
	}
}
