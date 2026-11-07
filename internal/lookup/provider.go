// Package lookup collects provisional public builds outside the game process.
package lookup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Character struct {
	GUID   string `json:"guid"`
	Name   string `json:"name"`
	Realm  string `json:"realm"`
	Region string `json:"region"`
}

func (c Character) Valid() bool {
	if !strings.HasPrefix(c.GUID, "Player-") || len(c.GUID) > 128 || !slices.Contains([]string{"us", "eu", "kr", "tw"}, c.Region) {
		return false
	}
	for _, value := range []string{c.Name, c.Realm} {
		if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
			return false
		}
		for _, r := range value {
			if !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsDigit(r) && r != '-' && r != '\'' && r != ' ' {
				return false
			}
		}
	}
	return true
}
func canonical(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '\'' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
func (c Character) Key() string {
	return c.Region + "/" + canonical(c.Realm) + "/" + strings.ToLower(c.Name)
}

type Talent struct {
	NodeID  int `json:"nodeId"`
	EntryID int `json:"entryId"`
	SpellID int `json:"spellId"`
	Rank    int `json:"rank"`
}
type Profile struct {
	Character       Character  `json:"character"`
	Source          string     `json:"source"`
	URL             string     `json:"url"`
	FetchedAt       time.Time  `json:"fetchedAt"`
	SourceUpdatedAt *time.Time `json:"sourceUpdatedAt"`
	SpecID          int        `json:"specId"`
	Talents         []Talent   `json:"talents"`
	Loadout         string     `json:"loadout"`
	Revision        string     `json:"revision"`
	Missing         []string   `json:"missing"`
}
type Failure struct {
	Code       string
	RetryAfter time.Duration
}

func (e *Failure) Error() string { return e.Code }

type Fetcher interface {
	Fetch(context.Context, Character) (Profile, error)
}

type RaiderIO struct {
	Client   *http.Client
	Endpoint string
}

func NewRaiderIO() *RaiderIO {
	return &RaiderIO{Client: &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Endpoint: "https://raider.io/api/v1/characters/profile"}
}
func (r *RaiderIO) Fetch(ctx context.Context, c Character) (Profile, error) {
	if !c.Valid() {
		return Profile{}, &Failure{Code: "invalid_identity"}
	}
	u, err := url.Parse(r.Endpoint)
	if err != nil {
		return Profile{}, &Failure{Code: "invalid_provider"}
	}
	q := u.Query()
	q.Set("region", c.Region)
	q.Set("realm", c.Realm)
	q.Set("name", c.Name)
	q.Set("fields", "talents")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Profile{}, &Failure{Code: "invalid_request"}
	}
	req.Header.Set("User-Agent", "LamdaEngine/0.7.3 (public talent lookup)")
	req.Header.Set("Accept", "application/json")
	response, err := r.Client.Do(req)
	if err != nil {
		return Profile{}, &Failure{Code: "network_error"}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		code := "http_error"
		switch response.StatusCode {
		case 400:
			code = "profile_unavailable"
		case 404:
			code = "profile_not_found"
		case 403:
			code = "profile_unavailable"
		case 429:
			code = "rate_limited"
		case 503:
			code = "provider_unavailable"
		}
		delay := time.Duration(0)
		if response.StatusCode == 429 || response.StatusCode == 503 {
			delay = 30 * time.Second
			if seconds, e := strconv.Atoi(response.Header.Get("Retry-After")); e == nil && seconds >= 0 {
				if int64(seconds) > (1<<63-1)/int64(time.Second) {
					delay = time.Duration(1<<63 - 1)
				} else {
					delay = time.Duration(seconds) * time.Second
				}
			} else if until, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil {
				delay = max(time.Second, time.Until(until))
			}
		}
		return Profile{}, &Failure{Code: code, RetryAfter: delay}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return Profile{}, &Failure{Code: "invalid_response"}
	}
	return ParseProfile(body, c, time.Now())
}

// The selected entryIndex is zero-based. Do not collect every choice in a node.
func ParseProfile(body []byte, c Character, now time.Time) (Profile, error) {
	var raw struct {
		Name, Realm, Region string
		LastCrawled         string `json:"last_crawled_at"`
		TalentLoadout       *struct {
			Spec    int    `json:"loadout_spec_id"`
			Text    string `json:"loadout_text"`
			Loadout []struct {
				Node struct {
					ID      int
					Entries []struct {
						ID, MaxRanks int
						Spell        struct{ ID int }
					}
				}
				EntryIndex *int
				Rank       int
			}
		}
	}
	if len(body) > 1<<20 || json.Unmarshal(body, &raw) != nil {
		return Profile{}, &Failure{Code: "invalid_response"}
	}
	if !strings.EqualFold(raw.Name, c.Name) || canonical(raw.Realm) != canonical(c.Realm) || !strings.EqualFold(raw.Region, c.Region) {
		return Profile{}, &Failure{Code: "identity_mismatch"}
	}
	p := Profile{Character: c, Source: "raider_io", URL: "https://raider.io/characters/" + url.PathEscape(c.Region) + "/" + url.PathEscape(strings.ToLower(strings.ReplaceAll(c.Realm, " ", "-"))) + "/" + url.PathEscape(c.Name), FetchedAt: now, Missing: []string{}, Talents: []Talent{}}
	if at, err := time.Parse(time.RFC3339Nano, raw.LastCrawled); err == nil && !at.After(now.Add(2*time.Minute)) {
		p.SourceUpdatedAt = &at
	} else {
		p.Missing = append(p.Missing, "source_time")
	}
	l := raw.TalentLoadout
	if l == nil || l.Spec <= 0 || l.Spec > 100000 || len(l.Loadout) > 256 || len(l.Text) > 8192 {
		return Profile{}, &Failure{Code: "missing_talents"}
	}
	p.SpecID, p.Loadout = l.Spec, l.Text
	seen := map[int]bool{}
	for _, node := range l.Loadout {
		if node.Rank == 0 {
			continue
		}
		if node.Node.ID <= 0 || seen[node.Node.ID] || node.Rank < 0 || node.Rank > 100 || node.EntryIndex == nil || *node.EntryIndex < 0 || *node.EntryIndex >= len(node.Node.Entries) {
			p.Missing = append(p.Missing, "talent_node")
			continue
		}
		entry := node.Node.Entries[*node.EntryIndex]
		if entry.ID <= 0 || entry.Spell.ID <= 0 || entry.MaxRanks < node.Rank {
			p.Missing = append(p.Missing, "talent_node")
			continue
		}
		seen[node.Node.ID] = true
		p.Talents = append(p.Talents, Talent{node.Node.ID, entry.ID, entry.Spell.ID, node.Rank})
	}
	if len(p.Talents) == 0 {
		return Profile{}, &Failure{Code: "missing_talents"}
	}
	sort.Slice(p.Talents, func(i, j int) bool { return p.Talents[i].NodeID < p.Talents[j].NodeID })
	sort.Strings(p.Missing)
	p.Missing = slices.Compact(p.Missing)
	encoded, _ := json.Marshal(struct {
		Spec    int
		Talents []Talent
	}{p.SpecID, p.Talents})
	p.Revision = fmt.Sprintf("%x", sha256.Sum256(encoded))
	return p, nil
}
