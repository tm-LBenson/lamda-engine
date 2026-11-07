package healsim

// Encounter learning deliberately resamples whole attempts. Independent random
// hits destroy the correlation between tank pressure, group bursts and downtime.
// A held-out attempt is never used for sampling or for the preparation forecast.
import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type EncounterSample struct {
	ID         string `json:"id"`
	Cohort     string `json:"cohort"`
	RecordedAt int64  `json:"recordedAt"`
	Replay     Replay `json:"replay"`
}
type DamageMetrics struct {
	DamagePS        float64 `json:"damagePs"`
	PeakFiveSeconds float64 `json:"peakFiveSeconds"`
}
type TrainingSummary struct {
	Method             string        `json:"method"`
	Cohort             string        `json:"cohort"`
	Encounter          string        `json:"encounter"`
	TrainingIDs        []string      `json:"trainingIds"`
	HoldoutID          string        `json:"holdoutId"`
	Training           DamageMetrics `json:"training"`
	Holdout            DamageMetrics `json:"holdout"`
	DamageErrorPercent float64       `json:"damageErrorPercent"`
	BurstErrorPercent  float64       `json:"burstErrorPercent"`
	MinSeconds         float64       `json:"minSeconds"`
	MaxSeconds         float64       `json:"maxSeconds"`
}
type EncounterTraining struct {
	forecast []float64
	Schema   int               `json:"schema"`
	Samples  []EncounterSample `json:"samples"`
	Holdout  EncounterSample   `json:"holdout"`
	Summary  TrainingSummary   `json:"summary"`
}

func damageMetrics(r Replay) DamageMetrics {
	var total, window, peak float64
	left := 0
	for i, e := range r.Events {
		total += e.Amount
		window += e.Amount
		for left < i && r.Events[left].Time < e.Time-5 {
			window -= r.Events[left].Amount
			left++
		}
		peak = math.Max(peak, window)
	}
	return DamageMetrics{total / r.Seconds, peak}
}

func Train(samples []EncounterSample) (*EncounterTraining, error) {
	unique := map[string]bool{}
	rows := append([]EncounterSample(nil), samples...)
	for _, s := range rows {
		if s.ID == "" || len(s.ID) > 128 || s.Cohort == "" || len(s.Cohort) > 256 || s.RecordedAt <= 0 {
			return nil, fmt.Errorf("invalid encounter identity")
		}
		if unique[s.ID] {
			return nil, fmt.Errorf("duplicate encounter cannot count as another training fight")
		}
		unique[s.ID] = true
		if err := s.Replay.Validate(); err != nil {
			return nil, err
		}
		first := rows[0]
		if s.Cohort != first.Cohort || s.Replay.Encounter != first.Replay.Encounter || s.Replay.Patch != first.Replay.Patch || s.Replay.Difficulty != first.Replay.Difficulty || len(s.Replay.Members) != len(first.Replay.Members) {
			return nil, fmt.Errorf("training cannot mix encounter, patch, difficulty, key level or roster roles")
		}
	}
	if len(rows) < 3 || len(rows) > 100 {
		return nil, fmt.Errorf("need at least three distinct complete fights of the same encounter, difficulty, key level and roster roles (two to train, one held out)")
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RecordedAt == rows[j].RecordedAt {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].RecordedAt < rows[j].RecordedAt
	})
	t := &EncounterTraining{Schema: 1, Samples: rows[:len(rows)-1], Holdout: rows[len(rows)-1]}
	t.Summary = TrainingSummary{Method: "whole-attempt empirical resampling; latest fight held out", Cohort: rows[0].Cohort, Encounter: rows[0].Replay.Encounter, HoldoutID: t.Holdout.ID, Holdout: damageMetrics(t.Holdout.Replay), MinSeconds: 600}
	for _, s := range t.Samples {
		t.Summary.TrainingIDs = append(t.Summary.TrainingIDs, s.ID)
		m := damageMetrics(s.Replay)
		t.Summary.Training.DamagePS += m.DamagePS / float64(len(t.Samples))
		t.Summary.Training.PeakFiveSeconds += m.PeakFiveSeconds / float64(len(t.Samples))
		t.Summary.MinSeconds = math.Min(t.Summary.MinSeconds, s.Replay.Seconds)
		t.Summary.MaxSeconds = math.Max(t.Summary.MaxSeconds, s.Replay.Seconds)
	}
	if t.Summary.Holdout.DamagePS > 0 {
		t.Summary.DamageErrorPercent = 100 * (t.Summary.Training.DamagePS/t.Summary.Holdout.DamagePS - 1)
	}
	if t.Summary.Holdout.PeakFiveSeconds > 0 {
		t.Summary.BurstErrorPercent = 100 * (t.Summary.Training.PeakFiveSeconds/t.Summary.Holdout.PeakFiveSeconds - 1)
	}
	t.fitForecast()
	return t, nil
}
func (t *EncounterTraining) Validate() error {
	if t == nil || t.Schema != 1 {
		return fmt.Errorf("missing learned encounter")
	}
	rows := append(append([]EncounterSample(nil), t.Samples...), t.Holdout)
	fitted, err := Train(rows)
	if err != nil {
		return err
	}
	if fitted.Holdout.ID != t.Holdout.ID {
		return fmt.Errorf("holdout must be the latest independent fight")
	}
	t.forecast = fitted.forecast
	t.Summary = fitted.Summary
	return nil
}
func (t *EncounterTraining) sample(seed int64) *Replay {
	// Damage selection is independent of healer decisions and proc RNG. Policies
	// and future build comparisons receive the exact same workload for a seed.
	copy := t.draw(seed).Replay
	return &copy
}
func (t *EncounterTraining) draw(seed int64) EncounterSample {
	r := rand.New(rand.NewSource(seed ^ 0x71ea913))
	return t.Samples[r.Intn(len(t.Samples))]
}
func (t *EncounterTraining) burstSoon(now float64, intensity int) bool {
	second := int(now)
	return second >= 0 && second < len(t.forecast) && t.forecast[second]*float64(intensity)/100 >= .3
}

// Forecast is fitted once, on one-second bins, using only prior training
// attempts. Sliding windows avoid rescanning logs for every simulated cast.
func (t *EncounterTraining) fitForecast() {
	bins := make([][]float64, 601)
	for _, sample := range t.Samples {
		r := sample.Replay
		left, right := 0, 0
		damage := make([]float64, len(r.Members))
		for second := 0; second <= 600; second++ {
			for right < len(r.Events) && r.Events[right].Time <= float64(second+5) {
				e := r.Events[right]
				damage[e.Target] += e.Amount
				right++
			}
			for left < right && r.Events[left].Time < float64(second+1) {
				e := r.Events[left]
				damage[e.Target] -= e.Amount
				left++
			}
			peak := 0.
			for i, amount := range damage {
				peak = math.Max(peak, amount/r.Members[i].MaxHealth)
			}
			bins[second] = append(bins[second], peak)
		}
	}
	t.forecast = make([]float64, len(bins))
	for second, values := range bins {
		sort.Float64s(values)
		t.forecast[second] = values[len(values)/2]
	}
}
