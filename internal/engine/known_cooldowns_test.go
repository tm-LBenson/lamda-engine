package engine

import (
	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"reflect"
	"testing"
	"time"
)

func TestPartyMatchesNativeCapacityAndOwnFallbackKeepsCustomLayout(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	g := frameFixture(now)
	g.Frames[0].X = 500
	g.Frames[1].X = 500
	g.PartyLayout = &framewire.Layout{Size: 40, Spacing: 2, PerRow: 6, Maximum: 6, Opacity: 100, Anchor: 3, Gap: 4, OffsetX: 83, OffsetY: -10}
	s := InferenceSnapshot{Cooldowns: []CooldownEvidence{{GUID: "Player-1-FRIEND", Spell: 374227, Status: "unknown"}, {GUID: g.Player, Spell: 374227, Status: "unknown"}}}
	d := FrameView(s, c, g, now, now, 123)
	// Native anchors six slots, not the single occupied slot; preserve saved offsets.
	if len(d.Icons) != 2 || d.Icons[0].X != 329 || d.Icons[0].Y != 510 || d.Icons[0].Size != 40 || d.Icons[0].Opacity != 100 {
		t.Fatal(d)
	}
	if d.Icons[1].X != 500 || d.Icons[1].Y != 360 || d.Icons[1].Size != 32 {
		t.Fatal("own fallback moved over native lane", d)
	}
	g.PartyLayout = nil
	if d = FrameView(s, c, g, now, now, 123); d.Icons[0].X != 500 || d.Icons[0].Y != 460 || d.Icons[0].Size != 32 {
		t.Fatal("custom layout ignored", d)
	}
}

func knownFixture(now time.Time) InferenceSnapshot {
	return InferenceSnapshot{PublicBuilds: []lookup.Profile{{Character: lookup.Character{GUID: "Player-1-FRIEND", Name: "Friend", Realm: "Realm", Region: "us"}, FetchedAt: now, Source: "raider_io", SpecID: 1468, Revision: "public", Talents: []lookup.Talent{
		{NodeID: 1, EntryID: 10, SpellID: 374227, Rank: 1}, {NodeID: 2, EntryID: 20, SpellID: 363916, Rank: 1},
		{NodeID: 3, EntryID: 30, SpellID: 357170, Rank: 0}, {NodeID: 4, EntryID: 40, SpellID: 1, Rank: 1},
	}}}}
}

func TestKnownAbilitiesHaveNoInventedTimerOrChargeState(t *testing.T) {
	now := time.Now()
	s := knownFixture(now)
	items := knownCooldowns(s, now)
	if len(items) != 2 {
		t.Fatal(items)
	}
	for _, e := range items {
		if e.Status != "unknown" || e.EstimatedEnd != 0 || !e.EventAt.IsZero() || e.AvailableCharges != nil || e.Source != "public_build" {
			t.Fatal(e)
		}
	}
	s.Cooldowns = []CooldownEvidence{{GUID: "Player-1-FRIEND", Spell: 374227, Status: "estimated_cooldown", EstimatedEnd: 100}}
	if items = knownCooldowns(s, now); len(items) != 1 || items[0].Spell != 363916 {
		t.Fatal("build replaced observed cast", items)
	}
}

func TestKnownAbilitiesRespectNewLocalTalentEntriesAndProfileAge(t *testing.T) {
	now := time.Now()
	s := knownFixture(now)
	s.Builds = []BuildSnapshot{{GUID: "Player-1-FRIEND", At: now, SpecID: 1467, Talents: [][3]int{{1, 10, 1}, {2, 99, 1}}}}
	items := knownCooldowns(s, now)
	if len(items) != 1 || items[0].Spell != 374227 {
		t.Fatal("stale public selection overrode newer local build", items)
	}
	s.Builds[0].Talents = nil
	if len(knownCooldowns(s, now)) != 0 {
		t.Fatal("partial local build treated as confirmed selection")
	}
	s.Builds = nil
	for _, at := range []time.Time{now.Add(time.Second), now.Add(-25 * time.Hour), {}} {
		s.PublicBuilds[0].FetchedAt = at
		if len(knownCooldowns(s, now)) != 0 {
			t.Fatal("invalid profile time accepted", at)
		}
	}
}

func TestPersistentFramesFillKnownIconsWithoutMutatingEvidence(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	s := knownFixture(now)
	g := frameFixture(now)
	g.Persistent = true
	before := knownFixture(now)
	d := FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 2 || d.Icons[0].Status != "unknown" || d.Icons[0].Ends != 0 || !reflect.DeepEqual(s, before) {
		t.Fatal(d)
	}
	g.Persistent = false
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal("ignored observation-only mode", d)
	}
	g.Persistent = true
	g.Unknown = false
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal("ignored unknown visibility", d)
	}
	g.Unknown = true
	c.FrameSpells = []int{374227}
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 1 || d.Icons[0].Spell != 374227 {
		t.Fatal(d)
	}
	g.Party = false
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal(d)
	}
}

func TestPersistentRowKeepsReadyAndPrioritizesActiveCooldowns(t *testing.T) {
	now := time.Now()
	c := DefaultConfig()
	c.FrameEstimates = true
	s := knownFixture(now)
	g := frameFixture(now)
	g.Persistent = true
	g.Maximum = 1
	s.Cooldowns = []CooldownEvidence{{GUID: "Player-1-FRIEND", Spell: 374227, Status: "estimated_cooldown", BaselineSeconds: 120, EstimatedEnd: float64(now.Unix()) + 30}}
	d := FrameView(s, c, g, now, now, 123)
	if len(d.Icons) != 1 || d.Icons[0].Spell != 374227 || d.Icons[0].Duration != 120 {
		t.Fatal("unknown slot displaced running timer", d)
	}
	s.PublicBuilds = nil
	s.Cooldowns[0].Status = "estimated_ready"
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 1 || d.Icons[0].Status != "estimated_ready" {
		t.Fatal(d)
	}
	g.Persistent = false
	if d = FrameView(s, c, g, now, now, 123); len(d.Icons) != 0 {
		t.Fatal("legacy ready flag ignored", d)
	}
}
