package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"reflect"
	"testing"
	"time"
)

func TestCapturePausePreservesAuthoringAndSavedMemory(t *testing.T) {
	s, r := savedStore(t)
	at := time.Unix(2000000000, 0)
	captureHistory(s, r, "Alice", "Moon Guard")
	historyValue(t, s, r, "Alice", "MoonGuard", 1, true)
	rules := s.doc
	s.runtime.displays["test"] = engine.OverlayDisplay{Kind: "text", Text: "old"}
	s.SetCaptureEnabled(false)
	if s.runtime.last != nil || len(s.runtime.displays) != 0 || len(s.runtime.queue) != 0 || len(s.runtime.sounds) != 0 {
		t.Fatal("live work retained")
	}
	if !reflect.DeepEqual(s.doc, rules) {
		t.Fatal("authoring changed")
	}
	historyValue(t, s, r, "Alice", "MoonGuard", 1, true)
	s.Accept(resourceSample([]Rule{r}, at, 1, 30), at)
	in := engine.FrameDisplay{Overlays: []engine.OverlayDisplay{{Kind: "text", Text: "another module"}}}
	if got := s.Merge(in, at); !reflect.DeepEqual(got, in) || s.runtime.last != nil {
		t.Fatal("disabled store processed capture")
	}
	if e := s.SeedCombatLog("missing/file", 1, at); e != nil {
		t.Fatal("disabled store opened log", e)
	}
	s.IngestCombatLog("irrelevant", at)
	if s.NeedsCombatLog(at) {
		t.Fatal("disabled store requested log")
	}
	// Editing stays available while capture is paused.
	r.Name = "Edited while off"
	if err := s.Save(s.doc.Revision, []Rule{r}); err != nil {
		t.Fatal(err)
	}
	s.SetCaptureEnabled(true)
	if s.runtime.last != nil || len(s.Merge(engine.FrameDisplay{}, at).Overlays) != 0 {
		t.Fatal("stale display resumed")
	}
	s.Accept(resourceSample([]Rule{r}, at, 2, 30), at)
	if s.runtime.last == nil || len(s.Merge(engine.FrameDisplay{}, at).Overlays) != 1 {
		t.Fatal("fresh capture did not resume")
	}
	historyValue(t, s, r, "Alice", "MoonGuard", 1, true)
}
func TestCombatLogDemandRequiresLiveCaptureAndEnabledRule(t *testing.T) {
	s, p, at := combatSetup(t, combatTestRule("damage"))
	if !s.NeedsCombatLog(at) {
		t.Fatal("combat rule lost required input")
	}
	if s.NeedsCombatLog(at.Add(engine.FrameLifetime + time.Second)) {
		t.Fatal("stale capture keeps log open")
	}
	s.SetCaptureEnabled(false)
	if s.NeedsCombatLog(at) {
		t.Fatal("disabled module needs log")
	}
	s.SetCaptureEnabled(true)
	if s.NeedsCombatLog(at) {
		t.Fatal("resume without capture requested log")
	}
	p.Overlays.Seq++
	s.Accept(p, at)
	if !s.NeedsCombatLog(at) {
		t.Fatal("new capture failed to resume log")
	}
	r := combatTestRule("damage")
	r.Enabled = false
	if e := s.Save(s.doc.Revision, []Rule{r}); e != nil {
		t.Fatal(e)
	}
	s.Accept(combatTestSample([]Rule{r}, at), at)
	if s.NeedsCombatLog(at) {
		t.Fatal("disabled rule needs log")
	}
	r = rule()
	if e := s.Save(s.doc.Revision, []Rule{r}); e != nil {
		t.Fatal(e)
	}
	s.Accept(sample([]Rule{r}, at, 1), at)
	if s.NeedsCombatLog(at) {
		t.Fatal("ordinary aura rule needs log")
	}
}
