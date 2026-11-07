package simming

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/augmentation"
)

func TestAugmentationReadRoutesAndOriginGuards(t *testing.T) {
	s := &Service{Augmentation: augmentation.New("")}
	handler := s.Handler("127.0.0.1:48797")
	for _, path := range []string{"/api/augmentation", "/augmentation", "/augmentation.js", "/augmentation.css"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:48797"+path, nil))
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if path == "/api/augmentation" && !strings.Contains(w.Body.String(), `"enabled":false`) {
			t.Fatal(w.Body.String())
		}
	}
	for _, test := range []struct{ url, method, origin string }{{"http://evil.example/api/augmentation", "GET", ""}, {"http://127.0.0.1:48797/api/augmentation", "GET", "https://evil.example"}, {"http://127.0.0.1:48797/api/augmentation", "POST", ""}} {
		r := httptest.NewRequest(test.method, test.url, nil)
		r.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code == 200 {
			t.Fatal("unauthorized method/origin accepted", test)
		}
	}
}

func TestMeterBreakdownRouteReturnsRecordedCreditsAndGuardsMissingData(t *testing.T) {
	dir := t.TempDir()
	owner := "Player-1-Aug"
	ally := "Player-1-Ally"
	segment := "boss/123/1"
	state := augmentation.State{Schema: 1, Client: "retail", DamageMeters: []augmentation.DamageMeter{{ID: segment, Owner: owner, Kind: "boss", Valid: true, RawTotal: 400, AdjustedTotal: 400,
		Rows: []augmentation.DamageRow{{GUID: owner, Name: "Aug", Raw: 100, Added: 200, Adjusted: 300}, {GUID: ally, Name: "Ally", Raw: 300, Removed: 200, Adjusted: 100}}}}}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "reports.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	details := map[string]any{"schema": 1, "client": "retail", "scenes": []any{map[string]any{"owner": owner, "segment": segment, "entries": []augmentation.DamageDetailEntry{
		{Source: owner, SpellID: 10, SpellName: "Attack", Amount: 100, TargetName: "Boss"},
		{Source: ally, SpellID: 20, SpellName: "Attack", Amount: 300, TargetName: "Boss"},
		{Source: ally, SourceName: "Ally", SupportOwner: owner, SpellID: 395152, SpellName: "Ebon Might", Amount: 200, TargetName: "Boss", OriginalSpell: 20, OriginalName: "Attack"},
	}}}}
	data, err = json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meter-details.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	s := &Service{Augmentation: augmentation.New(path)}
	defer s.Augmentation.Close()
	h := s.Handler("127.0.0.1:48797")
	q := url.Values{"owner": {owner}, "segment": {segment}, "actor": {owner}}
	endpoint := "http://127.0.0.1:48797/api/augmentation/breakdown?" + q.Encode()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", endpoint, nil))
	var result augmentation.MeterBreakdown
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !result.Complete || result.Total != 300 || result.Expected != 300 {
		t.Fatal(w.Code, result, w.Body.String())
	}
	q.Set("actor", "Player-1-Unknown")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:48797/api/augmentation/breakdown?"+q.Encode(), nil))
	if w.Code != 404 {
		t.Fatal("unknown actor became zero details", w.Code, w.Body.String())
	}
	r := httptest.NewRequest("GET", endpoint, nil)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatal("foreign origin read accepted")
	}
}
