package simming

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/healsim"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ProviderConfig struct {
	Path       string    `json:"path"`
	Version    string    `json:"version"`
	SHA256     string    `json:"sha256"`
	Source     string    `json:"source"`
	Builds     []string  `json:"builds"`
	ReviewedAt time.Time `json:"reviewedAt"`
	Specs      []int     `json:"specs"`
}

// Calculator implementations must validate their own client, role and metrics.
// Unsupported requests never fall back to another client's combat model.
type Calculator interface {
	Capability(*Snapshot, time.Time) Capability
	Run(context.Context, *Snapshot, Scenario, []string, string) (*Result, error)
}

type Provider struct {
	Config     ProviderConfig
	HideWindow func(*exec.Cmd)
}

type packagedProviderManifest struct {
	Provider   string            `json:"provider"`
	Client     string            `json:"client"`
	Version    string            `json:"version"`
	Source     string            `json:"source"`
	Networking *bool             `json:"networking"`
	Files      map[string]string `json:"files"`
}

var damageSpecs = map[int]string{
	62: "arcane", 63: "fire", 64: "frost", 70: "retribution", 71: "arms", 72: "fury",
	102: "balance", 103: "feral", 250: "blood", 251: "frost", 252: "unholy", 253: "beast_mastery",
	254: "marksmanship", 255: "survival", 258: "shadow", 259: "assassination", 260: "outlaw",
	261: "subtlety", 262: "elemental", 263: "enhancement", 265: "affliction", 266: "demonology",
	267: "destruction", 269: "windwalker", 577: "havoc", 1467: "devastation", 1473: "augmentation",
}

type Capability struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Available  bool   `json:"available"`
	Reason     string `json:"reason"`
	Version    string `json:"version"`
	Validation string `json:"validation"`
}

func loadPackagedProvider(dir string) *Provider {
	p := &Provider{}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return p
	}
	var manifest packagedProviderManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Provider != "simc" || manifest.Client != "retail" ||
		manifest.Networking == nil || *manifest.Networking || manifest.Version == "" || !strings.HasPrefix(manifest.Source, "https://github.com/simulationcraft/simc/commit/") {
		return p
	}
	hash := manifest.Files["simc.exe"]
	if decoded, err := hex.DecodeString(hash); err != nil || len(decoded) != sha256.Size {
		return p
	}
	path, err := filepath.Abs(filepath.Join(dir, "simc.exe"))
	if err != nil {
		return p
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return p
	}
	worker, err := os.Open(path)
	if err != nil {
		return p
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, worker)
	closeErr := worker.Close()
	if copyErr != nil || closeErr != nil || hex.EncodeToString(digest.Sum(nil)) != hash {
		return p
	}
	p.Config = ProviderConfig{Path: path, Version: manifest.Version, SHA256: hash, Source: manifest.Source, Builds: []string{}, Specs: []int{}}
	return p
}

// LoadProvider keeps an explicitly registered worker authoritative. A verified
// worker shipped beside the engine is the zero-setup fallback for clean installs.
func LoadProvider(dir string, packages ...string) *Provider {
	p := &Provider{}
	if data, err := os.ReadFile(filepath.Join(dir, "simc-provider.json")); err == nil {
		_ = json.Unmarshal(data, &p.Config)
		if p.Config.Path != "" {
			return p
		}
	}
	for _, packageDir := range packages {
		if packaged := loadPackagedProvider(packageDir); packaged.Config.Path != "" {
			return packaged
		}
	}
	return p
}
func (p *Provider) Capability(s *Snapshot, now time.Time) Capability {
	c := Capability{ID: "simc", Name: "SimulationCraft", Version: p.Config.Version, Validation: "unreviewed"}
	if s != nil {
		if s.Client != "retail" {
			c.Reason = "Forever combat models are unfinished; calculations are unavailable."
			return c
		}
		if s.Character.Role != "DAMAGER" || s.Character.Spec == 1468 {
			c.Reason = "This specialization isn't supported yet."
			if s.Character.Role == "HEALER" || s.Character.Spec == 1468 {
				c.Reason = "Healing sims aren't available yet."
			} else if s.Character.Role == "TANK" {
				c.Reason = "Tank sims aren't available yet."
			}
			return c
		}
		if s.Character.Spec == 1473 && s.Character.Level != 90 {
			c.Reason = "Augmentation group comparisons require level 90."
			return c
		}
		profile, err := ParseProfile(s.Profile)
		if err != nil {
			c.Reason = err.Error()
			return c
		}
		if s.Character.Spec == 1473 && (s.Character.Class != "EVOKER" || profile.Class != "evoker" || profile.Level != 90) {
			c.Reason = "Augmentation requires a matching level 90 Evoker export."
			return c
		}
		if damageSpecs[s.Character.Spec] == "" || damageSpecs[s.Character.Spec] != profile.Spec {
			c.Reason = "Unsupported specialization or mismatched export."
			return c
		}
		if (profile.Role != "attack" && profile.Role != "spell") || profile.Class != strings.ToLower(s.Character.Class) {
			c.Reason = "Export doesn't match this character's class or role."
			return c
		}
	}
	if p.Config.Path == "" {
		c.Reason = "SimulationCraft isn't installed."
		return c
	}
	if !filepath.IsAbs(p.Config.Path) || len(p.Config.SHA256) != 64 || p.Config.Version == "" {
		c.Reason = "Worker registration is incomplete."
		return c
	}
	if _, err := os.Stat(p.Config.Path); err != nil {
		c.Reason = "Registered worker is missing."
		return c
	}
	c.Available = true
	if s != nil {
		buildOK, specOK := false, false
		for _, b := range p.Config.Builds {
			if b == s.Build {
				buildOK = true
			}
		}
		for _, spec := range p.Config.Specs {
			if spec == s.Character.Spec {
				specOK = true
			}
		}
		if buildOK && specOK && !p.Config.ReviewedAt.IsZero() && now.Sub(p.Config.ReviewedAt) >= 0 && now.Sub(p.Config.ReviewedAt) <= 7*24*time.Hour {
			c.Validation = "reviewed"
		} else {
			c.Reason = "Model not reviewed for this build and spec."
		}
	}
	return c
}

type ResultItem struct {
	Venomstones  int    `json:"venomstones,omitempty"`
	Variant      string `json:"variant,omitempty"`
	CraftedStats string `json:"craftedStats,omitempty"`
	ID           int    `json:"id"`
	CandidateID  string `json:"candidateID,omitempty"`
	Level        int    `json:"level"`
	Slot         string `json:"slot"`
	Label        string `json:"label"`
	Key          string `json:"key"`
	BaseKey      string `json:"baseKey,omitempty"`
	Source       string `json:"source,omitempty"`
}
type Metric struct {
	Label       string             `json:"label,omitempty"`
	CandidateID string             `json:"candidateID,omitempty"`
	Source      string             `json:"source,omitempty"`
	Slot        string             `json:"slot,omitempty"`
	Variant     string             `json:"variant,omitempty"`
	Level       int                `json:"level,omitempty"`
	MaxLevel    int                `json:"maxLevel,omitempty"`
	Items       []ResultItem       `json:"items,omitempty"`
	Model       string             `json:"model,omitempty"`
	Stats       map[string]float64 `json:"stats,omitempty"`
	Name        string             `json:"name"`
	Mean        float64            `json:"mean"`
	StdDev      float64            `json:"stdDev"`
	MeanError   float64            `json:"meanError"`
	Refined     bool               `json:"refined,omitempty"`
}
type TargetResult struct {
	EncounterID         string               `json:"encounterID,omitempty"`
	EncounterName       string               `json:"encounterName,omitempty"`
	TargetCount         int                  `json:"targetCount,omitempty"`
	Bloodlust           string               `json:"bloodlust,omitempty"`
	Targets             int                  `json:"targets"`
	FightStyle          string               `json:"fightStyle,omitempty"`
	RaidBuffs           string               `json:"raidBuffs,omitempty"`
	Seconds             int                  `json:"seconds,omitempty"`
	Baseline            Metric               `json:"baseline"`
	Comparisons         []Metric             `json:"comparisons"`
	RejectedComparisons []RejectedComparison `json:"rejectedComparisons,omitempty"`
	Tested              int                  `json:"tested"`
	Requested           int                  `json:"requested"`
	Refined             int                  `json:"refined,omitempty"`
	Iterations          int                  `json:"iterations"`
	ElapsedSeconds      float64              `json:"elapsedSeconds"`
	Complete            bool                 `json:"complete"`
}
type Upgrade struct {
	Link          string       `json:"link,omitempty"`
	Key           string       `json:"key,omitempty"`
	CapturedLevel int          `json:"capturedLevel,omitempty"`
	Items         []ResultItem `json:"items,omitempty"`
	BIS           bool         `json:"bis,omitempty"`
	BISRank       int          `json:"bisRank,omitempty"`
	BISSlot       string       `json:"bisSlot,omitempty"`
	LootVerified  bool         `json:"lootVerified,omitempty"`
	PoolComplete  bool         `json:"poolComplete,omitempty"`
	PoolSize      int          `json:"poolSize,omitempty"`
	LootSourceID  string       `json:"lootSourceID,omitempty"`
	CraftedStats  string       `json:"craftedStats,omitempty"`
	DifficultyID  int          `json:"difficultyID,omitempty"`
	KeyLevel      int          `json:"keyLevel,omitempty"`
	Rare          bool         `json:"rare,omitempty"`
	ID            int          `json:"id"`
	Level         int          `json:"level"`
	Slot          string       `json:"slot"`
	Label         string       `json:"label"`
	Source        string       `json:"source"`
	SourceID      string       `json:"sourceID"`
	Type          string       `json:"type"`
	Kind          string       `json:"kind"`
	Mean          float64      `json:"mean"`
	Gain          float64      `json:"gain"`
	Owned         bool         `json:"owned"`
}
type Result struct {
	processedComparisons int
	Incomplete           bool                 `json:"incomplete,omitempty"`
	Healing              *healsim.Report      `json:"healing,omitempty"`
	Builds               *BuildReport         `json:"builds,omitempty"`
	InputRevision        int                  `json:"inputRevision,omitempty"`
	RejectedComparisons  []RejectedComparison `json:"rejectedComparisons,omitempty"`
	IncompleteSources    []string             `json:"incompleteSources,omitempty"`
	RollSources          map[string]string    `json:"rollSources,omitempty"`
	ModelInputs          map[string]string    `json:"modelInputs,omitempty"`
	Upgrades             []Upgrade            `json:"upgrades,omitempty"`
	TargetSet            *Metric              `json:"targetSet,omitempty"`
	SearchTests          int                  `json:"searchTests,omitempty"`
	Season               int                  `json:"season,omitempty"`
	Provider             string               `json:"provider,omitempty"`
	Model                string               `json:"model,omitempty"`
	Metric               string               `json:"metric"`
	Baseline             Metric               `json:"baseline"`
	Comparisons          []Metric             `json:"comparisons"`
	TargetResults        []TargetResult       `json:"targetResults,omitempty"`
	ProviderVersion      string               `json:"providerVersion"`
	ProviderSHA256       string               `json:"providerSHA256"`
	Validation           string               `json:"validation"`
	Warnings             []string             `json:"warnings"`
}

func (p *Provider) Run(ctx context.Context, s *Snapshot, scenario Scenario, selected []string, dir string) (*Result, error) {
	if len(scenario.Encounters) > 0 && scenario.Task != "analysis" {
		return nil, fmt.Errorf("multiple encounters require an analysis")
	}
	if scenario.Task == "upgrades" {
		return p.runLoot(ctx, s, scenario, s.Loot, dir)
	}
	if scenario.Task == "analysis" {
		return p.runAnalysis(ctx, s, scenario, selected, dir)
	}
	if scenario.Task != "" {
		return nil, fmt.Errorf("automatic loot comparisons are not available for this calculator")
	}
	profile, err := ParseProfile(s.Profile)
	if err != nil {
		return nil, err
	}
	profile, err = prepareComparisonProfile(s, profile, scenario, selected)
	if err != nil {
		return nil, err
	}
	comparisons, err := profile.Comparisons(scenario, selected)
	if err != nil {
		return nil, err
	}
	comparisons, rejected, warnings, err := applyGearRules(s, profile, comparisons, scenario.CompareMode == "topgear" || scenario.CompareMode == "loadout")
	if err != nil {
		return nil, err
	}
	result, err := p.runComparisons(ctx, s, profile, scenario, comparisons, dir)
	if err != nil {
		return nil, err
	}
	result.RejectedComparisons = append(result.RejectedComparisons, rejected...)
	result.Warnings = append(result.Warnings, warnings...)
	if len(rejected) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d equipment combinations were excluded by item restrictions.", len(rejected)))
	}
	return result, nil
}

// runPrepared executes only input assembled by the allowlisted profile and comparison builders.
func (p *Provider) runPrepared(ctx context.Context, s *Snapshot, scenario Scenario, input string, comparisons []Comparison, dir string) (*Result, error) {
	capability := p.Capability(s, time.Now())
	if !capability.Available {
		return nil, fmt.Errorf("%s", capability.Reason)
	}
	f, err := os.Open(p.Config.Path)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	f.Close()
	if err != nil {
		return nil, err
	}
	if hex.EncodeToString(h.Sum(nil)) != p.Config.SHA256 {
		return nil, fmt.Errorf("worker checksum changed; register and review this version again")
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	gearPath := filepath.Join(dir, "verified-gear.simc")
	if err = os.Remove(gearPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(dir, "input.simc"), []byte(input+"\nprofileset_output_data=gear\nsave_gear=verified-gear.simc\n"), 0600); err != nil {
		return nil, err
	}
	output, err := os.OpenFile(filepath.Join(dir, "worker.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer output.Close()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Config.Path, "input.simc", "json2=result.json")
	cmd.Dir = dir
	cmd.Stdout = output
	cmd.Stderr = output
	if p.HideWindow != nil {
		p.HideWindow(cmd)
	}
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exited *exec.ExitError
		if errors.As(err, &exited) && (exited.ExitCode() == 81 || exited.ExitCode() == 82) {
			return nil, &workerInputError{"SimulationCraft rejected item or talent input; see this run's worker log"}
		}
		return nil, fmt.Errorf("SimulationCraft failed; see this run's worker log: %w", err)
	}
	data, err := readBounded(filepath.Join(dir, "result.json"), 32*1024*1024)
	if err != nil {
		return nil, err
	}
	gear, err := readBounded(gearPath, 64*1024)
	if err != nil {
		return nil, fmt.Errorf("worker equipment report unavailable: %w", err)
	}
	if err := verifyWorkerGear(data, gear, s, nil); err != nil {
		return nil, err
	}
	accepted := make([]Comparison, 0, len(comparisons))
	rejected := []RejectedComparison{}
	for _, comparison := range comparisons {
		if verifyErr := verifyWorkerGear(data, gear, s, []Comparison{comparison}); verifyErr != nil {
			if !comparisonRetryable(verifyErr) {
				return nil, verifyErr
			}
			rejected = append(rejected, RejectedComparison{ID: comparison.ID, Label: comparison.Label, Reason: verifyErr.Error()})
			continue
		}
		accepted = append(accepted, comparison)
	}
	var aug *augmentationConfig
	if s.Character.Spec == 1473 {
		settings := augmentationSettings(scenario)
		aug = &settings
	}
	result, err := parseWorkerResult(data, aug)
	if err != nil {
		return nil, err
	}
	acceptedNames := map[string]bool{}
	for _, comparison := range accepted {
		acceptedNames[comparison.ID] = true
	}
	filtered := result.Comparisons[:0]
	for _, comparison := range result.Comparisons {
		if acceptedNames[comparison.Name] {
			filtered = append(filtered, comparison)
		}
	}
	result.Comparisons = filtered
	if len(result.Comparisons) != len(accepted) {
		return nil, fmt.Errorf("worker did not return every requested comparison")
	}
	wanted := map[string]bool{}
	for _, comparison := range accepted {
		wanted[comparison.ID] = true
	}
	for _, metric := range result.Comparisons {
		if !wanted[metric.Name] {
			return nil, fmt.Errorf("worker returned an unexpected comparison")
		}
		delete(wanted, metric.Name)
	}
	for i := range result.Comparisons {
		for _, c := range accepted {
			if result.Comparisons[i].Name == c.ID {
				decorateMetric(&result.Comparisons[i], c)
			}
		}
	}
	result.RejectedComparisons = append(result.RejectedComparisons, rejected...)
	if len(rejected) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d comparisons were rejected because SimulationCraft did not apply the requested equipment.", len(rejected)))
	}
	sortComparisons(result)
	if aug != nil {
		aug.describe(result)
		result.ModelInputs["encounter"] = fmt.Sprintf("%d stationary targets for %d seconds; raid buffs %s; Bloodlust %s. Fixed simulated allies are unchanged across candidates.", scenario.Targets, scenario.Seconds, scenario.RaidBuffs, scenario.Bloodlust)
	}
	if scenario.Bloodlust != "" {
		result.Warnings = append(result.Warnings, "Bloodlust explicitly "+scenario.Bloodlust+"; independent of the selected raid-buff preset.")
	}
	result.InputRevision = 2
	result.Provider = "SimulationCraft"
	result.ProviderVersion = p.Config.Version
	result.ProviderSHA256 = p.Config.SHA256
	result.Validation = capability.Validation
	result.Warnings = append(result.Warnings, s.Warnings...)
	switch scenario.FightStyle {
	case "dungeon_slice":
		result.Warnings = append(result.Warnings, "Dungeon Slice, six-minute variable target pattern, self buffs only.")
	case "patchwerk", "":
		if scenario.RaidBuffs == "none" {
			result.Warnings = append(result.Warnings, "Stationary targets, self buffs only, no movement.")
		} else {
			result.Warnings = append(result.Warnings, "Stationary targets, full raid buffs, no movement.")
		}
	}
	if capability.Reason != "" {
		result.Warnings = append(result.Warnings, capability.Reason)
	}
	return result, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("file exceeds limit")
	}
	return b, err
}

func ParseResult(data []byte) (*Result, error) {
	var report struct {
		Logs []struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		} `json:"logs"`
		Sim struct {
			Options struct {
				ConfidenceEstimator float64 `json:"confidence_estimator"`
			} `json:"options"`
			Players []struct {
				Collected struct {
					DPS struct {
						Mean       float64 `json:"mean"`
						StdDev     float64 `json:"std_dev"`
						MeanError  float64 `json:"mean_error"`
						MeanStdDev float64 `json:"mean_std_dev"`
					} `json:"dps"`
				} `json:"collected_data"`
			} `json:"players"`
			Profilesets struct {
				Results []struct {
					Name      string  `json:"name"`
					Mean      float64 `json:"mean"`
					StdDev    float64 `json:"stddev"`
					MeanError float64 `json:"mean_error"`
				} `json:"results"`
			} `json:"profilesets"`
		} `json:"sim"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	if len(report.Sim.Players) != 1 || report.Sim.Players[0].Collected.DPS.Mean <= 0 {
		return nil, fmt.Errorf("worker did not produce one positive DPS result")
	}
	dps := report.Sim.Players[0].Collected.DPS
	if dps.MeanError == 0 {
		dps.MeanError = dps.MeanStdDev * report.Sim.Options.ConfidenceEstimator
	}
	r := &Result{Metric: "DPS", Baseline: Metric{Name: "Current gear", Mean: dps.Mean, StdDev: dps.StdDev, MeanError: dps.MeanError}, Comparisons: []Metric{}, Warnings: []string{}}
	for _, entry := range report.Logs {
		if entry.Level == "error" {
			return nil, fmt.Errorf("SimulationCraft reported an error: %.1024s", entry.Message)
		}
		if (strings.Contains(entry.Level, "warn") || entry.Level == "implementation_not_yet_verified") && len(r.Warnings) < 32 {
			r.Warnings = append(r.Warnings, fmt.Sprintf("SimulationCraft: %.1024s", entry.Message))
		}
	}
	for _, v := range report.Sim.Profilesets.Results {
		if v.Mean <= 0 {
			return nil, fmt.Errorf("worker returned an invalid comparison")
		}
		r.Comparisons = append(r.Comparisons, Metric{Name: v.Name, Mean: v.Mean, StdDev: v.StdDev, MeanError: v.MeanError})
	}
	return r, nil
}
