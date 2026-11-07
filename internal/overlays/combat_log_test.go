package overlays

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func combatFixtures(t *testing.T) map[string][]string {
	t.Helper()
	b, e := os.ReadFile("testdata/combat-v22.json")
	if e != nil {
		t.Fatal(e)
	}
	var f map[string][]string
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	return f
}
func combatLine(f []string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(f)
	w.Flush()
	return "10/13/2026 00:00:00.000-4  " + strings.TrimSpace(b.String())
}
func TestCombatLogActualRetailLayouts(t *testing.T) {
	fs := combatFixtures(t)
	format := combatFormat{}
	if !format.Observe(fs["COMBAT_LOG_VERSION"]) || !format.Known || !format.Advanced {
		t.Fatal(format)
	}
	tests := []struct {
		raw, kind, key string
		value          float64
	}{
		{"SPELL_DAMAGE", "damage", "amount", 6134}, {"SPELL_DAMAGE", "damage", "baseAmount", 2947}, {"SPELL_DAMAGE", "damage", "overkill", 0}, {"SPELL_DAMAGE", "damage", "effective", 6134}, {"SPELL_DAMAGE", "damage", "critical", 1},
		{"SWING_DAMAGE", "damage", "amount", 5226}, {"SWING_DAMAGE", "damage", "critical", 0},
		{"SPELL_HEAL", "heal", "amount", 44798}, {"SPELL_HEAL", "heal", "overheal", 44798}, {"SPELL_HEAL", "heal", "effective", 0}, {"SPELL_HEAL", "heal", "critical", 1},
		{"SPELL_ENERGIZE", "energize", "amount", 8}, {"SPELL_ENERGIZE", "energize", "power", 17},
	}
	for _, test := range tests {
		e, ok := parseCombatRecord(combatLine(fs[test.raw]), format)
		if !ok || e.Kind != test.kind {
			t.Fatal(test, e, ok)
		}
		v, ok := e.Values[test.key]
		if !ok || v != test.value {
			t.Fatal(test, e.Values)
		}
	}
	for _, kind := range []string{"SPELL_INTERRUPT", "SPELL_DISPEL", "SPELL_AURA_APPLIED", "SPELL_AURA_REMOVED", "SPELL_AURA_APPLIED_DOSE", "SPELL_AURA_REMOVED_DOSE", "SPELL_MISSED", "UNIT_DIED"} {
		e, ok := parseCombatRecord(combatLine(fs[kind]), format)
		if !ok {
			t.Fatal(kind)
		}
		if (e.Kind == "interrupt" || e.Kind == "dispel") && (e.ExtraSpell == 0 || e.ExtraName == "") {
			t.Fatal("missing affected spell", e)
		}
		if e.Kind == "miss" && e.Miss != "DODGE" {
			t.Fatal(e)
		}
		if e.Kind == "aurastacks" {
			if _, ok := e.Values["stacks"]; !ok {
				t.Fatal("missing explicit count", e)
			}
		}
	}
}
func TestCombatPayloadMissingAndUnsupportedStayUnknown(t *testing.T) {
	f := combatFixtures(t)["SPELL_DAMAGE"]
	format := combatFormat{Known: true, Version: 22, Advanced: true}
	for _, missing := range []combatFormat{{}, {Known: true, Version: 23, Advanced: true}, {Known: true, Version: 22, Advanced: false}} {
		e, ok := parseCombatRecord(combatLine(f), missing)
		if !ok || len(e.Values) != 0 || e.Critical != nil || e.Spell == 0 {
			t.Fatal("unknown format invented readings", e, ok)
		}
	}
	short := append([]string(nil), f[:len(f)-2]...)
	e, ok := parseCombatRecord(combatLine(short), format)
	if !ok || len(e.Values) != 0 {
		t.Fatal("short payload gave values", e)
	}
	bad := append([]string(nil), f...)
	bad[31] = "NaN"
	e, ok = parseCombatRecord(combatLine(bad), format)
	if _, known := e.Values["amount"]; !ok || known {
		t.Fatal("invalid amount known", e)
	}
	bad[31] = "0"
	e, ok = parseCombatRecord(combatLine(bad), format)
	if amount, known := e.Values["amount"]; !ok || !known || amount != 0 {
		t.Fatal("real zero became unknown", e)
	}
	bad[38] = "unreadable"
	e, _ = parseCombatRecord(combatLine(bad), format)
	if e.Critical != nil {
		t.Fatal("unknown critical became false")
	}
	bad[1] = "run()"
	if _, ok = parseCombatRecord(combatLine(bad), format); ok {
		t.Fatal("invalid identity accepted")
	}
	for _, kind := range []string{"SWING_DAMAGE_LANDED", "SPELL_DAMAGE_SUPPORT", "UNKNOWN_EVENT"} {
		bad = append([]string(nil), f...)
		bad[0] = kind
		if _, ok = parseCombatRecord(combatLine(bad), format); ok {
			t.Fatal("duplicate/unsupported damage counted", kind)
		}
	}
}
func TestCombatFormatResetAndNonAdvancedShape(t *testing.T) {
	fs := combatFixtures(t)
	format := combatFormat{}
	format.Observe(fs["COMBAT_LOG_VERSION"])
	f := fs["SPELL_HEAL"]
	simple := append(append([]string(nil), f[:12]...), f[31:]...)
	format.Advanced = false
	e, ok := parseCombatRecord(combatLine(simple), format)
	if !ok || e.Values["amount"] != 44798 || e.Values["effective"] != 0 {
		t.Fatal(e)
	}
	format.Observe([]string{"COMBAT_LOG_VERSION", "22", "wrong"})
	if format.Known {
		t.Fatal("invalid header retained old metadata")
	}
	if format.Observe([]string{"ZONE_CHANGE", "100"}) {
		t.Fatal("unrelated header treated as format")
	}
}
