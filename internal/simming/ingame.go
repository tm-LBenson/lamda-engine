package simming

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/atomicfile"
)

const (
	GameResultsMarker   = "-- LamdaUI managed sim results v1"
	MaxGameResultsBytes = 2 * 1024 * 1024
	MaxGameGroups       = 8
	MaxGameJobsPerGroup = 10
	maxGameStringBytes  = 64 * 1024
)

var errGameResultsSize = errors.New("in-game results exceed the file size limit")

type gameResults struct {
	Schema           int              `json:"schema"`
	Client           string           `json:"client"`
	GeneratedAt      int64            `json:"generatedAt"`
	Snapshots        []Summary        `json:"snapshots"`
	Jobs             []Job            `json:"jobs"`
	OmittedSnapshots int              `json:"omittedSnapshots"`
	OmittedJobs      int              `json:"omittedJobs"`
	OmittedWorkflows int              `json:"omittedWorkflows,omitempty"`
	OmittedDecisions int              `json:"omittedDecisions,omitempty"`
	Workflows        []Workflow       `json:"workflows,omitempty"`
	Decisions        []DecisionReport `json:"decisions,omitempty"`
}

func gameGroup(s Summary) string {
	return s.Snapshot.Character.GUID + ":" + strconv.Itoa(s.Snapshot.Character.Spec)
}

// BuildGameResults exports ordinary data literals, never simulator or user Lua.
// Each included result is complete. History is bounded by whole jobs and
// character/spec groups; omission counts make that boundary explicit in game.
func BuildGameResults(state WorkspaceState, now time.Time) ([]byte, error) {
	return buildGameResults(state, now, "")
}

func buildGameResults(state WorkspaceState, now time.Time, requiredRequest string) ([]byte, error) {
	if (state.Client != "retail" && state.Client != "forever") || now.Unix() <= 0 {
		return nil, fmt.Errorf("invalid in-game results client or timestamp")
	}
	if len(state.Snapshots) > 512 || len(state.Jobs) > 512 {
		return nil, fmt.Errorf("in-game results input is unbounded")
	}
	snapshots := append([]Summary(nil), state.Snapshots...)
	sort.SliceStable(snapshots, func(i, j int) bool {
		if snapshots[i].Snapshot.CapturedAt == snapshots[j].Snapshot.CapturedAt {
			return snapshots[i].Snapshot.ID > snapshots[j].Snapshot.ID
		}
		return snapshots[i].Snapshot.CapturedAt > snapshots[j].Snapshot.CapturedAt
	})
	byID, newest, groups := map[string]Summary{}, map[string]string{}, []string{}
	for _, summary := range snapshots {
		s := summary.Snapshot
		if s.Client != state.Client || !requestID.MatchString(s.ID) || s.Character.GUID == "" || len(s.Character.GUID) > 128 || s.Character.Spec < 0 || s.CapturedAt <= 0 {
			return nil, fmt.Errorf("invalid snapshot identity in in-game results")
		}
		if _, exists := byID[s.ID]; exists {
			return nil, fmt.Errorf("repeated snapshot in in-game results")
		}
		// Even callers other than Workspace.State must not export the raw profile
		// or duplicate the full inventory/catalog alongside comparison results.
		summary.Snapshot.Profile = ""
		summary.Snapshot.Items = nil
		summary.Snapshot.Gear = nil
		summary.Snapshot.Rules = nil
		summary.Snapshot.Loot = nil
		byID[s.ID] = summary
		group := gameGroup(summary)
		if newest[group] == "" && len(groups) < MaxGameGroups {
			newest[group] = s.ID
			groups = append(groups, group)
		}
	}
	jobs := append([]Job(nil), state.Jobs...)
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].ID > jobs[j].ID
		}
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	// Keep a currently running calculation visible even when it belongs to an
	// older character capture. Otherwise groups are ordered by latest capture.
	for _, job := range jobs {
		if job.State != "running" && job.State != "queued" {
			continue
		}
		if summary, ok := byID[job.SnapshotID]; ok {
			group := gameGroup(summary)
			if newest[group] == "" {
				for _, candidate := range snapshots {
					if gameGroup(candidate) == group {
						newest[group] = candidate.Snapshot.ID
						break
					}
				}
				groups = append([]string{group}, groups...)
				if len(groups) > MaxGameGroups {
					delete(newest, groups[len(groups)-1])
					groups = groups[:MaxGameGroups]
				}
			}
		}
	}
	selected, perGroup, seenJobs := []Job{}, map[string]int{}, map[string]bool{}
	for _, job := range jobs {
		if !requestID.MatchString(job.ID) || seenJobs[job.ID] {
			return nil, fmt.Errorf("invalid or repeated job in in-game results")
		}
		seenJobs[job.ID] = true
	}
	chosen, latestTask := map[string]bool{}, map[string]bool{}
	add := func(job Job) {
		summary, exists := byID[job.SnapshotID]
		if !exists || newest[gameGroup(summary)] == "" || perGroup[gameGroup(summary)] >= MaxGameJobsPerGroup || chosen[job.ID] {
			return
		}
		selected = append(selected, job)
		chosen[job.ID] = true
		perGroup[gameGroup(summary)]++
	}
	// Reserve the newest job of each task before filling with recent history;
	// repeated gear comparisons must not hide the last talents or loot result.
	for _, job := range jobs {
		if job.State == "running" || job.State == "queued" {
			add(job)
		}
	}
	for _, job := range jobs {
		summary, exists := byID[job.SnapshotID]
		if !exists {
			continue
		}
		key := gameGroup(summary) + ":" + gameJobTask(job)
		if !latestTask[key] {
			add(job)
			latestTask[key] = true
		}
	}
	for _, job := range jobs {
		add(job)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].CreatedAt.Equal(selected[j].CreatedAt) {
			return selected[i].ID > selected[j].ID
		}
		return selected[i].CreatedAt.After(selected[j].CreatedAt)
	})
	includeWorkflows, includeDecisions := true, true
	decisionReports := map[string]DecisionReport{}
	for _, id := range newest {
		if report, err := buildDecisions(state, id, ""); err == nil && len(report.Views) > 0 {
			decisionReports[id] = *report
		}
	}
	makeEnvelope := func() gameResults {
		included := map[string]bool{}
		for _, group := range groups {
			included[newest[group]] = true
		}
		for _, job := range selected {
			included[job.SnapshotID] = true
		}
		summaries := []Summary{}
		for _, s := range snapshots {
			if included[s.Snapshot.ID] {
				summaries = append(summaries, byID[s.Snapshot.ID])
			}
		}
		envelope := gameResults{Schema: 1, Client: state.Client, GeneratedAt: now.Unix(), Snapshots: summaries, Jobs: selected,
			OmittedSnapshots: len(state.Snapshots) - len(summaries), OmittedJobs: len(state.Jobs) - len(selected)}
		for _, entry := range state.Workflows {
			if newest[entry.CharacterGUID+":"+strconv.Itoa(entry.Spec)] != "" {
				if includeWorkflows {
					envelope.Workflows = append(envelope.Workflows, entry)
				} else {
					envelope.OmittedWorkflows++
				}
			}
		}
		for _, id := range newest {
			if report, ok := decisionReports[id]; ok {
				if includeDecisions {
					envelope.Decisions = append(envelope.Decisions, report)
				} else {
					envelope.OmittedDecisions++
				}
			}
		}
		sort.Slice(envelope.Decisions, func(i, j int) bool { return envelope.Decisions[i].SnapshotID < envelope.Decisions[j].SnapshotID })
		return envelope
	}
	for {
		envelope := makeEnvelope()
		if requiredRequest != "" {
			present := false
			for _, job := range envelope.Jobs {
				if job.RequestID == requiredRequest && job.SnapshotID == requiredRequest {
					present = true
					break
				}
			}
			if !present {
				return nil, fmt.Errorf("reload request omitted by in-game result history bounds")
			}
		}
		data, err := marshalGameResults(envelope)
		if !errors.Is(err, errGameResultsSize) {
			return data, err
		}
		// Current decisions and choices take precedence over older in-game
		// history. Complete job history remains on disk in the companion.
		removed := false
		for i := len(selected) - 1; i >= 0; i-- {
			group := gameGroup(byID[selected[i].SnapshotID])
			if perGroup[group] > 1 {
				selected = append(selected[:i], selected[i+1:]...)
				perGroup[group]--
				removed = true
				break
			}
		}
		if removed {
			continue
		}
		if len(groups) <= 1 {
			// Extras must still never block the newest complete matching job.
			if includeDecisions && len(envelope.Decisions) > 0 {
				includeDecisions = false
				continue
			}
			if includeWorkflows && len(envelope.Workflows) > 0 {
				includeWorkflows = false
				continue
			}
			return nil, err // Never replace the newest result with a partial result.
		}
		oldest := groups[len(groups)-1]
		groups = groups[:len(groups)-1]
		delete(newest, oldest)
		keep := selected[:0]
		for _, job := range selected {
			if gameGroup(byID[job.SnapshotID]) != oldest {
				keep = append(keep, job)
			}
		}
		selected = keep
	}
}

// Older browser jobs predate gameTask; infer their displayed result category.
func gameJobTask(job Job) string {
	if job.GameTask != "" {
		return job.GameTask
	}
	if builtinTask(job.Scenario.Task) {
		return job.Scenario.Task
	}
	if job.Plan != nil {
		return "plan"
	}
	if job.Scenario.Task == "upgrades" || (job.Result != nil && len(job.Result.Upgrades) > 0) {
		return "upgrades"
	}
	if len(job.Scenario.Talents) > 0 {
		return "talents"
	}
	if len(job.Selected) > 0 || (job.Result != nil && len(job.Result.Comparisons) > 0) {
		return "compare"
	}
	return "baseline"
}

func marshalGameResults(value gameResults) ([]byte, error) {
	// A digest excludes the clock so lifecycle polling does not rewrite identical
	// results. generatedAt remains the timestamp of the last actual data change.
	stamp := value.GeneratedAt
	value.GeneratedAt = 0
	identity, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("invalid in-game result data: %w", err)
	}
	if len(identity) > MaxGameResultsBytes {
		return nil, errGameResultsSize
	}
	digest := sha256.Sum256(identity)
	value.GeneratedAt = stamp
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var tree any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&tree); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString("LamdaSimResults = ")
	nodes := 0
	if err = writeLuaData(&out, tree, 0, &nodes); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	payloadDigest := sha256.Sum256(out.Bytes())
	header := GameResultsMarker + "\n-- Content digest: " + hex.EncodeToString(digest[:]) +
		"\n-- Payload digest: " + hex.EncodeToString(payloadDigest[:]) + "\n"
	if len(header)+out.Len() > MaxGameResultsBytes {
		return nil, errGameResultsSize
	}
	return append([]byte(header), out.Bytes()...), nil
}

// JSON types form a deliberately small grammar. Decimal byte escapes always
// have three digits, so a following digit cannot extend an escape (Lua 5.1).
func writeLuaString(out *bytes.Buffer, text string) error {
	if len(text) > maxGameStringBytes {
		return fmt.Errorf("in-game result string exceeds the limit")
	}
	out.WriteByte('"')
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '"' || c == '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		case c < 32 || c == 127:
			fmt.Fprintf(out, "\\%03d", c)
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte('"')
	return nil
}

func writeLuaData(out *bytes.Buffer, value any, depth int, nodes *int) error {
	*nodes++
	if out.Len() > MaxGameResultsBytes || *nodes > 200000 {
		return errGameResultsSize
	}
	if depth > 32 {
		return fmt.Errorf("in-game result nesting exceeds the limit")
	}
	switch v := value.(type) {
	case nil:
		out.WriteString("nil")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case string:
		return writeLuaString(out, v)
	case json.Number:
		n, err := strconv.ParseFloat(string(v), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("in-game result contains an invalid number")
		}
		out.WriteString(string(v)) // json.Marshal already produced valid decimal syntax.
	case []any:
		out.WriteString("{\n")
		for _, item := range v {
			if err := writeLuaData(out, item, depth+1, nodes); err != nil {
				return err
			}
			out.WriteString(",\n")
		}
		out.WriteByte('}')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteString("{\n")
		for _, key := range keys {
			out.WriteByte('[')
			if err := writeLuaString(out, key); err != nil {
				return err
			}
			out.WriteString("]=")
			if err := writeLuaData(out, v[key], depth+1, nodes); err != nil {
				return err
			}
			out.WriteString(",\n")
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("unsupported in-game data type")
	}
	return nil
}

func checkGamePath(path string) error {
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("in-game results destination is linked")
		}
		if filepath.Dir(p) == p {
			return nil
		}
	}
}

func gameDestination(addonsDir, client string) (string, []byte, []byte, error) {
	if !filepath.IsAbs(addonsDir) {
		return "", nil, nil, fmt.Errorf("in-game results require an absolute AddOns directory")
	}
	folder := filepath.Join(addonsDir, "lamdaUI")
	tocPath, destination := filepath.Join(folder, "lamdaUI.toc"), filepath.Join(folder, "SimResults.lua")
	for _, path := range []string{tocPath, destination} {
		if err := checkGamePath(path); err != nil {
			return "", nil, nil, fmt.Errorf("in-game results not enabled: %w", err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, nil, fmt.Errorf("in-game results require ordinary managed files")
		}
	}
	toc, err := readBounded(tocPath, 64*1024)
	if err != nil {
		return "", nil, nil, err
	}
	enabled, matching, dataIndex, moduleIndex, dataCount, clientCount := false, false, -1, -1, 0, 0
	for i, raw := range strings.Split(string(toc), "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if line == "## X-Lamda-SimResults: 1" {
			enabled = true
		}
		if line == "## X-Lamda-SimResults-Client: "+client {
			matching = true
		}
		if strings.HasPrefix(line, "## X-Lamda-SimResults-Client:") {
			clientCount++
		}
		line = strings.ReplaceAll(line, "\\", "/")
		if line == "SimResults.lua" {
			dataIndex, dataCount = i, dataCount+1
		}
		if line == "Modules/Sim.lua" {
			moduleIndex = i
		}
	}
	if !enabled || !matching || clientCount != 1 || dataCount != 1 || dataIndex < 0 || moduleIndex <= dataIndex {
		return "", nil, nil, fmt.Errorf("addon has not opted into this client's in-game results")
	}
	previous, err := readBounded(destination, MaxGameResultsBytes)
	if err != nil {
		return "", nil, nil, err
	}
	if !bytes.HasPrefix(previous, []byte(GameResultsMarker+"\n")) && !bytes.HasPrefix(previous, []byte(GameResultsMarker+"\r\n")) {
		return "", nil, nil, fmt.Errorf("refusing to replace an unmanaged results file")
	}
	return destination, previous, toc, nil
}

// ExportGameResults writes only the explicitly opted-in generated addon file.
// Never remove the old file first. A brief Windows sharing violation is retried
// while the complete prior file remains available to WoW for its next reload.
func ExportGameResults(addonsDir string, state WorkspaceState, now time.Time) error {
	return exportGameResults(addonsDir, state, now, "")
}

func exportGameResults(addonsDir string, state WorkspaceState, now time.Time, requiredRequest string) error {
	data, err := buildGameResults(state, now, requiredRequest)
	if err != nil {
		return err
	}
	return writeGameResults(addonsDir, state.Client, data)
}

func writeGameResults(addonsDir, client string, data []byte) error {
	path, previous, toc, err := gameDestination(addonsDir, client)
	if err != nil {
		return err
	}
	line := func(data []byte) string {
		parts := bytes.SplitN(data, []byte("\n"), 3)
		if len(parts) < 2 {
			return ""
		}
		return string(bytes.TrimSpace(parts[1]))
	}
	parts := bytes.SplitN(previous, []byte("\n"), 4)
	payloadValid := false
	if len(parts) == 4 {
		checksum := sha256.Sum256(parts[3])
		payloadValid = string(parts[2]) == "-- Payload digest: "+hex.EncodeToString(checksum[:])
	}
	if payloadValid && line(previous) == line(data) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sim-results-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	_, current, currentTOC, err := gameDestination(addonsDir, client)
	if err != nil {
		return err
	}
	if !bytes.Equal(previous, current) || !bytes.Equal(toc, currentTOC) {
		return fmt.Errorf("in-game results destination changed while writing")
	}
	return atomicfile.Replace(name, path)
}
