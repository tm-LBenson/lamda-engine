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

func encounterRule(event string) Rule {
	r := rule()
	r.Roots[0].Props = map[string]any{"event": event, "encounterID": "1234", "difficulty": "15"}
	r.Roots[0].Children = []Node{{ID: "display", Type: "show", Props: map[string]any{"display": "text", "text": "{name} {outcome} {elapsed}s {encounterID}", "duration": float64(5)}}}
	return r
}
func encounterSample(rs []Rule, at time.Time, seq int, e *lookup.OverlayEncounter) lookup.BridgeSample {
	s := sample(rs, at, seq)
	s.Overlays.Watches = nil
	s.Overlays.Hash = WatchHash([]Watch{})
	s.Overlays.EncountersVersion = 1
	s.Overlays.Encounter = e
	return s
}
func TestEncounterConditionsUnknownFiltersAndComparisons(t *testing.T) {
	p := &lookup.OverlayInput{EncountersVersion: 1, Encounter: &lookup.OverlayEncounter{Active: ptr(true), ID: 1234, Difficulty: 15, Elapsed: ptr(60.0)}}
	q := map[string]any{"condition": "encounter", "check": "elapsed", "encounterID": "1234", "difficulty": "15", "operator": "gte", "value": float64(60)}
	for _, op := range []string{"lt", "lte", "eq", "ne", "gt", "gte"} {
		q["operator"] = op
		want := truthOf(compareNumber(60, 60, op))
		if got := predicate(q, p, ""); got != want {
			t.Fatal(op, got, want)
		}
	}
	q["check"] = "active"
	p.Encounter.ID = 0
	if predicate(q, p, "") != unknown {
		t.Fatal("unknown ID matched")
	}
	q["check"] = "idle"
	if predicate(q, p, "") != unknown {
		t.Fatal("unknown became inactive")
	}
	p.Encounter.ID = 5678
	if predicate(q, p, "") != yes {
		t.Fatal("different encounter not inactive")
	}
	p.Encounter.Active = ptr(false)
	p.Encounter.ID = 0
	if predicate(q, p, "") != yes {
		t.Fatal("known idle")
	}
	p.Encounter.Active = nil
	not := map[string]any{"condition": "group", "mode": "not", "tests": []any{q}}
	if predicate(not, p, "") != unknown {
		t.Fatal("NOT unknown passed")
	}
	p.Encounter.Active = ptr(true)
	p.Encounter.ID = 1234
	p.Encounter.Elapsed = nil
	q["check"] = "elapsed"
	if predicate(q, p, "") != unknown {
		t.Fatal("elapsed fabricated")
	}
}
func TestEncounterEventFiltersFreshnessRepeatsAndWaitContext(t *testing.T) {
	r := encounterRule("encounterend")
	r.Roots[0].Props["outcome"] = "victory"
	r.Roots[0].Children = append([]Node{{ID: "wait", Type: "wait", Props: map[string]any{"seconds": float64(1)}}}, r.Roots[0].Children...)
	rs := []Rule{r}
	if e := Validate(rs); e != nil {
		t.Fatal(e)
	}
	st, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	st.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	end := lookup.OverlayEncounterEvent{Seq: 1, Kind: "encounterend", ID: 1234, Name: "Original", Difficulty: 15, Success: ptr(true), Elapsed: ptr(42.5)}
	state := &lookup.OverlayEncounter{Active: ptr(false), Seq: 1, Events: []lookup.OverlayEncounterEvent{end}}
	st.Accept(encounterSample(rs, at, 1, state), at)
	if len(st.runtime.queue) != 0 {
		t.Fatal("buffer replayed on connect")
	}
	captureSeq := 1
	emit := func(seq int, entry lookup.OverlayEncounterEvent, elapsed time.Duration) {
		at = at.Add(elapsed)
		state = &lookup.OverlayEncounter{Active: ptr(false), Seq: seq, Events: []lookup.OverlayEncounterEvent{entry}}
		captureSeq++
		st.Accept(encounterSample(rs, at, captureSeq, state), at)
	}
	for i, entry := range []lookup.OverlayEncounterEvent{
		{Kind: "encounterend", ID: 1234, Difficulty: 15, Success: nil},
		{Kind: "encounterend", ID: 1234, Difficulty: 15, Success: ptr(false)},
		{Kind: "encounterend", ID: 5678, Difficulty: 15, Success: ptr(true)},
		{Kind: "encounterend", ID: 1234, Difficulty: 16, Success: ptr(true)},
	} {
		entry.Seq = i + 2
		emit(entry.Seq, entry, 200*time.Millisecond)
		if len(st.runtime.queue) > 0 {
			t.Fatal("wrong/unknown filter matched", entry)
		}
	}
	end.Seq = 6
	emit(6, end, 200*time.Millisecond)
	if len(st.runtime.queue) != 1 {
		t.Fatal("matching result failed", st.runtime.problem)
	}
	// Future captures may name another fight. WAIT still owns the original event.
	for i := 1; i <= 6; i++ {
		at = at.Add(200 * time.Millisecond)
		next := *state
		next.Name = "New boss"
		next.Events = nil
		captureSeq++
		st.Accept(encounterSample(rs, at, captureSeq, &next), at)
	}
	d := st.Merge(engine.FrameDisplay{}, at)
	if len(d.Overlays) != 1 || d.Overlays[0].Text != "Original Victory 42.5s 1234" {
		t.Fatalf("lost trigger: %+v", d.Overlays)
	}
	end.Seq = 7
	emit(7, end, 200*time.Millisecond)
	if len(st.runtime.queue) != 1 {
		t.Fatal("second encounter did not rearm")
	}
	// Reconnect discards queued output and buffered event history.
	end.Seq = 8
	emit(8, end, 10*time.Second)
	if len(st.runtime.queue) != 0 || len(st.runtime.displays) != 0 {
		t.Fatal("reconnect replay")
	}
	end.Seq = 1
	emit(1, end, 200*time.Millisecond)
	if len(st.runtime.queue) != 0 {
		t.Fatal("sequence reset replay")
	}
}
func TestEncounterPersistentElapsedAndSeparateLiveSource(t *testing.T) {
	r := encounterRule("state")
	r.Roots[0].Props = map[string]any{"event": "state"}
	show := r.Roots[0].Children[0]
	show.Props["visibility"] = "conditions"
	show.Props["text"] = "{name} {elapsed}"
	r.Roots[0].Children = []Node{{ID: "fight", Type: "if", Props: map[string]any{"condition": "encounter", "check": "elapsed", "operator": "gte", "value": float64(60)}, Children: []Node{show}}}
	rs := []Rule{r}
	s, _ := Open(t.TempDir())
	s.doc.Rules = rs
	at := time.Unix(2000000000, 0)
	for i, elapsed := range []float64{59, 60, 61} {
		p := &lookup.OverlayEncounter{Active: ptr(true), ID: 1234, Name: "Live boss", Elapsed: &elapsed}
		s.Accept(encounterSample(rs, at, i+1, p), at)
		d := s.Merge(engine.FrameDisplay{}, at)
		if i == 0 && len(d.Overlays) != 0 || i > 0 && len(d.Overlays) != 1 {
			t.Fatal("threshold", i, d.Overlays)
		}
		if i == 2 && d.Overlays[0].Text != "Live boss 61" {
			t.Fatal("not updated", d.Overlays[0].Text)
		}
		at = at.Add(200 * time.Millisecond)
	}
	s.Accept(encounterSample(rs, at, 4, &lookup.OverlayEncounter{Active: ptr(false)}), at)
	if len(s.runtime.displays) != 0 {
		t.Fatal("held display survived end")
	}
	data := sourceValues(displaySource{Kind: "encounter"}, s.runtime.last)
	if data.Text["name"] != "" {
		t.Fatal("inactive live source retained old boss")
	}
}
func TestEncounterDemandAndPublishedFlag(t *testing.T) {
	r := encounterRule("encounterstart")
	if !needsEncounters([]Rule{r}) {
		t.Fatal("event demand missing")
	}
	r.Roots[0].Props["outcome"] = "victory"
	if Validate([]Rule{r}) == nil {
		t.Fatal("victory on start allowed")
	}
	delete(r.Roots[0].Props, "outcome")
	r.Enabled = false
	if needsEncounters([]Rule{r}) {
		t.Fatal("disabled rule collects")
	}
	r.Enabled = true
	s, _ := Open(t.TempDir())
	s.doc.Rules = []Rule{r}
	path := filepath.Join(t.TempDir(), "OverlaysData.lua")
	if err := os.WriteFile(path, []byte(Marker+"\nLamdaOverlaysData={}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.SetGameFile(path)
	if s.publishError != "" {
		t.Fatal(s.publishError)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), `["encounters"]=true`) {
		t.Fatal("encounter flag not published", err)
	}
}
