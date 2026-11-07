// Package heallogs learns bounded, anonymous encounter workloads from local
// advanced logs. It never reads game memory, sends data, or drives gameplay.
package heallogs

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/healsim"
)

const parserVersion = 1

type Audit struct {
	Files      int            `json:"files"`
	Cached     int            `json:"cached"`
	Accepted   int            `json:"accepted"`
	Duplicates int            `json:"duplicates"`
	Rejected   map[string]int `json:"rejected"`
	Cohorts    map[string]int `json:"cohorts"`
}
type fileCache struct {
	Version  int                       `json:"version"`
	Size     int64                     `json:"size"`
	Modified int64                     `json:"modified"`
	Samples  []healsim.EncounterSample `json:"samples"`
	Rejected map[string]int            `json:"rejected"`
}
type hit struct {
	at     float64
	target string
	amount float64
}
type fight struct {
	start                            time.Time
	id, name, difficulty, patch, key string
	size                             int
	players                          map[string]int
	health                           map[string][]float64
	hits                             []hit
	deaths                           map[string]bool
	invalid                          string
}

func role(spec int) string {
	switch spec {
	case 65, 105, 256, 257, 264, 270, 1468:
		return "healer"
	case 66, 73, 104, 250, 268, 581:
		return "tank"
	default:
		return "damage"
	}
}
func numeric(s string) (float64, error) {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || !(n >= 0 && n <= 1e10) {
		return 0, fmt.Errorf("invalid numeric damage/resource field")
	}
	return n, nil
}
func (f *fight) finish(at time.Time, source string) (healsim.EncounterSample, error) {
	if f.invalid != "" {
		return healsim.EncounterSample{}, fmt.Errorf("%s", f.invalid)
	}
	if len(f.players) != f.size {
		return healsim.EncounterSample{}, fmt.Errorf("incomplete roster")
	}
	var healers, others []string
	for guid, spec := range f.players {
		if f.deaths[guid] {
			return healsim.EncounterSample{}, fmt.Errorf("player death truncates future damage")
		}
		if role(spec) == "healer" {
			healers = append(healers, guid)
		} else {
			others = append(others, guid)
		}
	}
	if len(healers) == 0 {
		return healsim.EncounterSample{}, fmt.Errorf("no recorded healer")
	}
	sort.Strings(healers)
	sort.Slice(others, func(i, j int) bool {
		a, b := role(f.players[others[i]]), role(f.players[others[j]])
		if a != b {
			return a == "tank"
		}
		return others[i] < others[j]
	})
	order := append(healers[:1:1], others...)
	order = append(order, healers[1:]...)
	indices := map[string]int{}
	roles := []string{}
	r := healsim.Replay{Schema: 1, Source: "local:" + source, Encounter: f.name, Difficulty: f.difficulty + f.key, Patch: f.patch, Seconds: at.Sub(f.start).Seconds()}
	for i, guid := range order {
		values := f.health[guid]
		if len(values) == 0 {
			return healsim.EncounterSample{}, fmt.Errorf("missing advanced max health")
		}
		sort.Float64s(values)
		med := values[len(values)/2]
		if len(values)%2 == 0 {
			med = (med + values[len(values)/2-1]) / 2
		}
		label := fmt.Sprintf("%s %d", role(f.players[guid]), i+1)
		if i == 0 {
			label = "You"
		}
		r.Members = append(r.Members, healsim.ReplayMember{Label: label, MaxHealth: med})
		indices[guid] = i
		roles = append(roles, role(f.players[guid]))
	}
	for _, h := range f.hits {
		if target, ok := indices[h.target]; ok {
			r.Events = append(r.Events, healsim.DamageEvent{Time: h.at, Target: target, Amount: h.amount})
		}
	}
	sort.SliceStable(r.Events, func(i, j int) bool { return r.Events[i].Time < r.Events[j].Time })
	r.Warnings = []string{"Recorded post-mitigation health damage: existing absorbs and defensives remain baked in. Simulated shields protect that remaining workload; original mitigation is not reconstructed.", "Median recorded maximum health; temporary max-health changes, movement, self-healing and co-healing are omitted. One recorded healer slot is replaced with the captured character.", "Training uses completed kills without recorded player deaths. This selects successful pulls and cannot estimate all wipe patterns. Logs identify the patch, not server hotfix state."}
	if err := r.Validate(); err != nil {
		return healsim.EncounterSample{}, err
	}
	identity := fmt.Sprintf("%s|%s|%s", f.start.UTC().Format(time.RFC3339Nano), f.id, strings.Join(order, ","))
	cohort := fmt.Sprintf("%s/%s/%s%s/%s", f.patch, f.id, f.difficulty, f.key, strings.Join(roles, ","))
	return healsim.EncounterSample{ID: fmt.Sprintf("%x", sha256.Sum256([]byte(identity))), Cohort: cohort, RecordedAt: f.start.UnixMilli(), Replay: r}, nil
}

// Parse ignores unfinished tails. It rejects an entire affected fight rather
// than quietly dropping an unreadable damage event and lowering the workload.
func Parse(ctx context.Context, input io.Reader, source string) ([]healsim.EncounterSample, map[string]int, error) {
	var current *fight
	var samples []healsim.EncounterSample
	rejected := map[string]int{}
	patch, key := "", ""
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 65536), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
		}
		line := scanner.Text()
		_, payload, ok := strings.Cut(line, "  ")
		if !ok {
			if current != nil {
				current.invalid = "unreadable encounter record"
			}
			continue
		}
		kind, _, _ := strings.Cut(payload, ",")
		meta := kind == "COMBAT_LOG_VERSION" || kind == "CHALLENGE_MODE_START" || kind == "CHALLENGE_MODE_END" || kind == "ZONE_CHANGE" || kind == "ENCOUNTER_START" || kind == "ENCOUNTER_END"
		if !meta && current == nil {
			continue
		}
		base := -1
		switch kind {
		case "SPELL_DAMAGE", "SPELL_PERIODIC_DAMAGE", "RANGE_DAMAGE", "DAMAGE_SHIELD", "DAMAGE_SPLIT", "SPELL_HEAL", "SPELL_PERIODIC_HEAL":
			base = 12
		case "SWING_DAMAGE":
			base = 9
		case "ENVIRONMENTAL_DAMAGE":
			base = 10
		}
		if !meta && base < 0 && kind != "COMBATANT_INFO" && kind != "UNIT_DIED" && kind != "UNIT_DESTROYED" {
			continue
		}
		at, f, err := engine.RecordFields(line)
		if err != nil {
			if current != nil {
				current.invalid = "unreadable encounter record"
			}
			continue
		}
		switch kind {
		case "COMBAT_LOG_VERSION":
			if current != nil {
				rejected["log reset during fight"]++
				current = nil
			}
			patch = ""
			key = ""
			if len(f) >= 8 && f[1] == "22" && f[2] == "ADVANCED_LOG_ENABLED" && f[3] == "1" && f[4] == "BUILD_VERSION" && strings.HasPrefix(f[5], "12.1.") {
				patch = f[5]
			}
		case "CHALLENGE_MODE_START":
			key = "unknown"
			if len(f) >= 6 {
				if level, e := strconv.Atoi(f[4]); e == nil && level > 0 && level <= 100 {
					key = fmt.Sprintf("/key%d/affixes%s", level, strings.Join(f[5:], ","))
				}
			}
		case "CHALLENGE_MODE_END":
			key = ""
		case "ZONE_CHANGE":
			if len(f) > 3 && f[3] != "8" {
				key = ""
			}
		case "ENCOUNTER_START":
			if current != nil {
				rejected["unfinished fight"]++
			}
			current = nil
			if len(f) < 5 || patch == "" {
				rejected["unsupported patch or basic logging"]++
				continue
			}
			size, e := strconv.Atoi(f[4])
			if e != nil || size < 2 || size > 40 {
				rejected["unsupported group size"]++
				continue
			}
			current = &fight{start: at, id: f[1], name: f[2], difficulty: f[3], size: size, patch: patch, players: map[string]int{}, health: map[string][]float64{}, deaths: map[string]bool{}}
			if f[3] == "8" {
				current.key = key
				if key == "" || key == "unknown" {
					current.invalid = "unknown keystone level or affixes"
				}
			}
		case "ENCOUNTER_END":
			if current == nil {
				continue
			}
			if len(f) < 6 || f[1] != current.id || f[5] != "1" {
				rejected["incomplete or failed encounter"]++
				current = nil
				continue
			}
			sample, e := current.finish(at, source)
			if e != nil {
				rejected[e.Error()]++
			} else {
				samples = append(samples, sample)
			}
			current = nil
		case "COMBATANT_INFO":
			if current != nil && len(f) > 25 && strings.HasPrefix(f[1], "Player-") {
				spec, e := strconv.Atoi(f[25])
				if e != nil || spec <= 0 {
					current.invalid = "invalid combatant specialization"
				} else {
					current.players[f[1]] = spec
				}
			}
		case "UNIT_DIED", "UNIT_DESTROYED":
			if current != nil && len(f) > 5 {
				current.deaths[f[5]] = true
			}
		default:
			if current == nil || base < 0 || len(f) <= 8 {
				continue
			}
			if len(f) <= base+19 {
				current.invalid = "missing advanced resource payload"
				continue
			}
			if strings.HasPrefix(f[base], "Player-") {
				maximum, e := numeric(f[base+3])
				if e != nil {
					current.invalid = "invalid maximum health"
				} else if maximum > 0 && maximum <= 1e9 && len(current.health[f[base]]) < 2048 {
					current.health[f[base]] = append(current.health[f[base]], maximum)
				}
			}
			if kind == "SPELL_HEAL" || kind == "SPELL_PERIODIC_HEAL" || !strings.HasPrefix(f[5], "Player-") {
				continue
			}
			if len(f) < base+29 || !(strings.HasPrefix(f[base], "Player-") || strings.HasPrefix(f[base], "Creature-") || strings.HasPrefix(f[base], "Pet-") || strings.HasPrefix(f[base], "Vehicle-") || f[base] == "0000000000000000") {
				current.invalid = "unsupported advanced damage layout"
				continue
			}
			amount, e := numeric(f[base+19])
			if e != nil {
				current.invalid = "invalid damage amount"
				continue
			}
			if len(current.hits) >= 100000 {
				current.invalid = "encounter exceeds event limit"
				continue
			}
			if amount > 0 {
				current.hits = append(current.hits, hit{at.Sub(current.start).Seconds(), f[5], amount})
			}
		}
	}
	if current != nil {
		rejected["unfinished fight"]++
	}
	return samples, rejected, scanner.Err()
}

// Learn is user-triggered by a simulation request. At most 24 recent files and
// 4 GiB are read, with per-file caches; no frame-loop or startup scan is added.
func Learn(ctx context.Context, dir, cache, selected string) (*healsim.EncounterTraining, []healsim.TrainingSummary, Audit, error) {
	audit := Audit{Rejected: map[string]int{}, Cohorts: map[string]int{}}
	paths, err := filepath.Glob(filepath.Join(dir, "WoWCombatLog*.txt"))
	if err != nil {
		return nil, nil, audit, err
	}
	type file struct {
		path string
		info os.FileInfo
	}
	var files []file
	for _, path := range paths {
		if info, e := os.Stat(path); e == nil && info.Mode().IsRegular() {
			files = append(files, file{path, info})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].info.ModTime().After(files[j].info.ModTime()) })
	if err = os.MkdirAll(cache, 0700); err != nil {
		return nil, nil, audit, err
	}
	groups := map[string][]healsim.EncounterSample{}
	seen := map[string]bool{}
	var bytes int64
	for i, file := range files {
		if err = ctx.Err(); err != nil {
			return nil, nil, audit, err
		}
		if i >= 24 || bytes+file.info.Size() > 4<<30 {
			audit.Rejected["scan budget: older files not scanned"] += len(files) - i
			break
		}
		bytes += file.info.Size()
		audit.Files++
		name := fmt.Sprintf("%x.json", sha256.Sum256([]byte(file.path)))
		cached := fileCache{}
		if b, e := os.ReadFile(filepath.Join(cache, name)); e == nil && len(b) <= 32<<20 {
			_ = json.Unmarshal(b, &cached)
		}
		if cached.Version == parserVersion && cached.Size == file.info.Size() && cached.Modified == file.info.ModTime().UnixNano() {
			audit.Cached++
		} else {
			f, e := os.Open(file.path)
			if e != nil {
				return nil, nil, audit, e
			}
			samples, rejected, e := Parse(ctx, io.LimitReader(f, file.info.Size()), filepath.Base(file.path))
			f.Close()
			if e != nil {
				return nil, nil, audit, e
			}
			cached = fileCache{Version: parserVersion, Size: file.info.Size(), Modified: file.info.ModTime().UnixNano(), Samples: samples, Rejected: rejected}
			b, e := json.Marshal(cached)
			if e != nil {
				return nil, nil, audit, e
			}
			if len(b) > 32<<20 {
				return nil, nil, audit, fmt.Errorf("normalized file exceeds cache limit")
			}
			pending := filepath.Join(cache, name+".pending")
			if e = os.WriteFile(pending, b, 0600); e == nil {
				e = os.Rename(pending, filepath.Join(cache, name))
			}
			if e != nil {
				return nil, nil, audit, e
			}
		}
		for reason, n := range cached.Rejected {
			audit.Rejected[reason] += n
		}
		for _, s := range cached.Samples {
			if seen[s.ID] {
				audit.Duplicates++
				continue
			}
			seen[s.ID] = true
			groups[s.Cohort] = append(groups[s.Cohort], s)
			audit.Accepted++
		}
	}
	var trained []*healsim.EncounterTraining
	for cohort, samples := range groups {
		audit.Cohorts[cohort] = len(samples)
		if len(samples) < 3 {
			continue
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i].RecordedAt > samples[j].RecordedAt })
		samples = samples[:min(100, len(samples))]
		t, e := healsim.Train(samples)
		if e != nil {
			return nil, nil, audit, e
		}
		trained = append(trained, t)
	}
	sort.Slice(trained, func(i, j int) bool {
		if trained[i].Holdout.RecordedAt == trained[j].Holdout.RecordedAt {
			return trained[i].Summary.Cohort < trained[j].Summary.Cohort
		}
		return trained[i].Holdout.RecordedAt > trained[j].Holdout.RecordedAt
	})
	var chosen *healsim.EncounterTraining
	var summaries []healsim.TrainingSummary
	for _, t := range trained {
		summaries = append(summaries, t.Summary)
		if chosen == nil && (selected == "" || selected == t.Summary.Cohort) {
			chosen = t
		}
	}
	if chosen == nil {
		return nil, summaries, audit, fmt.Errorf("found %d complete usable fights, but need three of the same encounter, difficulty, key level and roles for the selected training cohort; see damage-training-audit.json", audit.Accepted)
	}
	return chosen, summaries, audit, nil
}
