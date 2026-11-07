package simming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gameStateFixture() WorkspaceState {
	now := time.Unix(1800000000, 0).UTC()
	s := fixture(now)
	s.Profile = "raw profile must stay out of the bridge"
	summary := Summary{Snapshot: *s, Capability: Capability{ID: "simc", Available: true},
		Candidates: []Candidate{{ID: "item1", Label: "Necklace", Slot: "neck", Value: "id=123", Source: "owned"}},
		Talents:    []TalentBuild{{Label: "Captured build", Code: "ABC=="}}, LootPools: 16, LootItems: 86}
	result := &Result{Metric: "group_dps", Validation: "review-required", Baseline: Metric{Name: "Equipped", Mean: 100.25, MeanError: .125},
		Comparisons: []Metric{{Name: "item1", Mean: 102.5}}, Upgrades: []Upgrade{{ID: 123, Mean: 103, PoolComplete: false, PoolSize: 6}},
		TargetResults: []TargetResult{{Targets: 1, Baseline: Metric{Name: "Equipped", Mean: 100.25}, Comparisons: []Metric{{Name: "item1", CandidateID: "item1", Source: "owned", Slot: "neck", Mean: 102.5}}}},
		TargetSet:     &Metric{Name: "Best tested", Mean: 104}, ModelInputs: map[string]string{"allies": "Fixed"},
		Warnings: []string{"Model not reviewed"}}
	return WorkspaceState{Client: "retail", Snapshots: []Summary{summary}, Jobs: []Job{{ID: "job_1", SnapshotID: s.ID, CreatedAt: now, State: "complete", Result: result}}}
}

func TestGameResultsRetainCompleteDataAndEscapeLua(t *testing.T) {
	state := gameStateFixture()
	attack := "\"}; injected=true; --\n\x00123\\\r\t|cffff0000é猫|r\u2028"
	state.Jobs[0].Result.Warnings = append(state.Jobs[0].Result.Warnings, attack)
	data, err := BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(state.Snapshots[0].Snapshot.Profile)) || !bytes.Contains(data, []byte("\\000123")) || !bytes.Contains(data, []byte(`["poolSize"]=6`)) {
		t.Fatalf("raw profile leaked, escapes ambiguous, or loot boundary lost: %s", data)
	}
	// The project's Lua 5.1 harness executes the generated file with an empty
	// environment, then compares exact data. No Lua io/os APIs are available.
	python := filepath.Join("..", "..", "..", "..", ".venv", "bin", "python")
	if _, err := os.Stat(python); err != nil {
		t.Skip("project Lua 5.1 harness unavailable on this platform")
	}
	payload, _ := json.Marshal(map[string]string{"source": string(data), "expected": attack})
	script := `import json,sys
from lupa.lua51 import LuaRuntime
p=json.load(sys.stdin)
sys.path.insert(0,'../../../../Tools')
from sim_results_runtime import validate_results
parsed=validate_results(p['source'].encode(),'retail')
assert parsed['jobs'][0]['result']['warnings'][1]==p['expected']
lua=LuaRuntime()
load=lua.eval('function(s) local f,e=loadstring(s); assert(f,e); local env={}; setfenv(f,env); f(); return env end')
env=load(p['source'])
assert set(env.keys())=={'LamdaSimResults'}
r=env['LamdaSimResults']
assert r['schema']==1 and r['client']=='retail'
j=r['jobs'][1]
assert j['state']=='complete' and j['result']['metric']=='group_dps'
assert j['result']['warnings'][2]==p['expected']
assert j['result']['baseline']['mean']==100.25 and j['result']['baseline']['meanError']==0.125
assert j['result']['targetSet']['mean']==104 and j['result']['upgrades'][1]['poolSize']==6
assert j['result']['targetResults'][1]['targets']==1 and j['result']['targetResults'][1]['comparisons'][1]['source']=='owned'
assert not j['result']['upgrades'][1]['poolComplete']
assert r['snapshots'][1]['candidates'][1]['id']=='item1' and r['snapshots'][1]['talents'][1]['code']=='ABC=='
`
	cmd := exec.Command(python, "-c", script)
	cmd.Stdin = bytes.NewReader(payload)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Lua data roundtrip failed: %v\n%s", err, output)
	}
}

func TestGameResultsBoundHistoryAndRetainReferencedSnapshots(t *testing.T) {
	state := gameStateFixture()
	base := state.Snapshots[0]
	state.Snapshots, state.Jobs = nil, nil
	for group := 0; group < 9; group++ {
		for old := 0; old < 2; old++ {
			s := base
			s.Snapshot.ID = fmt.Sprintf("snapshot_%d_%d", group, old)
			s.Snapshot.Character.GUID = fmt.Sprintf("Player-%d", group)
			s.Snapshot.CapturedAt = 1800000000 - int64(group*100+old)
			state.Snapshots = append(state.Snapshots, s)
		}
		for job := 0; job < MaxGameJobsPerGroup+1; job++ {
			state.Jobs = append(state.Jobs, Job{ID: fmt.Sprintf("job_%d_%d", group, job), SnapshotID: fmt.Sprintf("snapshot_%d_1", group), CreatedAt: time.Unix(1800000000-int64(group*100+job), 0), State: "complete"})
		}
	}
	data, err := BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), `["state"]=`) != 8*MaxGameJobsPerGroup || strings.Contains(string(data), `snapshot_8_`) || !strings.Contains(string(data), `snapshot_0_0`) || !strings.Contains(string(data), `snapshot_0_1`) || !strings.Contains(string(data), fmt.Sprintf(`["omittedJobs"]=%d`, 9+MaxGameJobsPerGroup)) || !strings.Contains(string(data), `["omittedSnapshots"]=2`) {
		t.Fatal("bounded history omitted a referenced snapshot or reported wrong counts")
	}
}

func TestGameResultsRetainTasksAcrossRepeatedComparisonsAndActiveOldGroup(t *testing.T) {
	state := gameStateFixture()
	state.Jobs = nil
	for n := 0; n < 12; n++ {
		task := "compare"
		if n == 10 {
			task = "talents"
		} else if n == 11 {
			task = "upgrades"
		}
		state.Jobs = append(state.Jobs, Job{ID: fmt.Sprintf("job_%d", n), SnapshotID: state.Snapshots[0].Snapshot.ID, CreatedAt: time.Unix(1800000000-int64(n), 0), State: "complete", GameTask: task})
	}
	data, err := BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil || !bytes.Contains(data, []byte(`"job_10"`)) || !bytes.Contains(data, []byte(`"job_11"`)) || bytes.Contains(data, []byte(`"job_9"`)) {
		t.Fatalf("latest task results lost to repeated comparisons: %v", err)
	}
	old := state.Snapshots[0]
	old.Snapshot.ID = "old_running_snapshot"
	old.Snapshot.Character.GUID = "old-running-character"
	old.Snapshot.CapturedAt = 1600000000
	state.Snapshots = append(state.Snapshots, old)
	state.Jobs = append(state.Jobs, Job{ID: "active_old_job", SnapshotID: old.Snapshot.ID, CreatedAt: time.Unix(1800000000, 0), State: "running", GameTask: "upgrades"})
	for n := 0; n < 8; n++ {
		s := state.Snapshots[0]
		s.Snapshot.ID = fmt.Sprintf("other_%d", n)
		s.Snapshot.Character.GUID = fmt.Sprintf("other-character-%d", n)
		state.Snapshots = append(state.Snapshots, s)
	}
	data, err = BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil || !bytes.Contains(data, []byte(`"active_old_job"`)) || !bytes.Contains(data, []byte(`"old_running_snapshot"`)) {
		t.Fatalf("active calculation on older snapshot disappeared: %v", err)
	}
}

func TestGameResultsSizeDropsWholeOlderResults(t *testing.T) {
	state := gameStateFixture()
	state.Jobs = nil
	for n := 0; n < 3; n++ {
		warnings := make([]string, 32)
		for i := range warnings {
			warnings[i] = strings.Repeat("w", 30000)
		}
		state.Jobs = append(state.Jobs, Job{ID: fmt.Sprintf("job_%d", n), SnapshotID: state.Snapshots[0].Snapshot.ID, CreatedAt: time.Unix(1800000000-int64(n), 0), State: "complete", Result: &Result{Warnings: warnings}})
	}
	data, err := BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil || len(data) > MaxGameResultsBytes {
		t.Fatalf("failed bounded history export: %v", err)
	}
	if !bytes.Contains(data, []byte(`["omittedJobs"]=1`)) || bytes.Contains(data, []byte(`"job_2"`)) || bytes.Count(data, []byte(strings.Repeat("w", 30000))) != 64 {
		t.Fatal("results were sliced or wrong history entry dropped")
	}
	state.Jobs[0].Result.Warnings = append(state.Jobs[0].Result.Warnings, state.Jobs[0].Result.Warnings...)
	state.Jobs[0].Result.Warnings = append(state.Jobs[0].Result.Warnings, state.Jobs[0].Result.Warnings...)
	if _, err := BuildGameResults(state, time.Unix(1800000010, 0)); err == nil {
		t.Fatal("oversized newest result must fail instead of silently truncating it")
	}
}

func TestOptionalWorkflowDataCannotBlockTheMatchingJob(t *testing.T) {
	state := gameStateFixture()
	warnings := []string{}
	for i := 0; i < 52; i++ {
		warnings = append(warnings, strings.Repeat("w", 30000))
	}
	state.Jobs[0].Result.Warnings = warnings
	settings := WorkflowDraft{Context: "m+", Scenario: DefaultScenario(), CompareMode: "loadout"}
	for i := 0; i < 18; i++ {
		settings.Selected = append(settings.Selected, GameSelection{Source: "owned", Slot: "head", Value: strings.Repeat("v", 8192)})
	}
	for _, context := range []string{"raid", "m+"} {
		settings.Context = context
		state.Workflows = append(state.Workflows, Workflow{CharacterGUID: state.Snapshots[0].Snapshot.Character.GUID, Spec: state.Snapshots[0].Snapshot.Character.Spec, Revision: 1, Settings: settings})
	}
	// Lua quoting expands these values enough to exercise the actual wire cap.
	for i := range state.Workflows {
		for j := range state.Workflows[i].Settings.Selected {
			state.Workflows[i].Settings.Selected[j].Value = strings.Repeat("\\", 8192)
		}
	}
	data, err := BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil || !bytes.Contains(data, []byte(`["id"]="job_1"`)) || !bytes.Contains(data, []byte(`["omittedWorkflows"]=2`)) || !bytes.Contains(data, []byte(strings.Repeat("w", 30000))) {
		t.Fatalf("optional data blocked/damaged the job: %v", err)
	}
}

func TestCurrentDecisionsTakePriorityOverOlderInGameHistory(t *testing.T) {
	state := gameStateFixture()
	state.Snapshots[0].SetupHash = "same-current-build"
	state.Jobs[0].SetupHash = state.Snapshots[0].SetupHash
	state.Jobs[0].Result = &Result{Metric: "DPS", Baseline: Metric{Mean: 100}, Comparisons: []Metric{{Name: "upgrade", Mean: 110}}, Warnings: []string{}}
	for i := 0; i < 32; i++ {
		state.Jobs[0].Result.Warnings = append(state.Jobs[0].Result.Warnings, strings.Repeat("w", 30000))
	}
	old := state.Jobs[0]
	old.ID = "older_job"
	old.CreatedAt = old.CreatedAt.Add(-time.Hour)
	state.Jobs = append(state.Jobs, old)
	data, err := BuildGameResults(state, time.Unix(1800000010, 0))
	if err != nil || !bytes.Contains(data, []byte(`["decisions"]=`)) || bytes.Contains(data, []byte(`["omittedDecisions"]=`)) || !bytes.Contains(data, []byte(`["omittedJobs"]=1`)) {
		t.Fatalf("current decisions lost to old history: %v", err)
	}
}

func TestGameResultsRejectInvalidData(t *testing.T) {
	for name, change := range map[string]func(*WorkspaceState){
		"foreign snapshot": func(s *WorkspaceState) { s.Snapshots[0].Snapshot.Client = "forever" },
		"invalid client":   func(s *WorkspaceState) { s.Client = "../retail" },
		"duplicate job":    func(s *WorkspaceState) { s.Jobs = append(s.Jobs, s.Jobs[0]) },
		"nan metric":       func(s *WorkspaceState) { s.Jobs[0].Result.Baseline.Mean = math.NaN() },
		"infinite metric":  func(s *WorkspaceState) { s.Jobs[0].Result.Baseline.Mean = math.Inf(1) },
	} {
		t.Run(name, func(t *testing.T) {
			s := gameStateFixture()
			change(&s)
			if _, err := BuildGameResults(s, time.Unix(1800000010, 0)); err == nil {
				t.Fatal("invalid data accepted")
			}
		})
	}
}

func seedGameDestination(t *testing.T, client string) (string, string) {
	t.Helper()
	addons := t.TempDir()
	dir := filepath.Join(addons, "lamdaUI")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	toc := "## X-Lamda-SimResults: 1\n## X-Lamda-SimResults-Client: " + client + "\nSimResults.lua\nModules\\Sim.lua\n"
	if err := os.WriteFile(filepath.Join(dir, "lamdaUI.toc"), []byte(toc), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SimResults.lua")
	if err := os.WriteFile(path, []byte(GameResultsMarker+"\nLamdaSimResults = {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return addons, path
}

func TestGameResultsAtomicUpdatesAndNoClockOnlyRewrite(t *testing.T) {
	addons, path := seedGameDestination(t, "retail")
	state := gameStateFixture()
	at := time.Unix(1800000010, 0)
	if err := ExportGameResults(addons, state, at); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
	if err := ExportGameResults(addons, state, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || !info.ModTime().Equal(at) {
		t.Fatal("clock-only poll rewrote results")
	}
	// An intact content header must not hide damaged payload bytes on a poll.
	if err := os.WriteFile(path, append(append([]byte(nil), before...), []byte("damaged payload")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ExportGameResults(addons, state, at); err != nil {
		t.Fatal(err)
	}
	repaired, _ := os.ReadFile(path)
	if !bytes.Equal(before, repaired) {
		t.Fatal("unchanged state failed to repair a damaged managed payload")
	}
	state.Jobs[0].State, state.Jobs[0].Error = "failed", "Example failure"
	if err := ExportGameResults(addons, state, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if bytes.Equal(before, after) || !bytes.Contains(after, []byte("Example failure")) {
		t.Fatal("job status/error did not replace the old result")
	}
	state.Jobs[0].Result.Baseline.Mean = math.Inf(1)
	if err := ExportGameResults(addons, state, at); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(after, unchanged) {
		t.Fatal("failed write changed existing data")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".sim-results-*"))
	if len(files) != 0 {
		t.Fatal("temporary files leaked")
	}
}

func TestGameResultsRequireManagedClientOptIn(t *testing.T) {
	for _, mode := range []string{"foreign", "no optin", "late file", "unmanaged", "missing"} {
		t.Run(mode, func(t *testing.T) {
			addons, path := seedGameDestination(t, "retail")
			tocPath := filepath.Join(filepath.Dir(path), "lamdaUI.toc")
			toc, _ := os.ReadFile(tocPath)
			switch mode {
			case "foreign":
				toc = bytes.ReplaceAll(toc, []byte("Client: retail"), []byte("Client: forever"))
			case "no optin":
				toc = bytes.ReplaceAll(toc, []byte("SimResults: 1"), []byte("SimResults: 0"))
			case "late file":
				toc = bytes.ReplaceAll(toc, []byte("SimResults.lua\nModules\\Sim.lua"), []byte("Modules\\Sim.lua\nSimResults.lua"))
			case "unmanaged":
				os.WriteFile(path, []byte("-- User file\n"), 0600)
			case "missing":
				os.Remove(path)
			}
			os.WriteFile(tocPath, toc, 0600)
			before, _ := os.ReadFile(path)
			if err := ExportGameResults(addons, gameStateFixture(), time.Unix(1800000010, 0)); err == nil {
				t.Fatal("unmanaged destination accepted")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("refused file was changed")
			}
		})
	}
}
