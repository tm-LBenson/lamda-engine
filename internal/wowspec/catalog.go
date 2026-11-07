// Package wowspec holds the Retail specialization identity shared by build
// references and encounter providers. URL segments are fixed data, never input.
package wowspec

type Spec struct {
	ID                                      int
	Class, ClassName, Name, ClassSlug, Slug string
	Healer                                  bool
}

var All = []Spec{
	{250, "DEATHKNIGHT", "Death Knight", "Blood", "death-knight", "blood", false},
	{251, "DEATHKNIGHT", "Death Knight", "Frost", "death-knight", "frost", false},
	{252, "DEATHKNIGHT", "Death Knight", "Unholy", "death-knight", "unholy", false},
	{577, "DEMONHUNTER", "Demon Hunter", "Havoc", "demon-hunter", "havoc", false},
	{581, "DEMONHUNTER", "Demon Hunter", "Vengeance", "demon-hunter", "vengeance", false},
	{1480, "DEMONHUNTER", "Demon Hunter", "Devourer", "demon-hunter", "devourer", false},
	{102, "DRUID", "Druid", "Balance", "druid", "balance", false},
	{103, "DRUID", "Druid", "Feral", "druid", "feral", false},
	{104, "DRUID", "Druid", "Guardian", "druid", "guardian", false},
	{105, "DRUID", "Druid", "Restoration", "druid", "restoration", true},
	{1467, "EVOKER", "Evoker", "Devastation", "evoker", "devastation", false},
	{1468, "EVOKER", "Evoker", "Preservation", "evoker", "preservation", true},
	{1473, "EVOKER", "Evoker", "Augmentation", "evoker", "augmentation", false},
	{253, "HUNTER", "Hunter", "Beast Mastery", "hunter", "beast-mastery", false},
	{254, "HUNTER", "Hunter", "Marksmanship", "hunter", "marksmanship", false},
	{255, "HUNTER", "Hunter", "Survival", "hunter", "survival", false},
	{62, "MAGE", "Mage", "Arcane", "mage", "arcane", false},
	{63, "MAGE", "Mage", "Fire", "mage", "fire", false},
	{64, "MAGE", "Mage", "Frost", "mage", "frost", false},
	{268, "MONK", "Monk", "Brewmaster", "monk", "brewmaster", false},
	{270, "MONK", "Monk", "Mistweaver", "monk", "mistweaver", true},
	{269, "MONK", "Monk", "Windwalker", "monk", "windwalker", false},
	{65, "PALADIN", "Paladin", "Holy", "paladin", "holy", true},
	{66, "PALADIN", "Paladin", "Protection", "paladin", "protection", false},
	{70, "PALADIN", "Paladin", "Retribution", "paladin", "retribution", false},
	{256, "PRIEST", "Priest", "Discipline", "priest", "discipline", true},
	{257, "PRIEST", "Priest", "Holy", "priest", "holy", true},
	{258, "PRIEST", "Priest", "Shadow", "priest", "shadow", false},
	{259, "ROGUE", "Rogue", "Assassination", "rogue", "assassination", false},
	{260, "ROGUE", "Rogue", "Outlaw", "rogue", "outlaw", false},
	{261, "ROGUE", "Rogue", "Subtlety", "rogue", "subtlety", false},
	{262, "SHAMAN", "Shaman", "Elemental", "shaman", "elemental", false},
	{263, "SHAMAN", "Shaman", "Enhancement", "shaman", "enhancement", false},
	{264, "SHAMAN", "Shaman", "Restoration", "shaman", "restoration", true},
	{265, "WARLOCK", "Warlock", "Affliction", "warlock", "affliction", false},
	{266, "WARLOCK", "Warlock", "Demonology", "warlock", "demonology", false},
	{267, "WARLOCK", "Warlock", "Destruction", "warlock", "destruction", false},
	{71, "WARRIOR", "Warrior", "Arms", "warrior", "arms", false},
	{72, "WARRIOR", "Warrior", "Fury", "warrior", "fury", false},
	{73, "WARRIOR", "Warrior", "Protection", "warrior", "protection", false},
}

func Lookup(id int) (Spec, bool) {
	for _, s := range All {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}
func (s Spec) Path() string  { return "/" + s.ClassSlug + "/" + s.Slug }
func (s Spec) Label() string { return s.Name + " " + s.ClassName }
