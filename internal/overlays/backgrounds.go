package overlays

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxBackgroundBytes = 32 * 1024 * 1024

var ErrBackgroundConflict = errors.New("The screenshot library changed. Try again")

type Background struct {
	ID        string `json:"id"`
	Character string `json:"character"`
	Realm     string `json:"realm"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"format"`
	Image     string `json:"image"`
	UpdatedAt string `json:"updatedAt"`
}

type BackgroundLibrary struct {
	Version  int          `json:"version"`
	Revision int64        `json:"revision"`
	Images   []Background `json:"images"`
}

func backgroundID(character, realm string) (string, error) {
	for _, s := range []string{character, realm} {
		if strings.TrimSpace(s) == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 80 || strings.IndexFunc(s, unicode.IsControl) >= 0 {
			return "", fmt.Errorf("Enter a character name and realm (up to 80 characters each)")
		}
	}
	realmKey := strings.ToLower(strings.ReplaceAll(strings.Join(strings.Fields(realm), ""), "-", ""))
	if realmKey == "" {
		return "", fmt.Errorf("Enter a realm")
	}
	key := strings.ToLower(strings.TrimSpace(character)) + "\x00" + realmKey
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:]), nil
}

func backgroundHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Preview files have their own lock and revision. Image uploads never reset
// active rules, change Load filters, or publish anything to the addon.
func (s *Store) backgroundLibrary() (BackgroundLibrary, error) {
	d := BackgroundLibrary{Version: 1, Images: []Background{}}
	p := filepath.Join(s.dir, "backgrounds.json")
	info, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if !info.Mode().IsRegular() || info.Size() > 128*1024 {
		return d, fmt.Errorf("Screenshot library is invalid")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return d, err
	}
	if json.Unmarshal(b, &d) != nil || d.Version != 1 || d.Revision < 0 || len(d.Images) > 100 {
		return d, fmt.Errorf("Screenshot library is invalid")
	}
	if d.Images == nil {
		d.Images = []Background{}
	}
	seen := map[string]bool{}
	for _, v := range d.Images {
		id, err := backgroundID(v.Character, v.Realm)
		if err != nil || id != v.ID || seen[id] || !backgroundHash(v.Image) || (v.Format != "png" && v.Format != "jpeg") || !backgroundDimensions(v.Width, v.Height) {
			return d, fmt.Errorf("Screenshot library is invalid")
		}
		seen[id] = true
	}
	return d, nil
}

func backgroundDimensions(width, height int) bool {
	return width >= 16 && height >= 16 && width <= 16384 && height <= 16384 && int64(width)*int64(height) <= 64*1024*1024
}

func (s *Store) Backgrounds() (BackgroundLibrary, error) {
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()
	return s.backgroundLibrary()
}

func (s *Store) SaveBackground(revision int64, character, realm string, data []byte) (BackgroundLibrary, error) {
	character, realm = strings.TrimSpace(character), strings.TrimSpace(realm)
	id, err := backgroundID(character, realm)
	if err != nil {
		return BackgroundLibrary{}, err
	}
	if len(data) > MaxBackgroundBytes {
		return BackgroundLibrary{}, fmt.Errorf("Choose a PNG or JPG screenshot smaller than 32 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || !backgroundDimensions(cfg.Width, cfg.Height) {
		return BackgroundLibrary{}, fmt.Errorf("Choose a valid PNG or JPG screenshot (up to 64 megapixels)")
	}
	// Validate the full file before publishing its dimensions or URL.
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return BackgroundLibrary{}, fmt.Errorf("The screenshot could not be read")
	}
	h := sha256.Sum256(data)
	v := Background{ID: id, Character: character, Realm: realm, Width: cfg.Width, Height: cfg.Height, Format: format, Image: hex.EncodeToString(h[:]), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()
	d, err := s.backgroundLibrary()
	if err != nil {
		return d, err
	}
	if d.Revision != revision {
		return d, ErrBackgroundConflict
	}
	old := Background{}
	for i, item := range d.Images {
		if item.ID == id {
			old = item
			d.Images[i] = v
			break
		}
	}
	if old.ID == "" {
		if len(d.Images) >= 100 {
			return d, fmt.Errorf("Remove an unused screenshot before adding another")
		}
		d.Images = append(d.Images, v)
	}
	dir := filepath.Join(s.dir, "backgrounds")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return d, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return d, fmt.Errorf("Screenshot directory unavailable")
	}
	if err = atomic(filepath.Join(dir, v.Image+"."+v.Format), data); err != nil {
		return d, err
	}
	sort.Slice(d.Images, func(i, j int) bool {
		return strings.ToLower(d.Images[i].Character+" "+d.Images[i].Realm) < strings.ToLower(d.Images[j].Character+" "+d.Images[j].Realm)
	})
	if err = s.saveBackgroundLibrary(&d); err != nil {
		return d, err
	}
	s.removeUnusedBackground(old, d)
	return d, nil
}

func (s *Store) saveBackgroundLibrary(d *BackgroundLibrary) error {
	d.Revision++
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return atomic(filepath.Join(s.dir, "backgrounds.json"), b)
}

func (s *Store) removeUnusedBackground(old Background, d BackgroundLibrary) {
	if old.ID == "" {
		return
	}
	for _, v := range d.Images {
		if v.Image == old.Image && v.Format == old.Format {
			return
		}
	}
	_ = os.Remove(filepath.Join(s.dir, "backgrounds", old.Image+"."+old.Format))
}

func (s *Store) RemoveBackground(revision int64, id string) (BackgroundLibrary, error) {
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()
	d, err := s.backgroundLibrary()
	if err != nil {
		return d, err
	}
	if d.Revision != revision {
		return d, ErrBackgroundConflict
	}
	for i, v := range d.Images {
		if v.ID != id {
			continue
		}
		d.Images = append(d.Images[:i], d.Images[i+1:]...)
		if err = s.saveBackgroundLibrary(&d); err != nil {
			return d, err
		}
		s.removeUnusedBackground(v, d)
		return d, nil
	}
	return d, os.ErrNotExist
}

func (s *Store) BackgroundFile(id, version string) (string, string, error) {
	s.backgroundMu.Lock()
	defer s.backgroundMu.Unlock()
	d, err := s.backgroundLibrary()
	if err != nil {
		return "", "", err
	}
	for _, v := range d.Images {
		if v.ID != id || v.Image != version {
			continue
		}
		p := filepath.Join(s.dir, "backgrounds", v.Image+"."+v.Format)
		dir, err := os.Lstat(filepath.Dir(p))
		if err != nil || !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 {
			return "", "", os.ErrNotExist
		}
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBackgroundBytes {
			return "", "", os.ErrNotExist
		}
		return p, "image/" + v.Format, nil
	}
	return "", "", os.ErrNotExist
}
