package overlays

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxMediaBytes = 8 * 1024 * 1024
const MaxMediaBatchBytes = 16 * 1024 * 1024
const MaxMediaLibraryBytes = 64 * 1024 * 1024

var ErrMediaConflict = errors.New("The media library changed. Try again")

type MediaAsset struct {
	Available bool    `json:"available,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Format    string  `json:"format"`
	Bytes     int     `json:"bytes"`
	Width     int     `json:"width,omitempty"`
	Height    int     `json:"height,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
}
type MediaLibrary struct {
	Version  int          `json:"version"`
	Revision int64        `json:"revision"`
	Assets   []MediaAsset `json:"assets"`
}
type MediaUpload struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Data []byte `json:"data"`
}
type MediaStatus struct {
	MediaLibrary
	Error   string   `json:"error,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

func mediaName(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= 80 && strings.IndexFunc(name, unicode.IsControl) < 0
}
func mediaDimensions(w, h int) bool {
	return w >= 1 && h >= 1 && w <= 4096 && h <= 4096 && int64(w)*int64(h) <= 16*1024*1024
}

// SoundPlayer accepts PCM WAV. Reject compressed or ambiguous chunks before
// publishing a file, with a duration bound independent of its byte size.
func waveDuration(data []byte) (float64, error) {
	bad := errors.New("Choose a PCM WAV sound up to 30 seconds (mono/stereo, 8 or 16 bit)")
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return 0, bad
	}
	var rate, align, dataBytes uint32
	fmtSeen, dataSeen := false, false
	for at := 12; at < len(data); {
		if len(data)-at < 8 {
			return 0, bad
		}
		kind := string(data[at : at+4])
		size := uint64(binary.LittleEndian.Uint32(data[at+4 : at+8]))
		at += 8
		if size > uint64(len(data)-at) {
			return 0, bad
		}
		chunk := data[at : at+int(size)]
		switch kind {
		case "fmt ":
			if fmtSeen || len(chunk) < 16 || len(chunk) > 40 || binary.LittleEndian.Uint16(chunk[:2]) != 1 {
				return 0, bad
			}
			fmtSeen = true
			channels := uint32(binary.LittleEndian.Uint16(chunk[2:4]))
			rate = binary.LittleEndian.Uint32(chunk[4:8])
			bits := uint32(binary.LittleEndian.Uint16(chunk[14:16]))
			align = uint32(binary.LittleEndian.Uint16(chunk[12:14]))
			if channels < 1 || channels > 2 || rate < 8000 || rate > 96000 || bits != 8 && bits != 16 || align != channels*bits/8 || binary.LittleEndian.Uint32(chunk[8:12]) != rate*align {
				return 0, bad
			}
		case "data":
			if dataSeen || size == 0 {
				return 0, bad
			}
			dataSeen = true
			dataBytes = uint32(size)
		}
		at += int(size)
		if size%2 != 0 {
			at++
			if at > len(data) {
				return 0, bad
			}
		}
	}
	if !fmtSeen || !dataSeen || dataBytes%align != 0 {
		return 0, bad
	}
	duration := float64(dataBytes) / float64(rate*align)
	if duration <= 0 || duration > 30 {
		return 0, bad
	}
	return duration, nil
}
func validateMedia(upload MediaUpload) (MediaAsset, error) {
	v := MediaAsset{Name: strings.TrimSpace(upload.Name), Bytes: len(upload.Data)}
	if !mediaName(v.Name) || len(upload.Data) == 0 || len(upload.Data) > MaxMediaBytes {
		return v, fmt.Errorf("Choose an image or WAV under 8 MB with a name up to 80 characters")
	}
	hash := sha256.Sum256(upload.Data)
	v.ID = hex.EncodeToString(hash[:])
	if upload.ID != "" && upload.ID != v.ID {
		return v, fmt.Errorf("Media content does not match its ID")
	}
	if bytes.HasPrefix(upload.Data, []byte("RIFF")) {
		duration, err := waveDuration(upload.Data)
		if err != nil {
			return v, err
		}
		v.Kind = "sound"
		v.Format = "wav"
		v.Duration = duration
		return v, nil
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(upload.Data))
	if err != nil || !one(format, "png", "jpeg") || !mediaDimensions(cfg.Width, cfg.Height) {
		return v, fmt.Errorf("Choose a PNG or JPG image up to 4096 × 4096")
	}
	if _, _, err = image.Decode(bytes.NewReader(upload.Data)); err != nil {
		return v, fmt.Errorf("The image could not be read")
	}
	v.Kind = "image"
	v.Format = format
	v.Width = cfg.Width
	v.Height = cfg.Height
	return v, nil
}

func (s *Store) mediaDirectory(create bool) (string, error) {
	dir := filepath.Join(s.dir, "media")
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Media directory unavailable")
	}
	return dir, nil
}

// Called once at startup. Media errors cannot disable unrelated healthy rules.
func (s *Store) loadMedia() {
	s.media = MediaLibrary{Version: 1, Assets: []MediaAsset{}}
	path := filepath.Join(s.dir, "media.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 128*1024 {
		s.mediaErr = fmt.Errorf("Media library unavailable")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.mediaErr = err
		return
	}
	var lib MediaLibrary
	if json.Unmarshal(data, &lib) != nil || lib.Version != 1 || lib.Revision < 0 || len(lib.Assets) > 64 {
		s.mediaErr = fmt.Errorf("Invalid media library")
		return
	}
	total := 0
	seen := map[string]bool{}
	for _, v := range lib.Assets {
		total += v.Bytes
		if !backgroundHash(v.ID) || seen[v.ID] || !mediaName(v.Name) || v.Bytes < 1 || v.Bytes > MaxMediaBytes || total > MaxMediaLibraryBytes || !(v.Kind == "image" && one(v.Format, "png", "jpeg") && mediaDimensions(v.Width, v.Height) || v.Kind == "sound" && v.Format == "wav" && bounded(v.Duration, 0.00001, 30)) {
			s.mediaErr = fmt.Errorf("Invalid media library")
			return
		}
		seen[v.ID] = true
	}
	if lib.Assets == nil {
		lib.Assets = []MediaAsset{}
	}
	s.media = lib
	s.refreshMediaLocked()
}

// File availability refreshes on library requests, never on the capture loop.
func (s *Store) refreshMediaLocked() {
	s.mediaUnavailable = map[string]bool{}
	dir, err := s.mediaDirectory(false)
	for _, v := range s.media.Assets {
		if err != nil {
			s.mediaUnavailable[v.ID] = true
			continue
		}
		info, e := os.Lstat(filepath.Join(dir, v.ID+"."+v.Format))
		if e != nil || !info.Mode().IsRegular() || info.Size() != int64(v.Bytes) {
			s.mediaUnavailable[v.ID] = true
		}
	}
}
func (s *Store) mediaStatusLocked() MediaStatus {
	v := MediaStatus{MediaLibrary: s.media}
	v.Assets = append([]MediaAsset{}, s.media.Assets...)
	for i := range v.Assets {
		v.Assets[i].Available = s.mediaErr == nil && !s.mediaUnavailable[v.Assets[i].ID]
	}
	if s.mediaErr != nil {
		v.Error = s.mediaErr.Error()
	}
	for id, kinds := range mediaRequirements(s.doc.Rules) {
		asset := s.mediaAssetLocked(id)
		if asset.ID == "" || s.mediaUnavailable[id] || kinds == 3 || kinds == 1 && asset.Kind != "image" || kinds == 2 && asset.Kind != "sound" {
			v.Missing = append(v.Missing, id)
		}
	}
	sort.Strings(v.Missing)
	return v
}
func (s *Store) Media() MediaStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshMediaLocked()
	return s.mediaStatusLocked()
}
func (s *Store) mediaAssetLocked(id string) MediaAsset {
	for _, v := range s.media.Assets {
		if v.ID == id {
			return v
		}
	}
	return MediaAsset{}
}
func (s *Store) mediaPathLocked(id, kind string) string {
	if s.mediaErr != nil || s.mediaUnavailable[id] {
		return ""
	}
	v := s.mediaAssetLocked(id)
	if v.ID == "" || kind != "" && kind != v.Kind {
		return ""
	}
	return filepath.Join(s.dir, "media", v.ID+"."+v.Format)
}
func (s *Store) MediaFile(id string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !backgroundHash(id) {
		return "", "", os.ErrNotExist
	}
	if _, err := s.mediaDirectory(false); err != nil {
		return "", "", os.ErrNotExist
	}
	v := s.mediaAssetLocked(id)
	path := s.mediaPathLocked(id, "")
	if path == "" {
		return "", "", os.ErrNotExist
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(v.Bytes) {
		return "", "", os.ErrNotExist
	}
	mime := "image/" + v.Format
	if v.Kind == "sound" {
		mime = "audio/wav"
	}
	return path, mime, nil
}
func (s *Store) SaveMedia(revision int64, uploads []MediaUpload) (MediaStatus, error) {
	if len(uploads) < 1 || len(uploads) > 64 {
		return MediaStatus{}, fmt.Errorf("Choose up to 64 media files")
	}
	assets := make([]MediaAsset, 0, len(uploads))
	total := 0
	seen := map[string]bool{}
	for _, upload := range uploads {
		total += len(upload.Data)
		if total > MaxMediaBatchBytes {
			return MediaStatus{}, fmt.Errorf("Keep imported media under 16 MB")
		}
		v, err := validateMedia(upload)
		if err != nil {
			return MediaStatus{}, err
		}
		if seen[v.ID] {
			return MediaStatus{}, fmt.Errorf("Duplicate media in import")
		}
		seen[v.ID] = true
		assets = append(assets, v)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mediaErr != nil {
		return s.mediaStatusLocked(), s.mediaErr
	}
	if revision != s.media.Revision {
		return s.mediaStatusLocked(), ErrMediaConflict
	}
	lib := s.media
	lib.Assets = append([]MediaAsset{}, lib.Assets...)
	size := 0
	for _, v := range lib.Assets {
		size += v.Bytes
	}
	for _, v := range assets {
		if s.mediaAssetLocked(v.ID).ID == "" {
			lib.Assets = append(lib.Assets, v)
			size += v.Bytes
		}
	}
	if len(lib.Assets) > 64 || size > MaxMediaLibraryBytes {
		return s.mediaStatusLocked(), fmt.Errorf("Remove unused media first (64 files, 64 MB maximum)")
	}
	dir, err := s.mediaDirectory(true)
	if err != nil {
		return s.mediaStatusLocked(), err
	}
	created := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, path := range created {
				_ = os.Remove(path)
			}
		}
	}()
	for i, v := range assets {
		path := filepath.Join(dir, v.ID+"."+v.Format)
		if info, e := os.Lstat(path); e == nil {
			if !info.Mode().IsRegular() || info.Size() != int64(v.Bytes) {
				return s.mediaStatusLocked(), fmt.Errorf("Existing media is invalid")
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return s.mediaStatusLocked(), e
			}
			h := sha256.Sum256(data)
			if hex.EncodeToString(h[:]) != v.ID {
				return s.mediaStatusLocked(), fmt.Errorf("Existing media is invalid")
			}
			continue
		} else if !os.IsNotExist(e) {
			return s.mediaStatusLocked(), e
		}
		if err = atomic(path, uploads[i].Data); err != nil {
			return s.mediaStatusLocked(), err
		}
		created = append(created, path)
	}
	sort.Slice(lib.Assets, func(i, j int) bool { return strings.ToLower(lib.Assets[i].Name) < strings.ToLower(lib.Assets[j].Name) })
	lib.Revision++
	data, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return s.mediaStatusLocked(), err
	}
	if err = atomic(filepath.Join(s.dir, "media.json"), data); err != nil {
		return s.mediaStatusLocked(), err
	}
	s.media = lib
	s.refreshMediaLocked()
	committed = true
	return s.mediaStatusLocked(), nil
}
func (s *Store) EditMedia(revision int64, id, name string, remove bool) (MediaStatus, error) {
	name = strings.TrimSpace(name)
	if !backgroundHash(id) || !remove && !mediaName(name) {
		return MediaStatus{}, fmt.Errorf("Invalid media selection or name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mediaErr != nil {
		return s.mediaStatusLocked(), s.mediaErr
	}
	if revision != s.media.Revision {
		return s.mediaStatusLocked(), ErrMediaConflict
	}
	old := s.mediaAssetLocked(id)
	if old.ID == "" {
		return s.mediaStatusLocked(), os.ErrNotExist
	}
	if remove {
		if s.historyErr != nil {
			return s.mediaStatusLocked(), fmt.Errorf("Recover pack backups before removing media")
		}
		if s.historyUsesMedia(id) {
			return s.mediaStatusLocked(), fmt.Errorf("This media is used by a pack backup. Remove that backup first")
		}
		for _, used := range MediaReferences(s.doc.Rules) {
			if used == id {
				return s.mediaStatusLocked(), fmt.Errorf("This media is used by a saved overlay")
			}
		}
	}
	lib := s.media
	lib.Assets = []MediaAsset{}
	for _, v := range s.media.Assets {
		if v.ID == id {
			if remove {
				continue
			}
			v.Name = name
		}
		lib.Assets = append(lib.Assets, v)
	}
	lib.Revision++
	data, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return s.mediaStatusLocked(), err
	}
	if err = atomic(filepath.Join(s.dir, "media.json"), data); err != nil {
		return s.mediaStatusLocked(), err
	}
	s.media = lib
	if remove {
		if dir, err := s.mediaDirectory(false); err == nil {
			_ = os.Remove(filepath.Join(dir, old.ID+"."+old.Format))
		}
	}
	return s.mediaStatusLocked(), nil
}
func validSound(name string, empty bool) bool {
	return empty && name == "" || one(name, "chime", "alert", "soft", "sonar") || strings.HasPrefix(name, "media:") && backgroundHash(strings.TrimPrefix(name, "media:"))
}
func mediaRequirements(rules []Rule) map[string]int {
	refs := map[string]int{}
	sound := func(name string) {
		if strings.HasPrefix(name, "media:") {
			refs[strings.TrimPrefix(name, "media:")] |= 2
		}
	}
	for _, r := range rules {
		if r.Style.Visual != nil && r.Style.Visual.Image != "" {
			refs[r.Style.Visual.Image] |= 1
		}
		sound(r.Style.OnShow)
		sound(r.Style.OnHide)
		for _, m := range r.Style.Modifiers {
			sound(m.Sound)
		}
		var walk func([]Node)
		walk = func(ns []Node) {
			for _, n := range ns {
				if n.Type == "sound" {
					sound(str(n.Props, "sound"))
				}
				walk(n.Children)
			}
		}
		walk(r.Roots)
	}
	return refs
}
func MediaReferences(rules []Rule) []string {
	refs := mediaRequirements(rules)
	out := []string{}
	for id := range refs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
