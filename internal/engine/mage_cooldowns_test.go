package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestReplaySevenMissingMageCastsFromNovemberFirstArenas(t *testing.T) {
	var casts []struct {
		MatchStart, At, GUID, Name string
		Spell                      int
	}
	raw, err := os.ReadFile("testdata/arena_mage_20261101.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &casts); err != nil {
		t.Fatal(err)
	}
	if len(casts) != 7 {
		t.Fatal("incomplete log fixture")
	}
	m := NewModel()
	lastMatch := ""
	for _, c := range casts {
		at, _ := time.Parse(time.RFC3339Nano, c.At)
		if c.MatchStart != lastMatch {
			m = NewModel()
			mageBuild(m, at)
			lastMatch = c.MatchStart
		}
		m.ObservePartyContext(lookup.PartyContext{Arena: true, At: at.Truncate(time.Second), Epoch: c.MatchStart}, at)
		stamp := at.In(time.FixedZone("EST", -5*3600)).Format("1/2/2006 15:04:05.000") + "-5"
		line := fmt.Sprintf("%s  SPELL_CAST_SUCCESS,%s,\"Fixturemage-Example-US\",0x512,0x0,0000000000000000,nil,0x80000000,0x0,%d,\"%s\",0x40", stamp, c.GUID, c.Spell, c.Name)
		if !m.IngestWithRoster(line, at.Add(250*time.Millisecond), true, map[string]bool{c.GUID: true}) {
			t.Fatal("previously missing cast remains missing", c)
		}
		key := fmt.Sprintf("%s/%d", c.GUID, CooldownSpell(c.Spell))
		if e := m.Evidence[key]; e.Name != c.Name || e.CooldownSeconds <= 0 {
			t.Fatal(c, e)
		}
	}
}

func mageBuild(m *Model, now time.Time) BuildSnapshot {
	b := BuildSnapshot{GUID: "Player-1-00000002", Player: "Fixturemage", Source: "inspect", At: now.Add(-time.Second), SpecID: 64, GameBuild: "12.1.5",
		SpellRanks: map[int]int{45438: 1, 414658: 1, 342245: 1, 342249: 2, 382424: 2, 1244110: 1, 1265517: 1}, Talents: [][3]int{{1, 1, 1}}}
	m.ObserveInspectedBuild(b, now)
	return m.InspectedBuilds[b.GUID]
}

func TestMageVariantHasOneSharedQueueAndColdSnapResetsIt(t *testing.T) {
	now := time.Unix(1793573400, 0)
	m := NewModel()
	b := mageBuild(m, now)
	m.ObservePartyContext(lookup.PartyContext{Arena: true, At: now, Epoch: "round:1"}, now)
	var audit []TrackingEvent
	m.TrackingAudit = func(e TrackingEvent) { audit = append(audit, e) }
	for _, offset := range []time.Duration{time.Second, 32 * time.Second} {
		at := now.Add(offset)
		m.ObservePartyContext(lookup.PartyContext{Arena: true, At: at, Epoch: "round:1"}, at)
		m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: at, Source: "unit_spellcast"}, at.Add(250*time.Millisecond))
		m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: at}, at.Add(90*time.Second))
	}
	e := m.Evidence[b.GUID+"/45438"]
	if len(m.Evidence) != 1 || e.DisplaySpell != 414658 || e.Name != "Ice Cold" || e.CooldownSeconds != 150 ||
		e.MaxCharges == nil || *e.MaxCharges != 2 || len(e.RechargeEnds) != 2 || e.RechargeEnds[1]-e.RechargeEnds[0] != 150 || m.Diagnostics.AcceptedCasts != 2 {
		t.Fatal("variant, PvP rules, duplicate delivery or charge queue", e, m.Diagnostics)
	}
	if audit[0].Spell != 414658 || audit[0].Canonical != 45438 || audit[0].Evidence == nil {
		t.Fatal(audit)
	}
	at := now.Add(40 * time.Second)
	m.Observe(Cast{GUID: b.GUID, Spell: 235219, At: at, Source: "unit_spellcast"}, at)
	e = m.Evidence[b.GUID+"/45438"]
	if e.Kind != "reset" || e.DisplaySpell != 414658 || len(e.RechargeEnds) != 0 {
		t.Fatal(e)
	}
}

func TestMagePvEAndUnknownContextDoNotAssumePvPRules(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	for _, arena := range []bool{false, true} {
		m := NewModel()
		b := mageBuild(m, now)
		m.ObservePartyContext(lookup.PartyContext{Arena: arena, At: now, Epoch: "test:1"}, now)
		m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now}, now)
		want := 120.0
		if arena {
			want = 150
		}
		if got := m.Evidence[b.GUID+"/45438"].CooldownSeconds; got != want {
			t.Fatal(arena, got)
		}
	}
	m := NewModel()
	b := mageBuild(m, now)
	m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now}, now)
	e := m.Evidence[b.GUID+"/45438"]
	if e.CooldownSeconds != 180 || e.PvP != nil || !slices.Contains(e.Reasons, "pvp_context_unknown") {
		t.Fatal(e)
	}
}

func TestMageContextAcceptsMeasuredServerClockSkewWithoutRejectingCurrentCast(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	for _, skew := range []time.Duration{-3 * time.Second, 0, 3 * time.Second, 5 * time.Second} {
		m := NewModel()
		b := mageBuild(m, now)
		m.ObservePartyContext(lookup.PartyContext{Arena: true, At: now.Add(skew), Epoch: "round:1"}, now)
		m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now, Source: "unit_spellcast"}, now.Add(200*time.Millisecond))
		if e := m.Evidence[b.GUID+"/45438"]; e.CooldownSeconds != 150 || e.PvP == nil || !*e.PvP {
			t.Fatal(skew, e)
		}
	}
}

func TestMageNativeOnsetsQueueChargesAndLogConfirmationDoesNotDoubleSpend(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m := NewModel()
	b := mageBuild(m, now)
	for _, offset := range []time.Duration{time.Second, 32 * time.Second} {
		at := now.Add(offset)
		m.ObservePartyContext(lookup.PartyContext{Arena: true, At: at, Epoch: "round:1"}, at)
		m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: at, Source: "aura_activation"}, at.Add(250*time.Millisecond))
		m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: at.Add(-100 * time.Millisecond)}, at.Add(time.Second))
	}
	e := m.Evidence[b.GUID+"/45438"]
	if e.Source != "combat_log" || e.DisplaySpell != 414658 || len(e.RechargeEnds) != 2 || e.RechargeEnds[1]-e.RechargeEnds[0] != 150 || m.Diagnostics.ReconciledAuraCasts != 2 {
		t.Fatal(e, m.Diagnostics)
	}
}

func TestAlterTimeReturnNeverStartsAnotherCooldown(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m := NewModel()
	b := mageBuild(m, now)
	line := func(spell int, at time.Time) string {
		stamp := at.In(time.FixedZone("EST", -5*3600)).Format("1/2/2006 15:04:05.000") + "-5"
		return fmt.Sprintf("%s  SPELL_CAST_SUCCESS,%s,\"Fixturemage-Example-US\",0x512,0x0,0000000000000000,nil,0x80000000,0x0,%d,\"Alter Time\",0x40", stamp, b.GUID, spell)
	}
	if !m.IngestWithRoster(line(342245, now), now.Add(200*time.Millisecond), true, map[string]bool{b.GUID: true}) {
		t.Fatal("start missing")
	}
	before := m.Evidence[b.GUID+"/342245"]
	m.IngestWithRoster(line(342247, now.Add(3*time.Second)), now.Add(3*time.Second), true, map[string]bool{b.GUID: true})
	after := m.Evidence[b.GUID+"/342245"]
	if before.CooldownSeconds != 50 || after.EstimatedEnd != before.EstimatedEnd || m.Diagnostics.AcceptedCasts != 1 {
		t.Fatal(before, after)
	}
}

func TestMageDisplayUsesVariantAndExistingIceBlockSelection(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m := NewModel()
	b := mageBuild(m, now)
	known := knownCooldowns(m.Snapshot(now), now)
	var count int
	for _, e := range known {
		if e.Spell == 45438 {
			count++
			if e.DisplaySpell != 414658 {
				t.Fatal(e)
			}
		}
	}
	if count != 1 {
		t.Fatal(known)
	}
	m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now}, now)
	g := frameFixture(now)
	g.Frames[0].GUID = b.GUID
	g.Persistent, g.Unknown = true, true
	c := DefaultConfig()
	c.FrameEstimates = true
	c.FrameSpells = []int{45438}
	d := FrameView(m.Snapshot(now), c, g, now, now, 123)
	count = 0
	for _, i := range d.Icons {
		if i.GUID == b.GUID {
			count++
			if i.Spell != 414658 || i.Name != "Ice Cold" {
				t.Fatal(i)
			}
		}
	}
	if count != 1 {
		t.Fatal(d)
	}
}

func TestNewArenaRoundDropsOldCooldownsAndRejectsDelayedOldCasts(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m := NewModel()
	b := mageBuild(m, now)
	m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now.Add(-time.Second)}, now)
	m.ObservePartyContext(lookup.PartyContext{Arena: true, At: now, Epoch: "round:1"}, now)
	if len(m.Evidence) != 0 {
		t.Fatal("old match survived")
	}
	m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now.Add(-10 * time.Second)}, now)
	if len(m.Evidence) != 0 {
		t.Fatal("old log revived a cooldown")
	}
	m.Observe(Cast{GUID: b.GUID, Spell: 414658, At: now.Add(time.Second)}, now.Add(time.Second))
	m.ObservePartyContext(lookup.PartyContext{Arena: true, At: now.Add(time.Second), Epoch: "round:1"}, now.Add(time.Second))
	if len(m.Evidence) != 1 {
		t.Fatal("same round reset live evidence")
	}
	m.ObservePartyContext(lookup.PartyContext{Arena: true, At: now.Add(20 * time.Second), Epoch: "round:2"}, now.Add(20*time.Second))
	if len(m.Evidence) != 0 {
		t.Fatal("next round retained previous cooldown")
	}
}
