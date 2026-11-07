package simming

import (
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/kbb"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOverlayCharacterPickerSourcesAndIsolation(t *testing.T) {
	planner, err := kbb.Open(t.TempDir(), "retail")
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range [][2]string{{"Lamda", "Area 52"}, {"Lamda", "Quel'Thalas"}} {
		guid := []string{"Player-1-one", "Player-1-two"}[i]
		if err := planner.Merge(kbb.Snapshot{Schema: 1, Client: "retail", GUID: guid, Name: c[0], Realm: c[1], Records: []kbb.Record{}}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	names := planner.CharacterNames()
	names[0][0] = "Changed copy"
	s := &Service{Planner: planner, Workspace: &Workspace{snapshots: map[string]*Snapshot{
		"old":   {Character: Character{Name: "lamda", Realm: "area-52"}, Profile: "private gear"},
		"alt":   {Character: Character{Name: "Alt", Realm: "Azjol-Nerub"}},
		"blank": {Character: Character{Name: "Unknown"}},
	}}}
	handler := s.Handler("127.0.0.1:48797")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:48797/api/overlays/characters", nil))
	var got struct {
		Characters []map[string]string `json:"characters"`
	}
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Characters) != 3 {
		t.Fatal(got)
	}
	for _, c := range got.Characters {
		if len(c) != 2 || c["character"] == "Changed copy" || c["realm"] == "" {
			t.Fatal(c)
		}
	}
	if strings.Contains(w.Body.String(), "private gear") {
		t.Fatal("leaked profile")
	}
	bad := httptest.NewRequest("GET", "http://evil.example/api/overlays/characters", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, bad)
	if w.Code != 403 {
		t.Fatal("host guard", w.Code)
	}
	w = httptest.NewRecorder()
	(&Service{}).Handler("127.0.0.1:48797").ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:48797/api/overlays/characters", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"characters":[]`) {
		t.Fatal(w.Body.String())
	}
}
