package lookup

import (
	"strings"
	"testing"
)

func TestPetMetadataSanitization(t *testing.T) {
	for _, exists := range []*bool{nil, ptrPet(false), ptrPet(true)} {
		p := OverlayInput{PetVersion: 1, Pet: &OverlayPet{Exists: exists, Alive: ptrPet(true), Name: "Pet", Family: "Wolf", Specialization: "Ferocity"}}
		sanitizeOverlayPet(&p)
		if p.Pet == nil || (exists != nil && *exists && p.Pet.Name != "Pet") {
			t.Fatal(p.Pet)
		}
		if (exists == nil || !*exists) && (p.Pet.Alive != nil || p.Pet.Name != "" || p.Pet.Family != "" || p.Pet.Specialization != "") {
			t.Fatal("stale pet identity retained")
		}
	}
	for _, label := range []string{strings.Repeat("x", 161), "bad\npet", string([]byte{255})} {
		p := OverlayInput{PetVersion: 1, Pet: &OverlayPet{Exists: ptrPet(true), Family: label}}
		sanitizeOverlayPet(&p)
		if p.Pet != nil {
			t.Fatal("invalid label retained")
		}
	}
	p := OverlayInput{PetVersion: 2, Pet: &OverlayPet{Exists: ptrPet(false)}}
	sanitizeOverlayPet(&p)
	if p.Pet != nil {
		t.Fatal("unknown version accepted")
	}
}
func ptrPet(v bool) *bool { return &v }

func TestRevivableHunterPetKeepsDeathButNotMissingUnitLabels(t *testing.T) {
	p := OverlayInput{PetVersion: 1, Class: "HUNTER", Pet: &OverlayPet{Exists: ptrPet(false), Revivable: ptrPet(true), Name: "old corpse", Family: "Wolf"}}
	sanitizeOverlayPet(&p)
	if p.Pet == nil || p.Pet.Revivable == nil || !*p.Pet.Revivable || p.Pet.Name != "" || p.Pet.Family != "" {
		t.Fatal(p.Pet)
	}
	for _, class := range []string{"WARLOCK", "DEATHKNIGHT", ""} {
		p.Class = class
		p.Pet = &OverlayPet{Exists: ptrPet(false), Revivable: ptrPet(true)}
		sanitizeOverlayPet(&p)
		if p.Pet.Revivable != nil {
			t.Fatal("nonhunter retained revival history", class)
		}
	}
}
