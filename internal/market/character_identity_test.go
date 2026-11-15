package market

import "testing"

func TestCharactersWithSameGivenNameRemainSeparateMakers(t *testing.T) {
	m := model{settings: DefaultSettings(), now: fixtureNow.Unix(), quotes: map[string]quote{}, vendors: []Vendor{{ID: "v", Item: "raw", Buy: 10, Bundle: 1, Stock: -1}}, items: map[string]Item{}, chars: []Character{
		{ID: "mage", Name: "Example", Surname: "Mage", Professions: []Profession{{ID: "p", Rank: 100}}, Known: []string{"final"}},
		{ID: "warlock", Name: "Example", Surname: "Warlock", Professions: []Profession{{ID: "p", Rank: 100}}, Known: []string{"bound"}},
	}, recipes: []Recipe{
		{ID: "bound", Name: "Bound reagent", Profession: "p", Inputs: []Reagent{{Item: "raw", Quantity: 1}}, Output: Reagent{Item: "bound", Quantity: 1}},
		{ID: "final", Name: "Final item", Profession: "p", Inputs: []Reagent{{Item: "bound", Quantity: 1}}, Output: Reagent{Item: "final", Quantity: 1}},
	}}
	if _, ok := m.acquire("final", 1, map[string]bool{}, 0); ok {
		t.Fatal("same given name allowed a bound intermediate across characters")
	}
	m.items["bound"] = Item{ID: "bound", Transferable: true}
	p, ok := m.acquire("final", 1, map[string]bool{}, 0)
	if !ok || p.Maker != "Example-Mage" || p.MakerID != "mage" || len(p.ActorIDs) != 2 || len(p.Actors) != 2 {
		t.Fatal("full-name plan lost distinct makers", p, ok)
	}
	m.items["bound"] = Item{ID: "bound"}
	m.chars[1].Known = append(m.chars[1].Known, "final")
	p, ok = m.acquire("final", 1, map[string]bool{}, 0)
	if !ok || p.Maker != "Example-Warlock" || len(p.ActorIDs) != 1 {
		t.Fatal("capable same-character path not selected", p, ok)
	}
}
