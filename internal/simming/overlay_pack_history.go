package simming

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/tm-LBenson/lamda-engine/internal/overlays"
)

func (s *Service) overlayPackHistory(w http.ResponseWriter, r *http.Request) {
	if s.Overlays == nil {
		failure(w, 503, fmt.Errorf("Overlays unavailable"))
		return
	}
	var result any
	var err error
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/overlays/pack-history":
		if id := r.URL.Query().Get("id"); id != "" {
			result, err = s.Overlays.PackSnapshot(id)
		} else {
			result = s.Overlays.PackHistory()
		}
	case r.Method == "POST" && (r.URL.Path == "/api/overlays/pack-history/delete" || r.URL.Path == "/api/overlays/pack-history/recover"):
		var edit struct {
			Revision int64  `json:"revision"`
			ID       string `json:"id"`
		}
		if err = decodeRequest(w, r, &edit); err == nil {
			if r.URL.Path == "/api/overlays/pack-history/recover" {
				err = s.Overlays.RecoverPackHistory(edit.Revision)
			} else {
				err = s.Overlays.DeletePackSnapshot(edit.Revision, edit.ID)
			}
		}
		if err == nil {
			result = s.Overlays.PackHistory()
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		code := 400
		if errors.Is(err, overlays.ErrConflict) {
			code = 409
		}
		if errors.Is(err, os.ErrNotExist) {
			code = 404
		}
		failure(w, code, err)
		return
	}
	jsonResponse(w, result)
}
