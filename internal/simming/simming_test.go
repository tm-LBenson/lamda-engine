package simming

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

const profileFixture = `mage="Test"
level=90
race=human
role=attack
spec=frost
talents=ABCD+ef/==
main_hand=,id=123,bonus_id=1/2
### Gear from Bags
# Bag staff
# main_hand=,id=124,bonus_id=2/3
### Weekly Reward Choices
# Vault staff
# main_hand=,id=125
`

func fixture(now time.Time) *Snapshot {
	return &Snapshot{Schema: 1, ID: "player_123", Client: "retail", CapturedAt: now.Unix(), Build: "test-build", Interface: 120100, Character: Character{GUID: "Player-1-TEST", Name: "Test", Class: "MAGE", Spec: 64, Role: "DAMAGER", Level: 90}, Profile: profileFixture, Warnings: []string{}}
}
func chunkPackets(t *testing.T, s *Snapshot) [][]byte {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var chunks [][]byte
	count := (len(data) + ChunkSize - 1) / ChunkSize
	for i := 0; i < count; i++ {
		body := fmt.Sprintf("1|%s|%s|%d|%d|%d|%d|%d\n", s.ID, s.Client, s.CapturedAt, len(data), i+1, count, crc32.ChecksumIEEE(data)) + string(data[i*ChunkSize:min((i+1)*ChunkSize, len(data))])
		packet := make([]byte, lookup.BridgeBytes)
		copy(packet, "LUS1")
		binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
		binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE([]byte(body)))
		copy(packet[10:], body)
		chunks = append(chunks, packet)
	}
	return chunks
}
func TestTransferReorderingLossReplayIsolation(t *testing.T) {
	now := time.Now()
	s := fixture(now)
	s.Profile += strings.Repeat("# UTF-8: é\n", 500)
	packets := chunkPackets(t, s)
	a := Assembler{Client: "retail"}
	for i := len(packets) - 1; i > 0; i-- {
		v, err := a.Accept(lookup.DecodeSample(packets[i]).Sim, now)
		if err != nil || v != nil {
			t.Fatalf("partial: %v %v", v, err)
		}
	}
	first := lookup.DecodeSample(packets[0])
	if first.Event.ID != "" || first.Frames != nil || first.Observations != nil {
		t.Fatal("sim packet leaked into tracking")
	}
	got, err := a.Accept(first.Sim, now)
	if err != nil || got == nil || got.Profile != s.Profile {
		t.Fatalf("reassembly: %v %v", got, err)
	}
	for _, p := range packets {
		got, err = a.Accept(lookup.DecodeSample(p).Sim, now)
		if err != nil || got != nil {
			t.Fatal("replay accepted")
		}
	}
	other := Assembler{Client: "forever"}
	if _, err = other.Accept(first.Sim, now); err == nil {
		t.Fatal("wrong client accepted")
	}
	expired := Assembler{Client: "retail"}
	if _, err = expired.Accept(first.Sim, now.Add(TransferTTL+time.Second)); err == nil {
		t.Fatal("expired accepted")
	}
	packets[0][20] ^= 1
	if lookup.DecodeSample(packets[0]).Error != "bridge_checksum" {
		t.Fatal("corrupt packet accepted")
	}
}
func TestTransferRejectsConflictingAndOversizedChunks(t *testing.T) {
	now := time.Now()
	s := fixture(now)
	s.Profile += strings.Repeat("#x\n", 500)
	packets := chunkPackets(t, s)
	a := Assembler{Client: "retail"}
	body := lookup.DecodeSample(packets[0]).Sim
	if _, err := a.Accept(body, now); err != nil {
		t.Fatal(err)
	}
	bad := append([]byte{}, body...)
	bad[len(bad)-1] ^= 1
	if _, err := a.Accept(bad, now); err == nil {
		t.Fatal("conflicting chunk accepted")
	}
	if _, err := a.Accept([]byte(fmt.Sprintf("1|foo|retail|%d|9999999|1|1|0\nx", now.Unix())), now); err == nil {
		t.Fatal("oversized accepted")
	}
	for _, data := range [][]byte{nil, []byte("1"), []byte("{}{}"), []byte(`{"schema":1,"unexpected":true}`)} {
		if _, err := DecodeSnapshot(data, "retail"); err == nil {
			t.Fatal("bad snapshot accepted")
		}
	}
}
func TestProfileSafeInputAndCandidates(t *testing.T) {
	p, err := ParseProfile(profileFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Candidates) != 2 || p.Candidates[0].Source != "owned" || p.Candidates[1].Source != "vault" {
		t.Fatal(p.Candidates)
	}
	input, err := p.Input(DefaultScenario(), []string{"item1", "item2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input, "profileset.item2=main_hand=,id=125") || strings.Contains(input, `mage="Test"`) {
		t.Fatal(input)
	}
	dungeon := DefaultScenario()
	dungeon.FightStyle = "dungeon_slice"
	dungeon.RaidBuffs = "full" // Dungeon Slice must still suppress optimal raid buffs.
	input, err = p.Input(dungeon, nil)
	if err != nil || !strings.Contains(input, "fight_style=DungeonSlice") || !strings.Contains(input, "max_time=360") ||
		!strings.Contains(input, "desired_targets=1") || !strings.Contains(input, "optimal_raid=0") {
		t.Fatalf("Dungeon Slice input is not comparable to Raidbots: %s %v", input, err)
	}
	selfBuffed := DefaultScenario()
	selfBuffed.FightStyle = "patchwerk"
	selfBuffed.RaidBuffs = "none"
	input, err = p.Input(selfBuffed, nil)
	if err != nil || !strings.Contains(input, "fight_style=Patchwerk") || !strings.Contains(input, "optimal_raid=0") {
		t.Fatalf("self-buffed Patchwerk input is wrong: %s %v", input, err)
	}
	for _, injection := range []string{"input=/private/file", "output=/tmp/overwrite", "actions=/use_item,name=x", "http_clear_cache=1", "mage=other", "profileset.x=talents=abc", "main_hand=,id=1,source=wowhead"} {
		if _, err := ParseProfile(profileFixture + injection + "\n"); err == nil {
			t.Fatalf("accepted %s", injection)
		}
	}
	if _, err := p.Input(Scenario{Targets: 99, Seconds: 300, Iterations: 1000}, nil); err == nil {
		t.Fatal("unbounded scenario accepted")
	}
	badStyle := DefaultScenario()
	badStyle.FightStyle = "made_up"
	if _, err := p.Input(badStyle, nil); err == nil {
		t.Fatal("unknown fight style accepted")
	}
	if _, err := p.Input(DefaultScenario(), []string{"../x"}); err == nil {
		t.Fatal("unknown candidate accepted")
	}
}

func TestProfileAcceptsEveryRetailClassHeader(t *testing.T) {
	for _, class := range []string{
		"deathknight", "demonhunter", "druid", "evoker", "hunter", "mage", "monk",
		"paladin", "priest", "rogue", "shaman", "warlock", "warrior",
	} {
		t.Run(class, func(t *testing.T) {
			text := strings.Replace(profileFixture, `mage="Test"`, class+`="Test"`, 1)
			profile, err := ParseProfile(text)
			if err != nil {
				t.Fatal(err)
			}
			if profile.Class != class || !strings.Contains(profile.Base, class+"=Lamda\n") {
				t.Fatalf("class header was not preserved: %#v", profile)
			}
		})
	}
}

func TestProviderAcceptsEverySupportedDamageSpecialization(t *testing.T) {
	tests := []struct {
		class string
		spec  string
		id    int
	}{
		{"mage", "arcane", 62}, {"mage", "fire", 63}, {"mage", "frost", 64},
		{"paladin", "retribution", 70}, {"warrior", "arms", 71}, {"warrior", "fury", 72},
		{"druid", "balance", 102}, {"druid", "feral", 103},
		{"deathknight", "blood", 250}, {"deathknight", "frost", 251}, {"deathknight", "unholy", 252},
		{"hunter", "beast_mastery", 253}, {"hunter", "marksmanship", 254}, {"hunter", "survival", 255},
		{"priest", "shadow", 258}, {"rogue", "assassination", 259}, {"rogue", "outlaw", 260}, {"rogue", "subtlety", 261},
		{"shaman", "elemental", 262}, {"shaman", "enhancement", 263},
		{"warlock", "affliction", 265}, {"warlock", "demonology", 266}, {"warlock", "destruction", 267},
		{"monk", "windwalker", 269}, {"demonhunter", "havoc", 577},
		{"evoker", "devastation", 1467}, {"evoker", "augmentation", 1473},
	}
	provider := testProvider(t)
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s_%s", test.class, test.spec), func(t *testing.T) {
			snapshot := fixture(time.Now())
			snapshot.Character.Class = strings.ToUpper(test.class)
			snapshot.Character.Spec = test.id
			snapshot.Profile = strings.Replace(profileFixture, `mage="Test"`, test.class+`="Test"`, 1)
			snapshot.Profile = strings.Replace(snapshot.Profile, "spec=frost", "spec="+test.spec, 1)
			capability := provider.Capability(snapshot, time.Now())
			if !capability.Available {
				t.Fatalf("expected supported export, got %q", capability.Reason)
			}
		})
	}
}
func testProvider(t *testing.T) *Provider {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return &Provider{Config: ProviderConfig{Path: path, Version: "test-worker", SHA256: hex.EncodeToString(sum[:])}}
}
func TestProviderCapabilities(t *testing.T) {
	p := testProvider(t)
	s := fixture(time.Now())
	if !p.Capability(s, time.Now()).Available {
		t.Fatal(p.Capability(s, time.Now()))
	}
	s.Character.Role = "HEALER"
	if p.Capability(s, time.Now()).Available {
		t.Fatal("healer DPS allowed")
	}
	s.Character.Role = "DAMAGER"
	s.Client = "forever"
	if p.Capability(s, time.Now()).Available {
		t.Fatal("retail model allowed for Forever")
	}
	s.Client = "retail"
	s.Character.Spec = 63
	if p.Capability(s, time.Now()).Available {
		t.Fatal("mismatched spec allowed")
	}
	s.Character.Spec = 64
	p.Config.Builds = []string{s.Build}
	p.Config.Specs = []int{64}
	p.Config.ReviewedAt = time.Now().Add(-8 * 24 * time.Hour)
	if p.Capability(s, time.Now()).Validation == "reviewed" {
		t.Fatal("expired review current")
	}
}

func TestResultPreservesModelWarningsAndRejectsErrors(t *testing.T) {
	base := `{"sim":{"players":[{"gear":{"main_hand":{"encoded_item":"id=123"}},"collected_data":{"dps":{"mean":100}}}]},"logs":[{"level":"warning","message":"fixture model warning"}]}`
	r, err := ParseResult([]byte(base))
	if err != nil || len(r.Warnings) != 1 {
		t.Fatalf("warning lost: %v %v", r, err)
	}
	if _, err = ParseResult([]byte(strings.Replace(base, `"warning"`, `"error"`, 1))); err == nil {
		t.Fatal("error accepted as complete")
	}
	s := fixture(time.Now())
	s.Client = "forever"
	if c := (&Provider{}).Capability(s, time.Now()); c.Available || !strings.Contains(c.Reason, "Forever") {
		t.Fatal(c)
	}
}

func TestMain(m *testing.M) {
	if mode := os.Getenv("LAMDA_SIM_TEST_WORKER"); mode != "" {
		if mode == "planned_timeout" {
			if dir, _ := os.Getwd(); strings.HasSuffix(dir, "variant-001") {
				time.Sleep(time.Minute)
				os.Exit(0)
			}
		}
		if mode == "sleep" {
			time.Sleep(time.Minute)
			os.Exit(0)
		}
		if mode == "fail" {
			fmt.Println("fixture worker error")
			os.Exit(2)
		}
		if mode != "missing_gear" {
			_ = os.WriteFile("verified-gear.simc", []byte("main_hand=fixture_weapon,id=123\n"), 0600)
		}
		if mode == "item_rejection" {
			input, _ := os.ReadFile("input.simc")
			if strings.Contains(string(input), "profileset.item2") {
				fmt.Println("Invalid item string: fixture item2")
				os.Exit(82)
			}
			if strings.Contains(string(input), "profileset.item1") {
				_ = os.WriteFile("result.json", []byte(`{"sim":{"players":[{"gear":{"main_hand":{"encoded_item":"id=123"}},"collected_data":{"dps":{"mean":12345}}}],"profilesets":{"results":[{"name":"item1","mean":12900,"overrides":{"gear":{"main_hand":{"item_id":124,"item_level":0}}}}]}}}`), 0600)
				os.Exit(0)
			}
		}
		if mode == "analysis" {
			input, _ := os.ReadFile("input.simc")
			text := string(input)
			targets := 1
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "desired_targets=") {
					_, _ = fmt.Sscanf(line, "desired_targets=%d", &targets)
				}
			}
			results := []map[string]any{}
			seen := map[string]bool{}
			for _, line := range strings.Split(text, "\n") {
				if !strings.HasPrefix(line, "profileset.") {
					continue
				}
				nameAndValue := strings.TrimPrefix(line, "profileset.")
				name, value, ok := strings.Cut(nameAndValue, "=")
				slot, item, okItem := strings.Cut(value, "=")
				if !ok || !okItem || strings.HasSuffix(name, "+") || seen[name] {
					continue
				}
				seen[name] = true
				overrides := map[string]any{}
				if slot != "talents" {
					id, _ := strconv.Atoi(itemID(item))
					overrides[reportedSlot(slot)] = map[string]any{"item_id": id, "item_level": itemLevelFromValue(item)}
				}
				results = append(results, map[string]any{"name": name, "mean": float64(12345 + targets*100 + len(results) + 1), "stddev": 40.0, "mean_error": 2.0,
					"overrides": map[string]any{"gear": overrides}})
			}
			report := map[string]any{"sim": map[string]any{
				"players":     []any{map[string]any{"gear": map[string]any{"main_hand": map[string]any{"encoded_item": "id=123"}}, "collected_data": map[string]any{"dps": map[string]any{"mean": float64(12345 + targets*100), "std_dev": 40.0, "mean_error": 2.0}}}},
				"profilesets": map[string]any{"results": results},
			}}
			data, _ := json.Marshal(report)
			_ = os.WriteFile("result.json", data, 0600)
			os.Exit(0)
		}
		if mode == "partial_gear" {
			_ = os.WriteFile("result.json", []byte(`{"sim":{"players":[{"gear":{"main_hand":{"encoded_item":"id=123"}},"collected_data":{"dps":{"mean":12345,"std_dev":40,"mean_error":2}}}],"profilesets":{"results":[{"name":"item1","mean":12900,"stddev":40,"mean_error":2,"overrides":{"gear":{"main_hand":{"item_id":124,"item_level":0}}}},{"name":"item2","mean":12345,"stddev":40,"mean_error":2,"overrides":{"gear":{}}}]}}}`), 0600)
			os.Exit(0)
		}
		_ = os.WriteFile("result.json", []byte(`{"sim":{"players":[{"gear":{"main_hand":{"encoded_item":"id=123"}},"collected_data":{"dps":{"mean":12345,"std_dev":40,"mean_error":2}}}],"profilesets":{"results":[]}}}`), 0600)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func waitJob(t *testing.T, w *Workspace, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range w.State().Jobs {
			if j.ID == id && j.State != "running" && j.State != "cancelling" {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
func TestWorkspaceWorkerAndPersistence(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "success")
	dir := t.TempDir()
	p := testProvider(t)
	w, err := NewWorkspace(context.Background(), dir, "retail", p)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := fixture(time.Now())
	fresh, err := w.Receive(s)
	if !fresh || err != nil {
		t.Fatal(err)
	}
	if fresh, err = w.Receive(s); fresh || err != nil {
		t.Fatal("duplicate snapshot")
	}
	j, err := w.Start(s.ID, DefaultScenario(), nil)
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, w, j.ID)
	if finished.State != "complete" || finished.Result.Baseline.Mean != 12345 {
		t.Fatal(finished)
	}
	w2, err := NewWorkspace(context.Background(), dir, "retail", p)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	if len(w2.State().Jobs) != 1 {
		t.Fatal("history lost")
	}
	if fresh, _ := w2.Receive(s); fresh {
		t.Fatal("replay accepted after restart")
	}
	p.Config.SHA256 = strings.Repeat("0", 64)
	j, err = w.Start(s.ID, DefaultScenario(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if finished = waitJob(t, w, j.ID); finished.State != "failed" || !strings.Contains(finished.Error, "checksum") {
		t.Fatal(finished)
	}
}
func TestCancellationAndSingleWorker(t *testing.T) {
	t.Setenv("LAMDA_SIM_TEST_WORKER", "sleep")
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", testProvider(t))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := fixture(time.Now())
	w.Receive(s)
	j, err := w.Start(s.ID, DefaultScenario(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Start(s.ID, DefaultScenario(), nil); err == nil {
		t.Fatal("second worker allowed")
	}
	if err = w.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	if got := waitJob(t, w, j.ID); got.State != "cancelled" {
		t.Fatal(got)
	}
}
func TestLocalHTTPBoundary(t *testing.T) {
	w, err := NewWorkspace(context.Background(), t.TempDir(), "retail", &Provider{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	service := &Service{Workspace: w}
	handler := service.Handler("127.0.0.1:47641")
	for _, tc := range []struct {
		host, origin, method, path, header string
		want                               int
	}{
		{"evil.test", "", "GET", "/api/state", "", 403},
		{"127.0.0.1:47641", "https://evil.test", "GET", "/api/state", "", 403},
		{"127.0.0.1:47641", "", "POST", "/api/run", "", 403},
		{"127.0.0.1:47641", "", "GET", "/api/state", "", 200},
		{"127.0.0.1:47641", "", "GET", "/", "", 200},
		{"127.0.0.1:47641", "", "GET", "/addons", "", 200},
		{"127.0.0.1:47641", "", "GET", "/api/addons", "", 503},
		{"127.0.0.1:47641", "", "GET", "/api/log?id=../../secret", "", 404},
		{"127.0.0.1:47641", "http://127.0.0.1:47641", "POST", "/api/run", "1", 400},
	} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, strings.NewReader(`{}`))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Lamda-Sim", tc.header)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		if rec.Code != tc.want {
			t.Fatalf("%+v: %d %s", tc, rec.Code, rec.Body.String())
		}
	}
	for _, file := range []string{"index.html", "style.css", "app.js", "addons.html", "addons.css", "addons.js"} {
		if _, err := webFiles.ReadFile("web/" + file); err != nil {
			t.Fatal(err)
		}
	}
}
func TestInterruptedRunAndAuditStaleness(t *testing.T) {
	dir := t.TempDir()
	j := Job{ID: "run1", State: "running"}
	if err := writeJSON(filepath.Join(dir, "jobs", "run1", "job.json"), j); err != nil {
		t.Fatal(err)
	}
	w, err := NewWorkspace(context.Background(), dir, "retail", &Provider{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if w.State().Jobs[0].State != "interrupted" {
		t.Fatal("stale running job")
	}
	if len((Audit{}).Warnings(time.Now())) == 0 {
		t.Fatal("missing audit accepted")
	}
}
