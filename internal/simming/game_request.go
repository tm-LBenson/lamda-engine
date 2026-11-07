package simming

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func gameJobID(s *Snapshot) string {
	h := sha256.Sum256([]byte(s.ID))
	return fmt.Sprintf("%d_game_%x", s.CapturedAt*1000, h[:12])
}

// Snapshot and job writes are separate atomic operations. Recover a capture
// persisted immediately before a crash without automatically rerunning it.
func (w *Workspace) recoverGameRequests() error {
	for _, s := range w.snapshots {
		if s.GameRequest == nil {
			continue
		}
		id := gameJobID(s)
		if w.jobs[id] != nil {
			continue
		}
		path := filepath.Join(w.dir, "jobs", id, "job.json")
		if data, err := readBounded(path, 2*1024*1024); err == nil {
			var j Job
			if json.Unmarshal(data, &j) == nil && j.ID == id && j.RequestID == s.ID && j.SnapshotID == s.ID {
				if j.State == "running" || j.State == "cancelling" {
					j.State = "interrupted"
					j.Error = "Engine stopped before this calculation completed. Send it again."
					if err := writeJSON(path, &j); err != nil {
						return err
					}
				}
				w.jobs[id] = &j
				continue
			}
		}
		if err := w.recordGameControl(s, "interrupted", "Engine stopped before this request started. Send it again.", nil); err != nil {
			return err
		}
	}
	return nil
}

// Items are matched again against the newly captured export. Positional itemN
// IDs are never trusted across captures or bag sorting.
type GameSelection struct {
	Source       string           `json:"source"`
	Slot         string           `json:"slot"`
	Placement    string           `json:"placement,omitempty"`
	PlannedLevel int              `json:"plannedLevel,omitempty"`
	Value        string           `json:"value"`
	Key          string           `json:"key,omitempty"`
	Link         string           `json:"link,omitempty"`
	Options      []ItemAdjustment `json:"options,omitempty"`
}

func compatiblePlacement(source, target string) bool {
	if source == target {
		return true
	}
	if strings.HasPrefix(source, "finger") && strings.HasPrefix(target, "finger") || strings.HasPrefix(source, "trinket") && strings.HasPrefix(target, "trinket") {
		return true
	}
	return source == "main_hand" && target == "off_hand"
}

type GameRequest struct {
	Schema      int             `json:"schema"`
	ID          string          `json:"id"`
	Task        string          `json:"task"`
	Selected    []GameSelection `json:"selected,omitempty"`
	Talents     []TalentBuild   `json:"talents,omitempty"`
	Scenario    Scenario        `json:"scenario"`
	CompareMode string          `json:"compareMode,omitempty"`
	JobID       string          `json:"jobID,omitempty"`
	PlanOptions *PlanOptions    `json:"planOptions,omitempty"`
	Workflow    *WorkflowDraft  `json:"workflow,omitempty"`
}

func (r *GameRequest) Validate(snapshotID, client string) error {
	if r.Schema != 1 || r.ID != snapshotID || !requestID.MatchString(r.ID) {
		return fmt.Errorf("invalid in-game request identity")
	}
	switch r.Task {
	case "baseline", "compare", "talents", "upgrades", "analysis", "cancel", "plan", "healing", "builds":
	default:
		return fmt.Errorf("unknown in-game calculation")
	}
	selectedLimit := 8
	if r.CompareMode == "loadout" {
		selectedLimit = 18
	}
	if len(r.Selected) > selectedLimit || len(r.Talents) > 16 {
		return fmt.Errorf("select at most eight items or sixteen talent builds")
	}
	if r.CompareMode != "" && r.CompareMode != "individual" && r.CompareMode != "topgear" && r.CompareMode != "loadout" {
		return fmt.Errorf("unknown comparison mode")
	}
	if r.Scenario.Task != "" || len(r.Scenario.Talents) != 0 || r.Scenario.CompareMode != "" || len(r.Scenario.ItemOptions) > 0 {
		return fmt.Errorf("use the request task, talents and comparison mode")
	}
	if err := r.Scenario.Validate(); err != nil {
		return err
	}
	if len(r.Scenario.Encounters) > 0 && r.Task != "analysis" {
		return fmt.Errorf("select one encounter or use a multi-encounter analysis")
	}
	planned := map[string]bool{}
	for _, c := range r.Selected {
		if (c.Source != "owned" && c.Source != "vault" && !(client == "retail" && c.Source == "equipped")) || (!slots[c.Slot] && !(client == "forever" && c.Slot == "ranged")) || len(c.Value) > 8192 || len(c.Key) > 160 || len(c.Link) > 4096 {
			return fmt.Errorf("invalid selected item")
		}
		if c.Placement != "" && (r.CompareMode != "loadout" || !slots[c.Placement] || !compatiblePlacement(c.Slot, c.Placement) || planned[c.Placement]) {
			return fmt.Errorf("invalid planned item placement")
		}
		if c.Placement != "" {
			planned[c.Placement] = true
		}
		if c.PlannedLevel != 0 && (r.CompareMode != "loadout" || c.PlannedLevel <= 0 || c.PlannedLevel > 2000) {
			return fmt.Errorf("invalid planned item level")
		}
		if _, err := itemValue(c.Value); err != nil {
			return err
		}
		if len(c.Options) > 8 || (len(c.Options) > 0 && (client != "retail" || (r.CompareMode != "topgear" && r.CompareMode != "loadout"))) {
			return fmt.Errorf("item alternatives require a Retail gear combination")
		}
		for _, a := range c.Options {
			if err := a.Validate(); err != nil {
				return err
			}
		}
	}
	seen := map[string]bool{}
	for _, t := range r.Talents {
		if len(t.Label) > 200 || strings.ContainsAny(t.Label, "\r\n\x00") || len(t.Code) > 2048 || !talentCode.MatchString(t.Code) || seen[t.Code] {
			return fmt.Errorf("invalid or duplicate talent build")
		}
		seen[t.Code] = true
	}
	loadout := r.Task == "compare" && r.CompareMode == "loadout"
	jointAnalysis := r.Task == "analysis" && r.CompareMode == "topgear"
	wrongSelection := r.Task == "compare" && !loadout && len(r.Selected) == 0 || (r.Task != "compare" && !jointAnalysis && len(r.Selected) > 0)
	wrongLoadout := loadout && (len(r.Selected) == 0 && len(r.Talents) == 0 || len(r.Talents) > 1)
	wrongTalents := r.Task == "talents" && len(r.Talents) == 0 || len(r.Talents) > 0 && r.Task != "talents" && r.Task != "analysis" && !loadout
	if wrongSelection || wrongLoadout || wrongTalents {
		return fmt.Errorf("select items or builds for the matching comparison")
	}
	if r.Task == "cancel" || r.Task == "plan" {
		if !requestID.MatchString(r.JobID) {
			return fmt.Errorf("select a saved run first")
		}
	} else if r.JobID != "" {
		return fmt.Errorf("unexpected saved run")
	}
	if (r.Task == "plan") != (r.PlanOptions != nil) {
		return fmt.Errorf("planning options require a planning request")
	}
	if client == "forever" && (r.Workflow != nil || len(r.Scenario.Encounters) > 0 || r.Scenario.Bloodlust != "") {
		return fmt.Errorf("combat workflow settings are Retail only")
	}
	if r.Workflow != nil {
		if err := r.Workflow.validate(); err != nil {
			return err
		}
	}
	return nil
}

func resolveGameSelection(s *Snapshot) ([]string, error) {
	if builtinTask(s.GameRequest.Task) && len(s.GameRequest.Selected) == 0 {
		return []string{}, nil
	}
	var candidates []Candidate
	if s.Client == "forever" {
		candidates = ForeverCandidates(s)
	} else {
		p, err := ParseProfile(s.Profile)
		if err != nil {
			return nil, err
		}
		candidates = p.Candidates
		for _, c := range displayGear(p) {
			candidates = append(candidates, c)
		}
	}
	selected := []string{}
	used := map[string]bool{}
	for _, wanted := range s.GameRequest.Selected {
		match := ""
		for _, c := range candidates {
			if used[c.ID] || c.Source != wanted.Source || c.Slot != wanted.Slot || c.Value != wanted.Value {
				continue
			}
			if s.Client == "forever" {
				if wanted.Key == "" || c.ID != wanted.Key || wanted.Link == "" {
					continue
				}
				exact := false
				if s.Gear != nil {
					for _, item := range s.Gear.Bags {
						if item.Key == wanted.Key && item.Link == wanted.Link {
							exact = true
							break
						}
					}
				}
				if !exact {
					continue
				}
			} else if wanted.Key != "" || wanted.Link != "" {
				return nil, fmt.Errorf("unexpected physical item key")
			}
			match = c.ID
			break
		}
		if match == "" {
			return nil, fmt.Errorf("a selected item changed or left your bags; refresh items and select it again")
		}
		if wanted.PlannedLevel > 0 {
			valid := false
			for _, upgrade := range s.Upgrades {
				if upgrade.Source != wanted.Source || upgrade.Slot != wanted.Slot || upgrade.Value != wanted.Value {
					continue
				}
				for _, level := range upgrade.RankLevels {
					if level == wanted.PlannedLevel {
						valid = true
						break
					}
				}
			}
			if !valid {
				return nil, fmt.Errorf("the planned item rank is no longer available; update the character")
			}
		}
		used[match] = true
		selected = append(selected, match)
	}
	if len(s.GameRequest.Selected) > 0 {
		scenario := s.GameRequest.Scenario
		scenario.CompareMode = s.GameRequest.CompareMode
		scenario.ItemOptions = map[string][]ItemAdjustment{}
		for i, id := range selected {
			if len(s.GameRequest.Selected[i].Options) > 0 {
				scenario.ItemOptions[id] = s.GameRequest.Selected[i].Options
			}
		}
		if len(scenario.ItemOptions) > 0 {
			p, err := ParseProfile(s.Profile)
			if err != nil {
				return nil, err
			}
			if _, err = prepareComparisonProfile(s, p, scenario, selected); err != nil {
				return nil, err
			}
		}
	}
	return selected, nil
}

func labelGameTalents(s *Snapshot, result *Result) {
	if s.GameRequest == nil || result == nil {
		return
	}
	for i := range result.Comparisons {
		metric := &result.Comparisons[i]
		if !strings.HasPrefix(metric.Name, "talent") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(metric.Name, "talent"))
		if err == nil && n > 0 && n <= len(s.GameRequest.Talents) {
			if label := s.GameRequest.Talents[n-1].Label; label != "" {
				metric.Label = label
			}
		}
	}
}

// ReceiveGame persists every explicit request, including rejected requests, so
// the next /reload can explain failure rather than showing an endless spinner.
func (w *Workspace) ReceiveGame(s *Snapshot) (bool, error) {
	if s.GameRequest == nil {
		return false, fmt.Errorf("missing in-game request")
	}
	fresh, err := w.Receive(s)
	if err != nil || !fresh {
		return fresh, err
	}
	r := s.GameRequest
	var runErr error
	if r.Workflow != nil {
		_, runErr = w.SaveWorkflow(s.ID, *r.Workflow)
	}
	if runErr != nil {
		return true, w.recordGameFailure(s, runErr)
	}
	if r.Task == "cancel" || r.Task == "plan" {
		runErr = w.gameControl(s)
	} else {
		var selected []string
		selected, runErr = resolveGameSelection(s)
		if runErr == nil {
			scenario := r.Scenario
			scenario.CompareMode = r.CompareMode
			scenario.ItemOptions = map[string][]ItemAdjustment{}
			for i, id := range selected {
				if len(r.Selected[i].Options) > 0 {
					scenario.ItemOptions[id] = r.Selected[i].Options
				}
			}
			if r.CompareMode == "loadout" {
				scenario.LoadoutSlots = map[string]string{}
				scenario.LoadoutLevels = map[string]int{}
				for index, id := range selected {
					placement := r.Selected[index].Placement
					if placement == "" {
						placement = r.Selected[index].Slot
					}
					if used := scenario.LoadoutSlots[id]; used != "" && used != placement {
						runErr = fmt.Errorf("a planned item has conflicting placements")
						break
					}
					scenario.LoadoutSlots[id] = placement
					if level := r.Selected[index].PlannedLevel; level > 0 {
						scenario.LoadoutLevels[id] = level
					}
				}
			}
			if r.Task == "upgrades" || r.Task == "analysis" || builtinTask(r.Task) {
				scenario.Task = r.Task
			}
			for _, t := range r.Talents {
				scenario.Talents = append(scenario.Talents, t.Code)
			}
			if runErr == nil {
				_, runErr = w.start(s.ID, scenario, selected, true)
			}
		}
	}
	if runErr != nil {
		if err := w.recordGameControl(s, "failed", runErr.Error(), nil); err != nil {
			return true, err
		}
	}
	return true, runErr
}

func (w *Workspace) recordGameFailure(s *Snapshot, err error) error {
	if saveErr := w.recordGameControl(s, "failed", err.Error(), nil); saveErr != nil {
		return saveErr
	}
	return err
}

func (w *Workspace) gameControl(s *Snapshot) error {
	r := s.GameRequest
	w.mu.Lock()
	old := w.jobs[r.JobID]
	var oldSnapshot *Snapshot
	if old != nil {
		oldSnapshot = w.snapshots[old.SnapshotID]
	}
	same := oldSnapshot != nil && oldSnapshot.Character.GUID == s.Character.GUID && oldSnapshot.Character.Spec == s.Character.Spec
	w.mu.Unlock()
	if !same {
		return fmt.Errorf("that run belongs to another character or specialization, or has expired")
	}
	if r.Task == "cancel" {
		if err := w.Cancel(r.JobID); err != nil {
			return err
		}
		return w.recordGameControl(s, "complete", "", nil)
	}
	plan, err := w.Plan(r.JobID, *r.PlanOptions)
	if err != nil {
		return err
	}
	return w.recordGameControl(s, "complete", "", plan)
}
func (w *Workspace) recordGameControl(s *Snapshot, state, problem string, plan *AcquisitionPlan) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now().UTC()
	j := &Job{ID: gameJobID(s), SnapshotID: s.ID, CreatedAt: now, FinishedAt: &now, State: state, Scenario: s.GameRequest.Scenario, GameTask: s.GameRequest.Task, RequestID: s.GameRequest.ID, Error: problem, Plan: plan, Selected: []string{}, Audit: w.auditor.State()}
	if err := writeJSON(filepath.Join(w.dir, "jobs", j.ID, "job.json"), j); err != nil {
		return err
	}
	w.jobs[j.ID] = j
	w.revision++
	return nil
}
