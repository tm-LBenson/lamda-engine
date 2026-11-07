package engine

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func buildLine(at time.Time, talents string) string {
	fields := []string{"COMBATANT_INFO", "Player-1-ABCD"}
	for len(fields) < 25 {
		fields = append(fields, "0")
	}
	fields = append(fields, "1468", talents, "(0,1241352,370960,378444)", "[(271501,311,(),(6652,13696),()),(171618,311,(),(),(240983,295))]", "[]", "144", "0", "0", "0")
	return at.Format("1/2/2006 15:04:05.000-07:00") + "  " + strings.Join(fields, ",")
}

func versionLine(at time.Time) string {
	return at.Format("1/2/2006 15:04:05.000-07:00") + "  COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1,BUILD_VERSION,12.1.0,PROJECT_ID,1"
}

func TestEvidenceProceedsWithoutBuildAndDoesNotConfirmReadiness(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	if !m.Observe(c, c.At.Add(10*time.Second)) {
		t.Fatal("missing build blocked inference")
	}
	s := m.Snapshot(c.At.Add(10 * time.Second))
	e := s.Cooldowns[0]
	if e.Status != "estimated_cooldown" || e.DeliverySeconds != 10 || e.AgeSeconds != 10 || e.BuildRevision != "" || !slices.Contains(e.Reasons, "missing_build") {
		t.Fatal(e)
	}
	if m.Snapshot(c.At.Add(121 * time.Second)).Cooldowns[0].Status != "estimated_ready" {
		t.Fatal("baseline expiry lost")
	}
	if m.Snapshot(c.At.Add(181 * time.Second)).Cooldowns[0].Status != "unknown" {
		t.Fatal("old evidence stayed ready")
	}
	if m.Diagnostics.Gaps["missing_build"].Count != 1 {
		t.Fatal("render ticks inflated gaps")
	}
	// Retained snapshots do not change when new diagnostics arrive.
	m.RecordGap("missing_build", c.At, c)
	if s.Diagnostics.Gaps["missing_build"].Count != 1 {
		t.Fatal("snapshot aliases diagnostics")
	}
	if len(m.Snapshot(c.At.Add(observationRetention+time.Second)).Cooldowns) != 0 {
		t.Fatal("unbounded evidence retention")
	}
}

func TestLateCastRetainedAsEvidenceAndFutureCastRejected(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	if m.Observe(c, c.At.Add(130*time.Second)) {
		t.Fatal("expired cast became an active overlay row")
	}
	s := m.Snapshot(c.At.Add(130 * time.Second))
	if len(s.Cooldowns) != 1 || s.Cooldowns[0].Status != "estimated_ready" || s.Diagnostics.Gaps["arrived_after_estimated_end"].Count != 1 {
		t.Fatal(s)
	}
	c.At = c.At.Add(500 * time.Second)
	if m.Observe(c, c.At.Add(-5*time.Second)) || m.Diagnostics.Gaps["future_event"].Count != 1 {
		t.Fatal("future event accepted")
	}
	if m.Observe(c, c.At.Add(observationRetention)) || m.Diagnostics.Gaps["event_too_old"].Count != 1 {
		t.Fatal("ancient event accepted")
	}
}

func TestUnknownChargesReuseDedupAndOrdering(t *testing.T) {
	c, _ := Parse(mirror)
	c.Spell = 363916
	m := NewModel()
	m.Observe(c, c.At)
	older := c
	c.At = c.At.Add(10 * time.Second)
	m.Observe(c, c.At)
	m.Observe(c, c.At)
	m.Observe(older, c.At)
	s := m.Snapshot(c.At)
	e := s.Cooldowns[0]
	if e.Status != "estimated_recharge" || e.AvailableCharges != nil || e.MaxCharges != nil || e.BaselineSeconds != 90 {
		t.Fatal(e)
	}
	if !slices.Contains(e.Reasons, "reuse_before_baseline") || s.Diagnostics.AcceptedCasts != 2 || s.Diagnostics.Duplicates != 1 || s.Diagnostics.OutOfOrder != 1 {
		t.Fatal(s.Diagnostics, e)
	}
	if m.Snapshot(c.At.Add(91 * time.Second)).Cooldowns[0].Status != "unknown" {
		t.Fatal("unknown charges became full charges")
	}
	b, _ := json.Marshal(e)
	if !strings.Contains(string(b), `"availableCharges":null`) {
		t.Fatal("unknown charge value not explicit", string(b))
	}
}

func TestResetEvidenceCannotBeReversedByDelayedCast(t *testing.T) {
	c, _ := Parse(mirror)
	c.Spell = 45438
	m := NewModel()
	m.Observe(c, c.At)
	r := c
	r.Spell = 235219
	r.At = r.At.Add(time.Second)
	m.Observe(r, r.At)
	m.Observe(c, r.At)
	e := m.Snapshot(r.At).Cooldowns[0]
	if e.Kind != "reset" || e.Status != "estimated_ready" || e.EventAt != r.At {
		t.Fatal(e)
	}
	m.Reset()
	if len(m.Snapshot(r.At).Cooldowns) != 0 || m.Diagnostics.AcceptedCasts != 2 || m.Diagnostics.SessionResets != 1 {
		t.Fatal("session reset lost audit or kept state")
	}
}

func TestBuildParserNestedFieldsPartialInputsAndRevisions(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	m.Ingest(versionLine(c.At), c.At, true)
	line := buildLine(c.At, "[(93236,115538,2),(93237,115539,1)]")
	m.Ingest(line, c.At, true)
	b := m.Builds[c.GUID]
	if b.SpecID != 1468 || len(b.Talents) != 2 || b.Talents[0] != [3]int{93236, 115538, 2} || len(b.Equipment) != 2 || len(b.Missing) != 0 || b.LogVersion != 22 || b.GameBuild != "12.1.0" {
		t.Fatalf("%+v", b)
	}
	if !m.Observe(c, c.At) || m.Evidence[c.GUID+"/55342"].BuildRevision != b.Revision || m.Diagnostics.Gaps["unmapped_build_modifiers"].Count != 1 {
		t.Fatal("build not attached")
	}
	next := c.At.Add(time.Second)
	m.Ingest(buildLine(next, "[(93236,115538,2),(93237,115539,1)]"), next, true)
	if m.Builds[c.GUID].Revision != b.Revision {
		t.Fatal("time changed build identity")
	}
	next = next.Add(time.Second)
	m.Ingest(buildLine(next, "[(93236,115538,1)]"), next, true)
	if m.Builds[c.GUID].Revision == b.Revision || !slices.Contains(m.Snapshot(next).Cooldowns[0].Reasons, "build_changed_since_cast") {
		t.Fatal("changed build unnoticed")
	}
	m.Ingest(line, next, true)
	if !m.Builds[c.GUID].At.Equal(next) || m.Diagnostics.Gaps["out_of_order_build"].Count != 1 {
		t.Fatal("old build replaced new build")
	}
	next = next.Add(time.Second)
	m.Ingest(buildLine(next, "[]"), next, true)
	b = m.Builds[c.GUID]
	if b.SpecID != 1468 || len(b.Equipment) != 2 || !slices.Contains(b.Missing, "talents") {
		t.Fatal("partial build lost useful fields", b)
	}
	c.At = next
	m.Observe(c, next)
	if m.Diagnostics.Gaps["partial_build"].Count != 1 {
		t.Fatal("partial metadata not diagnosed")
	}
	m.Ingest(buildLine(next, "[]"), next, true)
	if m.Diagnostics.Gaps["build_missing_talents"].Count != 1 {
		t.Fatal("duplicate build inflated missing-field count")
	}
	next = next.Add(time.Second)
	partial := strings.Replace(buildLine(next, "[(1,2,1)]"), "[(271501,311,(),(6652,13696),()),(171618,311,(),(),(240983,295))]", "[null,(171618,311,(),(),())]", 1)
	m.Ingest(partial, next, true)
	b = m.Builds[c.GUID]
	if len(b.Equipment) != 1 || b.SpecID != 1468 || !slices.Contains(b.Missing, "equipment") {
		t.Fatal("partial equipment discarded usable build or hid missing data", b)
	}
}

func TestMissingOrUnknownBuildSchemaDoesNotBlockCasts(t *testing.T) {
	c, _ := Parse(mirror)
	for _, version := range []int{0, 99} {
		m := NewModel()
		if version != 0 {
			m.Ingest(strings.Replace(versionLine(c.At), ",22,", ",99,", 1), c.At, true)
		}
		m.Ingest(buildLine(c.At, "[(93236,115538,2)]"), c.At, true)
		if !m.Observe(c, c.At) {
			t.Fatal("metadata uncertainty blocked cast")
		}
		if version == 0 && !slices.Contains(m.Builds[c.GUID].Missing, "log_version") {
			t.Fatal("missing schema assumed confirmed")
		}
		if version == 99 && (len(m.Builds) != 0 || m.Diagnostics.Gaps["unsupported_build_schema"].Count != 1) {
			t.Fatal("unknown schema guessed")
		}
	}
}

func TestMalformedBuildAndUnsupportedCastsAreDistinct(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	m.Ingest(buildLine(c.At, "[(1,2,3]"), c.At, true)
	m.Ingest(strings.Replace(mirror, ",55342,", ",1234567,", 1), c.At, true)
	m.Ingest(strings.Replace(mirror, "0x512", "0x518", 1), c.At, true)
	m.Ingest(strings.Replace(mirror, "0x512", "notflags", 1), c.At, true)
	if m.Diagnostics.Gaps["malformed_build"].Count != 1 || m.Diagnostics.UnsupportedCasts != 1 || m.Diagnostics.Gaps["malformed_cast"].Count != 1 || len(m.Evidence) != 0 {
		t.Fatal(m.Diagnostics)
	}
	for _, raw := range []string{`a,[(1,2),3]`, `a,"quoted, field",[(1,2)]`, `a,[]`} {
		if _, err := structuredFields(raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`a,[(1,2]`, `a,"missing quote`, strings.Repeat("[", 9) + strings.Repeat("]", 9), strings.Repeat("a", (64<<10)+1)} {
		if _, err := structuredFields(raw); err == nil {
			t.Fatal("malformed nesting accepted")
		}
	}
}

func TestReplayUsesEventClockAndKeepsAuditAcrossSessions(t *testing.T) {
	c, _ := Parse(mirror)
	input := versionLine(c.At) + "\n" + buildLine(c.At, "[(1,2,1)]") + "\n" + strings.Replace(mirror, "0x512", "0x511", 1) + "\n" + versionLine(c.At.Add(time.Second)) + "\n" + mirror + "\n"
	s, err := ReplayAudit(strings.NewReader(input), 15*time.Second)
	if err != nil || s.Diagnostics.AcceptedCasts != 2 || s.Diagnostics.SelfCasts != 1 || s.Diagnostics.SessionResets != 2 || s.Diagnostics.BuildRecords != 1 || len(s.Cooldowns) != 1 || len(s.Builds) != 0 || s.LogVersion != 22 {
		t.Fatal(s, err)
	}
	if s.Diagnostics.Gaps["late_delivery"].Count != 2 {
		t.Fatal("simulated delay not recorded")
	}
}

func TestBuildAndGapMemoryBounds(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	for i := 0; i < 160; i++ {
		line := strings.Replace(buildLine(c.At.Add(time.Duration(i)*time.Second), "[(1,2,1)]"), c.GUID, fmt.Sprintf("Player-1-%d", i), 1)
		m.Ingest(line, c.At.Add(time.Duration(i)*time.Second), true)
		m.RecordGap(fmt.Sprintf("gap_%d", i), c.At, c)
	}
	if len(m.Builds) > 128 || len(m.Diagnostics.Gaps) > 65 {
		t.Fatal("unbounded diagnostic state")
	}
	if len(m.Snapshot(c.At.Add(30*time.Minute)).Builds) != 0 {
		t.Fatal("stale builds retained")
	}
}
