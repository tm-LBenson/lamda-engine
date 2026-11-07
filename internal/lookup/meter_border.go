package lookup

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"regexp"
)

const MeterTileWidth = 8
const MeterTileHeight = 8
const MeterTileBytes = 36

var nativeMeterScope = regexp.MustCompile(`^(boss|overall|previous|key|native:[1-9][0-9]{0,9}|recorded:[A-Za-z0-9_/:.-]{1,100})$`)

// Edges/weights are top,right,bottom,left. The default thin border grows at
// the bottom; configurable weights can share overflow across multiple edges.
type MeterBorderLayout struct {
	Generic     bool   `json:"generic,omitempty"`
	Fast        bool   `json:"fast,omitempty"`
	InnerWidth  int    `json:"innerWidth"`
	InnerHeight int    `json:"innerHeight"`
	Edges       [4]int `json:"edges"`
}

func (b MeterBorderLayout) Standalone() bool { return b.InnerHeight == 0 }

func PlanMeterStrip(width, tiles, maxWidth, maxHeight int) (MeterBorderLayout, error) {
	width = min(width, maxWidth) / 8 * 8
	if width < 320 || width > 4096 || tiles < 1 || tiles > 4096 {
		return MeterBorderLayout{}, fmt.Errorf("invalid meter strip dimensions")
	}
	height := (tiles + width/8 - 1) / (width / 8) * 8
	if height > maxHeight {
		return MeterBorderLayout{}, fmt.Errorf("meter strip does not fit")
	}
	return MeterBorderLayout{InnerWidth: width, Edges: [4]int{height, 0, 0, 0}}, nil
}

func (b MeterBorderLayout) Width() int  { return b.InnerWidth + b.Edges[1] + b.Edges[3] }
func (b MeterBorderLayout) Height() int { return b.InnerHeight + b.Edges[0] + b.Edges[2] }
func (b MeterBorderLayout) Bands() []image.Rectangle {
	w, h := b.Width(), b.Height()
	t, r, bottom, l := b.Edges[0], b.Edges[1], b.Edges[2], b.Edges[3]
	return []image.Rectangle{image.Rect(0, 0, w, t), image.Rect(w-r, t, w, h-bottom), image.Rect(0, h-bottom, w, h), image.Rect(0, t, l, h-bottom)}
}
func (b MeterBorderLayout) capacity() int {
	return b.Width()/8*(b.Edges[0]/8+b.Edges[2]/8) + (b.Edges[1]/8+b.Edges[3]/8)*(b.InnerHeight/8)
}
func (b MeterBorderLayout) Slots() []image.Point {
	result := make([]image.Point, 0, b.capacity())
	w, h := b.Width(), b.Height()
	t, r, bottom, l := b.Edges[0], b.Edges[1], b.Edges[2], b.Edges[3]
	for y := 0; y+8 <= t; y += 8 {
		for x := 0; x+8 <= w; x += 8 {
			result = append(result, image.Pt(x, y))
		}
	}
	for x := w - r; x+8 <= w; x += 8 {
		for y := t; y+8 <= t+b.InnerHeight; y += 8 {
			result = append(result, image.Pt(x, y))
		}
	}
	for y := h - bottom; y+8 <= h; y += 8 {
		for x := 0; x+8 <= w; x += 8 {
			result = append(result, image.Pt(x, y))
		}
	}
	for x := 0; x+8 <= l; x += 8 {
		for y := t; y+8 <= t+b.InnerHeight; y += 8 {
			result = append(result, image.Pt(x, y))
		}
	}
	return result
}
func MeterBorderTiles(bodyBytes, members int) (int, error) {
	if bodyBytes < 1 || bodyBytes > 6134 || members < 1 || members > 40 {
		return 0, fmt.Errorf("invalid meter border payload")
	}
	// Header, public metadata, alphabet calibration, duration (15 decimal
	// digits+X twice), then each member (15 decimal digits+X twice plus aura).
	return 1 + (bodyBytes+35)/36 + 17 + 33 + 34*members, nil
}
func PlanMeterBorder(width, height, thin, tiles int, weights [4]int, maxWidth, maxHeight int) (MeterBorderLayout, error) {
	b := MeterBorderLayout{InnerWidth: width, InnerHeight: height, Edges: [4]int{thin, thin, thin, thin}}
	if width < 300 || width > 1000 || height < 120 || height > 1000 || thin < 8 || thin > 64 || tiles < 1 || tiles > 4096 || maxWidth < 300 || maxHeight < 120 || b.Width() > maxWidth || b.Height() > maxHeight {
		return b, fmt.Errorf("invalid meter border dimensions")
	}
	total := 0
	for _, weight := range weights {
		if weight < 0 || weight > 8 {
			return b, fmt.Errorf("invalid meter border weights")
		}
		total += weight
	}
	if total == 0 {
		return b, fmt.Errorf("empty meter border distribution")
	}
	for b.capacity() < tiles {
		chosen := -1
		score := 1e9
		for _, side := range []int{2, 0, 3, 1} {
			if weights[side] == 0 {
				continue
			}
			step := 8
			candidate := b
			candidate.Edges[side] += step
			if candidate.Width() > maxWidth || candidate.Height() > maxHeight {
				continue
			}
			cost := float64(b.Edges[side]-thin) / float64(weights[side])
			if cost < score {
				chosen = side
				score = cost
			}
		}
		if chosen < 0 {
			return b, fmt.Errorf("meter capture border does not fit")
		}
		step := 8
		b.Edges[chosen] += step
	}
	return b, nil
}

type MeterBorderHeader struct {
	Layout    MeterBorderLayout
	BodyBytes int
	Serial    int
	Tiles     int
	BodyCRC   uint32
}

type MeterBorderPointer struct {
	X, Y, Width, Height int
	Inactive            bool
	Paused              bool
	ControlOnly         bool
	ShareCapable        bool
	ShareOpenVersion    int
	WindowHeld          bool
	WindowHidden        bool
}

func (p MeterBorderPointer) Encode() []byte {
	b := make([]byte, 18)
	copy(b, "LML1")
	for i, v := range []int{p.X, p.Y, p.Width, p.Height} {
		binary.BigEndian.PutUint16(b[4+i*2:6+i*2], uint16(v))
	}
	if p.WindowHeld {
		b[12] = 7
	} else if p.WindowHidden {
		b[12] = 6
	} else if p.ControlOnly {
		b[12] = 3
	} else if p.ShareCapable {
		b[12] = 4
		if p.ShareOpenVersion == 2 {
			b[12] = 5
		}
	} else if p.Inactive {
		b[12] = 1
	} else if p.Paused {
		b[12] = 2
	}
	binary.BigEndian.PutUint32(b[14:], crc32.ChecksumIEEE(b[:14]))
	return b
}
func ParseMeterBorderPointer(b []byte) (MeterBorderPointer, error) {
	var p MeterBorderPointer
	if len(b) != 18 || string(b[:4]) != "LML1" || b[12] > 7 || b[13] != 0 || binary.BigEndian.Uint32(b[14:]) != crc32.ChecksumIEEE(b[:14]) {
		return p, fmt.Errorf("meter locator checksum")
	}
	p = MeterBorderPointer{X: int(binary.BigEndian.Uint16(b[4:6])), Y: int(binary.BigEndian.Uint16(b[6:8])), Width: int(binary.BigEndian.Uint16(b[8:10])), Height: int(binary.BigEndian.Uint16(b[10:12])), Inactive: b[12] == 1, Paused: b[12] == 2, ControlOnly: b[12] == 3, ShareCapable: b[12] == 4 || b[12] == 5}
	p.WindowHidden = b[12] == 6
	p.WindowHeld = b[12] == 7
	if b[12] == 5 {
		p.ShareOpenVersion = 2
	}
	if p.ControlOnly {
		if p.X > 16000 || p.Y > 16000 || p.Width != 64 || p.Height != 8 {
			return p, fmt.Errorf("meter control dimensions")
		}
		return p, nil
	}
	if p.X > 16000 || p.Y > 16000 || p.Width < 208 || p.Width > 4096 || p.Height < 8 || p.Height > 4096 {
		return p, fmt.Errorf("meter locator dimensions")
	}
	return p, nil
}

// The six-cell locator occupies the existing visibility icon's bottom left.
// Match the addon's four edge-placement branches, including window clamping.
func MeterLocatorPosition(base BridgePosition, width, height int) (BridgePosition, bool) {
	b, ok := base.Clamp(width, height)
	if !ok {
		return BridgePosition{}, false
	}
	x, y := 0, b.Y
	if y+22 > height {
		y += 16 - 22
	}
	switch {
	case b.X+208 <= width:
		x = b.X + 130
	case b.X >= 80:
		x = b.X - 80
	case b.Y >= 24:
		x = b.X + 50
		y = b.Y - 24
	default:
		x = b.X + 50
		y = b.Y + 18
	}
	x += 4
	y += 16
	return BridgePosition{X: x, Y: y}, x >= 0 && y >= 0 && x+8 <= width && y+4 <= height
}

func (h MeterBorderHeader) Encode() []byte {
	p := make([]byte, 36)
	copy(p, "LMR1")
	put := func(at, n int) { binary.BigEndian.PutUint16(p[at:at+2], uint16(n)) }
	put(4, h.BodyBytes)
	put(6, h.Layout.InnerWidth)
	put(8, h.Layout.InnerHeight)
	for i, n := range h.Layout.Edges {
		put(10+i*2, n)
	}
	p[18], p[19], p[20] = byte(h.Serial>>16), byte(h.Serial>>8), byte(h.Serial)
	p[21] = 2
	if h.Layout.Generic {
		p[21] = 5
	} else if h.Layout.Fast {
		p[21] = 4
	} else if h.Layout.Standalone() {
		p[21] = 3
	}
	put(22, h.Tiles)
	binary.BigEndian.PutUint32(p[24:28], h.BodyCRC)
	binary.BigEndian.PutUint32(p[32:36], crc32.ChecksumIEEE(p[:32]))
	return p
}
func ParseMeterBorderHeader(p []byte) (MeterBorderHeader, error) {
	var h MeterBorderHeader
	if len(p) != 36 || string(p[:4]) != "LMR1" || (p[21] != 2 && p[21] != 3 && p[21] != 4 && p[21] != 5) || binary.BigEndian.Uint32(p[28:32]) != 0 || binary.BigEndian.Uint32(p[32:]) != crc32.ChecksumIEEE(p[:32]) {
		return h, fmt.Errorf("meter border header checksum")
	}
	get := func(at int) int { return int(binary.BigEndian.Uint16(p[at : at+2])) }
	h.BodyBytes = get(4)
	h.Layout.InnerWidth = get(6)
	h.Layout.InnerHeight = get(8)
	for i := range h.Layout.Edges {
		h.Layout.Edges[i] = get(10 + i*2)
		if h.Layout.Edges[i] > 2048 || (p[21] == 2 && h.Layout.Edges[i] < 8) {
			return h, fmt.Errorf("meter border edge out of range")
		}
	}
	h.Serial = int(p[18])<<16 | int(p[19])<<8 | int(p[20])
	h.Tiles = get(22)
	h.BodyCRC = binary.BigEndian.Uint32(p[24:28])
	b := h.Layout
	validLayout := b.InnerWidth >= 300 && b.InnerWidth <= 1000 && b.InnerHeight >= 120 && b.InnerHeight <= 1000
	if p[21] == 3 || p[21] == 4 || p[21] == 5 {
		validLayout = b.InnerHeight == 0 && b.InnerWidth >= 320 && b.InnerWidth <= 4096 && b.InnerWidth%8 == 0 && b.Edges[0] >= 8 && b.Edges[0]%8 == 0 && b.Edges[1] == 0 && b.Edges[2] == 0 && b.Edges[3] == 0
	}
	if p[21] == 5 {
		h.Layout.Generic = true
		if b.InnerHeight > 0 {
			validLayout = b.InnerWidth >= 128 && b.InnerWidth <= 2000 && b.InnerHeight >= 32 && b.InnerHeight <= 2000 && b.Width() >= 208
			for _, edge := range b.Edges {
				validLayout = validLayout && edge >= 2 && edge%2 == 0
			}
		} else {
			validLayout = b.InnerWidth >= 208 && b.InnerWidth <= 4096 && b.InnerWidth%208 == 0 && b.Edges[0] >= 8 && b.Edges[0]%8 == 0 && b.Edges[1] == 0 && b.Edges[2] == 0 && b.Edges[3] == 0
		}
	}
	if p[21] == 4 || p[21] == 5 {
		h.Layout.Fast = true
		validLayout = validLayout && (p[21] == 5 || b.InnerWidth%208 == 0) && h.Tiles == b.capacity()
	}
	if h.BodyBytes < 1 || h.BodyBytes > 6134 && (p[21] != 5 || h.BodyBytes > 16384) || h.Serial < 1 || h.Tiles < 1 || h.Tiles > 4096 || !validLayout || b.Width() > 4096 || b.Height() > 4096 || b.capacity() < h.Tiles {
		return h, fmt.Errorf("meter border header dimensions")
	}
	return h, nil
}
