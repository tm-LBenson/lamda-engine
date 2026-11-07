package overlays

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func screenshot(t *testing.T, width, height int, jpg bool) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, width, height))
	im.Set(0, 0, color.White)
	var b bytes.Buffer
	var err error
	if jpg {
		err = jpeg.Encode(&b, im, nil)
	} else {
		err = png.Encode(&b, im)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestBackgroundLibraryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(s.State())
	pngData := screenshot(t, 160, 90, false)
	d, err := s.SaveBackground(0, " Example ", "Area 52", pngData)
	if err != nil {
		t.Fatal(err)
	}
	first := d.Images[0]
	if d.Revision != 1 || first.Width != 160 || first.Character != "Example" {
		t.Fatal(d)
	}
	path, mime, err := s.BackgroundFile(first.ID, first.Image)
	data, _ := os.ReadFile(path)
	if err != nil || mime != "image/png" || !bytes.Equal(data, pngData) {
		t.Fatal("image bytes changed", err)
	}
	if _, err = s.SaveBackground(0, "Another", "Realm", pngData); !errors.Is(err, ErrBackgroundConflict) {
		t.Fatal("stale upload accepted", err)
	}
	// Same name on another realm is independent; shared content survives replacement.
	d, err = s.SaveBackground(1, "Example", "Other Realm", pngData)
	if err != nil {
		t.Fatal(err)
	}
	d, err = s.SaveBackground(2, "EXAMPLE", "area-52", screenshot(t, 210, 90, true))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Images) != 2 || d.Revision != 3 {
		t.Fatal(d)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("shared screenshot deleted", err)
	}
	if _, _, err = s.BackgroundFile(first.ID, first.Image); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale version served", err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Backgrounds()
	if err != nil || loaded.Revision != 3 || len(loaded.Images) != 2 {
		t.Fatal(loaded, err)
	}
	if _, err = reopened.RemoveBackground(2, first.ID); !errors.Is(err, ErrBackgroundConflict) {
		t.Fatal(err)
	}
	loaded, err = reopened.RemoveBackground(3, first.ID)
	if err != nil || len(loaded.Images) != 1 {
		t.Fatal(loaded, err)
	}
	loaded, err = reopened.RemoveBackground(4, loaded.Images[0].ID)
	if err != nil || len(loaded.Images) != 0 {
		t.Fatal(loaded, err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unused screenshot retained")
	}
	after, _ := json.Marshal(s.State())
	if !bytes.Equal(before, after) {
		t.Fatal("screenshot changed rules/runtime")
	}
	if _, err = os.Stat(filepath.Join(dir, "rules.json")); !os.IsNotExist(err) {
		t.Fatal("screenshot wrote rules")
	}
}
func TestBackgroundRejectsInvalidFilesAndPaths(t *testing.T) {
	s, _ := Open(t.TempDir())
	valid := screenshot(t, 160, 90, false)
	for _, v := range []struct {
		character, realm string
		data             []byte
	}{
		{"", "Realm", valid}, {"Name", "  ", valid}, {"Name\n", "Realm", valid[:20]},
		{"Name", "Realm", []byte("<svg></svg>")}, {"Name", "Realm", valid[:len(valid)/2]},
		{"Name", "Realm", screenshot(t, 10, 10, false)}, {"Name", "Realm", make([]byte, MaxBackgroundBytes+1)},
	} {
		if _, err := s.SaveBackground(0, v.character, v.realm, v.data); err == nil {
			t.Fatal("invalid upload accepted")
		}
	}
	if _, _, err := s.BackgroundFile("../../rules.json", ""); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	d, _ := s.Backgrounds()
	if d.Revision != 0 || len(d.Images) != 0 {
		t.Fatal("invalid uploads changed library")
	}
}
