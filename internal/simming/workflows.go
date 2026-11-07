package simming

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrWorkflowConflict = errors.New("sim workflow changed; refresh it before saving")

// Saved selections use captured item identities, never itemN offsets. Context
// and GUID/spec isolate raid, M+, alts and specialization preferences.
type WorkflowDraft struct {
	ExpectedRevision uint64          `json:"expectedRevision"`
	Context          string          `json:"context"`
	Scenario         Scenario        `json:"scenario"`
	CompareMode      string          `json:"compareMode"`
	Selected         []GameSelection `json:"selected"`
	Talents          []TalentBuild   `json:"talents,omitempty"`
}
type Workflow struct {
	CharacterGUID string        `json:"characterGUID"`
	Spec          int           `json:"spec"`
	Revision      uint64        `json:"revision"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Settings      WorkflowDraft `json:"settings"`
}
type WorkflowSelection struct {
	Item      GameSelection `json:"item"`
	Available bool          `json:"available"`
	Reason    string        `json:"reason,omitempty"`
}
type WorkflowView struct {
	Workflow   Workflow            `json:"workflow"`
	Selections []WorkflowSelection `json:"selections"`
	Presets    []Encounter         `json:"presets"`
}
type workflowFile struct {
	Schema  int                 `json:"schema"`
	Client  string              `json:"client"`
	Records map[string]Workflow `json:"records"`
}

func workflowKey(guid string, spec int, context string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", guid, spec, context))))
}
func cloneJSON[T any](value T) T {
	data, _ := json.Marshal(value)
	var out T
	_ = json.Unmarshal(data, &out)
	return out
}
func (d WorkflowDraft) validate() error {
	if d.Context != "raid" && d.Context != "m+" {
		return fmt.Errorf("choose Raid or Mythic+ workflow context")
	}
	if d.CompareMode != "individual" && d.CompareMode != "topgear" && d.CompareMode != "loadout" {
		return fmt.Errorf("unknown workflow comparison mode")
	}
	if d.Scenario.Task != "" || len(d.Scenario.Talents) > 0 || len(d.Scenario.ItemOptions) > 0 || len(d.Scenario.LoadoutLevels) > 0 || len(d.Scenario.LoadoutSlots) > 0 || d.Scenario.CompareMode != "" {
		return fmt.Errorf("save physical item identities and choices rather than snapshot offsets")
	}
	task := "compare"
	if len(d.Selected) == 0 && len(d.Talents) == 0 {
		task = "baseline"
	} else if len(d.Selected) == 0 && d.CompareMode != "loadout" {
		task = "talents"
	}
	// Talent alternatives are kept separately from gear alternatives, unless
	// the user explicitly selected the one talent build for an exact loadout.
	talents := d.Talents
	if d.CompareMode != "loadout" && task != "talents" {
		talents = nil
	}
	if err := d.Scenario.Validate(); err != nil {
		return err
	}
	scenario := d.Scenario
	scenario.Encounters = nil
	r := GameRequest{Schema: 1, ID: "workflow", Task: task, Scenario: scenario, CompareMode: d.CompareMode, Selected: d.Selected, Talents: talents}
	if err := r.Validate("workflow", "retail"); err != nil {
		return err
	}
	if len(d.Talents) > 16 {
		return fmt.Errorf("choose at most sixteen talent builds")
	}
	seen := map[string]bool{}
	for _, t := range d.Talents {
		if !talentCode.MatchString(t.Code) || len(t.Code) > 2048 || len(t.Label) > 200 || strings.ContainsAny(t.Label, "\r\n\x00") || seen[t.Code] {
			return fmt.Errorf("invalid saved talent choice")
		}
		seen[t.Code] = true
	}
	return nil
}

func (w *Workspace) loadWorkflows() {
	w.workflows = map[string]Workflow{}
	data, err := readBounded(filepath.Join(w.dir, "workflow-state.json"), 1024*1024)
	if os.IsNotExist(err) {
		return
	}
	var file workflowFile
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&file)
		if err == nil && decoder.Decode(new(any)) != io.EOF {
			err = fmt.Errorf("extra workflow data")
		}
	}
	if err == nil && (file.Schema != 1 || file.Client != w.client || len(file.Records) > 128) {
		err = fmt.Errorf("invalid saved workflows")
	}
	if err == nil {
		for key, entry := range file.Records {
			if entry.CharacterGUID == "" || len(entry.CharacterGUID) > 128 || entry.Spec <= 0 || entry.Revision == 0 || entry.Settings.validate() != nil || key != workflowKey(entry.CharacterGUID, entry.Spec, entry.Settings.Context) {
				err = fmt.Errorf("invalid saved workflow record")
				break
			}
		}
	}
	if err != nil {
		w.workflowProblem = "Saved sim workflows could not be read. The original file was preserved; other simulations remain available."
		return
	}
	if file.Records != nil {
		w.workflows = file.Records
	}
}

func (w *Workspace) SaveWorkflow(snapshotID string, draft WorkflowDraft) (*Workflow, error) {
	draft = cloneJSON(draft)
	if err := draft.validate(); err != nil {
		return nil, err
	}
	s, err := w.Snapshot(snapshotID)
	if err != nil {
		return nil, err
	}
	if s.Client != "retail" {
		return nil, fmt.Errorf("saved combat workflows are Retail only")
	}
	if s.Character.Role != "DAMAGER" && (len(draft.Scenario.Encounters) > 0 || draft.Scenario.Bloodlust != "" || draft.CompareMode != "individual" || hasItemOptions(draft.Selected)) {
		return nil, fmt.Errorf("this calculator cannot apply combat presets or complete gear alternatives")
	}
	r := GameRequest{Schema: 1, ID: s.ID, Task: "compare", Scenario: draft.Scenario, CompareMode: draft.CompareMode, Selected: draft.Selected}
	check := *s
	check.GameRequest = &r
	if _, err = resolveGameSelection(&check); err != nil {
		return nil, err
	}
	key := workflowKey(s.Character.GUID, s.Character.Spec, draft.Context)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.workflowProblem != "" {
		return nil, fmt.Errorf("%s", w.workflowProblem)
	}
	current := w.workflows[key]
	if current.Revision != draft.ExpectedRevision {
		return nil, ErrWorkflowConflict
	}
	if current.Revision == 0 && len(w.workflows) >= 128 {
		return nil, fmt.Errorf("saved workflow capacity reached")
	}
	entry := Workflow{s.Character.GUID, s.Character.Spec, current.Revision + 1, time.Now().UTC(), draft}
	entry.Settings.ExpectedRevision = entry.Revision
	records := cloneJSON(w.workflows)
	records[key] = entry
	data, err := json.MarshalIndent(workflowFile{1, w.client, records}, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, fmt.Errorf("saved workflows exceed the storage limit")
	}
	if err = writeJSON(filepath.Join(w.dir, "workflow-state.json"), workflowFile{1, w.client, records}); err != nil {
		return nil, err
	}
	w.workflows = records
	w.revision++
	copy := cloneJSON(entry)
	return &copy, nil
}

func hasItemOptions(items []GameSelection) bool {
	for _, item := range items {
		if len(item.Options) > 0 {
			return true
		}
	}
	return false
}

func defaultEncounters() []Encounter {
	return []Encounter{{"raid", "Raid boss", "patchwerk", 1, 300, "full", ""}, {"dungeon", "Dungeon Slice", "dungeon_slice", 1, 360, "none", ""},
		{"trash", "Seven-target trash test", "patchwerk", 7, 180, "none", "off"}, {"long_raid", "Ten-minute boss test", "patchwerk", 1, 600, "full", "on"}}
}

func (w *Workspace) Workflow(snapshotID, context string) (*WorkflowView, error) {
	if context != "raid" && context != "m+" {
		return nil, fmt.Errorf("choose Raid or Mythic+ workflow context")
	}
	s, err := w.Snapshot(snapshotID)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	problem := w.workflowProblem
	entry := cloneJSON(w.workflows[workflowKey(s.Character.GUID, s.Character.Spec, context)])
	w.mu.Unlock()
	if problem != "" {
		return nil, fmt.Errorf("%s", problem)
	}
	if entry.Revision == 0 {
		entry = Workflow{CharacterGUID: s.Character.GUID, Spec: s.Character.Spec, Settings: WorkflowDraft{Context: context, Scenario: DefaultScenario(), CompareMode: "topgear"}}
	}
	view := &WorkflowView{Workflow: entry, Selections: []WorkflowSelection{}, Presets: defaultEncounters()}
	for _, item := range entry.Settings.Selected {
		r := GameRequest{Selected: []GameSelection{item}, Scenario: entry.Settings.Scenario, CompareMode: entry.Settings.CompareMode}
		check := *s
		check.GameRequest = &r
		_, err := resolveGameSelection(&check)
		status := WorkflowSelection{Item: item, Available: err == nil}
		if err != nil {
			status.Reason = err.Error()
		}
		view.Selections = append(view.Selections, status)
	}
	return view, nil
}

type WorkflowRun struct {
	SnapshotID   string   `json:"snapshotID"`
	Context      string   `json:"context"`
	Revision     uint64   `json:"revision"`
	Task         string   `json:"task"`
	EncounterIDs []string `json:"encounterIDs,omitempty"`
}

func (w *Workspace) RunWorkflow(request WorkflowRun) (*Job, error) {
	if request.Task != "baseline" && request.Task != "compare" && request.Task != "analysis" && request.Task != "upgrades" && request.Task != "talents" {
		return nil, fmt.Errorf("choose a workflow calculation")
	}
	view, err := w.Workflow(request.SnapshotID, request.Context)
	if err != nil {
		return nil, err
	}
	if request.Revision == 0 || request.Revision != view.Workflow.Revision {
		return nil, ErrWorkflowConflict
	}
	d := view.Workflow.Settings
	scenario := d.Scenario
	if len(request.EncounterIDs) > 0 {
		chosen := []Encounter{}
		seen := map[string]bool{}
		for _, id := range request.EncounterIDs {
			if seen[id] {
				return nil, fmt.Errorf("select each encounter once")
			}
			seen[id] = true
			found := false
			for _, e := range scenario.Encounters {
				if e.ID == id {
					chosen = append(chosen, e)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("encounter is not in the saved workflow")
			}
		}
		scenario.Encounters = chosen
	}
	if request.Task != "analysis" && len(scenario.Encounters) > 0 {
		if len(scenario.Encounters) != 1 {
			return nil, fmt.Errorf("select one encounter for this calculation")
		}
		scenario = scenario.Encounters[0].apply(scenario)
	}
	s, err := w.Snapshot(request.SnapshotID)
	if err != nil {
		return nil, err
	}
	r := GameRequest{Schema: 1, ID: s.ID, Task: request.Task, Scenario: scenario, CompareMode: d.CompareMode}
	if request.Task == "compare" || (request.Task == "analysis" && d.CompareMode == "topgear") {
		r.Selected = d.Selected
	}
	if request.Task == "talents" || (request.Task == "compare" && d.CompareMode == "loadout") {
		r.Talents = d.Talents
	}
	if err = r.Validate(s.ID, w.client); err != nil {
		return nil, err
	}
	check := *s
	check.GameRequest = &r
	selected, err := resolveGameSelection(&check)
	if err != nil {
		return nil, err
	}
	scenario.CompareMode = r.CompareMode
	scenario.ItemOptions = map[string][]ItemAdjustment{}
	if r.CompareMode == "loadout" {
		scenario.LoadoutSlots = map[string]string{}
		scenario.LoadoutLevels = map[string]int{}
	}
	for i, id := range selected {
		item := r.Selected[i]
		if len(item.Options) > 0 {
			scenario.ItemOptions[id] = item.Options
		}
		if r.CompareMode == "loadout" {
			slot := item.Placement
			if slot == "" {
				slot = item.Slot
			}
			scenario.LoadoutSlots[id] = slot
			if item.PlannedLevel > 0 {
				scenario.LoadoutLevels[id] = item.PlannedLevel
			}
		}
	}
	for _, t := range r.Talents {
		scenario.Talents = append(scenario.Talents, t.Code)
	}
	if request.Task == "analysis" || request.Task == "upgrades" {
		scenario.Task = request.Task
	}
	return w.Start(s.ID, scenario, selected)
}
