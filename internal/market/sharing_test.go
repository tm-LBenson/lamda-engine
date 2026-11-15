package market

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCollectorSignatureAndPrivateDataGuards(t *testing.T) {
	s := fixture(t)
	profiles, err := OpenProfiles(t.TempDir(), "forever")
	if err != nil {
		t.Fatal(err)
	}
	defer profiles.Close()
	access, signer := accessFixture(t)
	h, err := NewSharedServer(s, profiles, access, "market.example.com", "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := collectorID(pub)
	register := func(user string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://market.example.com/api/market/collectors", bytes.NewReader(encoded(Collector{Name: "Home PC", PublicKey: base64.RawURLEncoding.EncodeToString(pub), Enabled: true})))
		r.Header.Set("Origin", "https://market.example.com")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Lamda-Sim", "1")
		r.Header.Set("Cf-Access-Jwt-Assertion", accessToken(t, signer, claimsFor(user)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := register("reader"); w.Code != 403 {
		t.Fatal("reader enrolled publisher", w.Code)
	}
	if w := register("owner"); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	request := func(e Event, nonce string, mutate func(*http.Request)) *httptest.ResponseRecorder {
		body := encoded(map[string]any{"events": []Event{e}})
		at := strconv.FormatInt(time.Now().Unix(), 10)
		r := httptest.NewRequest("POST", "https://market.example.com/api/collector/import", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Lamda-Collector", id)
		r.Header.Set("X-Lamda-Time", at)
		r.Header.Set("X-Lamda-Nonce", nonce)
		r.Header.Set("X-Lamda-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, signedMessage("market.example.com", at, nonce, body))))
		if mutate != nil {
			mutate(r)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	nonce := func() string { v := make([]byte, 16); rand.Read(v); return hex.EncodeToString(v) }
	e := scan("signed", time.Now().Unix(), []Lot{{Item: "ore", Price: 80, Quantity: 1, Count: 10}})
	n := nonce()
	if w := request(e, n, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(e, n, nil); w.Code != 409 {
		t.Fatal("request replay accepted", w.Code)
	}
	if w := request(e, nonce(), nil); w.Code != 200 {
		t.Fatal("idempotent event retry failed", w.Code, w.Body)
	}
	for _, mutate := range []func(*http.Request){func(r *http.Request) { r.Header.Set("X-Lamda-Time", "1") }, func(r *http.Request) { r.Header.Set("X-Lamda-Signature", "fake") }, func(r *http.Request) { r.Header.Set("X-Lamda-Nonce", strings.Repeat("f", 32)) }, func(r *http.Request) { r.Body = http.NoBody }} {
		if w := request(e, nonce(), mutate); w.Code != 401 {
			t.Fatal("bad signature accepted", w.Code)
		}
	}
	ce := event("private", "character")
	ce.Character = &Character{ID: "secret", Name: "Private", Inventory: map[string]int64{}, Known: []string{}, Professions: []Profession{}}
	if w := request(ce, nonce(), nil); w.Code != 400 {
		t.Fatal("character shared", w.Code)
	}
	e.ID = "mixed"
	e.Character = ce.Character
	if w := request(e, nonce(), nil); w.Code != 400 {
		t.Fatal("mixed private data shared", w.Code)
	}
	_, _ = s.db.Exec(`UPDATE collectors SET enabled=0 WHERE id=?`, id)
	e.Character = nil
	if w := request(e, nonce(), nil); w.Code != 401 {
		t.Fatal("revoked collector accepted", w.Code)
	}
	var chars []Character
	if err = s.documents(fixtureMarket.Key(), "character", &chars); err != nil || len(chars) != 0 {
		t.Fatal("private character stored in shared feed", chars, err)
	}
}

type redirectTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	u := *r.URL
	u.Scheme = t.target.Scheme
	u.Host = t.target.Host
	clone.URL = &u
	clone.Host = r.URL.Host
	return t.base.RoundTrip(clone)
}
func TestPublisherEndToEndCursorPrivacyAndRetry(t *testing.T) {
	local := fixture(t)
	shared := fixture(t)
	profiles, err := OpenProfiles(t.TempDir(), "forever")
	if err != nil {
		t.Fatal(err)
	}
	defer profiles.Close()
	h, err := NewSharedServer(shared, profiles, nil, "market.example.com", "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(h)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = local.StartPublisher(ctx); err != nil {
		t.Fatal(err)
	}
	p := local.publisher
	p.cancel()
	<-p.done
	target, _ := url.Parse(server.URL)
	client := server.Client()
	client.Transport = redirectTransport{base: client.Transport, target: target}
	p.client = client
	if err = p.Configure("https://market.example.com", true); err != nil {
		t.Fatal(err)
	}
	pub := p.key.Public().(ed25519.PublicKey)
	if _, err = shared.db.Exec(`INSERT INTO collectors(id,name,public_key,enabled) VALUES(?,?,?,1)`, collectorID(pub), "Home", base64.RawURLEncoding.EncodeToString(pub)); err != nil {
		t.Fatal(err)
	}
	ce := event("character", "character")
	ce.Character = &Character{ID: "secret", Name: "Private", Inventory: map[string]int64{"ore": 20}, Known: []string{}, Professions: []Profession{}}
	ingest(t, local, ce)
	e := scan("observation", fixtureNow.Unix(), []Lot{{Item: "ore", Price: 75, Quantity: 1, Count: 20}})
	e.Character = ce.Character
	ingest(t, local, e)
	if _, err = p.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := p.Status()
	if status.Pending != 0 || status.Cursor != 2 || status.LastSync == 0 {
		t.Fatal(status)
	}
	data, _, err := shared.PublicEvents(0)
	if err != nil || len(data) != 1 {
		t.Fatal(len(data), err)
	}
	var got Event
	json.Unmarshal(data[0], &got)
	if got.Character != nil || strings.Contains(string(data[0]), "Private") {
		t.Fatal("private information sent")
	}
	cards, err := shared.Cards(fixtureMarket.Key(), 0)
	if err != nil || len(cards) != 1 || cards[0].Price != 75 {
		t.Fatal(cards, err)
	}
	_, _ = shared.db.Exec(`UPDATE collectors SET enabled=0`)
	e.ID = "second"
	e.Scan = "second"
	ingest(t, local, e)
	if _, err = p.syncOnce(context.Background()); err == nil {
		t.Fatal("revocation ignored")
	}
	if p.Status().Cursor != 2 || p.Status().Pending != 1 {
		t.Fatal("failed upload skipped observations")
	}
	_, _ = shared.db.Exec(`UPDATE collectors SET enabled=1`)
	if _, err = p.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.Status().Pending != 0 {
		t.Fatal("retry did not drain")
	}
	for _, raw := range []string{"http://market.example.com", "https://127.0.0.1", "https://user:pass@market.example.com", "https://market.example.com/other", "https://market.example.com?x=y"} {
		if p.Configure(raw, true) == nil {
			t.Fatal("invalid destination accepted", raw)
		}
	}
}
