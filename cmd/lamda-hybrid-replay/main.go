// Replay measured log receipts against an IDEALIZED immediate native meter.
// This evaluates estimator error, not actual capture latency or hook completeness.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/augmentation"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"math"
	"os"
	"sort"
	"time"
)

type receipt struct {
	At    time.Time
	Order int64
	Line  string
}
type truth struct {
	At          time.Time
	GUID, Owner string
	Amount      int64
	Support     bool
}
type aura struct {
	At     time.Time
	GUID   string
	Bit    int
	Active bool
}
type trace struct {
	Name, Player   string
	SessionID      int
	Start, End     time.Time
	Context        []string
	Members        []lookup.NativeMeterMember
	Receipts       []receipt
	Truth          []truth
	Auras          []aura
	CalibrationLog string
}

func main() {
	input := flag.String("trace", "", "Private receipt trace")
	state := flag.String("state", "", "Only history completed before this fight")
	output := flag.String("output", "", "Private result JSON")
	flag.Parse()
	if *input == "" || *state == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-trace, -state and -output are required")
		os.Exit(2)
	}
	if e := run(*input, *state, *output); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[int(float64(len(c)-1)*p)]
}
func run(input, state, output string) error {
	data, e := os.ReadFile(input)
	if e != nil {
		return e
	}
	var t trace
	if e = json.Unmarshal(data, &t); e != nil {
		return e
	}
	if t.Start.IsZero() || !t.End.After(t.Start) || len(t.Members) == 0 {
		return fmt.Errorf("invalid trace")
	}
	s := augmentation.New(state)
	defer s.Close()
	s.SetEnabled(true)
	for _, m := range s.Snapshot().DamageMeters {
		if !m.EndedAt.Before(t.Start) {
			return fmt.Errorf("future encounter leaked into calibration history")
		}
	}
	classes := map[string]int{}
	for _, m := range t.Members {
		classes[m.GUID] = m.Class
	}
	for _, line := range t.Context {
		s.Ingest(line, t.Start)
	}
	if t.CalibrationLog != "" {
		if err := s.SeedCalibrationLog(t.CalibrationLog, t.Start, classes); err != nil {
			return err
		}
	}
	sort.SliceStable(t.Receipts, func(i, j int) bool {
		if t.Receipts[i].At.Equal(t.Receipts[j].At) {
			return t.Receipts[i].Order < t.Receipts[j].Order
		}
		return t.Receipts[i].At.Before(t.Receipts[j].At)
	})
	sort.SliceStable(t.Truth, func(i, j int) bool { return t.Truth[i].At.Before(t.Truth[j].At) })
	until := t.End.Add(2 * time.Second)
	for _, r := range t.Receipts {
		if r.At.After(until) {
			until = r.At.Add(2 * time.Second)
		}
	}
	amounts := map[string]int64{}
	credited := int64(0)
	ri, ti, serial := 0, 0, 0
	ai := 0
	masks := map[string]int{}
	sort.SliceStable(t.Auras, func(i, j int) bool { return t.Auras[i].At.Before(t.Auras[j].At) })
	conserved := true
	var errors, warm []float64
	var last *augmentation.DamageMeter
	for now := t.Start; !now.After(until); now = now.Add(250 * time.Millisecond) {
		for ti < len(t.Truth) && !t.Truth[ti].At.After(now) {
			r := t.Truth[ti]
			if !r.Support {
				amounts[r.GUID] += r.Amount
			} else if r.Owner == t.Player && r.GUID != t.Player {
				credited += r.Amount
			}
			ti++
		}
		for ai < len(t.Auras) && !t.Auras[ai].At.After(now) {
			a := t.Auras[ai]
			if a.Active {
				masks[a.GUID] |= a.Bit
			} else {
				masks[a.GUID] &= ^a.Bit
			}
			ai++
		}
		serial++
		duration := now.Sub(t.Start)
		if now.After(t.End) {
			duration = t.End.Sub(t.Start)
		}
		n := &lookup.NativeMeter{ID: "idealized-replay:1", Serial: serial, SessionID: t.SessionID, Player: t.Player, Spec: 1473, Combat: now.Before(t.End), DurationMillis: duration.Milliseconds(), Captured: true, Members: append([]lookup.NativeMeterMember(nil), t.Members...)}
		for i := range n.Members {
			n.Members[i].Total = amounts[n.Members[i].GUID]
			n.Members[i].AuraMode = 15
			n.Members[i].AuraMask = masks[n.Members[i].GUID]
		}
		s.ObserveNative(n, now, "")
		for ri < len(t.Receipts) && !t.Receipts[ri].At.After(now) {
			r := t.Receipts[ri]
			s.Ingest(r.Line, r.At)
			ri++
		}
		v := s.HybridView(now)
		if v == nil {
			return fmt.Errorf("no hybrid output")
		}
		last = v
		conserved = conserved && v.RawTotal == v.AdjustedTotal
		actual := amounts[t.Player] + credited
		if actual > 0 && now.Before(t.End) {
			for _, row := range v.Rows {
				if row.GUID == t.Player {
					errorPct := 100 * math.Abs(float64(row.Adjusted-actual)) / float64(actual)
					errors = append(errors, errorPct)
					if now.Sub(t.Start) >= 10*time.Second {
						warm = append(warm, errorPct)
					}
				}
			}
		}
	}
	actual := amounts[t.Player] + credited
	observed := int64(-1)
	for _, row := range last.Rows {
		if row.GUID == t.Player {
			observed = row.Adjusted
		}
	}
	result := map[string]any{"name": t.Name, "frames": serial, "receipts": len(t.Receipts), "groupConservedEveryFrame": conserved,
		"absoluteAugErrorPct":    map[string]float64{"median": percentile(errors, .5), "p95": percentile(errors, .95), "max": percentile(errors, 1)},
		"after10SecondsErrorPct": map[string]float64{"median": percentile(warm, .5), "p95": percentile(warm, .95), "max": percentile(warm, 1)},
		"finalExpectedAug":       actual, "finalObservedAug": observed, "finalProvisional": last.Provisional, "finalMeter": last,
		"assumptions": "IDEALIZED instant native totals sampled at 250ms; actual measured damage receipts and public roster metadata; idealized native buff presence from original aura events; not live-capture verification, a simulation result or proof of complete support hooks"}
	encoded, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(output, append(encoded, '\n'), 0600); e != nil {
		return e
	}
	fmt.Printf("%s: estimator median %.2f%%, p95 %.2f%%; final %d/%d; conserved=%v; final provisional=%v\n", t.Name, percentile(errors, .5), percentile(errors, .95), observed, actual, conserved, last.Provisional)
	if !conserved || observed != actual {
		return fmt.Errorf("reconciliation failed")
	}
	return nil
}
