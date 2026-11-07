// Offline, explicit-input development runner. No gameplay or networking.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/heallogs"
	"github.com/tm-LBenson/lamda-engine/internal/healsim"
)

func main() {
	spec := flag.Int("spec", 1468, "Retail healer specialization ID")
	input := flag.String("stats", "", "captured healingStats JSON file (required)")
	replay := flag.String("replay", "", "normalized damage profile JSON")
	logs := flag.String("logs", "", "learn encounters from this local Retail Logs directory")
	cache := flag.String("cache", "", "local parsed-log cache/output directory, required with -logs")
	cohort := flag.String("cohort", "", "exact learned encounter cohort; defaults to latest eligible encounter")
	intensity := flag.Int("intensity", 100, "incoming damage percentage")
	scenario := flag.String("scenario", "dungeon", "tank, dungeon or raid")
	seconds := flag.Int("seconds", 180, "encounter duration")
	runs := flag.Int("runs", 25, "Monte Carlo runs per policy")
	flag.Parse()
	if *input == "" {
		fmt.Fprintln(os.Stderr, "Provide -stats. Sample stats must never be mistaken for your character.")
		os.Exit(2)
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		fail(err)
	}
	var stats healsim.Stats
	if err = json.Unmarshal(data, &stats); err != nil {
		fail(err)
	}
	opts := healsim.Options{Spec: *spec, Scenario: *scenario, Seconds: *seconds, Intensity: *intensity, Runs: *runs, Policy: "compare", Seed: 1729}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if *logs != "" {
		if *cache == "" || *replay != "" {
			fail(fmt.Errorf("use -cache with -logs, without -replay"))
		}
		training, _, audit, e := heallogs.Learn(ctx, *logs, *cache, *cohort)
		auditBytes, _ := json.MarshalIndent(audit, "", "  ")
		if err := os.WriteFile(filepath.Join(*cache, "audit.json"), auditBytes, 0600); err != nil {
			fail(err)
		}
		if e != nil {
			fail(e)
		}
		encoded, _ := json.MarshalIndent(training, "", "  ")
		if err := os.WriteFile(filepath.Join(*cache, "training.json"), encoded, 0600); err != nil {
			fail(err)
		}
		opts.Scenario = "learned"
		opts.Training = training
	}
	if *replay != "" {
		data, err = os.ReadFile(*replay)
		if err != nil {
			fail(err)
		}
		opts.Replay = &healsim.Replay{}
		if err = json.Unmarshal(data, opts.Replay); err != nil {
			fail(err)
		}
		opts.Scenario = "replay"
		opts.Seconds = int(math.Ceil(opts.Replay.Seconds))
	}
	result, err := healsim.Run(ctx, stats, opts)
	if err != nil {
		fail(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(result); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
