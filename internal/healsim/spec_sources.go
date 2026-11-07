package healsim

import "github.com/tm-LBenson/lamda-engine/internal/wowspec"

// Guide catalogs are specialization-scoped. DPS/tanks get their leaderboard
// source, not Preservation healing advice. Reviewed qualitative rules are
// independent of the incomplete numerical spell model.
func SourcesFor(id int) []Source {
	if id == 1468 {
		return append([]Source{}, Sources...)
	}
	spec, ok := wowspec.Lookup(id)
	if !ok {
		return nil
	}
	out := []Source{{ID: "murlok", Name: "Murlok.io", URL: "https://murlok.io" + spec.Path() + "/m+", AuthorGroup: "leaderboard", Reviewed: "2026-09-29", Patch: "12.1", Use: "Ranked-player choices; popularity is not simulated throughput."}}
	if !spec.Healer {
		return out
	}
	data := map[int][5]string{
		105: {"BirraGzeeZ", "2026-09-13", "Voulk", "voulk", "2026-08-10"},
		256: {"Grafe", "2026-09-17", "Clandon", "clandon", "2026-08-17"},
		257: {"Grafe", "2026-09-17", "Niphyr", "niphyr", "2026-08-25"},
		65:  {"Joki", "2026-08-27", "Mytholxgy", "mytholxgy", "2026-08-10"},
		264: {"Radio", "2026-08-11", "Seksixeny", "seksixeny", "2026-08-10"},
		270: {"Tincell", "2026-08-27", "Dhaubbs", "dhaubbs", "2026-08-12"},
	}[id]
	slug := spec.Slug + "-" + spec.ClassSlug
	out = append(out, Source{ID: "method", Name: "Method / " + data[0], URL: "https://www.method.gg/guides/" + slug + "/playstyle-and-rotation", AuthorGroup: data[0], Updated: data[1], Reviewed: "2026-09-29", Patch: "12.1", Use: "Spec-specific raid and dungeon priorities; optional talent sequences remain conditional."})
	out = append(out, Source{ID: "icyveins", Name: "Icy Veins / " + data[2], URL: "https://www.icy-veins.com/wow/" + slug + "-pve-healing-rotation-cooldowns-abilities", AuthorGroup: data[3], Updated: data[4], Reviewed: "2026-09-29", Patch: "12.1", Use: "Independent rotation reference; recommendations do not imply full simulation coverage."})
	return out
}
func ConsensusFor(id int) []Recommendation {
	if id == 1468 {
		return Consensus()
	}
	advice := map[int]string{
		105: "Maintain useful Lifebloom coverage and prepare HoTs before predictable damage; use direct healing for urgent targets.",
		256: "Apply Atonement before predictable damage, then use damaging spells during the healing window; direct-heal urgent targets.",
		257: "Use Holy Words for the matching damage pattern; Flash Heal and Prayer of Healing rebuild their availability. Maintain Prayer of Mending.",
		65:  "Generate and spend Holy Power without capping; choose single-target or group spenders according to injuries and the selected talents.",
		264: "Maintain Riptide and totems; choose Healing Wave for single targets or Chain Heal when several allies are injured.",
		270: "Maintain Renewing Mist coverage and avoid wasted charges; use Vivify for injured allies and plan major cooldowns around damage.",
	}[id]
	if advice == "" {
		return nil
	}
	support, count := averageSourceOpinions(SourcesFor(id), map[string]float64{"method": 1, "icyveins": 1})
	return []Recommendation{{ID: "core_priority", Context: "raid,dungeon,tank", Text: advice, Sources: []string{"method", "icyveins"}, Support: support, Opinions: count}}
}
