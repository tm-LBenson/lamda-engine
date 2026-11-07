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

	"github.com/tm-LBenson/lamda-engine/internal/atomicfile"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type Spell struct {
	Name     string  `json:"name"`
	Cooldown float64 `json:"cooldown"`
	Charges  bool    `json:"charges"`
}
type Cast struct {
	At            time.Time
	GUID, Name    string
	Spell         int
	Companion     bool
	Self          bool
	RosterMatched bool
	Source        string
	// Public packets and native screen capture can deliver the same cast.
	LivePath string
}

var stamp = regexp.MustCompile(`^(\d+/\d+/\d+ \d+:\d+:\d+\.\d+)([+-]\d{1,2}(?::\d{2})?)?\s+(.+)$`)

// RecordFields parses a bounded combat-log record without applying friendly
// affiliation rules. Consumers must establish their own exact actor identity.
func RecordFields(line string) (time.Time, []string, error) {
	if len(line) > 65536 {
		return time.Time{}, nil, fmt.Errorf("record too large")
	}
	return parseRecord(line)
}
func parseRecord(line string) (time.Time, []string, error) {
	m := stamp.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, nil, fmt.Errorf("timestamp")
	}
	// Modern WoW writes exactly three millisecond digits, then an unsigned
	// positive UTC offset. .9471 means .947 at UTC+1, not 947.1 ms locally.
	if m[2] == "" {
		dot := strings.LastIndexByte(m[1], '.')
		if digits := len(m[1]) - dot - 1; digits > 3 {
			if digits > 5 {
				return time.Time{}, nil, fmt.Errorf("timestamp precision")
			}
			m[2] = "+" + m[1][dot+4:]
			m[1] = m[1][:dot+4]
		}
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
	return parseCast(line, nil)
}

// roster contains only GUIDs from a validated, fresh in-game party snapshot.
// A public talent profile or a character name cannot establish membership.
func parseCast(line string, roster map[string]bool) (Cast, error) {
	return parsePartySpell(line, roster, "SPELL_CAST_SUCCESS")
}

func parsePartySpell(line string, roster map[string]bool, event string) (Cast, error) {
	at, f, e := parseRecord(line)
	if e != nil || len(f) < 12 {
		return Cast{}, fmt.Errorf("fields")
	}
	if f[0] != event {
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
	// Without a confirmed roster, require exactly one friendly affiliation:
	// self, party or raid. Ambiguous flags alone cannot establish membership.
	affiliation := flags & 0xf
	if strings.HasPrefix(f[1], "Player-") && (affiliation == 1 || affiliation == 2 || affiliation == 4) && flags&0xf0 == 0x10 {
		return Cast{At: at, GUID: f[1], Name: f[2], Spell: id, Self: affiliation == 1}, nil
	}
	// The live party roster can disagree with affiliation/reaction flags in
	// outdoor logs. Match the exact player GUID before accepting those casts.
	if strings.HasPrefix(f[1], "Player-") && roster[f[1]] {
		return Cast{At: at, GUID: f[1], Name: f[2], Spell: id, RosterMatched: true}, nil
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
	DisabledDefensives, DisabledInterrupts bool
	Rows                                   map[string]Row
	Seen                                   map[string]time.Time
	CompanionHistory                       map[string]companionHistory
	Evidence                               map[string]CooldownEvidence
	Builds                                 map[string]BuildSnapshot
	Diagnostics                            Diagnostics
	TrackingAudit                          func(TrackingEvent)
	LogVersion                             int
	GameBuild                              string
	PublicBuilds                           map[string]lookup.Profile
	InspectedBuilds                        map[string]BuildSnapshot
	FreshKey                               *FreshKey
	lastFreshKeyAt                         time.Time
	partyMembers                           map[string]time.Time
	partyBuilds                            map[string]partyBuild
	interruptCounts                        interruptCounts
	context                                partyContext
}

func NewModel() *Model {
	return &Model{Rows: map[string]Row{}, Seen: map[string]time.Time{}, CompanionHistory: map[string]companionHistory{}, Evidence: map[string]CooldownEvidence{}, Builds: map[string]BuildSnapshot{}, InspectedBuilds: map[string]BuildSnapshot{}, PublicBuilds: map[string]lookup.Profile{}, Diagnostics: Diagnostics{Gaps: map[string]Gap{}}}
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
		delete(m.Evidence, oldest)
	}
	m.Seen[key] = at
}

func (m *Model) Observe(c Cast, now time.Time) (applied bool) {
	if m.TrackingAudit != nil && !c.Companion {
		if _, supported := trackedSpell(c.Spell); supported {
			defer func() {
				e := m.trackingEvent("cooldown_observation", c, now)
				e.Applied = applied
				if evidence, ok := m.Evidence[fmt.Sprintf("%s/%d", c.GUID, CooldownSpell(c.Spell))]; ok {
					e.Evidence = &evidence
				}
				m.TrackingAudit(e)
			}()
		}
	}
	if (IsInterrupt(c.Spell) && m.DisabledInterrupts) || (!IsInterrupt(c.Spell) && m.DisabledDefensives) {
		return false
	}
	if c.Companion {
		return m.observeCompanion(c, now)
	}
	m.countInterruptUse(c, now, false)
	return m.observePlayer(c, now)
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
		if now.Sub(at) > observationRetention && !m.keepKeyEvidence(m.Evidence[k], now) && !m.keepPartyEvidence(m.Evidence[k], now) {
			delete(m.Seen, k)
			delete(m.Evidence, k)
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
	ScanTiming          int     `json:"scanTiming"`
	ScanMinutes         int     `json:"scanMinutes"`
	ResourceAdvanced    bool    `json:"resourceAdvanced"`
	ResourceLow         bool    `json:"resourceLow"`
	ResourceLookups     bool    `json:"resourceLookups"`
	ResourceFastIdle    bool    `json:"resourceFastIdle"`
	ResourceSmooth      bool    `json:"resourceSmooth"`
	ColorStripVersion   int     `json:"colorStripVersion"`
	KickBridgeX         int     `json:"kickBridgeX"`
	KickBridgeY         int     `json:"kickBridgeY"`
	GladiusBridgeX      int     `json:"gladiusBridgeX"`
	GladiusBridgeY      int     `json:"gladiusBridgeY"`
	OverlaysEnabled     bool    `json:"overlaysEnabled"`
	KBBEnabled          bool    `json:"kbbEnabled"`
	AugLiveHybrid       bool    `json:"augLiveHybrid"`
	MeterOverlay        bool    `json:"meterOverlay"`
	AugEnabled          bool    `json:"augEnabled"`
	AugDebug            bool    `json:"augDebug"`
	AugShowMeter        bool    `json:"augShowMeter"`
	AugX                int     `json:"augX"`
	AugY                int     `json:"augY"`
	AugUIWidth          int     `json:"augUIWidth"`
	AugUIHeight         int     `json:"augUIHeight"`
	AugMeterScope       int     `json:"augMeterScope"`
	AugmentationBridgeX int     `json:"augmentationBridgeX"`
	AugmentationBridgeY int     `json:"augmentationBridgeY"`
	SimEnabled          bool    `json:"simEnabled"`
	OverlaysBridgeWidth int     `json:"overlaysBridgeWidth"`
	OverlaysBridgeX     int     `json:"overlaysBridgeX"`
	OverlaysBridgeY     int     `json:"overlaysBridgeY"`
	KBBBridgeX          int     `json:"kbbBridgeX"`
	KBBBridgeY          int     `json:"kbbBridgeY"`
	SimBridgeX          int     `json:"simBridgeX"`
	SimBridgeY          int     `json:"simBridgeY"`
	Anchor              int     `json:"anchor"`
	Grow                int     `json:"grow"`
	Width               int     `json:"width"`
	Height              int     `json:"height"`
	Gap                 int     `json:"gap"`
	FontSize            int     `json:"fontSize"`
	Opacity             int     `json:"opacity"`
	Columns             int     `json:"columns"`
	MaxRows             int     `json:"maxRows"`
	Accent              int     `json:"accent"`
	ShowNames           bool    `json:"showNames"`
	ShowSpells          bool    `json:"showSpells"`
	ShowTimers          bool    `json:"showTimers"`
	Border              bool    `json:"border"`
	Bars                bool    `json:"bars"`
	Enabled             bool    `json:"enabled"`
	DefensiveTracking   bool    `json:"defensiveTracking"`
	KickSpells          []int   `json:"kickSpells"`
	KickEnabled         bool    `json:"kickEnabled"`
	ArenaTracking       bool    `json:"arenaTracking"`
	TrackingEnabled     bool    `json:"trackingEnabled"`
	PublicLookups       bool    `json:"publicLookups"`
	LookupInterval      int     `json:"lookupInterval"`
	LookupBridge        bool    `json:"lookupBridge"`
	LookupBridgeX       int     `json:"lookupBridgeX"`
	LookupBridgeY       int     `json:"lookupBridgeY"`
	FrameEstimates      bool    `json:"frameEstimates"`
	FrameSpells         []int   `json:"frameSpells"`
	EstimatePanel       bool    `json:"estimatePanel"`
	EstimateConfigured  bool    `json:"-"`
	EstimateSelf        bool    `json:"estimateSelf"`
	EstimateParty       bool    `json:"estimateParty"`
	EstimateReady       bool    `json:"estimateReady"`
	EstimateRows        int     `json:"estimateRows"`
	EstimateSize        int     `json:"estimateSize"`
	EstimateX           int     `json:"estimateX"`
	EstimateY           int     `json:"estimateY"`
	Companions          bool    `json:"companions"`
	Preview             bool    `json:"preview"`
	Updates             bool    `json:"updates"`
	Notify              bool    `json:"notify"`
	Days                int     `json:"days"`
	X                   int     `json:"x"`
	Y                   int     `json:"y"`
	Scale               float64 `json:"scale"`
}

func DefaultConfig() Config {
	return Config{AugLiveHybrid: true, AugmentationBridgeX: 288, AugmentationBridgeY: 320, AugDebug: true, AugX: 20, AugY: 300, AugUIWidth: 1920, AugUIHeight: 1080, AugMeterScope: 1, ScanTiming: 2, ScanMinutes: 3, ResourceLookups: true, ResourceFastIdle: true, ResourceSmooth: true, OverlaysBridgeX: 0, OverlaysBridgeY: 320, KickBridgeX: 224, GladiusBridgeX: 448, KBBBridgeY: 224, SimBridgeX: 288, SimBridgeY: 224, DefensiveTracking: true, Anchor: 1, Grow: 1, Width: 340, Height: 34, Gap: 3, FontSize: 14, Opacity: 95, Columns: 1, MaxRows: 12, Accent: 1, ShowNames: true, ShowSpells: true, ShowTimers: true, Border: true, Bars: true, Enabled: true, TrackingEnabled: true, PublicLookups: true, LookupInterval: 1, LookupBridge: true, EstimateSelf: true, EstimateParty: true, EstimateRows: 8, EstimateSize: 2, EstimateX: 60, EstimateY: 200, Companions: true, Updates: true, Notify: true, Days: 1, X: 60, Y: 240, Scale: 1}
}

// Simple mode is a preset, not a destructive rewrite of advanced/module choices.
func (c Config) OptionalWork(option bool) bool {
	if c.ResourceAdvanced {
		return option
	}
	return !c.ResourceLow
}

// Read only top-level engine settings. Nested addon data is skipped, never executed.
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
	values, e := savedEngineFields(b)
	if e != nil {
		return c, e
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
	for k, p := range map[string]*bool{"meterOverlay": &c.MeterOverlay, "resourceAdvanced": &c.ResourceAdvanced, "resourceLow": &c.ResourceLow, "resourceLookups": &c.ResourceLookups, "resourceFastIdle": &c.ResourceFastIdle, "resourceSmooth": &c.ResourceSmooth, "overlaysEnabled": &c.OverlaysEnabled, "kbbEnabled": &c.KBBEnabled, "simEnabled": &c.SimEnabled, "augEnabled": &c.AugEnabled, "augLiveHybrid": &c.AugLiveHybrid, "augDebug": &c.AugDebug, "augShowMeter": &c.AugShowMeter, "kickEnabled": &c.KickEnabled, "estimatePanel": &c.EstimatePanel, "estimateSelf": &c.EstimateSelf, "estimateParty": &c.EstimateParty, "estimateReady": &c.EstimateReady, "publicLookups": &c.PublicLookups, "lookupBridge": &c.LookupBridge, "engineTracking": &c.TrackingEnabled, "companions": &c.Companions, "preview": &c.Preview, "showNames": &c.ShowNames, "showSpells": &c.ShowSpells, "showTimers": &c.ShowTimers, "border": &c.Border, "bars": &c.Bars} {
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
		"scanTiming": {&c.ScanTiming, 1, 4}, "scanMinutes": {&c.ScanMinutes, 1, 5},
		"colorStripVersion": {&c.ColorStripVersion, 2, 2},
		"kickBridgeX":       {&c.KickBridgeX, 0, 16000}, "kickBridgeY": {&c.KickBridgeY, 0, 16000},
		"gladiusBridgeX": {&c.GladiusBridgeX, 0, 16000}, "gladiusBridgeY": {&c.GladiusBridgeY, 0, 16000},
		"overlaysBridgeWidth": {&c.OverlaysBridgeWidth, 16, 1152},
		"overlaysBridgeX":     {&c.OverlaysBridgeX, 0, 16000}, "overlaysBridgeY": {&c.OverlaysBridgeY, 0, 16000},
		"kbbBridgeX": {&c.KBBBridgeX, 0, 16000}, "kbbBridgeY": {&c.KBBBridgeY, 0, 16000},
		"simBridgeX": {&c.SimBridgeX, 0, 16000}, "simBridgeY": {&c.SimBridgeY, 0, 16000},
		"augmentationBridgeX": {&c.AugmentationBridgeX, 0, 16000}, "augmentationBridgeY": {&c.AugmentationBridgeY, 0, 16000},
		"augX": {&c.AugX, 0, 16000}, "augY": {&c.AugY, 0, 16000}, "augUIWidth": {&c.AugUIWidth, 1, 16000}, "augUIHeight": {&c.AugUIHeight, 1, 16000}, "augMeterScope": {&c.AugMeterScope, 1, 2},
		"lookupInterval": {&c.LookupInterval, 1, 5},
		"lookupBridgeX":  {&c.LookupBridgeX, 0, 16000},
		"lookupBridgeY":  {&c.LookupBridgeY, 0, 16000},
		"estimateRows":   {&c.EstimateRows, 1, 12}, "estimateSize": {&c.EstimateSize, 1, 3},
		"estimateX": {&c.EstimateX, 0, 16000}, "estimateY": {&c.EstimateY, 0, 16000},
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
	if w := c.OverlaysBridgeWidth; w != 0 && (w%4 != 0 || 18432%w != 0 || (18432/w)%4 != 0) {
		return c, fmt.Errorf("invalid overlaysBridgeWidth")
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
	c.EstimateConfigured = field("estimatePanel") != ""
	if value := field("engineFrameEstimates"); value != "" {
		if value != "true" && value != "false" {
			return c, fmt.Errorf("invalid engineFrameEstimates")
		}
		c.FrameEstimates = value == "true"
	}
	if value := field("engineFrameSpells"); value != "" {
		list, err := strconv.Unquote(value)
		if err != nil || len(list) > 2048 {
			return c, fmt.Errorf("invalid engineFrameSpells")
		}
		c.FrameSpells = []int{}
		seen := map[int]bool{}
		if list != "" {
			for _, part := range strings.Split(list, ",") {
				id, err := strconv.Atoi(part)
				if err != nil || id < 1 || id > 10000000 || seen[id] || len(c.FrameSpells) >= 128 {
					return c, fmt.Errorf("invalid engineFrameSpells")
				}
				seen[id] = true
				c.FrameSpells = append(c.FrameSpells, id)
			}
		}
		if field("defensiveCatalogVersion") != "2" {
			c.FrameSpells = upgradeDefensiveDefaults(c.FrameSpells, field("defensiveCatalogVersion"))
		}
	}
	if value := field("kickSpells"); value != "" {
		list, err := strconv.Unquote(value)
		if err != nil || len(list) > 2048 {
			return c, fmt.Errorf("invalid kickSpells")
		}
		c.KickSpells = []int{}
		seen := map[int]bool{}
		if list != "" {
			for _, part := range strings.Split(list, ",") {
				id, err := strconv.Atoi(part)
				if err != nil || id < 1 || id > 10000000 || seen[id] || len(c.KickSpells) >= 128 {
					return c, fmt.Errorf("invalid kickSpells")
				}
				seen[id] = true
				c.KickSpells = append(c.KickSpells, id)
			}
		}
	}
	if Forever && field("foreverEngineV1") != "true" {
		c = activateForeverConfig(c)
	}
	c.FrameEstimates = c.Enabled && c.TrackingEnabled && c.FrameEstimates && c.LookupBridge
	c.EstimatePanel = c.Enabled && c.EstimatePanel
	c.DefensiveTracking = c.FrameEstimates
	c.PublicLookups = c.DefensiveTracking && c.PublicLookups && c.OptionalWork(c.ResourceLookups)
	c.TrackingEnabled = c.DefensiveTracking || c.KickEnabled
	c.LookupBridge = c.DefensiveTracking || c.KickEnabled
	c.AugEnabled = !Forever && c.AugEnabled
	c.ArenaTracking = !Forever && field("gladiusEnabled") == "true"
	for _, key := range []string{"gladiusEnabled", "gladiusEngine"} {
		value := field(key)
		if value != "" && value != "true" && value != "false" {
			return c, fmt.Errorf("invalid %s", key)
		}
		if value == "false" {
			c.ArenaTracking = false
		}
	}
	// Native frame auras own the display in the addon. Suppress the detached
	// estimate overlay when that module mode is selected, including old previews.
	native := field("nativeFrames")
	if native != "" && native != "true" && native != "false" {
		return c, fmt.Errorf("invalid nativeFrames")
	}
	if native == "true" {
		c.Enabled = false
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
	return atomicfile.Replace(tmp, path)
}
