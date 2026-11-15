package market

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HostedHandler exposes only the market workspace. It requires verified Access
// identity and keeps personal writes in separate profile databases.
func (s *Store) HostedHandler(host string, access *Access, profiles *Profiles) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.Host != host {
			problem(w, 403, fmt.Errorf("invalid host"))
			return
		}
		if r.Method != "GET" && r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if r.Method == "POST" && (r.Header.Get("Origin") != "https://"+host || r.Header.Get("Sec-Fetch-Site") == "cross-site") {
			problem(w, 403, fmt.Errorf("use this site's controls"))
			return
		}
		identity, err := access.Verify(r.Context(), r.Header.Get("Cf-Access-Jwt-Assertion"), time.Now())
		if err != nil {
			problem(w, 401, err)
			return
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/market/", http.StatusTemporaryRedirect)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/market/") && !strings.HasPrefix(r.URL.Path, "/api/market/") {
			http.NotFound(w, r)
			return
		}
		profile, err := profiles.Get(identity.Subject)
		if err != nil {
			problem(w, 503, fmt.Errorf("profile unavailable"))
			return
		}
		r = withProfile(WithUser(r, identity.Subject), profile)
		if r.URL.Path == "/api/market/import" {
			problem(w, 403, fmt.Errorf("use character import or an authorized collector"))
			return
		}
		s.ServeHTTP(w, r)
	})
}
