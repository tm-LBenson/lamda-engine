package engine

import (
	"strings"
	"testing"
	"time"
)

func TestEstimatePanelFiltersAndKeepsUncertainty(t *testing.T) {
	now := time.Now()
	m := NewModel()
	c := DefaultConfig()
	c.EstimatePanel = true
	m.Observe(Cast{At: now.Add(-15 * time.Second), GUID: "Player-SELF", Name: "Self", Spell: 55342, Self: true}, now)
	m.Observe(Cast{At: now, GUID: "Player-FRIEND", Name: "Friend", Spell: 363916}, now)
	snapshot := m.Snapshot(now)
	p := PanelFor(snapshot, c, "receiving", now, nil, now)
	if len(p.Rows) != 2 || !p.Rows[0].Self || !strings.Contains(p.Rows[0].Detail, "log delay 15s") || p.Rows[1].Status != "estimated_recharge" || !strings.Contains(p.Rows[1].Detail, "charges unknown") {
		t.Fatal(p)
	}
	if p.Rows[0].Ends != snapshot.Cooldowns[1].EstimatedEnd {
		t.Fatal("presentation changed the inferred end time")
	}
	c.EstimateSelf = false
	if p = PanelFor(snapshot, c, "idle", now, nil, now); len(p.Rows) != 1 || p.Rows[0].Self {
		t.Fatal(p)
	}
	if p = PanelFor(snapshot, c, "idle", now, map[string]bool{}, now); len(p.Rows) != 0 {
		t.Fatal("departed party member shown", p)
	}
	c.EstimateSelf = true
	c.EstimateParty = false
	if p = PanelFor(snapshot, c, "idle", now, nil, now); len(p.Rows) != 1 || !p.Rows[0].Self {
		t.Fatal(p)
	}
	c.EstimateSelf = false
	if p = PanelFor(snapshot, c, "idle", now, nil, now); !strings.Contains(p.Empty, "Choose") {
		t.Fatal(p)
	}
	if m.Diagnostics.AcceptedCasts != 2 || len(m.Evidence) != 2 {
		t.Fatal("display filters changed collection")
	}
}

func TestPanelExpiryDoesNotClaimConfirmedReadiness(t *testing.T) {
	now := time.Now()
	m := NewModel()
	c := DefaultConfig()
	c.EstimatePanel = true
	m.Observe(Cast{At: now, GUID: "Player-SELF", Name: "Self", Spell: 55342, Self: true}, now)
	m.Observe(Cast{At: now, GUID: "Player-SELF", Name: "Self", Spell: 363916, Self: true}, now)
	later := now.Add(125 * time.Second)
	snapshot := m.Snapshot(later)
	if p := PanelFor(snapshot, c, "idle", now, nil, later); len(p.Rows) != 0 {
		t.Fatal(p)
	}
	c.EstimateReady = true
	p := PanelFor(snapshot, c, "idle", now, nil, later)
	if len(p.Rows) != 1 || p.Rows[0].Status != "estimated_ready" || p.Rows[0].Spell != 55342 {
		t.Fatal("unknown charges became ready", p)
	}
	if p = PanelFor(m.Snapshot(now.Add(181*time.Second)), c, "idle", now, nil, now.Add(181*time.Second)); len(p.Rows) != 0 {
		t.Fatal("stale evidence displayed as ready", p)
	}
}

func TestPanelStatusAndLimit(t *testing.T) {
	now := time.Now()
	m := NewModel()
	c := DefaultConfig()
	c.EstimatePanel = true
	c.EstimateRows = 1
	for _, id := range []int{55342, 363916, 102342} {
		m.Observe(Cast{At: now, GUID: "Player-A", Name: "A", Spell: id}, now)
	}
	p := PanelFor(m.Snapshot(now), c, "receiving", now, nil, now)
	if len(p.Rows) != 1 || p.Hidden != 2 {
		t.Fatal(p)
	}
	c.TrackingEnabled = false
	if p = PanelFor(m.Snapshot(now), c, "disabled", now, nil, now); len(p.Rows) != 0 || p.Status != "Tracking is off." {
		t.Fatal(p)
	}
	c.TrackingEnabled = true
	if p = PanelFor(NewModel().Snapshot(now), c, "waiting_for_log", time.Time{}, nil, now); !strings.Contains(p.Empty, "/combatlog") {
		t.Fatal(p)
	}
	if p = PanelFor(m.Snapshot(now), c, "read_error", now, nil, now); !strings.Contains(p.Status, "Cannot read") {
		t.Fatal(p)
	}
}

func TestEstimateConfigIsIndependentAndStrict(t *testing.T) {
	base := `LamdaEngineDB = {["kickEnabled"]=false, ["schema"]=1, ["cdEnabled"]=true, ["engineFrameEstimates"]=true, ["nativeFrames"]=true,
["checkUpdates"]=false, ["notifyUpdates"]=false, ["checkDays"]=1,
["overlayX"]=60, ["overlayY"]=240, ["overlayScale"]=1, `
	for _, setting := range []string{`["estimatePanel"]=true`, `["estimateSelf"]=false`, `["estimateRows"]=12`, `["estimateX"]=16000`, `["estimateSize"]=3`} {
		cfg, err := ReadConfig(writeConfigFixture(t, base+setting+"}"))
		if err != nil || cfg.Enabled || !cfg.TrackingEnabled {
			t.Fatal(cfg, err)
		}
	}
	for _, setting := range []string{`["estimatePanel"]=nil`, `["estimateRows"]=13`, `["estimateX"]=-1`, `["estimateY"]=1.5`, `["estimateSize"]=0`} {
		if _, err := ReadConfig(writeConfigFixture(t, base+setting+"}")); err == nil {
			t.Fatal("invalid panel setting accepted", setting)
		}
	}
	cfg, err := ReadConfig(writeConfigFixture(t, base+`["estimatePanel"]=false}`))
	if err != nil || !cfg.EstimateConfigured || cfg.EstimatePanel {
		t.Fatal("explicit opt-out was lost", cfg, err)
	}
	cfg, err = ReadConfig(writeConfigFixture(t, strings.Replace(base, `["cdEnabled"]=true`, `["cdEnabled"]=false`, 1)+`["estimatePanel"]=true}`))
	if err != nil || cfg.EstimatePanel || cfg.TrackingEnabled {
		t.Fatal("module master ignored", cfg, err)
	}
}
