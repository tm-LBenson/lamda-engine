package simming

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/tm-LBenson/lamda-engine/internal/overlays"
)

func (s *Service) overlayBackgrounds(w http.ResponseWriter, r *http.Request) {
	if s.Overlays == nil {
		failure(w, 503, fmt.Errorf("Overlays unavailable"))
		return
	}
	var library overlays.BackgroundLibrary
	var err error
	switch {
	case r.URL.Path == "/api/overlays/background" && r.Method == "GET":
		var path, mime string
		path, mime, err = s.Overlays.BackgroundFile(r.URL.Query().Get("id"), r.URL.Query().Get("version"))
		if err == nil {
			w.Header().Set("Content-Type", mime)
			http.ServeFile(w, r, path)
			return
		}
	case r.URL.Path == "/api/overlays/backgrounds" && r.Method == "GET":
		library, err = s.Overlays.Backgrounds()
	case r.URL.Path == "/api/overlays/backgrounds" && r.Method == "POST":
		if r.Header.Get("Content-Type") != "image/png" && r.Header.Get("Content-Type") != "image/jpeg" {
			failure(w, 400, fmt.Errorf("Choose a PNG or JPG screenshot"))
			return
		}
		revision, e := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if e != nil || revision < 0 {
			failure(w, 400, fmt.Errorf("Invalid screenshot revision"))
			return
		}
		data, e := io.ReadAll(http.MaxBytesReader(w, r.Body, overlays.MaxBackgroundBytes))
		if e != nil {
			code := 400
			var limit *http.MaxBytesError
			if errors.As(e, &limit) {
				code = 413
			}
			failure(w, code, fmt.Errorf("Choose a PNG or JPG screenshot smaller than 32 MB"))
			return
		}
		library, err = s.Overlays.SaveBackground(revision, r.URL.Query().Get("character"), r.URL.Query().Get("realm"), data)
	case r.URL.Path == "/api/overlays/backgrounds/remove" && r.Method == "POST":
		var edit struct {
			ID       string `json:"id"`
			Revision int64  `json:"revision"`
		}
		if err = decodeRequest(w, r, &edit); err == nil {
			library, err = s.Overlays.RemoveBackground(edit.Revision, edit.ID)
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		code := 400
		if errors.Is(err, overlays.ErrBackgroundConflict) {
			code = 409
		}
		if errors.Is(err, os.ErrNotExist) {
			code = 404
		}
		failure(w, code, err)
		return
	}
	jsonResponse(w, library)
}
