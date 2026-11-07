package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func combatTestRule(kind string) Rule {
	r := rule()
	r.Roots[0].Props = map[string]any{"event": "combatlog", "combatEvent": kind, "sourceUnit": "player", "targetUnit": "any"}
	r.Roots[0].Children = []Node{{ID: "display", Type: "show", Props: map[string]any{"display": "text", "text": "{sourceName}: {name} → {targetName} {amount} / {extraName}", "duration": float64(5)}}}
	return r
}
func combatTestSample(rs []Rule, at time.Time) lookup.BridgeSample {
	s := sample(rs, at, 1)
	p := s.Overlays
	p.Watches = nil
	p.Hash = WatchHash([]Watch{})
	p.CombatVersion = 1
	p.Logging = ptr(true)
	p.IdentityHash = CastHash(CombatWatches(rs))
	for _, u := range CombatWatches(rs) {
		id := "Creature-1-Enemy"
		if u == "player" {
			id = "Player-1-Mine"
		}
		p.Identities = append(p.Identities, lookup.OverlayIdentity{Unit: u, ID: id, Generation: 1})
	}
	return s
}
func combatAt(fields []string, at time.Time) string {
	line := combatLine(fields)
	_, csv, _ := strings.Cut(line, "  ")
	return at.UTC().Format("1/2/2006 15:04:05.000") + "+0  " + csv
}
func combatMine(t *testing.T, raw string) []string {
	f := append([]string(nil), combatFixtures(t)[raw]...)
	f[1] = "Player-1-Mine"
	f[2] = "Source"
	f[3] = "0x511"
	f[5] = "Creature-1-Enemy"
	f[6] = "Target"
	f[7] = "0xa48"
	return f
}
func combatSetup(t *testing.T, r Rule) (*Store, lookup.BridgeSample, time.Time) {
	t.Helper()
	if e := Validate([]Rule{r}); e != nil {
		t.Fatal(e)
	}
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.doc.Rules = []Rule{r}
	at := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	s.combat.started = at
	sample := combatTestSample(s.doc.Rules, at)
	s.Accept(sample, at)
	s.IngestCombatLog(combatAt(combatFixtures(t)["COMBAT_LOG_VERSION"], at), at)
	return s, sample, at
}
func TestCombatLoggerFreshnessAndConnectionBoundary(t *testing.T) {
	s, sample, at := combatSetup(t, combatTestRule("damage"))
	f := combatMine(t, "SPELL_DAMAGE")
	wrong := append([]string(nil), f...)
	wrong[1] = "Player-1-Another"
	s.IngestCombatLog(combatAt(wrong, at.Add(time.Second)), at.Add(time.Second))
	if len(s.runtime.displays) != 0 {
		t.Fatal("other logger triggered")
	}
	for _, delta := range []time.Duration{-time.Millisecond, 5 * time.Second} {
		s.IngestCombatLog(combatAt(f, at.Add(delta)), at)
		if len(s.runtime.displays) != 0 {
			t.Fatal("history/future triggered")
		}
	}
	s.IngestCombatLog(combatAt(f, at.Add(time.Second)), at.Add(time.Second))
	if len(s.runtime.displays) != 1 {
		t.Fatal("current logger failed", s.runtime.problem)
	}
	serial := s.runtime.serial
	s.IngestCombatLog(combatAt(f, at.Add(time.Second)), at.Add(time.Second))
	if s.runtime.serial != serial+1 {
		t.Fatal("equal hits were collapsed")
	}
	s.runtime.clearRule(s.doc.Rules[0].ID)
	s.IngestCombatLog(combatAt(f, at.Add(10*time.Second)), at.Add(10*time.Second))
	if len(s.runtime.displays) != 0 {
		t.Fatal("stale capture")
	}
	sample.Overlays.Seq++
	sample.Overlays.At = at.Add(10 * time.Second).Unix()
	s.Accept(sample, at.Add(10*time.Second))
	s.IngestCombatLog(combatAt(f, at.Add(9*time.Second)), at.Add(10*time.Second))
	if len(s.runtime.displays) != 0 {
		t.Fatal("reconnect replay")
	}
	sample.Overlays.Logging = ptr(false)
	sample.Overlays.Seq++
	s.Accept(sample, at.Add(10*time.Second))
	s.IngestCombatLog(combatAt(f, at.Add(10*time.Second)), at.Add(10*time.Second))
	if len(s.runtime.displays) != 0 {
		t.Fatal("logging off")
	}
}
func TestCombatDelayedContextAndEventValueGuards(t *testing.T) {
	r := combatTestRule("damage")
	show := r.Roots[0].Children[0]
	show.Props["visibility"] = "conditions"
	r.Roots[0].Children = []Node{{ID: "check", Type: "if", Props: map[string]any{"condition": "eventValue", "field": "amount", "operator": "gte", "value": float64(6000)}, Children: []Node{{ID: "pause", Type: "wait", Props: map[string]any{"seconds": float64(1)}}, show}}}
	s, sample, at := combatSetup(t, r)
	f := combatMine(t, "SPELL_DAMAGE")
	s.IngestCombatLog(combatAt(f, at.Add(100*time.Millisecond)), at.Add(100*time.Millisecond))
	if len(s.runtime.queue) != 1 {
		t.Fatal("not queued", s.runtime.queue)
	}
	other := combatMine(t, "SPELL_HEAL")
	s.IngestCombatLog(combatAt(other, at.Add(200*time.Millisecond)), at.Add(200*time.Millisecond))
	sample.Overlays.Seq++
	sample.Overlays.At = at.Add(time.Second).Unix()
	s.Accept(sample, at.Add(1200*time.Millisecond))
	ds := s.Merge(engine.FrameDisplay{}, at.Add(1200*time.Millisecond)).Overlays
	if len(ds) != 1 || !strings.Contains(ds[0].Text, "6134") || !strings.Contains(ds[0].Text, f[10]) {
		t.Fatal("original event lost", ds)
	}
	sample.Overlays.Seq++
	s.Accept(sample, at.Add(1300*time.Millisecond))
	if len(s.runtime.displays) != 1 {
		t.Fatal("event guard became unknown on next capture")
	}
	s.combat.started = at.Add(1400 * time.Millisecond)
	s.runtime.clearRule(r.ID)
	s.IngestCombatLog(combatAt(f, at.Add(time.Second)), at.Add(1500*time.Millisecond))
	if len(s.runtime.displays) != 0 {
		t.Fatal("rotation replay")
	}
}
func TestCombatSourceFiltersUnknownAndTargetReplacement(t *testing.T) {
	r := combatTestRule("interrupt")
	r.Roots[0].Props["targetUnit"] = "target"
	r.Roots[0].Children[0].Props["display"] = "icon"
	r.Roots[0].Children[0].Props["icon"] = "auto"
	r.Roots[0].Children[0].Props["dataSource"] = "affected"
	s, sample, at := combatSetup(t, r)
	f := combatMine(t, "SPELL_INTERRUPT")
	s.IngestCombatLog(combatAt(f, at.Add(time.Second)), at.Add(time.Second))
	ds := s.Merge(engine.FrameDisplay{}, at.Add(time.Second)).Overlays
	event, _ := parseCombatRecord(combatAt(f, at), s.combat.format)
	if len(ds) != 1 || ds[0].Spell != event.ExtraSpell {
		t.Fatal("affected spell", ds)
	}
	for i := range sample.Overlays.Identities {
		if sample.Overlays.Identities[i].Unit == "target" {
			sample.Overlays.Identities[i].Generation++
			sample.Overlays.Identities[i].ID = "Creature-1-Replaced"
		}
	}
	sample.Overlays.Seq++
	s.Accept(sample, at.Add(1100*time.Millisecond))
	if len(s.runtime.displays) != 0 {
		t.Fatal("old target retained")
	}
	s.IngestCombatLog(combatAt(f, at.Add(1200*time.Millisecond)), at.Add(1200*time.Millisecond))
	if len(s.runtime.displays) != 0 {
		t.Fatal("old target matched")
	}
	q := map[string]any{"condition": "group", "mode": "not", "tests": []any{map[string]any{"condition": "eventValue", "field": "amount", "operator": "gt", "value": float64(0)}}}
	if predicate(q, sample.Overlays, "player", &triggerContext{Combat: &event}) != unknown {
		t.Fatal("missing amount inverted")
	}
}
func TestCombatSeedReadsOnlyMetadataAndReplacesUnknownHeader(t *testing.T) {
	at := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	header := combatAt(combatFixtures(t)["COMBAT_LOG_VERSION"], at)
	f := combatMine(t, "SPELL_DAMAGE")
	path := filepath.Join(t.TempDir(), "log.txt")
	content := header + "\n" + combatAt(f, at) + "\n"
	if e := os.WriteFile(path, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
	feed := combatFeed{}
	if e := feed.seed(path, int64(len(content)), at.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if feed.logger != "Player-1-Mine" || feed.format.Version != 22 || !feed.started.Equal(at.Add(time.Second)) {
		t.Fatal(feed)
	}
	feed.observe(combatAt([]string{"COMBAT_LOG_VERSION", "99"}, at))
	if feed.logger != "" || feed.format.Known {
		t.Fatal("unknown header retained metadata")
	}
	f[3] = "0x512"
	feed.observe(combatAt(f, at))
	if feed.logger != "" {
		t.Fatal("party mistaken for logger")
	}
	f[3] = "0x1111"
	f[1] = "Pet-1-Mine"
	feed.observe(combatAt(f, at))
	if feed.logger != "" {
		t.Fatal("pet mistaken for player")
	}
	f = combatMine(t, "SPELL_DAMAGE")
	f[5] = "Player-1-Other"
	f[7] = "0x511"
	feed.observe(combatAt(f, at))
	if feed.logger != "" {
		t.Fatal("conflicting mine identities")
	}
}

func TestCombatNestedEventsAndAppearanceKeepTheirOwnTrigger(t *testing.T) {
	r := combatTestRule("damage")
	show := r.Roots[0].Children[0]
	show.Props["text"] = "{amount}"
	r.Roots[0].Children = []Node{{ID: "next", Type: "when", Props: map[string]any{"event": "combatlog", "combatEvent": "heal", "sourceUnit": "player"}, Children: []Node{show}}}
	r.Style.Modifiers = []AppearanceModifier{{Condition: map[string]any{"condition": "eventValue", "field": "amount", "operator": "gt", "value": float64(40000)}, Color: "red", Sound: "chime"}}
	s, sample, at := combatSetup(t, r)
	damage := combatMine(t, "SPELL_DAMAGE")
	heal := combatMine(t, "SPELL_HEAL")
	s.IngestCombatLog(combatAt(heal, at.Add(50*time.Millisecond)), at.Add(50*time.Millisecond))
	if len(s.runtime.displays) != 0 {
		t.Fatal("nested event before parent")
	}
	s.IngestCombatLog(combatAt(damage, at.Add(100*time.Millisecond)), at.Add(100*time.Millisecond))
	s.IngestCombatLog(combatAt(heal, at.Add(200*time.Millisecond)), at.Add(200*time.Millisecond))
	sample.Overlays.Seq++
	s.Accept(sample, at.Add(300*time.Millisecond))
	out := s.Merge(engine.FrameDisplay{}, at.Add(300*time.Millisecond))
	if len(out.Overlays) != 1 || out.Overlays[0].Text != "44798" || out.Overlays[0].Color != colorValue("red") || len(out.OverlaySounds) != 1 {
		t.Fatal(out.Overlays, out.OverlaySounds)
	}
	sample.Overlays.Seq++
	s.Accept(sample, at.Add(400*time.Millisecond))
	if len(s.runtime.sounds) != 1 {
		t.Fatal("appearance sound repeated")
	}
}
func TestCombatWatchContinuityLimitsAndUnknownFilters(t *testing.T) {
	r := combatTestRule("damage")
	r.Roots[0].Props["targetUnit"] = "target"
	rs := []Rule{r}
	at := time.Now()
	sample := combatTestSample(rs, at)
	added := combatTestRule("heal")
	added.ID = "added"
	added.Roots[0].Props["targetUnit"] = "focus"
	rt := NewRuntime()
	rt.Accept(append(rs, added), sample, at)
	if len(rt.last.Identities) != 2 || rt.problem != "Reload WoW for combat events" {
		t.Fatal(rt.problem, rt.last.Identities)
	}
	e := combatEvent{Kind: "damage", Source: combatActor{GUID: "Player-1-Mine"}, Target: combatActor{GUID: "Creature-1-Enemy"}, School: 20, Spell: 123, ExtraSpell: 456, RawKind: "SPELL_PERIODIC_DAMAGE", Values: map[string]float64{}}
	p := r.Roots[0].Props
	p["school"] = "4"
	p["periodic"] = "yes"
	if !combatMatches(p, rt.last, "player", &e) {
		t.Fatal("periodic multischool")
	}
	p["critical"] = "no"
	if combatMatches(p, rt.last, "player", &e) {
		t.Fatal("unknown critical became false")
	}
	e.Critical = ptr(false)
	if !combatMatches(p, rt.last, "player", &e) {
		t.Fatal("known false")
	}
	sample.Overlays.Identities[1].ID = ""
	if combatMatches(p, sample.Overlays, "player", &e) {
		t.Fatal("unknown target matched")
	}
}
func TestCombatBufferedLogReportsDelayAndRecovers(t *testing.T) {
	s, sample, at := combatSetup(t, combatTestRule("damage"))
	f := combatMine(t, "SPELL_DAMAGE")
	s.IngestCombatLog(combatAt(f, at.Add(-10*time.Second)), at)
	if s.runtime.combatProblem != "Combat log delayed" || len(s.runtime.displays) != 0 {
		t.Fatal(s.runtime.combatProblem)
	}
	sample.Overlays.Seq++
	s.Accept(sample, at.Add(100*time.Millisecond))
	if s.runtime.combatProblem != "Combat log delayed" {
		t.Fatal("capture concealed lag")
	}
	s.IngestCombatLog(combatAt(f, at.Add(200*time.Millisecond)), at.Add(200*time.Millisecond))
	if s.runtime.combatProblem != "" || len(s.runtime.displays) != 1 {
		t.Fatal("did not recover", s.runtime.combatProblem)
	}
}
