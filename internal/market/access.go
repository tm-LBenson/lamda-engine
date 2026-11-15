package market

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Access verifies Cloudflare Access application JWTs against the configured
// issuer's public keys. Identity headers and unsigned tokens are never trusted.
type Access struct {
	issuer, audience string
	client           *http.Client
	mu               sync.Mutex
	keys             map[string]*rsa.PublicKey
	refreshed        time.Time
}
type AccessIdentity struct{ Subject, Email string }

func NewAccess(issuer, audience string) (*Access, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".cloudflareaccess.com") || u.Host != u.Hostname() || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || audience == "" {
		return nil, fmt.Errorf("configure the Cloudflare Access team URL and application audience")
	}
	return &Access{issuer: issuer, audience: audience, client: &http.Client{Timeout: 8 * time.Second}, keys: map[string]*rsa.PublicKey{}}, nil
}
func (a *Access) key(ctx context.Context, id string) (*rsa.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if k := a.keys[id]; k != nil && time.Since(a.refreshed) < time.Hour {
		return k, nil
	}
	// Unknown key IDs cannot force unbounded public-key requests.
	if time.Since(a.refreshed) < 30*time.Second {
		return nil, fmt.Errorf("unknown signing key")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", a.issuer+"/cdn-cgi/access/certs", nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("signing keys unavailable")
	}
	var doc struct {
		Keys []struct{ Kty, Use, Kid, Alg, N, E string }
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" || (k.Alg != "" && k.Alg != "RS256") || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, e1 := base64.RawURLEncoding.DecodeString(k.N)
		e, e2 := base64.RawURLEncoding.DecodeString(k.E)
		if e1 != nil || e2 != nil || len(n) < 256 || len(n) > 1024 || len(e) > 4 || len(e) == 0 {
			continue
		}
		exponent := 0
		for _, v := range e {
			exponent = exponent*256 + int(v)
		}
		if exponent < 3 || exponent%2 == 0 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no usable signing keys")
	}
	a.keys = keys
	a.refreshed = time.Now()
	if k := keys[id]; k != nil {
		return k, nil
	}
	return nil, fmt.Errorf("unknown signing key")
}
func (a *Access) Verify(ctx context.Context, token string, now time.Time) (AccessIdentity, error) {
	fail := fmt.Errorf("sign in through Cloudflare Access")
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 16384 {
		return AccessIdentity{}, fail
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AccessIdentity{}, fail
	}
	var h struct {
		Alg, Kid string
		Crit     []string
	}
	if json.Unmarshal(header, &h) != nil || h.Alg != "RS256" || h.Kid == "" || len(h.Crit) != 0 {
		return AccessIdentity{}, fail
	}
	key, err := a.key(ctx, h.Kid)
	if err != nil {
		return AccessIdentity{}, fail
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err != nil || rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig) != nil {
		return AccessIdentity{}, fail
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessIdentity{}, fail
	}
	var claims struct {
		Iss, Sub, Email, Type string
		Aud                   []string
		Exp, Nbf, Iat         int64
	}
	if json.Unmarshal(data, &claims) != nil {
		return AccessIdentity{}, fail
	}
	validAudience := false
	for _, aud := range claims.Aud {
		if aud == a.audience {
			validAudience = true
		}
	}
	if claims.Iss != a.issuer || !validAudience || claims.Exp <= now.Unix() || claims.Nbf > now.Unix()+30 || claims.Iat > now.Unix()+30 || claims.Sub == "" || len(claims.Sub) > 512 || !strings.Contains(claims.Email, "@") || len(claims.Email) > 320 || claims.Type != "app" {
		return AccessIdentity{}, fail
	}
	return AccessIdentity{Subject: a.issuer + "/" + claims.Sub, Email: strings.ToLower(claims.Email)}, nil
}
