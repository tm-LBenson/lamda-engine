package engine

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Spell struct {
	Name     string  `json:"name"`
	Cooldown float64 `json:"cooldown"`
	Charges  bool    `json:"charges"`
}
type Cast struct {
	At         time.Time
	GUID, Name string
	Spell      int
	Companion  bool
}

var stamp = regexp.MustCompile(`^(\d+/\d+/\d+ \d+:\d+:\d+\.\d+)([+-]\d{1,2}(?::\d{2})?)?\s+(.+)$`)

func parseRecord(line string) (time.Time, []string, error) {
	m := stamp.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, nil, fmt.Errorf("timestamp")
	}
	loc := time.Local
	if m[2] != "" {
		s := m[2]
		sign := 1
		if s[0] == '-' {
			sign = -1
		}
		parts := strings.Split(s[1:], ":")
		h, _ := strconv.Atoi(parts[0])
		mins := 0
		if len(parts) > 1 {
			mins, _ = strconv.Atoi(parts[1])
		}
		if h > 23 || mins > 59 {
			return time.Time{}, nil, fmt.Errorf("timezone")
		}
		loc = time.FixedZone("log", sign*(h*3600+mins*60))
	}
	at, e := time.ParseInLocation("1/2/2006 15:04:05.999", m[1], loc)
	if e != nil {
		return time.Time{}, nil, e
	}
	r := csv.NewReader(strings.NewReader(m[3]))
	r.FieldsPerRecord = -1
	f, e := r.Read()
	return at, f, e
}

func Parse(line string) (Cast, error) {
	at, f, e := parseRecord(line)
	if e != nil || len(f) < 12 {
		return Cast{}, fmt.Errorf("fields")
	}
	if f[0] != "SPELL_CAST_SUCCESS" {
		return Cast{}, fmt.Errorf("not cast")
	}
	flags, e := strconv.ParseUint(f[3], 0, 64)
	if e != nil {
		return Cast{}, e
	}
	id, e := strconv.Atoi(f[9])
	if e != nil {
		return Cast{}, e
	}
	// Party and raid teammates are eligible; self, outsiders and hostile units are not.
	if strings.HasPrefix(f[1], "Player-") && flags&6 != 0 && flags&1 == 0 && flags&0x10 != 0 && flags&0x40 == 0 {
		return Cast{At: at, GUID: f[1], Name: f[2], Spell: id}, nil
	}
	if flags&3 != 0 && flags&0x10 != 0 && flags&0x40 == 0 {
		if _, ok := CompanionSpell(f[1], id); ok {
			return Cast{At: at, GUID: f[1], Name: f[2], Spell: id, Companion: true}, nil
		}
	}
	return Cast{}, fmt.Errorf("not a supported party member or companion")
}

type Row struct {
	GUID         string  `json:"guid"`
	Player       string  `json:"player"`
	Spell        int     `json:"spell"`
	Name         string  `json:"name"`
	Ends         float64 `json:"ends"`
	Charges      bool    `json:"charges"`
	Delay        float64 `json:"delay"`
	Companion    bool    `json:"companion"`
	ObservedOnly bool    `json:"observedOnly"`
	Duration     float64 `json:"duration"`
}
type Model struct {
	Rows             map[string]Row
	Seen             map[string]time.Time
	CompanionHistory map[string]companionHistory
}

func NewModel() *Model {
	return &Model{Rows: map[string]Row{}, Seen: map[string]time.Time{}, CompanionHistory: map[string]companionHistory{}}
}

const maxPlayerObservations = 4096
const observationRetention = 20 * time.Minute

func (m *Model) remember(key string, at time.Time) {
	if _, exists := m.Seen[key]; !exists && len(m.Seen) >= maxPlayerObservations {
		var oldest string
		var oldestAt time.Time
		for k, v := range m.Seen {
			if oldestAt.IsZero() || v.Before(oldestAt) {
				oldest, oldestAt = k, v
			}
		}
		delete(m.Seen, oldest)
		delete(m.Rows, oldest)
	}
	m.Seen[key] = at
}

func (m *Model) Observe(c Cast, now time.Time) bool {
	if c.Companion {
		return m.observeCompanion(c, now)
	}
	if c.Spell == 235219 {
		// A reset only supersedes earlier observations. Retain its timestamp so
		// delayed or duplicate pre-reset Ice Blocks cannot reappear afterward.
		delay := now.Sub(c.At)
		if delay < -2*time.Second || delay >= 10*time.Minute {
			return false
		}
		key := fmt.Sprintf("%s/%d", c.GUID, 45438)
		if prev, ok := m.Seen[key]; ok && !c.At.After(prev) {
			return false
		}
		delete(m.Rows, key)
		m.remember(key, c.At)
		return false
	}
	s, ok := Catalog[c.Spell]
	if !ok {
		return false
	}
	delay := now.Sub(c.At).Seconds()
	if delay < -2 || delay >= s.Cooldown {
		return false
	}
	key := fmt.Sprintf("%s/%d", c.GUID, c.Spell)
	if prev, ok := m.Seen[key]; ok && !c.At.After(prev) {
		return false
	}
	m.remember(key, c.At)
	m.Rows[key] = Row{GUID: c.GUID, Player: c.Name, Spell: c.Spell, Name: s.Name, Ends: float64(c.At.UnixMilli())/1000 + s.Cooldown, Charges: s.Charges, Delay: delay, Duration: s.Cooldown}
	return true
}
func (m *Model) Active(now time.Time) []Row {
	rows := []Row{}
	for k, r := range m.Rows {
		if r.Ends <= float64(now.UnixMilli())/1000 {
			delete(m.Rows, k)
		} else {
			rows = append(rows, r)
		}
	}
	for k, at := range m.Seen {
		if now.Sub(at) > observationRetention {
			delete(m.Seen, k)
		}
	}
	for k, history := range m.CompanionHistory {
		if now.Sub(history.Last) > observationRetention {
			delete(m.CompanionHistory, k)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Player == rows[j].Player {
			if rows[i].Spell == rows[j].Spell {
				return rows[i].GUID < rows[j].GUID
			}
			return rows[i].Spell < rows[j].Spell
		}
		return rows[i].Player < rows[j].Player
	})
	return rows
}

type Config struct {
	Anchor     int     `json:"anchor"`
	Grow       int     `json:"grow"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Gap        int     `json:"gap"`
	FontSize   int     `json:"fontSize"`
	Opacity    int     `json:"opacity"`
	Columns    int     `json:"columns"`
	MaxRows    int     `json:"maxRows"`
	Accent     int     `json:"accent"`
	ShowNames  bool    `json:"showNames"`
	ShowSpells bool    `json:"showSpells"`
	ShowTimers bool    `json:"showTimers"`
	Border     bool    `json:"border"`
	Bars       bool    `json:"bars"`
	Enabled    bool    `json:"enabled"`
	Companions bool    `json:"companions"`
	Preview    bool    `json:"preview"`
	Updates    bool    `json:"updates"`
	Notify     bool    `json:"notify"`
	Days       int     `json:"days"`
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Scale      float64 `json:"scale"`
}

func DefaultConfig() Config {
	return Config{Anchor: 1, Grow: 1, Width: 340, Height: 34, Gap: 3, FontSize: 14, Opacity: 95, Columns: 1, MaxRows: 12, Accent: 1, ShowNames: true, ShowSpells: true, ShowTimers: true, Border: true, Bars: true, Enabled: true, Companions: true, Updates: true, Notify: true, Days: 1, X: 60, Y: 240, Scale: 1}
}

// Read only a narrow, flat SavedVariables schema. Never execute addon Lua.
func ReadConfig(path string) (Config, error) {
	c := DefaultConfig()
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if e != nil {
		return c, e
	}
	if len(b) > 16<<20 {
		return c, fmt.Errorf("config too large")
	}
	block := regexp.MustCompile(`(?s)(?:^|\n)LamdaEngineDB\s*=\s*\{([^{}]*)\}`).FindSubmatch(b)
	if block == nil {
		return c, fmt.Errorf("missing or incomplete engine settings")
	}
	values := map[string]string{}
	for _, match := range regexp.MustCompile(`\["([^"\r\n]+)"\]\s*=\s*([^,\r\n]*)`).FindAllStringSubmatch(string(block[1]), -1) {
		key, value := match[1], strings.TrimSpace(match[2])
		if _, exists := values[key]; exists || value == "" {
			return c, fmt.Errorf("duplicate or empty setting %s", key)
		}
		values[key] = value
	}
	field := func(k string) string {
		return values[k]
	}
	if field("schema") != "1" {
		return c, fmt.Errorf("unsupported schema")
	}
	for k, p := range map[string]*bool{"cdEnabled": &c.Enabled, "checkUpdates": &c.Updates, "notifyUpdates": &c.Notify} {
		v := field(k)
		if v != "true" && v != "false" {
			return c, fmt.Errorf("missing %s", k)
		}
		*p = v == "true"
	}
	for k, p := range map[string]*bool{"companions": &c.Companions, "preview": &c.Preview, "showNames": &c.ShowNames, "showSpells": &c.ShowSpells, "showTimers": &c.ShowTimers, "border": &c.Border, "bars": &c.Bars} {
		v := field(k)
		if v != "" {
			if v != "true" && v != "false" {
				return c, fmt.Errorf("invalid %s", k)
			}
			*p = v == "true"
		}
	}
	for k, p := range map[string]*int{"checkDays": &c.Days, "overlayX": &c.X, "overlayY": &c.Y} {
		v, e := strconv.Atoi(field(k))
		if e != nil {
			return c, e
		}
		*p = v
	}
	for k, spec := range map[string]struct {
		p      *int
		lo, hi int
	}{
		"anchor": {&c.Anchor, 1, 9}, "grow": {&c.Grow, 1, 2}, "rowWidth": {&c.Width, 120, 900}, "rowHeight": {&c.Height, 20, 100}, "rowGap": {&c.Gap, 0, 40}, "fontSize": {&c.FontSize, 8, 32}, "opacity": {&c.Opacity, 20, 100}, "columns": {&c.Columns, 1, 4}, "maxRows": {&c.MaxRows, 1, 40}, "accent": {&c.Accent, 1, 4},
	} {
		v := field(k)
		if v != "" {
			n, e := strconv.Atoi(v)
			if e != nil || n < spec.lo || n > spec.hi {
				return c, fmt.Errorf("invalid %s", k)
			}
			*spec.p = n
		}
	}
	c.Scale, e = strconv.ParseFloat(field("overlayScale"), 64)
	if e != nil {
		return c, e
	}
	if c.Days != 1 && c.Days != 7 {
		return c, fmt.Errorf("invalid frequency")
	}
	if c.X < -16000 || c.Y < -16000 || c.X > 16000 || c.Y > 16000 || c.Scale < 0.5 || c.Scale > 3 || math.IsNaN(c.Scale) || math.IsInf(c.Scale, 0) {
		return c, fmt.Errorf("invalid layout")
	}
	return c, nil
}

// Tail handles rotation, truncation and partial lines. Start at EOF for live use.
type Tail struct {
	Path        string
	Offset      int64
	Pending     string
	Info        os.FileInfo
	Generation  uint64
	StartOffset int64 // Existing history skipped on initial attachment; zero for a fresh session.
	initialized bool
	checkpoint  []byte
	discardLine bool
}

func (t *Tail) Poll(dir string) ([]string, error) {
	files, e := filepath.Glob(filepath.Join(dir, "WoWCombatLog*.txt"))
	if e != nil {
		return nil, e
	}
	first := !t.initialized
	t.initialized = true
	var newest string
	var info os.FileInfo
	for _, p := range files {
		s, e := os.Stat(p)
		if e == nil && s.Mode().IsRegular() && (info == nil || s.ModTime().After(info.ModTime())) {
			newest = p
			info = s
		}
	}
	if info == nil {
		if t.Path != "" {
			t.Generation++
			t.Path, t.Pending, t.Offset, t.Info = "", "", 0, nil
			t.StartOffset = 0
			t.checkpoint, t.discardLine = nil, false
		}
		return nil, nil
	}
	if newest != t.Path || t.Info != nil && !os.SameFile(t.Info, info) {
		t.Generation++
		t.Path = newest
		t.Offset = 0
		t.StartOffset = 0
		t.Pending = ""
		t.checkpoint, t.discardLine = nil, false
		if first {
			t.Offset = info.Size()
			t.StartOffset = t.Offset
		}
		t.Info = info
	}
	f, e := os.Open(t.Path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	rewritten := info.Size() < t.Offset
	if !rewritten && len(t.checkpoint) > 0 {
		previous := make([]byte, len(t.checkpoint))
		n, err := f.ReadAt(previous, t.Offset-int64(len(previous)))
		rewritten = err != nil || n != len(previous) || !bytes.Equal(previous, t.checkpoint)
	}
	if rewritten {
		t.Generation++
		t.Offset, t.Pending = 0, ""
		t.StartOffset = 0
		t.checkpoint, t.discardLine = nil, false
	}
	if first && t.Offset > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, t.Offset-1); err == nil {
			t.discardLine = last[0] != '\n'
		}
	}
	if _, e = f.Seek(t.Offset, 0); e != nil {
		return nil, e
	}
	b, e := io.ReadAll(io.LimitReader(f, 4<<20))
	if e != nil {
		return nil, e
	}
	t.Offset += int64(len(b))
	t.Info = info
	checkpointSize := min(t.Offset, 128)
	t.checkpoint = make([]byte, checkpointSize)
	if _, e := f.ReadAt(t.checkpoint, t.Offset-checkpointSize); e != nil {
		t.checkpoint = nil
	}
	chunks := strings.Split(t.Pending+string(b), "\n")
	t.Pending = chunks[len(chunks)-1]
	lines := chunks[:len(chunks)-1]
	if t.discardLine && len(lines) > 0 {
		lines = lines[1:]
		t.discardLine = false
	}
	if len(t.Pending) > 1<<20 {
		t.Pending = ""
		t.discardLine = true
	}
	complete := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) <= 1<<20 {
			complete = append(complete, line)
		}
	}
	return complete, nil
}
func WriteJSON(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
