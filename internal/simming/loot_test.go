package simming

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func lootLink(fields ...int) string {
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = strconv.Itoa(field)
	}
	return "|cffa335ee|Hitem:" + strings.Join(parts, ":") + "|h[Test item]|h|r"
}

func lootFixture() LootItem {
	return LootItem{RulesKnown: true, ID: 124, Label: "Test item", Slot: "head", Level: 289, InventoryType: "INVTYPE_HEAD", Link: lootLink(124, 555, 1, 0, 2, 0, 0, 0, 90, 64, 0, 16, 2, 123, 456, 3, 9, 90, 28, 1234, 64, 567, 0, 1, 789)}
}

func TestLootLinkPreservesBonusesModifiersAndPreviewLevel(t *testing.T) {
	item := lootFixture()
	value, err := item.SimCValue()
	if err != nil {
		t.Fatal(err)
	}
	want := "id=124,enchant_id=555,gem_id=1/0/2,bonus_id=123/456,drop_level=90,content_tuning=1234,redirected_base_stats=567,gem_bonus_id=789,ilevel=289"
	if value != want {
		t.Fatalf("got %s, want %s", value, want)
	}
	for _, link := range []string{
		"item:124:0", "item:124::::::::::::2:100", strings.Replace(item.Link, "item:124", "item:125", 1),
		item.Link + "\noutput=unsafe", "item:124:0:0:0:0:0:1:0:90:64:0:16:0", "item:124:0:0:0:0:0:0:0:90:64:0:16:0:4:9:90",
	} {
		item.Link = link
		if _, err := item.SimCValue(); err == nil {
			t.Fatalf("accepted malformed link: %q", link)
		}
	}
}

func TestOldDropDisplayRetainsExactCapturedLinkAndWinningSlotWithoutMutatingHistory(t *testing.T) {
	s := fixture(time.Now())
	item := lootFixture()
	item.Slot = "finger1"
	item.InventoryType = "INVTYPE_FINGER"
	pool := LootPool{InstanceID: 123, EncounterID: 456, Type: "Raid", DifficultyID: 15, Name: "Captured source", Items: []LootItem{item}}
	s.Loot = &LootCatalog{Pools: []LootPool{pool}}
	original := &Result{Metric: "DPS", Baseline: Metric{Mean: 1000}, Upgrades: []Upgrade{{ID: item.ID, Slot: "finger2", Level: 300, Type: "max", SourceID: pool.SourceID(), Mean: 1100, Gain: 10}}}
	display := upgradeDisplayResult(s, original)
	actual := display.Upgrades[0]
	if actual.Link != item.Link || actual.CapturedLevel != item.Level || actual.Slot != "finger2" || len(actual.Items) != 1 || actual.Items[0].Slot != "finger2" || itemField(actual.Key, "ilevel") != "300" || actual.Gain != 10 || actual.Mean != 1100 {
		t.Fatalf("captured variant/placement lost: %+v", actual)
	}
	if original.Upgrades[0].Link != "" || len(original.Upgrades[0].Items) != 0 {
		t.Fatal("historical result mutated")
	}
}

func TestLootCatalogCaptureScopeAndCompleteness(t *testing.T) {
	s := fixture(time.Now())
	catalog := &LootCatalog{Schema: 1, Spec: 64, Season: 37, CapturedAt: s.CapturedAt, Pools: []LootPool{{InstanceID: 123, EncounterID: 456, Type: "Raid", DifficultyID: 15, Name: "Test boss", Complete: true, Items: []LootItem{lootFixture()}}}}
	if err := ValidateLoot(catalog, s); err != nil {
		t.Fatal(err)
	}
	catalog.Pools[0].Complete = false
	if err := ValidateLoot(catalog, s); err != nil {
		t.Fatal("incomplete data must remain representable", err)
	}
	catalog.Pools[0].Items = append(catalog.Pools[0].Items, lootFixture())
	if err := ValidateLoot(catalog, s); err == nil {
		t.Fatal("duplicate item alters pool probabilities")
	}
	catalog.Pools[0].Items = catalog.Pools[0].Items[:1]
	catalog.Spec = 63
	if err := ValidateLoot(catalog, s); err == nil {
		t.Fatal("different loot spec accepted")
	}
	catalog.Spec = 64
	catalog.CapturedAt -= 301
	if err := ValidateLoot(catalog, s); err == nil {
		t.Fatal("stale capture accepted")
	}
	catalog.CapturedAt = s.CapturedAt
	s.Client = "forever"
	if err := ValidateLoot(catalog, s); err == nil {
		t.Fatal("retail loot leaked into Forever")
	}
}

func TestLootMythicPreviewEvidenceRejectsMislabeledDifficultyAndLevels(t *testing.T) {
	s := fixture(time.Now())
	base := LootPool{InstanceID: 123, ChallengeMapID: 456, Type: "Dungeon", Name: "Preview dungeon", DifficultyID: 8, JournalDifficultyID: 8, KeyLevel: 10, Complete: true, Items: []LootItem{lootFixture()}}
	for _, level := range []int{0, 289} {
		pool := base
		pool.PreviewRewardLevel = level
		s.Loot = &LootCatalog{Schema: 1, Spec: 64, Season: 37, CapturedAt: s.CapturedAt, Pools: []LootPool{pool}}
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeSnapshot(data, "retail")
		if err != nil || got.Loot.Pools[0].JournalDifficultyID != 8 || got.Loot.Pools[0].PreviewRewardLevel != level {
			t.Fatalf("native preview evidence lost: %v", err)
		}
	}
	for name, mutate := range map[string]func(*LootPool){
		"M0 readback":             func(p *LootPool) { p.JournalDifficultyID = 23 },
		"M0 source":               func(p *LootPool) { p.DifficultyID = 23 },
		"M0 key":                  func(p *LootPool) { p.KeyLevel = 0 },
		"key one":                 func(p *LootPool) { p.KeyLevel = 1 },
		"raid evidence":           func(p *LootPool) { p.Type = "Raid"; p.EncounterID = 456; p.KeyLevel = 0 },
		"negative reward":         func(p *LootPool) { p.PreviewRewardLevel = -1 },
		"oversized reward":        func(p *LootPool) { p.PreviewRewardLevel = 2001 },
		"wrong item level":        func(p *LootPool) { p.PreviewRewardLevel = 290 },
		"reward without readback": func(p *LootPool) { p.JournalDifficultyID = 0; p.PreviewRewardLevel = 289 },
	} {
		t.Run(name, func(t *testing.T) {
			pool := base
			mutate(&pool)
			catalog := &LootCatalog{Schema: 1, Spec: 64, Season: 37, CapturedAt: s.CapturedAt, Pools: []LootPool{pool}}
			if err := ValidateLoot(catalog, s); err == nil {
				t.Fatal("accepted conflicting preview evidence")
			}
		})
	}
}

func TestLootWeaponAndAccessoryPlacements(t *testing.T) {
	p, err := ParseProfile(profileFixture)
	if err != nil {
		t.Fatal(err)
	}
	item := lootFixture()
	item.Slot, item.InventoryType = "main_hand", "INVTYPE_2HWEAPON"
	variants := lootComparisons(p, item, "staff")
	if len(variants) != 1 || len(variants[0].Items) != 2 || variants[0].Items[1].Slot != "off_hand" || variants[0].Items[1].Value != "" {
		t.Fatalf("staff did not clear offhand: %+v", variants)
	}
	input, err := lootInput(p, DefaultScenario(), variants)
	if err != nil || !strings.Contains(input, "profileset.staff_0+=off_hand=\n") {
		t.Fatalf("offhand clear was not generated: %v %s", err, input)
	}
	p.Class, p.Spec = "warrior", "fury"
	variants = lootComparisons(p, item, "weapon")
	if len(variants) != 2 || len(variants[0].Items) != 1 || variants[1].Items[0].Slot != "off_hand" {
		t.Fatal("Fury two-handed weapon placements were lost")
	}
	item.Slot, item.InventoryType = "finger1", "INVTYPE_FINGER"
	if variants = lootComparisons(p, item, "ring"); len(variants) != 2 || variants[1].Items[0].Slot != "finger2" {
		t.Fatal("ring second position missing")
	}
}

func TestLootRunnerIsolatesRejectedItemsAndKeepsIncompleteSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a Unix Python launcher; native SimC checked separately")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 test helper unavailable")
	}
	dir := t.TempDir()
	worker := []byte("#!" + python + `
import json,re,sys
text=open('input.simc').read()
if 'id=999999' in text:
 print('Rejected fixture item');sys.exit(82)
sets={}
for line in text.splitlines():
 if line.startswith('profileset.'):
  key,value=line.split('=',1);key=key.removeprefix('profileset.').rstrip('+')
  sets.setdefault(key,[]).append(value)
gear={}
for line in text.splitlines():
 if not line.startswith('profileset.') and '=' in line:
  slot,value=line.split('=',1)
  if re.search(r'(?:^|,)id=\d+',value): gear[slot]={'encoded_item':value}
open('verified-gear.simc','w').write('\n'.join(slot+'=fixture_item,'+item['encoded_item'].lstrip(',') for slot,item in gear.items()))
results=[]
for key,values in sets.items():
 overrides={}
 for line in values:
  slot,value=line.split('=',1);ident=re.search(r'(?:^|,)id=(\d+)',value);level=re.search(r'ilevel=(\d+)',value)
  if ident: overrides[slot]={'item_id':int(ident.group(1)),'item_level':int(level.group(1)) if level else 0}
 results.append({'name':key,'mean':100+sum(int(v)%100 for v in re.findall(r'id=(\d+)', '\n'.join(values)))/10,'stddev':0,'mean_error':0,'overrides':{'gear':overrides}})
json.dump({'sim':{'players':[{'gear':gear,'collected_data':{'dps':{'mean':100}}}],'profilesets':{'results':results}}},open('result.json','w'))
`)
	path := filepath.Join(dir, "worker.py")
	if err = os.WriteFile(path, worker, 0700); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(worker)
	p := &Provider{Config: ProviderConfig{Path: path, Version: "fixture", SHA256: hex.EncodeToString(hash[:])}}
	s := fixture(time.Now())
	s.Rules = []ItemRule{{ID: 123, BonusIDs: []int{1, 2}, InventoryType: "INVTYPE_WEAPONMAINHAND"}}
	good, bad := lootFixture(), lootFixture()
	bad.ID, bad.Link = 999999, lootLink(999999, 0, 0, 0, 0, 0, 0, 0, 90, 64, 0, 16, 0)
	s.Loot = &LootCatalog{Schema: 1, Spec: 64, CapturedAt: s.CapturedAt, Season: 37, Pools: []LootPool{
		{InstanceID: 123, EncounterID: 456, Name: "Incomplete boss", Type: "Raid", DifficultyID: 15, Complete: true, Items: []LootItem{good, bad}},
		{InstanceID: 123, EncounterID: 457, Name: "Complete boss", Type: "Raid", DifficultyID: 15, Complete: true, Items: []LootItem{good}},
	}}
	scenario := DefaultScenario()
	scenario.Task = "upgrades"
	result, err := p.Run(context.Background(), s, scenario, nil, filepath.Join(dir, "run"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Upgrades) != 2 || result.TargetSet == nil {
		t.Fatalf("invalid partial result: %+v", result)
	}
	for _, item := range result.Upgrades {
		if item.Link != good.Link || item.Key == "" || item.CapturedLevel != good.Level || len(item.Items) == 0 || item.Items[0].Slot != item.Slot {
			t.Fatalf("exact drop tooltip or winning equipment placement lost: %+v", item)
		}
		if !item.LootVerified || item.Type != "drop" || item.Kind != "Raid" || item.PoolComplete != (item.Source == "Complete boss") {
			t.Fatalf("source completeness incorrect: %+v", item)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "run", "items", "retry-loot0_1_drop_0", "worker.log")); err != nil {
		t.Fatal("failed retry log not retained", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.Run(ctx, s, scenario, nil, filepath.Join(dir, "cancelled"))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(fmt.Sprintf("cancelled job returned %v", err))
	}
}

// Optional real-worker check. The supplied snapshot must be a non-personal
// reference profile, and the synthetic source is never imported into user data.
func TestLootRealSimulationCraft(t *testing.T) {
	configPath, snapshotPath := os.Getenv("LAMDA_SIMC_TEST_PROVIDER"), os.Getenv("LAMDA_SIMC_TEST_SNAPSHOT")
	if configPath == "" || snapshotPath == "" {
		t.Skip("set LAMDA_SIMC_TEST_PROVIDER and LAMDA_SIMC_TEST_SNAPSHOT for real-worker integration")
	}
	var provider Provider
	var snapshot Snapshot
	for path, value := range map[string]any{configPath: &provider.Config, snapshotPath: &snapshot} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, value); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.HasPrefix(snapshot.ID, "reference_") {
		t.Fatal("real-worker fixture must be a reference snapshot")
	}
	snapshot.CapturedAt = time.Now().Unix()
	snapshot.Loot = &LootCatalog{Schema: 1, Spec: snapshot.Character.Spec, Season: 0, CapturedAt: snapshot.CapturedAt, Pools: []LootPool{{InstanceID: 999, EncounterID: 999, Name: "Integration fixture, not actual loot", Type: "Raid", DifficultyID: 15, Complete: true, Items: []LootItem{
		{ID: 271564, Label: "Fixture head", Slot: "head", Level: 321, InventoryType: "INVTYPE_HEAD", Link: lootLink(271564, 0, 0, 0, 0, 0, 0, 0, 90, 64, 0, 16, 5, 13692, 13698, 13750, 13846, 13848, 1, 64, 271874)},
		{ID: 251136, Label: "Fixture ring", Slot: "finger1", Level: 321, InventoryType: "INVTYPE_FINGER", Link: lootLink(251136, 0, 0, 0, 0, 0, 0, 0, 90, 64, 0, 16, 3, 4786, 12854, 13750)},
		{ID: 260482, MaxLevel: 334, Label: "Fixture staff", Slot: "main_hand", Level: 321, InventoryType: "INVTYPE_2HWEAPON", Link: lootLink(260482, 0, 0, 0, 0, 0, 0, 0, 90, 64, 0, 16, 0)},
	}}}}
	profile, err := ParseProfile(snapshot.Profile)
	if err != nil {
		t.Fatal(err)
	}
	for slot, candidate := range baseGear(profile) {
		inv := ""
		for inventory, target := range lootInventoryTypes {
			if target == slot {
				inv = inventory
				break
			}
		}
		if slot == "main_hand" {
			inv = "INVTYPE_WEAPONMAINHAND"
		}
		if slot == "off_hand" {
			inv = "INVTYPE_HOLDABLE"
		}
		if slot == "finger2" {
			inv = "INVTYPE_FINGER"
		}
		if slot == "trinket2" {
			inv = "INVTYPE_TRINKET"
		}
		parts := strings.Split(valueRuleKey(candidate.Value), ":")
		id, _ := strconv.Atoi(parts[0])
		rule := ItemRule{ID: id, InventoryType: inv}
		for _, part := range parts[1:] {
			n, _ := strconv.Atoi(part)
			rule.BonusIDs = append(rule.BonusIDs, n)
		}
		snapshot.Rules = append(snapshot.Rules, rule)
	}
	for i := range snapshot.Loot.Pools[0].Items {
		snapshot.Loot.Pools[0].Items[i].RulesKnown = true
	}
	scenario := DefaultScenario()
	scenario.Task, scenario.Iterations, scenario.Seconds = "upgrades", 100, 30
	output := os.Getenv("LAMDA_SIMC_TEST_OUTPUT")
	if output == "" {
		output = t.TempDir()
	}
	result, err := provider.Run(context.Background(), &snapshot, scenario, nil, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.Baseline.Mean <= 0 || len(result.Upgrades) != 4 || result.TargetSet == nil {
		t.Fatalf("actual worker did not model all fixture items: %+v", result)
	}
	foundMax := false
	for _, item := range result.Upgrades {
		if item.Type == "max" && item.ID == 260482 && item.Level == 334 {
			foundMax = true
		}
	}
	if !foundMax {
		t.Fatal("client-reported upgrade cap was not modeled")
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(filepath.Join(output, "lamda-result.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("Actual SimC: %d items, %d target tests, baseline %.2f DPS, target %.2f DPS; output %s", len(result.Upgrades), result.SearchTests, result.Baseline.Mean, result.TargetSet.Mean, output)
	// A local item-DB miss must reject only that comparison, with the baseline
	// and a valid item still reported. This exercises the real worker's exit 82.
	snapshot.Profile += "\n### Gear from Bags\n# Valid integration item\n# head=,id=271564,ilevel=321\n# Missing integration item\n# head=,id=9999999\n"
	snapshot.Rules = append(snapshot.Rules, ItemRule{ID: 271564, InventoryType: "INVTYPE_HEAD"}, ItemRule{ID: 9999999, InventoryType: "INVTYPE_HEAD"})
	parsed, err := ParseProfile(snapshot.Profile)
	if err != nil {
		t.Fatal(err)
	}
	scenario.Task = ""
	candidates := parsed.Candidates[len(parsed.Candidates)-2:]
	partial, err := provider.Run(context.Background(), &snapshot, scenario, []string{candidates[0].ID, candidates[1].ID}, filepath.Join(output, "selected-gear-retry"))
	if err != nil || len(partial.Comparisons) != 1 || len(partial.RejectedComparisons) != 1 {
		t.Fatalf("actual item error was not isolated: %+v %v", partial, err)
	}
	t.Log("Actual SimC selected-gear retry kept one valid comparison and identified the rejected local-DB item.")
}
