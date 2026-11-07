package simming

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMeterShareEndpointOnlyQueuesAUserRequest(t *testing.T) {
	s := &Service{MeterShares: make(chan struct{}, 1)}
	h := s.Handler("127.0.0.1:47641")
	request := func(header bool) int {
		r := httptest.NewRequest("POST", "http://127.0.0.1:47641/api/augmentation/share", strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		if header {
			r.Header.Set("X-Lamda-Sim", "1")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if request(false) != 403 || len(s.MeterShares) != 0 {
		t.Fatal("unprotected input request")
	}
	if request(true) != 200 || request(true) != 200 || len(s.MeterShares) != 1 {
		t.Fatal("request did not coalesce")
	}
}
