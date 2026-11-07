package arena

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

var start = time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC)

func sample(epoch string, seq int, now time.Time, slot, spell, age int) lookup.BridgeSample {
	a := &lookup.ArenaSnapshot{Epoch: epoch, Sequence: seq, At: now, Width: 1920, Height: 1080, Flags: 31, Maximum: 8, Opacity: 100,
		Actors: []lookup.ArenaActor{{Slot: 1, Spec: 62, X: 700, Y: 300, Size: 24, GUID: "Player-1-enemy"}, {Slot: 2, Spec: 62, X: 700, Y: 420, Size: 24}}}
	casts := []lookup.LiveCast{}
	if spell > 0 {
		casts = append(casts, lookup.LiveCast{ID: "Screen:1:1", GUID: a.Identity(slot), Spell: spell, AgeMillis: age})
	}
	return lookup.BridgeSample{Arena: a, Window: 9, WindowPID: 10, Event: lookup.Event{At: now}, Observations: &lookup.Observations{Screen: &lookup.ScreenCasts{ID: "panel:" + epoch, Captured: true, Casts: casts}}}
}
func observed(t *testing.T, spell int) *Model {
	t.Helper()
	m := &Model{}
	m.Handle(sample("round1", 1, start, 1, spell, 0), start)
	if len(m.View(start, false).Icons) != 0 {
		t.Fatal("single reading created timer")
	}
	m.Handle(sample("round1", 2, start.Add(400*time.Millisecond), 1, spell, 400), start.Add(400*time.Millisecond))
	return m
}
func TestCastStartsDuringActiveEffectAndDoesNotRestart(t *testing.T) {
	m := observed(t, 642)
	d := m.View(start.Add(time.Second), false)
	if len(d.Icons) != 1 || d.Icons[0].GUID != "Arena:round1:1" || d.Icons[0].Ends != float64(start.Unix())+Catalog[642].Cooldown {
		t.Fatalf("wrong timer %+v", d)
	}
	m.Handle(sample("round1", 3, start.Add(time.Second), 1, 642, 1000), start.Add(time.Second))
	if m.Status().Observed != 1 || m.View(start.Add(time.Second), false).Icons[0].Ends != d.Icons[0].Ends {
		t.Fatal("buffer restarted cooldown")
	}
	if len(d.Notices) != 1 || d.Notices[0].Text != "Divine Shield" {
		t.Fatalf("cast notice %+v", d.Notices)
	}
}
func TestSameSpecOpponentsAndPetAlias(t *testing.T) {
	m := observed(t, 2139)
	m.Handle(sample("round1", 3, start.Add(800*time.Millisecond), 2, 119910, 0), start.Add(800*time.Millisecond))
	m.Handle(sample("round1", 4, start.Add(1200*time.Millisecond), 2, 119910, 400), start.Add(1200*time.Millisecond))
	d := m.View(start.Add(1200*time.Millisecond), false)
	if len(d.Icons) != 2 || d.Icons[0].Spell != 2139 || d.Icons[1].Spell != 19647 || d.Icons[1].Y != 420 {
		t.Fatalf("actor alias leak %+v", d.Icons)
	}
}
func TestConflictingDigitsPoisonDeliveryAndFrozenPixelsExpire(t *testing.T) {
	m := &Model{}
	m.Handle(sample("r", 1, start, 1, 2139, 0), start)
	m.Handle(sample("r", 2, start.Add(400*time.Millisecond), 1, 1766, 400), start.Add(400*time.Millisecond))
	m.Handle(sample("r", 3, start.Add(800*time.Millisecond), 1, 2139, 800), start.Add(800*time.Millisecond))
	if len(m.View(start.Add(time.Second), false).Icons) != 0 || m.Status().Rejected == 0 {
		t.Fatal("conflicting reading accepted")
	}
	m = observed(t, 2139)
	m.Handle(sample("round1", 2, start.Add(400*time.Millisecond), 1, 2139, 400), start.Add(2*time.Second))
	if m.View(start.Add(2*time.Second), false).Enabled {
		t.Fatal("frozen packet renewed lease")
	}
}
func TestRoundWindowSpecAndGUIDReset(t *testing.T) {
	for _, kind := range []string{"round", "window", "spec", "guid"} {
		t.Run(kind, func(t *testing.T) {
			m := observed(t, 2139)
			s := sample("round1", 3, start.Add(time.Second), 0, 0, 0)
			switch kind {
			case "round":
				s.Arena.Epoch = "round2"
				s.Arena.Sequence = 1
			case "window":
				s.Window = 11
			case "spec":
				s.Arena.Actors[0].Spec = 63
			case "guid":
				s.Arena.Actors[0].GUID = "Player-1-new"
			}
			m.Handle(s, start.Add(time.Second))
			if len(m.View(start.Add(time.Second), false).Icons) > 0 {
				t.Fatal("old opponent survived")
			}
		})
	}
}
func TestLearnedReadyPersistsAndCaptureGapIsRecorded(t *testing.T) {
	m := observed(t, 1766)
	now := start.Add(20 * time.Second)
	s := sample("round1", 3, now, 0, 0, 0)
	s.Observations = nil
	m.Handle(s, now)
	d := m.View(now, false)
	if len(d.Icons) != 1 || d.Icons[0].Status != "estimated_ready" {
		t.Fatalf("learned readiness lost %+v", d.Icons)
	}
	if m.Status().CaptureGaps != 1 {
		t.Fatal("missing capture not recorded")
	}
	s.Arena.Hidden = true
	s.Arena.Sequence++
	m.Handle(s, now)
	if m.View(now, false).Enabled {
		t.Fatal("hidden switch ignored")
	}
	s.Arena.Hidden = false
	s.Arena.Flags = 0
	s.Arena.Sequence++
	m.Handle(s, now)
	if len(m.Status().Cooldowns) != 0 || m.View(now, false).Enabled {
		t.Fatal("disabled state persisted")
	}
}
func TestBackgroundOptInAndNoResurrection(t *testing.T) {
	m := observed(t, 2139)
	now := start.Add(time.Second)
	if m.CanHold(now) || m.View(start.Add(10*time.Second), true).Enabled {
		t.Fatal("default background enabled")
	}
	s := sample("round1", 3, now, 0, 0, 0)
	s.Event.KeepUIWhenUnfocused = true
	m.Handle(s, now)
	if !m.CanHold(now) || m.CanHold(now.Add(2*time.Second)) || !m.View(now.Add(10*time.Second), true).Enabled {
		t.Fatal("background lease")
	}
	m.Pause()
	if m.View(now.Add(10*time.Second), true).Enabled {
		t.Fatal("paused arena revived")
	}
}
func record(event, source, target string, id int, extra string) string {
	return fmt.Sprintf("9/28/2026 14:00:00.500+00  %s,%s,\"SameName\",0x548,0x0,%s,\"SameName\",0x548,0x0,%d,\"Spell\",1%s", event, source, target, id, extra)
}
func TestLogsNeedExactActorAndSuccessfulInterrupt(t *testing.T) {
	m := &Model{}
	m.Handle(sample("r", 1, start, 0, 0, 0), start)
	now := start.Add(time.Second)
	m.IngestLog(record("SPELL_CAST_SUCCESS", "Player-1-other", "Player-1-enemy", 1766, ""), now)
	if len(m.View(now, false).Notices) != 0 || len(m.Status().Cooldowns) != 0 {
		t.Fatal("cast created target lockout or guessed identity")
	}
	m.IngestLog(record("SPELL_INTERRUPT", "Player-1-other", "Player-1-enemy", 1766, ",133,\"Fireball\",4"), now)
	d := m.View(now, false)
	if len(d.Notices) != 1 || !d.Notices[0].Countdown || d.Notices[0].Text != "Fire locked ~" {
		t.Fatalf("confirmed lockout absent %+v", d.Notices)
	}
	m.IngestLog(record("SPELL_CAST_SUCCESS", "Player-1-enemy", "Player-1-other", 2139, ""), now)
	if len(m.Status().Cooldowns) != 1 || m.Status().Cooldowns[0].Slot != 1 {
		t.Fatal("mapped hostile cast missed")
	}
	m = observed(t, 2139)
	m.IngestLog(record("SPELL_CAST_SUCCESS", "Player-1-enemy", "Player-1-other", 2139, ""), now)
	if m.Status().Observed != 1 {
		t.Fatal("log duplicated screen cast")
	}
}
func TestMergePreservesPartyAndProtectsCapture(t *testing.T) {
	m := observed(t, 2139)
	base := engine.FrameDisplay{Enabled: true, Window: "9", Width: 1920, Height: 1080, KickPanel: &engine.KickPanel{Width: 100}}
	d := Merge(base, m.View(start.Add(time.Second), false))
	if d.KickPanel == nil || len(d.Icons) != 1 {
		t.Fatal("party display replaced")
	}
	engine.ReserveCaptureArea(&d, 700, 300, 128, 176)
	if len(d.Icons) != 0 || len(d.Notices) != 0 {
		t.Fatal("arena display covers capture")
	}
}
