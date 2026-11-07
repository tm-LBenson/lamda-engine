package overlays

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

// Disk records have different offsets from the historical in-addon CLEU API.
// Numeric payloads below are verified against Retail v22 logs; unsupported
// layouts retain their event identity but never acquire invented measurements.
type combatFormat struct {
	Version  int
	Advanced bool
	Build    string
	Known    bool
}
type combatActor struct {
	GUID, Name string
	Flags      uint64
}
type combatEvent struct {
	At             time.Time
	Kind, RawKind  string
	Source, Target combatActor
	Spell          int
	Name           string
	School         int
	ExtraSpell     int
	ExtraName      string
	ExtraSchool    int
	AuraKind, Miss string
	Values         map[string]float64
	Critical       *bool
}

var combatGUID = regexp.MustCompile(`^(Player|Creature|Pet|Vehicle|GameObject)-[A-Za-z0-9-]{1,110}$`)

func knownCombatGUID(s string) bool { return combatGUID.MatchString(s) }
func combatName(s string) string {
	if s == "nil" || len(s) > 256 || !utf8.ValidString(s) {
		return ""
	}
	return s
}
func (f *combatFormat) Observe(fields []string) bool {
	if len(fields) == 0 || fields[0] != "COMBAT_LOG_VERSION" {
		return false
	}
	*f = combatFormat{}
	if len(fields) < 8 || fields[2] != "ADVANCED_LOG_ENABLED" || fields[4] != "BUILD_VERSION" || fields[6] != "PROJECT_ID" || fields[7] != "1" {
		return true
	}
	version, e := strconv.Atoi(fields[1])
	if e != nil || version < 1 || version > 100 || !one(fields[3], "0", "1") || len(fields[5]) > 32 {
		return true
	}
	f.Version, f.Advanced, f.Build, f.Known = version, fields[3] == "1", fields[5], true
	return true
}
func combatNumber(s string, lo, hi float64) *float64 {
	v, e := strconv.ParseFloat(s, 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < lo || v > hi {
		return nil
	}
	return &v
}
func combatInt(s string, lo, hi int) (int, bool) {
	v, e := strconv.ParseInt(s, 0, 32)
	return int(v), e == nil && v >= int64(lo) && v <= int64(hi)
}
func combatBool(s string) *bool {
	if !one(s, "nil", "0", "1", "false", "true") {
		return nil
	}
	b := s == "1" || s == "true"
	return &b
}
func parseCombatRecord(line string, format combatFormat) (combatEvent, bool) {
	at, f, err := engine.RecordFields(line)
	if err != nil {
		return combatEvent{}, false
	}
	return parseCombatFields(at, f, format)
}
func parseCombatFields(at time.Time, f []string, format combatFormat) (combatEvent, bool) {
	if len(f) < 9 {
		return combatEvent{}, false
	}
	e := combatEvent{At: at, RawKind: f[0], Values: map[string]float64{}}
	switch f[0] {
	case "SPELL_DAMAGE", "SPELL_PERIODIC_DAMAGE", "RANGE_DAMAGE", "DAMAGE_SHIELD", "DAMAGE_SPLIT", "SWING_DAMAGE", "ENVIRONMENTAL_DAMAGE":
		e.Kind = "damage"
	case "SPELL_HEAL", "SPELL_PERIODIC_HEAL":
		e.Kind = "heal"
	case "SPELL_INTERRUPT":
		e.Kind = "interrupt"
	case "SPELL_DISPEL", "SPELL_STOLEN":
		e.Kind = "dispel"
	case "SPELL_DISPEL_FAILED":
		e.Kind = "dispelfailed"
	case "SPELL_AURA_APPLIED":
		e.Kind = "auraapply"
	case "SPELL_AURA_REMOVED":
		e.Kind = "auraremove"
	case "SPELL_AURA_REFRESH":
		e.Kind = "aurarefresh"
	case "SPELL_AURA_APPLIED_DOSE", "SPELL_AURA_REMOVED_DOSE":
		e.Kind = "aurastacks"
	case "SPELL_CAST_START":
		e.Kind = "logcaststart"
	case "SPELL_CAST_SUCCESS":
		e.Kind = "logcastsuccess"
	case "SPELL_CAST_FAILED":
		e.Kind = "logcastfailed"
	case "SPELL_ENERGIZE", "SPELL_PERIODIC_ENERGIZE":
		e.Kind = "energize"
	case "SPELL_MISSED", "SPELL_PERIODIC_MISSED", "RANGE_MISSED", "SWING_MISSED":
		e.Kind = "miss"
	case "UNIT_DIED":
		e.Kind = "death"
	case "UNIT_DESTROYED", "UNIT_DISSIPATES":
		e.Kind = "despawn"
	case "SPELL_SUMMON":
		e.Kind = "summon"
	case "SPELL_RESURRECT":
		e.Kind = "resurrect"
	default:
		return e, false // Parallel *_SUPPORT / *_LANDED records aren't new hits.
	}
	actor := func(index int) (combatActor, bool) {
		id, name := f[index], combatName(f[index+1])
		flags, err := strconv.ParseUint(f[index+2], 0, 32)
		if err != nil {
			return combatActor{}, false
		}
		if one(id, "", "nil", "0000000000000000") {
			id = ""
		} else if !knownCombatGUID(id) {
			return combatActor{}, false
		}
		return combatActor{id, name, flags}, true
	}
	var ok bool
	if e.Source, ok = actor(1); !ok {
		return e, false
	}
	if e.Target, ok = actor(5); !ok {
		return e, false
	}
	base := 9
	if strings.HasPrefix(f[0], "SPELL_") || strings.HasPrefix(f[0], "RANGE_") || one(f[0], "DAMAGE_SHIELD", "DAMAGE_SPLIT") {
		if len(f) < 12 {
			return e, false
		}
		e.Spell, ok = combatInt(f[9], 1, 9999999)
		if !ok {
			return e, false
		}
		e.Name = combatName(f[10])
		e.School, ok = combatInt(f[11], 0, 127)
		if !ok {
			return e, false
		}
		base = 12
	} else if f[0] == "ENVIRONMENTAL_DAMAGE" {
		if len(f) < 10 {
			return e, false
		}
		e.Name = combatName(f[9])
		base = 10
	}
	if one(e.Kind, "interrupt", "dispel", "dispelfailed") {
		if len(f) < base+3 {
			return e, false
		}
		e.ExtraSpell, ok = combatInt(f[base], 1, 9999999)
		if !ok {
			return e, false
		}
		e.ExtraName = combatName(f[base+1])
		e.ExtraSchool, ok = combatInt(f[base+2], 0, 127)
		if !ok {
			return e, false
		}
		if len(f) > base+3 && one(f[base+3], "BUFF", "DEBUFF") {
			e.AuraKind = strings.ToLower(f[base+3])
		}
	}
	if one(e.Kind, "auraapply", "auraremove", "aurarefresh", "aurastacks") {
		if len(f) <= base || !one(f[base], "BUFF", "DEBUFF") {
			return e, false
		}
		e.AuraKind = strings.ToLower(f[base])
		if e.Kind == "aurastacks" && len(f) > base+1 {
			if n := combatNumber(f[base+1], 0, 999); n != nil && math.Trunc(*n) == *n {
				e.Values["stacks"] = *n
			}
		}
	}
	if e.Kind == "miss" {
		if len(f) <= base {
			return e, false
		}
		if one(f[base], "ABSORB", "BLOCK", "DEFLECT", "DODGE", "EVADE", "IMMUNE", "MISS", "PARRY", "REFLECT", "RESIST") {
			e.Miss = f[base]
		}
	}
	if !one(e.Kind, "damage", "heal", "energize") {
		return e, true
	}
	if !format.Known || format.Version != 22 {
		return e, true
	}
	if format.Advanced {
		if len(f) < base+19 || !(knownCombatGUID(f[base]) || f[base] == "0000000000000000") || !(knownCombatGUID(f[base+1]) || f[base+1] == "0000000000000000") {
			return e, true
		}
		base += 19
	}
	fields := f[base:]
	set := func(key string, index int, lo float64) {
		if index < len(fields) {
			if v := combatNumber(fields[index], lo, 1e12); v != nil {
				e.Values[key] = *v
			}
		}
	}
	switch e.Kind {
	case "damage":
		if len(fields) != 10 && !(len(fields) == 11 && one(fields[10], "ST", "AOE")) {
			return e, true
		}
		set("amount", 0, 0)
		set("baseAmount", 1, 0)
		set("overkill", 2, -1)
		if v, ok := e.Values["overkill"]; ok && v < 0 {
			e.Values["overkill"] = 0
		}
		if school, ok := combatInt(fields[3], 0, 127); ok {
			e.School = school
		}
		set("resisted", 4, 0)
		set("blocked", 5, 0)
		set("absorbed", 6, 0)
		e.Critical = combatBool(fields[7])
	case "heal":
		if len(fields) != 5 {
			return e, true
		}
		set("amount", 0, 0)
		set("baseAmount", 1, 0)
		set("overheal", 2, 0)
		set("absorbed", 3, 0)
		e.Critical = combatBool(fields[4])
	case "energize":
		if len(fields) != 4 {
			return e, true
		}
		set("amount", 0, 0)
		set("wasted", 1, 0)
		if power, ok := combatInt(fields[2], 0, 31); ok {
			e.Values["power"] = float64(power)
		}
	}
	if amount, ok := e.Values["amount"]; ok {
		key := "overkill"
		if e.Kind == "heal" {
			key = "overheal"
		}
		if waste, known := e.Values[key]; known {
			e.Values["effective"] = math.Max(0, amount-waste)
		}
	}
	if e.Critical != nil {
		if *e.Critical {
			e.Values["critical"] = 1
		} else {
			e.Values["critical"] = 0
		}
	}
	return e, true
}
