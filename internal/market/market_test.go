package market

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixtureNow = time.Date(2026, 11, 14, 20, 0, 0, 0, time.UTC)
var fixtureMarket = Identity{Client: "forever", Region: "us", Realm: "Test Realm", Faction: "Alliance"}

func fixture(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "market.sqlite"), "forever")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func event(id, kind string) Event {
	return Event{Version: 1, ID: id, Market: fixtureMarket, At: fixtureNow.Unix(), Kind: kind}
}
func ingest(t *testing.T, s *Store, e Event) {
	t.Helper()
	if err := s.Ingest(encoded(e), fixtureNow); err != nil {
		t.Fatal(err)
	}
}
func scan(id string, at int64, lots []Lot) Event {
	e := event(id, "auction")
	e.At = at
	e.Scan = id
	e.Page = 1
	e.Pages = 1
	e.Coverage = "full"
	e.Lots = lots
	return e
}
func TestAtomicScanReplayAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "market.sqlite")
	s, err := Open(path, "forever")
	if err != nil {
		t.Fatal(err)
	}
	e := scan("scan", fixtureNow.Unix(), []Lot{{Item: "fish", Price: 20, Quantity: 1, Count: 5}})
	e.Pages = 2
	e.ID = "page2"
	e.Page = 2
	e.Items = []Item{{ID: "fish", Name: "Raw Fish"}}
	ingest(t, s, e)
	cards, err := s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 0 {
		t.Fatalf("partial scan leaked: %+v %v", cards, err)
	}
	ingest(t, s, e)
	conflict := e
	conflict.ID = "page2-conflict"
	conflict.Lots = []Lot{{Item: "fish", Price: 999, Quantity: 1, Count: 1}}
	if s.Ingest(encoded(conflict), fixtureNow) == nil {
		t.Fatal("conflicting page accepted")
	}
	e.ID = "page1"
	e.Page = 1
	e.Lots = []Lot{{Item: "fish", Price: 30, Quantity: 1, Count: 2}}
	ingest(t, s, e)
	ingest(t, s, e)
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path, "forever")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cards, err = s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || cards[0].Quantity != 7 || cards[0].Price != 20 || len(cards[0].History) != 1 {
		t.Fatalf("bad persistent snapshot %+v %v", cards, err)
	}
	old := scan("older", fixtureNow.Add(-time.Hour).Unix(), []Lot{{Item: "fish", Price: 1, Quantity: 1, Count: 1}})
	ingest(t, s, old)
	cards, _ = s.Cards(fixtureMarket.Key(), 0)
	if cards[0].Price != 20 {
		t.Fatal("old observation replaced latest")
	}
	empty := scan("empty", fixtureNow.Add(time.Minute).Unix(), nil)
	ingest(t, s, empty)
	cards, _ = s.Cards(fixtureMarket.Key(), 0)
	if cards[0].Quantity != 0 {
		t.Fatal("absent listing still offered")
	}
	other := e
	other.ID = "other-faction"
	other.Market.Faction = "Horde"
	other.Scan = "other"
	other.Pages = 1
	ingest(t, s, other)
	markets, _ := s.Markets()
	if len(markets) != 2 {
		t.Fatal("factions merged")
	}
	wrong := e
	wrong.Market.Client = "retail"
	if s.Ingest(encoded(wrong), fixtureNow) == nil {
		t.Fatal("cross-client observation accepted")
	}
}
func TestCostsUseStacksBundlesAndDepth(t *testing.T) {
	p, ok := lotCost([]Lot{{Item: "ore", Price: 100, Quantity: 20, Count: 1}, {Item: "ore", Price: 200, Quantity: 10, Count: 1}}, 21)
	if !ok || p.Cost != 300 || p.Acquired != 30 {
		t.Fatalf("stack cost: %+v %v", p, ok)
	}
	if _, ok = lotCost([]Lot{{Price: 100, Quantity: 20, Count: 1}}, 21); ok {
		t.Fatal("invented market depth")
	}
	p, ok = lotCost([]Lot{{Price: 7, Quantity: 1, Count: 100, Divisible: true}}, 3)
	if !ok || p.Cost != 21 || p.Acquired != 3 {
		t.Fatal("commodity cost")
	}
	m := model{settings: DefaultSettings(), now: fixtureNow.Unix(), quotes: map[string]quote{}, vendors: []Vendor{{ID: "fish", Item: "fish", Buy: 20, Bundle: 5, Stock: 10}}, items: map[string]Item{}}
	p, ok = m.acquire("fish", 6, map[string]bool{}, 0)
	if !ok || p.Cost != 40 || p.Acquired != 10 {
		t.Fatal("vendor bundles split")
	}
	m.vendors[0].Stock = 5
	if _, ok = m.acquire("fish", 6, map[string]bool{}, 0); ok {
		t.Fatal("vendor stock invented")
	}
	m.vendors[0].Stock = -1
	m.vendors[0].Requirement = "Reputation"
	if _, ok = m.acquire("fish", 1, map[string]bool{}, 0); ok {
		t.Fatal("unmet vendor requirement ignored")
	}
}
func TestGenericChainsCharactersTheoryAndCycles(t *testing.T) {
	m := model{settings: DefaultSettings(), now: fixtureNow.Unix(), quotes: map[string]quote{"herb": {At: fixtureNow.Unix(), Lots: []Lot{{Price: 10, Quantity: 1, Count: 10}}, Coverage: "full"}}, items: map[string]Item{"extract": {ID: "extract", Transferable: true}}, recipes: []Recipe{
		{ID: "extract", Name: "Extract", Profession: "alchemy", Rank: 10, RankKnown: true, Inputs: []Reagent{{Item: "herb", Quantity: 2}}, Output: Reagent{Item: "extract", Quantity: 1}},
		{ID: "bandage", Name: "Infused bandage", Profession: "aid", Rank: 20, RankKnown: true, Inputs: []Reagent{{Item: "extract", Quantity: 1}}, Output: Reagent{Item: "bandage", Quantity: 2}},
	}, chars: []Character{{ID: "one", Name: "One", Professions: []Profession{{ID: "alchemy", Rank: 50}}, Known: []string{"extract"}}, {ID: "two", Name: "Two", Professions: []Profession{{ID: "aid", Rank: 50}}, Known: []string{"bandage"}}}}
	p, ok := m.acquire("bandage", 2, map[string]bool{}, 0)
	if !ok || p.Cost != 20 || len(p.Actors) != 2 {
		t.Fatalf("cross-character chain %+v %v", p, ok)
	}
	m.items["extract"] = Item{ID: "extract", Transferable: false}
	if _, ok = m.acquire("bandage", 2, map[string]bool{}, 0); ok {
		t.Fatal("bound intermediate crossed characters")
	}
	m.settings.Mode = "character"
	m.settings.Character = "one"
	if _, ok = m.acquire("bandage", 2, map[string]bool{}, 0); ok {
		t.Fatal("current character used alt capability")
	}
	m.settings.Mode = "theory"
	m.settings.Theory = map[string]int{"alchemy": 50, "aid": 50}
	m.items["extract"] = Item{ID: "extract", Transferable: true}
	if _, ok = m.acquire("bandage", 2, map[string]bool{}, 0); !ok {
		t.Fatal("theory chain failed")
	}
	if len(m.chars[0].Known) != 1 {
		t.Fatal("theory changed characters")
	}
	m.settings.Theory["aid"] = 1
	if _, ok = m.acquire("bandage", 2, map[string]bool{}, 0); ok {
		t.Fatal("assumed rank ignored")
	}
	m.settings.Theory["aid"] = 50
	m.recipes[1].RankKnown = false
	if _, ok = m.acquire("bandage", 2, map[string]bool{}, 0); ok {
		t.Fatal("unknown rank treated as known")
	}
	m.recipes[1].RankKnown = true
	m.recipes[0].Inputs = []Reagent{{Item: "bandage", Quantity: 1}}
	if _, ok = m.acquire("bandage", 2, map[string]bool{}, 0); ok {
		t.Fatal("cyclic free materials")
	}
}
func TestFishCookingFeesFreshnessAndSummary(t *testing.T) {
	s := fixture(t)
	key := fixtureMarket.Key()
	e := event("vendor", "vendor")
	e.Vendors = []Vendor{{ID: "npc", Item: "raw", Name: "Fishmonger", Buy: 20, Bundle: 1, Stock: -1}}
	ingest(t, s, e)
	e = event("recipe", "recipes")
	e.Recipes = []Recipe{{ID: "cook", Name: "Cook fish", Profession: "185", Rank: 1, RankKnown: true, Inputs: []Reagent{{Item: "raw", Quantity: 1}}, Output: Reagent{Item: "cooked", Quantity: 1}}}
	ingest(t, s, e)
	e = event("char", "character")
	e.Character = &Character{ID: "toon", Name: "Cook", Professions: []Profession{{ID: "185", Rank: 50}}, Known: []string{"cook"}, RecipesObserved: true}
	ingest(t, s, e)
	ingest(t, s, scan("auction", fixtureNow.Unix(), []Lot{{Item: "cooked", Price: 30, Quantity: 1, Count: 4}}))
	opps, err := s.Opportunities(key, fixtureNow)
	if err != nil || len(opps) != 1 || opps[0].Cost != 20 || opps[0].Revenue != 28 || opps[0].Profit != 8 {
		t.Fatalf("fish estimate %+v %v", opps, err)
	}
	settings := s.Settings(key)
	settings.LostDeposit = 9
	if err = s.SaveSettings(key, settings); err != nil {
		t.Fatal(err)
	}
	opps, _ = s.Opportunities(key, fixtureNow)
	if len(opps) != 0 {
		t.Fatal("deposit loss omitted")
	}
	settings.LostDeposit = 0
	s.SaveSettings(key, settings)
	opps, _ = s.Opportunities(key, fixtureNow.Add(25*time.Hour))
	if len(opps) != 0 {
		t.Fatal("stale prices used")
	}
	summary := scan("summary", fixtureNow.Add(time.Minute).Unix(), []Lot{{Item: "raw", Price: 1, Quantity: 100, Count: 1}})
	summary.Coverage = "summary"
	ingest(t, s, summary)
	// The raw summary cannot price inputs; the real observed merchant remains usable.
	m := model{settings: settings, now: fixtureNow.Unix(), quotes: map[string]quote{"raw": {Lots: summary.Lots, At: fixtureNow.Unix(), Coverage: "summary"}}}
	if _, ok := m.acquire("raw", 1, map[string]bool{}, 0); ok {
		t.Fatal("browse summary treated as executable depth")
	}
}
func TestHTTPBoundaryImportAndWrites(t *testing.T) {
	s := fixture(t)
	h := s.LocalHandler("127.0.0.1:47643")
	request := func(method, path, host, origin, header, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader(body))
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if header != "" {
			r.Header.Set("X-Lamda-Sim", header)
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, args := range [][3]string{{"evil.example", "", "1"}, {"127.0.0.1:47643", "https://evil.example", "1"}, {"127.0.0.1:47643", "", ""}} {
		w := request("POST", "/api/market/import", args[0], args[1], args[2], `{"events":[]}`)
		if w.Code != 403 {
			t.Fatalf("boundary code %d", w.Code)
		}
	}
	e := scan("http", fixtureNow.Unix(), []Lot{{Item: "cloth", Price: 10, Quantity: 1, Count: 3}})
	body := fmt.Sprintf(`{"events":[%s]}`, encoded(e))
	w := request("POST", "/api/market/import", "127.0.0.1:47643", "", "1", body)
	if w.Code != 200 {
		t.Fatalf("import %d %s", w.Code, w.Body)
	}
	path := "/api/market/watch?market=" + strings.ReplaceAll(fixtureMarket.Key(), " ", "%20")
	w = request("POST", path, "127.0.0.1:47643", "", "1", `{"item":"cloth","on":true}`)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = request("GET", "/market/", "127.0.0.1:47643", "", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Lamda Market") {
		t.Fatal("assets unavailable")
	}
	w = request("GET", "/api/market/state", "127.0.0.1:47643", "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}
func TestOpticalBoundsAndConcurrency(t *testing.T) {
	a := NewAssembler()
	e := event("chunk", "vendor")
	e.Items = []Item{{ID: "x", Name: strings.Repeat("x", 200)}}
	data := encoded(e)
	body := append([]byte(fmt.Sprintf("1|chunk|1|1|%d\n", len(data))), data...)
	got, err := a.Accept(body, fixtureNow)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal(err)
	}
	bad := []byte("1|chunk|1|999999|999999999\n{}")
	if _, err = a.Accept(bad, fixtureNow); err == nil {
		t.Fatal("unbounded transfer")
	}
	s := fixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.Ingest(encoded(scan(fmt.Sprint(i), fixtureNow.Unix(), []Lot{{Item: "x", Price: 10, Quantity: 1, Count: 1}})), fixtureNow); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	cards, err := s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || len(cards[0].History) != 4 {
		t.Fatal("concurrent ingest lost observations", err)
	}
}
func TestActualLuaPacketsToSQLite(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	fixtureScript := filepath.Join(root, "Tools/market-wire-fixture.py")
	if _, err := os.Stat(fixtureScript); err != nil {
		root, err = filepath.Abs("../..") // Public companion repository layout.
		if err != nil {
			t.Fatal(err)
		}
		fixtureScript = filepath.Join(root, "tools/market-wire-fixture.py")
	}
	python := os.Getenv("LAMDA_TEST_PYTHON")
	if python == "" {
		python = filepath.Join(root, ".venv/bin/python")
		if _, err := os.Stat(python); err != nil {
			python, err = exec.LookPath("python3")
			if err != nil {
				t.Fatal("market wire fixture requires Python with lupa:", err)
			}
		}
	}
	cmd := exec.Command(python, fixtureScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Lua fixture: %v %s", err, output)
	}
	var packets []string
	if err = json.Unmarshal(output, &packets); err != nil {
		t.Fatal(err, string(output))
	}
	s := fixture(t)
	// Reverse chunks and replay twice through the actual optical decoder.
	for pass := 0; pass < 2; pass++ {
		for i := len(packets) - 1; i >= 0; i-- {
			packet, err := base64.StdEncoding.DecodeString(packets[i])
			if err != nil {
				t.Fatal(err)
			}
			sample := lookup.DecodeSample(packet)
			if sample.Error != "" || sample.Source != "market" {
				t.Fatalf("decode %+v", sample)
			}
			if err = s.Accept(sample.Market, fixtureNow); err != nil {
				t.Fatal(err)
			}
		}
	}
	cards, err := s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || cards[0].Price != 20 || len(cards[0].History) != 1 {
		t.Fatalf("Lua→SQLite %+v %v", cards, err)
	}
	packet, _ := base64.StdEncoding.DecodeString(packets[0])
	packet[100] ^= 1
	if lookup.DecodeSample(packet).Error != "bridge_checksum" {
		t.Fatal("corrupted packet accepted")
	}
}

var _ = http.MethodGet

func TestLedgerAverageBasisAndBalance(t *testing.T) {
	s := fixture(t)
	key := fixtureMarket.Key()
	at := time.Now().Unix()
	if err := s.Trade(key, Trade{ID: "unowned", Item: "cloth", Side: "sell", Quantity: 1, Total: 10, At: at}); err == nil {
		t.Fatal("unowned holdings sold")
	}
	buy := Trade{ID: "buy", Item: "cloth", Side: "buy", Quantity: 10000000, Total: 1000000000000, At: at}
	if err := s.Trade(key, buy); err != nil {
		t.Fatal(err)
	}
	if err := s.Trade(key, buy); err == nil {
		t.Fatal("duplicate ledger transaction accepted")
	}
	if err := s.Trade(key, Trade{ID: "sale", Item: "cloth", Side: "sell", Quantity: 5000000, Total: 600000000000, At: at}); err != nil {
		t.Fatal(err)
	}
	cards, err := s.Cards(key, 0)
	if err != nil || len(cards) != 1 || cards[0].Holding != 5000000 || cards[0].Basis != 500000000000 {
		t.Fatalf("average basis or integer overflow: %+v %v", cards, err)
	}
}

func TestProfessionSnapshotsPreserveUnopenedSkills(t *testing.T) {
	s := fixture(t)
	e := event("skills-one", "character")
	e.Character = &Character{ID: "toon", Name: "Toon", RecipesObserved: true, Professions: []Profession{{ID: "185", Rank: 50}, {ID: "186", Rank: 50}}, KnownByProfession: map[string][]string{"185": {"cook"}, "186": {"smelt"}}, RecipeTimes: map[string]int64{"185": e.At - 100, "186": e.At - 50}}
	ingest(t, s, e)
	e.ID = "skills-two"
	e.Character = &Character{ID: "toon", Name: "Toon", RecipesObserved: true, Professions: []Profession{{ID: "185", Rank: 75}, {ID: "186", Rank: 50}}, KnownByProfession: map[string][]string{"185": {"cook", "feast"}}, RecipeTimes: map[string]int64{"185": e.At}}
	ingest(t, s, e)
	var chars []Character
	if err := s.documents(fixtureMarket.Key(), "character", &chars); err != nil {
		t.Fatal(err)
	}
	if len(chars) != 1 || len(chars[0].Known) != 3 || chars[0].RecipeTimes["186"] != e.At-50 {
		t.Fatalf("unopened profession erased %+v", chars)
	}
	e.ID = "forgot-mining"
	e.Character.Professions = []Profession{{ID: "185", Rank: 75}}
	ingest(t, s, e)
	chars = nil // Read a fresh snapshot, as the HTTP handler does; JSON maps otherwise merge into the test destination.
	s.documents(fixtureMarket.Key(), "character", &chars)
	if len(chars[0].KnownByProfession) != 1 {
		t.Fatal("unlearned profession remained a capability")
	}
}

func TestRangeKeepsLastQuoteWithoutInventingHistory(t *testing.T) {
	s := fixture(t)
	e := scan("last", fixtureNow.Add(-48*time.Hour).Unix(), []Lot{{Item: "cloth", Price: 20, Quantity: 1, Count: 5}})
	ingest(t, s, e)
	cards, err := s.Cards(fixtureMarket.Key(), fixtureNow.Add(-24*time.Hour).Unix())
	if err != nil || len(cards) != 1 || cards[0].Price != 20 || len(cards[0].History) != 0 || cards[0].At != e.At {
		t.Fatalf("range erased quote or fabricated history: %+v %v", cards, err)
	}
}
func TestVendorSaleUsesGuaranteedWholeOutput(t *testing.T) {
	s := fixture(t)
	e := event("vendor-yield", "vendor")
	e.Vendors = []Vendor{{ID: "cloth", Item: "cloth", Buy: 1, Bundle: 1, Stock: -1}, {ID: "bandage", Item: "bandage", Sell: 2, Bundle: 1, Stock: 0}}
	ingest(t, s, e)
	e = event("recipe-yield", "recipes")
	e.Recipes = []Recipe{{ID: "bandages", Name: "Bandages", Profession: "129", RankKnown: true, Inputs: []Reagent{{Item: "cloth", Quantity: 1}}, Output: Reagent{Item: "bandage", Quantity: 3}}}
	ingest(t, s, e)
	settings := DefaultSettings()
	settings.Mode = "theory"
	settings.Theory = map[string]int{"129": 300}
	s.SaveSettings(fixtureMarket.Key(), settings)
	opps, err := s.Opportunities(fixtureMarket.Key(), fixtureNow)
	if err != nil || len(opps) != 1 || opps[0].Quantity != 3 || opps[0].Revenue != 6 || opps[0].Profit != 5 {
		t.Fatalf("whole output not sold: %+v %v", opps, err)
	}
}

func TestSharedInputsUseJointDepthAndOnePurchase(t *testing.T) {
	m := model{settings: DefaultSettings(), now: fixtureNow.Unix(), quotes: map[string]quote{"raw": {At: fixtureNow.Unix(), Coverage: "full", Lots: []Lot{{Price: 100, Quantity: 20, Count: 1}, {Price: 300, Quantity: 20, Count: 1}}}}, items: map[string]Item{}, chars: []Character{{ID: "a", Name: "A", Professions: []Profession{{ID: "p", Rank: 100}}, Known: []string{"left", "right", "final"}}}, recipes: []Recipe{
		{ID: "left", Name: "Left", Profession: "p", Inputs: []Reagent{{Item: "raw", Quantity: 15}}, Output: Reagent{Item: "left", Quantity: 1}},
		{ID: "right", Name: "Right", Profession: "p", Inputs: []Reagent{{Item: "raw", Quantity: 15}}, Output: Reagent{Item: "right", Quantity: 1}},
		{ID: "final", Name: "Final", Profession: "p", Inputs: []Reagent{{Item: "left", Quantity: 1}, {Item: "right", Quantity: 1}}, Output: Reagent{Item: "final", Quantity: 1}},
	}}
	p, ok := m.acquire("final", 1, map[string]bool{}, 0)
	if !ok || p.Cost != 400 || p.Used["ah/raw"] != 30 {
		t.Fatalf("shared market depth: %+v %v", p, ok)
	}
	purchases := 0
	for _, step := range p.Steps {
		if strings.HasPrefix(step, "Buy ") {
			purchases++
		}
	}
	if purchases != 1 {
		t.Fatalf("purchase list disagrees with joint price: %+v", p.Steps)
	}
	m.quotes["raw"] = quote{At: fixtureNow.Unix(), Coverage: "full", Lots: []Lot{{Price: 100, Quantity: 20, Count: 1}}}
	if _, ok = m.acquire("final", 1, map[string]bool{}, 0); ok {
		t.Fatal("shared input duplicated unavailable supply")
	}
}
func TestBoundIntermediateUsesCapableSameCharacter(t *testing.T) {
	m := model{settings: DefaultSettings(), now: fixtureNow.Unix(), quotes: map[string]quote{}, vendors: []Vendor{{ID: "v", Item: "raw", Buy: 10, Bundle: 1, Stock: -1}}, items: map[string]Item{}, chars: []Character{
		{ID: "first", Name: "First", Professions: []Profession{{ID: "p", Rank: 100}}, Known: []string{"final"}},
		{ID: "second", Name: "Second", Professions: []Profession{{ID: "p", Rank: 100}}, Known: []string{"bound", "final"}},
	}, recipes: []Recipe{{ID: "bound", Profession: "p", Inputs: []Reagent{{Item: "raw", Quantity: 1}}, Output: Reagent{Item: "bound", Quantity: 1}}, {ID: "final", Profession: "p", Inputs: []Reagent{{Item: "bound", Quantity: 1}}, Output: Reagent{Item: "final", Quantity: 1}}}}
	p, ok := m.acquire("final", 1, map[string]bool{}, 0)
	if !ok || p.Maker != "Second" || len(p.Actors) != 1 {
		t.Fatalf("capable character not selected: %+v %v", p, ok)
	}
}
func TestLedgerCorrectionsPreserveChronologicalBalance(t *testing.T) {
	s := fixture(t)
	key := fixtureMarket.Key()
	at := time.Now().Unix()
	buy := Trade{ID: "b", Item: "x", Side: "buy", Quantity: 10, Total: 100, At: at}
	if err := s.Trade(key, buy); err != nil {
		t.Fatal(err)
	}
	sale := Trade{ID: "s", Item: "x", Side: "sell", Quantity: 4, Total: 80, At: at}
	if err := s.Trade(key, sale); err != nil {
		t.Fatal(err)
	}
	cards, _ := s.Cards(key, 0)
	if cards[0].Realized != 40 || cards[0].Basis != 60 {
		t.Fatal("realized profit")
	}
	if err := s.RemoveTrade(key, "b"); err == nil {
		t.Fatal("dependent sale allowed after removing purchase")
	}
	buy.Quantity = 2
	if err := s.EditTrade(key, buy); err == nil {
		t.Fatal("invalid correction accepted")
	}
	sale.Total = 100
	if err := s.EditTrade(key, sale); err != nil {
		t.Fatal(err)
	}
	cards, _ = s.Cards(key, 0)
	if cards[0].Realized != 60 {
		t.Fatal("sale correction omitted")
	}
	if err := s.RemoveTrade(key, "s"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveTrade(key, "b"); err != nil {
		t.Fatal(err)
	}
}
func TestRecipeOverridesDoNotChangeCharacterExports(t *testing.T) {
	s := fixture(t)
	key := fixtureMarket.Key()
	e := event("r", "recipes")
	e.Recipes = []Recipe{{ID: "r", Name: "Observed", Profession: "185", Inputs: []Reagent{{Item: "raw", Quantity: 1}}, Output: Reagent{Item: "food", Quantity: 1}}}
	ingest(t, s, e)
	edit := e.Recipes[0]
	edit.Name = "User model"
	edit.Output.Quantity = 3
	edit.RankKnown = true
	if err := s.SaveRecipe(key, edit); err != nil {
		t.Fatal(err)
	}
	e.ID = "new-observation"
	ingest(t, s, e)
	recipes, err := s.Recipes(key)
	if err != nil || len(recipes) != 1 || recipes[0].Name != "User model" || !recipes[0].Custom {
		t.Fatalf("override lost: %+v %v", recipes, err)
	}
	var chars []Character
	s.documents(key, "character", &chars)
	if len(chars) != 0 {
		t.Fatal("model invented character knowledge")
	}
	if err = s.ResetRecipe(key, "r"); err != nil {
		t.Fatal(err)
	}
	recipes, _ = s.Recipes(key)
	if recipes[0].Name != "Observed" || recipes[0].Custom {
		t.Fatal("observed recipe was not restored")
	}
}

func TestCompressedTransferRejectsInflationAndConflicts(t *testing.T) {
	var buffer bytes.Buffer
	w, _ := flate.NewWriter(&buffer, 1)
	w.Write(bytes.Repeat([]byte("x"), MaxEventBytes+1))
	w.Close()
	data := buffer.Bytes()
	a := NewAssembler()
	body := append([]byte(fmt.Sprintf("2|bomb|1|1|%d|10\n", len(data))), data...)
	if _, err := a.Accept(body, fixtureNow); err == nil {
		t.Fatal("expanded event exceeded bound")
	}
	raw := encoded(event("compressed", "vendor"))
	buffer.Reset()
	w, _ = flate.NewWriter(&buffer, 1)
	w.Write(raw)
	w.Close()
	data = buffer.Bytes()
	body = append([]byte(fmt.Sprintf("2|compressed|1|1|%d|%d\n", len(data), len(raw))), data...)
	got, err := a.Accept(body, fixtureNow)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("valid deflate rejected", err)
	}
}

func TestExactWholeStacksMinimizePurchaseTotal(t *testing.T) {
	lots := []Lot{{Price: 100, Quantity: 20, Count: 1}, {Price: 9, Quantity: 1, Count: 3}, {Price: 14, Quantity: 2, Count: 2}}
	cases := []struct{ qty, cost, acquired int64 }{{1, 9, 1}, {2, 14, 2}, {3, 23, 3}, {4, 28, 4}, {8, 100, 20}}
	for _, c := range cases {
		p, ok := lotCost(lots, c.qty)
		if !ok || p.Cost != c.cost || p.Acquired != c.acquired {
			t.Fatalf("quantity %d: %+v, %v; expected %d/%d", c.qty, p, ok, c.cost, c.acquired)
		}
	}
}
func TestExactStackSelectionAgainstIndependentEnumeration(t *testing.T) {
	seed := uint64(42)
	next := func(n uint64) int64 { seed = seed*6364136223846793005 + 1; return int64(seed >> 32 % n) }
	for trial := 0; trial < 120; trial++ {
		lots := make([]Lot, 5)
		for i := range lots {
			lots[i] = Lot{Price: next(40) + 1, Quantity: next(7) + 1, Count: next(3) + 1}
		}
		qty := next(20) + 1
		bestCost, bestQty := maxCost, int64(0)
		var visit func(int, int64, int64)
		visit = func(i int, cost, acquired int64) {
			if i == len(lots) {
				if acquired >= qty && (cost < bestCost || cost == bestCost && acquired < bestQty) {
					bestCost, bestQty = cost, acquired
				}
				return
			}
			for n := int64(0); n <= lots[i].Count; n++ {
				visit(i+1, cost+lots[i].Price*n, acquired+lots[i].Quantity*n)
			}
		}
		visit(0, 0, 0)
		p, ok := lotCost(lots, qty)
		if ok != (bestCost < maxCost) || ok && (p.Cost != bestCost || p.Acquired != bestQty) {
			t.Fatalf("trial %d qty %d lots %+v: got %+v/%v expected %d/%d", trial, qty, lots, p, ok, bestCost, bestQty)
		}
	}
}
func TestResaleRetainsProfitableVendorBundles(t *testing.T) {
	s := fixture(t)
	e := event("bulk-vendor", "vendor")
	e.Vendors = []Vendor{{ID: "bulk", Item: "thread", Buy: 80, Bundle: 20, Stock: -1}}
	ingest(t, s, e)
	ingest(t, s, scan("thread-market", fixtureNow.Unix(), []Lot{{Item: "thread", Price: 100, Quantity: 20, Count: 2}, {Item: "thread", Price: 9, Quantity: 1, Count: 1}}))
	opportunities, err := s.Opportunities(fixtureMarket.Key(), fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range opportunities {
		if o.Kind == "source resale" {
			found = true
			if o.Quantity != 20 || o.Cost != 80 || o.Profit != 15 {
				t.Fatalf("bundle: %+v", o)
			}
		}
	}
	if !found {
		t.Fatal("profitable bundle hidden by cheaper single unit")
	}
}
