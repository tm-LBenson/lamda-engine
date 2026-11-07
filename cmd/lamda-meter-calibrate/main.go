// Prepare private, measured buff-conditioned calibration from an existing log.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/augmentation"
	"os"
	"time"
)

func main() {
	input := flag.String("log", "", "Existing advanced combat log, read only")
	output := flag.String("output", "", "Private calibration JSON")
	classesPath := flag.String("classes", "", "Optional public GUID/class metadata JSON")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-log and -output required")
		os.Exit(2)
	}
	classes := map[string]int{}
	if *classesPath != "" {
		data, e := os.ReadFile(*classesPath)
		if e != nil {
			panic(e)
		}
		if e = json.Unmarshal(data, &classes); e != nil {
			panic(e)
		}
	}
	s := augmentation.New("")
	defer s.Close()
	s.SetEnabled(true)
	if e := s.SeedCalibrationLog(*input, time.Time{}, classes); e != nil {
		panic(e)
	}
	state := s.Snapshot()
	state.Reports = nil
	state.Live = nil
	state.DamageMeters = nil
	data, e := json.MarshalIndent(state, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(*output, append(data, '\n'), 0600); e != nil {
		panic(e)
	}
	fmt.Printf("Prepared %d measured recipient/buff calibrations; no live game or settings changed.\n", len(state.Calibrations))
}
