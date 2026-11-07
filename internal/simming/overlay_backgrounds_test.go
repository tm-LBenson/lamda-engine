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

func TestOverlayBackgroundHTTP(t *testing.T) {
	store, _ := overlays.Open(t.TempDir())
	s := &Service{Overlays: store}
	handler := s.Handler("127.0.0.1:48797")
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 160, 90)))
	request := func(method, path, host, origin, header, mime string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://"+host+path, bytes.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Lamda-Sim", header)
		r.Header.Set("Content-Type", mime)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	endpoint := "/api/overlays/backgrounds?revision=0&character=Test&realm=Realm"
	for _, v := range []struct{ host, origin, header, mime string }{
		{"evil.example", "", "1", "image/png"}, {"127.0.0.1:48797", "http://evil.example", "1", "image/png"}, {"127.0.0.1:48797", "", "", "image/png"}, {"127.0.0.1:48797", "", "1", "text/plain"},
	} {
		w := request("POST", endpoint, v.host, v.origin, v.header, v.mime, pngData.Bytes())
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := request("POST", endpoint, "127.0.0.1:48797", "", "1", "image/png", pngData.Bytes())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var d overlays.BackgroundLibrary
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	v := d.Images[0]
	w = request("GET", "/api/overlays/background?id="+v.ID+"&version="+v.Image, "127.0.0.1:48797", "", "", "", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(w.Body.Bytes(), pngData.Bytes()) {
		t.Fatal("image response", w.Code)
	}
	w = request("POST", endpoint, "127.0.0.1:48797", "", "1", "image/png", pngData.Bytes())
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	w = request("POST", "/api/overlays", "127.0.0.1:48797", "", "1", "image/png", pngData.Bytes())
	if w.Code != 403 {
		t.Fatal("image exception escaped endpoint", w.Code)
	}
	w = request("POST", "/api/overlays/backgrounds?revision=1&character=Test&realm=Realm", "127.0.0.1:48797", "", "1", "image/png", make([]byte, overlays.MaxBackgroundBytes+1))
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
	w = request("POST", "/api/overlays/backgrounds/remove", "127.0.0.1:48797", "", "1", "application/json", []byte(`{"revision":1,"id":"`+v.ID+`"}`))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/overlays/background?id="+v.ID+"&version="+v.Image, "127.0.0.1:48797", "", "", "", nil)
	if w.Code != 404 {
		t.Fatal("removed image served", w.Code)
	}
}
