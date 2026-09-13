package engine

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

func Parse(line string) (Cast, error) {
	m := stamp.FindStringSubmatch(line)
	if m == nil {
		return Cast{}, fmt.Errorf("timestamp")
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
		loc = time.FixedZone("log", sign*(h*3600+mins*60))
	}
	at, e := time.ParseInLocation("1/2/2006 15:04:05.999", m[1], loc)
	if e != nil {
		return Cast{}, e
	}
	r := csv.NewReader(strings.NewReader(m[3]))
	r.FieldsPerRecord = -1
	f, e := r.Read()
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
	if strings.HasPrefix(f[1], "Player-") && flags&2 != 0 {
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
func (m *Model) Observe(c Cast, now time.Time) bool {
	if c.Companion {
		return m.observeCompanion(c, now)
	}
	if c.Spell == 235219 {
		key := fmt.Sprintf("%s/%d", c.GUID, 45438)
		delete(m.Rows, key)
		delete(m.Seen, key)
		return false
	}
	s, ok := Catalog[c.Spell]
	if !ok {
		return false
	}
	delay := now.Sub(c.At).Seconds()
	if delay < -2 || delay > s.Cooldown {
		return false
	}
	key := fmt.Sprintf("%s/%d", c.GUID, c.Spell)
	if prev, ok := m.Seen[key]; ok && !c.At.After(prev) {
		return false
	}
	m.Seen[key] = c.At
	m.Rows[key] = Row{GUID: c.GUID, Player: c.Name, Spell: c.Spell, Name: s.Name, Ends: float64(c.At.UnixMilli())/1000 + s.Cooldown, Charges: s.Charges, Delay: delay, Duration: s.Cooldown}
	return true
}
func (m *Model) Active(now time.Time) []Row {
	rows := []Row{}
	for k, r := range m.Rows {
		if r.Ends <= float64(now.UnixMilli())/1000 {
			delete(m.Rows, k)
			delete(m.Seen, k)
		} else {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Player == rows[j].Player {
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
	b, e := os.ReadFile(path)
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
	text := string(block[1])
	field := func(k string) string {
		r := regexp.MustCompile(`\["` + k + `"\]\s*=\s*(true|false|[-0-9.]+)\s*[,\n]`)
		m := r.FindStringSubmatch(text)
		if m == nil {
			return ""
		}
		return m[1]
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
	if c.X < -16000 || c.Y < -16000 || c.X > 16000 || c.Y > 16000 || c.Scale < 0.5 || c.Scale > 3 {
		return c, fmt.Errorf("invalid layout")
	}
	return c, nil
}

// Tail handles rotation, truncation and partial lines. Start at EOF for live use.
type Tail struct {
	Path    string
	Offset  int64
	Pending string
	Info    os.FileInfo
}

func (t *Tail) Poll(dir string) ([]string, error) {
	files, e := filepath.Glob(filepath.Join(dir, "WoWCombatLog*.txt"))
	if e != nil {
		return nil, e
	}
	var newest string
	var info os.FileInfo
	for _, p := range files {
		s, e := os.Stat(p)
		if e == nil && (info == nil || s.ModTime().After(info.ModTime())) {
			newest = p
			info = s
		}
	}
	if info == nil {
		return nil, nil
	}
	if newest != t.Path || t.Info != nil && !os.SameFile(t.Info, info) {
		first := t.Path == ""
		t.Path = newest
		t.Offset = 0
		t.Pending = ""
		if first {
			t.Offset = info.Size()
		}
		t.Info = info
	}
	if info.Size() < t.Offset {
		t.Offset = 0
		t.Pending = ""
	}
	f, e := os.Open(t.Path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	if _, e = f.Seek(t.Offset, 0); e != nil {
		return nil, e
	}
	b, e := io.ReadAll(io.LimitReader(f, 4<<20))
	if e != nil {
		return nil, e
	}
	t.Offset += int64(len(b))
	chunks := strings.Split(t.Pending+string(b), "\n")
	t.Pending = chunks[len(chunks)-1]
	if len(t.Pending) > 1<<20 {
		t.Pending = ""
	}
	return chunks[:len(chunks)-1], nil
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
