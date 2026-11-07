package engine

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

func TestLeftCooldownRowsStartBesideFrameAcrossCountsAndWraps(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	c.FrameSpells = nil
	for _, matched := range []bool{false, true} {
		for _, count := range []int{1, 2, 3, 4, 7} {
			g := frameFixture(now)
			g.Frames = g.Frames[:1]
			g.Frames[0].X = 800
			g.Anchor = 3
			g.Growth = 1
			g.Size = 41
			g.Spacing = 2
			g.Gap = 4
			g.OffsetX = -3
			g.OffsetY = -10
			g.Maximum = 8
			g.PerRow = 3
			if matched {
				g.PartyLayout = &framewire.Layout{Size: 41, Spacing: 2, PerRow: 3, Maximum: 8, Opacity: 100, Anchor: 3, Gap: 4, OffsetX: -3, OffsetY: -10, Growth: 1}
			}
			s := InferenceSnapshot{}
			for i := 0; i < count; i++ {
				s.Cooldowns = append(s.Cooldowns, CooldownEvidence{GUID: g.Frames[0].GUID, Spell: 100 + i, Status: "unknown"})
			}
			d := FrameView(s, c, g, now, now, 123)
			if len(d.Icons) != count {
				t.Fatal(matched, count, d)
			}
			for i, icon := range d.Icons {
				if icon.X != 752-(i%3)*43 || icon.Y != 510+(i/3)*43 || icon.X+icon.Size > 793 {
					t.Fatal(matched, count, i, icon)
				}
			}
		}
	}
}

func TestCooldownGrowthOverridesAndLegacyPlacement(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	c.FrameSpells = nil
	g := frameFixture(now)
	g.Frames = g.Frames[:1]
	g.Frames[0].X = 800
	g.PartyLayout = &framewire.Layout{Size: 40, Spacing: 2, PerRow: 6, Maximum: 6, Opacity: 100, Anchor: 3, Gap: 4, OffsetX: -3}
	s := InferenceSnapshot{Cooldowns: []CooldownEvidence{{GUID: g.Frames[0].GUID, Spell: 1, Status: "unknown"}, {GUID: g.Frames[0].GUID, Spell: 2, Status: "unknown"}}}
	for _, mode := range []int{0, 1, 2, 3} {
		g.PartyLayout.Growth = mode
		d := FrameView(s, c, g, now, now, 123)
		if mode == 0 || mode == 2 {
			if d.Icons[0].X != 543 || d.Icons[1].X != 585 {
				t.Fatal(mode, d)
			}
		} else if d.Icons[0].X != 753 || d.Icons[1].X != 711 {
			t.Fatal(mode, d)
		}
	}
	g.PartyLayout.Anchor = 4
	g.PartyLayout.Growth = 1
	d := FrameView(s, c, g, now, now, 123)
	if d.Icons[0].X != 1001 || d.Icons[1].X != 1043 {
		t.Fatal("right-side automatic growth", d)
	}
}

func TestNativePriorityPerSpellFrameAndAutomaticFallback(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	m := NewModel()
	for _, guid := range []string{"Player-1-SELF", "Player-1-FRIEND"} {
		for _, spell := range []int{363916, 374227} {
			m.Observe(Cast{At: now, GUID: guid, Spell: spell, Self: guid == "Player-1-SELF"}, now)
		}
	}
	s := m.Snapshot(now)
	original := m.Snapshot(now)
	g := frameFixture(now)
	g.Maximum = 1
	g.Frames[1].NativeSpells = []int{363916}
	// Another frame for the same player without a native widget still gets estimates.
	g.Frames = append(g.Frames, framewire.Rect{GUID: g.Player, X: 700, Y: 400, Width: 200, Height: 40})
	d := FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 3 || d.Icons[0].Spell != 363916 || d.Icons[1].Spell != 374227 || d.Icons[2].Spell != 363916 || d.Omitted != 2 {
		t.Fatal(d)
	}
	// Withdrawal makes the retained observation eligible without another cast.
	g.Frames[1].NativeSpells = nil
	d = FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 3 || d.Icons[1].Spell != 363916 {
		t.Fatal(d)
	}
	g.Frames[1].NativeSpells = []int{363916, 374227}
	d = FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 2 || !reflect.DeepEqual(original, m.Snapshot(now)) {
		t.Fatal("native priority changed evidence", d)
	}
	g.Self = false
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 1 || d.Icons[0].GUID != "Player-1-FRIEND" {
		t.Fatal(d)
	}
}

func TestNativePriorityAlsoSuppressesUnknownAndEstimatedReady(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	g := frameFixture(now)
	g.Ready = true
	g.Frames[1].NativeSpells = []int{374227}
	for _, status := range []string{"estimated_cooldown", "estimated_recharge", "estimated_ready", "unknown"} {
		s := InferenceSnapshot{Cooldowns: []CooldownEvidence{{GUID: g.Player, Spell: 374227, Status: status}}}
		if d := FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 || d.NativeHidden != 1 || d.Status != "native_cooldowns" {
			t.Fatal(status, d)
		}
		g.Frames[1].NativeSpells = nil
		if d := FrameView(s, c, g, now, now, 123); len(d.Icons) != 1 || d.Icons[0].Status != status {
			t.Fatal(status, d)
		}
		g.Frames[1].NativeSpells = []int{374227}
	}
}

func frameFixture(now time.Time) *framewire.Snapshot {
	return &framewire.Snapshot{ID: "1:1", At: now, Width: 1600, Height: 1000, Player: "Player-1-SELF", Size: 32, Spacing: 2, PerRow: 6, Maximum: 6, Opacity: 80, Anchor: 2, Gap: 8, Self: true, Party: true, Unknown: true,
		Frames: []framewire.Rect{{GUID: "Player-1-FRIEND", X: 100, Y: 500, Width: 200, Height: 40}, {GUID: "Player-1-SELF", X: 100, Y: 400, Width: 200, Height: 40}}}
}

func TestFrameViewJoinsGUIDAndTracksMovementWithUncertainty(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	m := NewModel()
	m.Observe(Cast{At: now.Add(-15 * time.Second), GUID: "Player-1-SELF", Name: "SameName", Spell: 55342, Self: true}, now)
	m.Observe(Cast{At: now, GUID: "Player-1-FRIEND", Name: "SameName", Spell: 363916}, now)
	g := frameFixture(now)
	s := m.Snapshot(now)
	d := FrameView(s, c, g, now, now, 123)
	if !d.Enabled || len(d.Icons) != 2 || d.Window != "123" || d.Icons[0].GUID != "Player-1-FRIEND" || d.Icons[0].Y != 460 || !d.Icons[0].ChargesUnknown || d.Icons[1].DeliverySeconds != 15 {
		t.Fatal(d)
	}
	g.Frames[0].GUID = "Player-1-SELF"
	g.Frames[0].X = 420
	d = FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 2 || d.Icons[0].Spell != 55342 || d.Icons[0].X != 420 {
		t.Fatal("stale frame association", d)
	}
	g.Frames = nil
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal("hidden frame retained", d)
	}
	if m.Diagnostics.AcceptedCasts != 2 {
		t.Fatal("presentation mutated evidence")
	}
}

func TestFrameViewExpiresGeometryAndNeverPromotesChargesToReady(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	m := NewModel()
	m.Observe(Cast{At: now, GUID: "Player-1-SELF", Name: "Self", Spell: 363916, Self: true}, now)
	g := frameFixture(now)
	if d := FrameView(m.Snapshot(now), c, g, now, now.Add(FrameLifetime+time.Millisecond), 123); d.Enabled || len(d.Icons) != 0 {
		t.Fatal(d)
	}
	g.At = now.Add(-6 * time.Second)
	if d := FrameView(m.Snapshot(now), c, g, now, now, 123); d.Enabled {
		t.Fatal("old replay accepted", d)
	}
	later := now.Add(125 * time.Second)
	g.At = later
	g.Ready = true
	d := FrameView(m.Snapshot(later), c, g, later, later, 123)
	if len(d.Icons) != 1 || d.Icons[0].Status != "unknown" {
		t.Fatal("unknown charges became ready", d)
	}
	g.Unknown = false
	if d = FrameView(m.Snapshot(later), c, g, later, later, 123); len(d.Icons) != 0 {
		t.Fatal(d)
	}
}

func TestFrameViewSelectionAndPlacementDoNotChangeCollection(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	m := NewModel()
	for _, id := range []int{55342, 363916, 102342} {
		m.Observe(Cast{At: now, GUID: "Player-1-SELF", Name: "Self", Spell: id, Self: true}, now)
	}
	g := frameFixture(now)
	s := m.Snapshot(now)
	g.PerRow = 1
	g.Maximum = 2
	g.Anchor = 4
	g.OffsetX = 7
	g.OffsetY = 3
	d := FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 2 || d.Omitted != 1 || d.Icons[0].X != 315 || d.Icons[0].Y != 397 || d.Icons[1].Y != 431 {
		t.Fatal(d)
	}
	c.FrameSpells = []int{363916}
	d = FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 1 || d.Icons[0].Spell != 363916 || len(m.Evidence) != 3 {
		t.Fatal(d)
	}
	c.FrameSpells = []int{}
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal("empty selection ignored")
	}
	c.FrameSpells = nil
	c.TrackingEnabled = false
	if d = FrameView(s, c, g, now, now, 123); d.Enabled {
		t.Fatal(d)
	}
}

func TestFrameConfigPreservesCSVAndMasterSwitch(t *testing.T) {
	base := `LamdaEngineDB = {["schema"]=1,["cdEnabled"]=true,["nativeFrames"]=true,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,["engineFrameEstimates"]=true,`
	c, err := ReadConfig(writeConfigFixture(t, base+`["engineFrameSpells"]="55342,363916"}`))
	if err != nil || !c.FrameEstimates || c.Enabled || len(c.FrameSpells) != 2 || c.FrameSpells[1] != 363916 {
		t.Fatal(c, err)
	}
	for _, bad := range []string{`"1,1"`, `"0"`, `"1,,2"`, `"1,notCode"`, `"10000001"`, `nil`} {
		if _, err := ReadConfig(writeConfigFixture(t, base+`["engineFrameSpells"]=`+bad+`}`)); err == nil {
			t.Fatal("invalid selection accepted", bad)
		}
	}
	c, err = ReadConfig(writeConfigFixture(t, strings.Replace(base, `["cdEnabled"]=true`, `["cdEnabled"]=false`, 1)+`["engineFrameSpells"]=""}`))
	if err != nil || c.FrameEstimates || c.FrameSpells == nil || len(c.FrameSpells) != 0 {
		t.Fatal(c, err)
	}
}
