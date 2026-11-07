package healsim

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func trainingSamples() []EncounterSample {
	var samples []EncounterSample
	for i := 0; i < 4; i++ {
		samples = append(samples, EncounterSample{ID: fmt.Sprint(i), Cohort: "fixture/difficulty2/key0", RecordedAt: int64(i + 1), Replay: Replay{Schema: 1, Source: "fixture", Encounter: "Training encounter", Difficulty: "2", Patch: "12.1", Seconds: 60 + float64(i), Members: []ReplayMember{{"You", 800000}, {"Tank", 1000000}}, Events: []DamageEvent{{10, 0, 300000}, {10, 1, 400000}, {20, 1, 100000}}}})
	}
	return samples
}
func TestTrainingRejectsLeakageMixedCohortsAndSparseData(t *testing.T) {
	for _, change := range []func([]EncounterSample) []EncounterSample{
		func(s []EncounterSample) []EncounterSample { return s[:2] },
		func(s []EncounterSample) []EncounterSample { s[1].ID = s[0].ID; return s },
		func(s []EncounterSample) []EncounterSample { s[1].Cohort = "different key"; return s },
		func(s []EncounterSample) []EncounterSample { s[1].Replay.Patch = "12.0"; return s },
	} {
		if _, err := Train(change(trainingSamples())); err == nil {
			t.Fatal("unsafe training accepted")
		}
	}
	tr, err := Train(trainingSamples())
	if err != nil {
		t.Fatal(err)
	}
	if tr.Holdout.ID != "3" || len(tr.Samples) != 3 {
		t.Fatal("wrong chronological split")
	}
	tr.Holdout, tr.Samples[0] = tr.Samples[0], tr.Holdout
	if tr.Validate() == nil {
		t.Fatal("holdout leaked into training")
	}
}
func TestTrainingPreservesBurstsAndForecastNeverReadsHoldout(t *testing.T) {
	samples := trainingSamples()
	samples[3].Replay.Events = []DamageEvent{{5, 0, 700000}}
	tr, err := Train(samples)
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(0); seed < 100; seed++ {
		s := tr.draw(seed)
		if s.ID == tr.Holdout.ID || len(s.Replay.Events) != 3 {
			t.Fatal("holdout sampled")
		}
		if s.Replay.Events[0].Time != s.Replay.Events[1].Time || s.Replay.Events[1].Amount != 400000 {
			t.Fatal("burst correlation lost")
		}
	}
	if tr.burstSoon(1, 100) || !tr.burstSoon(6, 100) {
		t.Fatal("forecast learned held-out future or lost training burst")
	}
	o := testOptions()
	o.Scenario = "learned"
	o.Training = tr
	o.Runs = 3
	a, err := Run(context.Background(), testStats(), o)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Workloads) != 3 || len(a.HoldoutOutcomes) != 2 {
		t.Fatal("missing reproducibility or validation outcomes")
	}
	samples[3].Replay.Events = []DamageEvent{{4, 0, 800000}}
	o.Training, err = Train(samples)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(context.Background(), testStats(), o)
	if err != nil {
		t.Fatal(err)
	}
	aa, _ := json.Marshal(a.Outcomes)
	bb, _ := json.Marshal(b.Outcomes)
	if string(aa) != string(bb) {
		t.Fatal("held-out damage changed training simulation")
	}
	if b.HoldoutOutcomes[0].Deaths <= a.HoldoutOutcomes[0].Deaths {
		t.Fatal("held-out simulation ignored new lethal damage")
	}
}
func TestTrainingDamageValidationUsesWholeFightAndBurstWindow(t *testing.T) {
	samples := trainingSamples()
	tr, err := Train(samples)
	if err != nil {
		t.Fatal(err)
	}
	near(t, tr.Summary.Training.PeakFiveSeconds, 700000)
	near(t, tr.Summary.Holdout.DamagePS, 800000./63)
	near(t, tr.Summary.MinSeconds, 60)
	near(t, tr.Summary.MaxSeconds, 62)
}
func TestCapturedDreamBreathTalentsChangeAmountTicksAndDuration(t *testing.T) {
	stats := testStats()
	base := newSimulation(stats, testOptions(), "consensus", 1)
	for _, id := range []int{1239581, 444845, 1264321, 1218447} {
		stats.Talents[id] = 1
	}
	s := newSimulation(stats, testOptions(), "consensus", 1)
	near(t, s.spells[breath].direct, base.spells[breath].direct*1.1)
	near(t, s.spells[breath].hot, base.spells[breath].hot*1.1*1.2)
	near(t, s.spells[breath].period, base.spells[breath].period*.85)
	near(t, s.spells[breath].duration, base.spells[breath].duration+6)
	stats.Talents[1239581] = 2
	if validatePreservation(stats) == nil {
		t.Fatal("invalid talent rank accepted")
	}
}
