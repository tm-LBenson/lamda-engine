package simming

import "testing"

func planFixture() (*Job, *Profile, PlanOptions) {
	j := &Job{ID: "job", Result: &Result{RollSources: map[string]string{"99": "a"}, Upgrades: []Upgrade{
		{ID: 1, SourceID: "a", Source: "Dungeon", Type: "bonus", Kind: "Dungeon", Gain: 10, BIS: true, PoolComplete: true, PoolSize: 2},
		{ID: 2, SourceID: "a", Source: "Dungeon", Type: "bonus", Kind: "Dungeon", Gain: -10, PoolComplete: true, PoolSize: 2},
	}}}
	p := &Profile{Planning: PlanningData{Currencies: []Currency{{1, 2}}, Rolls: []BonusRoll{{1, 99, 4, 10, 2, 64}, {1, 99, 4, 10, 1, 63}, {1, 99, 4, 9, 1, 64}}}}
	o := PlanOptions{Mode: "reroll", View: "bonus", Chance: 1, Cost: 1, Currency: 1, AssumeBonusPool: true, HistoryConfirmed: true}
	return j, p, o
}
func TestAcquisitionKeepsNonUpgradesInDenominatorAndUsesBalance(t *testing.T) {
	j, p, o := planFixture()
	r, e := planAcquisition(j, p, 64, o)
	if e != nil || len(r.Sources) != 1 || r.Sources[0].Expected != 5 || r.Sources[0].UpgradeChance != 0.5 || r.Sources[0].UpgradeCount != 1 || r.Sources[0].BISCount != 1 || *r.Affordable != 2 {
		t.Fatal(r, e)
	}
	j.Result.Upgrades[0].Owned = true
	r, e = planAcquisition(j, p, 64, o)
	if e != nil || r.Sources[0].Pool != 2 {
		t.Fatal("ownership wrongly removes reward", e)
	}
}
func TestAcquisitionHistoryIsScopedAndExhaustedPoolsReset(t *testing.T) {
	j, p, o := planFixture()
	o.History = &p.Planning.Rolls[0]
	r, e := planAcquisition(j, p, 64, o)
	if e != nil || r.Sources[0].Pool != 1 || r.Sources[0].Expected != 10 {
		t.Fatal(r, e)
	}
	p.Planning.Rolls = append(p.Planning.Rolls, BonusRoll{1, 99, 4, 10, 1, 64})
	r, e = planAcquisition(j, p, 64, o)
	if e != nil || r.Sources[0].Pool != 2 || r.Sources[0].Expected != 5 {
		t.Fatal("completed cycle not reset", r, e)
	}
	o.History = &p.Planning.Rolls[1]
	if _, e = planAcquisition(j, p, 64, o); e == nil {
		t.Fatal("wrong spec history accepted")
	}
}
func TestAcquisitionIncompleteSourcesNeverRankAndWeightsAreExplicit(t *testing.T) {
	j, p, o := planFixture()
	j.Result.Upgrades[1].PoolComplete = false
	r, e := planAcquisition(j, p, 64, o)
	if e != nil || len(r.Sources) != 0 {
		t.Fatal("partial pool ranked", r, e)
	}
	j.Result.Upgrades[1].PoolComplete = true
	o.Weights = map[string]float64{"a/1": 0.1, "a/2": 0.9}
	r, e = planAcquisition(j, p, 64, o)
	if e != nil || r.Sources[0].Expected != 1 {
		t.Fatal(r, e)
	}
	o.Weights["a/1"] = -1
	if _, e = planAcquisition(j, p, 64, o); e == nil {
		t.Fatal("bad weight accepted")
	}
}

func TestAcquisitionRejectsUnprovenPoolsAndIncompatibleHistory(t *testing.T) {
	j, p, o := planFixture()
	o.AssumeBonusPool = false
	if _, err := planAcquisition(j, p, 64, o); err == nil {
		t.Fatal("bonus pool treated as a fact")
	}
	o.AssumeBonusPool = true
	o.History = &p.Planning.Rolls[0]
	o.HistoryConfirmed = false
	if _, err := planAcquisition(j, p, 64, o); err == nil {
		t.Fatal("history applied without matching the pool")
	}
	o.HistoryConfirmed = true
	p.Planning.Rolls = append(p.Planning.Rolls, BonusRoll{1, 99, 4, 10, 9999, 64})
	if _, err := planAcquisition(j, p, 64, o); err == nil {
		t.Fatal("incompatible history applied")
	}
	o.History = nil
	j.Result.Upgrades[0].PoolSize = 3
	r, err := planAcquisition(j, p, 64, o)
	if err != nil || len(r.Sources) != 0 {
		t.Fatal("missing outcome removed from denominator", err)
	}
	o.Cost = 1e-300
	if _, err = planAcquisition(j, p, 64, o); err == nil {
		t.Fatal("unbounded planning rate")
	}
}

func TestAcquisitionRerollsDoNotInventIndependentDrawMedian(t *testing.T) {
	j, p, o := planFixture()
	r, err := planAcquisition(j, p, 64, o)
	if err != nil || r.Sources[0].Attempts50 != nil {
		t.Fatal("reroll median used independent draws", err)
	}
	o.Mode = "farm"
	o.Chance = 1e-300
	r, err = planAcquisition(j, p, 64, o)
	if err != nil || r.Sources[0].Attempts50 != nil {
		t.Fatal("unbounded attempt count", err)
	}
}
