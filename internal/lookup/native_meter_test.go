package lookup

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"
)

var meterGlyphs = []string{
	"01110100011001110101110011000101110", "00100011000010000100001000010001110",
	"01110100010000100010001000100011111", "11110000010000101110000010000111110",
	"00010001100101010010111110001000010", "11111100001000011110000010000111110",
	"01110100001000011110100011000101110", "11111000010001000100010000100001000",
	"01110100011000101110100011000101110", "01110100011000101111000010000101110",
	"10001100010101000100010101000110001",
}

func clearNativeRect(p Pixels, x, y, w, h int) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			paintCastColor(p, xx, yy, 1, 0)
		}
	}
}
func init() {
	for id := range meterGlyphs {
		var b strings.Builder
		for k := 0; k < 35; k++ {
			v := id & ((k % 15) + 1)
			parity := 0
			for v > 0 {
				parity ^= v & 1
				v >>= 1
			}
			if k == 0 || k == 4 || k == 30 || k == 34 || parity == 1 {
				b.WriteByte('1')
			} else {
				b.WriteByte('0')
			}
		}
		meterGlyphs[id] = b.String()
	}
}
func paintMeterText(p Pixels, x, y int, s string) {
	for i, ch := range s {
		id := int(ch - '0')
		if ch == 'X' {
			id = 10
		}
		if id < 0 || id > 10 {
			continue
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if meterGlyphs[id][row*5+col] == '1' {
					paintCastColor(p, x+i*6+col, y+4+row, 1, 0xffffff)
				}
			}
		}
	}
}
func nativePacket(body string) []byte {
	p := make([]byte, 6144)
	copy(p, "LUM1")
	copy(p[10:], body)
	binary.BigEndian.PutUint16(p[4:6], uint16(len(body)))
	binary.BigEndian.PutUint32(p[6:10], crc32.ChecksumIEEE([]byte(body)))
	return p
}
func nativeFixture() (*NativeMeter, Pixels) {
	body := "1|123:45:1|7|68|Player-1-Aug|1473|1|6000|32|12345\n1|Player-1-Aug|13|Aug|0\n2|Player-1-Ally|8|Ally|0"
	n := DecodeSample(nativePacket(body)).NativeMeter
	p := Pixels{Width: 256, Height: n.Height(), BGRA: make([]byte, 256*n.Height()*4)}
	paintMeterText(p, 12, 0, "0123456789X")
	for index, value := range []int64{12, 4294967296, 999999999999999} {
		y := 16 + index*16
		h := []byte{173, byte(index), 118, 0, 0, 7, 59, 0, 93, 205, 127, 61}
		for i := 0; i < 7; i++ {
			h[7] += h[i] * byte(i+1)
		}
		for cell := 0; cell < 4; cell++ {
			j := cell * 3
			paintCastColor(p, 2+cell%2*2, y+6+cell/2*2, 2, int(h[j])<<16|int(h[j+1])<<8|int(h[j+2]))
		}
		paintMeterText(p, 12, y, fmt.Sprintf("%015dX", value))
		paintMeterText(p, 140, y, fmt.Sprintf("%015dX", value))
	}
	return n, p
}
func TestNativeMeterCarriesFullIntegersWithout24BitTruncation(t *testing.T) {
	n, p := nativeFixture()
	DecodeNativeMeter(n, p, 0)
	if !n.Captured || n.Error != "" || n.Members[0].Total != 4294967296 || n.Members[1].Total != 999999999999999 {
		t.Fatal(n)
	}
}
func TestNativeMeterRejectsTearingOverflowAndUnusableSamples(t *testing.T) {
	for _, kind := range []string{"copy", "serial", "marker", "overflow", "calibration", "duration"} {
		t.Run(kind, func(t *testing.T) {
			n, p := nativeFixture()
			switch kind {
			case "copy":
				clearNativeRect(p, 140, 32, 116, 16)
			case "serial":
				paintCastColor(p, 4, 38, 2, 0)
			case "marker":
				paintCastColor(p, 12+15*6, 32, 6, 0)
			case "overflow":
				clearNativeRect(p, 12, 32, 116, 16)
				clearNativeRect(p, 140, 32, 116, 16)
				paintMeterText(p, 12, 32, "10000000000000000X")
				paintMeterText(p, 140, 32, "10000000000000000X")
			case "calibration":
				clearNativeRect(p, 12, 0, 116, 16)
			case "duration":
				n.DurationMillis = 15000
			}
			DecodeNativeMeter(n, p, 0)
			if n.Captured || n.Error == "" || n.Members[0].Total != 0 {
				t.Fatal("bad frame accepted", n)
			}
		})
	}
}
func TestNativeMeterMetadataAndCRCBindRosterAndSession(t *testing.T) {
	good := "1|123:45:1|7|68|Player-1-Aug|1473|1|6000|32|12345\n1|Player-1-Aug|13|Aug|0"
	for _, body := range []string{strings.Replace(good, "6000", "99999", 1), strings.Replace(good, "|7|", "|0|", 1), strings.Replace(good, "1|Player-1-Aug|13|Aug|0", "2|Player-1-Aug|13|Aug|0", 1), good + "\n2|Player-1-Aug|13|Duplicate|0", strings.Replace(good, "|32|", "|31|", 1)} {
		if DecodeSample(nativePacket(body)).Error == "" {
			t.Fatal("accepted", body)
		}
	}
	p := nativePacket(good)
	p[18] ^= 1
	if DecodeSample(p).Error != "bridge_checksum" {
		t.Fatal("CRC did not bind metadata")
	}
}

func TestNativeMeterOwnedAuraPixelsAreDuplicatedAndRosterBound(t *testing.T) {
	n, p := nativeFixture()
	n.Members[0].AuraMode = 15
	for bit := 0; bit < 4; bit++ {
		color := 0xa02060
		if bit == 0 || bit == 3 {
			color = 0x18e060
		}
		for _, x := range []int{128, 134} {
			paintCastColor(p, x, 32+3+bit*2, 2, color)
		}
	}
	DecodeNativeMeter(n, p, 0)
	if !n.Captured || n.Members[0].AuraMask != 9 {
		t.Fatal(n)
	}
	n, p = nativeFixture()
	n.Members[0].AuraMode = 15
	for bit := 0; bit < 4; bit++ {
		for _, x := range []int{128, 134} {
			paintCastColor(p, x, 32+3+bit*2, 2, 0xa02060)
		}
	}
	paintCastColor(p, 128, 35, 2, 0x18e060)
	DecodeNativeMeter(n, p, 0)
	if n.Captured || n.Error != "native_aura_duplicate_mismatch" {
		t.Fatal(n)
	}
}

func TestNativeMeterPublicFailurePacket(t *testing.T) {
	for _, body := range []string{"0|Waiting for native font layout", "0|Meter font unavailable"} {
		sample := DecodeSample(nativePacket(body))
		if sample.Source != "augmentation" || sample.Error != "native_meter_addon: "+body[2:] || sample.NativeMeter != nil {
			t.Fatal(sample)
		}
	}
	for _, body := range []string{"0|", "0|bad\nrow", "0|bad|field"} {
		if sample := DecodeSample(nativePacket(body)); sample.Error != "native_meter_metadata_invalid" {
			t.Fatal(sample)
		}
	}
}
