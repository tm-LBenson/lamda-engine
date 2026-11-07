package simming

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/overlays"
)

func TestOverlayPackHistoryHTTP(t *testing.T) {
	dir := t.TempDir()
	const rule = `{"id":"one","name":"One","enabled":false,"load":{},"style":{"size":46,"opacity":100,"color":"gold","font":16,"transparent":true,"x":48,"y":40},"roots":[{"id":"when","type":"when","props":{"event":"state"},"children":[{"id":"show","type":"show","props":{"display":"text","text":"Hi","visibility":"conditions"}}]}],"pack":{"id":"author","instance":"local","member":"one","members":["one"],"name":"Pack","version":1,"nodes":{"when":"when","show":"show"}}}`
	if err := os.WriteFile(filepath.Join(dir, "rules.json"), []byte(`{"version":1,"revision":3,"rules":[`+rule+`]}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := overlays.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler := (&Service{Overlays: store}).Handler("127.0.0.1:48797")
	req := func(method, path, body, origin, header string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:48797"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Lamda-Sim", header)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	post := func(path, body string) *httptest.ResponseRecorder { return req("POST", path, body, "", "1") }
	read := func(path string) *httptest.ResponseRecorder { return req("GET", path, "", "", "") }
	base := "/api/overlays/pack-history"
	body := `{"revision":3,"rules":[` + strings.Replace(rule, `"version":1`, `"version":2`, 1) + `],"packUpdates":["local"]}`
	for _, tc := range []struct{ origin, header string }{{"http://evil.example", "1"}, {"", ""}} {
		for _, path := range []string{"/api/overlays", base + "/delete", base + "/recover"} {
			if w := req("POST", path, body, tc.origin, tc.header); w.Code != 403 {
				t.Fatal(path, w.Code, w.Body.String())
			}
		}
	}
	if w := post("/api/overlays", strings.Replace(body, `"revision":3`, `"revision":2`, 1)); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(store.PackHistory().Entries) != 0 {
		t.Fatal("failed request made backup")
	}
	if w := post("/api/overlays", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := read(base)
	var h overlays.PackHistoryStatus
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &h) != nil || len(h.Entries) != 1 || h.Entries[0].Version != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = read(base + "?id=" + h.Entries[0].ID)
	var snapshot overlays.PackSnapshot
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &snapshot) != nil || snapshot.Rules[0].Pack.Version != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = read(base + "?id=missing"); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	edit, _ := json.Marshal(map[string]any{"revision": h.Revision - 1, "id": snapshot.ID})
	if w = post(base+"/delete", string(edit)); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	edit, _ = json.Marshal(map[string]any{"revision": h.Revision, "id": snapshot.ID, "unknown": true})
	if w = post(base+"/delete", string(edit)); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	edit, _ = json.Marshal(map[string]any{"revision": h.Revision, "id": snapshot.ID})
	if w = post(base+"/delete", string(edit)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = read(base + "?id=" + snapshot.ID); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err = os.WriteFile(filepath.Join(dir, "pack-history.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err = overlays.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	handler = (&Service{Overlays: store}).Handler("127.0.0.1:48797")
	w = read(base)
	h = overlays.PackHistoryStatus{}
	_ = json.Unmarshal(w.Body.Bytes(), &h)
	if w.Code != 200 || h.Error == "" || !h.Recoverable {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = post(base+"/recover", `{"revision":0}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "pack-history.corrupt-*.json"))
	if len(backups) != 1 {
		t.Fatal("corrupt backup missing")
	}
	if w = post(base+"/recover", `{"revision":1}`); w.Code != 400 {
		t.Fatal("healthy recovery accepted", w.Code, w.Body.String())
	}
}
