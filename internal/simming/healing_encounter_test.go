package simming

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/healsim"
)

func encounterSnapshot() *Snapshot {
	return &Snapshot{Schema: 1, ID: "healing_test", Client: "retail", CapturedAt: 1, Build: "69933", Interface: 120100, Character: Character{GUID: "Player-test", Name: "Test", Class: "EVOKER", Spec: 1468, Role: "HEALER", Level: 90}, HealingStats: &healsim.Stats{SpellPower: 20000, Health: 800000, Mana: 250000, ManaRegen: 1250, Crit: .2, Haste: .2, Mastery: .4, Talents: map[int]int{364343: 1, 366155: 1, 355936: 1, 360995: 1}}}
}
func TestEncounterUsesCapturedStatsWithoutQEOrSimC(t *testing.T) {
	p := &encounterProvider{task: "healing"}
	s := encounterSnapshot()
	scenario := DefaultScenario()
	scenario.Task = "healing"
	scenario.Seconds = 30
	scenario.HealingScenario = "dungeon"
	scenario.HealingPolicy = "compare"
	scenario.HealingRuns = 2
	scenario.HealingIntensity = 100
	r, e := p.Run(context.Background(), s, scenario, nil, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if r.Metric != "healing_hps" || r.Healing == nil || r.Baseline.Mean <= 0 || len(r.Comparisons) != 1 || r.Validation != "experimental" {
		t.Fatal("wrong result", r)
	}
	s.Client = "forever"
	if p.Capability(s, time.Now()).Available {
		t.Fatal("Retail model allowed on Forever")
	}
	s.Client = "retail"
	s.Character.Spec = 1473
	if p.Capability(s, time.Now()).Available {
		t.Fatal("wrong spec accepted")
	}
}
func TestGameRequestRoutesNewTasksAndRejectsReplacements(t *testing.T) {
	for _, task := range []string{"healing", "builds"} {
		s := encounterSnapshot()
		r := &GameRequest{ID: s.ID, Schema: 1, Task: task, Scenario: DefaultScenario()}
		s.GameRequest = r
		if e := r.Validate(s.ID, s.Client); e != nil {
			t.Fatal(e)
		}
		ids, e := resolveGameSelection(s)
		if e != nil || len(ids) != 0 {
			t.Fatal("built-in requested SimC profile", e)
		}
		r.Talents = []TalentBuild{{Code: "ABC=="}}
		if r.Validate(s.ID, s.Client) == nil {
			t.Fatal("unmodeled build replacement accepted")
		}
	}
}
func TestHealingSnapshotRoundTripAndBadData(t *testing.T) {
	s := encounterSnapshot()
	data, _ := json.Marshal(s)
	copy, e := DecodeSnapshot(data, "retail")
	if e != nil || copy.HealingStats.SpellPower != s.HealingStats.SpellPower {
		t.Fatal(e)
	}
	s.HealingStats.Mana = 0
	data, _ = json.Marshal(s)
	if _, e = DecodeSnapshot(data, "retail"); e == nil {
		t.Fatal("accepted missing stats")
	}
}

func TestEncounterGameRequestPublishesThroughNormalPipeline(t *testing.T) {
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", &Providers{Damage: &Provider{}, Healing: &HealingProvider{}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := encounterSnapshot()
	s.CapturedAt = time.Now().Unix()
	scenario := DefaultScenario()
	scenario.Seconds = 30
	scenario.HealingScenario = "dungeon"
	scenario.HealingIntensity = 100
	scenario.HealingRuns = 2
	scenario.HealingPolicy = "compare"
	s.GameRequest = &GameRequest{Schema: 1, ID: s.ID, Task: "healing", Scenario: scenario}
	if _, err = w.ReceiveGame(s); err != nil {
		t.Fatal(err)
	}
	job := waitJob(t, w, gameJobID(s))
	if job.State != "complete" || job.Result.Healing == nil {
		t.Fatal("new task did not finish", job.Error)
	}
	data, err := BuildGameResults(w.State(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "healing_hps") || !strings.Contains(string(data), "manaStarvedSeconds") {
		t.Fatal("healing report lost in publication")
	}
}

func TestReplayInputIsArchivedExactlyWithTheJob(t *testing.T) {
	root := t.TempDir()
	output := t.TempDir()
	p := &encounterProvider{task: "healing", cache: filepath.Join(root, "build-cache")}
	raw := []byte(`{"schema":1,"source":"test","seconds":30.125,"members":[{"label":"You","maxHealth":800000},{"label":"Ally","maxHealth":1000000}],"events":[{"time":1,"target":1,"amount":100000}]}`)
	if err := os.WriteFile(filepath.Join(root, "healing-replay.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	scenario := DefaultScenario()
	scenario.Task = "healing"
	scenario.HealingScenario = "replay"
	scenario.HealingPolicy = "compare"
	scenario.HealingRuns = 1
	scenario.HealingIntensity = 100
	r, err := p.Run(context.Background(), encounterSnapshot(), scenario, nil, output)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := os.ReadFile(filepath.Join(output, "damage-replay.json"))
	if err != nil || !bytes.Equal(raw, copy) || len(r.ModelInputs["damageProfileSHA256"]) != 64 {
		t.Fatal("replay input cannot be reproduced", err)
	}
}

func TestLearnedEncounterArchivesTrainingAndReportsUnmodeledTrinkets(t *testing.T) {
	root, logs, output := t.TempDir(), t.TempDir(), t.TempDir()
	var b strings.Builder
	for day := 20; day <= 22; day++ {
		line := func(second int, payload string) {
			fmt.Fprintf(&b, "9/%d/2026 13:00:%02d.000-4  %s\n", day, second, payload)
		}
		line(0, "COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1,BUILD_VERSION,12.1.0,PROJECT_ID,1")
		line(0, "ENCOUNTER_START,12,Fixture,2,2")
		for i, spec := range []int{1468, 73} {
			guid := fmt.Sprintf("Player-%d", i)
			line(0, "COMBATANT_INFO,"+guid+strings.Repeat(",0", 23)+fmt.Sprintf(",%d", spec))
			line(1, "SPELL_DAMAGE,Creature-1,Enemy,0,0,"+guid+",Ally,0,0,123,Hit,1,"+guid+",0,700000,800000"+strings.Repeat(",0", 15)+",100000"+strings.Repeat(",0", 9))
		}
		line(40, "ENCOUNTER_END,12,Fixture,2,2,1")
	}
	if err := os.WriteFile(filepath.Join(logs, "WoWCombatLog-test.txt"), []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	p := &encounterProvider{task: "healing", cache: filepath.Join(root, "build-cache"), logs: logs}
	s := encounterSnapshot()
	s.Items = []Item{{Slot: 13, Link: "item:270167"}}
	o := DefaultScenario()
	o.Task = "healing"
	o.HealingScenario = "learned"
	o.HealingRuns = 2
	o.HealingIntensity = 100
	o.HealingPolicy = "compare"
	r, err := p.Run(context.Background(), s, o, nil, output)
	if err != nil {
		t.Fatal(err)
	}
	if r.Healing.Training == nil || len(r.Healing.LearnedEncounters) != 1 || len(r.Healing.HoldoutOutcomes) != 2 || r.ModelInputs["unmodeledTrinketSlot13"] != "item:270167" {
		t.Fatal("incomplete learned/build report", r)
	}
	data, err := os.ReadFile(filepath.Join(output, "damage-training.json"))
	if err != nil {
		t.Fatal(err)
	}
	var training healsim.EncounterTraining
	if json.Unmarshal(data, &training) != nil || training.Validate() != nil || len(training.Samples) != 2 || len(r.ModelInputs["damageTrainingSHA256"]) != 64 {
		t.Fatal("training archive invalid")
	}
	o.HealingCohort = "missing"
	if _, err = p.Run(context.Background(), s, o, nil, output); err == nil {
		t.Fatal("missing training became synthetic damage")
	}
}

func buildPage() []byte {
	var b strings.Builder
	b.WriteString(`<html><time datetime="2026-09-29T02:03:40Z">Updated</time><span>Midnight Season 2</span><p>Patch 12.1.</p><a class="vi-intro-icon-button" href="/evoker/preservation/flameshaper/m+"><img alt="Flameshaper icon"><span>46 Flameshapers</span></a><a class="vi-intro-icon-button" href="/evoker/preservation/chronowarden/m+"><img alt="Chronowarden icon"><span>4 Chronowardens</span></a>`)
	b.WriteString(`<section id="stat-priority"><li class="guide-stats-chart-item"><span>25% Critical Strike</span><span>+924</span></li><li class="guide-stats-chart-item"><span>22% Haste</span><span>+948</span></li><li class="guide-stats-chart-item"><span>43% Mastery</span><span>+672</span></li><li class="guide-stats-chart-item"><span>9% Versatility</span><span>+465</span></li><h4>Stat priority</h4><ol><li>Haste</li><li>Critical Strike</li><li>Mastery</li><li>Versatility</li></ol></section><section id="gear">`)
	for i, slot := range []string{"Head", "Neck", "Shoulders", "Back", "Chest", "Wrist", "Hands", "Waist", "Legs", "Feet", "Rings", "Trinkets"} {
		fmt.Fprintf(&b, `<h3>%s</h3><ol><li><a class="list-item" href="https://www.wowhead.com/item=%d"><h4>Best %s</h4></a></li><li><a class="list-item" href="https://www.wowhead.com/item=%d"><h4>Second %s</h4></a></li></ol>`, slot, 270000+i*2, slot, 270001+i*2, slot)
	}
	b.WriteString(`</section>`)
	b.WriteString(`<a class="vi-box top-ratings-item legendary" href="/character/eu/realm/player/pve"><div></div><div class="h3">1</div><div><div class="h3">A&amp;B</div><div>Realm (EU)</div><div>Dracthyr | Flameshaper | 330 ilvl</div></div><div><div class="h3">3985</div></div></a>`)
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, `<a class="guide-talent guide-talent-background" href="/evoker/preservation/talents#test%d"><img alt="Talent %d"><div class="guide-talent-count">%d</div></a>`, i, i, 50-i)
	}
	return []byte(b.String())
}
func TestBuildSummaryParsingAndFailure(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	r, e := parseBuildPage(buildPage(), 1468, "m+", now)
	if e != nil {
		t.Fatal(e)
	}
	if r.Sample != 50 || r.Players[0].Name != "A&B" || r.Heroes[0].Percent != 92 || len(r.Talents) != 12 || r.Stale || len(r.Stats) != 4 || r.Stats[0].Key != "haste" || len(r.Gear) != 24 || !r.Gear[0].BIS || r.Gear[1].BIS || !r.Gear[20].BIS || !r.Gear[21].BIS {
		t.Fatal("wrong sample", r)
	}
	if _, e = parseBuildPage([]byte("challenge page"), 1468, "m+", now); e == nil {
		t.Fatal("incomplete page became valid")
	}
	r, e = parseBuildPage(buildPage(), 1468, "m+", now.Add(72*time.Hour))
	if e != nil || !r.Stale {
		t.Fatal("old data appeared fresh")
	}
}
func TestBuildCacheSkipsNetworkWithinTTL(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	r, e := parseBuildPage(buildPage(), 1468, "m+", now)
	if e != nil {
		t.Fatal(e)
	}
	r.UpdatedAt = now
	r.FetchedAt = now
	if e = writeJSON(filepath.Join(dir, "1468-mplus.json"), r); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, e := fetchBuilds(ctx, 1468, "m+", dir)
	if e != nil || !got.Cached {
		t.Fatal("fresh cache attempted network", e)
	}
	if _, e = fetchBuilds(context.Background(), 1468, "../../outside", dir); e == nil {
		t.Fatal("accepted arbitrary URL/cache path")
	}
}
func TestRealMurlokCaptureOptIn(t *testing.T) {
	path := os.Getenv("LAMDA_TEST_MURLOK_HTML")
	if path == "" {
		t.Skip("opt-in captured public page")
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	r, e := parseBuildPage(data, 1468, "m+", time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("parsed %d players, %d talents, sample %d, %s", len(r.Players), len(r.Talents), r.Sample, r.UpdatedAt)
}

func TestEveryHealerCapturesRoutesAndPublishesItsOwnModel(t *testing.T) {
	kits := map[int][]int{
		65:   {19750, 82326, 20473, 85673, 85222, 31884, 53563},
		105:  {8936, 774, 33763, 18562, 48438, 145205, 740},
		256:  {2061, 17, 194509, 585, 47540},
		257:  {2061, 596, 2050, 34861, 33076, 64843},
		264:  {77472, 8004, 61295, 1064, 73920, 5394, 108280},
		270:  {116670, 115151, 124682, 115310},
		1468: {361469, 364343, 366155, 355936, 360995, 355913, 359816, 363534, 370537, 1291636},
	}
	classes := map[int]string{65: "PALADIN", 105: "DRUID", 256: "PRIEST", 257: "PRIEST", 264: "SHAMAN", 270: "MONK", 1468: "EVOKER"}
	p := &encounterProvider{task: "healing"}
	for id, spells := range kits {
		t.Run(healsim.SpecName(id), func(t *testing.T) {
			s := encounterSnapshot()
			s.Character.Spec = id
			s.Character.Class = classes[id]
			s.HealingStats.KnownSpells = map[int]bool{}
			for _, spell := range spells {
				s.HealingStats.KnownSpells[spell] = true
			}
			if id == 1468 {
				s.HealingStats.KnownSpells[373861] = false
			}
			if id != 1468 {
				s.HealingStats.Talents = map[int]int{spells[0]: 1}
			}
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := DecodeSnapshot(data, "retail")
			if err != nil {
				t.Fatal(err)
			}
			if !snapshot.HealingStats.KnownSpells[spells[0]] {
				t.Fatal("lost spellbook in transfer")
			}
			scenario := DefaultScenario()
			scenario.Task = "healing"
			scenario.Seconds = 30
			scenario.HealingRuns = 2
			scenario.HealingScenario = "dungeon"
			scenario.HealingPolicy = "compare"
			scenario.HealingIntensity = 100
			result, err := p.Run(context.Background(), snapshot, scenario, nil, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if result.Healing.Spec != id || result.Healing.SpecName != healsim.SpecName(id) || result.Metric != "healing_hps" || result.Validation != "experimental" {
				t.Fatal("wrong model result")
			}
			if id == 1468 {
				for _, out := range result.Healing.Outcomes {
					if out.AbsorbPS <= 0 {
						t.Fatal("captured Barrier absorption lost in provider result")
					}
					for _, spell := range out.Spells {
						if spell.Name == "Temporal Anomaly" && spell.Casts != 0 {
							t.Fatal("provider cast replaced Anomaly")
						}
					}
				}
			}
			snapshot.Character.Class = "MAGE"
			if p.Capability(snapshot, time.Now()).Available {
				t.Fatal("mismatched class/spec accepted")
			}
		})
	}
}
