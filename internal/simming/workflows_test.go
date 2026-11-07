package simming

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func workflowWorkspace(t *testing.T) (*Workspace, *Snapshot) {
	t.Helper()
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	s := fixture(time.Now())
	if _, err = w.Receive(s); err != nil {
		t.Fatal(err)
	}
	return w, s
}
func workflowDraft() WorkflowDraft {
	s := DefaultScenario()
	s.Encounters = []Encounter{{"trash", "Trash", "patchwerk", 7, 180, "none", "off"}, {"boss", "Boss", "patchwerk", 1, 600, "full", "on"}}
	return WorkflowDraft{Context: "m+", Scenario: s, CompareMode: "individual", Selected: []GameSelection{{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3"}}}
}

func TestWorkflowPersistsRebasesSelectionsAndProtectsContexts(t *testing.T) {
	w, s := workflowWorkspace(t)
	draft := workflowDraft()
	saved, err := w.SaveWorkflow(s.ID, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.SaveWorkflow(s.ID, draft); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatalf("stale save accepted: %v", err)
	}
	// Returned values cannot mutate the authoritative saved settings.
	saved.Settings.Selected[0].Value = "id=999"
	newer := *s
	newer.ID = "fresh_sorted"
	newer.CapturedAt++
	newer.Profile = strings.Replace(s.Profile, "### Gear from Bags", "### Gear from Bags\n# Newly looted item\n# head=,id=777", 1)
	if _, err = w.Receive(&newer); err != nil {
		t.Fatal(err)
	}
	view, err := w.Workflow(newer.ID, "m+")
	if err != nil {
		t.Fatal(err)
	}
	if view.Workflow.Revision != 1 || !view.Selections[0].Available || view.Selections[0].Item.Value != "id=124,bonus_id=2/3" {
		t.Fatal(view)
	}
	raid, _ := w.Workflow(newer.ID, "raid")
	if raid.Workflow.Revision != 0 {
		t.Fatal("raid settings inherited from M+")
	}
	other := newer
	other.ID = "other_spec"
	other.Character.Spec = 63
	if _, err = w.Receive(&other); err != nil {
		t.Fatal(err)
	}
	offspec, _ := w.Workflow(other.ID, "m+")
	if offspec.Workflow.Revision != 0 {
		t.Fatal("off-spec settings inherited")
	}
	reloaded, err := NewWorkspace(context.Background(), w.dir, "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	view, err = reloaded.Workflow(newer.ID, "m+")
	if err != nil || view.Workflow.Revision != 1 || len(view.Workflow.Settings.Scenario.Encounters) != 2 {
		t.Fatalf("restart lost preferences: %+v %v", view, err)
	}
	changed := newer
	changed.ID = "new_gem"
	changed.Profile = strings.Replace(newer.Profile, "id=124,bonus_id=2/3", "id=124,bonus_id=2/3,gem_id=99", 1)
	if _, err = w.Receive(&changed); err != nil {
		t.Fatal(err)
	}
	unavailable, _ := w.Workflow(changed.ID, "m+")
	if unavailable.Selections[0].Available || unavailable.Selections[0].Reason == "" {
		t.Fatal("modified physical identity silently reused")
	}
	retained, _ := w.Workflow(newer.ID, "m+")
	if !retained.Selections[0].Available {
		t.Fatal("unavailable item was removed from the saved selections")
	}
}

func TestCorruptWorkflowFileIsPreservedAndCannotBeOverwritten(t *testing.T) {
	w, s := workflowWorkspace(t)
	path := filepath.Join(w.dir, "workflow-state.json")
	original := []byte(`{"schema":1,"broken":true}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewWorkspace(context.Background(), w.dir, "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	if _, err = reloaded.SaveWorkflow(s.ID, workflowDraft()); err == nil {
		t.Fatal("corrupt preferences overwritten")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatal("original preferences lost")
	}
	if reloaded.State().WorkflowError == "" {
		t.Fatal("missing recovery explanation")
	}
}

func TestWorkflowHTTPBoundaryAndRevisionConflict(t *testing.T) {
	w, s := workflowWorkspace(t)
	service := &Service{Workspace: w}
	handler := service.Handler("127.0.0.1:47641")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:47641"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://127.0.0.1:47641")
		r.Header.Set("X-Lamda-Sim", "1")
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, r)
		return out
	}
	draft := workflowDraft()
	data, _ := json.Marshal(map[string]any{"snapshotID": s.ID, "settings": draft})
	if r := call("POST", "/api/sim/workflow", string(data)); r.Code != http.StatusOK {
		t.Fatalf("save: %d %s", r.Code, r.Body.String())
	}
	if r := call("POST", "/api/sim/workflow", string(data)); r.Code != http.StatusConflict {
		t.Fatalf("conflict: %d %s", r.Code, r.Body.String())
	}
	if r := call("GET", "/api/sim/workflow?snapshotID="+s.ID+"&context=m%2B", ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"revision":1`) {
		t.Fatal(r.Body.String())
	}
	if r := call("POST", "/api/sim/workflow", `{"snapshotID":"player_123","unexpected":true}`); r.Code != 400 {
		t.Fatal("unknown fields accepted")
	}
	if r := call("POST", "/api/sim/workflow/run", `{"snapshotID":"player_123","context":"m+","revision":99,"task":"analysis"}`); r.Code != 409 {
		t.Fatal("stale workflow executed")
	}
}

func TestCustomEncounterAnalysisKeepsActualConditions(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "analysis")
	s := fixture(time.Now())
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 125, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	scenario := workflowDraft().Scenario
	scenario.Task = "analysis"
	result, err := testProvider(t).Run(context.Background(), s, scenario, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.TargetResults) != 2 {
		t.Fatal(result)
	}
	first, second := result.TargetResults[0], result.TargetResults[1]
	if first.Targets != 1 || first.TargetCount != 7 || first.Seconds != 180 || first.Bloodlust != "off" || first.EncounterID != "trash" || second.Targets != 5 || second.TargetCount != 1 || second.Seconds != 600 || second.Bloodlust != "on" {
		t.Fatalf("conditions lost: %+v", result.TargetResults)
	}
}

func TestSavedWorkflowRunsRebasedItemsInOnePresetWithoutChangingPreferences(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "analysis")
	w, s := workflowWorkspace(t)
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}, {ID: 124, BonusIDs: []int{2, 3}, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	s.ID = "rules_capture"
	if _, err := w.Receive(s); err != nil {
		t.Fatal(err)
	}
	saved, err := w.SaveWorkflow(s.ID, workflowDraft())
	if err != nil {
		t.Fatal(err)
	}
	newer := *s
	newer.ID = "resorted_capture"
	newer.Profile = strings.Replace(s.Profile, "### Gear from Bags", "### Gear from Bags\n# New helm\n# head=,id=777", 1)
	if _, err = w.Receive(&newer); err != nil {
		t.Fatal(err)
	}
	job, err := w.RunWorkflow(WorkflowRun{SnapshotID: newer.ID, Context: "m+", Revision: saved.Revision, Task: "compare", EncounterIDs: []string{"boss"}})
	if err != nil {
		t.Fatal(err)
	}
	complete := waitJob(t, w, job.ID)
	if complete.State != "complete" || len(complete.Selected) != 1 || complete.Selected[0] != "item2" || complete.Scenario.Targets != 1 || complete.Scenario.Seconds != 600 || complete.Scenario.Bloodlust != "on" {
		t.Fatalf("saved conditions or rebased item lost: %+v", complete)
	}
	decisions, err := w.Decisions(newer.ID, "m+")
	if err != nil || len(decisions.Views) != 1 || decisions.Views[0].Metric != "dps" || len(decisions.Views[0].Equip) != 1 {
		t.Fatalf("worker DPS result did not reach decisions: %+v %v", decisions, err)
	}
	view, _ := w.Workflow(newer.ID, "m+")
	if view.Workflow.Revision != saved.Revision || len(view.Workflow.Settings.Scenario.Encounters) != 2 || view.Workflow.Settings.Selected[0].Value != "id=124,bonus_id=2/3" {
		t.Fatal("running one encounter changed the saved workflow")
	}
}

func TestSavedTalentAlternativesRunSeparatelyFromTheRetainedGearChoices(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "analysis")
	w, s := workflowWorkspace(t)
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	s.ID = "talent_rules"
	if _, err := w.Receive(s); err != nil {
		t.Fatal(err)
	}
	draft := workflowDraft()
	draft.Talents = []TalentBuild{{Label: "Alternate build", Code: "OTHER+=="}}
	saved, err := w.SaveWorkflow(s.ID, draft)
	if err != nil {
		t.Fatal(err)
	}
	job, err := w.RunWorkflow(WorkflowRun{SnapshotID: s.ID, Context: "m+", Revision: saved.Revision, Task: "talents", EncounterIDs: []string{"boss"}})
	if err != nil {
		t.Fatal(err)
	}
	result := waitJob(t, w, job.ID)
	if result.State != "complete" || len(result.Selected) != 0 || len(result.Scenario.Talents) != 1 || len(result.Result.Comparisons) != 1 {
		t.Fatalf("talent choices ignored or mixed with gear: job=%+v result=%+v", result, result.Result)
	}
	view, _ := w.Workflow(s.ID, "m+")
	if len(view.Workflow.Settings.Selected) != 1 || view.Workflow.Revision != saved.Revision {
		t.Fatal("talent comparison erased gear selections")
	}
	draft.Selected = nil
	draft.ExpectedRevision = saved.Revision
	if _, err := w.SaveWorkflow(s.ID, draft); err != nil {
		t.Fatalf("talent-only settings could not be saved: %v", err)
	}
}

func TestItemAlternativesRetainCompletedResultsWhenBudgetExpires(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "planned_timeout")
	p := testProvider(t)
	s := fixture(time.Now())
	profile, _ := ParseProfile(s.Profile)
	zero := 0
	scenario := DefaultScenario()
	scenario.CompareMode = "topgear"
	scenario.ItemOptions = map[string][]ItemAdjustment{"equipped_main_hand": {{EnchantID: &zero}}}
	item, err := adjustedItem(Candidate{ID: "equipped_main_hand", Source: "equipped", Slot: "main_hand", Value: "id=123,bonus_id=1/2"}, ItemAdjustment{EnchantID: &zero})
	if err != nil {
		t.Fatal(err)
	}
	comparisons := []Comparison{{ID: "first", Items: []Candidate{item}}, {ID: "slow", Items: []Candidate{item}}}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := os.Stat(filepath.Join(dir, "screen-000", "variant-001", "input.simc")); err == nil {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	result, tested, _, _, complete, err := p.runBudgetedAnalysisTarget(ctx, s, profile, scenario, comparisons, dir)
	<-done
	if err != nil || complete || tested != 1 || result == nil || !result.Incomplete || len(result.Comparisons) != 1 || result.Comparisons[0].Name != "first" {
		t.Fatalf("completed partial result lost: %+v tested=%d complete=%v err=%v", result, tested, complete, err)
	}
}

func TestBloodlustIsIndependentAndInvalidEncountersFail(t *testing.T) {
	p, _ := ParseProfile(profileFixture)
	for _, buffs := range []string{"none", "full"} {
		for _, lust := range []string{"on", "off"} {
			s := DefaultScenario()
			s.RaidBuffs, s.Bloodlust, s.Targets = buffs, lust, 20
			input, err := p.Input(s, nil)
			if err != nil {
				t.Fatal(err)
			}
			expected := "override.bloodlust=0"
			if lust == "on" {
				expected = "override.bloodlust=1"
			}
			if !strings.Contains(input, expected) || strings.Index(input, expected) < strings.Index(input, "optimal_raid=") {
				t.Fatal(input)
			}
		}
	}
	for _, change := range []func(*Scenario){func(s *Scenario) { s.Targets = 21 }, func(s *Scenario) { s.Bloodlust = "maybe" }, func(s *Scenario) { s.FightStyle = "dungeon_slice"; s.Bloodlust = "on" }, func(s *Scenario) { s.Encounters = []Encounter{{ID: "../file"}} }} {
		s := DefaultScenario()
		change(&s)
		if s.Validate() == nil {
			t.Fatal("invalid encounter accepted")
		}
	}
}

func TestItemAlternativesKeepPhysicalIdentityRanksAndSockets(t *testing.T) {
	s := fixture(time.Now())
	s.Profile = strings.Replace(s.Profile, "id=124,bonus_id=2/3", "id=124,bonus_id=2/3,gem_id=901", 1)
	p, _ := ParseProfile(s.Profile)
	zero := 0
	scenario := DefaultScenario()
	scenario.CompareMode = "topgear"
	scenario.ItemOptions = map[string][]ItemAdjustment{"item1": {{EnchantID: &zero}, {GemIDs: []int{902}}}}
	prepared, err := prepareComparisonProfile(s, p, scenario, []string{"item1"})
	if err != nil {
		t.Fatal(err)
	}
	choices, err := prepared.Comparisons(scenario, []string{"item1"})
	if err != nil || len(choices) != 3 {
		t.Fatalf("alternatives: %+v %v", choices, err)
	}
	for _, c := range choices {
		if len(c.Items) != 1 || itemID(c.Items[0].Value) != "124" || itemField(c.Items[0].Value, "bonus_id") != "2/3" {
			t.Fatal("physical item or bonuses changed")
		}
	}
	scenario.ItemOptions["item1"] = []ItemAdjustment{{GemIDs: []int{901, 902}}}
	if _, err = prepareComparisonProfile(s, p, scenario, []string{"item1"}); err == nil {
		t.Fatal("socket addition accepted")
	}
	scenario.ItemOptions["item1"] = []ItemAdjustment{{Level: 350}}
	if _, err = prepareComparisonProfile(s, p, scenario, []string{"item1"}); err == nil {
		t.Fatal("uncaptured upgrade rank accepted")
	}
	s.Upgrades = []ItemUpgrade{{Source: "owned", Slot: "main_hand", Value: p.Candidates[0].Value, RankLevels: []int{340, 350}}}
	if _, err = prepareComparisonProfile(s, p, scenario, []string{"item1"}); err != nil {
		t.Fatal(err)
	}
}

func TestCombinedResultIncludesEveryChange(t *testing.T) {
	metric := Metric{Name: "combo", Mean: 100}
	c := Comparison{ID: "combo", Items: []Candidate{{ID: "one", Source: "owned", Slot: "finger1", Value: "id=101"}, {ID: "two", Source: "owned", Slot: "trinket1", Value: "id=102"}}}
	decorateMetric(&metric, c)
	if len(metric.Items) != 2 || metric.Source != "set" || metric.CandidateID != "combo" || metric.Items[1].ID != 102 {
		t.Fatalf("combined set truncated: %+v", metric)
	}
}

func TestNativeWorkflowSnapshotOptIn(t *testing.T) {
	path := os.Getenv("LAMDA_SIM_WORKFLOW_SNAPSHOT")
	if path == "" {
		t.Skip("set LAMDA_SIM_WORKFLOW_SNAPSHOT to test the real Lua exporter")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeSnapshot(data, "retail")
	if err != nil {
		t.Fatal(err)
	}
	r := s.GameRequest
	if r == nil || r.Task != "analysis" || r.CompareMode != "topgear" || len(r.Scenario.Encounters) != 2 || len(r.Selected[0].Options) != 1 {
		t.Fatal("native workflow contract lost data")
	}
	if _, err = resolveGameSelection(s); err != nil {
		t.Fatal(err)
	}
}

func TestNativeCustomUIWorkflowSnapshotOptIn(t *testing.T) {
	path := os.Getenv("LAMDA_SIM_CUSTOM_UI_SNAPSHOT")
	if path == "" {
		t.Skip("set LAMDA_SIM_CUSTOM_UI_SNAPSHOT to test the real customization UI")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeSnapshot(data, "retail")
	if err != nil {
		t.Fatal(err)
	}
	r := s.GameRequest
	if r == nil || r.Workflow == nil || r.Task != "analysis" || r.CompareMode != "topgear" || len(r.Selected) != 1 || len(r.Scenario.Encounters) != 2 || r.Workflow.Context != "m+" || r.Scenario.Encounters[0].Targets != 7 || r.Scenario.Encounters[1].Bloodlust != "on" {
		t.Fatal("customization UI contract lost data")
	}
	if _, err = resolveGameSelection(s); err != nil {
		t.Fatal(err)
	}
}
