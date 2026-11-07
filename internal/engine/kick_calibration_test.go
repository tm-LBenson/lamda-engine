package engine

import (
	"testing"
	"time"
)

func TestKickCalibrationJoinExpiryUseAndRejoin(t *testing.T) {
	start := time.Unix(1789500000, 0)
	m := NewModel()
	k := kickFixture(start)
	c := DefaultConfig()
	c.KickEnabled = true
	member := k.Members[1]
	member.Spec = 262
	k.Members[1] = member
	roster := map[string]bool{member.GUID: true}
	m.TrackPartyRoster(roster, start)
	for _, check := range []struct {
		seconds int
		status  string
	}{{0, "calibrating"}, {11, "calibrating"}, {12, "estimated_ready"}, {300, "estimated_ready"}} {
		now := start.Add(time.Duration(check.seconds) * time.Second)
		k.At = now
		view := KickView(m.Snapshot(now), c, k, now, now, 123)
		if b := view.KickBars[1]; b.Status != check.status || b.Ends != float64(start.Unix()+12) {
			t.Fatal(b)
		}
	}
	if m.Diagnostics.AcceptedCasts != 0 || len(m.Seen) != 0 {
		t.Fatal("calibration fabricated a cast")
	}
	at := start.Add(301 * time.Second)
	m.Observe(Cast{GUID: member.GUID, Spell: 57994, At: at}, at)
	k.At = at
	if b := KickView(m.Snapshot(at), c, k, at, at, 123).KickBars[1]; b.Status != "estimated_cooldown" || b.Ends != float64(at.Unix()+12) {
		t.Fatal(b)
	}
	m.TrackPartyRoster(map[string]bool{}, at.Add(time.Second))
	m.TrackPartyRoster(roster, at.Add(50*time.Second))
	k.At = at.Add(50 * time.Second)
	if b := KickView(m.Snapshot(k.At), c, k, k.At, k.At, 123).KickBars[1]; b.Status != "calibrating" {
		t.Fatal("rejoin inherited old ready", b)
	}
}

func TestInspectedCalibrationUsesModifiedDurationAndKeepsInitialClock(t *testing.T) {
	start := time.Unix(1789500000, 0)
	m := NewModel()
	k := kickFixture(start)
	c := DefaultConfig()
	c.KickEnabled = true
	guid := k.Members[1].GUID
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start.Add(3*time.Second), 264, map[int]int{57994: 1})
	for _, check := range []struct {
		seconds int
		status  string
	}{{10, "calibrating"}, {29, "calibrating"}, {30, "estimated_ready"}} {
		now := start.Add(time.Duration(check.seconds) * time.Second)
		k.At = now
		b := KickView(m.Snapshot(now), c, k, now, now, 123).KickBars[1]
		if b.Status != check.status || b.Ends != float64(start.Unix()+30) {
			t.Fatal(b)
		}
	}
}

func TestKickCalibrationHonorsKnownSpecWithoutCompleteInspection(t *testing.T) {
	now := time.Unix(1789500000, 0)
	m := NewModel()
	k := kickFixture(now)
	member := k.Members[1]
	member.Spec = 264
	m.TrackPartyRoster(map[string]bool{member.GUID: true}, now)
	e := kickEvidence(m.Snapshot(now.Add(12*time.Second)), member, now.Add(12*time.Second))
	if e.Status != "calibrating" || e.CooldownSeconds != 30 {
		t.Fatal("known Restoration baseline shortened", e)
	}
	m.Observe(Cast{GUID: member.GUID, Spell: 57994, At: now.Add(31 * time.Second)}, now.Add(31*time.Second))
	m.TrackPartyRoster(map[string]bool{}, now.Add(32*time.Second))
	m.TrackPartyRoster(map[string]bool{member.GUID: true}, now.Add(33*time.Second))
	e = kickEvidence(m.Snapshot(now.Add(34*time.Second)), member, now.Add(34*time.Second))
	if e.Status != "estimated_cooldown" || e.EstimatedEnd != float64(now.Unix()+43) {
		t.Fatal("rejoin replaced observed active timer", e)
	}
}

func TestNativePlayerOwnershipOverridesEstimatesAndKeepsFirstSlot(t *testing.T) {
	now := time.Unix(1789500000, 0)
	k := kickFixture(now)
	k.OwnSpell = 351338
	k.Members[0].Class = 13
	k.Members[0].Spec = 1468
	c := DefaultConfig()
	c.KickEnabled = true
	m := NewModel()
	m.Observe(Cast{GUID: k.Player, Spell: 351338, At: now.Add(-5 * time.Minute)}, now)
	for _, sort := range []int{1, 2} {
		k.Sort = sort
		s := m.Snapshot(now)
		s.InterruptCounts = map[string]int{k.Player: 7}
		s.InterruptAttempts = map[string]int{k.Player: 10}
		k.Counts = true
		d := KickView(s, c, k, now, now, 123)
		if len(d.KickBars) != 2 || !d.KickBars[0].Native || d.KickBars[0].Status != "native" || d.KickBars[0].Spell != 351338 || *d.KickBars[0].Count != 7 || *d.KickBars[0].Attempts != 10 {
			t.Fatal(d)
		}
		if len(d.Icons) != 1 || d.Icons[0].GUID == k.Player {
			t.Fatal("engine covered native icon", d)
		}
		if d.KickBars[0].Y != k.Y+24 {
			t.Fatal("own row moved", d)
		}
	}
	k.OwnSpell = -1
	if d := KickView(m.Snapshot(now), c, k, now, now, 123); len(d.KickBars) != 1 {
		t.Fatal("unlearned own interrupt shown", d)
	}
	k.OwnSpell = 351338
	k.Self = false
	if d := KickView(m.Snapshot(now), c, k, now, now, 123); len(d.KickBars) != 1 {
		t.Fatal("self toggle ignored", d)
	}
}
