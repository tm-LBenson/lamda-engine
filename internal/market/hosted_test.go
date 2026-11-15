package market

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"

	"strings"
	"testing"
	"time"
)

func accessFixture(t *testing.T) (*Access, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAccess("https://test.cloudflareaccess.com", "market-audience")
	if err != nil {
		t.Fatal(err)
	}
	a.keys["test"] = &key.PublicKey
	a.refreshed = time.Now()
	return a, key
}
func accessToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test"}`))
	data := base64.RawURLEncoding.EncodeToString(encoded(claims))
	unsigned := head + "." + data
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func claimsFor(subject string) map[string]any {
	return map[string]any{"iss": "https://test.cloudflareaccess.com", "aud": []string{"market-audience"}, "sub": subject, "email": subject + "@example.com", "type": "app", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix()}
}
func TestAccessRejectsSpoofedExpiredAndWrongApplication(t *testing.T) {
	a, key := accessFixture(t)
	token := accessToken(t, key, claimsFor("alice"))
	if who, err := a.Verify(context.Background(), token, time.Now()); err != nil || who.Email != "alice@example.com" {
		t.Fatal(who, err)
	}
	for _, change := range []map[string]any{{"aud": []string{"other-app"}}, {"iss": "https://attacker.example.com"}, {"exp": time.Now().Unix() - 1}, {"nbf": time.Now().Add(time.Hour).Unix()}, {"type": "service"}, {"sub": ""}, {"email": ""}} {
		claims := claimsFor("alice")
		for k, v := range change {
			claims[k] = v
		}
		if _, err := a.Verify(context.Background(), accessToken(t, key, claims), time.Now()); err == nil {
			t.Fatal("invalid claims accepted", change)
		}
	}
	parts := strings.Split(token, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString(encoded(claimsFor("bob")))
	if _, err := a.Verify(context.Background(), strings.Join(parts, "."), time.Now()); err == nil {
		t.Fatal("tampered token accepted")
	}
	for _, token := range []string{"", "alice@example.com", base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + ".e30."} {
		if _, err := a.Verify(context.Background(), token, time.Now()); err == nil {
			t.Fatal("unsigned identity accepted")
		}
	}
}
func TestHostedPrivateProfilesSharePricesOnly(t *testing.T) {
	shared := fixture(t)
	key := fixtureMarket.Key()
	ingest(t, shared, scan("shared", fixtureNow.Unix(), []Lot{{Item: "ore", Price: 50, Quantity: 1, Count: 10}}))
	// Even an accidentally populated shared database cannot reveal private exports.
	ce := event("owner-character", "character")
	ce.Character = &Character{ID: "secret", Name: "Owner", Professions: []Profession{}, Known: []string{}, Inventory: map[string]int64{}}
	ingest(t, shared, ce)
	if err := shared.Trade(key, Trade{ID: "secret-buy", Item: "ore", Side: "buy", Quantity: 2, Total: 100, At: fixtureNow.Unix()}); err != nil {
		t.Fatal(err)
	}
	profiles, err := OpenProfiles(t.TempDir(), "forever")
	if err != nil {
		t.Fatal(err)
	}
	defer profiles.Close()
	access, signer := accessFixture(t)
	handler := shared.HostedHandler("market.example.com", access, profiles)
	request := func(user, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		method := "GET"
		var data []byte
		if body != nil {
			method = "POST"
			data = encoded(body)
		}
		r := httptest.NewRequest(method, "https://market.example.com"+strings.ReplaceAll(path, " ", "%20"), bytes.NewReader(data))
		if user != "" {
			r.Header.Set("Cf-Access-Jwt-Assertion", accessToken(t, signer, claimsFor(user)))
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Lamda-Sim", "1")
		r.Header.Set("Origin", "https://market.example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/api/market/state?market=" + key
	if w := request("", path, nil); w.Code != 401 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request("alice", "/api/market/lists/create?market="+key, map[string]string{"id": "metals", "name": "Alice's metals"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request("alice", "/api/market/trade?market="+key, Trade{ID: "buy", Item: "ore", Side: "buy", Quantity: 3, Total: 150, At: fixtureNow.Unix()}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	for _, user := range []string{"alice", "bob"} {
		w := request(user, path, nil)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var data struct {
			Cards      []Card
			Characters []Character
			Lists      []FavoriteList
			Trades     []Trade
		}
		if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Characters) != 0 || len(data.Cards) != 1 || data.Cards[0].Price != 50 {
			t.Fatal("shared data/privacy mismatch", user, data)
		}
		if user == "bob" && (len(data.Lists) != 1 || len(data.Trades) != 0 || data.Cards[0].Holding != 0) {
			t.Fatal("private data leaked to bob", data)
		}
		if user == "alice" && (len(data.Lists) != 2 || len(data.Trades) != 1 || data.Cards[0].Holding != 3) {
			t.Fatal("alice lost private state", data)
		}
	}
	if w := request("bob", "/api/market/import", map[string]any{"events": []Event{scan("poison", time.Now().Unix(), nil)}}); w.Code != 403 {
		t.Fatal("reader changed shared feed", w.Code)
	}
	if w := request("bob", "/api/market/lists/delete?market="+key, map[string]string{"id": "metals"}); w.Code != 400 {
		t.Fatal("bob deleted alice's list", w.Code)
	}
	r := httptest.NewRequest("POST", "https://market.example.com/api/market/settings?market="+strings.ReplaceAll(key, " ", "%20"), strings.NewReader("{}"))
	r.Header.Set("Cf-Access-Jwt-Assertion", accessToken(t, signer, claimsFor("alice")))
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
}
