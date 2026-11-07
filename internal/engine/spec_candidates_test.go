package engine

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMissingProfilesStillShowTankAndHealerCandidatesWithoutReadiness(t *testing.T) {
	now := time.Now()
	s := InferenceSnapshot{Builds: []BuildSnapshot{
		{GUID: "Player-1-TANK", At: now, SpecID: 73, Source: "combat_log"},
		{GUID: "Player-1-HEALER", At: now, SpecID: 257, Source: "combat_log"},
		{GUID: "Player-1-SELF", At: now, SpecID: 253, Source: "combat_log"},
	}}
	before := append([]BuildSnapshot{}, s.Builds...)
	items := candidateCooldowns(s, now, "Player-1-SELF")
	spells := map[string][]int{}
	for _, e := range items {
		if e.Source != "spec_candidate" || e.Status != "unknown" || e.EstimatedEnd != 0 || e.BaselineSeconds != 0 || !e.EventAt.IsZero() || e.AvailableCharges != nil || e.MaxCharges != nil || !slices.Contains(e.Reasons, "ability_not_verified") {
			t.Fatal("suggestion invented ability or readiness evidence", e)
		}
		spells[e.GUID] = append(spells[e.GUID], e.Spell)
	}
	if !slices.Contains(spells["Player-1-TANK"], 2565) || !slices.Contains(spells["Player-1-TANK"], 190456) || !slices.Contains(spells["Player-1-HEALER"], 19236) || len(spells["Player-1-SELF"]) != 0 || !reflect.DeepEqual(before, s.Builds) || len(s.Cooldowns) != 0 {
		t.Fatal(spells, s)
	}
}

func TestCandidatesNeedFreshSupportedSpecializationAndRespectObservations(t *testing.T) {
	now := time.Now()
	for _, b := range []BuildSnapshot{
		{GUID: "Player-1-FRIEND", At: now.Add(time.Second), SpecID: 73},
		{GUID: "Player-1-FRIEND", At: now.Add(-observationRetention), SpecID: 73},
		{GUID: "Player-1-FRIEND", SpecID: 73},
		{GUID: "Player-1-FRIEND", At: now, SpecID: 99999},
	} {
		if got := candidateCooldowns(InferenceSnapshot{Builds: []BuildSnapshot{b}}, now, ""); len(got) != 0 {
			t.Fatal(b, got)
		}
	}
	s := InferenceSnapshot{Builds: []BuildSnapshot{{GUID: "Player-1-FRIEND", At: now, SpecID: 266}},
		Cooldowns: []CooldownEvidence{{GUID: "Player-1-FRIEND", Spell: 104773, Status: "estimated_cooldown"}}}
	items := candidateCooldowns(s, now, "")
	if len(items) != 1 || items[0].Spell != 108416 {
		t.Fatal("observed spell duplicated", items)
	}
	for spec, ids := range specializationDefensives {
		seen := map[int]bool{}
		for _, id := range ids {
			if _, ok := Catalog[id]; !ok || seen[id] {
				t.Fatal("invalid candidate catalog", spec, id)
			}
			seen[id] = true
		}
	}
}

func TestVerifiedLocalAndPublicBuildUsesSelectedSpellsInsteadOfCandidates(t *testing.T) {
	now := time.Now()
	s := knownFixture(now)
	s.Builds = []BuildSnapshot{{GUID: "Player-1-FRIEND", At: now, SpecID: 1468, Talents: [][3]int{{1, 10, 1}}}}
	if items := candidateCooldowns(s, now, ""); len(items) != 0 {
		t.Fatal("verified talent selection got unselected suggestions", items)
	}
	s.PublicBuilds[0].SpecID = 1467
	if items := candidateCooldowns(s, now, ""); len(items) == 0 {
		t.Fatal("old specialization suppressed fallback")
	}
}

func TestCandidateFrameVisibilityAndObservedPriority(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	g := frameFixture(now)
	g.Persistent = true
	g.Unknown = true
	s := InferenceSnapshot{Builds: []BuildSnapshot{{GUID: "Player-1-FRIEND", At: now, SpecID: 257}}}
	d := FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 2 || d.Icons[0].Source != "spec_candidate" {
		t.Fatal("missing priest fallback", d)
	}
	for _, gate := range []string{"persistent", "unknown", "party", "selection"} {
		next := *g
		config := c
		switch gate {
		case "persistent":
			next.Persistent = false
		case "unknown":
			next.Unknown = false
		case "party":
			next.Party = false
		case "selection":
			config.FrameSpells = []int{}
		}
		if got := FrameView(s, config, &next, now, now, 123); len(got.Icons) != 0 {
			t.Fatal("visibility ignored", gate, got)
		}
	}
	s.Cooldowns = []CooldownEvidence{{GUID: "Player-1-FRIEND", Spell: 47788, Source: "combat_log", Status: "unknown"}}
	g.Maximum = 1
	d = FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 1 || d.Icons[0].Spell != 47788 || d.Icons[0].Source != "combat_log" {
		t.Fatal("suggestion displaced observed ability", d)
	}
}

func TestTankMitigationRecordsCastsWithoutInventingResourceOrChargeReadiness(t *testing.T) {
	now := time.Now()
	m := NewModel()
	c := Cast{GUID: "Player-1-TANK", Name: "Tank", Spell: 190456, At: now}
	if m.Observe(c, now) {
		t.Fatal("Ignore Pain created an active recharge row")
	}
	for _, at := range []time.Time{now, now.Add(time.Second), now.Add(time.Minute)} {
		e := m.Snapshot(at).Cooldowns[0]
		if e.Status != "unknown" || e.EstimatedEnd != 0 || e.BaselineSeconds != 0 || !slices.Contains(e.Reasons, "resource_based_ability") {
			t.Fatal(e)
		}
	}
	c.Spell = 2565
	if !m.Observe(c, now) {
		t.Fatal("Shield Block cast ignored")
	}
	for _, e := range m.Snapshot(now.Add(17 * time.Second)).Cooldowns {
		if e.Spell == 2565 && (e.Status != "unknown" || !e.MayHaveCharges || e.AvailableCharges != nil) {
			t.Fatal("unobserved Shield Block charge count invented", e)
		}
	}
}

func TestDiskDefaultsMigrateWithoutWaitingForSecondReload(t *testing.T) {
	legacy := "48707,51052,49028,48792,49039,55233,198589,196718,204021,187827,22812,102342,61336,363916,363534,357170,374227,374348,186265,109304,264735,110959,45438,55342,115203,116849,122470,31850,1022,6940,498,642,86659,633,19236,47585,47788,33206,62618,31224,5277,108271,98008,108416,104773,118038,184364,97462,871"
	base := `LamdaEngineDB = {["schema"]=1,["cdEnabled"]=true,["nativeFrames"]=true,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,`
	for _, list := range []string{legacy, strings.Replace(legacy, ",374348", "", 1)} {
		c, err := ReadConfig(writeConfigFixture(t, base+`["engineFrameSpells"]=`+strconv.Quote(list)+`}`))
		if err != nil || !slices.Contains(c.FrameSpells, 2565) || !slices.Contains(c.FrameSpells, 190456) {
			t.Fatal(c, err)
		}
		c, err = ReadConfig(writeConfigFixture(t, base+`["defensiveCatalogVersion"]=1,["engineFrameSpells"]=`+strconv.Quote(list)+`}`))
		if err != nil || slices.Contains(c.FrameSpells, 2565) {
			t.Fatal("later deliberate removal was undone", c, err)
		}
	}
	for _, list := range []string{"", "871", strings.TrimPrefix(legacy, "48707,")} {
		c, err := ReadConfig(writeConfigFixture(t, base+`["engineFrameSpells"]=`+strconv.Quote(list)+`}`))
		if err != nil || slices.Contains(c.FrameSpells, 2565) {
			t.Fatal("custom selection changed", c, err)
		}
	}
}
