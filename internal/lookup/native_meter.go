package lookup

import (
	"image"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// NativeMeter is decoded from one screenshot containing its CRC-bound public
// roster, calibration, numeric display sinks and duplicate readings.
type MeterShareRequest struct {
	Compact         bool
	EndedAt         int64
	EncounterID     string
	Options         uint8
	Token           string
	Expires         int64
	Channel, Scope  string
	Roster          uint32
	TransferVersion int
	FocusBlocked    bool
	ReceivedBytes   int
}
type MeterAutoScope struct {
	Token, Scope       string
	OnCombat, AfterKey bool
}
type NativeMeter struct {
	Aliases           []*NativeMeter      `json:"-"`
	View              *NativeMeterView    `json:"view,omitempty"`
	NavigationCapable bool                `json:"navigationCapable,omitempty"`
	NavigationFailed  bool                `json:"navigationFailed,omitempty"`
	ShareAck          string              `json:"shareAck,omitempty"`
	ControlHidden     bool                `json:"controlHidden,omitempty"`
	ControlOnly       bool                `json:"controlOnly,omitempty"`
	ControlWidth      int                 `json:"controlWidth,omitempty"`
	ControlHeight     int                 `json:"controlHeight,omitempty"`
	Auto              *MeterAutoScope     `json:"auto,omitempty"`
	ShareCapable      bool                `json:"shareCapable,omitempty"`
	ShareOpenVersion  int                 `json:"shareOpenVersion,omitempty"`
	ShareRequest      *MeterShareRequest  `json:"shareRequest,omitempty"`
	Metric            int                 `json:"metric,omitempty"`
	HeaderOnly        bool                `json:"headerOnly,omitempty"`
	Frame             *NativeMeterFrame   `json:"frame,omitempty"`
	Border            *MeterBorderLayout  `json:"border,omitempty"`
	ID                string              `json:"id"`
	Serial            int                 `json:"serial"`
	SessionID         int                 `json:"sessionID"`
	Player            string              `json:"player"`
	Spec              int                 `json:"spec"`
	Combat            bool                `json:"combat"`
	Pitch             float64             `json:"pitch"`
	Offset            int                 `json:"offset"`
	DurationMillis    int64               `json:"durationMillis"`
	Members           []NativeMeterMember `json:"members"`
	Captured          bool                `json:"captured"`
	Error             string              `json:"error,omitempty"`
}
type NativeMeterFrame struct {
	WindowID     int              `json:"windowID,omitempty"`
	HeaderHeight int              `json:"headerHeight,omitempty"`
	RowHeight    int              `json:"rowHeight,omitempty"`
	FontSize     int              `json:"fontSize,omitempty"`
	Opacity      int              `json:"opacity,omitempty"`
	Values       string           `json:"values,omitempty"`
	ClassColors  bool             `json:"classColors,omitempty"`
	Minimized    bool             `json:"minimized,omitempty"`
	Label        string           `json:"label,omitempty"`
	Standalone   bool             `json:"standalone,omitempty"`
	Menu         *NativeMeterMenu `json:"menu,omitempty"`
	X            int              `json:"x"`
	Y            int              `json:"y"`
	Width        int              `json:"width"`
	Height       int              `json:"height"`
	Scope        string           `json:"scope"`
	Locked       bool             `json:"locked"`
}
type NativeMeterMenu struct{ X, Y, Width, Height int }
type NativeMeterMember struct {
	AuraMode int    `json:"auraMode"`
	AuraMask int    `json:"auraMask"`
	Slot     int    `json:"slot"`
	GUID     string `json:"guid"`
	Class    int    `json:"class"`
	Name     string `json:"name"`
	Total    int64  `json:"total"`
}

func (n *NativeMeter) Height() int { return 16 + (len(n.Members)+1)*16 }
func parseNativeMeter(body []byte) BridgeSample {
	bad := BridgeSample{Source: "augmentation", Error: "native_meter_metadata_invalid"}
	if bytes, ok := strings.CutPrefix(string(body), "0|"); ok {
		if len(bytes) > 0 && len(bytes) <= 160 && utf8.ValidString(bytes) && !strings.ContainsAny(bytes, "|\r\n\x00") {
			return BridgeSample{Source: "augmentation", Error: "native_meter_addon: " + bytes}
		}
		return bad
	}
	lines := strings.Split(string(body), "\n")
	if len(lines) < 2 || len(lines) > 41 {
		return bad
	}
	p := strings.Split(lines[0], "|")
	if !((len(p) == 10 && p[0] == "1") || (len(p) == 16 && p[0] == "2") || (len(p) == 20 && p[0] == "3") || (len(p) == 21 && p[0] == "4") || (len(p) == 27 && p[0] == "5") || (len(p) == 31 && p[0] == "6") || (len(p) == 40 && p[0] == "7")) || !observationID.MatchString(p[1]) || !observationGUID.MatchString(p[4]) {
		return bad
	}
	integer := func(s string, lo, hi int64) (int64, bool) {
		v, e := strconv.ParseInt(s, 10, 64)
		return v, e == nil && v >= lo && v <= hi && strconv.FormatInt(v, 10) == s
	}
	serial, ok := integer(p[2], 1, 16777215)
	if !ok {
		return bad
	}
	session, ok := integer(p[3], 1, 2147483647)
	if !ok {
		return bad
	}
	spec, ok := integer(p[5], 0, 100000)
	if !ok {
		return bad
	}
	if p[6] != "0" && p[6] != "1" {
		return bad
	}
	pitch, ok := integer(p[7], 4000, 7400)
	if !ok {
		return bad
	}
	offset, ok := integer(p[8], -1024, 32)
	if !ok {
		return bad
	}
	duration, ok := integer(p[9], -1, 604800000)
	if !ok {
		return bad
	}
	n := &NativeMeter{ID: p[1], Serial: int(serial), SessionID: int(session), Player: p[4], Spec: int(spec), Combat: p[6] == "1", Pitch: float64(pitch) / 1000, Offset: int(offset), DurationMillis: duration}
	if p[0] != "1" {
		x, a := integer(p[10], 0, 16000)
		y, b := integer(p[11], 0, 16000)
		minWidth, minHeight := int64(300), int64(120)
		maxWidth, maxHeight := int64(1000), int64(1000)
		if p[0] == "7" {
			minWidth, minHeight = 128, 32
			maxWidth, maxHeight = 2000, 2000
		}
		w, c := integer(p[12], minWidth, maxWidth)
		h, d := integer(p[13], minHeight, maxHeight)
		if !a || !b || !c || !d || len(p[14]) > 110 || (p[15] != "0" && p[15] != "1") || !nativeMeterScope.MatchString(p[14]) {
			return bad
		}
		n.Frame = &NativeMeterFrame{X: int(x), Y: int(y), Width: int(w), Height: int(h), Scope: p[14], Locked: p[15] == "1"}
		if p[0] != "2" {
			mx, a := integer(p[16], 0, 16000)
			my, b := integer(p[17], 0, 16000)
			mw, c := integer(p[18], 0, 4096)
			mh, d := integer(p[19], 0, 4096)
			if !a || !b || !c || !d || (mw == 0) != (mh == 0) {
				return bad
			}
			if mw > 0 {
				n.Frame.Menu = &NativeMeterMenu{X: int(mx), Y: int(my), Width: int(mw), Height: int(mh)}
			}
		}
		if p[0] != "2" && p[0] != "3" {
			metric, ok := integer(p[20], 0, 10)
			if !ok || (p[0] != "7" && metric != 0 && metric != 2) {
				return bad
			}
			n.Metric = int(metric)
		}
	}
	if p[0] == "5" || p[0] == "6" || p[0] == "7" {
		if p[21] != "0" && p[21] != "1" && p[21] != "2" && !(p[0] == "7" && (p[21] == "3" || p[21] == "4")) {
			return bad
		}
		n.ShareCapable = p[21] != "0" && p[21] != "4"
		if p[21] == "2" || p[21] == "3" {
			n.ShareOpenVersion = 2
		}
		n.NavigationCapable = p[21] == "3" || p[21] == "4"
		expires, a := integer(p[23], 0, 4102444800)
		roster, b := integer(p[26], 0, 4294967295)
		if !a || !b {
			return bad
		}
		if p[22] != "-" {
			if !reloadToken.MatchString(p[22]) || expires == 0 || (p[25] != "key" && p[25] != "boss" && p[25] != "boss-uptime" && !(p[0] == "7" && (meterWindowScope.MatchString(p[25]) || meterHistoryScope.MatchString(p[25])))) || (p[24] != "PARTY" && p[24] != "RAID" && p[24] != "INSTANCE_CHAT") {
				return bad
			}
			n.ShareRequest = &MeterShareRequest{Token: p[22], Expires: expires, Channel: p[24], Scope: p[25], Roster: uint32(roster)}
		} else if expires != 0 || roster != 0 || p[24] != "-" || p[25] != "-" {
			return bad
		}
	}
	if p[0] == "6" || p[0] == "7" {
		if (p[29] != "0" && p[29] != "1") || (p[30] != "0" && p[30] != "1") {
			return bad
		}
		n.Auto = &MeterAutoScope{OnCombat: p[29] == "1", AfterKey: p[30] == "1"}
		if p[27] != "-" {
			if !observationID.MatchString(p[27]) || (p[28] != "boss" && p[28] != "overall") {
				return bad
			}
			n.Auto.Token = p[27]
			n.Auto.Scope = p[28]
		} else if p[28] != "-" {
			return bad
		}
	}
	if p[0] == "7" {
		id, a := integer(p[31], 1, 8)
		header, b := integer(p[32], 24, 106)
		row, c := integer(p[33], 8, 72)
		font, d := integer(p[34], 6, 40)
		opacity, e := integer(p[35], 200, 1000)
		if !a || !b || !c || !d || !e || (p[36] != "both" && p[36] != "total" && p[36] != "rate" && p[36] != "percent") || (p[37] != "0" && p[37] != "1") || (p[38] != "0" && p[38] != "1") || len(p[39]) < 1 || len(p[39]) > 100 || !utf8.ValidString(p[39]) {
			return bad
		}
		n.Frame.WindowID = int(id)
		n.Frame.HeaderHeight = int(header)
		n.Frame.RowHeight = int(row)
		n.Frame.FontSize = int(font)
		n.Frame.Opacity = int(opacity)
		n.Frame.Values = p[36]
		n.Frame.ClassColors = p[37] == "1"
		n.Frame.Minimized = p[38] == "1"
		n.Frame.Label = p[39]
	}
	seen := map[string]bool{}
	own := false
	for i, line := range lines[1:] {
		v := strings.Split(line, "|")
		if len(v) != 5 || !observationGUID.MatchString(v[1]) || seen[v[1]] {
			return bad
		}
		slot, ok := integer(v[0], int64(i+1), int64(i+1))
		if !ok {
			return bad
		}
		class, ok := integer(v[2], 0, 13)
		if !ok {
			return bad
		}
		mode, ok := integer(v[4], 0, 15)
		if !ok {
			return bad
		}
		if len(v[3]) < 1 || len(v[3]) > 160 || !utf8.ValidString(v[3]) || strings.ContainsAny(v[3], "\x00\r\n") {
			return bad
		}
		seen[v[1]] = true
		own = own || v[1] == n.Player
		n.Members = append(n.Members, NativeMeterMember{Slot: int(slot), GUID: v[1], Class: int(class), Name: v[3], AuraMode: int(mode), AuraMask: -1})
	}
	if !own || (n.Offset != 32 && n.Offset != -(n.Height()+8)) {
		return bad
	}
	return BridgeSample{Source: "augmentation", NativeMeter: n}
}

func nativeNumber(p Pixels, x, y int, pitch float64, templates []glyph) (int64, bool) {
	gs, ok := p.glyphs(image.Rect(x, y, x+116, y+16), 16, pitch)
	if !ok {
		return 0, false
	}
	marker := distance(gs[15], templates[10])
	if marker > .10 {
		return 0, false
	}
	for _, t := range templates[:10] {
		if distance(gs[15], t)-marker < .045 {
			return 0, false
		}
	}
	// The terminal X rejects overlong, negative, fractional or clipped values;
	// fifteen decimal places avoid the old interrupt color field's 24-bit cap.
	var result int64
	for _, g := range gs[:15] {
		best, second, id := 1.0, 1.0, -1
		for digit, t := range templates[:10] {
			d := distance(g, t)
			if d < best {
				second, best, id = best, d, digit
			} else if d < second {
				second = d
			}
		}
		if id < 0 || best > 0.14 || second-best < 0.045 {
			return 0, false
		}
		result = result*10 + int64(id)
	}
	return result, result <= 999999999999999
}
func DecodeNativeMeter(n *NativeMeter, p Pixels, y int) {
	n.Captured = false
	n.Error = "native_meter_unreadable"
	calibration, ok := p.glyphs(image.Rect(12, y, 128, y+16), 11, n.Pitch)
	if !ok {
		return
	}
	for i, a := range calibration {
		for j, b := range calibration {
			if i != j && distance(a, b) < 0.08 {
				n.Error = "native_meter_calibration_ambiguous"
				return
			}
		}
	}
	values := make([]int64, len(n.Members)+1)
	for index := range values {
		yy := y + 16 + index*16
		var h []byte
		for cell := 0; cell < 4; cell++ {
			r, g, b := p.rgb(3+cell%2*2, yy+7+cell/2*2)
			h = append(h, r, g, b)
		}
		sum := 0
		for i := 0; i < 7; i++ {
			sum += int(h[i]) * (i + 1)
		}
		serial := int(h[3])<<16 | int(h[4])<<8 | int(h[5])
		if h[0] != 173 || int(h[1]) != index || h[2] != 118 || serial != n.Serial || h[6] != 59 || h[7] != byte(sum%256) || h[8] != 93 || h[9] != 205 || h[10] != 127 || h[11] != 61 {
			n.Error = "native_meter_header_mismatch"
			return
		}
		first, good := nativeNumber(p, 12, yy, n.Pitch, calibration)
		second, copyOK := nativeNumber(p, 140, yy, n.Pitch, calibration)
		if !good || !copyOK || first != second {
			n.Error = "native_meter_duplicate_mismatch"
			return
		}
		values[index] = first
	}
	if n.DurationMillis < 0 {
		n.DurationMillis = values[0] * 1000
	}
	if n.DurationMillis > 604800000 || math.Abs(float64(n.DurationMillis)/1000-float64(values[0])) > .75 {
		n.Error = "native_meter_duration_mismatch"
		return
	}
	for i := range n.Members {
		n.Members[i].Total = values[i+1]
		m := &n.Members[i]
		if m.AuraMode == 15 {
			mask := 0
			yy := y + 16 + (i+1)*16
			for bit := 0; bit < 4; bit++ {
				first := [3]byte{}
				second := [3]byte{}
				first[0], first[1], first[2] = p.rgb(129, yy+4+bit*2)
				second[0], second[1], second[2] = p.rgb(135, yy+4+bit*2)
				if first != second {
					n.Error = "native_aura_duplicate_mismatch"
					return
				}
				if first == ([3]byte{24, 224, 96}) {
					mask |= 1 << bit
				} else if first != ([3]byte{160, 32, 96}) {
					n.Error = "native_aura_unreadable"
					return
				}
			}
			m.AuraMask = mask
		}
	}
	n.Captured = true
	n.Error = ""
}
func nativeMeterMappingEqual(a, b *NativeMeter) bool {
	if a == nil || b == nil || a.ID != b.ID || a.Serial != b.Serial || a.SessionID != b.SessionID || a.Player != b.Player || a.Spec != b.Spec || a.Offset != b.Offset || a.DurationMillis != b.DurationMillis || a.Pitch != b.Pitch || a.Combat != b.Combat || len(a.Members) != len(b.Members) {
		return false
	}
	for i := range a.Members {
		if a.Members[i] != b.Members[i] {
			return false
		}
	}
	return true
}
