package market

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var web embed.FS

func Files() http.Handler { sub, _ := fs.Sub(web, "web"); return http.FileServer(http.FS(sub)) }
func response(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	response(w, map[string]string{"error": err.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		return err
	}
	return strictJSON(b, v)
}

type ScanStatus struct {
	ID       string `json:"id"`
	At       int64  `json:"at"`
	Pages    int    `json:"pages"`
	Received int    `json:"received"`
	Complete bool   `json:"complete"`
	Coverage string `json:"coverage"`
}

func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	profile := requestProfile(r, s)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/market" {
		http.Redirect(w, r, "/market/", 307)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/market/") && r.Method == "GET" {
		http.StripPrefix("/market/", Files()).ServeHTTP(w, r)
		return
	}
	if r.Method == "POST" && (r.Header.Get("X-Lamda-Sim") != "1" || r.Header.Get("Content-Type") != "application/json") {
		problem(w, 403, fmt.Errorf("use local market controls"))
		return
	}
	if r.URL.Path == "/api/market/characters/export" && r.Method == "GET" {
		key := r.URL.Query().Get("market")
		rows, err := profile.db.Query(`SELECT market,at,body FROM documents WHERE kind='character' AND market=? ORDER BY id`, key)
		if err != nil {
			problem(w, 500, err)
			return
		}
		defer rows.Close()
		events := []Event{}
		for rows.Next() {
			var market string
			var at int64
			var raw []byte
			if err = rows.Scan(&market, &at, &raw); err != nil {
				problem(w, 500, err)
				return
			}
			var c Character
			if err = json.Unmarshal(raw, &c); err != nil {
				problem(w, 500, err)
				return
			}
			parts := splitKey(market)
			if len(parts) != 4 {
				continue
			}
			sum := sha256.Sum256(raw)
			events = append(events, Event{Version: 1, ID: fmt.Sprintf("character-export-%x", sum[:16]), Market: Identity{parts[0], parts[1], parts[2], parts[3]}, At: at, Kind: "character", Character: &c})
		}
		if err = rows.Err(); err != nil {
			problem(w, 500, err)
			return
		}
		response(w, map[string]any{"events": events})
		return
	}
	if r.URL.Path == "/api/market/publisher" {
		if profile != s {
			http.NotFound(w, r)
			return
		}
		s.publisherHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/market/status" && r.Method == "GET" {
		response(w, map[string]any{"capture": s.Capture(), "revision": marketRevision(s, profile)})
		return
	}
	if r.URL.Path == "/api/market/icon" && r.Method == "GET" {
		id, err := strconv.Atoi(r.URL.Query().Get("item"))
		if err != nil || id < 1 || id >= 10000000 {
			problem(w, 400, fmt.Errorf("invalid item ID"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		if path := s.icons.Get(ctx, id); path != "" {
			w.Header().Set("Cache-Control", "private, max-age=86400")
			http.ServeFile(w, r, path)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "private, max-age=30")
		fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="3" fill="#202b30"/><text x="16" y="23" text-anchor="middle" font-family="sans-serif" font-size="22" fill="#8a9a9f">?</text></svg>`)
		return
	}
	if r.URL.Path == "/api/market/import" && r.Method == "POST" {
		var v struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := decode(w, r, &v); err != nil {
			problem(w, 400, err)
			return
		}
		if len(v.Events) == 0 || len(v.Events) > 2048 {
			problem(w, 400, fmt.Errorf("supply 1–2048 events"))
			return
		}
		for _, b := range v.Events {
			var e Event
			if len(b) > MaxEventBytes {
				problem(w, 400, fmt.Errorf("event too large"))
				return
			}
			if err := strictJSON(b, &e); err != nil {
				problem(w, 400, err)
				return
			}
			if err := e.Validate(time.Now()); err != nil || e.Market.Client != s.client {
				problem(w, 400, fmt.Errorf("invalid event or wrong client: %v", err))
				return
			}
		}
		n := 0
		for _, b := range v.Events {
			if err := s.Ingest(b, time.Now()); err != nil {
				problem(w, 400, fmt.Errorf("%d events accepted before error: %w", n, err))
				return
			}
			n++
		}
		response(w, map[string]int{"accepted": n})
		return
	}
	if r.URL.Path == "/api/market/state" && r.Method == "GET" {
		markets, err := s.Markets()
		if err != nil {
			problem(w, 500, err)
			return
		}
		key := r.URL.Query().Get("market")
		if key == "" && len(markets) > 0 {
			key = markets[0].Key()
		}
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		if days < 1 || days > 365 {
			days = 7
		}
		cards, err := s.cardsFor(profile, key, time.Now().Add(-time.Duration(days)*24*time.Hour).Unix())
		if err != nil {
			problem(w, 500, err)
			return
		}
		chars := []Character{}
		recipes := []Recipe{}
		vendors := []Vendor{}
		if err = profile.documents(key, "character", &chars); err != nil {
			problem(w, 500, err)
			return
		}
		if recipes, err = s.recipesFor(profile, key); err != nil {
			problem(w, 500, err)
			return
		}
		if err = s.documents(key, "vendor", &vendors); err != nil {
			problem(w, 500, err)
			return
		}
		opportunities, err := s.opportunitiesFor(profile, key, time.Now())
		if err != nil {
			problem(w, 500, err)
			return
		}
		trades, err := profile.Trades(key)
		if err != nil {
			problem(w, 500, err)
			return
		}
		scans := []ScanStatus{}
		rows, err := s.db.Query(`SELECT id,at,pages,complete,coverage,(SELECT count(*) FROM pages p WHERE p.market=scans.market AND p.scan=scans.id) FROM scans WHERE market=? ORDER BY CASE WHEN coverage IN ('full','summary') THEN 0 ELSE 1 END, at DESC LIMIT 10`, key)
		if err != nil {
			problem(w, 500, err)
			return
		}
		for rows.Next() {
			var v ScanStatus
			var n int
			rows.Scan(&v.ID, &v.At, &v.Pages, &n, &v.Coverage, &v.Received)
			v.Complete = n == 1
			scans = append(scans, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			problem(w, 500, err)
			return
		}
		items, err := s.Items(key)
		if err != nil {
			problem(w, 500, err)
			return
		}
		lists, err := profile.Lists(key, requestUser(r))
		if err != nil {
			problem(w, 500, err)
			return
		}
		response(w, map[string]any{"revision": marketRevision(s, profile), "lists": lists, "items": items, "capture": s.Capture(), "client": s.client, "hosted": profile != s, "market": key, "markets": markets, "cards": cards, "settings": profile.Settings(key), "characters": chars, "recipes": recipes, "vendors": vendors, "opportunities": opportunities, "trades": trades, "scans": scans, "at": time.Now().Unix()})
		return
	}
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	key := r.URL.Query().Get("market")
	parts := splitKey(key)
	if len(parts) != 4 || parts[0] != s.client {
		problem(w, 400, fmt.Errorf("choose a market first"))
		return
	}
	if err := (Identity{parts[0], parts[1], parts[2], parts[3]}).Validate(); err != nil {
		problem(w, 400, err)
		return
	}
	var err error
	switch r.URL.Path {
	case "/api/market/lists/create", "/api/market/lists/rename":
		var v struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		err = decode(w, r, &v)
		if err == nil {
			err = profile.SaveList(key, requestUser(r), v.ID, v.Name, r.URL.Path == "/api/market/lists/create")
		}
	case "/api/market/lists/delete":
		var v struct {
			ID string `json:"id"`
		}
		err = decode(w, r, &v)
		if err == nil {
			err = profile.DeleteList(key, requestUser(r), v.ID)
		}
	case "/api/market/lists/items":
		var v struct {
			Lists []string `json:"lists"`
			Items []string `json:"items"`
			Mode  string   `json:"mode"`
		}
		err = decode(w, r, &v)
		if err == nil {
			err = profile.ListItems(key, requestUser(r), v.Lists, v.Items, v.Mode)
		}
	case "/api/market/lists/move":
		var v struct {
			List   string `json:"list"`
			Item   string `json:"item"`
			Before string `json:"before"`
		}
		err = decode(w, r, &v)
		if err == nil {
			err = profile.MoveListItem(key, requestUser(r), v.List, v.Item, v.Before)
		}
	case "/api/market/settings":
		var v Settings
		err = decode(w, r, &v)
		if err == nil {
			err = profile.SaveSettings(key, v)
		}
	case "/api/market/watch":
		var v struct {
			Item string `json:"item"`
			On   bool   `json:"on"`
		}
		err = decode(w, r, &v)
		if err == nil {
			mode := "remove"
			if v.On {
				mode = "add"
			}
			err = profile.ListItems(key, requestUser(r), []string{DefaultListID}, []string{v.Item}, mode)
		}
	case "/api/market/recipe":
		var recipe Recipe
		err = decode(w, r, &recipe)
		if err == nil {
			err = profile.SaveRecipe(key, recipe)
		}
	case "/api/market/recipe/reset":
		var v struct {
			ID string `json:"id"`
		}
		err = decode(w, r, &v)
		if err == nil {
			err = profile.ResetRecipe(key, v.ID)
		}
	case "/api/market/trade/edit":
		var t Trade
		err = decode(w, r, &t)
		if err == nil {
			err = profile.EditTrade(key, t)
		}
	case "/api/market/trade/remove":
		var v struct {
			ID string `json:"id"`
		}
		err = decode(w, r, &v)
		if err == nil {
			err = profile.RemoveTrade(key, v.ID)
		}
	case "/api/market/trade":
		var v Trade
		err = decode(w, r, &v)
		if err == nil {
			err = profile.Trade(key, v)
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		problem(w, 400, err)
		return
	}
	s.revision.Add(1)
	response(w, map[string]bool{"ok": true})
}

// LocalHandler provides the same host/origin boundary as the Engine workspace.
func (s *Store) LocalHandler(host string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Host != host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+host) {
			problem(w, 403, fmt.Errorf("local origin required"))
			return
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/market/", 307)
			return
		}
		s.ServeHTTP(w, r)
	})
}
