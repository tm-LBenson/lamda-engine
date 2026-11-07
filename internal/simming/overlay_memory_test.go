package simming

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/overlays"
)

func TestOverlaySavedMemoryHTTP(t *testing.T) {
	dir := t.TempDir()
	rule := `{"id":"memory","name":"Memory","enabled":true,"load":{},"style":{"size":46,"opacity":100,"color":"gold","font":16,"transparent":true,"x":48,"y":40},"logic":{"values":[{"key":"count","name":"Count","expr":{"kind":"literal","value":1},"remember":{"initial":0,"when":{"event":"ability"},"reset":"saved"}}]},"roots":[{"id":"when","type":"when","props":{"event":"state"},"children":[{"id":"show","type":"show","props":{"display":"text","text":"{value:count}","visibility":"conditions"}}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "rules.json"), []byte(`{"version":1,"revision":3,"rules":[`+rule+`]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.json"), []byte(`{"version":1,"records":[{"character":"Alice","realm":"Moon Guard","rule":"memory","key":"count","value":42,"updatedAt":"2026-10-13T00:00:00Z"},{"character":"Bob","realm":"Realm","rule":"memory","key":"count","value":7,"updatedAt":"2026-10-13T00:00:00Z"}]}`), 0600); err != nil {
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
	read := func(path string) *httptest.ResponseRecorder { return req("GET", path, "", "", "") }
	post := func(body string) *httptest.ResponseRecorder {
		return req("POST", "/api/overlays/memory/reset", body, "", "1")
	}
	path := "/api/overlays/memory?rule=memory&key=count&character=Alice&realm=MoonGuard"
	w := read(path)
	var state overlays.MemoryStatus
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &state) != nil || state.Value != 42.0 || len(state.Characters) != 2 || !state.Captured {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = read("/api/overlays/characters"); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("Alice")) {
		t.Fatal("history character missing", w.Body.String())
	}
	body := `{"revision":3,"rule":"memory","key":"count","character":"Alice","realm":"MoonGuard"}`
	for _, tc := range []struct{ origin, header string }{{"http://evil.example", "1"}, {"", ""}} {
		if w = req("POST", "/api/overlays/memory/reset", body, tc.origin, tc.header); w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w = post(strings.Replace(body, `"revision":3`, `"revision":2`, 1)); w.Code != 409 {
		t.Fatal("stale reset accepted", w.Code, w.Body.String())
	}
	if w = post(strings.TrimSuffix(body, "}") + `,"unexpected":true}`); w.Code != 400 {
		t.Fatal("unknown fields accepted", w.Code)
	}
	if w = post(body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	state = overlays.MemoryStatus{}
	w = read(path)
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	if state.Captured || state.Value != 0.0 {
		t.Fatal("reset failed", w.Body.String())
	}
	other, _ := store.Memory("memory", "count", "Bob", "Realm")
	if other.Value != 7.0 {
		t.Fatal("reset another character", other)
	}
	reopened, err := overlays.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	other, _ = reopened.Memory("memory", "count", "Alice", "MoonGuard")
	if other.Captured {
		t.Fatal("reset not durable")
	}
	if w = req("POST", "/api/overlays/memory/recover", `{"revision":3}`, "", "1"); w.Code != 400 {
		t.Fatal("healthy recovery accepted", w.Code)
	}
}
