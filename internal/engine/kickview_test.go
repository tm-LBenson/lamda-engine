package engine

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDelayedInterruptDoesNotInventReadiness(t *testing.T) {
	start := time.Unix(1789500000, 0)
	for _, source := range []string{"", "unit_spellcast"} {
		for _, delay := range []time.Duration{12 * time.Second, 30 * time.Second} {
			m := NewModel()
			at := start.Add(delay)
			k := kickFixture(at)
			c := DefaultConfig()
			c.KickEnabled = true
			m.Observe(Cast{GUID: k.Members[1].GUID, Name: "Shaman", Spell: 57994, At: start, Source: source}, at)
			s := m.Snapshot(at)
			d := KickView(s, c, k, at, at, 123)
			bar := d.KickBars[1]
			if bar.Status != "unknown" || !slices.Contains(bar.Reasons, "arrived_after_estimated_end") || bar.Ends != float64(start.Unix()+12) {
				t.Fatalf("expired %q delivery became ready or restarted its timer: %+v", source, bar)
			}
			// A genuinely new readable cast must recover the same teammate row.
			at = at.Add(2 * time.Second)
			m.Observe(Cast{GUID: k.Members[1].GUID, Spell: 57994, At: at, Source: "unit_spellcast"}, at)
			k.At = at
			d = KickView(m.Snapshot(at), c, k, at, at, 123)
			if d.KickBars[1].Status != "estimated_cooldown" || d.KickBars[1].Ends != float64(at.Unix()+12) {
				t.Fatal("fresh cast did not recover countdown", d.KickBars[1])
			}
		}
	}
}

func TestLateInterruptWithRemainingTimeKeepsActualCastClock(t *testing.T) {
	start := time.Unix(1789500000, 0)
	m := NewModel()
	k := kickFixture(start)
	c := DefaultConfig()
	c.KickEnabled = true
	m.Observe(Cast{GUID: k.Members[1].GUID, Spell: 57994, At: start}, start.Add(8*time.Second))
	for _, check := range []struct {
		seconds int
		status  string
	}{{8, "estimated_cooldown"}, {12, "estimated_ready"}, {73, "unknown"}} {
		at := start.Add(time.Duration(check.seconds) * time.Second)
		k.At = at
		d := KickView(m.Snapshot(at), c, k, at, at, 123)
		if bar := d.KickBars[1]; bar.Status != check.status || bar.Ends != float64(start.Unix()+12) {
			t.Fatal("delay restarted or discarded a valid countdown", bar)
		}
	}
}

func kickFixture(now time.Time) *framewire.KickSnapshot {
	return &framewire.KickSnapshot{At: now, ID: "1:1", Player: "Player-1-SELF", Width: 1600, Height: 1000, Mode: 3, Self: true,
		X: 450, Y: 120, BarWidth: 250, BarHeight: 30, Size: 28, Opacity: 95, Anchor: 4, Gap: 4, Sort: 1,
		Members: []framewire.KickMember{{GUID: "Player-1-SELF", Name: "Warrior", Class: 1, Frame: &framewire.Rect{X: 100, Y: 300, Width: 200, Height: 40}},
			{GUID: "Player-1-SHAMAN", Name: "Shaman", Class: 7, Spec: 264, Frame: &framewire.Rect{X: 100, Y: 350, Width: 200, Height: 40}}}}
}

func TestKickDisplaySurvivesServerClockRoundingWithoutExtendingCaptureLease(t *testing.T) {
	// The live strip advanced every 200 ms with server timestamps 1.89–2.91s
	// ahead of receipt. The old two-second limit hid 118 of 138 valid samples.
	// Walk both sides of each whole server second, as real GetServerTime does.
	start := time.Unix(1793314070, 0)
	c := DefaultConfig()
	c.KickEnabled = true
	for step := 0; step < 40; step++ {
		now := start.Add(time.Duration(step) * 200 * time.Millisecond)
		stamp := now.Add(2900 * time.Millisecond).Truncate(time.Second)
		k := kickFixture(stamp)
		d := KickView(InferenceSnapshot{}, c, k, now, now, 123)
		if !d.Enabled || d.KickPanel == nil || len(d.KickBars) != 2 {
			t.Fatalf("healthy layout blinked at %v (server ahead %v): %+v", now, stamp.Sub(now), d)
		}
		if d.Expires != float64(now.Add(FrameLifetime).UnixMilli())/1000 {
			t.Fatal("clock tolerance extended the physical capture lease")
		}
		if KickView(InferenceSnapshot{}, c, k, now, now.Add(FrameLifetime+time.Millisecond), 123).Enabled {
			t.Fatal("stopped capture retained a stale tracker")
		}
	}
	for _, offset := range []time.Duration{-FrameTimestampSkew, FrameTimestampSkew} {
		k := kickFixture(start.Add(offset))
		if !KickView(InferenceSnapshot{}, c, k, start, start, 123).Enabled {
			t.Fatal("valid clock tolerance boundary rejected", offset)
		}
		if offset < 0 {
			k.At = k.At.Add(-time.Millisecond)
		} else {
			k.At = k.At.Add(time.Millisecond)
		}
		if KickView(InferenceSnapshot{}, c, k, start, start, 123).Enabled {
			t.Fatal("out-of-range layout timestamp accepted", offset)
		}
	}
}
func TestKickModesIndependentUnknownAndCountdownLifecycle(t *testing.T) {
	now := time.Unix(1789500000, 0)
	k := kickFixture(now)
	c := DefaultConfig()
	c.KickEnabled = true
	c.FrameEstimates = false
	m := NewModel()
	for mode := 1; mode <= 3; mode++ {
		k.Mode = mode
		d := KickView(m.Snapshot(now), c, k, now, now, 123)
		if !d.Enabled || (len(d.Icons) > 0) != (mode != 2) || (len(d.KickBars) > 0) != (mode != 1) {
			t.Fatal(d)
		}
		for _, b := range d.KickBars {
			if b.Status != "unknown" || b.Ends != 0 {
				t.Fatal("join invented readiness", b)
			}
		}
	}
	k.Mode = 3
	if !m.Observe(Cast{GUID: k.Members[1].GUID, Name: "Shaman", Spell: 57994, At: now, Source: "unit_spellcast"}, now) {
		t.Fatal("cast missing")
	}
	for _, check := range []struct {
		seconds int
		status  string
	}{{1, "estimated_cooldown"}, {12, "estimated_ready"}, {74, "unknown"}} {
		at := now.Add(time.Duration(check.seconds) * time.Second)
		k.At = at
		d := KickView(m.Snapshot(at), c, k, at, at, 123)
		if d.KickBars[1].Status != check.status || d.Icons[1].Status != check.status {
			t.Fatal(d)
		}
	}
	k.At = now
	k.Self = false
	k.Members[1].Frame.Y = 500
	d := KickView(m.Snapshot(now), c, k, now, now, 123)
	if len(d.Icons) != 1 || d.Icons[0].GUID != k.Members[1].GUID || d.Icons[0].Y != 506 {
		t.Fatal("identity or self toggle ignored", d)
	}
	k.Members = k.Members[:1]
	d = KickView(m.Snapshot(now), c, k, now, now, 123)
	if len(d.KickBars) != 0 || len(d.Icons) != 0 {
		t.Fatal("departed player retained", d)
	}
	if KickView(m.Snapshot(now), c, k, now, now.Add(2*time.Second), 123).Enabled {
		t.Fatal("stale geometry accepted")
	}
	k.Mode = 0
	if KickView(m.Snapshot(now), c, k, now, now, 123).Enabled {
		t.Fatal("context off ignored")
	}
}

func TestInterruptCatalogIsolationTalentsAliasesAndMissedInterrupt(t *testing.T) {
	now := time.Unix(1789500000, 0)
	m := NewModel()
	guid := "Player-1-SELF"
	m.ObserveInspectedBuild(BuildSnapshot{GUID: guid, Source: "inspect", At: now.Add(-time.Second), SpecID: 73, GameBuild: "12.1.0", SpellRanks: map[int]int{6552: 1, 391271: 1}}, now)
	m.Observe(Cast{GUID: guid, Spell: 6552, At: now, Source: "unit_spellcast"}, now)
	e := m.Snapshot(now).Cooldowns[0]
	if e.CooldownSeconds != 13.5 {
		t.Fatal("Honed Reflexes missing", e)
	}
	c := DefaultConfig()
	c.FrameEstimates = true
	k := kickFixture(now)
	g := &framewire.Snapshot{At: now, Width: 1600, Height: 1000, Player: guid, Self: true, Party: true, Unknown: true, Frames: []framewire.Rect{*k.Members[0].Frame}}
	g.Frames[0].GUID = guid
	if d := FrameView(m.Snapshot(now), c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal("interrupt leaked into LamdaCD", d)
	}
	line := fmt.Sprintf("%s+0  SPELL_CAST_SUCCESS,Player-1-SHAMAN,Shaman,0x512,0x0,0000000000000000,nil,0x80000000,0x0,57994,Wind Shear,0x1", now.UTC().Format("1/2/2006 15:04:05.000"))
	if !m.Ingest(line, now, false) {
		t.Fatal("successful use without SPELL_INTERRUPT must consume cooldown")
	}
	if m.Ingest(strings.Replace(line, "SPELL_CAST_SUCCESS", "SPELL_CAST_FAILED", 1), now, false) {
		t.Fatal("failed spellcast consumed cooldown")
	}
	for _, order := range [][]int{{19647, 119910}, {119910, 19647}} {
		model := NewModel()
		model.Observe(Cast{GUID: guid, Spell: order[0], At: now}, now)
		model.Observe(Cast{GUID: guid, Spell: order[1], At: now.Add(200 * time.Millisecond)}, now.Add(time.Second))
		if model.Snapshot(now).Cooldowns[0].EstimatedEnd != float64(now.Unix())+24 {
			t.Fatal("command/pet alias restarted timer", order)
		}
	}
}

func TestKickSortingPreviewAndDisabledCollection(t *testing.T) {
	now := time.Now()
	k := kickFixture(now)
	c := DefaultConfig()
	c.KickEnabled = true
	m := NewModel()
	castAt := now.Add(-13 * time.Second)
	m.Observe(Cast{GUID: k.Members[1].GUID, Spell: 57994, At: castAt}, castAt)
	k.Sort = 2
	d := KickView(m.Snapshot(now), c, k, now, now, 123)
	if d.KickBars[0].GUID != k.Members[1].GUID || d.Icons[0].GUID != k.Members[0].GUID {
		t.Fatal("sort moved frame bindings", d)
	}
	c.KickSpells = []int{6552}
	d = KickView(m.Snapshot(now), c, k, now, now, 123)
	if len(d.KickBars) != 1 || d.KickBars[0].Spell != 6552 {
		t.Fatal("spell selection ignored", d)
	}
	c.KickSpells = []int{}
	if KickView(m.Snapshot(now), c, k, now, now, 123).Enabled {
		t.Fatal("empty selection displayed interrupts")
	}
	c.KickSpells = nil
	k.Preview = true
	k.Mode = 2
	before := len(m.Evidence)
	d = KickView(m.Snapshot(now), c, k, now, now, 123)
	if len(d.KickBars) != 5 || !d.KickPanel.Preview || len(m.Evidence) != before {
		t.Fatal("preview polluted evidence", d)
	}
	m.DisabledDefensives = true
	if m.Observe(Cast{GUID: "Player-1-X", Spell: 108271, At: now}, now) {
		t.Fatal("LamdaCD collection stayed on")
	}
	if !m.Observe(Cast{GUID: "Player-1-X", Spell: 57994, At: now}, now) {
		t.Fatal("LamdaCD disabled LamdaKick")
	}
	m.DisabledInterrupts = true
	m.Reset()
	if m.Observe(Cast{GUID: "Player-1-X", Spell: 57994, At: now}, now) {
		t.Fatal("disabled interrupt collection reset")
	}
}

func TestKickConfigDefaultsAndExplicitOptIn(t *testing.T) {
	base := `LamdaEngineDB = {["schema"]=1,["cdEnabled"]=false,["nativeFrames"]=true,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,["engineTracking"]=false,["lookupBridge"]=false,`
	for _, value := range []string{"", `["kickEnabled"]=true,`, `["kickEnabled"]=false,`} {
		c, err := ReadConfig(writeConfigFixture(t, base+value+`}`))
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Contains(value, "true")
		if c.KickEnabled != want || c.TrackingEnabled != want || c.LookupBridge != want || c.DefensiveTracking || c.FrameEstimates || c.Enabled {
			t.Fatal(c)
		}
	}
	if _, err := ReadConfig(writeConfigFixture(t, base+`["kickEnabled"]="true"}`)); err == nil {
		t.Fatal("invalid flag accepted")
	}
}

func TestPetInterruptRequiresFreshExactOwner(t *testing.T) {
	now := time.Unix(1789500000, 0)
	k := kickFixture(now)
	k.Members[1].Class = 9
	k.Members[1].Pet = "Pet-1-DEMON"
	line := fmt.Sprintf("%s+0  SPELL_CAST_SUCCESS,Pet-1-DEMON,Felhunter,0x1112,0x0,0000000000000000,nil,0x80000000,0x0,19647,Spell Lock,0x20", now.UTC().Format("1/2/2006 15:04:05.000"))
	roster := map[string]bool{k.Members[1].GUID: true}
	m := NewModel()
	if !m.IngestTeam(line, now, false, roster, k, now) {
		t.Fatal("owned pet missing")
	}
	if e := m.Snapshot(now).Cooldowns[0]; e.GUID != k.Members[1].GUID || e.Spell != 19647 {
		t.Fatal(e)
	}
	for _, wrong := range []string{"Pet-1-OTHER", "Creature-1-DEMON"} {
		if NewModel().IngestTeam(strings.Replace(line, "Pet-1-DEMON", wrong, 1), now, false, roster, k, now) {
			t.Fatal("guessed pet owner", wrong)
		}
	}
	if NewModel().IngestTeam(line, now.Add(2*time.Second), false, roster, k, now) {
		t.Fatal("stale owner accepted")
	}
}
