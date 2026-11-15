package spellbook

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

type iconTransport func(*http.Request) (*http.Response, error)

func (f iconTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestIconCacheMetadataBoundsAndOfflineReuse(t *testing.T) {
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 64, 64)))
	cache := NewIconCache(t.TempDir())
	var calls atomic.Int32
	cache.client.Transport = iconTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		body := pngData.String()
		if r.URL.Host == "nether.wowhead.com" {
			body = `{"icon":"ability_test"}`
		} else if r.URL.Host != "wow.zamimg.com" || r.URL.Path != "/images/wow/icons/large/ability_test.jpg" {
			t.Errorf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	path := cache.Get(context.Background(), 123)
	if path == "" {
		t.Fatal("download failed")
	}
	for range 10 {
		if cache.Request(123) != path {
			t.Fatal("cached icon not immediately available")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("duplicate download", calls.Load())
	}
	offline := NewIconCache(cache.dir)
	offline.client.Transport = iconTransport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("offline") })
	if offline.Get(context.Background(), 123) != path {
		t.Fatal("disk cache failed offline")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	bad := NewIconCache(t.TempDir())
	bad.client.Transport = iconTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"icon":"../../evil"}`))}, nil
	})
	if bad.Get(context.Background(), 456) != "" || bad.Get(context.Background(), -1) != "" {
		t.Fatal("invalid asset accepted")
	}
}

func TestItemIconsUseItemMetadataAndSeparateCache(t *testing.T) {
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 64, 64)))
	cache := NewItemIconCache(t.TempDir())
	cache.client.Transport = iconTransport(func(r *http.Request) (*http.Response, error) {
		body := pngData.String()
		if r.URL.Host == "nether.wowhead.com" {
			if r.URL.Path != "/tooltip/item/2589" {
				t.Errorf("wrong item metadata endpoint %s", r.URL)
			}
			body = `{"icon":"inv_fabric_linen_01"}`
		} else if r.URL.Host != "wow.zamimg.com" || r.URL.Path != "/images/wow/icons/large/inv_fabric_linen_01.jpg" {
			t.Errorf("unexpected asset URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	if path := cache.Get(context.Background(), 2589); path == "" {
		t.Fatal("item icon not cached")
	}
}
