package healsim

// Sources are deliberately curated, versioned data. Guide prose is never
// downloaded or interpreted as executable rotations. QE and Wowhead share an
// author, so they constitute one independent opinion in the consensus.
type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	AuthorGroup string `json:"authorGroup"`
	Updated     string `json:"updated"`
	Reviewed    string `json:"reviewed"`
	Patch       string `json:"patch"`
	Use         string `json:"use"`
}

var Sources = []Source{
	{"qe", "Questionably Epic", "https://www.questionablyepic.com/live/", "voulk", "2026-09-02", "2026-09-29", "12.1", "Gear model reference; fixed profiles are not encounter HPS."},
	{"wowhead", "Wowhead / Voulk", "https://www.wowhead.com/guide/classes/evoker/preservation/rotation-cooldowns-pve-healer", "voulk", "2026-09-21", "2026-09-29", "12.1", "Encounter-specific priorities and Echo / Merithra interactions."},
	{"method", "Method / Cryve", "https://www.method.gg/guides/preservation-evoker/playstyle-and-rotation", "cryve", "2026-08-20", "2026-09-29", "12.1", "Raid preparation and dungeon spot-healing priorities."},
	{"icyveins", "Icy Veins / Mytholxgy", "https://www.icy-veins.com/wow/preservation-evoker-pve-healing-rotation-cooldowns-abilities", "mytholxgy", "2026-08-13", "2026-09-29", "12.1", "Maintenance healing and cooldown sequencing; contextual disagreements retained."},
	{"spiritbloom", "SpiritbloomPro / Harrek, YouTee, Kig", "https://spiritbloom.pro/preservation", "spiritbloom", "", "2026-09-29", "12.1", "Setup and burst philosophy. Interaction page is still being updated for Season 2; not treated as fully current numerical data."},
	{"murlok", "Murlok.io", "https://murlok.io/evoker/preservation/m+", "leaderboard", "", "2026-09-29", "12.1", "Observed top-player choices, kept separate from guide votes and throughput predictions."},
}

type Recommendation struct {
	ID       string   `json:"id"`
	Context  string   `json:"context"`
	Text     string   `json:"text"`
	Sources  []string `json:"sources"`
	Support  float64  `json:"support"`
	Opinions int      `json:"opinions"`
}

// A repeated author gets one vote. For contradictory publications from the
// same author, use the newest dated recommendation, not whichever is iterated
// last. Leaderboard popularity isn't a strategy vote.
func averageOpinions(votes map[string]float64) (float64, int) {
	return averageSourceOpinions(Sources, votes)
}
func averageSourceOpinions(sources []Source, votes map[string]float64) (float64, int) {
	type opinion struct {
		value   float64
		updated string
	}
	groups := map[string]opinion{}
	for _, source := range sources {
		value, ok := votes[source.ID]
		if !ok || source.AuthorGroup == "leaderboard" || !finiteRange(value, 0, 1) {
			continue
		}
		old, exists := groups[source.AuthorGroup]
		if !exists || source.Updated > old.updated {
			groups[source.AuthorGroup] = opinion{value, source.Updated}
		}
	}
	if len(groups) == 0 {
		return 0, 0
	}
	sum := 0.
	for _, v := range groups {
		sum += v.value
	}
	return sum / float64(len(groups)), len(groups)
}

// Support is an equal-weight average of independent applicable opinions, not a
// confidence percentage or an average of incompatible talent export strings.
func Consensus() []Recommendation {
	rows := []Recommendation{
		{ID: "prepare", Context: "raid,dungeon,tank", Text: "Prepare Echo before known damage; spend the prepared healing when damage arrives.", Sources: []string{"wowhead", "method", "icyveins", "spiritbloom"}},
		{ID: "reversion", Context: "raid,dungeon,tank", Text: "Maintain useful Reversion coverage; reserve an immediate heal for lethal damage.", Sources: []string{"wowhead", "method", "icyveins"}},
		{ID: "breath", Context: "raid,dungeon", Text: "Use low-rank Dream Breath for sustained healing; adjust the rank for urgent damage.", Sources: []string{"wowhead", "method", "icyveins"}},
		{ID: "echo_target", Context: "dungeon", Text: "Echo consumption depends on the damage pattern: Dream Breath, Merithra or Verdant Embrace can each be appropriate.", Sources: []string{"wowhead", "method", "icyveins"}},
	}
	for i := range rows {
		votes := map[string]float64{}
		for _, id := range rows[i].Sources {
			votes[id] = 1
		}
		rows[i].Support, rows[i].Opinions = averageOpinions(votes)
	}
	return rows
}
