package simming

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var healerSpecs = map[int]string{65: "holy", 105: "restoration", 256: "discipline", 257: "holy", 264: "restoration", 270: "mistweaver", 1468: "preservation"}

type HealingConfig struct {
	ProviderConfig
	Runtime       string `json:"runtime"`
	RuntimeSHA256 string `json:"runtimeSHA256"`
}
type HealingProvider struct {
	Config     HealingConfig
	HideWindow func(*exec.Cmd)
}
type Providers struct {
	Damage  *Provider
	Healing *HealingProvider
}

func LoadProviders(dir string, hide func(*exec.Cmd), damagePackages ...string) *Providers {
	p := &Providers{Damage: LoadProvider(dir, damagePackages...), Healing: &HealingProvider{}}
	p.Damage.HideWindow = hide
	p.Healing.HideWindow = hide
	if data, err := os.ReadFile(filepath.Join(dir, "healing-provider.json")); err == nil {
		_ = json.Unmarshal(data, &p.Healing.Config)
	}
	return p
}
func (p *Providers) calculator(s *Snapshot) Calculator {
	if s != nil && s.Client == "forever" {
		return &ForeverProvider{}
	}
	if s != nil && s.Client == "retail" && (s.Character.Role == "HEALER" || healerSpecs[s.Character.Spec] != "") {
		return p.Healing
	}
	return p.Damage
}
func (p *Providers) Capability(s *Snapshot, now time.Time) Capability {
	return p.calculator(s).Capability(s, now)
}
func (p *Providers) Run(ctx context.Context, s *Snapshot, scenario Scenario, selected []string, dir string) (*Result, error) {
	return p.calculator(s).Run(ctx, s, scenario, selected, dir)
}
func (p *HealingProvider) Capability(s *Snapshot, now time.Time) Capability {
	c := Capability{ID: "qe", Name: "Questionably Epic", Version: p.Config.Version, Validation: "unreviewed"}
	if s != nil {
		if s.Client != "retail" || s.Character.Role != "HEALER" || healerSpecs[s.Character.Spec] == "" {
			c.Reason = "This healer specialization isn't supported."
			return c
		}
		profile, err := ParseProfile(s.Profile)
		if err != nil {
			c.Reason = err.Error()
			return c
		}
		if profile.Spec != healerSpecs[s.Character.Spec] || profile.Class != strings.ToLower(s.Character.Class) {
			c.Reason = "Healer export doesn't match this character."
			return c
		}
	}
	if p.Config.Path == "" || p.Config.Runtime == "" {
		c.Reason = "Healer calculator isn't installed."
		return c
	}
	for _, file := range []string{p.Config.Path, p.Config.Runtime} {
		if !filepath.IsAbs(file) {
			c.Reason = "Healer registration is incomplete."
			return c
		}
		if _, err := os.Stat(file); err != nil {
			c.Reason = "Healer calculator is missing."
			return c
		}
	}
	if len(p.Config.SHA256) != 64 || len(p.Config.RuntimeSHA256) != 64 {
		c.Reason = "Healer registration is incomplete."
		return c
	}
	c.Available = true
	c.Reason = "Preset model · not reviewed for this build and spec."
	if s != nil && reviewed(p.Config.ProviderConfig, s, now) {
		c.Validation = "reviewed"
		c.Reason = "Preset healing model."
	}
	return c
}
func reviewed(c ProviderConfig, s *Snapshot, now time.Time) bool {
	build, spec := false, false
	for _, b := range c.Builds {
		build = build || b == s.Build
	}
	for _, v := range c.Specs {
		spec = spec || v == s.Character.Spec
	}
	return build && spec && !c.ReviewedAt.IsZero() && now.Sub(c.ReviewedAt) >= 0 && now.Sub(c.ReviewedAt) <= 7*24*time.Hour
}
func checkHash(path, expected string) error {
	b, err := readBounded(path, 256*1024*1024)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("calculator checksum changed; register and review this version again")
	}
	return nil
}

// healingAnalysisCandidates describes the gear questions supported by the QE
// model. The worker evaluates these deterministic score comparisons in one
// process; unlike SimulationCraft analysis there is no target-count or random
// error refinement pass.
func healingAnalysisCandidates(s *Snapshot, profile *Profile) []Candidate {
	upgrades := map[string]ItemUpgrade{}
	for _, upgrade := range s.Upgrades {
		upgrades[upgrade.Source+"\x00"+upgrade.Slot+"\x00"+upgrade.Value] = upgrade
	}
	out := []Candidate{}
	for _, raw := range profile.Candidates {
		if raw.Source != "owned" && raw.Source != "vault" {
			continue
		}
		candidate := raw
		candidate.ComparisonID = candidate.ID
		candidate.Level = itemLevelFromValue(candidate.Value)
		candidate.Variant = "current"
		if upgrade, ok := upgrades[candidate.Source+"\x00"+candidate.Slot+"\x00"+candidate.Value]; ok {
			candidate.Level, candidate.MaxLevel = upgrade.Level, upgrade.MaxLevel
		}
		out = append(out, candidate)
		if candidate.MaxLevel > candidate.Level {
			maximum := candidate
			maximum.ID += "_max"
			maximum.Value = withItemLevel(maximum.Value, maximum.MaxLevel)
			maximum.Level = maximum.MaxLevel
			maximum.Variant = "max"
			out = append(out, maximum)
		}
	}
	for _, comparison := range analysisEquippedComparisons(s, profile) {
		for index, item := range comparison.Items {
			item.ComparisonID = item.ID
			item.ID = comparison.ID
			if len(comparison.Items) > 1 {
				item.ID = fmt.Sprintf("%s_%d", comparison.ID, index+1)
			}
			out = append(out, item)
		}
	}
	return out
}

func (p *HealingProvider) Run(ctx context.Context, s *Snapshot, scenario Scenario, selected []string, dir string) (*Result, error) {
	if len(scenario.ItemOptions) > 0 || len(scenario.Encounters) > 0 || scenario.Bloodlust != "" {
		return nil, fmt.Errorf("the healing preset model cannot apply combat encounters or gem/enchant alternatives")
	}
	c := p.Capability(s, time.Now())
	if !c.Available {
		return nil, fmt.Errorf("%s", c.Reason)
	}
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	if len(scenario.Talents) > 0 {
		return nil, fmt.Errorf("this healer model uses preset talents")
	}
	if err := checkHash(p.Config.Path, p.Config.SHA256); err != nil {
		return nil, err
	}
	if err := checkHash(p.Config.Runtime, p.Config.RuntimeSHA256); err != nil {
		return nil, err
	}
	profile, err := ParseProfile(s.Profile)
	if err != nil {
		return nil, err
	}
	if _, err = profile.Input(scenario, selected); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	candidates := profile.Candidates
	if scenario.Task == "analysis" {
		candidates = healingAnalysisCandidates(s, profile)
	}
	input := map[string]any{"client": s.Client, "spec": s.Character.Spec, "profile": profile.Base, "candidates": candidates, "selected": selected, "scenario": scenario, "loot": s.Loot}
	if err = writeJSON(filepath.Join(dir, "input.json"), input); err != nil {
		return nil, err
	}
	log, err := os.Create(filepath.Join(dir, "worker.log"))
	if err != nil {
		return nil, err
	}
	defer log.Close()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Config.Runtime, "--max-old-space-size=512", p.Config.Path, "input.json")
	cmd.Dir = dir
	cmd.Stdout = log
	cmd.Stderr = log
	if p.HideWindow != nil {
		p.HideWindow(cmd)
	}
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("healer calculation failed; see this run's worker log: %w", err)
	}
	data, err := readBounded(filepath.Join(dir, "result.json"), 32*1024*1024)
	if err != nil {
		return nil, err
	}
	var result Result
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Metric != "healing_score" || result.Baseline.Mean <= 0 {
		return nil, fmt.Errorf("healer returned an invalid metric")
	}
	if scenario.Task == "analysis" {
		if len(result.Comparisons) != 0 || len(result.TargetResults) != 1 || result.TargetResults[0].Targets != 1 || result.TargetResults[0].Baseline.Mean <= 0 {
			return nil, fmt.Errorf("healer returned an invalid gear analysis")
		}
		for _, m := range result.TargetResults[0].Comparisons {
			if m.Mean <= 0 || m.CandidateID == "" || m.Source == "" || m.Variant == "" {
				return nil, fmt.Errorf("healer returned an invalid gear comparison")
			}
		}
	} else {
		expected := len(selected)
		if scenario.CompareMode == "topgear" && expected > 0 {
			expected = 1
		}
		if len(result.Comparisons) != expected {
			return nil, fmt.Errorf("healer did not return all comparisons")
		}
		for _, m := range result.Comparisons {
			if m.Mean <= 0 {
				return nil, fmt.Errorf("healer returned an invalid comparison")
			}
		}
	}
	result.Provider = "Questionably Epic"
	result.ProviderVersion = p.Config.Version
	result.ProviderSHA256 = p.Config.SHA256
	result.Validation = c.Validation
	result.Warnings = append(result.Warnings, c.Reason)
	for _, warning := range s.Warnings {
		if warning != "Healing comparisons aren't available yet." {
			result.Warnings = append(result.Warnings, warning)
		}
	}
	return &result, nil
}
