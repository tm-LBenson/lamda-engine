// lamda-meter-replay reviews a local log without controlling or attaching to WoW.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tm-LBenson/lamda-engine/internal/augmentation"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func main() {
	input := flag.String("log", "", "Original local combat log (read only)")
	output := flag.String("output", "", "Private output directory")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-log and -output are required")
		os.Exit(2)
	}
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(input, output string) error {
	if err := os.MkdirAll(output, 0700); err != nil {
		return err
	}
	store := augmentation.New(filepath.Join(output, "reports.json"))
	defer store.Close()
	store.SetEnabled(true)
	store.SetDebug(true)
	source, err := os.Open(input)
	if err != nil {
		return err
	}
	defer source.Close()
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		at, _, err := engine.RecordFields(line)
		if err != nil {
			continue
		}
		store.Ingest(line, at)
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store.Snapshot(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "state.json"), append(data, '\n'), 0600); err != nil {
		return err
	}
	c := engine.DefaultConfig()
	c.AugEnabled = true
	c.AugShowMeter = true
	frame := store.Merge(engine.FrameDisplay{}, c, lookup.WindowContext{Window: 123, PID: 456, Width: 1920, Height: 1080, Foreground: true}, store.Snapshot().LastEventAt)
	data, err = json.Marshal(map[string]any{"fixtureOnly": true, "frameDisplay": frame})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "native-frame.json"), data, 0600); err != nil {
		return err
	}
	fmt.Printf("Replayed %d records; this does not measure live delivery latency.\n", count)
	return nil
}
