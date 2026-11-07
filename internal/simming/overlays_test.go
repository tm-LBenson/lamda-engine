package simming

import (
	"bytes"
	"context"
	"github.com/tm-LBenson/lamda-engine/internal/overlays"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOverlaysManagerSecurity(t *testing.T) {
	store, _ := overlays.Open(t.TempDir())
	s := &Service{Overlays: store}
	handler := s.Handler("127.0.0.1:48797")
	for _, tc := range []struct {
		path, method, host, origin, header string
		code                               int
	}{{"/overlays/", "GET", "127.0.0.1:48797", "", "", 200}, {"/api/overlays", "GET", "evil.example", "", "", 403}, {"/api/overlays", "POST", "127.0.0.1:48797", "http://evil.example", "1", 403}, {"/api/overlays", "POST", "127.0.0.1:48797", "", "", 403}, {"/api/overlays", "POST", "127.0.0.1:48797", "", "1", 200}} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, bytes.NewBufferString(`{"revision":0,"rules":[]}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Lamda-Sim", tc.header)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s %s %d: %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:48797/api/overlays", bytes.NewBufferString(`{"revision":0,"rules":[]}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Lamda-Sim", "1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("conflict not exposed", w.Code)
	}
}

// Explicit opt-in local harness, never part of a normal test run.
func TestServeOverlaysBrowser(t *testing.T) {
	dir := os.Getenv("LAMDA_OVERLAYS_BROWSER_DIR")
	if dir == "" {
		t.Skip("browser harness")
	}
	store, e := overlays.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Overlays: store, Context: context.Background()}
	t.Fatal(http.ListenAndServe("127.0.0.1:48797", s.Handler("127.0.0.1:48797")))
}
