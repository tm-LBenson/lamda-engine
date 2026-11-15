package market

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStateCompressionNegotiation(t *testing.T) {
	body := strings.Repeat(`{"observedPrice":100,"name":"Copper Ore"}`, 2000)
	for _, header := range []string{"gzip, br", "gzip;q=0", "br", "gzip;q=1"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "https://market.example.com/api/market/state", nil)
		r.Header.Set("Accept-Encoding", header)
		compressedState(w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			io.WriteString(w, body)
		}))
		got := w.Body.Bytes()
		if acceptsGzip(header) {
			if w.Header().Get("Content-Encoding") != "gzip" || len(got) > len(body)/10 {
				t.Fatal("state not compressed")
			}
			reader, err := gzip.NewReader(bytes.NewReader(got))
			if err != nil {
				t.Fatal(err)
			}
			got, err = io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			reader.Close()
		} else if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("ignored encoding preference")
		}
		if string(got) != body || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("compressed response changed content/privacy")
		}
	}
}
