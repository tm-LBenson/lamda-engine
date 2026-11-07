package simming

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/spellbook"
)

func TestOverlaySpellLookupAPI(t *testing.T) {
	s := &Service{}
	h := s.Handler("127.0.0.1:48797")
	for _, tc := range []struct {
		path, host, origin, method string
		status                     int
	}{
		{"/api/overlays/spells?q=Essence%20Burst&kind=auras", "127.0.0.1:48797", "", "GET", 200},
		{"/api/overlays/spells?q=x&offset=bad", "127.0.0.1:48797", "", "GET", 400},
		{"/api/overlays/spells?q=x", "evil.example", "", "GET", 403},
		{"/api/overlays/spells?q=x", "127.0.0.1:48797", "http://evil.example", "GET", 403},
		{"/api/overlays/spells?q=x", "127.0.0.1:48797", "", "POST", 403},
		{"/api/overlays/spells/license", "127.0.0.1:48797", "", "GET", 200},
	} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if w.Code == 200 && strings.Contains(tc.path, "?q=") {
			var result spellbook.Result
			if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil || result.Total < 3 {
				t.Fatal(result, e)
			}
		}
	}
	s.Workspace = &Workspace{client: "forever"}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:48797/api/overlays/spells?q=Essence", nil))
	if w.Code != 503 {
		t.Fatal("Retail data exposed as Forever data")
	}
}
