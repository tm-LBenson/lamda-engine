// Package simming owns the optional local workspace, independently of tracking.
package simming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/healsim"
	"hash/crc32"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxSnapshot = 128 * 1024
const ChunkSize = 1200     // Legacy LUS1.
const WideChunkSize = 5600 // LUS2, inside the 6144-byte optical packet.
const TransferTTL = 3 * time.Minute

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)

type Character struct {
	GUID  string `json:"guid"`
	Name  string `json:"name"`
	Realm string `json:"realm"`
	Class string `json:"class"`
	Spec  int    `json:"spec"`
	Role  string `json:"role"`
	Level int    `json:"level"`
}

type Snapshot struct {
	HealingStats *healsim.Stats `json:"healingStats,omitempty"`
	GameRequest  *GameRequest   `json:"gameRequest,omitempty"`
	Rules        []ItemRule     `json:"rules,omitempty"`
	Upgrades     []ItemUpgrade  `json:"upgrades,omitempty"`
	Schema       int            `json:"schema"`
	ID           string         `json:"id"`
	Client       string         `json:"client"`
	CapturedAt   int64          `json:"capturedAt"`
	Build        string         `json:"build"`
	Interface    int            `json:"interface"`
	Character    Character      `json:"character"`
	Profile      string         `json:"profile"`
	Items        []Item         `json:"items"`
	Warnings     []string       `json:"warnings"`
	Loot         *LootCatalog   `json:"loot,omitempty"`
	Gear         *GearSnapshot  `json:"gear,omitempty"`
}
type Item struct {
	Slot int    `json:"slot"`
	Link string `json:"link"`
}
type ItemUpgrade struct {
	Source      string `json:"source"`
	Slot        string `json:"slot"`
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Level       int    `json:"level"`
	NextLevel   int    `json:"nextLevel,omitempty"`
	MaxLevel    int    `json:"maxLevel"`
	Track       string `json:"track,omitempty"`
	CurrentRank int    `json:"currentRank,omitempty"`
	MaxRank     int    `json:"maxRank,omitempty"`
	RankLevels  []int  `json:"rankLevels,omitempty"`
}

func DecodeSnapshot(data []byte, client string) (*Snapshot, error) {
	if len(data) == 0 || len(data) > MaxSnapshot {
		return nil, fmt.Errorf("snapshot size is invalid")
	}
	var s Snapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s); err != nil {
		return nil, fmt.Errorf("invalid snapshot: %w", err)
	}
	if !json.Valid(data) || s.Schema != 1 || !requestID.MatchString(s.ID) || s.Client != client || (client != "retail" && client != "forever") || s.CapturedAt <= 0 || s.Build == "" || len(s.Build) > 64 || s.Character.GUID == "" || len(s.Character.GUID) > 128 || len(s.Character.Name) > 128 || len(s.Character.Realm) > 128 || s.Character.Level < 1 || s.Character.Level > 200 || len(s.Items) > 19 || len(s.Upgrades) > 256 || len(s.Warnings) > 32 {
		return nil, fmt.Errorf("snapshot identity, client or schema is invalid")
	}
	if (client == "forever" && s.Interface != 16001) || (client == "retail" && s.Interface < 100000) {
		return nil, fmt.Errorf("snapshot belongs to a different client")
	}
	if s.HealingStats != nil {
		if client != "retail" || !healsim.Supported(s.Character.Spec) {
			return nil, fmt.Errorf("unexpected healing stats")
		}
		if err := s.HealingStats.Validate(); err != nil {
			return nil, err
		}
	}
	for _, item := range s.Items {
		if item.Slot < 1 || item.Slot > 19 || len(item.Link) > 4096 {
			return nil, fmt.Errorf("invalid inventory item")
		}
	}
	for _, warning := range s.Warnings {
		if len(warning) > 2048 {
			return nil, fmt.Errorf("invalid warning")
		}
	}
	upgradeSeen := map[string]bool{}
	for _, upgrade := range s.Upgrades {
		if (upgrade.Source != "equipped" && upgrade.Source != "owned" && upgrade.Source != "vault") || !slots[upgrade.Slot] || len(upgrade.Label) > 200 || len(upgrade.Track) > 40 || strings.ContainsAny(upgrade.Track, "\x00\r\n") || upgrade.Level <= 0 || upgrade.MaxLevel <= upgrade.Level || upgrade.MaxLevel > 2000 || (upgrade.NextLevel != 0 && (upgrade.NextLevel <= upgrade.Level || upgrade.NextLevel > upgrade.MaxLevel)) {
			return nil, fmt.Errorf("invalid item upgrade metadata")
		}
		hasRanks := upgrade.CurrentRank != 0 || upgrade.MaxRank != 0 || len(upgrade.RankLevels) != 0
		if hasRanks {
			if upgrade.CurrentRank < 1 || upgrade.MaxRank <= upgrade.CurrentRank || upgrade.MaxRank > 20 || len(upgrade.RankLevels) != upgrade.MaxRank-upgrade.CurrentRank+1 || upgrade.RankLevels[0] != upgrade.Level || upgrade.RankLevels[len(upgrade.RankLevels)-1] != upgrade.MaxLevel || (upgrade.NextLevel != 0 && upgrade.RankLevels[1] != upgrade.NextLevel) {
				return nil, fmt.Errorf("invalid item upgrade ranks")
			}
			previous := 0
			for _, level := range upgrade.RankLevels {
				if level <= previous || level > 2000 {
					return nil, fmt.Errorf("invalid item upgrade ranks")
				}
				previous = level
			}
		}
		if _, err := itemValue(upgrade.Value); err != nil {
			return nil, fmt.Errorf("invalid item upgrade metadata: %w", err)
		}
		key := upgrade.Source + "\x00" + upgrade.Slot + "\x00" + upgrade.Value
		if upgradeSeen[key] {
			return nil, fmt.Errorf("repeated item upgrade metadata")
		}
		upgradeSeen[key] = true
	}
	if err := ValidateSnapshotRules(&s); err != nil {
		return nil, err
	}
	if err := ValidateLoot(s.Loot, &s); err != nil {
		return nil, err
	}
	if s.Gear != nil {
		if err := s.Gear.Validate(); err != nil {
			return nil, err
		}
	}
	if s.GameRequest != nil {
		if err := s.GameRequest.Validate(s.ID, s.Client); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

type transfer struct {
	header      string
	at          time.Time
	size, count int
	checksum    uint32
	chunks      map[int]string
}

// One active user request per client. Recently completed IDs survive packet
// repetition; the workspace also persists IDs to cover an engine restart.
type Assembler struct {
	Client string
	active *transfer
	done   map[string]time.Time
}

func (a *Assembler) Accept(body []byte, now time.Time) (*Snapshot, error) {
	header, part, ok := strings.Cut(string(body), "\n")
	f := strings.Split(header, "|")
	if !ok || len(f) != 8 || (f[0] != "1" && f[0] != "2") || !requestID.MatchString(f[1]) || f[2] != a.Client {
		return nil, fmt.Errorf("invalid sim transfer header")
	}
	chunkSize := ChunkSize
	if f[0] == "2" {
		chunkSize = WideChunkSize
	}
	at, e1 := strconv.ParseInt(f[3], 10, 64)
	size, e2 := strconv.Atoi(f[4])
	index, e3 := strconv.Atoi(f[5])
	count, e4 := strconv.Atoi(f[6])
	checksum, e5 := strconv.ParseUint(f[7], 10, 32)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || size < 1 || size > MaxSnapshot || count != (size+chunkSize-1)/chunkSize || index < 1 || index > count || len(part) != min(chunkSize, size-(index-1)*chunkSize) {
		return nil, fmt.Errorf("invalid sim chunk")
	}
	stamp := time.Unix(at, 0)
	if now.Sub(stamp) > TransferTTL || stamp.Sub(now) > 30*time.Second {
		return nil, fmt.Errorf("expired sim transfer; click Run Sim again")
	}
	if a.done == nil {
		a.done = map[string]time.Time{}
	}
	for id, t := range a.done {
		if now.Sub(t) > TransferTTL {
			delete(a.done, id)
		}
	}
	if _, exists := a.done[f[1]]; exists {
		return nil, nil
	}
	identity := strings.Join([]string{f[1], f[0], f[2], f[3], f[4], f[6], f[7]}, "|")
	if a.active != nil && a.active.header != identity {
		oldID := strings.Split(a.active.header, "|")[0]
		if oldID == f[1] {
			return nil, fmt.Errorf("conflicting sim transfer metadata")
		}
		if stamp.Before(a.active.at) {
			return nil, fmt.Errorf("superseded sim transfer")
		}
		a.done[oldID] = now
		a.active = nil
	}
	if a.active == nil {
		a.active = &transfer{identity, stamp, size, count, uint32(checksum), map[int]string{}}
	}
	t := a.active
	if previous, exists := t.chunks[index]; exists && previous != part {
		return nil, fmt.Errorf("conflicting sim chunk")
	}
	t.chunks[index] = part
	if len(t.chunks) != count {
		return nil, nil
	}
	var data strings.Builder
	for i := 1; i <= count; i++ {
		data.WriteString(t.chunks[i])
	}
	if data.Len() != size || crc32.ChecksumIEEE([]byte(data.String())) != t.checksum {
		a.active = nil
		return nil, fmt.Errorf("sim snapshot checksum failed")
	}
	s, err := DecodeSnapshot([]byte(data.String()), a.Client)
	if err != nil {
		a.active = nil
		return nil, err
	}
	if s.ID != f[1] || s.CapturedAt != at {
		a.active = nil
		return nil, fmt.Errorf("sim snapshot identity changed")
	}
	a.done[s.ID] = now
	a.active = nil
	return s, nil
}
