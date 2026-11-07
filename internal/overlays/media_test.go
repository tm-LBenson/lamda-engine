package overlays

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

func imageMedia(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 24, 16))
	img.Set(8, 8, color.NRGBA{R: 255, A: 128})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func soundMedia() []byte {
	data := make([]byte, 44+800)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 8000)
	binary.LittleEndian.PutUint32(data[28:], 8000)
	binary.LittleEndian.PutUint16(data[32:], 1)
	binary.LittleEndian.PutUint16(data[34:], 8)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], 800)
	return data
}
func TestMediaValidationBoundsAndWaveChunks(t *testing.T) {
	good := MediaUpload{Name: " Arrow ", Data: imageMedia(t)}
	asset, err := validateMedia(good)
	if err != nil || asset.Kind != "image" || asset.Width != 24 || asset.Height != 16 || asset.Name != "Arrow" {
		t.Fatal(asset, err)
	}
	for _, u := range []MediaUpload{{Name: "a\x00b", Data: good.Data}, {Name: "empty", Data: nil}, {Name: "bad", Data: good.Data[:len(good.Data)/2]}, {Name: "bad", Data: []byte("<svg/>")}, {Name: "bad", ID: strings.Repeat("a", 64), Data: good.Data}} {
		if _, e := validateMedia(u); e == nil {
			t.Fatal("invalid media accepted")
		}
	}
	wave := soundMedia()
	v, err := validateMedia(MediaUpload{Name: "Test sound", Data: wave})
	if err != nil || v.Kind != "sound" || v.Duration != .1 {
		t.Fatal(v, err)
	}
	for _, patch := range []func([]byte){func(b []byte) { binary.LittleEndian.PutUint16(b[20:], 3) }, func(b []byte) { binary.LittleEndian.PutUint16(b[22:], 6) }, func(b []byte) { binary.LittleEndian.PutUint32(b[28:], 3) }, func(b []byte) { binary.LittleEndian.PutUint32(b[40:], 9999999) }, func(b []byte) { binary.LittleEndian.PutUint32(b[4:], 12) }} {
		bad := append([]byte{}, wave...)
		patch(bad)
		if _, err := waveDuration(bad); err == nil {
			t.Fatal("invalid sound accepted")
		}
	}
}
func TestMediaLibraryRoundtripConflictReferencesAndRemoval(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	uploads := []MediaUpload{{Name: "Arrow", Data: imageMedia(t)}, {Name: "Ping", Data: soundMedia()}}
	lib, err := s.SaveMedia(0, uploads)
	if err != nil || len(lib.Assets) != 2 {
		t.Fatal(lib, err)
	}
	img, sound := lib.Assets[0], lib.Assets[1]
	if _, err = s.SaveMedia(0, uploads); !errors.Is(err, ErrMediaConflict) {
		t.Fatal("stale import accepted", err)
	}
	lib, err = s.SaveMedia(1, uploads)
	if err != nil || len(lib.Assets) != 2 || lib.Revision != 2 {
		t.Fatal("duplicate upload not idempotent", lib, err)
	}
	for _, v := range lib.Assets {
		path, mime, e := s.MediaFile(v.ID)
		if e != nil || !strings.Contains(mime, v.Format) || filepath.Base(path) != v.ID+"."+v.Format {
			t.Fatal(path, mime, e)
		}
	}
	r := rule()
	r.Enabled = false
	r.Style.Visual = &VisualStyle{Image: img.ID}
	r.Style.OnHide = "media:" + sound.ID
	if err = s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditMedia(2, img.ID, "", true); err == nil {
		t.Fatal("deleted referenced image")
	}
	if _, err = s.EditMedia(2, sound.ID, "", true); err == nil {
		t.Fatal("deleted referenced sound")
	}
	lib, err = s.EditMedia(2, img.ID, "Renamed", false)
	if err != nil || s.doc.Revision != 1 {
		t.Fatal("rename changed rules", err)
	}
	reopened, err := Open(dir)
	if err != nil || len(reopened.Media().Assets) != 2 || len(reopened.Media().Missing) != 0 {
		t.Fatal("media lost on reopen", err)
	}
	if err = s.Save(1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditMedia(3, img.ID, "", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.MediaFile(img.ID); !os.IsNotExist(err) {
		t.Fatal("removed file available")
	}
	for _, id := range []string{"../backgrounds/secret", strings.Repeat("A", 64), "sonar"} {
		if _, _, err = s.MediaFile(id); !os.IsNotExist(err) {
			t.Fatal("unsafe id accepted")
		}
	}
}
func TestMediaFailedBatchRollsBackAndPrivateScreenshotsStayPrivate(t *testing.T) {
	s, _ := Open(t.TempDir())
	good := MediaUpload{Name: "Image", Data: imageMedia(t)}
	if _, err := s.SaveMedia(0, []MediaUpload{good, {Name: "Bad", Data: []byte("bad")}}); err == nil {
		t.Fatal("corrupt batch accepted")
	}
	if len(s.Media().Assets) != 0 {
		t.Fatal("partial import")
	}
	if err := os.Mkdir(filepath.Join(s.dir, "media.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveMedia(0, []MediaUpload{good}); err == nil {
		t.Fatal("blocked metadata write succeeded")
	}
	files, _ := os.ReadDir(filepath.Join(s.dir, "media"))
	if len(files) != 0 || s.Media().Revision != 0 {
		t.Fatal("failed commit left files")
	}
	os.Remove(filepath.Join(s.dir, "media.json"))
	_, err := s.SaveBackground(0, "Person", "Realm", good.Data)
	if err != nil {
		t.Fatal(err)
	}
	asset, _ := validateMedia(good)
	if _, _, err = s.MediaFile(asset.ID); !os.IsNotExist(err) {
		t.Fatal("private screenshot exposed as media")
	}
	if err = os.WriteFile(filepath.Join(s.dir, "media.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.dir)
	if err != nil || reopened.Media().Error == "" {
		t.Fatal("broken media disabled store or hid error", err)
	}
}
func TestMediaRuntimePreservesOtherDisplaysAndSuppressesMissingSound(t *testing.T) {
	s, _ := Open(t.TempDir())
	lib, err := s.SaveMedia(0, []MediaUpload{{Name: "Arrow", Data: imageMedia(t)}, {Name: "Ping", Data: soundMedia()}})
	if err != nil {
		t.Fatal(err)
	}
	img, sound := lib.Assets[0], lib.Assets[1]
	r := rule()
	r.Style.Visual = &VisualStyle{Image: img.ID}
	r.Roots[0].Children[0].Children[0].Props["display"] = "image"
	r.Roots[0].Children[0].Children[1].Props["sound"] = "media:" + sound.ID
	r.Roots[0].Children[0].Children = append(r.Roots[0].Children[0].Children, Node{ID: "text", Type: "show", Props: map[string]any{"display": "text", "text": "Still works", "duration": 5.0}})
	if err = s.Save(0, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0)
	s.Accept(sample([]Rule{r}, now, 1), now)
	out := s.Merge(engine.FrameDisplay{}, now)
	if len(out.Overlays) != 2 || len(out.OverlaySounds) != 1 || out.OverlaySounds[0].Path == "" {
		t.Fatal("media output missing", out)
	}
	if out.Overlays[0].Kind != "image" || out.Overlays[0].ImagePath == "" {
		t.Fatal("image path missing")
	}
	path, _, _ := s.MediaFile(img.ID)
	os.Remove(path)
	_ = s.Media()
	out = s.Merge(engine.FrameDisplay{}, now)
	if len(out.Overlays) != 1 || out.Overlays[0].Text != "Still works" || len(s.Media().Missing) != 1 {
		t.Fatal("missing image hid healthy display")
	}
	path, _, _ = s.MediaFile(sound.ID)
	os.Remove(path)
	_ = s.Media()
	out = s.Merge(engine.FrameDisplay{}, now)
	if len(out.OverlaySounds) != 0 || len(s.Media().Missing) != 2 {
		t.Fatal("missing sound emitted")
	}
}
