package lookup

import (
	"fmt"
	"hash/crc32"
	"image"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type NativeMeterView struct {
	View, Actor       string
	Offset, TotalRows int
	Primary           bool
	ColorCompact      bool
	Total, Maximum    int64
	Rows              []NativeViewRow
	Choices           []NativeViewChoice
}
type NativeViewChoice struct{ Scope, Label string }
type NativeViewRow struct {
	Key, Name                                            string
	Class, SpellID, RecapID, NativeIndex, Mask, AuraMode int
	Total, Rate                                          int64
	Icon, NameImage                                      *image.NRGBA `json:"-"`
}

func viewInteger(s string, lo, hi int) (int, bool) {
	n, e := strconv.Atoi(s)
	return n, e == nil && n >= lo && n <= hi && strconv.Itoa(n) == s
}
func parseMeterView(body []byte) (BridgeSample, int) {
	bad := BridgeSample{Source: "augmentation", Error: "meter_view_metadata_invalid"}
	lines := strings.Split(string(body), "\n")
	if len(lines) < 1 {
		return bad, 0
	}
	p := strings.Split(lines[0], "|")
	if len(p) != 47 || (p[0] != "8" && p[0] != "9") {
		return bad, 0
	}
	// Reuse all v7 geometry, owner, lease and style validation. This temporary
	// member is discarded; generic rows must pass their own identity checks.
	header := append([]string(nil), p[:40]...)
	header[0] = "7"
	s := parseNativeMeter([]byte(strings.Join(header, "|") + "\n1|" + p[4] + "|0|Player|0"))
	n := s.NativeMeter
	if s.Error != "" || n == nil {
		return bad, 0
	}
	n.Members = nil
	if p[40] != "actors" && p[40] != "spells" && p[40] != "targets" {
		return bad, 0
	}
	if p[41] != "_" && !observationGUID.MatchString(p[41]) {
		return bad, 0
	}
	offset, a := viewInteger(p[42], 0, 10000)
	total, b := viewInteger(p[43], 0, 10000)
	count, c := viewInteger(p[45], 0, 40)
	masks, d := viewInteger(p[46], 0, 40)
	if !a || !b || !c || !d || masks > count || (p[44] != "0" && p[44] != "1") || len(lines) < count+1 || len(lines) > count+27 {
		return bad, 0
	}
	v := &NativeMeterView{ColorCompact: p[0] == "9", View: p[40], Actor: p[41], Offset: offset, TotalRows: total, Primary: p[44] == "1", Rows: []NativeViewRow{}}
	if v.Primary && (v.View != "actors" || n.Metric >= 9 || offset != 0 || total != count) {
		return bad, 0
	}
	seen := map[string]bool{}
	seenMasks := map[int]bool{}
	own := false
	for i, line := range lines[1 : count+1] {
		r := strings.Split(line, "|")
		if len(r) != 9 {
			return bad, 0
		}
		index, a := viewInteger(r[0], i+1, i+1)
		_ = index
		class, b := viewInteger(r[2], 0, 13)
		spell, c := viewInteger(r[4], 0, 100000000)
		recap, d := viewInteger(r[5], 0, 2147483647)
		native, e := viewInteger(r[6], 0, 10000)
		mask, f := viewInteger(r[7], 0, masks)
		mode, g := viewInteger(r[8], 0, 15)
		if !a || !b || !c || !d || !e || !f || !g || len(r[3]) < 1 || len(r[3]) > 160 || !utf8.ValidString(r[3]) || strings.ContainsAny(r[3], "\r\x00") || seen[r[1]] {
			return bad, 0
		}
		if v.Primary {
			if !observationGUID.MatchString(r[1]) || native != 0 || mask != 0 {
				return bad, 0
			}
			own = own || r[1] == n.Player
			n.Members = append(n.Members, NativeMeterMember{Slot: i + 1, GUID: r[1], Class: class, Name: r[3], AuraMode: mode, AuraMask: -1})
		} else if native != offset+i+1 || r[1] != fmt.Sprintf("row:%d", native) || mode != 0 {
			return bad, 0
		}
		if mask > 0 {
			if r[3] != "_" || seenMasks[mask] {
				return bad, 0
			}
			seenMasks[mask] = true
		}
		seen[r[1]] = true
		v.Rows = append(v.Rows, NativeViewRow{Key: r[1], Name: r[3], Class: class, SpellID: spell, RecapID: recap, NativeIndex: native, Mask: mask, AuraMode: mode})
	}
	if v.Primary && !own || len(seenMasks) != masks || offset+count > total {
		return bad, 0
	}
	aliasIDs := map[int]bool{n.Frame.WindowID: true}
	for _, line := range lines[count+1:] {
		if strings.HasPrefix(line, "@") {
			alias := parseMeterAlias(header, line)
			if !v.Primary || alias == nil || len(n.Aliases) >= 7 || aliasIDs[alias.Frame.WindowID] {
				return bad, 0
			}
			aliasIDs[alias.Frame.WindowID] = true
			n.Aliases = append(n.Aliases, alias)
			continue
		}
		scope, label, ok := strings.Cut(strings.TrimPrefix(line, "!"), "|")
		if !strings.HasPrefix(line, "!") || !ok || !nativeMeterScope.MatchString(scope) || len(label) < 1 || len(label) > 100 || !utf8.ValidString(label) || strings.ContainsAny(label, "|\r\x00") {
			return bad, 0
		}
		v.Choices = append(v.Choices, NativeViewChoice{scope, label})
	}
	n.View = v
	return s, masks
}

func planMeterView(bytes, rows int, primary bool, masks, width int) MeterBorderLayout {
	return planMeterViewDensity(bytes, rows, primary, masks, width, 0)
}
func meterViewTile(pitch int) (int, int) {
	if pitch > 0 {
		return 32*pitch + 16, 4
	}
	return 208, 8
}
func planMeterViewDensity(bytes, rows int, primary bool, masks, width, pitch int) MeterBorderLayout {
	tileWidth, rowHeight := meterViewTile(pitch)
	columns := width / tileWidth
	if width < 8 || columns < 1 || masks > 0 && width < 192 {
		return MeterBorderLayout{}
	}
	meta := ((1 + (bytes+35)/36 + width/8 - 1) / (width / 8)) * 8
	count := 4 + rows
	if !primary {
		count += rows
	}
	media := meta + (count+columns-1)/columns*rowHeight
	icons := (rows + width/16 - 1) / (width / 16) * 16
	names := (masks + max(1, width/192) - 1) / max(1, width/192) * 16
	return MeterBorderLayout{Generic: true, Fast: true, InnerWidth: width, Edges: [4]int{((media + icons + names + 7) / 8) * 8, 0, 0, 0}}
}
func meterViewPositions(b MeterBorderLayout, bytes, rows int, primary bool, masks int) ([]image.Point, int, bool) {
	return meterViewPositionsDensity(b, bytes, rows, primary, masks, 0)
}
func meterViewPositionsDensity(b MeterBorderLayout, bytes, rows int, primary bool, masks, pitch int) ([]image.Point, int, bool) {
	tileWidth, rowHeight := meterViewTile(pitch)
	width := b.Width()
	columns := width / tileWidth
	if columns < 1 || masks > 0 && width < 192 {
		return nil, 0, false
	}
	meta := ((1 + (bytes+35)/36 + width/8 - 1) / (width / 8)) * 8
	iconHeight := (rows + width/16 - 1) / (width / 16) * 16
	media := iconHeight + (masks+max(1, width/192)-1)/(max(1, width/192))*16
	count := 4 + rows
	if !primary {
		count += rows
	}
	points := []image.Point{}
	if b.Standalone() {
		for i := 0; i < count; i++ {
			points = append(points, image.Pt(i%columns*tileWidth, meta+i/columns*rowHeight))
		}
		return points, meta + (count+columns-1)/columns*rowHeight, true
	}
	if b.Edges[0] < meta {
		return nil, 0, false
	}
	bottom := b.Edges[0] + b.InnerHeight
	inBottom := b.Edges[2] >= media
	mediaY := b.Edges[0] - media
	topEnd := mediaY
	bottomEnd := b.Height()
	if inBottom {
		mediaY = b.Height() - media
		topEnd = b.Edges[0]
		bottomEnd = mediaY
	}
	for y := meta; y+rowHeight <= topEnd; y += rowHeight {
		for col := 0; col < columns && len(points) < count; col++ {
			points = append(points, image.Pt(col*tileWidth, y))
		}
	}
	for y := bottom; y+rowHeight <= bottomEnd; y += rowHeight {
		for col := 0; col < columns && len(points) < count; col++ {
			points = append(points, image.Pt(col*tileWidth, y))
		}
	}
	return points, mediaY, mediaY >= meta && len(points) == count
}
func viewMediaOrigin(b MeterBorderLayout, bytes, rows int, primary bool) int {
	_, media, _ := meterViewPositions(b, bytes, rows, primary, 0)
	return media
}
func meterViewImage(p Pixels, r image.Rectangle, mask bool) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			red, green, blue := p.rgb(r.Min.X+x, r.Min.Y+y)
			at := out.PixOffset(x, y)
			if mask {
				out.Pix[at], out.Pix[at+1], out.Pix[at+2], out.Pix[at+3] = 255, 255, 255, max(red, green, blue)
			} else {
				out.Pix[at], out.Pix[at+1], out.Pix[at+2], out.Pix[at+3] = red, green, blue, 255
			}
		}
	}
	return out
}
func decodeMeterView(p Pixels, h MeterBorderHeader) BridgeSample {
	bad := BridgeSample{Source: "augmentation", Error: "meter_view_unreadable"}
	slots := h.Layout.Slots()
	meta := (h.BodyBytes + 35) / 36
	if meta+1 > len(slots) {
		return bad
	}
	body := make([]byte, 0, meta*36)
	for i := 1; i <= meta; i++ {
		at := slots[i]
		body = append(body, p.Cells(image.Rect(at.X, at.Y, at.X+6, at.Y+8))...)
	}
	if len(body) < h.BodyBytes {
		return bad
	}
	body = body[:h.BodyBytes]
	if crc32.ChecksumIEEE(body) != h.BodyCRC {
		bad.Error = "meter_view_metadata_checksum"
		return bad
	}
	s, masks := parseMeterView(body)
	n := s.NativeMeter
	if s.Error != "" || n == nil {
		return s
	}
	n.Border = &h.Layout
	n.Frame.Standalone = h.Layout.Standalone()
	n.Error = "meter_view_unreadable"
	v := n.View
	colorPitch := 0
	if v.ColorCompact {
		colorPitch = int(n.Pitch)
		if n.Pitch != float64(colorPitch) || colorPitch < 4 || colorPitch > 6 {
			n.Error = "meter_view_pitch"
			return s
		}
	}
	expected := planMeterViewDensity(len(body), len(v.Rows), v.Primary, masks, p.Width, colorPitch)
	points, media, layoutOK := meterViewPositionsDensity(h.Layout, len(body), len(v.Rows), v.Primary, masks, colorPitch)
	if !layoutOK || h.Layout.Standalone() && expected.Height() != p.Height || n.Serial != h.Serial {
		n.Error = "meter_view_geometry"
		return s
	}
	at := points[0]
	if !v.ColorCompact && n.Pitch != 5 && n.Pitch != 6 {
		n.Error = "meter_view_pitch"
		return s
	}
	cal, ok := compactCalibration(p, at.X+8, at.Y, n.Pitch)
	if n.Pitch == 5 {
		cal, ok = monoDecimalCalibration(p, at.X+8, at.Y)
	}
	if v.ColorCompact {
		cal, ok = meterColorCalibration(p, at.X+6, at.Y, colorPitch)
	}
	if !ok {
		n.Error = "meter_view_calibration"
		return s
	}
	number := func(x, y int) (int64, bool) {
		gs := make([]compactGlyph, 16)
		for i := range gs {
			g, valid := compactGlyphAt(p, x+int(float64(i)*n.Pitch), y, n.Pitch)
			if v.ColorCompact {
				g, valid = meterColorGlyph(p, x+i*colorPitch, y, colorPitch)
			}
			if !valid {
				return 0, false
			}
			gs[i] = g
		}
		if n.Pitch == 5 || v.ColorCompact {
			return monoDecimalValue(gs, cal)
		}
		return compactValue(gs, 10, cal)
	}
	count := 3 + len(v.Rows)
	if !v.Primary {
		count += len(v.Rows)
	}
	values := make([]int64, count)
	for row := range values {
		at := points[row+1]
		raw := p.Cells(image.Rect(at.X, at.Y, at.X+6, at.Y+8))
		if v.ColorCompact {
			raw = p.Cells(image.Rect(at.X, at.Y, at.X+4, at.Y+4))
		}
		if len(raw) != 36 && (!v.ColorCompact || len(raw) != 12) {
			return s
		}
		sum := 0
		for i := 0; i < 7; i++ {
			sum += int(raw[i]) * (i + 1)
		}
		serial := int(raw[3])<<16 | int(raw[4])<<8 | int(raw[5])
		if raw[0] != 173 || int(raw[1]) != row || raw[2] != 118 || serial != h.Serial || raw[6] != 59 || raw[7] != byte(sum%256) || raw[8] != 93 || raw[9] != 205 || raw[10] != 127 || raw[11] != 61 {
			n.Error = "meter_view_row_serial"
			return s
		}
		first, second := 8, 106
		if v.ColorCompact {
			first, second = 6, 8+16*colorPitch
		}
		a, good := number(at.X+first, at.Y)
		b, good2 := number(at.X+second, at.Y)
		if !good || !good2 || a != b {
			n.Error = "meter_view_duplicate_mismatch"
			return s
		}
		values[row] = a
		if v.Primary && row >= 3 && n.Members[row-3].AuraMode == 15 {
			mask := 0
			for bit := 0; bit < 4; bit++ {
				rect := image.Rect(at.X+204, at.Y+bit*2, at.X+208, at.Y+bit*2+2)
				if v.ColorCompact {
					x, y := at.X+32*colorPitch+8+bit%2*4, at.Y+bit/2*2
					rect = image.Rect(x, y, x+4, y+2)
				}
				colors := p.Cells(rect)
				if len(colors) != 6 || colors[0] != colors[3] || colors[1] != colors[4] || colors[2] != colors[5] {
					n.Error = "meter_view_aura_mismatch"
					return s
				}
				rgb := int(colors[0])<<16 | int(colors[1])<<8 | int(colors[2])
				if rgb == 0x18e060 {
					mask |= 1 << bit
				} else if rgb != 0xa02060 {
					n.Error = "meter_view_aura_unreadable"
					return s
				}
			}
			n.Members[row-3].AuraMask = mask
		}
	}
	if n.DurationMillis < 0 {
		n.DurationMillis = values[0] * 1000
	}
	if n.DurationMillis > 604800000 || math.Abs(float64(n.DurationMillis)/1000-float64(values[0])) > .75 {
		n.Error = "meter_view_duration"
		return s
	}
	v.Total, v.Maximum = values[1], values[2]

	nameY := media + (len(v.Rows)+p.Width/16-1)/(p.Width/16)*16
	for i := range v.Rows {
		r := &v.Rows[i]
		idx := 3 + i
		if !v.Primary {
			idx = 3 + i*2
		}
		r.Total = values[idx]
		if v.Primary {
			n.Members[i].Total = r.Total
		} else {
			r.Rate = values[idx+1]
		}
		x, y := i%(p.Width/16)*16, media+i/(p.Width/16)*16
		r.Icon = meterViewImage(p, image.Rect(x, y, x+16, y+16), false)
		if r.Mask > 0 {
			x, y = (r.Mask-1)%max(1, p.Width/192)*192, nameY+(r.Mask-1)/max(1, p.Width/192)*16
			r.NameImage = meterViewImage(p, image.Rect(x, y, x+192, y+16), true)
		}
	}
	if v.Primary {
		var sum int64
		for _, m := range n.Members {
			sum += m.Total
		}
		if sum != v.Total {
			n.Error = "native_roster_incomplete"
			return s
		}
	}
	n.Captured = true
	n.Error = ""
	return s
}
