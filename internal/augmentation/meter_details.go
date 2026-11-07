package augmentation

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

const maxDetailEntries = 16384
const maxDetailHistoryEntries = 65536

func (s *Store) hasMeterDetails(m DamageMeter) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.Metric != 0 && m.Metric != 1 {
		return false
	}
	if _, ok := s.meterDetails[m.Owner+"/"+m.ID]; ok {
		return true
	}
	for _, seg := range s.active {
		if fmt.Sprintf("%s/%s/%d", seg.kind, seg.id, seg.start.UnixMilli()) == m.ID {
			return true
		}
	}
	return false
}

type damageDetailKey struct {
	source, target, credit string
	spell                  int
	original               int
}

type detailHitKey struct{ source, target, event string }
type detailHit struct {
	key       detailHitKey
	at        time.Time
	signature [32]byte
	spell     int
	name      string
}

// These are recorded log inputs, not inferred spell scaling. Keeping them out
// of State avoids copying large spell/target lists into every UI heartbeat.
type DamageDetailEntry struct {
	Source         string `json:"source"`
	ResolvedSource string `json:"resolvedSource,omitempty"`
	SourceName     string `json:"sourceName"`
	TargetName     string `json:"targetName"`
	SupportOwner   string `json:"supportOwner,omitempty"`
	SpellID        int    `json:"spellID"`
	SpellName      string `json:"spellName"`
	OriginalSpell  int    `json:"originalSpell,omitempty"`
	OriginalName   string `json:"originalName,omitempty"`
	Amount         int64  `json:"amount"`
}

// Support records name the buff, not the attack it enhanced. Only associate
// them with a preceding attack when the event family, source, target, advanced
// unit snapshot and damage flags agree. A millisecond timestamp rollover is
// present in the recorded key. Unmatched hooks remain explicit adjustments;
// they must never be subtracted from an unrelated spell with the buff's ID.
func detailSignature(fields []string, base int) [32]byte {
	return sha256.Sum256([]byte(strings.Join(fields[base:base+19], "\x00") + "\x01" + strings.Join(fields[base+21:base+29], "\x00")))
}

func (s *Store) observeDetailHit(fields []string) {
	base, support, ok := damagePayload(fields)
	if fields[0] == "SWING_DAMAGE_LANDED" {
		base, ok = 9, true // identity only: ordinary SWING_DAMAGE already counts it
	}
	if !ok || support || strings.HasSuffix(fields[0], "_MISSED") || len(fields) < base+29 || !friendlyGroup(fields[3]) || flags(fields[7])&0x10 != 0 {
		return
	}
	spell, name := 1, "Melee"
	if base == 12 {
		var err error
		spell, err = strconv.Atoi(fields[9])
		if err != nil || spell < 1 || spell > 10000000 {
			return
		}
		name = summaryText(fields[10], 100)
	}
	if s.detailHits == nil || len(s.detailHits) >= 4096 {
		s.detailHits = map[detailHitKey]detailHit{}
	}
	key := detailHitKey{fields[1], fields[5], fields[0]}
	hit, exists := s.detailHits[key]
	if exists {
		key = hit.key
	} else {
		key.source, key.target, key.event = strings.Clone(key.source), strings.Clone(key.target), strings.Clone(key.event)
	}
	if exists && hit.spell == spell {
		name = hit.name
	} else {
		name = strings.Clone(name)
	}
	s.detailHits[key] = detailHit{key: key, at: s.last, signature: detailSignature(fields, base), spell: spell, name: name}
}

type meterDetailRecord struct {
	Owner     string              `json:"owner"`
	Segment   string              `json:"segment"`
	EndedAt   time.Time           `json:"endedAt"`
	Truncated bool                `json:"truncated,omitempty"`
	Entries   []DamageDetailEntry `json:"entries"`
}

type MeterBreakdownRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	SpellID int    `json:"spellID,omitempty"`
	Amount  int64  `json:"amount"`
	Raw     int64  `json:"raw"`
	Given   int64  `json:"given"`
	Removed int64  `json:"removed"`
}

type MeterBreakdown struct {
	Segment  string              `json:"segment"`
	Actor    string              `json:"actor"`
	Spell    string              `json:"spell,omitempty"`
	View     string              `json:"view"`
	Expected int64               `json:"expected"`
	Total    int64               `json:"total"`
	Complete bool                `json:"complete"`
	Reason   string              `json:"reason,omitempty"`
	Rows     []MeterBreakdownRow `json:"rows"`
}

func (s *Store) trackMeterDetail(seg *segment, fields []string, amount int64, support bool, base int) {
	if seg.damage.details == nil {
		seg.damage.details = map[damageDetailKey]*DamageDetailEntry{}
	}
	id := 1
	name := "Melee"
	if base == 12 {
		var err error
		id, err = strconv.Atoi(fields[9])
		if err != nil || id < 0 || id > 10000000 {
			seg.damage.detailsTruncated = true
			return
		}
		name = fields[10]
	}
	credit := ""
	original, originalName := 0, ""
	if support {
		credit = fields[base+29]
		key := detailHitKey{fields[1], fields[5], strings.TrimSuffix(fields[0], "_SUPPORT")}
		if hit, ok := s.detailHits[key]; ok && !s.last.Before(hit.at) && s.last.Sub(hit.at) <= 2*time.Millisecond && hit.signature == detailSignature(fields, base) {
			original, originalName = hit.spell, hit.name
		}
	}
	// Native target views aggregate by unit name, not spawn GUID. This also
	// bounds repeated trash spawns without losing their damage contribution.
	target := summaryText(fields[6], 100)
	key := damageDetailKey{source: fields[1], target: target, credit: credit, spell: id, original: original}
	entry := seg.damage.details[key]
	if entry == nil {
		if len(seg.damage.details) >= maxDetailEntries {
			seg.damage.detailsTruncated = true
			return
		}
		key.source, key.target, key.credit = strings.Clone(key.source), strings.Clone(key.target), strings.Clone(key.credit)
		entry = &DamageDetailEntry{Source: key.source, SourceName: strings.Clone(summaryText(fields[2], 100)), TargetName: key.target, SupportOwner: key.credit, SpellID: id, SpellName: strings.Clone(summaryText(name, 100)), OriginalSpell: original, OriginalName: originalName}
		seg.damage.details[key] = entry
	}
	entry.Amount += amount
}

func (s *Store) rememberMeterDetails(seg *segment, meter DamageMeter) {
	if s.meterDetails == nil {
		s.meterDetails = map[string]meterDetailRecord{}
	}
	value := meterDetailRecord{Owner: meter.Owner, Segment: meter.ID, EndedAt: meter.EndedAt, Truncated: seg.damage.detailsTruncated, Entries: make([]DamageDetailEntry, 0, len(seg.damage.details))}
	for _, entry := range seg.damage.details {
		row := *entry
		row.ResolvedSource = s.sourceOwner(row.Source)
		value.Entries = append(value.Entries, row)
	}
	sort.Slice(value.Entries, func(i, j int) bool {
		a, b := value.Entries[i], value.Entries[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.SpellID != b.SpellID {
			return a.SpellID < b.SpellID
		}
		if a.OriginalSpell != b.OriginalSpell {
			return a.OriginalSpell < b.OriginalSpell
		}
		if a.TargetName != b.TargetName {
			return a.TargetName < b.TargetName
		}
		return a.SupportOwner < b.SupportOwner
	})
	s.meterDetails[summaryKey(meter)] = value
	s.trimMeterDetails()
	s.detailRevision++
}

func (s *Store) trimMeterDetails() {
	var keys []string
	for key := range s.meterDetails {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return s.meterDetails[keys[i]].EndedAt.After(s.meterDetails[keys[j]].EndedAt) })
	entries := 0
	for i, key := range keys {
		entries += len(s.meterDetails[key].Entries)
		if i >= 40 || entries > maxDetailHistoryEntries {
			delete(s.meterDetails, key)
		}
	}
}

func (s *Store) repairMeterDetailOwners() {
	for key, value := range s.meterDetails {
		var entries []DamageDetailEntry
		for i, entry := range value.Entries {
			if player(entry.Source) {
				continue
			}
			resolved := s.sourceOwner(entry.Source)
			if resolved == "" || resolved == entry.ResolvedSource {
				continue
			}
			if entries == nil {
				entries = append([]DamageDetailEntry{}, value.Entries...)
			}
			entries[i].ResolvedSource = resolved
		}
		if entries != nil {
			value.Entries = entries
			s.meterDetails[key] = value
			s.detailRevision++
		}
	}
}

func (s *Store) loadMeterDetails() {
	if s.path == "" {
		return
	}
	f, err := os.Open(filepath.Join(filepath.Dir(s.path), "meter-details.json"))
	if err != nil {
		return
	}
	defer f.Close()
	var data struct {
		Schema int                 `json:"schema"`
		Client string              `json:"client"`
		Scenes []meterDetailRecord `json:"scenes"`
	}
	if json.NewDecoder(io.LimitReader(f, 32<<20)).Decode(&data) != nil || data.Schema != 1 || data.Client != "retail" || len(data.Scenes) > 40 {
		return
	}
	for _, scene := range data.Scenes {
		if !player(scene.Owner) || len(scene.Owner) > 160 || scene.Segment == "" || len(scene.Segment) > 160 || len(scene.Entries) > maxDetailEntries {
			continue
		}
		valid := true
		var sum int64
		for _, entry := range scene.Entries {
			sum += entry.Amount
			if entry.Amount < 0 || entry.Amount > 1e15 || sum < 0 || sum > 1e16 || entry.SpellID < 0 || entry.SpellID > 10000000 || entry.OriginalSpell < 0 || entry.OriginalSpell > 10000000 || entry.Source == "" || len(entry.Source) > 160 || len(entry.ResolvedSource) > 160 || (entry.ResolvedSource != "" && !player(entry.ResolvedSource)) || len(entry.TargetName) > 100 || len(entry.SourceName) > 100 || len(entry.SpellName) > 100 || len(entry.OriginalName) > 100 || len(entry.SupportOwner) > 160 || (entry.SupportOwner != "" && !player(entry.SupportOwner)) {
				valid = false
				break
			}
		}
		if valid {
			s.meterDetails[scene.Owner+"/"+scene.Segment] = scene
		}
	}
	s.trimMeterDetails()
}

// Background persistence only. Neither drawing nor Share waits for this I/O.
func (s *Store) flushMeterDetails() error {
	s.detailIO.Lock()
	defer s.detailIO.Unlock()
	s.mu.Lock()
	if s.path == "" || s.detailRevision == s.detailSaved {
		s.mu.Unlock()
		return nil
	}
	revision := s.detailRevision
	data := struct {
		Schema int                 `json:"schema"`
		Client string              `json:"client"`
		Scenes []meterDetailRecord `json:"scenes"`
	}{Schema: 1, Client: "retail", Scenes: []meterDetailRecord{}}
	for _, value := range s.meterDetails {
		data.Scenes = append(data.Scenes, value)
	}
	path := filepath.Join(filepath.Dir(s.path), "meter-details.json")
	s.mu.Unlock()
	sort.Slice(data.Scenes, func(i, j int) bool { return data.Scenes[i].EndedAt.After(data.Scenes[j].EndedAt) })
	if err := engine.WriteJSON(path, data); err != nil {
		s.mu.Lock()
		if s.detailError != err.Error() && s.debug != nil {
			s.debug.write(map[string]any{"kind": "meter_details_save_error", "error": err.Error()})
		}
		s.detailError = err.Error()
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.detailSaved = revision
	s.detailError = ""
	s.mu.Unlock()
	return nil
}

// DamageBreakdown exposes recorded spell/target attribution. The sum must match
// its parent recorded actor; missing/over-transferred inputs stay incomplete.
func (s *Store) DamageBreakdown(owner, segment, actor, spell string) (MeterBreakdown, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := MeterBreakdown{Segment: segment, Actor: actor, Spell: spell, View: "spells", Rows: []MeterBreakdownRow{}}
	if !player(owner) || !player(actor) || len(owner) > 160 || len(actor) > 160 || len(segment) > 160 || len(spell) > 200 {
		return result, fmt.Errorf("invalid meter selection")
	}
	var parent *DamageMeter
	cache, ok := s.meterDetails[owner+"/"+segment]
	for _, meter := range s.damageMeters {
		if meter.Owner == owner && meter.ID == segment && meter.Metric == 0 {
			copy := meter
			parent = &copy
			break
		}
	}
	if parent == nil && s.own == owner {
		for _, active := range s.active {
			view := s.damageView(active, s.last, true)
			if view.ID != segment {
				continue
			}
			parent = &view
			cache = meterDetailRecord{Owner: owner, Segment: segment, Truncated: active.damage.detailsTruncated}
			for _, entry := range active.damage.details {
				cache.Entries = append(cache.Entries, *entry)
			}
			ok = true
			break
		}
	}
	if parent == nil {
		return result, fmt.Errorf("recorded session unavailable")
	}
	known := false
	for _, row := range parent.Rows {
		if row.GUID == actor {
			result.Expected = row.Adjusted
			known = true
			break
		}
	}
	if !ok || !known {
		return result, fmt.Errorf("spell details are not retained for this session")
	}
	groups := map[string]*MeterBreakdownRow{}
	spellTotals := map[string]int64{}
	for _, entry := range cache.Entries {
		source := s.sourceOwner(entry.Source)
		if source == "" {
			source = entry.ResolvedSource
		}
		if source == "" {
			result.Reason = "unresolved_source"
			continue
		}
		id := "raw/" + strconv.Itoa(entry.SpellID)
		displaySpell := entry.SpellID
		name := entry.SpellName
		raw, given, removed := int64(0), int64(0), int64(0)
		if entry.SupportOwner == "" && source == actor {
			raw = entry.Amount
		} else if entry.SupportOwner == actor && source != actor {
			id = "support/" + source + "/" + strconv.Itoa(entry.SpellID)
			sourceName := entry.SourceName
			for _, row := range parent.Rows {
				if row.GUID == source {
					sourceName = row.Name
					break
				}
			}
			name = entry.SpellName + " · " + strings.Split(sourceName, "-")[0]
			given = entry.Amount
		} else if source == actor && entry.SupportOwner != "" && entry.SupportOwner != actor {
			removed = entry.Amount
			if entry.OriginalSpell > 0 {
				id = "raw/" + strconv.Itoa(entry.OriginalSpell)
				name = entry.OriginalName
				displaySpell = entry.OriginalSpell
			} else {
				id = "credit/" + strconv.Itoa(entry.SpellID)
				name = entry.SpellName + " credit"
				result.Reason = "unlinked_support_spell"
			}
		} else {
			continue
		}
		amount := raw + given - removed
		spellTotals[id] += amount
		if spell != "" {
			if id != spell {
				continue
			}
			id, name = entry.TargetName, entry.TargetName
			result.View = "targets"
		}
		row := groups[id]
		if row == nil {
			row = &MeterBreakdownRow{ID: id, Name: name, SpellID: displaySpell}
			groups[id] = row
		}
		row.Amount += amount
		row.Raw += raw
		row.Given += given
		row.Removed += removed
	}
	var total int64
	for id, amount := range spellTotals {
		total += amount
		if amount < 0 && !strings.HasPrefix(id, "credit/") {
			result.Reason = "support_exceeds_spell_raw"
		}
	}
	result.Complete = parent.Valid && !parent.Partial && !parent.Provisional && !cache.Truncated && total == result.Expected && result.Reason == ""
	if !result.Complete && result.Reason == "" {
		result.Reason = "incomplete_recorded_details"
	}
	if spell != "" {
		var found bool
		result.Expected, found = spellTotals[spell]
		if !found {
			return result, fmt.Errorf("spell selection unavailable")
		}
	}
	for _, row := range groups {
		result.Rows = append(result.Rows, *row)
		result.Total += row.Amount
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		if result.Rows[i].Amount == result.Rows[j].Amount {
			return result.Rows[i].ID < result.Rows[j].ID
		}
		return result.Rows[i].Amount > result.Rows[j].Amount
	})
	if result.Total != result.Expected {
		result.Complete = false
		result.Reason = "detail_total_mismatch"
	}
	s.debugBreakdown(result)
	return result, nil
}
