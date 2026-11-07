package simming

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/wowspec"
)

func pageForSpec(id int) []byte {
	spec, _ := wowspec.Lookup(id)
	return []byte(strings.ReplaceAll(string(buildPage()), "/evoker/preservation/", spec.Path()+"/"))
}

func TestAllSpecializationsKeepSeparateBuildCaches(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, spec := range wowspec.All {
		for mode := range buildModes {
			r, err := parseBuildPage(pageForSpec(spec.ID), spec.ID, mode, now)
			if err != nil {
				t.Fatalf("%s %s: %v", spec.Label(), mode, err)
			}
			r.UpdatedAt = now
			name := fmt.Sprintf("%d-%s.json", spec.ID, strings.ReplaceAll(mode, "+", "plus"))
			if err = writeJSON(filepath.Join(dir, name), r); err != nil {
				t.Fatal(err)
			}
			got, err := fetchBuilds(ctx, spec.ID, mode, dir)
			if err != nil || !got.Cached || got.Spec != spec.ID || got.Mode != mode || got.SpecName != spec.Label() || got.Class != spec.Class {
				t.Fatalf("wrong cache for %s %s: %v", spec.Label(), mode, err)
			}
			for _, source := range got.Sources {
				if source.ID == "murlok" && source.URL != got.URL {
					t.Fatal("source link disagrees with the selected mode")
				}
			}
		}
		if spec.ID != 1468 {
			if _, err := parseBuildPage(buildPage(), spec.ID, "m+", now); err == nil {
				t.Fatalf("accepted another spec's HTML for %s", spec.Label())
			}
		}
	}
	// Even a recent cache is invalid if its embedded identity disagrees.
	r, _ := parseBuildPage(buildPage(), 1468, "m+", now)
	if err := writeJSON(filepath.Join(dir, "65-mplus.json"), r); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchBuilds(ctx, 65, "m+", dir); err == nil {
		t.Fatal("mismatched cache accepted")
	}
	if _, err := fetchBuilds(ctx, -1, "m+", dir); err == nil {
		t.Fatal("unknown specialization accepted")
	}
}

func TestAllSpecsCanBrowseAnotherSpecsBuilds(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	r, err := parseBuildPage(pageForSpec(65), 65, "m+", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeJSON(filepath.Join(dir, "65-mplus.json"), r); err != nil {
		t.Fatal(err)
	}
	p := &encounterProvider{task: "builds", cache: dir}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cached requests need no network
	for _, spec := range wowspec.All {
		s := encounterSnapshot()
		s.Character.Spec = spec.ID
		s.Character.Class = spec.Class
		s.HealingStats = nil
		if !p.Capability(s, now).Available {
			t.Fatalf("no builds for %s", spec.Label())
		}
		scenario := DefaultScenario()
		scenario.BuildSpec = 65
		scenario.BuildMode = "m+"
		result, err := p.Run(ctx, s, scenario, nil, t.TempDir())
		if err != nil || result.Builds.Spec != 65 || result.Builds.Class != "PALADIN" {
			t.Fatalf("wrong selected spec from %s: %v", spec.Label(), err)
		}
	}
}

func TestBuildGearMarksPersonalDropWithoutReplacingItsSimResult(t *testing.T) {
	result := &Result{Baseline: Metric{Mean: 1000}, Upgrades: []Upgrade{
		{ID: 270000, Label: "Meta helm", Source: "Test dungeon", Gain: 2.5, Mean: 1025},
		{ID: 270001, Label: "Alternative helm", Source: "Test dungeon", Gain: 4, Mean: 1040},
	}}
	report := &BuildReport{Spec: 1468, Mode: "m+", Gear: []BuildGear{
		{ID: 270000, Slot: "Head", Name: "Meta helm", Rank: 1, BIS: true},
		{ID: 270001, Slot: "Head", Name: "Alternative helm", Rank: 2, BIS: false},
	}}
	applyBuildGear(result, report)
	if result.Builds != report || !result.Upgrades[0].BIS || result.Upgrades[0].BISRank != 1 || result.Upgrades[0].BISSlot != "Head" || result.Upgrades[1].BIS {
		t.Fatalf("BiS metadata was not matched by item ID: %+v", result.Upgrades)
	}
	if result.Upgrades[0].Gain != 2.5 || result.Upgrades[1].Gain != 4 {
		t.Fatal("meta classification changed the personal simulation result")
	}
}

func TestRealAllSpecBuildCapturesOptIn(t *testing.T) {
	dir := os.Getenv("LAMDA_TEST_BUILD_CAPTURES")
	if dir == "" {
		t.Skip("opt-in captured public pages")
	}
	for _, spec := range wowspec.All {
		t.Run(spec.Label(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("murlok-%d.html", spec.ID)))
			if err != nil {
				t.Fatal(err)
			}
			r, err := parseBuildPage(data, spec.ID, "m+", time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Players) != 10 || len(r.Talents) < 10 || r.Sample < 10 || len(r.Heroes) < 1 || r.Spec != spec.ID {
				t.Fatalf("incomplete report: %+v", r)
			}
			t.Logf("%d players, %d talents, %d sampled", len(r.Players), len(r.Talents), r.Sample)
		})
	}
}
