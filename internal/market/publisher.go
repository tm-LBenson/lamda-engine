package market

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PublisherStatus struct {
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"publicKey"`
	LastSync  int64  `json:"lastSync"`
	Error     string `json:"error"`
	Cursor    int64  `json:"cursor"`
	Pending   int64  `json:"pending"`
}
type Publisher struct {
	store  *Store
	mu     sync.Mutex
	key    ed25519.PrivateKey
	status PublisherStatus
	client *http.Client
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}
}

func validPublisherURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.Host == u.Hostname() && net.ParseIP(u.Hostname()) == nil && strings.Contains(u.Hostname(), ".") && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Path == "" || u.Path == "/")
}

// StartPublisher is used by the local Engine only; hosted servers never create
// or export signing keys. A key is retained inside the local SQLite database.
func (s *Store) StartPublisher(ctx context.Context) error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS publisher_config(id INTEGER PRIMARY KEY CHECK(id=1),private_key BLOB,url TEXT,enabled INTEGER,cursor INTEGER,last_sync INTEGER);`)
	if err != nil {
		return err
	}
	p := &Publisher{store: s, client: &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, done: make(chan struct{}), wake: make(chan struct{}, 1)}
	var raw []byte
	err = s.db.QueryRow(`SELECT private_key,url,enabled,cursor,last_sync FROM publisher_config WHERE id=1`).Scan(&raw, &p.status.URL, &p.status.Enabled, &p.status.Cursor, &p.status.LastSync)
	if err != nil {
		var count int
		if e := s.db.QueryRow(`SELECT count(*) FROM publisher_config`).Scan(&count); e != nil || count != 0 {
			return fmt.Errorf("publisher configuration unavailable")
		}
		_, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return e
		}
		raw = key
		if _, e = s.db.Exec(`INSERT INTO publisher_config VALUES(1,?,'',0,0,0)`, raw); e != nil {
			return e
		}
	}
	if len(raw) != ed25519.PrivateKeySize {
		return fmt.Errorf("publisher key is invalid")
	}
	p.key = ed25519.PrivateKey(raw)
	p.status.PublicKey = base64.RawURLEncoding.EncodeToString(p.key.Public().(ed25519.PublicKey))
	ctx, p.cancel = context.WithCancel(ctx)
	s.publisher = p
	go p.run(ctx)
	return nil
}
func (p *Publisher) Status() PublisherStatus {
	p.mu.Lock()
	status := p.status
	p.mu.Unlock()
	_ = p.store.db.QueryRow(`SELECT count(*) FROM events WHERE rowid>? AND json_extract(body,'$.kind') IN ('auction','items','vendor','recipes')`, status.Cursor).Scan(&status.Pending)
	return status
}
func (p *Publisher) Configure(raw string, enabled bool) error {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "/")
	if raw != "" && !validPublisherURL(raw) || enabled && raw == "" {
		return fmt.Errorf("enter the HTTPS address of the shared market")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	cursor := p.status.Cursor
	last := p.status.LastSync
	if raw != p.status.URL {
		cursor = 0
		last = 0
	}
	if _, err := p.store.db.Exec(`UPDATE publisher_config SET url=?,enabled=?,cursor=?,last_sync=? WHERE id=1`, raw, enabled, cursor, last); err != nil {
		return err
	}
	p.status.URL = raw
	p.status.Enabled = enabled
	p.status.Cursor = cursor
	p.status.LastSync = last
	p.status.Error = ""
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return nil
}
func (p *Publisher) run(ctx context.Context) {
	defer close(p.done)
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-p.wake:
		}
		for n := 0; n < 16; n++ {
			more, err := p.syncOnce(ctx)
			if err != nil {
				p.mu.Lock()
				p.status.Error = err.Error()
				p.mu.Unlock()
				break
			}
			if !more || ctx.Err() != nil {
				break
			}
		}
	}
}
func (p *Publisher) syncOnce(ctx context.Context) (bool, error) {
	p.mu.Lock()
	cfg := p.status
	p.mu.Unlock()
	if !cfg.Enabled {
		return false, nil
	}
	events, next, err := p.store.PublicEvents(cfg.Cursor)
	if err != nil {
		return false, fmt.Errorf("could not read local observations")
	}
	if next == cfg.Cursor {
		return false, nil
	}
	if len(events) > 0 {
		body := encoded(struct {
			Events []json.RawMessage `json:"events"`
		}{events})
		u, err := url.Parse(cfg.URL)
		if err != nil {
			return false, err
		}
		at := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := make([]byte, 16)
		if _, err = rand.Read(nonce); err != nil {
			return false, err
		}
		nonceText := hex.EncodeToString(nonce)
		req, err := http.NewRequestWithContext(ctx, "POST", cfg.URL+"/api/collector/import", bytes.NewReader(body))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Lamda-Collector", collectorID(p.key.Public().(ed25519.PublicKey)))
		req.Header.Set("X-Lamda-Time", at)
		req.Header.Set("X-Lamda-Nonce", nonceText)
		req.Header.Set("X-Lamda-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(p.key, signedMessage(u.Host, at, nonceText, body))))
		resp, err := p.client.Do(req)
		if err != nil {
			return false, fmt.Errorf("shared market unavailable; retrying")
		}
		defer resp.Body.Close()
		if resp.StatusCode == 401 {
			return false, fmt.Errorf("authorize this collector's public key on the shared market")
		}
		if resp.StatusCode != 200 {
			return false, fmt.Errorf("shared market returned HTTP %d; retrying", resp.StatusCode)
		}
		var result struct {
			Accepted int `json:"accepted"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result) != nil || result.Accepted != len(events) {
			return false, fmt.Errorf("shared market did not confirm this batch")
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status.URL != cfg.URL || !p.status.Enabled {
		return false, nil
	}
	last := time.Now().Unix()
	if _, err = p.store.db.Exec(`UPDATE publisher_config SET cursor=?,last_sync=? WHERE id=1`, next, last); err != nil {
		return false, err
	}
	p.status.Cursor = next
	p.status.LastSync = last
	p.status.Error = ""
	return true, nil
}
func (p *Publisher) Close() { p.cancel(); <-p.done }
func (s *Store) publisherHTTP(w http.ResponseWriter, r *http.Request) {
	if s.publisher == nil {
		problem(w, 503, fmt.Errorf("sharing is available in the Lamda Engine workspace"))
		return
	}
	if r.Method == "GET" {
		response(w, s.publisher.Status())
		return
	}
	if r.Method == "POST" {
		var cfg struct {
			URL     string `json:"url"`
			Enabled bool   `json:"enabled"`
		}
		if err := decode(w, r, &cfg); err != nil {
			problem(w, 400, err)
			return
		}
		if err := s.publisher.Configure(cfg.URL, cfg.Enabled); err != nil {
			problem(w, 400, err)
			return
		}
		response(w, s.publisher.Status())
		return
	}
	w.WriteHeader(405)
}
