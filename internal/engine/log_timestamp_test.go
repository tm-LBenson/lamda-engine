package engine

import (
	"testing"
	"time"
)

func TestCombatLogUnsignedUTCOffsetAndHeaderAgree(t *testing.T) {
	cases := []struct{ stamp, utc string }{
		{"3/29/2025 19:57:40.9471", "2025-03-29T18:57:40.947Z"},
		{"10/13/2026 00:00:00.00110", "2026-10-12T14:00:00.001Z"},
		{"10/13/2026 00:00:00.0010", "2026-10-13T00:00:00.001Z"},
		{"10/13/2026 00:00:00.001-4", "2026-10-13T04:00:00.001Z"},
		{"10/13/2026 00:00:00.001+5:30", "2026-10-12T18:30:00.001Z"},
	}
	for _, c := range cases {
		line := c.stamp + "  SPELL_HEAL,Player-1-SOURCE,Source,0x511,0,Player-1-TARGET,Target,0x511,0,123,Heal,2"
		at, fields, e := RecordFields(line)
		if e != nil || len(fields) != 12 || at.UTC().Format(time.RFC3339Nano) != c.utc {
			t.Fatal(c, at, fields, e)
		}
		header, kind, e := RecordHeader(line)
		if e != nil || kind != "SPELL_HEAL" || !header.Equal(at) {
			t.Fatal("header disagrees", header, kind, e)
		}
	}
	for _, stamp := range []string{"10/13/2026 00:00:00.00199", "10/13/2026 00:00:00.001123", "10/13/2026 00:00:00.001+4:99"} {
		if _, _, e := RecordFields(stamp + "  UNIT_DIED"); e == nil {
			t.Fatal("invalid zone accepted", stamp)
		}
	}
	line := "10/13/2026 00:00:00.001  UNIT_DIED"
	at, _, e := RecordFields(line)
	if e != nil || !at.Equal(time.Date(2026, 10, 13, 0, 0, 0, 1e6, time.Local)) {
		t.Fatal("unmarked local timestamp changed", at, e)
	}
}
