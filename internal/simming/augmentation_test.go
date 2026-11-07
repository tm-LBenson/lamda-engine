package simming

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func augmentationSnapshot() *Snapshot {
	s := fixture(time.Now())
	s.Character.Class, s.Character.Spec, s.Character.Role = "EVOKER", 1473, "DAMAGER"
	s.Profile = strings.NewReplacer("mage=", "evoker=", "spec=frost", "spec=augmentation", "role=attack", "role=spell").Replace(s.Profile)
	return s
}

func TestAugmentationCapabilityKeepsClientAndCharacterScope(t *testing.T) {
	p := testProvider(t)
	if c := p.Capability(augmentationSnapshot(), time.Now()); !c.Available || c.Validation != "unreviewed" {
		t.Fatalf("registered Augmentation worker unavailable or silently reviewed: %+v", c)
	}
	for name, change := range map[string]func(*Snapshot){
		"Forever":      func(s *Snapshot) { s.Client = "forever" },
		"healing role": func(s *Snapshot) { s.Character.Role = "HEALER" },
		"tank role":    func(s *Snapshot) { s.Character.Role = "TANK" },
		"level below":  func(s *Snapshot) { s.Character.Level = 89 },
		"level above":  func(s *Snapshot) { s.Character.Level = 91 },
		"wrong spec":   func(s *Snapshot) { s.Profile = strings.Replace(s.Profile, "spec=augmentation", "spec=devastation", 1) },
		"wrong class":  func(s *Snapshot) { s.Profile = strings.Replace(s.Profile, "evoker=", "mage=", 1) },
		"wrong matching class": func(s *Snapshot) {
			s.Character.Class = "MAGE"
			s.Profile = strings.Replace(s.Profile, "evoker=", "mage=", 1)
		},
		"profile level":  func(s *Snapshot) { s.Profile = strings.Replace(s.Profile, "level=90", "level=80", 1) },
		"profile healer": func(s *Snapshot) { s.Profile = strings.Replace(s.Profile, "role=spell", "role=heal", 1) },
	} {
		t.Run(name, func(t *testing.T) {
			s := augmentationSnapshot()
			change(s)
			if c := p.Capability(s, time.Now()); c.Available {
				t.Fatalf("accepted incompatible character: %+v", c)
			}
		})
	}
}

func TestAugmentationScenarioValidation(t *testing.T) {
	for name, change := range map[string]func(*Scenario){
		"mode":           func(s *Scenario) { s.AugmentationMode = "party\noutput=bad" },
		"negative level": func(s *Scenario) { s.AllyItemLevel = -1 },
		"huge level":     func(s *Scenario) { s.AllyItemLevel = 1001 },
		"negative skill": func(s *Scenario) { s.AllySkill = -.1 },
		"too low skill":  func(s *Scenario) { s.AllySkill = .001 },
		"high skill":     func(s *Scenario) { s.AllySkill = 1.01 },
		"NaN skill":      func(s *Scenario) { s.AllySkill = math.NaN() },
		"infinite skill": func(s *Scenario) { s.AllySkill = math.Inf(1) },
	} {
		t.Run(name, func(t *testing.T) {
			s := DefaultScenario()
			change(&s)
			if s.Validate() == nil {
				t.Fatal("accepted unsupported ally assumption")
			}
		})
	}
	for _, skill := range []float64{0, .01, .9, 1} {
		s := DefaultScenario()
		s.AllySkill = skill
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAugmentationAcceptsExporterEmptyRole(t *testing.T) {
	// The real 12.1.0-03 Augmentation export emits role= with an empty value.
	s := augmentationSnapshot()
	s.Profile = strings.Replace(s.Profile, "role=spell", "role=", 1)
	p, err := ParseProfile(s.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != "spell" || !strings.Contains(p.Base, "role=spell\n") {
		t.Fatalf("empty Aug role not normalized: %+v", p)
	}
	if c := testProvider(t).Capability(s, time.Now()); !c.Available {
		t.Fatalf("real exporter input still unavailable: %+v", c)
	}
	for _, other := range []string{
		strings.Replace(profileFixture, "role=attack", "role=", 1),
		strings.Replace(s.Profile, "race=human", "race=", 1),
	} {
		if _, err := ParseProfile(other); err == nil {
			t.Fatal("blank non-Aug role or unrelated field accepted")
		}
	}
}

func TestAugmentationInputsFreezeAlliesThroughComparisonPaths(t *testing.T) {
	p, err := ParseProfile(augmentationSnapshot().Profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "dungeon", "raid"} {
		t.Run(mode, func(t *testing.T) {
			s := DefaultScenario()
			s.AugmentationMode = mode
			if mode == "raid" {
				s.AllyItemLevel, s.AllySkill = 312, .75
			}
			config := augmentationSettings(s)
			wantCount := 4
			wantDungeon, wantLevel, wantSkill := "1", "328", "0.9"
			if mode == "raid" {
				wantCount, wantDungeon, wantLevel, wantSkill = 8, "0", "312", "0.75"
			}
			if len(config.allyNames()) != wantCount {
				t.Fatal("incorrect cohort")
			}
			ordinary, err := p.Input(s, []string{"item1"})
			if err != nil {
				t.Fatal(err)
			}
			loot, err := lootInput(p, s, []Comparison{{ID: "loot1", Items: []Candidate{{Slot: "main_hand", Value: "id=124,bonus_id=2/3"}}}})
			if err != nil {
				t.Fatal(err)
			}
			for _, input := range []string{ordinary, loot} {
				for _, line := range []string{"profileset_metric=raid_dps", "evoker.make_simplified_if_alone=1", "evoker.patchwerk_in_dungeon=" + wantDungeon, "evoker.simplified_actor_ilevel=" + wantLevel, "evoker.ally_skill=" + wantSkill, "single_actor_batch=0", "profileset_main_actor_index=0", "profileset_report_player_index=0"} {
					if strings.Count(input, line+"\n") != 1 {
						t.Fatalf("missing/duplicate fixed option %q in %s", line, input)
					}
				}
				if strings.Contains(input, "profileset_metric=dps\n") {
					t.Fatal("personal metric leaked into group comparison")
				}
			}
		})
	}
	ordinary, _ := ParseProfile(profileFixture)
	input, err := ordinary.Input(DefaultScenario(), nil)
	if err != nil || !strings.Contains(input, "profileset_metric=dps\n") || strings.Contains(input, "evoker.") {
		t.Fatalf("ordinary DPS input changed: %s %v", input, err)
	}
}

// Minimal shape taken from the pinned native JSON probe, retaining actor
// identity, aggregate uncertainty, metric markers and gear ownership.
func augmentationReport(mode string) map[string]any {
	config := augmentationSettings(Scenario{AugmentationMode: mode})
	actor := func(name, race, spec string) map[string]any {
		return map[string]any{"name": name, "race": race, "specialization": spec, "level": 90,
			"collected_data": map[string]any{"dps": map[string]any{"mean": 100.0}},
			"gear":           map[string]any{"main_hand": map[string]any{"encoded_item": "weapon,id=123"}}}
	}
	players := []any{actor("Lamda", "dracthyr_horde", "Augmentation Evoker")}
	for _, name := range config.allyNames() {
		a := actor(name, "none", "Unknown")
		a["gear"] = map[string]any{"main_hand": map[string]any{"encoded_item": "ally_weapon,id=202565"}}
		players = append(players, a)
	}
	return map[string]any{"logs": []any{map[string]any{"level": "implementation_not_yet_verified", "message": "Native proc assumption"}}, "sim": map[string]any{
		"options":    map[string]any{"confidence_estimator": 1.96, "profileset_metric": "raid_dps"},
		"statistics": map[string]any{"raid_dps": map[string]any{"mean": float64(len(players)) * 100, "std_dev": 12.0, "mean_std_dev": 2.0}},
		"players":    players,
		"profilesets": map[string]any{"metric": "Raid Damage per Second", "results": []any{map[string]any{"name": "item1", "mean": float64(len(players))*100 + 10, "stddev": 13.0, "mean_error": 4.0,
			"overrides": map[string]any{"gear": map[string]any{"main_hand": map[string]any{"item_id": 124, "item_level": 289}}}}}},
	}}
}

func augmentationJSON(t *testing.T, report map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAugmentationAggregateAndUncertainty(t *testing.T) {
	for _, mode := range []string{"dungeon", "raid"} {
		config := augmentationSettings(Scenario{AugmentationMode: mode})
		r, err := parseWorkerResult(augmentationJSON(t, augmentationReport(mode)), &config)
		if err != nil {
			t.Fatal(err)
		}
		if r.Metric != "group_dps" || r.Baseline.Mean != float64(len(config.allyNames())+1)*100 || r.Baseline.StdDev != 12 || r.Baseline.MeanError != 3.92 || len(r.Comparisons) != 1 || r.Comparisons[0].MeanError != 4 || len(r.Warnings) != 1 {
			t.Fatalf("personal metric, lost uncertainty or missing warning: %+v", r)
		}
		config.describe(r)
		if !strings.Contains(r.ModelInputs["metric"], "Total Group DPS") || !strings.Contains(r.ModelInputs["comparisons"], "percentages use total Group DPS") || !strings.Contains(r.Model, mode) {
			t.Fatalf("missing group assumptions: %+v", r)
		}
	}
}

func TestAugmentationRejectsIncorrectWorkerGroup(t *testing.T) {
	for name, change := range map[string]func(map[string]any){
		"missing ally":           func(s map[string]any) { s["players"] = s["players"].([]any)[:4] },
		"duplicate ally":         func(s map[string]any) { p := s["players"].([]any); p[2] = p[1] },
		"unknown ally":           func(s map[string]any) { s["players"].([]any)[1].(map[string]any)["name"] = "Bob Stranger" },
		"ally race":              func(s map[string]any) { s["players"].([]any)[1].(map[string]any)["race"] = "human" },
		"ally specialization":    func(s map[string]any) { s["players"].([]any)[1].(map[string]any)["specialization"] = "Arcane Mage" },
		"ally level":             func(s map[string]any) { s["players"].([]any)[1].(map[string]any)["level"] = 89 },
		"wrong main actor":       func(s map[string]any) { p := s["players"].([]any); p[0], p[1] = p[1], p[0] },
		"main level":             func(s map[string]any) { s["players"].([]any)[0].(map[string]any)["level"] = 89 },
		"personal option":        func(s map[string]any) { s["options"].(map[string]any)["profileset_metric"] = "dps" },
		"personal profilesets":   func(s map[string]any) { s["profilesets"].(map[string]any)["metric"] = "Damage per Second" },
		"missing aggregate":      func(s map[string]any) { delete(s, "statistics") },
		"personal baseline":      func(s map[string]any) { s["statistics"].(map[string]any)["raid_dps"].(map[string]any)["mean"] = 100 },
		"double counted support": func(s map[string]any) { s["statistics"].(map[string]any)["raid_dps"].(map[string]any)["mean"] = 600 },
		"negative uncertainty":   func(s map[string]any) { s["statistics"].(map[string]any)["raid_dps"].(map[string]any)["std_dev"] = -1 },
		"missing confidence":     func(s map[string]any) { delete(s["options"].(map[string]any), "confidence_estimator") },
		"negative confidence":    func(s map[string]any) { s["options"].(map[string]any)["confidence_estimator"] = -1.96 },
		"overflowing confidence": func(s map[string]any) {
			s["options"].(map[string]any)["confidence_estimator"] = 1e308
			s["statistics"].(map[string]any)["raid_dps"].(map[string]any)["mean_std_dev"] = 1e308
		},
		"invalid comparison": func(s map[string]any) {
			s["profilesets"].(map[string]any)["results"].([]any)[0].(map[string]any)["mean"] = 0
		},
	} {
		t.Run(name, func(t *testing.T) {
			report := augmentationReport("dungeon")
			change(report["sim"].(map[string]any))
			config := augmentationSettings(Scenario{})
			if _, err := parseWorkerResult(augmentationJSON(t, report), &config); err == nil {
				t.Fatal("accepted inconsistent group result")
			}
		})
	}
	report := augmentationReport("dungeon")
	report["logs"] = []any{map[string]any{"level": "error", "message": "bad model"}}
	config := augmentationSettings(Scenario{})
	if _, err := parseWorkerResult(augmentationJSON(t, report), &config); err == nil {
		t.Fatal("worker error suppressed")
	}
}

func TestAugmentationGearVerificationUsesCharacterNotAllies(t *testing.T) {
	s := augmentationSnapshot()
	comparison := []Comparison{{ID: "item1", Items: []Candidate{{Slot: "main_hand", Value: "id=124,ilevel=289"}}}}
	report := augmentationReport("dungeon")
	gear := []byte("main_hand=weapon,id=123\n")
	if err := verifyWorkerGear(augmentationJSON(t, report), gear, s, comparison); err != nil {
		t.Fatal(err)
	}
	players := report["sim"].(map[string]any)["players"].([]any)
	players[0], players[1] = players[1], players[0]
	if err := verifyWorkerGear(augmentationJSON(t, report), gear, s, comparison); err == nil {
		t.Fatal("ally equipment treated as character gear")
	}
	players[0], players[1] = players[1], players[0]
	players[0].(map[string]any)["gear"] = players[1].(map[string]any)["gear"]
	if err := verifyWorkerGear(augmentationJSON(t, report), gear, s, comparison); err == nil {
		t.Fatal("wrong character equipment passed because ally happened to match")
	}
}

func TestAugmentationDoesNotRelaxOrdinaryDPSResult(t *testing.T) {
	data := []byte(`{"sim":{"options":{"confidence_estimator":1.96},"players":[{"collected_data":{"dps":{"mean":123,"std_dev":10,"mean_std_dev":2}}}]}}`)
	r, err := parseWorkerResult(data, nil)
	if err != nil || r.Metric != "DPS" || r.Baseline.Mean != 123 || r.Baseline.MeanError != 3.92 {
		t.Fatalf("ordinary result changed: %+v %v", r, err)
	}
	if _, err := parseWorkerResult(augmentationJSON(t, augmentationReport("dungeon")), nil); err == nil {
		t.Fatal("multi-actor result accepted for ordinary DPS")
	}
}
