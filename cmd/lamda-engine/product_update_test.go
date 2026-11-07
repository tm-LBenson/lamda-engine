package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProductUpdateUsesOnePublicReleaseRequest(t *testing.T) {
	previous := productReleaseVersion
	productReleaseVersion = "1.3.0"
	defer func() { productReleaseVersion = previous }()
	old := http.DefaultClient
	defer func() { http.DefaultClient = old }()
	for _, tc := range []struct{ body, want string }{
		{`{"tag_name":"v1.3.0"}`, ""},
		{`{"tag_name":"v1.3.1"}`, "v1.3.1"},
		{`{"tag_name":"v9.0.0-retail.1"}`, ""},
		{`{"tag_name":"v1.4.0","prerelease":true}`, ""},
	} {
		calls := 0
		http.DefaultClient = &http.Client{Transport: releaseTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Host != "api.github.com" || r.URL.Path != "/repos/tm-LBenson/lamda-engine/releases/latest" {
				t.Fatal(r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{}}, nil
		})}
		if got := checkUpdate(context.Background()); got != tc.want || calls != 1 {
			t.Fatal(got, tc.want, calls)
		}
	}
}
