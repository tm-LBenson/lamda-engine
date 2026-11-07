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

func (s *Service) overlayMedia(w http.ResponseWriter, r *http.Request) {
	if s.Overlays == nil {
		failure(w, 503, fmt.Errorf("Overlays unavailable"))
		return
	}
	var data overlays.MediaStatus
	var err error
	switch {
	case r.URL.Path == "/api/overlays/media/file" && r.Method == "GET":
		var path, mime string
		path, mime, err = s.Overlays.MediaFile(r.URL.Query().Get("id"))
		if err == nil {
			w.Header().Set("Content-Type", mime)
			http.ServeFile(w, r, path)
			return
		}
	case r.URL.Path == "/api/overlays/media" && r.Method == "GET":
		data = s.Overlays.Media()
	case r.URL.Path == "/api/overlays/media" && r.Method == "POST":
		mime := r.Header.Get("Content-Type")
		if mime != "image/png" && mime != "image/jpeg" && mime != "audio/wav" {
			failure(w, 400, fmt.Errorf("Choose a PNG, JPG or WAV"))
			return
		}
		revision, e := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if e != nil || revision < 0 {
			failure(w, 400, fmt.Errorf("Invalid media revision"))
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, overlays.MaxMediaBytes))
		if e != nil {
			failure(w, 413, fmt.Errorf("Keep each media file under 8 MB"))
			return
		}
		data, err = s.Overlays.SaveMedia(revision, []overlays.MediaUpload{{Name: r.URL.Query().Get("name"), Data: body}})
	case r.URL.Path == "/api/overlays/media/import" && r.Method == "POST":
		var edit struct {
			Revision int64                  `json:"revision"`
			Assets   []overlays.MediaUpload `json:"assets"`
		}
		if err = decodeRequest(w, r, &edit); err == nil {
			data, err = s.Overlays.SaveMedia(edit.Revision, edit.Assets)
		}
	case r.URL.Path == "/api/overlays/media/edit" && r.Method == "POST":
		var edit struct {
			Revision int64  `json:"revision"`
			ID       string `json:"id"`
			Name     string `json:"name"`
			Remove   bool   `json:"remove"`
		}
		if err = decodeRequest(w, r, &edit); err == nil {
			data, err = s.Overlays.EditMedia(edit.Revision, edit.ID, edit.Name, edit.Remove)
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		code := 400
		if errors.Is(err, overlays.ErrMediaConflict) {
			code = 409
		}
		if errors.Is(err, os.ErrNotExist) {
			code = 404
		}
		failure(w, code, err)
		return
	}
	jsonResponse(w, data)
}
