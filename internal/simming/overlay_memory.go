package simming

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tm-LBenson/lamda-engine/internal/overlays"
)

func (s *Service) overlayMemory(w http.ResponseWriter, r *http.Request) {
	if s.Overlays == nil {
		failure(w, 503, fmt.Errorf("Overlays unavailable"))
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/overlays/memory" {
		q := r.URL.Query()
		value, err := s.Overlays.Memory(q.Get("rule"), q.Get("key"), q.Get("character"), q.Get("realm"))
		if err != nil {
			failure(w, 400, err)
			return
		}
		jsonResponse(w, value)
		return
	}
	if r.Method != "POST" || r.URL.Path == "/api/overlays/memory" {
		http.NotFound(w, r)
		return
	}
	var edit struct {
		Revision  int64  `json:"revision"`
		Rule      string `json:"rule"`
		Key       string `json:"key"`
		Character string `json:"character"`
		Realm     string `json:"realm"`
	}
	err := decodeRequest(w, r, &edit)
	if err == nil {
		if r.URL.Path == "/api/overlays/memory/recover" {
			err = s.Overlays.RecoverMemory(edit.Revision)
		} else {
			err = s.Overlays.ResetMemory(edit.Revision, edit.Rule, edit.Key, edit.Character, edit.Realm)
		}
	}
	if err != nil {
		status := 400
		if errors.Is(err, overlays.ErrConflict) {
			status = 409
		}
		failure(w, status, err)
		return
	}
	jsonResponse(w, map[string]bool{"ok": true})
}
