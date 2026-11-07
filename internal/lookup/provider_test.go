package lookup

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveRaiderIOSmoke(t *testing.T) {
	if os.Getenv("LAMDA_LOOKUP_LIVE_TEST") != "1" {
		t.Skip("explicit live API smoke test only")
	}
	c := Character{"Player-1-SMOKE", "Archiowl", "Draenor", "eu"}
	p, err := NewRaiderIO().Fetch(context.Background(), c)
	if err != nil || p.SpecID <= 0 || len(p.Talents) == 0 {
		t.Fatal("public talent lookup", err)
	}
	t.Logf("Public API: spec=%d selected nodes=%d source date known=%t", p.SpecID, len(p.Talents), p.SourceUpdatedAt != nil)
}

var alice = Character{"Player-1-ALICE", "Alice", "Area 52", "us"}

const profileJSON = `{"name":"Alice","realm":"Area 52","region":"us","last_crawled_at":"2026-08-01T00:00:00Z","talentLoadout":{"loadout_spec_id":1468,"loadout_text":"sample","loadout":[{"node":{"id":10,"entries":[{"id":20,"maxRanks":1,"spell":{"id":30}},{"id":21,"maxRanks":2,"spell":{"id":31}}]},"entryIndex":1,"rank":2}]}}`

func TestPublicProfileKeepsSelectedChoiceAndSourceAge(t *testing.T) {
	now := time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)
	p, err := ParseProfile([]byte(profileJSON), alice, now)
	if err != nil || len(p.Talents) != 1 || p.Talents[0] != (Talent{10, 21, 31, 2}) || p.SourceUpdatedAt == nil || now.Sub(*p.SourceUpdatedAt) < 30*24*time.Hour || p.FetchedAt != now {
		t.Fatal(p, err)
	}
	newer, _ := ParseProfile([]byte(profileJSON), alice, now.Add(time.Hour))
	if p.Revision != newer.Revision {
		t.Fatal("fetch time changed build revision")
	}
	missing := strings.Replace(profileJSON, `"last_crawled_at":"2026-08-01T00:00:00Z",`, "", 1)
	p, err = ParseProfile([]byte(missing), alice, now)
	if err != nil || p.SourceUpdatedAt != nil || len(p.Missing) != 1 {
		t.Fatal("missing date treated as current", p, err)
	}
}
func TestUnavailablePublicFieldsAreNotSuccessfulBuilds(t *testing.T) {
	for _, body := range []string{
		`{"name":"Alice","realm":"Area 52","region":"us"}`,
		strings.Replace(profileJSON, `"entryIndex":1`, `"entryIndex":2`, 1),
		strings.Replace(profileJSON, `"entryIndex":1,`, "", 1),
		strings.Replace(profileJSON, `"rank":2`, `"rank":9`, 1),
		strings.Replace(profileJSON, `"Alice"`, `"Different"`, 1),
		strings.Replace(profileJSON, `"us"`, `"eu"`, 1),
		strings.Repeat(" ", (1<<20)+1),
	} {
		if p, err := ParseProfile([]byte(body), alice, time.Now()); err == nil {
			t.Fatal("invalid response accepted", p)
		}
	}
}
func TestHTTPProfileRequestAndErrors(t *testing.T) {
	for _, status := range []int{200, 404, 403, 429, 503, 302} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("fields") != "talents" || r.URL.Query().Get("realm") != "Area 52" || r.URL.Query().Get("region") != "us" || r.Header.Get("Authorization") != "" || r.Header.Get("User-Agent") == "" {
					t.Error("unexpected request", r.URL)
				}
				w.Header().Set("Retry-After", "20")
				w.WriteHeader(status)
				fmt.Fprint(w, profileJSON)
			}))
			defer server.Close()
			provider := NewRaiderIO()
			provider.Endpoint = server.URL
			p, err := provider.Fetch(context.Background(), alice)
			if status == 200 {
				if err != nil || p.SpecID != 1468 {
					t.Fatal(p, err)
				}
				return
			}
			var failure *Failure
			if !errors.As(err, &failure) {
				t.Fatal("missing structured error", err)
			}
			if (status == 429 || status == 503) && failure.RetryAfter != 20*time.Second {
				t.Fatal(failure)
			}
		})
	}
}
func TestHTTPTimeoutDoesNotHangEngine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	provider := NewRaiderIO()
	provider.Endpoint = server.URL
	provider.Client.Timeout = 20 * time.Millisecond
	_, err := provider.Fetch(context.Background(), alice)
	if err == nil {
		t.Fatal("timeout accepted")
	}
}
