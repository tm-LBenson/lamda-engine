package overlays

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func bossSample(rs []Rule, at time.Time, seq int, b *lookup.OverlayBosses) lookup.BridgeSample {
	s := encounterSample(rs, at, seq, nil)
	s.Overlays.EncountersVersion = 0
	s.Overlays.BossesVersion = 1
	s.Overlays.Bosses = b
	return s
}
func bossSnapshot() *lookup.OverlayBosses {
	return &lookup.OverlayBosses{Providers: []lookup.OverlayBossProvider{{Name: "dbm", Available: true, Complete: true, Stage: ptr(2.5), EncounterID: 1234}, {Name: "bigwigs", Available: true, Complete: true}}, Timers: []lookup.OverlayBossTimer{{Provider: "dbm", ID: 1, Spell: ptr(123), Name: "First timer", Kind: "timer", Remaining: ptr(3.0), Duration: ptr(10.0), Count: ptr(2.0), Paused: ptr(false)}, {Provider: "dbm", ID: 2, Spell: ptr(123), Name: "Second timer", Kind: "timer", Remaining: ptr(20.0), Duration: ptr(30.0), Paused: ptr(true)}}}
}
func TestBossConditionsPartialCaptureAndQualifiedSource(t *testing.T) {
	p := &lookup.OverlayInput{BossesVersion: 1, Bosses: bossSnapshot()}
	q := map[string]any{"condition": "bossTimer", "provider": "auto", "spell": "123", "check": "remaining", "operator": "gt", "value": float64(10)}
	if predicate(q, p, "") != yes {
		t.Fatal("known matching timer missed")
	}
	source := bossTimerSource(bossQuery(q), p)
	if source == nil || source.ID != 2 {
		t.Fatal("source ignored IF threshold", source)
	}
	q["check"] = "paused"
	if source = bossTimerSource(bossQuery(q), p); source == nil || source.ID != 2 {
		t.Fatal("source ignored paused condition")
	}
	q["check"] = "running"
	if source = bossTimerSource(bossQuery(q), p); source == nil || source.ID != 1 {
		t.Fatal("source ignored running condition")
	}
	q["check"] = "missing"
	q["spell"] = "456"
	if predicate(q, p, "") != yes {
		t.Fatal("complete missing")
	}
	p.Bosses.Providers[0].Complete = false
	if predicate(q, p, "") != unknown {
		t.Fatal("partial absence was true")
	}
	not := map[string]any{"condition": "group", "mode": "not", "tests": []any{q}}
	if predicate(not, p, "") != unknown {
		t.Fatal("NOT unknown")
	}
	p.Bosses.Providers[0].Complete = true
	p.Bosses.Timers[0].Spell = nil
	if predicate(q, p, "") != unknown {
		t.Fatal("unknown spell silently excluded")
	}
	q["spell"] = "123"
	if predicate(q, p, "") != no {
		t.Fatal("known present with unknown other")
	}
	q["condition"] = "bossStage"
	q["operator"] = "gte"
	q["value"] = float64(2.5)
	q["encounterID"] = "1234"
	if predicate(q, p, "") != yes {
		t.Fatal("stage")
	}
	p.Bosses.Providers[0].Stage = nil
	if predicate(q, p, "") != unknown {
		t.Fatal("unknown stage became zero")
	}
	p.Bosses.Providers[0].Available = false
	q["provider"] = "dbm"
	if predicate(q, p, "") != unknown {
		t.Fatal("missing provider")
	}
}
func TestBossTriggerLifetimeAndSnapshotSource(t *testing.T) {
	rule := rule()
	rule.Roots[0].Props = map[string]any{"event": "bossstart", "provider": "dbm", "spell": "123"}
	show := Node{ID: "boss-display", Type: "show", Props: map[string]any{"display": "bar", "text": "{name}: {remaining}", "duration": float64(10), "fill": "source"}}
	rule.Roots[0].Children = []Node{show}
	rs := []Rule{rule}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	b := bossSnapshot()
	s.Accept(bossSample(rs, at, 1, b), at)
	b = bossSnapshot()
	b.Seq = 1
	b.Events = []lookup.OverlayBossEvent{{Seq: 1, Kind: "bossstart", Timer: b.Timers[0]}}
	at = at.Add(200 * time.Millisecond)
	s.Accept(bossSample(rs, at, 2, b), at)
	d := s.Merge(engine.FrameDisplay{}, at)
	if len(d.Overlays) != 1 || d.Overlays[0].Text != "First timer: 3" || *d.Overlays[0].Fraction != .3 {
		t.Fatalf("start %+v", d.Overlays)
	}
	b = bossSnapshot()
	b.Seq = 1
	b.Timers[0].Remaining = ptr(8.0)
	b.Timers[0].Paused = ptr(true)
	at = at.Add(200 * time.Millisecond)
	s.Accept(bossSample(rs, at, 3, b), at)
	d = s.Merge(engine.FrameDisplay{}, at)
	if d.Overlays[0].Text != "First timer: 8" || *d.Overlays[0].Fraction != .8 {
		t.Fatal("timer correction not followed", d.Overlays)
	}
	s.doc.Rules[0].Roots[0].Children[0].Props["dataSource"] = "trigger"
	d = s.Merge(engine.FrameDisplay{}, at)
	if d.Overlays[0].Text != "First timer: 3" {
		t.Fatal("original event mutated", d.Overlays)
	}
	s.doc.Rules[0].Roots[0].Children[0].Props["dataSource"] = "auto"
	// Same-spell replacements must not become the original timer's source.
	b = bossSnapshot()
	b.Seq = 1
	b.Timers = b.Timers[1:]
	at = at.Add(200 * time.Millisecond)
	s.Accept(bossSample(rs, at, 4, b), at)
	d = s.Merge(engine.FrameDisplay{}, at)
	if d.Overlays[0].Text != "First timer: 0" {
		t.Fatal("timer identity replaced", d.Overlays)
	}
	b.Providers[0].Complete = false
	at = at.Add(200 * time.Millisecond)
	s.Accept(bossSample(rs, at, 5, b), at)
	if d = s.Merge(engine.FrameDisplay{}, at); len(d.Overlays) != 0 {
		t.Fatal("partial history guessed remaining", d.Overlays)
	}
}
func TestBossEventCursorProviderFiltersAndDelayedContext(t *testing.T) {
	r := rule()
	r.Roots[0].Props = map[string]any{"event": "bosswarning", "provider": "auto", "spell": "123"}
	r.Roots[0].Children = []Node{{ID: "wait", Type: "wait", Props: map[string]any{"seconds": float64(1)}}, {ID: "warning", Type: "show", Props: map[string]any{"display": "text", "text": "{provider}: {name}", "duration": float64(5)}}}
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	s.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	b := bossSnapshot()
	b.Seq = 1
	b.Events = []lookup.OverlayBossEvent{{Seq: 1, Kind: "bosswarning", Timer: lookup.OverlayBossTimer{Provider: "dbm", Spell: ptr(123), Name: "History"}}}
	s.Accept(bossSample(rs, at, 1, b), at)
	if len(s.runtime.queue) != 0 {
		t.Fatal("history replay")
	}
	b.Seq = 2
	b.Events[0].Seq = 2
	b.Events[0].Timer.Provider = "bigwigs"
	at = at.Add(200 * time.Millisecond)
	s.Accept(bossSample(rs, at, 2, b), at)
	if len(s.runtime.queue) != 0 {
		t.Fatal("auto duplicated providers")
	}
	b.Seq = 3
	b.Events[0].Seq = 3
	b.Events[0].Timer.Provider = "dbm"
	b.Events[0].Timer.Name = "Original"
	at = at.Add(200 * time.Millisecond)
	s.Accept(bossSample(rs, at, 3, b), at)
	if len(s.runtime.queue) != 1 {
		t.Fatal("warning did not schedule")
	}
	for i := 4; i <= 10; i++ {
		at = at.Add(200 * time.Millisecond)
		b = bossSnapshot()
		b.Seq = 3
		s.Accept(bossSample(rs, at, i, b), at)
	}
	d := s.Merge(engine.FrameDisplay{}, at)
	if len(d.Overlays) != 1 || d.Overlays[0].Text != "DBM: Original" {
		t.Fatal("delayed context", d.Overlays)
	}
	at = at.Add(10 * time.Second)
	b.Seq = 4
	b.Events = []lookup.OverlayBossEvent{{Seq: 4, Kind: "bosswarning", Timer: lookup.OverlayBossTimer{Provider: "dbm", Spell: ptr(123), Name: "Old"}}}
	s.Accept(bossSample(rs, at, 11, b), at)
	if len(s.runtime.queue) != 0 || len(s.runtime.displays) != 0 {
		t.Fatal("reconnect replay")
	}
}

func TestBossDemandPublishingAndAvailability(t *testing.T) {
	r := rule()
	r.Roots[0].Props = map[string]any{"event": "bossstart", "provider": "dbm", "spell": "123"}
	rs := []Rule{r}
	if !needsBosses(rs) {
		t.Fatal("event demand missing")
	}
	s, _ := Open(t.TempDir())
	s.doc.Rules = rs
	path := filepath.Join(t.TempDir(), "OverlaysData.lua")
	if err := os.WriteFile(path, []byte(Marker+"\nLamdaOverlaysData={}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.SetGameFile(path)
	b, err := os.ReadFile(path)
	if err != nil || s.publishError != "" || !strings.Contains(string(b), `["bosses"]=true`) {
		t.Fatal("boss flag not published", err, s.publishError)
	}
	rt := &Runtime{}
	rt.validateBosses(&lookup.OverlayInput{}, rs)
	if rt.problem != "Reload WoW for boss tracking" {
		t.Fatal(rt.problem)
	}
	p := &lookup.OverlayInput{BossesVersion: 1, Bosses: bossSnapshot()}
	rt.problem = ""
	rt.validateBosses(p, rs)
	if rt.problem != "" {
		t.Fatal(rt.problem)
	}
	p.Bosses.Providers[0].Available = false
	rt.validateBosses(p, rs)
	if rt.problem != "DBM unavailable" {
		t.Fatal(rt.problem)
	}
	rt.problem = ""
	p.Bosses.Overflow = true
	rt.validateBosses(p, rs)
	if rt.problem != "Boss timer limit reached" {
		t.Fatal(rt.problem)
	}
	rs[0].Enabled = false
	if needsBosses(rs) {
		t.Fatal("disabled rule collects")
	}
	rs[0].Enabled = true
	for _, props := range []map[string]any{
		{"event": "bossstart", "provider": "invented"}, {"event": "bossstart", "spell": "abc"}, {"event": "bossstart", "textFilter": strings.Repeat("a", 81)},
	} {
		rs[0].Roots[0].Props = props
		if Validate(rs) == nil {
			t.Fatal("invalid filter accepted", props)
		}
	}
	rs[0].Roots[0].Props = map[string]any{"event": "state"}
	rs[0].Roots[0].Children = []Node{{ID: "show", Type: "show", Props: map[string]any{"display": "text", "text": "{stage}", "duration": float64(5), "dataSource": "bossStage", "dataProvider": "bigwigs"}}}
	if !needsBosses(rs) {
		t.Fatal("display source demand missing")
	}
	if err := Validate(rs); err != nil {
		t.Fatal(err)
	}
}

func TestBlizzardTimelineSourcesAndUnknownSpell(t *testing.T) {
	p := &lookup.OverlayInput{BossesVersion: 1, Bosses: &lookup.OverlayBosses{Providers: []lookup.OverlayBossProvider{{Name: "blizzard", Available: true, Complete: true}}, Timers: []lookup.OverlayBossTimer{{Provider: "blizzard", ID: 1, Source: "encounter", Remaining: ptr(3.0), Duration: ptr(10.0)}, {Provider: "blizzard", ID: 2, Source: "script", Name: "Scripted", Spell: ptr(123), Remaining: ptr(8.0), Duration: ptr(10.0)}}}}
	q := map[string]any{"condition": "bossTimer", "provider": "blizzard", "check": "remaining", "operator": "lt", "value": float64(5), "timelineSource": "encounter"}
	if predicate(q, p, "") != yes {
		t.Fatal("public timing missing")
	}
	q["spell"] = "123"
	if predicate(q, p, "") != unknown {
		t.Fatal("restricted spell inferred")
	}
	q["timelineSource"] = "script"
	if predicate(q, p, "") != no {
		t.Fatal("wrong source selected")
	}
	q["value"] = float64(10)
	if predicate(q, p, "") != yes {
		t.Fatal("script source missing")
	}
	source := bossTimerSource(bossQuery(q), p)
	if source == nil || source.ID != 2 {
		t.Fatal("source filter ignored", source)
	}
	q["timelineSource"] = "invented"
	if validBossCondition(q) {
		t.Fatal("invalid source accepted")
	}
	r := rule()
	r.Roots[0].Children = []Node{{ID: "timeline", Type: "show", Props: map[string]any{"display": "bar", "duration": float64(5), "text": "{name}: {remaining}", "fill": "source", "dataSource": "bossTimer", "dataProvider": "blizzard", "dataTimelineSource": "script"}}}
	if err := Validate([]Rule{r}); err != nil {
		t.Fatal(err)
	}
	_, ds, ok := scopedShowSource(r.Roots, "timeline", displaySource{}, false)
	if !ok || ds.Boss.Source != "script" {
		t.Fatal("explicit source lost", ds)
	}
	values := sourceValues(ds, p)
	if values.Text["name"] != "Scripted" {
		t.Fatal(values)
	}
	r.Roots[0].Children[0].Props["dataTimelineSource"] = "wrong"
	if Validate([]Rule{r}) == nil {
		t.Fatal("bad display source accepted")
	}
}
