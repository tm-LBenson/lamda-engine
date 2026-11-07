package simming

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/spellbook"
)

func (s *Service) overlayIcon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("spell"))
	if err != nil || id < 1 || id >= 10000000 {
		failure(w, 400, fmt.Errorf("Invalid spell ID"))
		return
	}
	if s.Workspace != nil && s.Workspace.client != "retail" {
		failure(w, 503, fmt.Errorf("Spell icons are available for Retail"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	if path := s.Overlays.Icon(ctx, id); path != "" {
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeFile(w, r, path)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, max-age=30")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="5" fill="#1b2b35"/><text x="32" y="46" text-anchor="middle" font-family="sans-serif" font-size="42" fill="#f4c879">?</text></svg>`)
}

func (s *Service) overlaySpells(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/overlays/spells/license" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, spellbook.Attribution())
		return
	}
	if s.Workspace != nil && s.Workspace.client != "retail" {
		failure(w, 503, fmt.Errorf("Spell search is available for Retail"))
		return
	}
	q := r.URL.Query()
	offset := 0
	if q.Get("offset") != "" {
		var err error
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil {
			failure(w, 400, fmt.Errorf("Invalid spell search page"))
			return
		}
	}
	result, err := spellbook.Search(spellbook.Options{Query: q.Get("q"), Class: q.Get("class"), Prefer: q.Get("prefer"), Kind: q.Get("kind"), Offset: offset})
	if err != nil {
		failure(w, 400, err)
		return
	}
	jsonResponse(w, result)
}
