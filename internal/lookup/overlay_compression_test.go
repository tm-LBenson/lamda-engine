package lookup

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"testing"
)

func compressedMetadata(t *testing.T, b []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteString("Z1")
	w, err := flate.NewWriter(&out, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(b)
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestOverlayCompressedMetadataBoundsAndTruncation(t *testing.T) {
	raw := bytes.Repeat([]byte("a public field "), 1000)
	b := compressedMetadata(t, raw)
	expanded, e := expandOverlayMetadata(b)
	if e != nil || !bytes.Equal(expanded, raw) {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{[]byte("Z1broken"), b[:len(b)-2], append(append([]byte(nil), b...), 0), compressedMetadata(t, bytes.Repeat([]byte("x"), 65537)), bytes.Repeat([]byte("x"), 6135)} {
		if _, e := expandOverlayMetadata(bad); e == nil {
			t.Fatal("malformed/oversized metadata accepted")
		}
	}
	health, active := 99.0, true
	p := OverlayInput{Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, Native: true, Health: &health, Watches: []OverlayAura{{Spell: 123, Kind: "buff", Unit: "player", Available: true, Active: &active}}}
	raw, _ = json.Marshal(p)
	s := parseOverlays(compressedMetadata(t, raw))
	if s.Error != "" || s.Overlays == nil || s.Overlays.Health != nil || s.Overlays.Watches[0].Active != nil {
		t.Fatal("compressed JSON injected native readings", s)
	}
	s = parseOverlays(raw)
	if s.Error != "" {
		t.Fatal("legacy JSON changed", s.Error)
	}
}
