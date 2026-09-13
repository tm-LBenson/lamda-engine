package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const mirror = `9/13/2026 01:35:24.767-4  SPELL_CAST_SUCCESS,Player-1-ABCD,"Friend-Realm",0x512,0x0,0000000000000000,nil,0x80000000,0x0,55342,"Mirror Image",0x40`

func TestParseParty(t *testing.T) {
	c, e := Parse(mirror)
	if e != nil || c.Spell != 55342 || c.At.UTC().Hour() != 5 {
		t.Fatalf("%+v %v", c, e)
	}
	for _, line := range []string{strings.Replace(mirror, "0x512", "0x511", 1), strings.Replace(mirror, "Player-1-ABCD", "Vehicle-1-ABCD", 1), strings.Replace(mirror, "SPELL_CAST_SUCCESS", "SPELL_AURA_APPLIED", 1), "partial"} {
		if _, e := Parse(line); e == nil {
			t.Fatal("accepted non-party cast", line)
		}
	}
}
func TestModelDelayDuplicateAndExpiry(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	now := c.At.Add(10 * time.Second)
	if !m.Observe(c, now) || m.Observe(c, now) {
		t.Fatal("dedup")
	}
	rows := m.Active(now)
	if len(rows) != 1 || rows[0].Ends-float64(now.UnixMilli())/1000 != 110 {
		t.Fatal(rows)
	}
	if len(m.Active(c.At.Add(121*time.Second))) != 0 {
		t.Fatal("expiry")
	}
	if m.Observe(c, c.At.Add(200*time.Second)) || m.Observe(c, c.At.Add(-5*time.Second)) {
		t.Fatal("stale/future cast")
	}
}
func TestIndependentPlayers(t *testing.T) {
	a, _ := Parse(mirror)
	b := a
	b.GUID = "Player-2"
	m := NewModel()
	m.Observe(a, a.At)
	m.Observe(b, b.At)
	if len(m.Active(a.At)) != 2 {
		t.Fatal("merged players")
	}
}
func TestTailPartialRotationTruncation(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "WoWCombatLog-a.txt")
	os.WriteFile(p, []byte("old\n"), 0600)
	tail := &Tail{}
	if l, _ := tail.Poll(d); len(l) != 0 {
		t.Fatal("replayed history")
	}
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("new")
	f.Close()
	if l, _ := tail.Poll(d); len(l) != 0 {
		t.Fatal("partial")
	}
	f, _ = os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("\n")
	f.Close()
	if l, _ := tail.Poll(d); len(l) != 1 || l[0] != "new" {
		t.Fatal(l)
	}
	os.WriteFile(p, []byte("x\n"), 0600)
	if l, _ := tail.Poll(d); len(l) != 1 || l[0] != "x" {
		t.Fatal(l)
	}
	q := filepath.Join(d, "WoWCombatLog-b.txt")
	os.WriteFile(q, []byte("rotated\n"), 0600)
	future := time.Now().Add(time.Second)
	os.Chtimes(q, future, future)
	if l, _ := tail.Poll(d); len(l) != 1 || l[0] != "rotated" {
		t.Fatal(l)
	}
}
func TestConfigIsDataOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lamdaUI.lua")
	data := `LamdaUIDB = { ["unrelated"] = true }
LamdaEngineDB = {
["schema"] = 1,
["cdEnabled"] = false,
["checkUpdates"] = true,
["notifyUpdates"] = false,
["checkDays"] = 7,
["overlayX"] = 200,
["overlayY"] = 320,
["overlayScale"] = 1.5,
}`
	os.WriteFile(p, []byte(data), 0600)
	c, e := ReadConfig(p)
	if e != nil || c.Enabled || c.Days != 7 || c.Scale != 1.5 {
		t.Fatal(c, e)
	}
	for _, bad := range []string{strings.TrimSuffix(data, "}"), strings.Replace(data, "1.5", "99", 1), strings.Replace(data, "[\"schema\"] = 1", "[\"schema\"] = 9", 1), "os.execute('invalid')"} {
		os.WriteFile(p, []byte(bad), 0600)
		if _, e := ReadConfig(p); e == nil {
			t.Fatal("accepted invalid settings")
		}
	}
}
