package lookup

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"
)

// Opt-in acceptance check against a screenshot of the real visible game.
func TestCapturedMarketBoard(t *testing.T) {
	path := os.Getenv("LAMDA_MARKET_SCREENSHOT")
	if path == "" {
		t.Skip("real Market capture not requested")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	p := Pixels{Width: 512, Height: 64, BGRA: make([]byte, 512*64*4)}
	for y := 0; y < 64; y++ {
		for x := 0; x < 512; x++ {
			r, g, b, a := im.At(x, y+416).RGBA()
			i := (y*512 + x) * 4
			p.BGRA[i], p.BGRA[i+1], p.BGRA[i+2], p.BGRA[i+3] = byte(b>>8), byte(g>>8), byte(r>>8), byte(a>>8)
		}
	}
	data := p.Cells(image.Rect(0, 0, 512, 64))
	s := DecodeSample(data)
	if s.Error != "" {
		t.Fatalf("real frame %q: %s", string(data[:10]), s.Error)
	}
	line, _, _ := bytes.Cut(s.Market, []byte{'\n'})
	t.Logf("Real visible Market frame accepted: %s", line)
}

func marketPixels(packet []byte) Pixels {
	p := Pixels{Width: MarketBridgeWidth, Height: MarketBridgeHeight, BGRA: make([]byte, MarketBridgeWidth*MarketBridgeHeight*4)}
	for i := 0; i < len(packet)/3; i++ {
		x, y := (i%(p.Width/2))*2, (i/(p.Width/2))*2
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				at := ((y+dy)*p.Width + x + dx) * 4
				p.BGRA[at], p.BGRA[at+1], p.BGRA[at+2], p.BGRA[at+3] = packet[i*3+2], packet[i*3+1], packet[i*3], 255
			}
		}
	}
	return p
}

func TestMarketBulkPixelsGeometryCRCAndVersion(t *testing.T) {
	body := append([]byte("3|big|1|1|24000\n"), bytes.Repeat([]byte("x"), 24000)...)
	packet := simPacketFixture("LMK2", MarketBridgeBytes, body)
	pixels := marketPixels(packet)
	s, found := captureMarketBoard(BridgePosition{X: 1900, Y: 1070}, 1920, 1080, func(p BridgePosition, w, h int) (Pixels, bool) {
		if p != (BridgePosition{1408, 1016}) || w != 512 || h != 64 {
			t.Fatal("bulk origin not clamped to its own geometry", p, w, h)
		}
		return pixels, true
	})
	if !found || s.Error != "" || !bytes.Equal(s.Market, body) {
		t.Fatal("bulk raster rejected", found, s.Error)
	}
	// A frame changed while captured remains invalid, even at the last cell.
	packet[len(body)+9] ^= 1
	s, found = captureMarketBoard(BridgePosition{}, 1920, 1080, func(BridgePosition, int, int) (Pixels, bool) { return marketPixels(packet), true })
	if !found || s.Error != "bridge_checksum" {
		t.Fatal("torn frame accepted", found, s.Error)
	}
	for _, tc := range []struct {
		magic string
		size  int
		body  string
	}{{"LMK1", SimBridgeBytes, "3|"}, {"LMK2", MarketBridgeBytes, "2|"}, {"LMK2", SimBridgeBytes, "3|"}} {
		if s := DecodeSample(simPacketFixture(tc.magic, tc.size, []byte(tc.body))); s.Error == "" {
			t.Fatal("mixed geometry/version accepted", tc)
		}
	}
}

func TestMarketBulkProbePreservesLegacyFallback(t *testing.T) {
	legacy := marketPixels(simPacketFixture("LMK1", MarketBridgeBytes, []byte("1|legacy")))
	if _, found := captureMarketBoard(BridgePosition{}, 1920, 1080, func(BridgePosition, int, int) (Pixels, bool) { return legacy, true }); found {
		t.Fatal("legacy strip consumed as bulk")
	}
	if _, found := captureMarketBoard(BridgePosition{}, 400, 100, func(BridgePosition, int, int) (Pixels, bool) {
		t.Fatal("out of bounds capture")
		return Pixels{}, false
	}); found {
		t.Fatal("small window accepted")
	}
}
