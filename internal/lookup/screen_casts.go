package lookup

import (
	"fmt"
	"image"
	"strconv"
	"strings"
)

const ScreenCastHeight = 176
const ColorCastHeight = 12
const MeterCastHeight = 16

type MeterOwner struct {
	GUID              string
	Serial, AgeMillis int
	Amount            int
}

type ScreenStatus struct {
	State     string  `json:"state"`
	FontWidth float64 `json:"fontWidth"`
	Displayed int     `json:"displayed"`
	Errors    int     `json:"errors"`
	Error     string  `json:"error,omitempty"`
	At        string  `json:"at,omitempty"`
}

type ScreenCastMember struct {
	Slot, Member int
	GUID         string
}
type ScreenCasts struct {
	Version  int
	Pitch    float64
	ID       string
	Offset   int
	Members  []ScreenCastMember
	Captured bool
	Rejected int
	Casts    []LiveCast
	Meter    []MeterOwner
	Error    string
}

func (s *ScreenCasts) Height() int {
	if s != nil && s.Version == 3 {
		return MeterCastHeight
	}
	if s != nil && s.Version == 2 {
		return ColorCastHeight
	}
	return ScreenCastHeight
}

func parseScreenCasts(p []string) (*ScreenCasts, error) {
	bad := fmt.Errorf("screen_cast_metadata_invalid")
	if len(p) != 6 || (p[1] != "1" && p[1] != "2" && p[1] != "3") || !observationID.MatchString(p[2]) {
		return nil, bad
	}
	offset, e := strconv.Atoi(p[3])
	upper := -184
	if p[1] == "2" {
		upper = -20
	}
	if p[1] == "3" {
		upper = -24
	}
	if e != nil || (offset != 24 && offset != upper) {
		return nil, bad
	}
	pitch, e := strconv.Atoi(p[5])
	if e != nil || (p[1] == "1" && (pitch < 4000 || pitch > 10000)) || (p[1] != "1" && pitch != 0) {
		return nil, bad
	}
	version, _ := strconv.Atoi(p[1])
	s := &ScreenCasts{Version: version, ID: p[2], Offset: offset, Pitch: float64(pitch) / 1000}
	seen, members := map[int]bool{}, map[int]bool{}
	for _, row := range strings.Split(p[4], ";") {
		v := strings.Split(row, ",")
		if len(v) != 2 {
			return nil, bad
		}
		slot, e1 := strconv.Atoi(v[0])
		member, e2 := strconv.Atoi(v[1])
		if e1 != nil || e2 != nil || slot < 0 || slot > 4 || member < 1 || member > 5 || seen[slot] || members[member] || strconv.Itoa(slot) != v[0] || strconv.Itoa(member) != v[1] {
			return nil, bad
		}
		seen[slot] = true
		members[member] = true
		s.Members = append(s.Members, ScreenCastMember{Slot: slot, Member: member})
	}
	return s, nil
}

// Pixels is a bounded top-down BGRA screen copy. Packet and text are always
// decoded from the same copy; never associate text with an earlier roster.
type Pixels struct {
	Width, Height int
	BGRA          []byte
}

func (p Pixels) valid() bool { return p.Width > 0 && p.Height > 0 && len(p.BGRA) == p.Width*p.Height*4 }
func (p Pixels) rgb(x, y int) (byte, byte, byte) {
	if x < 0 || y < 0 || x >= p.Width || y >= p.Height || !p.valid() {
		return 0, 0, 0
	}
	i := (y*p.Width + x) * 4
	return p.BGRA[i+2], p.BGRA[i+1], p.BGRA[i]
}
func (p Pixels) Cells(r image.Rectangle) []byte {
	if !p.valid() || !r.In(image.Rect(0, 0, p.Width, p.Height)) {
		return nil
	}
	data := make([]byte, 0, r.Dx()/2*(r.Dy()/2)*3)
	for y := r.Min.Y + 1; y < r.Max.Y; y += 2 {
		for x := r.Min.X + 1; x < r.Max.X; x += 2 {
			red, g, b := p.rgb(x, y)
			data = append(data, red, g, b)
		}
	}
	return data
}

// Only bright neutral ink on a black backing counts. A live calibration row
// supplies the exact font raster at the current WoW scale (no installed OCR).
func (p Pixels) ink(x, y int) bool {
	r, g, b := p.rgb(x, y)
	return min(r, g, b) >= 160 && int(max(r, g, b))-int(min(r, g, b)) < 28
}

type glyph [320]bool

func (p Pixels) glyphs(r image.Rectangle, count int, pitch float64) ([]glyph, bool) {
	if !p.valid() || !r.In(image.Rect(0, 0, p.Width, p.Height)) {
		return nil, false
	}
	var bounds []image.Rectangle
	for i := 0; i < count; i++ {
		left, right := r.Min.X+int(float64(i)*pitch), r.Min.X+int(float64(i+1)*pitch)
		if right > r.Max.X {
			return nil, false
		}
		b := image.Rectangle{Min: image.Pt(right, r.Max.Y), Max: image.Pt(left, r.Min.Y)}
		for x := left; x < right; x++ {
			for y := r.Min.Y; y < r.Max.Y; y++ {
				if p.ink(x, y) {
					b.Min.X = min(b.Min.X, x)
					b.Min.Y = min(b.Min.Y, y)
					b.Max.X = max(b.Max.X, x+1)
					b.Max.Y = max(b.Max.Y, y+1)
				}
			}
		}
		if b.Dx() < 2 || b.Dx() > 10 || b.Dy() < 6 || b.Dy() > 13 {
			return nil, false
		}
		bounds = append(bounds, b)
	}
	if len(bounds) != count {
		return nil, false
	}
	result := make([]glyph, count)
	for i, b := range bounds {
		// Clipping at either field edge is not a valid complete number.
		if b.Min.Y <= r.Min.Y || b.Max.Y >= r.Max.Y {
			return nil, false
		}
		for y := 0; y < 20; y++ {
			for x := 0; x < 16; x++ {
				result[i][y*16+x] = p.ink(b.Min.X+(2*x+1)*b.Dx()/32, b.Min.Y+(2*y+1)*b.Dy()/40)
			}
		}
	}
	return result, true
}
func distance(a, b glyph) float64 {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return float64(n) / float64(len(a))
}
func digits(gs, calibration []glyph) (int, bool) {
	value := 0
	for _, g := range gs {
		best, second, id := 1.0, 1.0, -1
		for i, template := range calibration {
			d := distance(g, template)
			if d < best {
				second, best, id = best, d, i
			} else if d < second {
				second = d
			}
		}
		if id < 0 || best > 0.14 || second-best < 0.045 {
			return 0, false
		}
		value = value*10 + id
	}
	return value, true
}

func DecodeScreenCasts(s *ScreenCasts, p Pixels, y int) {
	s.Captured = false
	s.Casts = nil
	s.Meter = nil
	s.Rejected = 0
	s.Error = ""
	if s.Version == 2 || s.Version == 3 {
		decodeColorCasts(s, p, y)
		if s.Version == 3 && s.Captured {
			decodeMeter(s, p, y)
		}
		return
	}
	calibration, ok := p.glyphs(image.Rect(10, y, 120, y+16), 10, s.Pitch)
	if !ok {
		s.Error = "text_calibration_unreadable"
		return
	}
	// An occluded calibration digit must not silently become another template.
	for i, a := range calibration {
		for j, b := range calibration {
			if i != j && distance(a, b) < 0.08 {
				s.Error = "text_calibration_ambiguous"
				return
			}
		}
	}
	s.Captured = true
	owners := map[int]string{}
	for _, m := range s.Members {
		owners[m.Slot] = m.GUID
	}
	for index := 0; index < 20; index++ {
		guid := owners[index/4]
		if guid == "" {
			continue
		}
		x, yy := index%2*64, y+16+index/2*16
		var h []byte
		for cell := 0; cell < 4; cell++ {
			r, g, b := p.rgb(x+3+cell%2*2, yy+7+cell/2*2)
			h = append(h, r, g, b)
		}
		sum := 0
		for i := 0; i < 9; i++ {
			sum += int(h[i]) * (i + 1)
		}
		if h[0] != 172 || int(h[1]) != index || h[2] != 117 || h[8] != 57 || h[9] != byte(sum%256) || h[10] != 91 || h[11] != 203 {
			s.Rejected++
			continue
		}
		serial := int(h[3])<<16 | int(h[4])<<8 | int(h[5])
		age := int(h[6])<<8 | int(h[7])
		if serial == 0 {
			continue
		}
		if age > 30000 {
			s.Rejected++
			continue
		}
		gs, ok := p.glyphs(image.Rect(x+10, yy, x+64, yy+16), 7, s.Pitch)
		if !ok {
			s.Rejected++
			continue
		}
		spell, ok := digits(gs, calibration)
		if !ok || spell <= 0 || spell > 10000000 {
			s.Rejected++
			continue
		}
		s.Casts = append(s.Casts, LiveCast{ID: fmt.Sprintf("Screen:%s:%d", s.ID, serial), GUID: guid, Spell: spell, AgeMillis: age})
	}
}
