package augmentation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReportLabelUsesEncounterAugPresenceNotOtherSupportTransfers(t *testing.T) {
	s := ready()
	for i, augSpec := range []bool{false, true, false} {
		offset := i * 20
		start(s, offset+1)
		id := 1467
		if augSpec {
			id = 1473
		}
		spec(s, offset+1, aug, id)
		damageRecord(s, offset+2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
		damageRecord(s, offset+2, "SPELL_DAMAGE", other, "Creature-1", 300, 1, "")
		// Bombardments can transfer damage for Devastation too.
		damageRecord(s, offset+3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 20, 434481, aug)
		end(s, offset+10)
		m := s.damageMeters[0]
		if !m.AugPresenceKnown || m.HasAug != augSpec {
			t.Fatal(m)
		}
		v, err := s.PreparedSummaryForPlayer("boss", aug)
		if err != nil || strings.Contains(v.Lines[0], "Aug-adjusted") != augSpec {
			t.Fatal(v, err)
		}
	}
}

func TestReportVariantsArePreparedAndDamageOptionsKeepOtherAwards(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1467)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 300, 1, "")
	ingest(s, 3, actor("SPELL_DISPEL", other, "Creature-1", 1)...)
	end(s, 10)
	for option := uint8(0); option < 4; option++ {
		v, err := s.PreparedSummaryForPlayer("boss", aug, option)
		if err != nil || !strings.Contains(v.Text, "Clean-up crew") {
			t.Fatal(v, err)
		}
		if strings.Contains(v.Text, "1. ") != (option&1 == 0) || strings.Contains(v.Text, "Damage MVP:") != (option&2 == 0) {
			t.Fatal(option, v)
		}
		if option&1 != 0 && (!strings.Contains(v.Lines[0], "Awards") || strings.Contains(v.Lines[0], "DPS")) {
			t.Fatal(v)
		}
		v.Lines[0] = "mutated"
		again, _ := s.PreparedSummaryForPlayer("boss", aug, option)
		if again.Lines[0] == "mutated" {
			t.Fatal("cached variant was mutable")
		}
	}
	if _, err := s.PreparedSummaryForPlayer("boss", aug, 5); err == nil {
		t.Fatal("unsupported options allowed")
	}
}

func TestRankingsOnlySharesRecordedAugRowsWithoutAwardsOrUptime(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1473)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	damageRecord(s, 2, "SPELL_DAMAGE", other, "Creature-1", 300, 1, "")
	damageRecord(s, 3, "SPELL_DAMAGE_SUPPORT", other, "Creature-1", 100, 1, aug)
	ingest(s, 3, actor("SPELL_DISPEL", other, "Creature-1", 1)...)
	end(s, 10)
	v, err := s.PreparedSummaryForPlayer("boss-uptime", aug, 4)
	if err != nil || len(v.Lines) != 3 || !strings.Contains(v.Lines[0], "Aug-adjusted DPS") {
		t.Fatal(v, err)
	}
	for _, line := range v.Lines[1:] {
		if !strings.Contains(line, "DPS") || strings.Contains(line, "MVP") || strings.Contains(line, "Clean-up") || strings.Contains(line, "uptime") {
			t.Fatal(v)
		}
	}
	full, err := s.PreparedSummaryForPlayer("boss", aug)
	if err != nil || !strings.Contains(full.Text, "Clean-up crew") {
		t.Fatal("rankings-only share changed the full report", full, err)
	}
}

func TestNativeBoundaryDoesNotSharePreviousBossWhileFinalLogIsLate(t *testing.T) {
	s := ready()
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	end(s, 10)
	latest := s.damageMeters[0]
	for _, test := range []struct {
		end       time.Time
		encounter string
	}{
		{latest.EndedAt.Add(20 * time.Second), "123"},
		{latest.EndedAt, "456"},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		_, err := s.SummaryForBoundaryWhenReady(ctx, "boss", aug, test.end, test.encounter)
		cancel()
		if !errors.Is(err, errSummaryPending) {
			t.Fatal("stale boss escaped", err)
		}
	}
	v, err := s.SummaryForBoundaryWhenReady(context.Background(), "boss", aug, latest.EndedAt, "123", 3)
	if err != nil || v.Segment != latest.ID {
		t.Fatal(v, err)
	}
}
