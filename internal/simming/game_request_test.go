package simming

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func gameFixture() *Snapshot {
	s := fixture(time.Now())
	s.GameRequest = &GameRequest{Schema: 1, ID: s.ID, Task: "baseline", Scenario: DefaultScenario()}
	return s
}
func TestGameRequestValidationAndItemIdentity(t *testing.T) {
	s := gameFixture()
	s.GameRequest.Task = "compare"
	s.GameRequest.Selected = []GameSelection{{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3"}}
	data, _ := json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err != nil {
		t.Fatal(err)
	}
	// Insert another item before the selected item: stored item1 must not be used.
	s.Profile = strings.Replace(s.Profile, "# Bag staff", "# Another staff\n# main_hand=,id=888\n# Bag staff", 1)
	ids, err := resolveGameSelection(s)
	if err != nil || len(ids) != 1 || ids[0] != "item2" {
		t.Fatalf("remap: %v %v", ids, err)
	}
	s.GameRequest.Selected[0].Source = "vault"
	if _, err = resolveGameSelection(s); err == nil {
		t.Fatal("wrong source accepted")
	}
	for _, edit := range []func(*GameRequest){
		func(r *GameRequest) { r.ID = "other" }, func(r *GameRequest) { r.Task = "execute" },
		func(r *GameRequest) { r.Scenario.Task = "upgrades" }, func(r *GameRequest) { r.Talents = []TalentBuild{{Label: "x", Code: "abc"}} },
		func(r *GameRequest) { r.Task = "compare" }, func(r *GameRequest) { r.JobID = "../escape" },
	} {
		bad := gameFixture()
		edit(bad.GameRequest)
		raw, _ := json.Marshal(bad)
		if _, err := DecodeSnapshot(raw, "retail"); err == nil {
			t.Fatal("bad request accepted")
		}
	}
}

func TestGameAnalysisRequestRequiresNoManualSelection(t *testing.T) {
	s := gameFixture()
	s.GameRequest.Task = "analysis"
	s.GameRequest.Talents = []TalentBuild{{Label: "Saved raid", Code: "RAID=="}}
	data, _ := json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err != nil {
		t.Fatal(err)
	}
	s.GameRequest.Selected = []GameSelection{{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3"}}
	data, _ = json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("analysis accepted a manual selection")
	}
}

func TestGameLoadoutAllowsOneTalentAndEighteenExactReplacements(t *testing.T) {
	s := gameFixture()
	s.GameRequest.Task = "compare"
	s.GameRequest.CompareMode = "loadout"
	s.GameRequest.Talents = []TalentBuild{{Label: "Planned", Code: "PLAN=="}}
	s.GameRequest.Selected = []GameSelection{{Source: "owned", Slot: "main_hand", Placement: "off_hand", Value: "id=124,bonus_id=2/3"}}
	data, _ := json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err != nil {
		t.Fatal(err)
	}
	s.GameRequest.Talents = append(s.GameRequest.Talents, TalentBuild{Label: "Second", Code: "SECOND=="})
	data, _ = json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("loadout accepted more than one talent build")
	}
	s.GameRequest.Talents = nil
	s.GameRequest.Selected = nil
	data, _ = json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("empty loadout accepted")
	}
	s.GameRequest.Selected = []GameSelection{{Source: "owned", Slot: "main_hand", Placement: "bad_slot", Value: "id=124,bonus_id=2/3"}}
	data, _ = json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("loadout accepted an invalid exact placement")
	}
}

func TestGameLoadoutAcceptsOnlyCapturedPlannedRanks(t *testing.T) {
	s := gameFixture()
	s.GameRequest.Task = "compare"
	s.GameRequest.CompareMode = "loadout"
	s.GameRequest.Selected = []GameSelection{{Source: "owned", Slot: "main_hand", Placement: "main_hand", PlannedLevel: 300, Value: "id=124,bonus_id=2/3"}}
	s.Upgrades = []ItemUpgrade{{Source: "owned", Slot: "main_hand", Value: "id=124,bonus_id=2/3", Label: "Bag staff", Level: 289, NextLevel: 292, MaxLevel: 300, Track: "Hero", CurrentRank: 4, MaxRank: 6, RankLevels: []int{289, 292, 300}}}
	data, _ := json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err != nil {
		t.Fatal(err)
	}
	ids, err := resolveGameSelection(s)
	if err != nil || len(ids) != 1 || ids[0] != "item1" {
		t.Fatalf("planned rank did not resolve to the captured item: %v %v", ids, err)
	}
	s.GameRequest.Selected[0].PlannedLevel = 299
	if _, err = resolveGameSelection(s); err == nil || !strings.Contains(err.Error(), "rank") {
		t.Fatalf("uncaptured planned rank accepted: %v", err)
	}
	s.GameRequest.Selected[0].PlannedLevel = 300
	s.GameRequest.CompareMode = "individual"
	data, _ = json.Marshal(s)
	if _, err = DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("planned item level accepted outside a loadout")
	}
}

func TestWorkspaceSummaryIncludesEveryCapturedEquippedSlot(t *testing.T) {
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := fixture(time.Now())
	s.Profile = strings.Replace(s.Profile, "main_hand=,id=123,bonus_id=1/2", "head=,id=777,ilevel=300\nmain_hand=,id=123,bonus_id=1/2", 1)
	s.Items = []Item{{Slot: 1, Link: "|Hitem:777|h[Captured Helm]|h"}, {Slot: 16, Link: "|Hitem:123|h[Captured Staff]|h"}}
	s.Rules = append(s.Rules, ItemRule{ID: 777, InventoryType: "INVTYPE_HEAD"}, ItemRule{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_2HWEAPON"})
	if ok, err := w.Receive(s); !ok || err != nil {
		t.Fatal(ok, err)
	}
	equipped := w.State().Snapshots[0].Equipped
	if len(equipped) != 2 || equipped[0].Slot != "head" || equipped[0].Label != "Captured Helm" || equipped[0].Link == "" || equipped[0].InventoryType != "INVTYPE_HEAD" || equipped[1].Slot != "main_hand" || equipped[1].InventoryType == "" {
		t.Fatalf("complete equipped set missing: %+v", equipped)
	}
}

func TestCapturedItemUpgradeMetadataIsBoundedAndValidated(t *testing.T) {
	s := gameFixture()
	s.Upgrades = []ItemUpgrade{{Source: "equipped", Slot: "main_hand", Value: "id=123,bonus_id=1/2", Label: "Equipped staff", Level: 289, NextLevel: 292, MaxLevel: 300, Track: "Hero", CurrentRank: 4, MaxRank: 6, RankLevels: []int{289, 292, 300}}}
	data, _ := json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err != nil {
		t.Fatal(err)
	}
	s.Upgrades[0].RankLevels[1] = 293
	data, _ = json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("rank path inconsistent with next level was accepted")
	}
	s.Upgrades[0].RankLevels[1] = 292
	for _, next := range []int{289, 301} {
		s.Upgrades[0].NextLevel = next
		data, _ = json.Marshal(s)
		if _, err := DecodeSnapshot(data, "retail"); err == nil {
			t.Fatalf("invalid next item level %d accepted", next)
		}
	}
	s.Upgrades[0].NextLevel = 292
	s.Upgrades[0].MaxLevel = 288
	data, _ = json.Marshal(s)
	if _, err := DecodeSnapshot(data, "retail"); err == nil {
		t.Fatal("invalid maximum item level accepted")
	}
}
func TestGameRequestRunsOnceAndExportsNamedResults(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "success")
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := gameFixture()
	s.Upgrades = []ItemUpgrade{{Source: "equipped", Slot: "main_hand", Value: "id=123,bonus_id=1/2", Label: "Equipped staff", Level: 289, NextLevel: 292, MaxLevel: 300, Track: "Hero", CurrentRank: 4, MaxRank: 6, RankLevels: []int{289, 292, 300}}}
	fresh, err := w.ReceiveGame(s)
	if !fresh || err != nil {
		t.Fatal(fresh, err)
	}
	state := w.State()
	if len(state.Jobs) != 1 {
		t.Fatal(state.Jobs)
	}
	if len(state.Snapshots) != 1 || len(state.Snapshots[0].Equipped) != 1 || state.Snapshots[0].Equipped[0].NextLevel != 292 || state.Snapshots[0].Equipped[0].MaxLevel != 300 || state.Snapshots[0].Equipped[0].Track != "Hero" || len(state.Snapshots[0].Equipped[0].RankLevels) != 3 {
		t.Fatalf("equipped upgrade path missing from workspace state: %+v", state.Snapshots)
	}
	j := waitJob(t, w, state.Jobs[0].ID)
	if j.State != "complete" || j.RequestID != s.ID || j.GameTask != "baseline" {
		t.Fatal(j)
	}
	if fresh, err = w.ReceiveGame(s); fresh || err != nil || len(w.State().Jobs) != 1 {
		t.Fatal("replayed request reran", err)
	}
	s.GameRequest.Talents = []TalentBuild{{Label: "My dungeon build", Code: "abc"}}
	r := &Result{Comparisons: []Metric{{Name: "talent1", Label: "Talent build 1"}, {Name: "item1", Label: "Gear"}}}
	labelGameTalents(s, r)
	if r.Comparisons[0].Label != "My dungeon build" || r.Comparisons[1].Label != "Gear" {
		t.Fatal(r)
	}
}
func TestGameRejectionPersistsAndCancelCannotCrossCharacter(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "sleep")
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := gameFixture()
	if _, err = w.ReceiveGame(s); err != nil {
		t.Fatal(err)
	}
	active := w.State().Jobs[0]
	other := gameFixture()
	other.ID = "another_character"
	other.Character.GUID = "Player-2-OTHER"
	other.GameRequest.ID = other.ID
	other.GameRequest.Task = "cancel"
	other.GameRequest.JobID = active.ID
	if fresh, err := w.ReceiveGame(other); !fresh || err == nil {
		t.Fatal("cross-character control accepted", err)
	}
	jobs := w.State().Jobs
	if jobs[0].State != "failed" || jobs[0].RequestID != other.ID || !strings.Contains(jobs[0].Error, "another character") {
		t.Fatal(jobs)
	}
	cancel := gameFixture()
	cancel.ID = "cancel_request"
	cancel.GameRequest.ID = cancel.ID
	cancel.GameRequest.Task = "cancel"
	cancel.GameRequest.JobID = active.ID
	if _, err = w.ReceiveGame(cancel); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, w, active.ID); j.State != "cancelled" {
		t.Fatal(j)
	}
}
func TestGameMissingItemIsVisibleFailureNotSilentBaseline(t *testing.T) {
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := gameFixture()
	s.GameRequest.Task = "compare"
	s.GameRequest.Selected = []GameSelection{{Source: "owned", Slot: "head", Value: "id=999999"}}
	if fresh, err := w.ReceiveGame(s); !fresh || err == nil {
		t.Fatal(fresh, err)
	}
	jobs := w.State().Jobs
	if len(jobs) != 1 || jobs[0].State != "failed" || jobs[0].Result != nil || jobs[0].GameTask != "compare" {
		t.Fatal(jobs)
	}
	w2, err := NewWorkspace(context.Background(), w.dir, "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	if len(w2.State().Jobs) != 1 || w2.State().Jobs[0].Error == "" {
		t.Fatal("rejection lost on restart")
	}
}
