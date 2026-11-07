package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schema 22 / Retail 12.1.0 was verified against the recorded desktop log.
// Talent and equipment data are evidence, not yet a table of cooldown modifiers.
type BuildSnapshot struct {
	Player     string            `json:"player,omitempty"`
	SpellRanks map[int]int       `json:"spellRanks,omitempty"`
	GUID       string            `json:"guid"`
	At         time.Time         `json:"at"`
	ReceivedAt time.Time         `json:"receivedAt"`
	Source     string            `json:"source"`
	LogVersion int               `json:"logVersion"`
	GameBuild  string            `json:"gameBuild"`
	Revision   string            `json:"revision"`
	SpecID     int               `json:"specId"`
	Talents    [][3]int          `json:"talents"`
	PVPTalents []int             `json:"pvpTalents"`
	Equipment  []json.RawMessage `json:"equipment"`
	Missing    []string          `json:"missing"`
}

// Split only top-level commas: talent/gear tuples contain nested delimiters.
// Quotes use CSV escaping. Malformed/truncated nesting never shifts spec fields.
func structuredFields(s string) ([]string, error) {
	if len(s) > 64<<10 {
		return nil, fmt.Errorf("oversize metadata")
	}
	fields := []string{}
	stack := []rune{}
	quoted := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := rune(s[i])
		if c == '"' {
			if quoted && i+1 < len(s) && s[i+1] == '"' {
				i++
				continue
			}
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		switch c {
		case '[', '(':
			if len(stack) >= 8 {
				return nil, fmt.Errorf("metadata nesting")
			}
			stack = append(stack, c)
		case ']', ')':
			if len(stack) == 0 {
				return nil, fmt.Errorf("unmatched delimiter")
			}
			open := stack[len(stack)-1]
			if (c == ']' && open != '[') || (c == ')' && open != '(') {
				return nil, fmt.Errorf("mismatched delimiter")
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				fields = append(fields, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if quoted || len(stack) > 0 {
		return nil, fmt.Errorf("incomplete metadata")
	}
	return append(fields, strings.TrimSpace(s[start:])), nil
}

func (m *Model) readLogVersion(line string, now time.Time) {
	at, fields, err := parseRecord(line)
	// Discard an earlier version even when a new version marker is malformed.
	m.LogVersion, m.GameBuild = 0, ""
	if err != nil || len(fields) < 2 {
		m.RecordGap("invalid_log_version", now, Cast{At: at})
		return
	}
	version, err := strconv.Atoi(fields[1])
	if err != nil || version <= 0 {
		m.RecordGap("invalid_log_version", now, Cast{At: at})
		return
	}
	m.LogVersion = version
	for i := 2; i+1 < len(fields); i += 2 {
		if fields[i] == "BUILD_VERSION" && len(fields[i+1]) <= 32 {
			m.GameBuild = fields[i+1]
		}
	}
	if version != 22 {
		m.RecordGap("unsupported_log_version", now, Cast{At: at})
	}
}

func (m *Model) readBuild(line string, now time.Time) {
	at, _, err := RecordHeader(line)
	if err != nil {
		m.RecordGap("malformed_build", now, Cast{})
		return
	}
	if now.Sub(at) < -2*time.Second {
		m.RecordGap("future_build", now, Cast{At: at})
		return
	}
	if now.Sub(at) >= observationRetention {
		m.RecordGap("stale_build", now, Cast{At: at})
		return
	}
	if m.LogVersion != 0 && m.LogVersion != 22 {
		m.RecordGap("unsupported_build_schema", now, Cast{At: at})
		return
	}
	match := stamp.FindStringSubmatch(line)
	fields, err := structuredFields(match[3])
	if err != nil || len(fields) < 26 || len(fields) > 34 || !strings.HasPrefix(fields[1], "Player-") || len(fields[1]) > 128 {
		m.RecordGap("malformed_build", now, Cast{At: at})
		return
	}
	b := BuildSnapshot{GUID: fields[1], At: at, ReceivedAt: now, Source: "combat_log",
		LogVersion: m.LogVersion, GameBuild: m.GameBuild, Missing: []string{}}
	c := Cast{At: at, GUID: b.GUID}
	if prev, exists := m.Builds[b.GUID]; exists && !b.At.After(prev.At) {
		if b.At.Before(prev.At) {
			m.RecordGap("out_of_order_build", now, c)
		}
		return
	}
	missing := func(field string) { b.Missing = append(b.Missing, field); m.RecordGap("build_missing_"+field, now, c) }
	if m.LogVersion == 0 {
		missing("log_version")
	}
	if m.GameBuild == "" {
		missing("game_build")
	}
	b.SpecID, err = strconv.Atoi(fields[25])
	if err != nil || b.SpecID <= 0 || b.SpecID > 100000 {
		b.SpecID = 0
		missing("spec")
	}
	field := func(index int) string {
		if index >= len(fields) {
			return ""
		}
		return fields[index]
	}
	array := func(raw string, target any) bool {
		if len(raw) < 2 || (raw[0] != '[' && raw[0] != '(') {
			return false
		}
		converted := strings.NewReplacer("(", "[", ")", "]").Replace(raw)
		return json.Unmarshal([]byte(converted), target) == nil
	}
	var talents [][]int
	validTalents := array(field(26), &talents) && len(talents) > 0 && len(talents) <= 256
	for _, talent := range talents {
		if len(talent) != 3 || talent[0] <= 0 || talent[1] <= 0 || talent[2] <= 0 || talent[2] > 100 {
			validTalents = false
			break
		}
		b.Talents = append(b.Talents, [3]int{talent[0], talent[1], talent[2]})
	}
	if !validTalents {
		b.Talents = nil
		missing("talents")
	}
	validPVP := array(field(27), &b.PVPTalents) && len(b.PVPTalents) <= 16
	for _, id := range b.PVPTalents {
		if id < 0 {
			validPVP = false
		}
	}
	if !validPVP {
		b.PVPTalents = nil
		missing("pvp_talents")
	}
	var equipment []json.RawMessage
	validEquipment := array(field(28), &equipment) && len(equipment) > 0 && len(equipment) <= 32
	for _, item := range equipment {
		var tuple []json.RawMessage
		if json.Unmarshal(item, &tuple) != nil || len(tuple) < 2 || len(tuple) > 8 {
			validEquipment = false
			continue
		}
		var id, level int
		if string(tuple[0]) == "null" || string(tuple[1]) == "null" || json.Unmarshal(tuple[0], &id) != nil || json.Unmarshal(tuple[1], &level) != nil || id < 0 || level < 0 {
			validEquipment = false
			continue
		}
		if len(b.Equipment) < 32 {
			b.Equipment = append(b.Equipment, item)
		}
	}
	if !validEquipment {
		missing("equipment")
	}
	// Revision hashes only build content, never delivery time or mutable stats.
	identity := struct {
		Version   int
		Game      string
		Spec      int
		Talents   [][3]int
		PVP       []int
		Equipment []json.RawMessage
		Missing   []string
	}{b.LogVersion, b.GameBuild, b.SpecID, b.Talents, b.PVPTalents, b.Equipment, b.Missing}
	encoded, _ := json.Marshal(identity)
	b.Revision = fmt.Sprintf("%x", sha256.Sum256(encoded))
	if len(m.Builds) >= 128 {
		if _, exists := m.Builds[b.GUID]; !exists {
			var oldest string
			for guid, candidate := range m.Builds {
				if oldest == "" || candidate.At.Before(m.Builds[oldest].At) {
					oldest = guid
				}
			}
			delete(m.Builds, oldest)
		}
	}
	m.Builds[b.GUID] = b
	m.Diagnostics.BuildRecords++
}
