package simming

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/atomicfile"
)

type Job struct {
	SetupHash  string           `json:"setupHash,omitempty"`
	GameTask   string           `json:"gameTask,omitempty"`
	RequestID  string           `json:"requestID,omitempty"`
	Plan       *AcquisitionPlan `json:"plan,omitempty"`
	ID         string           `json:"id"`
	SnapshotID string           `json:"snapshotId"`
	CreatedAt  time.Time        `json:"createdAt"`
	FinishedAt *time.Time       `json:"finishedAt,omitempty"`
	State      string           `json:"state"`
	Scenario   Scenario         `json:"scenario"`
	Selected   []string         `json:"selected"`
	Result     *Result          `json:"result,omitempty"`
	Audit      Audit            `json:"audit"`
	Error      string           `json:"error,omitempty"`
}
type Workspace struct {
	revision            uint64
	publicationRevision uint64
	publicationState    WorkspaceState
	publicationData     []byte
	publicationAt       time.Time
	publicationMu       sync.Mutex
	mu                  sync.Mutex
	client, dir         string
	healingLogs         string
	ctx                 context.Context
	provider            Calculator
	auditor             *Auditor
	snapshots           map[string]*Snapshot
	jobs                map[string]*Job
	cancel              context.CancelFunc
	active              string
	wg                  sync.WaitGroup
	workflows           map[string]Workflow
	workflowProblem     string
}

func (w *Workspace) SetHealingLogs(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.healingLogs = dir
}

func NewWorkspace(ctx context.Context, dir, client string, provider Calculator) (*Workspace, error) {
	if client != "retail" && client != "forever" {
		return nil, fmt.Errorf("unknown client")
	}
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0700); err != nil {
		return nil, err
	}
	w := &Workspace{client: client, dir: dir, ctx: ctx, provider: provider, auditor: NewAuditor(dir), snapshots: map[string]*Snapshot{}, jobs: map[string]*Job{}}
	w.loadWorkflows()
	files, _ := filepath.Glob(filepath.Join(dir, "snapshots", "*.json"))
	modTimes := map[string]time.Time{}
	for _, file := range files {
		if info, err := os.Stat(file); err == nil {
			modTimes[file] = info.ModTime()
		}
	}
	sort.Slice(files, func(i, j int) bool { return modTimes[files[i]].After(modTimes[files[j]]) })
	// Bounded startup work; pruning retains the most recent snapshots below.
	for _, file := range files[:min(len(files), 50)] {
		if data, err := readBounded(file, MaxSnapshot); err == nil {
			if s, err := DecodeSnapshot(data, client); err == nil {
				w.snapshots[s.ID] = s
				w.revision++
			}
		}
	}
	files, _ = filepath.Glob(filepath.Join(dir, "jobs", "*", "job.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, file := range files[:min(len(files), 50)] {
		data, err := readBounded(file, 2*1024*1024)
		if err != nil {
			continue
		}
		var job Job
		if json.Unmarshal(data, &job) != nil || !requestID.MatchString(job.ID) {
			continue
		}
		if job.State == "running" || job.State == "cancelling" {
			job.State = "interrupted"
			job.Error = "Engine stopped before this calculation completed."
			_ = writeJSON(file, &job)
		}
		w.jobs[job.ID] = &job
	}
	if err := w.recoverGameRequests(); err != nil {
		return nil, err
	}
	return w, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return atomicfile.Replace(name, path)
}

func (w *Workspace) Receive(s *Snapshot) (bool, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return false, err
	}
	s, err = DecodeSnapshot(data, w.client)
	if err != nil {
		return false, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.snapshots[s.ID]; exists {
		return false, nil
	}
	path := filepath.Join(w.dir, "snapshots", s.ID+".json")
	if _, err = os.Stat(path); err == nil {
		return false, nil
	}
	if err = writeJSON(path, s); err != nil {
		return false, err
	}
	w.snapshots[s.ID] = s
	w.revision++
	if len(w.snapshots) > 50 {
		var oldest *Snapshot
		for _, v := range w.snapshots {
			if oldest == nil || v.CapturedAt < oldest.CapturedAt {
				oldest = v
			}
		}
		delete(w.snapshots, oldest.ID)
	}
	return true, nil
}

func (w *Workspace) Snapshot(id string) (*Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found")
	}
	copy := *s
	return &copy, nil
}

func (w *Workspace) Start(id string, scenario Scenario, selected []string) (*Job, error) {
	return w.start(id, scenario, selected, false)
}
func (w *Workspace) start(id string, scenario Scenario, selected []string, fromGame bool) (*Job, error) {
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	if len(scenario.Encounters) > 0 && scenario.Task != "analysis" {
		return nil, fmt.Errorf("multiple encounters require an analysis")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active != "" {
		return nil, fmt.Errorf("a sim is already running; finish or cancel it first")
	}
	s := w.snapshots[id]
	if s == nil {
		return nil, fmt.Errorf("snapshot not found")
	}
	if s.Client == "retail" && s.Character.Role == "DAMAGER" && !builtinTask(scenario.Task) && scenario.FightStyle == "dungeon_slice" {
		scenario.Targets, scenario.Seconds, scenario.RaidBuffs = 1, 360, "none"
	}
	calculator := w.provider
	if builtinTask(scenario.Task) {
		calculator = &encounterProvider{task: scenario.Task, cache: filepath.Join(w.dir, "build-cache"), logs: w.healingLogs}
	}
	c := calculator.Capability(s, time.Now())
	if !c.Available {
		return nil, fmt.Errorf("%s", c.Reason)
	}
	var err error
	if s.Client != "forever" && !builtinTask(scenario.Task) {
		profile, parseErr := ParseProfile(s.Profile)
		if parseErr != nil {
			return nil, parseErr
		}
		profile, err = prepareComparisonProfile(s, profile, scenario, selected)
		if err != nil {
			return nil, err
		}
		if _, err = profile.Input(scenario, selected); err != nil {
			return nil, err
		}
	}
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	job := &Job{ID: fmt.Sprintf("%d_%s", time.Now().UnixMilli(), hex.EncodeToString(random[:])), SnapshotID: id, CreatedAt: time.Now().UTC(), State: "running", Scenario: scenario, Selected: append([]string{}, selected...), Audit: w.auditor.State()}
	job.SetupHash = setupHash(s)
	if request := s.GameRequest; fromGame && request != nil {
		job.GameTask, job.RequestID = request.Task, request.ID
		job.ID = gameJobID(s)
	}
	dir := filepath.Join(w.dir, "jobs", job.ID)
	if err = writeJSON(filepath.Join(dir, "job.json"), job); err != nil {
		return nil, err
	}
	if err = writeJSON(filepath.Join(dir, "snapshot.json"), s); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.cancel = cancel
	w.active = job.ID
	w.jobs[job.ID] = job
	w.revision++
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer cancel()
		result, err := calculator.Run(ctx, s, scenario, selected, dir)
		if err == nil && result != nil && scenario.Task == "upgrades" {
			attachUpgradeBuilds(ctx, result, s.Character.Spec, filepath.Join(w.dir, "build-cache"))
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		now := time.Now().UTC()
		job.FinishedAt = &now
		w.revision++
		if ctx.Err() != nil {
			job.State = "cancelled"
		} else if err != nil {
			job.State = "failed"
			job.Error = err.Error()
		} else {
			job.State = "complete"
			job.Result = result
			if job.RequestID != "" {
				labelGameTalents(s, result)
			}
			providerName := result.Provider
			if providerName == "" {
				providerName = "SimulationCraft"
			}
			warnings := job.Audit.ProviderWarnings(now, providerName)
			result.Warnings = append(result.Warnings, warnings...)
			if len(warnings) > 0 {
				result.Validation = "review-required"
			}
		}
		w.active = ""
		w.cancel = nil
		if err := writeJSON(filepath.Join(dir, "job.json"), job); err != nil {
			job.Error = "Could not save result: " + err.Error()
		}
	}()
	copy := *job
	return &copy, nil
}
func (w *Workspace) Cancel(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active != id || w.cancel == nil {
		return fmt.Errorf("this job is not running")
	}
	w.cancel()
	w.jobs[id].State = "cancelling"
	w.revision++
	return nil
}
func (w *Workspace) Close() {
	w.mu.Lock()
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Unlock()
	w.wg.Wait()
}

type Summary struct {
	ActiveTalents string        `json:"activeTalents,omitempty"`
	SetupHash     string        `json:"setupHash,omitempty"`
	LootPreviews  []LootPool    `json:"lootPreviews,omitempty"`
	ItemRules     int           `json:"itemRules"`
	LootPools     int           `json:"lootPools"`
	LootItems     int           `json:"lootItems"`
	Planning      PlanningData  `json:"planning"`
	Snapshot      Snapshot      `json:"snapshot"`
	Capability    Capability    `json:"capability"`
	Candidates    []Candidate   `json:"candidates"`
	Equipped      []Candidate   `json:"equipped"`
	Talents       []TalentBuild `json:"talents"`
}

var displayInventorySlots = map[int]string{1: "head", 2: "neck", 3: "shoulder", 4: "shirt", 5: "chest", 6: "waist", 7: "legs", 8: "feet", 9: "wrist", 10: "hands", 11: "finger1", 12: "finger2", 13: "trinket1", 14: "trinket2", 15: "back", 16: "main_hand", 17: "off_hand", 19: "tabard"}

func displayGear(profile *Profile) map[string]Candidate {
	gear := map[string]Candidate{}
	for _, line := range strings.Split(profile.Base, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && slots[key] && value != "" {
			gear[key] = Candidate{ID: "equipped_" + key, Source: "equipped", Slot: key, Value: value, Level: itemLevelFromValue(value), Label: key}
		}
	}
	return gear
}

func itemLinkLabel(link string) string {
	start := strings.Index(link, "|h[")
	if start < 0 {
		return ""
	}
	rest := link[start+3:]
	end := strings.Index(rest, "]|h")
	if end <= 0 || end > 200 {
		return ""
	}
	return rest[:end]
}

type WorkspaceState struct {
	Workflows     []Workflow `json:"workflows,omitempty"`
	WorkflowError string     `json:"workflowError,omitempty"`
	Client        string     `json:"client"`
	Snapshots     []Summary  `json:"snapshots"`
	Jobs          []Job      `json:"jobs"`
	Audit         Audit      `json:"audit"`
	Provider      Capability `json:"provider"`
	Reviews       []Review   `json:"reviews"`
}

func (w *Workspace) State() WorkspaceState {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := WorkspaceState{Client: w.client, Snapshots: []Summary{}, Jobs: []Job{}, Audit: w.auditor.State(), Provider: w.provider.Capability(nil, time.Now())}
	out.WorkflowError = w.workflowProblem
	for _, entry := range w.workflows {
		out.Workflows = append(out.Workflows, cloneJSON(entry))
	}
	sort.Slice(out.Workflows, func(i, j int) bool {
		return workflowKey(out.Workflows[i].CharacterGUID, out.Workflows[i].Spec, out.Workflows[i].Settings.Context) < workflowKey(out.Workflows[j].CharacterGUID, out.Workflows[j].Spec, out.Workflows[j].Settings.Context)
	})
	if w.client == "forever" {
		out.Provider = (&ForeverProvider{}).Capability(nil, time.Now())
	}
	for _, s := range w.snapshots {
		copy := *s
		copy.Profile = ""
		copy.Items = nil
		copy.Gear = nil
		copy.Loot = nil
		copy.Rules = nil
		summary := Summary{Snapshot: copy, Capability: w.provider.Capability(s, time.Now()), Candidates: []Candidate{}, Equipped: []Candidate{}}
		summary.SetupHash = setupHash(s)
		summary.ItemRules = len(s.Rules)
		if s.Loot != nil {
			summary.LootPools = len(s.Loot.Pools)
			for _, pool := range s.Loot.Pools {
				summary.LootItems += len(pool.Items)
				preview := pool
				preview.Items = nil
				summary.LootPreviews = append(summary.LootPreviews, preview)
			}
		}
		if p, err := ParseProfile(s.Profile); err == nil {
			for _, line := range strings.Split(p.Base, "\n") {
				if strings.HasPrefix(line, "talents=") {
					summary.ActiveTalents = strings.TrimPrefix(line, "talents=")
				}
			}
			summary.Candidates = p.Candidates
			rules := snapshotRuleIndex(s)
			upgradeByItem := map[string]ItemUpgrade{}
			for _, upgrade := range s.Upgrades {
				upgradeByItem[upgrade.Source+"\x00"+upgrade.Slot+"\x00"+upgrade.Value] = upgrade
			}
			for i := range summary.Candidates {
				candidate := &summary.Candidates[i]
				candidate.Level = itemLevelFromValue(candidate.Value)
				if rule, ok := rules[valueRuleKey(candidate.Value)]; ok {
					candidate.InventoryType = rule.InventoryType
				}
				if upgrade, ok := upgradeByItem[candidate.Source+"\x00"+candidate.Slot+"\x00"+candidate.Value]; ok {
					candidate.Level, candidate.NextLevel, candidate.MaxLevel = upgrade.Level, upgrade.NextLevel, upgrade.MaxLevel
					candidate.Track, candidate.CurrentRank, candidate.MaxRank = upgrade.Track, upgrade.CurrentRank, upgrade.MaxRank
					candidate.RankLevels = append([]int{}, upgrade.RankLevels...)
				}
			}
			links := map[string]string{}
			for _, item := range s.Items {
				if slot := displayInventorySlots[item.Slot]; slot != "" {
					links[slot] = item.Link
				}
			}
			for slot, candidate := range displayGear(p) {
				if rule, ok := rules[valueRuleKey(candidate.Value)]; ok {
					candidate.InventoryType = rule.InventoryType
				}
				if link := links[slot]; link != "" {
					candidate.Link = link
					if label := itemLinkLabel(link); label != "" {
						candidate.Label = label
					}
				}
				if upgrade, ok := upgradeByItem["equipped\x00"+slot+"\x00"+candidate.Value]; ok {
					candidate.Label, candidate.Level = upgrade.Label, upgrade.Level
					candidate.NextLevel, candidate.MaxLevel = upgrade.NextLevel, upgrade.MaxLevel
					candidate.Track, candidate.CurrentRank, candidate.MaxRank = upgrade.Track, upgrade.CurrentRank, upgrade.MaxRank
					candidate.RankLevels = append([]int{}, upgrade.RankLevels...)
				}
				summary.Equipped = append(summary.Equipped, candidate)
			}
			sort.Slice(summary.Equipped, func(i, j int) bool { return summary.Equipped[i].Slot < summary.Equipped[j].Slot })
			summary.Talents = p.Talents
			summary.Planning = p.Planning
		}
		if s.Client == "forever" {
			summary.Candidates = ForeverCandidates(s)
		}
		out.Snapshots = append(out.Snapshots, summary)
	}
	sort.Slice(out.Snapshots, func(i, j int) bool {
		return out.Snapshots[i].Snapshot.CapturedAt > out.Snapshots[j].Snapshot.CapturedAt
	})
	for _, j := range w.jobs {
		job := *j
		job.Result = upgradeDisplayResult(w.snapshots[j.SnapshotID], j.Result)
		out.Jobs = append(out.Jobs, job)
	}
	sort.Slice(out.Jobs, func(i, j int) bool { return out.Jobs[i].CreatedAt.After(out.Jobs[j].CreatedAt) })
	if len(out.Jobs) > 50 {
		out.Jobs = out.Jobs[:50]
	}
	files, _ := filepath.Glob(filepath.Join(w.dir, "reviews", "*.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, f := range files[:min(len(files), 50)] {
		if data, err := readBounded(f, 8192); err == nil {
			var r Review
			if json.Unmarshal(data, &r) == nil {
				out.Reviews = append(out.Reviews, r)
			}
		}
	}
	return out
}
