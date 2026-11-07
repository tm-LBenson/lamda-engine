package lookup

// Public, explicitly watched personal build facts. Nil rank is unknown; zero is
// a verified unselected talent/unknown spell, never an API failure fallback.
type OverlayTalent struct {
	Kind  string `json:"kind"`
	ID    int    `json:"id"`
	Rank  *int   `json:"rank,omitempty"`
	Spell int    `json:"spell,omitempty"`
}

func ValidTalentWatch(kind string, id int) bool {
	if id <= 0 || id >= 10000000 {
		return false
	}
	switch kind {
	case "spell", "entry", "pvp", "hero", "known":
		return true
	}
	return false
}
func sanitizeOverlayTalents(p *OverlayInput) {
	if p.TalentsVersion != 1 || len(p.Talents) > 16 {
		p.Talents = nil
		return
	}
	seen := map[string]map[int]bool{}
	for _, t := range p.Talents {
		if !ValidTalentWatch(t.Kind, t.ID) || t.Rank != nil && (*t.Rank < 0 || *t.Rank > 100) || (t.Kind == "hero" || t.Kind == "pvp" || t.Kind == "known") && t.Rank != nil && *t.Rank > 1 || t.Spell < 0 || t.Spell >= 10000000 {
			p.Talents = nil
			return
		}
		if seen[t.Kind] == nil {
			seen[t.Kind] = map[int]bool{}
		}
		if seen[t.Kind][t.ID] {
			p.Talents = nil
			return
		}
		seen[t.Kind][t.ID] = true
	}
}
