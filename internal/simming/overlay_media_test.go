package simming

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/overlays"
)

func TestOverlayMediaHTTPIsolationImportAndRevisions(t *testing.T) {
	store, _ := overlays.Open(t.TempDir())
	handler := (&Service{Overlays: store}).Handler("127.0.0.1:48797")
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	request := func(method, path, host, origin, header, mime string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://"+host+path, bytes.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Lamda-Sim", header)
		r.Header.Set("Content-Type", mime)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	post := func(path, mime string, body []byte) *httptest.ResponseRecorder {
		return request("POST", path, "127.0.0.1:48797", "", "1", mime, body)
	}
	endpoint := "/api/overlays/media?revision=0&name=Image"
	for _, v := range []struct{ host, origin, header, mime string }{{"evil.example", "", "1", "image/png"}, {"127.0.0.1:48797", "http://evil.example", "1", "image/png"}, {"127.0.0.1:48797", "", "", "image/png"}, {"127.0.0.1:48797", "", "1", "text/plain"}} {
		if w := request("POST", endpoint, v.host, v.origin, v.header, v.mime, pngData.Bytes()); w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := post(endpoint, "image/png", pngData.Bytes())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var lib overlays.MediaStatus
	if err := json.Unmarshal(w.Body.Bytes(), &lib); err != nil || len(lib.Assets) != 1 {
		t.Fatal("bad library", err)
	}
	id := lib.Assets[0].ID
	w = request("GET", "/api/overlays/media/file?id="+id, "127.0.0.1:48797", "", "", "", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(w.Body.Bytes(), pngData.Bytes()) {
		t.Fatal("wrong image response", w.Code)
	}
	if w = post(endpoint, "image/png", pngData.Bytes()); w.Code != 409 {
		t.Fatal("stale upload accepted")
	}
	for _, path := range []string{"/api/overlays", "/api/overlays/media/import", "/api/overlays/media/edit"} {
		if w = post(path, "image/png", pngData.Bytes()); w.Code != 403 {
			t.Fatal("raw type escaped upload endpoint", path, w.Code)
		}
	}
	body, _ := json.Marshal(map[string]any{"revision": 1, "assets": []overlays.MediaUpload{{ID: id, Name: "Copy", Data: pngData.Bytes()}}})
	if w = post("/api/overlays/media/import", "application/json", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = post("/api/overlays/media/edit", "application/json", []byte(`{"revision":2,"id":"`+id+`","name":"Renamed"}`)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = post("/api/overlays/media/edit", "application/json", []byte(`{"revision":3,"id":"`+id+`","remove":true}`)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/overlays/media/file?id="+id, "127.0.0.1:48797", "", "", "", nil)
	if w.Code != 404 {
		t.Fatal("deleted file served")
	}
	if w = post("/api/overlays/media?revision=4&name=Huge", "image/png", make([]byte, overlays.MaxMediaBytes+1)); w.Code != 413 {
		t.Fatal("upload limit", w.Code)
	}
	if w = post("/api/overlays/media/import", "application/json", []byte(`{"revision":4,"assets":[],"path":"other"}`)); w.Code != 400 {
		t.Fatal("unknown import field accepted")
	}
}
