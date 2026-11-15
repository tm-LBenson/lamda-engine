package market

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Collector struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
	Enabled   bool   `json:"enabled"`
	LastSeen  int64  `json:"lastSeen"`
}
type SharedServer struct {
	Store            *Store
	Profiles         *Profiles
	Access           *Access
	Host, OwnerEmail string
	app              http.Handler
}

func NewSharedServer(store *Store, profiles *Profiles, access *Access, host, ownerEmail string) (*SharedServer, error) {
	if !strings.Contains(ownerEmail, "@") || strings.ContainsAny(host, "/: \r\n") {
		return nil, fmt.Errorf("configure host and owner email")
	}
	_, err := store.db.Exec(`CREATE TABLE IF NOT EXISTS collectors(id TEXT PRIMARY KEY,name TEXT,public_key TEXT,enabled INTEGER,last_seen INTEGER DEFAULT 0);CREATE TABLE IF NOT EXISTS collector_nonces(collector TEXT,nonce TEXT,at INTEGER,PRIMARY KEY(collector,nonce));`)
	if err != nil {
		return nil, err
	}
	return &SharedServer{Store: store, Profiles: profiles, Access: access, Host: host, OwnerEmail: strings.ToLower(ownerEmail), app: store.HostedHandler(host, access, profiles)}, nil
}
func collectorID(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}
func publicEvent(e Event) bool {
	return e.Kind == "auction" || e.Kind == "items" || e.Kind == "vendor" || e.Kind == "recipes"
}
func signedMessage(host, at, nonce string, body []byte) []byte {
	digest := sha256.Sum256(body)
	return []byte("LMKSYNC1\n" + host + "\n" + at + "\n" + nonce + "\n" + hex.EncodeToString(digest[:]))
}
func (h *SharedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	compressedState(w, r, http.HandlerFunc(h.serveHTTP))
}
func (h *SharedServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/healthz" && r.Method == "GET" {
		response(w, map[string]bool{"ok": true})
		return
	}
	if r.Host != h.Host {
		problem(w, 403, fmt.Errorf("invalid host"))
		return
	}
	if r.URL.Path == "/api/collector/import" {
		h.collect(w, r)
		return
	}
	if r.URL.Path != "/api/market/collectors" && r.URL.Path != "/api/market/characters/import" && r.URL.Path != "/api/market/shared/import" && r.URL.Path != "/api/market/account" {
		h.app.ServeHTTP(w, r)
		return
	}
	who, err := h.Access.Verify(r.Context(), r.Header.Get("Cf-Access-Jwt-Assertion"), time.Now())
	if err != nil {
		problem(w, 401, err)
		return
	}
	owner := who.Email == h.OwnerEmail
	if r.URL.Path == "/api/market/account" && r.Method == "GET" {
		response(w, map[string]any{"email": who.Email, "owner": owner})
		return
	}
	if r.Method == "POST" && (r.Header.Get("Origin") != "https://"+h.Host || r.Header.Get("Sec-Fetch-Site") == "cross-site" || r.Header.Get("X-Lamda-Sim") != "1" || r.Header.Get("Content-Type") != "application/json") {
		problem(w, 403, fmt.Errorf("use this site's controls"))
		return
	}
	if r.URL.Path == "/api/market/characters/import" && r.Method == "POST" {
		profile, err := h.Profiles.Get(who.Subject)
		if err != nil {
			problem(w, 503, fmt.Errorf("profile unavailable"))
			return
		}
		h.importEvents(w, r, profile, false)
		return
	}
	if !owner {
		problem(w, 403, fmt.Errorf("only the owner can manage market collectors"))
		return
	}
	if r.URL.Path == "/api/market/shared/import" && r.Method == "POST" {
		h.importEvents(w, r, h.Store, true)
		return
	}
	if r.URL.Path == "/api/market/collectors" && r.Method == "GET" {
		rows, err := h.Store.db.Query(`SELECT id,name,public_key,enabled,last_seen FROM collectors ORDER BY name,id`)
		if err != nil {
			problem(w, 500, fmt.Errorf("collectors unavailable"))
			return
		}
		defer rows.Close()
		list := []Collector{}
		for rows.Next() {
			var c Collector
			if err = rows.Scan(&c.ID, &c.Name, &c.PublicKey, &c.Enabled, &c.LastSeen); err != nil {
				problem(w, 500, err)
				return
			}
			list = append(list, c)
		}
		response(w, list)
		return
	}
	if r.URL.Path == "/api/market/collectors" && r.Method == "POST" {
		var c Collector
		if err = decode(w, r, &c); err != nil {
			problem(w, 400, err)
			return
		}
		pub, err := base64.RawURLEncoding.DecodeString(c.PublicKey)
		c.Name = strings.TrimSpace(c.Name)
		if err != nil || len(pub) != ed25519.PublicKeySize || !validListName(c.Name) {
			problem(w, 400, fmt.Errorf("enter a valid collector name and public key"))
			return
		}
		c.ID = collectorID(pub)
		_, err = h.Store.db.Exec(`INSERT INTO collectors(id,name,public_key,enabled) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,enabled=excluded.enabled`, c.ID, c.Name, c.PublicKey, c.Enabled)
		if err != nil {
			problem(w, 500, fmt.Errorf("collector could not be saved"))
			return
		}
		response(w, c)
		return
	}
	w.WriteHeader(405)
}
func (h *SharedServer) importEvents(w http.ResponseWriter, r *http.Request, dest *Store, public bool) {
	var batch struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := decode(w, r, &batch); err != nil {
		problem(w, 400, err)
		return
	}
	if len(batch.Events) == 0 || len(batch.Events) > 2048 {
		problem(w, 400, fmt.Errorf("supply 1–2048 events"))
		return
	}
	for _, raw := range batch.Events {
		var e Event
		if len(raw) > MaxEventBytes || strictJSON(raw, &e) != nil || e.Validate(time.Now()) != nil || e.Market.Client != dest.client || public && (!publicEvent(e) || e.Character != nil) || !public && e.Kind != "character" {
			problem(w, 400, fmt.Errorf("this import contains an invalid or unauthorized event"))
			return
		}
	}
	for _, raw := range batch.Events {
		if err := dest.Ingest(raw, time.Now()); err != nil {
			problem(w, 400, err)
			return
		}
	}
	response(w, map[string]int{"accepted": len(batch.Events)})
}
func (h *SharedServer) collect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		problem(w, 400, fmt.Errorf("send JSON"))
		return
	}
	id, at, nonce := r.Header.Get("X-Lamda-Collector"), r.Header.Get("X-Lamda-Time"), r.Header.Get("X-Lamda-Nonce")
	ts, err := strconv.ParseInt(at, 10, 64)
	now := time.Now()
	if err != nil || ts < now.Add(-5*time.Minute).Unix() || ts > now.Add(time.Minute).Unix() || len(nonce) != 32 || len(id) != 64 {
		problem(w, 401, fmt.Errorf("invalid collector signature"))
		return
	}
	var pub64 string
	var enabled bool
	if err = h.Store.db.QueryRow(`SELECT public_key,enabled FROM collectors WHERE id=?`, id).Scan(&pub64, &enabled); err != nil || !enabled {
		problem(w, 401, fmt.Errorf("collector is not authorized"))
		return
	}
	pub, err := base64.RawURLEncoding.DecodeString(pub64)
	sig, e2 := base64.RawURLEncoding.DecodeString(r.Header.Get("X-Lamda-Signature"))
	body, e3 := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil || e2 != nil || e3 != nil || len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, signedMessage(h.Host, at, nonce, body), sig) {
		problem(w, 401, fmt.Errorf("invalid collector signature"))
		return
	}
	_, _ = h.Store.db.Exec(`DELETE FROM collector_nonces WHERE at<?`, now.Add(-10*time.Minute).Unix())
	if _, err = h.Store.db.Exec(`INSERT INTO collector_nonces VALUES(?,?,?)`, id, nonce, ts); err != nil {
		problem(w, 409, fmt.Errorf("request already used"))
		return
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	h.importEvents(w, r, h.Store, true)
	_, _ = h.Store.db.Exec(`UPDATE collectors SET last_seen=? WHERE id=?`, now.Unix(), id)
}

// PublicEvents returns only observations safe to share. Character exports,
// inventory, ledgers, lists and research settings never enter this feed.
func (s *Store) PublicEvents(after int64) ([]json.RawMessage, int64, error) {
	rows, err := s.db.Query(`SELECT rowid,body FROM events WHERE rowid>? ORDER BY rowid LIMIT 128`, after)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	events := []json.RawMessage{}
	next, size := after, 0
	for rows.Next() {
		var seq int64
		var raw []byte
		if err = rows.Scan(&seq, &raw); err != nil {
			return nil, after, err
		}
		var e Event
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, after, err
		}
		if publicEvent(e) {
			e.Character = nil
			raw = encoded(e)
			if size+len(raw) > 6<<20 {
				break
			}
			events = append(events, json.RawMessage(raw))
			size += len(raw)
		}
		next = seq
	}
	return events, next, rows.Err()
}
