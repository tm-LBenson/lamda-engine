package spellbook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Only public spell IDs leave the machine. Rendering never waits on HTTP or disk.
// Assets are normalized to small PNGs and retained separately from shared rules.
type IconCache struct {
	mu      sync.Mutex
	dir     string
	entries map[int]*iconEntry
	workers chan struct{}
	client  *http.Client
}
type iconEntry struct {
	path  string
	done  chan struct{}
	retry time.Time
}

func NewIconCache(dir string) *IconCache {
	return &IconCache{dir: dir, entries: map[int]*iconEntry{}, workers: make(chan struct{}, 4), client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *IconCache) request(id int) *iconEntry {
	if c == nil || id < 1 || id >= 10000000 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[id]; e != nil {
		if e.retry.IsZero() || time.Now().Before(e.retry) {
			return e
		}
		delete(c.entries, id)
	}
	select {
	case c.workers <- struct{}{}:
	default:
		return nil
	}
	// Discard only completed metadata; downloaded files remain reusable.
	if len(c.entries) >= 1024 {
		for k, e := range c.entries {
			select {
			case <-e.done:
				delete(c.entries, k)
			default:
			}
			if len(c.entries) < 768 {
				break
			}
		}
	}
	e := &iconEntry{done: make(chan struct{})}
	c.entries[id] = e
	go func() {
		path, err := c.fetch(id)
		c.mu.Lock()
		e.path = path
		if err != nil {
			e.retry = time.Now().Add(time.Minute)
		}
		close(e.done)
		c.mu.Unlock()
		<-c.workers
	}()
	return e
}
func (c *IconCache) Request(id int) string {
	e := c.request(id)
	if e == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return e.path
}
func (c *IconCache) Get(ctx context.Context, id int) string {
	e := c.request(id)
	if e == nil {
		return ""
	}
	select {
	case <-e.done:
	case <-ctx.Done():
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return e.path
}
func (c *IconCache) read(url string, limit int64) ([]byte, error) {
	r, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("Icon HTTP %d", r.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, fmt.Errorf("Icon response invalid")
	}
	return b, nil
}

var iconName = regexp.MustCompile(`^[a-zA-Z0-9_]{1,120}$`)

func (c *IconCache) fetch(id int) (string, error) {
	path := filepath.Join(c.dir, strconv.Itoa(id)+".png")
	if f, err := os.Open(path); err == nil {
		info, e := f.Stat()
		cfg, format, decode := image.DecodeConfig(io.LimitReader(f, 256*1024))
		f.Close()
		if e == nil && info.Mode().IsRegular() && info.Size() <= 256*1024 && decode == nil && format == "png" && cfg.Width >= 8 && cfg.Height >= 8 && cfg.Width <= 256 && cfg.Height <= 256 {
			return path, nil
		}
	}
	b, err := c.read("https://nether.wowhead.com/tooltip/spell/"+strconv.Itoa(id), 128*1024)
	if err != nil {
		return "", err
	}
	var meta struct {
		Icon string `json:"icon"`
	}
	if json.Unmarshal(b, &meta) != nil || !iconName.MatchString(meta.Icon) {
		return "", fmt.Errorf("Spell icon unavailable")
	}
	b, err = c.read("https://wow.zamimg.com/images/wow/icons/large/"+meta.Icon+".jpg", 256*1024)
	if err != nil {
		return "", err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Width < 8 || cfg.Height < 8 || cfg.Width > 256 || cfg.Height > 256 {
		return "", fmt.Errorf("Invalid icon dimensions")
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(c.dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(c.dir, ".icon-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	err = png.Encode(f, im)
	ce := f.Close()
	if err != nil {
		return "", err
	}
	if ce != nil {
		return "", ce
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}
