package augmentation

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

type ChatSummary struct {
	EndedAt      time.Time `json:"endedAt"`
	rankingRows  int
	awardsHeader string
	Segment      string   `json:"segment"`
	Name         string   `json:"name"`
	Text         string   `json:"text"`
	Lines        []string `json:"lines"`
	Channel      string   `json:"channel,omitempty"`
	MetricsKnown bool     `json:"metricsKnown"`
}
type summaryCounts struct {
	EncounterAwardVersion                           int
	OverhealRoleVersion                             int
	HealingRoles                                    map[string]string
	HealingOthers, OverhealingOthers                map[string]int64
	MechanicUnknown                                 map[string]bool
	MechanicTotals                                  map[string]map[string]int64
	AwardVersion                                    int
	HealingKnown                                    bool
	Healing, Overhealing, Interrupts, Resurrections map[string]int64
	FirstDeaths, AvoidableHits, MathFails           map[string]int64
	Fish                                            map[string]int64
	FishKnown                                       bool
	Soaks, Avoidable                                map[string]int64
	MechanicsKnown                                  bool
	Dispels, Deaths                                 map[string]int64
	Known                                           bool
}

var errSummaryPending = errors.New("Report is still preparing")

func summaryText(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '|' || r == '~' {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > limit {
		s = s[:limit]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}
func awardNames(rows []DamageRow, value func(DamageRow) int64) (string, int64) {
	maximum := int64(0)
	names := []string{}
	for _, r := range rows {
		n := value(r)
		if n <= 0 || n < maximum {
			continue
		}
		if n > maximum {
			maximum = n
			names = nil
		}
		name := strings.Split(r.Name, "-")[0]
		name = summaryText(name, 14)
		if name == "" {
			name = "Player"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 1 {
		names = append(names[:1], fmt.Sprintf("+%d tied", len(names)-1))
	}
	return strings.Join(names, "/"), maximum
}
func rosterHash(rows []DamageRow) uint32 {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.GUID)
	}
	sort.Strings(ids)
	return crc32.ChecksumIEEE([]byte(strings.Join(ids, ",")))
}
func makeChatAwards(m DamageMeter, c summaryCounts) ChatSummary {
	title := summaryText(m.Name, 40)
	damage, amount := awardNames(m.Rows, func(r DamageRow) int64 { return r.Adjusted })
	parts := []string{"[LamdaMeters] " + title}
	if damage != "" && m.Valid && !m.Provisional {
		label := "Damage MVP: "
		if m.Partial {
			label = "Recorded damage MVP: "
		}
		parts = append(parts, label+damage+" "+compact(amount))
	}
	if c.Known {
		name, n := awardNames(m.Rows, func(r DamageRow) int64 { return c.Dispels[r.GUID] })
		if n > 0 {
			parts = append(parts, fmt.Sprintf("Clean-up crew: %s (%d %s)", name, n, plural(n, "dispel")))
		}
		name, n = awardNames(m.Rows, func(r DamageRow) int64 { return c.Deaths[r.GUID] })
		if n == 0 {
			parts = append(parts, "Deathless. Floors left uninspected!")
		}
		if n > 0 {
			award := deathAwardLabelFor(m)
			parts = append(parts, fmt.Sprintf("%s: %s (%d %s)", award, name, n, plural(n, "death")))
		}
	}
	if !m.Valid || m.Provisional {
		parts = append(parts, "Damage award pending: incomplete log")
	}
	awardLines := []string{}
	for _, part := range parts[1:] {
		awardLines = append(awardLines, "[LamdaMeters] "+part)
	}
	if !c.Known {
		awardLines = append(awardLines, "[LamdaMeters] Utility awards unavailable for this saved report")
	}
	text := strings.Join(parts, "; ")
	if len(text) > 255 {
		parts[0] = "[LamdaMeters] " + summaryText(m.Name, 16)
		text = strings.Join(parts, "; ")
	}
	// Never truncate an award into a different number/name. Shorten the joke
	// before dropping an optional part; each emitted message remains complete.
	if len(text) > 255 && c.Known {
		for i, p := range parts {
			for _, j := range []string{"Professional Floor Hugger", "Floor Inspector", "Gravity Tester", "Afterlife VIP"} {
				p = strings.ReplaceAll(p, j, "Top deaths")
			}
			parts[i] = p
		}
		text = strings.Join(parts, "; ")
	}
	for len(text) > 255 && len(parts) > 2 {
		parts = parts[:len(parts)-1]
		text = strings.Join(parts, "; ")
	}
	return ChatSummary{Segment: m.ID, Name: m.Name, Text: text, Lines: awardLines, MetricsKnown: c.Known}
}

// Text uses a printable separator for the nonce-bound optical transport.
// Each chat line is independently bounded; no ranked player is truncated away.
func makeChatSummary(m DamageMeter, c summaryCounts) ChatSummary {
	const prefix = "[LamdaMeters] "
	lines := []string{}
	if m.Valid && !m.Provisional && m.Seconds > 0 {
		label := "DPS"
		if m.HasAug {
			label = "Aug-adjusted DPS"
		}
		if m.Partial {
			label += " (partial log)"
		}
		if len(m.Rows) > 5 {
			label += " (top 5)"
		}
		lines = append(lines, prefix+summaryText(m.Name, 48)+" - "+label)
		rows := append([]DamageRow{}, m.Rows...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Adjusted == rows[j].Adjusted {
				return rows[i].GUID < rows[j].GUID
			}
			return rows[i].Adjusted > rows[j].Adjusted
		})
		for i, row := range rows[:min(5, len(rows))] {
			name := summaryText(strings.Split(row.Name, "-")[0], 36)
			if name == "" {
				name = "Player"
			}
			lines = append(lines, fmt.Sprintf("%s%d. %s %s DPS", prefix, i+1, name, compact(int64(float64(row.Adjusted)/m.Seconds))))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, prefix+summaryText(m.Name, 48)+" - Recorded report")
	}
	awards := makeChatAwards(m, c)
	lines = append(lines, awards.Lines...)
	rankingRows := 0
	if m.Valid && !m.Provisional && m.Seconds > 0 {
		rankingRows = min(5, len(m.Rows))
	}
	header := prefix + summaryText(m.Name, 48) + " - Awards"
	if m.Partial {
		header += " (partial log)"
	}
	return ChatSummary{rankingRows: rankingRows, awardsHeader: header, EndedAt: m.EndedAt, Segment: m.ID, Name: m.Name, Lines: lines, Text: strings.Join(lines, "~"), MetricsKnown: c.Known}
}

// Summary uses completed, recorded adjusted damage. Current estimates and HPS
// rows cannot win a damage award. Missing utility coverage is never zero.
func (s *Store) Summary(ctx context.Context, scope string, groupHash uint32) (ChatSummary, error) {
	return s.summary(ctx, scope, groupHash, "", false)
}

// The preview's verified identity selects its own historical report without
// changing the combat collector's current character or counters.
func (s *Store) SummaryForPlayer(ctx context.Context, scope string, groupHash uint32, owner string) (ChatSummary, error) {
	if !player(owner) {
		return ChatSummary{}, fmt.Errorf("The summary player is unavailable")
	}
	return s.summary(ctx, scope, groupHash, owner, true)
}

// A completed fight's log boundaries can reach the collector just after the
// player requests its report. Keep that single request alive while they arrive.
func (s *Store) SummaryForPlayerWhenReady(ctx context.Context, scope string, groupHash uint32, owner string, options ...uint8) (ChatSummary, error) {
	return s.summaryForBoundaryWhenReady(ctx, scope, owner, time.Time{}, "", options...)
}

func (s *Store) SummaryForBoundaryWhenReady(ctx context.Context, scope, owner string, endedAt time.Time, encounter string, options ...uint8) (ChatSummary, error) {
	return s.summaryForBoundaryWhenReady(ctx, scope, owner, endedAt, encounter, options...)
}

func (s *Store) summaryForBoundaryWhenReady(ctx context.Context, scope, owner string, endedAt time.Time, encounter string, options ...uint8) (ChatSummary, error) {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && last != nil {
				return ChatSummary{}, last
			}
			return ChatSummary{}, err
		}
		result, err := s.PreparedSummaryForPlayer(scope, owner, options...)
		if err == nil && !endedAt.IsZero() && (result.EndedAt.Before(endedAt.Add(-time.Second)) || scope != "key" && encounter != "" && encounter != "0" && encounterID(DamageMeter{ID: result.Segment}) != encounter) {
			err = fmt.Errorf("%w: waiting for this fight's final log records", errSummaryPending)
		}
		if !errors.Is(err, errSummaryPending) {
			return result, err
		}
		last = err
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
func (s *Store) summary(ctx context.Context, scope string, groupHash uint32, owner string, reviewed bool) (ChatSummary, error) {
	withUptime := scope == "boss-uptime"
	if withUptime {
		scope = "boss"
	}
	s.mu.Lock()
	if owner == "" {
		owner = s.own
	}
	if scope == "key" && s.active["key"] != nil {
		s.mu.Unlock()
		return ChatSummary{}, fmt.Errorf("%w: waiting for the key's final log records", errSummaryPending)
	}
	var meter *DamageMeter
	for _, v := range s.damageMeters {
		if v.Owner != owner || v.Live || v.Metric != 0 {
			continue
		}
		match := scope == "key" && v.Kind == "key" || scope == "boss" && v.Kind == "boss" || scope == "recorded:"+v.ID
		if match {
			copy := v
			copy.Rows = append([]DamageRow{}, v.Rows...)
			meter = &copy
			break
		}
	}
	var uptime *Report
	if withUptime && meter != nil {
		for _, r := range s.reports {
			if strings.TrimPrefix(r.ID, meter.Owner+"/") == meter.ID && r.GUID == meter.Owner && r.Seconds > 0 {
				copy := r
				uptime = &copy
				break
			}
		}
	}
	path := ""
	if s.debug != nil {
		path = s.debug.SourceFile
	}
	s.mu.Unlock()
	if reviewed {
		var err error
		meter, err = s.recoverSummary(ctx, path, scope, owner, meter)
		if err != nil {
			return ChatSummary{}, err
		}
		if withUptime && meter != nil {
			s.mu.Lock()
			for _, r := range s.reports {
				if r.GUID == owner && strings.TrimPrefix(r.ID, owner+"/") == meter.ID && r.Seconds > 0 {
					copy := r
					uptime = &copy
					break
				}
			}
			s.mu.Unlock()
		}
	}
	if meter == nil {
		return ChatSummary{}, fmt.Errorf("%w: no completed recorded %s is ready yet", errSummaryPending, scope)
	}

	if len(meter.Rows) == 0 || (!reviewed && rosterHash(meter.Rows) != groupHash) {
		return ChatSummary{}, fmt.Errorf("The report's party differs from your current group")
	}
	path = summaryLogPath(path, *meter)
	counts := summaryCounts{}
	if meter.Utility != nil && meter.Utility.Known {
		counts = *meter.Utility
	} else {
		key := meter.Owner + "/" + meter.ID
		s.mu.Lock()
		cached, ok := s.summaryUtilities[key]
		s.mu.Unlock()
		if ok {
			counts = cached
		} else {
			counts = readSummaryCounts(ctx, path, *meter)
			if counts.Known {
				s.mu.Lock()
				if s.summaryUtilities == nil || len(s.summaryUtilities) >= 128 {
					s.summaryUtilities = map[string]summaryCounts{}
				}
				s.summaryUtilities[key] = counts
				s.mu.Unlock()
			}
		}
	}
	if (!meter.Valid || meter.Provisional) && !counts.Known {
		return ChatSummary{}, fmt.Errorf("%w: the latest report is incomplete", errSummaryPending)
	}
	if meter.AttributionMode != "all" || meter.DamageBasis != "health+absorbs" {
		meter.Partial = true
	}
	result := makeChatSummary(*meter, counts)
	if uptime != nil {
		line := fmt.Sprintf("[LamdaMeters] %s Ebon Might uptime: %.1f%% (recorded)", summaryText(strings.Split(uptime.Player, "-")[0], 36), 100*uptime.EbonSeconds/uptime.Seconds)
		result.Lines = append(result.Lines, line)
		result.Text = strings.Join(result.Lines, "~")
	}
	return result, nil
}

// Read the original log, including prepull pet ownership. Only a matching pair
// of segment boundaries establishes zero deaths/dispels for that segment.
func readSummaryCounts(ctx context.Context, path string, m DamageMeter) summaryCounts {
	c := summaryCounts{Dispels: map[string]int64{}, Deaths: map[string]int64{}}
	if path == "" || m.StartedAt.IsZero() || m.EndedAt.IsZero() {
		return c
	}
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	roster := map[string]bool{}
	for _, r := range m.Rows {
		roster[r.GUID] = true
	}
	pets := map[string]string{}
	raw := map[string]int64{}
	start, end := false, false
	identity := strings.Split(m.ID, "/")
	segmentID := ""
	if len(identity) > 1 {
		segmentID = identity[1]
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return c
		}
		line := scanner.Text()
		_, payload, ok := strings.Cut(line, "  ")
		if !ok {
			continue
		}
		kind, _, _ := strings.Cut(payload, ",")
		switch kind {
		case "SPELL_SUMMON", "SPELL_CREATE", "SPELL_DISPEL", "SPELL_STOLEN", "UNIT_DIED", "ENCOUNTER_START", "ENCOUNTER_END", "CHALLENGE_MODE_START", "CHALLENGE_MODE_END":
		default:
			continue
		}
		at, fields, err := engine.RecordFields(line)
		if err != nil || len(fields) < 2 {
			continue
		}
		if at.After(m.EndedAt.Add(time.Minute)) {
			break
		}
		if (kind == "SPELL_SUMMON" || kind == "SPELL_CREATE") && len(fields) > 7 && friendlyGroup(fields[3]) {
			pets[fields[5]] = fields[1]
			continue
		}
		if at.Equal(m.StartedAt) && ((m.Kind == "key" && kind == "CHALLENGE_MODE_START" && len(fields) > 2 && fields[2] == segmentID) || (m.Kind == "boss" && kind == "ENCOUNTER_START" && fields[1] == segmentID)) {
			start = true
		}
		if at.Equal(m.EndedAt) && ((m.Kind == "key" && kind == "CHALLENGE_MODE_END" && fields[1] == segmentID) || (m.Kind == "boss" && kind == "ENCOUNTER_END" && fields[1] == segmentID)) {
			end = true
		}
		if at.Before(m.StartedAt) || at.After(m.EndedAt) || len(fields) < 9 {
			continue
		}
		if (kind == "SPELL_DISPEL" || kind == "SPELL_STOLEN") && friendlyGroup(fields[3]) {
			raw[fields[1]]++
		}
		if kind == "UNIT_DIED" && roster[fields[5]] && friendlyGroup(fields[7]) {
			c.Deaths[fields[5]]++
		}
	}
	if scanner.Err() != nil || !start || !end {
		return c
	}
	for id, n := range raw {
		seen := map[string]bool{}
		for !player(id) && pets[id] != "" && !seen[id] {
			seen[id] = true
			id = pets[id]
		}
		if !roster[id] {
			return c
		}
		c.Dispels[id] += n
	}
	c.Known = true
	return c
}

func plural(n int64, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Saved reports can outlive a log-file rollover. Choose a same-directory log
// whose creation timestamp precedes the report; boundary/row validation below
// still has to establish coverage before any utility award is emitted.
func summaryLogPath(current string, m DamageMeter) string {
	if current == "" || m.StartedAt.IsZero() {
		return current
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(current), "WoWCombatLog-*.txt"))
	if err != nil {
		return current
	}
	selected := current
	latest := time.Time{}
	for _, path := range paths {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "WoWCombatLog-"), ".txt")
		at, err := time.ParseInLocation("010206_150405", name, m.StartedAt.Location())
		if err == nil && !at.After(m.StartedAt) && at.After(latest) {
			latest = at
			selected = path
		}
	}
	return selected
}

// Applied once during preparation. A Share lookup only selects the cached variant.
func summaryWithOptions(v ChatSummary, options uint8) ChatSummary {
	lines := []string{}
	bytesUsed := 0
	for i, line := range v.Lines {
		// Rankings-only reports end after the cached ranked rows. Mechanics,
		// banter and uptime remain available through the separate awards view.
		if options&4 != 0 && i > v.rankingRows {
			continue
		}
		if options&1 != 0 && i > 0 && i <= v.rankingRows {
			continue
		}
		if options&2 != 0 && (strings.HasPrefix(line, "[LamdaMeters] Damage MVP:") || strings.HasPrefix(line, "[LamdaMeters] Recorded damage MVP:")) {
			continue
		}
		if i == 0 && options&1 != 0 {
			line = v.awardsHeader
		}
		// Preserve complete names/numbers and the existing fast transfer size.
		// Large award pools never make a preview fail or enlarge its protocol.
		bytes := bytesUsed + len(line)
		if len(lines) > 0 {
			bytes++
		}
		if len(line) > 255 || bytes > 4096 || len(lines) >= 50 {
			continue
		}
		lines = append(lines, line)
		bytesUsed = bytes
	}
	v.Lines = lines
	v.Text = strings.Join(lines, "~")
	return v
}
