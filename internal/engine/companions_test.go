package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const barrier = `9/13/2026 11:35:00.000-4  SPELL_CAST_SUCCESS,Vehicle-0-4222-2813-1-209059-ABCD,"Meredy Huntswell",0xa12,0x0,0000000000000000,nil,0x0,0x0,295238,"Blazing Barrier",0x4`

func TestCompanionSourceFiltering(t *testing.T) {
	c, e := Parse(barrier)
	if e != nil || !c.Companion {
		t.Fatal(c, e)
	}
	valeera := strings.NewReplacer("Vehicle-0-4222-2813-1-209059-ABCD", "Creature-0-4222-2813-1-248567-ABCD", "295238", "1248109", "0xa12", "0x2111").Replace(barrier)
	if c, e := Parse(valeera); e != nil || !c.Companion {
		t.Fatal(c, e)
	}
	for _, bad := range []string{strings.Replace(barrier, "0xa12", "0xa48", 1), strings.Replace(barrier, "0xa12", "0xa18", 1), strings.Replace(barrier, "209059", "999999", 1), strings.Replace(barrier, "295238", "55342", 1), strings.Replace(barrier, "SPELL_CAST_SUCCESS", "SPELL_AURA_APPLIED", 1)} {
		if _, e := Parse(bad); e == nil {
			t.Fatal("unsupported NPC accepted", bad)
		}
	}
}

func TestCompanionHistoryBoundDoesNotEvictOnExistingCast(t *testing.T) {
	c, _ := Parse(barrier)
	m := NewModel()
	for i := 0; i < 256; i++ {
		c.GUID = fmt.Sprintf("Vehicle-0-4222-2813-1-209059-%d", i)
		m.Observe(c, c.At)
	}
	c.At = c.At.Add(25 * time.Second)
	m.Observe(c, c.At)
	if len(m.CompanionHistory) != 256 {
		t.Fatal("updating an existing NPC evicted a different NPC", len(m.CompanionHistory))
	}
	c.GUID = "Vehicle-0-4222-2813-1-209059-new"
	m.Observe(c, c.At)
	if len(m.CompanionHistory) != 256 {
		t.Fatal("history cap exceeded", len(m.CompanionHistory))
	}
	m.Active(c.At.Add(observationRetention + time.Second))
	if len(m.CompanionHistory) != 0 {
		t.Fatal("idle learning retained")
	}
}
func TestCompanionLearningAndPulse(t *testing.T) {
	c, _ := Parse(barrier)
	m := NewModel()
	now := c.At.Add(12 * time.Second)
	if !m.Observe(c, now) {
		t.Fatal("first cast")
	}
	r := m.Active(now)[0]
	if !r.ObservedOnly || !r.Companion || r.Ends != float64(now.UnixMilli())/1000+8 {
		t.Fatal(r)
	}
	if m.Observe(c, now) {
		t.Fatal("duplicate")
	}
	m.Active(now.Add(9 * time.Second))
	for i := 0; i < 2; i++ {
		c.At = c.At.Add(25 * time.Second)
		m.Observe(c, c.At.Add(12*time.Second))
	}
	r = m.Active(c.At.Add(12 * time.Second))[0]
	if r.ObservedOnly || r.Ends != float64(c.At.UnixMilli())/1000+25 {
		t.Fatal("expected learned NPC interval", r)
	}
	c.At = c.At.Add(10 * time.Second)
	m.Observe(c, c.At)
	if !m.Active(c.At)[0].ObservedOnly {
		t.Fatal("contradicted estimate retained")
	}
	// Learning cannot cross NPC identities or train the player catalog.
	other := c
	other.GUID = strings.Replace(other.GUID, "ABCD", "NEW", 1)
	m.Observe(other, other.At)
	if len(m.CompanionHistory) != 2 {
		t.Fatal("merged NPCs")
	}
	if Catalog[55342].Cooldown != 120 {
		t.Fatal("changed player catalog")
	}
	old := c
	old.At = c.At.Add(time.Second)
	if m.Observe(old, old.At.Add(31*time.Second)) {
		t.Fatal("stale companion pulse")
	}
}
