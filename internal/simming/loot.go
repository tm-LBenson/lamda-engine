package simming

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LootCatalog is captured from the player's Encounter Journal with Blizzard's
// class/spec loot filter. It does not infer eligibility from armor or stats.
// A pool is one acquisition source and difficulty, never several bosses mixed
// together. Only complete pools can be used for acquisition probabilities.
type LootCatalog struct {
	Schema     int        `json:"schema"`
	Spec       int        `json:"spec"`
	Season     int        `json:"season"`
	CapturedAt int64      `json:"capturedAt"`
	Pools      []LootPool `json:"pools"`
}

type LootPool struct {
	InstanceID     int    `json:"instanceID"`
	EncounterID    int    `json:"encounterID,omitempty"`
	ChallengeMapID int    `json:"challengeMapID,omitempty"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	DifficultyID   int    `json:"difficultyID"`
	// JournalDifficultyID records the native selection read back after applying
	// the keystone preview. PreviewRewardLevel is present only when Blizzard's
	// reward API supplied a positive end-of-run level, independent of the Vault.
	JournalDifficultyID int        `json:"journalDifficultyID,omitempty"`
	PreviewRewardLevel  int        `json:"previewRewardLevel,omitempty"`
	KeyLevel            int        `json:"keyLevel,omitempty"`
	Complete            bool       `json:"complete"`
	Items               []LootItem `json:"items"`
}

type LootItem struct {
	MaxLevel      int    `json:"maxLevel,omitempty"`
	RulesKnown    bool   `json:"rulesKnown,omitempty"`
	Unique        bool   `json:"unique,omitempty"`
	LimitCategory int    `json:"limitCategory,omitempty"`
	LimitMax      int    `json:"limitMax,omitempty"`
	ID            int    `json:"id"`
	Label         string `json:"label"`
	Slot          string `json:"slot"`
	Link          string `json:"link"`
	Level         int    `json:"level"`
	InventoryType string `json:"inventoryType"`
	Rare          bool   `json:"rare,omitempty"`
}

func (p LootPool) SourceID() string {
	if p.Type == "Raid" {
		return fmt.Sprintf("raid:%d:%d:%d", p.InstanceID, p.EncounterID, p.DifficultyID)
	}
	return fmt.Sprintf("dungeon:%d:%d:%d:%d", p.InstanceID, p.ChallengeMapID, p.DifficultyID, p.KeyLevel)
}

var lootInventoryTypes = map[string]string{
	"INVTYPE_HEAD": "head", "INVTYPE_NECK": "neck", "INVTYPE_SHOULDER": "shoulder",
	"INVTYPE_CLOAK": "back", "INVTYPE_CHEST": "chest", "INVTYPE_ROBE": "chest",
	"INVTYPE_WRIST": "wrist", "INVTYPE_HAND": "hands", "INVTYPE_WAIST": "waist",
	"INVTYPE_LEGS": "legs", "INVTYPE_FEET": "feet", "INVTYPE_FINGER": "finger1",
	"INVTYPE_TRINKET": "trinket1", "INVTYPE_2HWEAPON": "main_hand", "INVTYPE_WEAPON": "main_hand",
	"INVTYPE_WEAPONMAINHAND": "main_hand", "INVTYPE_RANGED": "main_hand",
	"INVTYPE_RANGEDRIGHT": "main_hand", "INVTYPE_WEAPONOFFHAND": "off_hand",
	"INVTYPE_HOLDABLE": "off_hand", "INVTYPE_SHIELD": "off_hand",
}

func ValidateLoot(c *LootCatalog, s *Snapshot) error {
	if c == nil {
		return nil
	}
	if s == nil || s.Client != "retail" || c.Schema != 1 || c.Spec != s.Character.Spec || c.Season < 0 || c.CapturedAt <= 0 || c.CapturedAt > s.CapturedAt+30 || c.CapturedAt < s.CapturedAt-300 || len(c.Pools) == 0 || len(c.Pools) > 64 {
		return fmt.Errorf("loot catalog does not match this snapshot")
	}
	data, err := json.Marshal(c)
	if err != nil || len(data) > 110*1024 {
		return fmt.Errorf("loot catalog exceeds the capture limit")
	}
	total := 0
	seenPools := map[string]bool{}
	for _, pool := range c.Pools {
		if pool.InstanceID <= 0 || pool.InstanceID > 100000 || pool.EncounterID < 0 || pool.EncounterID > 100000 || pool.ChallengeMapID < 0 || pool.ChallengeMapID > 100000 || len(pool.Name) == 0 || len(pool.Name) > 240 || strings.ContainsAny(pool.Name, "\n\r\x00") || pool.DifficultyID < 1 || pool.DifficultyID > 100 || pool.KeyLevel < 0 || pool.KeyLevel > 100 || (pool.Type != "Raid" && pool.Type != "Dungeon") {
			return fmt.Errorf("invalid loot source")
		}
		if (pool.Type == "Raid" && (pool.EncounterID == 0 || pool.KeyLevel != 0)) || (pool.Type == "Dungeon" && pool.EncounterID != 0) || seenPools[pool.SourceID()] {
			return fmt.Errorf("invalid or repeated loot source")
		}
		if pool.JournalDifficultyID != 0 || pool.PreviewRewardLevel != 0 {
			if pool.Type != "Dungeon" || pool.DifficultyID != 8 || pool.JournalDifficultyID != 8 || pool.KeyLevel < 2 || pool.PreviewRewardLevel < 0 || pool.PreviewRewardLevel > 2000 {
				return fmt.Errorf("invalid Mythic+ journal preview evidence")
			}
		}
		seenPools[pool.SourceID()] = true
		seenItems := map[int]bool{}
		for _, item := range pool.Items {
			total++
			if pool.PreviewRewardLevel > 0 && item.Level != pool.PreviewRewardLevel {
				return fmt.Errorf("loot item does not match the captured end-of-run reward level")
			}
			if total > 512 || item.ID < 1 || item.ID > 10000000 || item.Level < 1 || item.Level > 2000 || item.MaxLevel > 2000 || item.MaxLevel < 0 || (item.MaxLevel > 0 && item.MaxLevel < item.Level) || item.Label == "" || len(item.Label) > 240 || strings.ContainsAny(item.Label, "\n\r\x00") || lootInventoryTypes[item.InventoryType] != item.Slot || item.Slot == "" || seenItems[item.ID] {
				return fmt.Errorf("invalid or repeated loot item")
			}
			seenItems[item.ID] = true
			if item.RulesKnown {
				if err := ValidateItemRules([]ItemRule{{ID: item.ID, InventoryType: item.InventoryType, Unique: item.Unique, LimitCategory: item.LimitCategory, LimitMax: item.LimitMax}}); err != nil {
					return err
				}
			}
			if _, err := item.SimCValue(); err != nil {
				return err
			}
		}
	}
	return nil
}

var itemLinkBody = regexp.MustCompile(`(?:^item:|\|Hitem:)([-0-9:]+)(?:$|\|h)`)

// SimCValue parses only numeric fields from an exact journal preview hyperlink.
// Offsets and modifier meanings follow the official SimulationCraft exporter.
// No label or untrusted simulator directive is copied into the worker input.
func (item LootItem) SimCValue() (string, error) {
	if len(item.Link) > 2048 || strings.ContainsAny(item.Link, "\n\r\x00") {
		return "", fmt.Errorf("invalid loot item link")
	}
	match := itemLinkBody.FindStringSubmatch(item.Link)
	if match == nil {
		return "", fmt.Errorf("loot item link is missing its numeric data")
	}
	parts := strings.Split(match[1], ":")
	if len(parts) < 13 || len(parts) > 256 {
		return "", fmt.Errorf("loot item link has an unsupported format")
	}
	fields := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			continue
		}
		n, err := strconv.ParseInt(part, 10, 32)
		if err != nil || (n < 0 && i != 6 && i != 7) {
			return "", fmt.Errorf("loot item link contains an invalid number")
		}
		fields[i] = int(n)
	}
	if fields[0] != item.ID || fields[6] != 0 {
		return "", fmt.Errorf("loot item ID mismatch or unsupported random suffix")
	}
	options := []string{fmt.Sprintf("id=%d", item.ID)}
	if fields[1] != 0 {
		options = append(options, fmt.Sprintf("enchant_id=%d", fields[1]))
	}
	gems := []string{}
	for i := 2; i <= 5; i++ {
		gems = append(gems, strconv.Itoa(fields[i]))
	}
	for len(gems) > 0 && gems[len(gems)-1] == "0" {
		gems = gems[:len(gems)-1]
	}
	if len(gems) > 0 {
		options = append(options, "gem_id="+strings.Join(gems, "/"))
	}
	count := fields[12]
	if count < 0 || count > 64 || len(fields) < 13+count {
		return "", fmt.Errorf("loot item bonuses are incomplete")
	}
	bonuses := []string{}
	for _, v := range fields[13 : 13+count] {
		if v <= 0 {
			return "", fmt.Errorf("loot item has an invalid bonus")
		}
		bonuses = append(bonuses, strconv.Itoa(v))
	}
	if len(bonuses) > 0 {
		options = append(options, "bonus_id="+strings.Join(bonuses, "/"))
	}
	// A missing modifier list is permitted in old/static journal links.
	modifierOffset := 13 + count
	if modifierOffset < len(fields) {
		pairs := fields[modifierOffset]
		if pairs < 0 || pairs > 32 || len(fields) < modifierOffset+1+2*pairs {
			return "", fmt.Errorf("loot item modifiers are incomplete")
		}
		crafted := []string{}
		seen := map[int]bool{}
		for i := 0; i < pairs; i++ {
			kind, value := fields[modifierOffset+1+2*i], fields[modifierOffset+2+2*i]
			if seen[kind] {
				return "", fmt.Errorf("loot item modifier is repeated")
			}
			seen[kind] = true
			switch kind {
			case 9:
				options = append(options, fmt.Sprintf("drop_level=%d", value))
			case 28:
				options = append(options, fmt.Sprintf("content_tuning=%d", value))
			case 29, 30:
				crafted = append(crafted, strconv.Itoa(value))
			case 64:
				options = append(options, fmt.Sprintf("redirected_base_stats=%d", value))
			}
		}
		if len(crafted) > 0 {
			options = append(options, "crafted_stats="+strings.Join(crafted, "/"))
		}
		gemOffset := modifierOffset + 2*pairs + 2
		if gemOffset < len(fields) {
			gemCount := fields[gemOffset]
			if gemCount < 0 || gemCount > 16 || len(fields) < gemOffset+1+gemCount {
				return "", fmt.Errorf("loot gem bonuses are incomplete")
			}
			if gemCount > 0 {
				bonuses := []string{}
				for _, value := range fields[gemOffset+1 : gemOffset+1+gemCount] {
					bonuses = append(bonuses, strconv.Itoa(value))
				}
				options = append(options, "gem_bonus_id="+strings.Join(bonuses, "/"))
			}
		}
	}
	// Exact client preview level, not a synthesized season/upgrade-track table.
	options = append(options, fmt.Sprintf("ilevel=%d", item.Level))
	return itemValue(strings.Join(options, ","))
}
