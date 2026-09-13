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
	if e != nil || flags&2 == 0 || !strings.HasPrefix(f[1], "Player-") {
		return Cast{}, fmt.Errorf("not party player")
	}
	id, e := strconv.Atoi(f[9])
	if e != nil {
		return Cast{}, e
	}
	return Cast{At: at, GUID: f[1], Name: f[2], Spell: id}, nil
}

type Row struct {
	GUID    string  `json:"guid"`
	Player  string  `json:"player"`
	Spell   int     `json:"spell"`
	Name    string  `json:"name"`
	Ends    float64 `json:"ends"`
	Charges bool    `json:"charges"`
	Delay   float64 `json:"delay"`
}
type Model struct {
	Rows map[string]Row
	Seen map[string]time.Time
}

func NewModel() *Model { return &Model{Rows: map[string]Row{}, Seen: map[string]time.Time{}} }
func (m *Model) Observe(c Cast, now time.Time) bool {
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
	m.Rows[key] = Row{c.GUID, c.Name, c.Spell, s.Name, float64(c.At.UnixMilli())/1000 + s.Cooldown, s.Charges, delay}
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
	Enabled bool    `json:"enabled"`
	Updates bool    `json:"updates"`
	Notify  bool    `json:"notify"`
	Days    int     `json:"days"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Scale   float64 `json:"scale"`
}

func DefaultConfig() Config { return Config{true, true, true, 1, 60, 240, 1} }

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
	for k, p := range map[string]*int{"checkDays": &c.Days, "overlayX": &c.X, "overlayY": &c.Y} {
		v, e := strconv.Atoi(field(k))
		if e != nil {
			return c, e
		}
		*p = v
	}
	c.Scale, e = strconv.ParseFloat(field("overlayScale"), 64)
	if e != nil {
		return c, e
	}
	if c.Days != 1 && c.Days != 7 {
		return c, fmt.Errorf("invalid frequency")
	}
	if c.X < 0 || c.Y < 0 || c.X > 16000 || c.Y > 16000 || c.Scale < 0.5 || c.Scale > 3 {
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
