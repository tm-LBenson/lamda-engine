package market

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestItemCategoriesPreserveZeroAndSurviveLegacyExports(t *testing.T) {
	s := fixture(t)
	class, subtype := 0, 0
	e := scan("categorized", fixtureNow.Unix(), []Lot{{Item: "food", Price: 21, Quantity: 1, Count: 100}})
	e.Items = []Item{{ID: "food", Name: "Food", BindingKnown: true, ClassID: &class, SubclassID: &subtype, ClassName: "Consumable", SubclassName: "Generic"}}
	ingest(t, s, e)
	old := event("legacy-item", "items")
	old.At++
	old.Items = []Item{{ID: "food", Name: "Food", BindingKnown: true}}
	ingest(t, s, old)
	cards, err := s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || cards[0].ClassID == nil || *cards[0].ClassID != 0 || cards[0].SubclassID == nil || *cards[0].SubclassID != 0 || cards[0].SubclassName != "Generic" || len(cards[0].History) != 1 {
		t.Fatal("legacy export erased classification or created prices", cards, err)
	}
	if !strings.Contains(string(encoded(cards[0].Item)), `"classID":0`) || !strings.Contains(string(encoded(cards[0].Item)), `"subclassID":0`) {
		t.Fatal("zero classification omitted from JSON", string(encoded(cards[0].Item)))
	}
	legacy := Item{BindingKnown: true, ID: "x", Name: "X", Transferable: true}
	if string(encoded(legacy)) != `{"bindingKnown":true,"id":"x","name":"X","transferable":true}` {
		t.Fatal("legacy event encoding changed; replay hashes must remain stable", string(encoded(legacy)))
	}
}

func TestItemCategoryChangesDoNotKeepTheOldSubclassName(t *testing.T) {
	s := fixture(t)
	class, cloth, herbs := 7, 5, 9
	e := event("cloth", "items")
	e.Items = []Item{{ID: "material", Name: "Material", ClassID: &class, SubclassID: &cloth, ClassName: "Trade Goods", SubclassName: "Cloth"}}
	ingest(t, s, e)
	e.ID, e.At = "herbs", e.At+1
	e.Items[0].SubclassID, e.Items[0].SubclassName = &herbs, ""
	ingest(t, s, e)
	items, err := s.Items(fixtureMarket.Key())
	if err != nil || len(items) != 1 || items[0].SubclassName != "" || *items[0].SubclassID != herbs {
		t.Fatal("new subclass retained the old label", items, err)
	}
}

func TestInvalidItemCategoriesAreRejected(t *testing.T) {
	negative, tooLarge, valid := -1, 256, 0
	for _, item := range []Item{
		{ID: "x", Name: "X", ClassID: &negative},
		{ID: "x", Name: "X", ClassID: &tooLarge},
		{ID: "x", Name: "X", SubclassID: &valid},
		{ID: "x", Name: "X", ClassID: &valid, SubclassID: &tooLarge},
		{ID: "x", Name: "X", ClassName: strings.Repeat("x", 201)},
	} {
		e := event("invalid-category", "items")
		e.Items = []Item{item}
		if e.Validate(fixtureNow) == nil {
			t.Fatal("accepted invalid category", item)
		}
	}
}

func TestCatalogConfirmationSurvivesManyItemInspections(t *testing.T) {
	s := fixture(t)
	ingest(t, s, scan("catalog", fixtureNow.Unix(), nil))
	for i := 1; i <= 15; i++ {
		e := scan(fmt.Sprintf("item-%d", i), fixtureNow.Unix()+int64(i), nil)
		e.Coverage = "partial"
		ingest(t, s, e)
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("GET", "/api/market/state", nil))
	var result struct {
		Scans []ScanStatus `json:"scans"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if r.Code != 200 || len(result.Scans) != 10 || result.Scans[0].ID != "catalog" || !result.Scans[0].Complete {
		t.Fatal("item inspections hid catalog confirmation", r.Code, result.Scans)
	}
}

func TestItemMetadataRefreshDoesNotCreatePriceHistory(t *testing.T) {
	s := fixture(t)
	quote := scan("prices", fixtureNow.Unix(), []Lot{{Item: "2589", Price: 21, Quantity: 1, Count: 100}})
	quote.Items = []Item{{ID: "2589", Name: "2589"}}
	ingest(t, s, quote)
	meta := event("names", "items")
	meta.At = fixtureNow.Add(time.Minute).Unix()
	meta.Items = []Item{{ID: "2589", Name: "Linen Cloth", BindingKnown: true, Transferable: true}}
	ingest(t, s, meta)
	cards, err := s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || cards[0].Name != "Linen Cloth" || len(cards[0].History) != 1 || cards[0].Price != 21 {
		t.Fatal("metadata changed price history", cards, err)
	}
	miss := event("later-cache-miss", "items")
	miss.At = meta.At + 1
	miss.Items = []Item{{ID: "2589", Name: "2589"}}
	ingest(t, s, miss)
	items, err := s.Items(fixtureMarket.Key())
	if err != nil || len(items) != 1 || items[0].Name != "Linen Cloth" || !items[0].BindingKnown || !items[0].Transferable {
		t.Fatal("cache miss erased known metadata", items, err)
	}
}

func TestVariantNumericCacheMissPreservesObservedName(t *testing.T) {
	s := fixture(t)
	e := event("variant-name", "items")
	e.Items = []Item{{ID: "browse:10003:43:0:0", Name: "Black Mageweave Gloves"}}
	ingest(t, s, e)
	e.ID = "variant-cache-miss"
	e.At++
	e.Items[0].Name = "10003"
	ingest(t, s, e)
	items, err := s.Items(fixtureMarket.Key())
	if err != nil || len(items) != 1 || items[0].Name != "Black Mageweave Gloves" {
		t.Fatal("numeric alias overwrote actual name", items, err)
	}
}

func TestCompleteSummaryExpiresAbsentSupplyWithoutInventingHistory(t *testing.T) {
	s := fixture(t)
	ingest(t, s, scan("listed", fixtureNow.Unix(), []Lot{{Item: "fish", Price: 21, Quantity: 1, Count: 100}}))
	empty := scan("empty-catalog", fixtureNow.Add(time.Minute).Unix(), nil)
	empty.Coverage = "summary"
	ingest(t, s, empty)
	cards, err := s.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || cards[0].Quantity != 0 || len(cards[0].History) != 1 || cards[0].Price != 21 {
		t.Fatal("catalog absence changed history or retained supply", cards, err)
	}
}
