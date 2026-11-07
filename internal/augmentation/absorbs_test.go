package augmentation

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"testing"
	"time"
)

func absorbedRecord(s *Store, second int, event, source string, amount, overkill, absorbed int, owner string) {
	f := actor(event, source, "Creature-1", 1)
	f[7] = "0xa48"
	for i := 0; i < 19; i++ {
		f = append(f, "0")
	}
	f = append(f, intString(amount), intString(amount), intString(overkill), "12", "0", "0", intString(absorbed), "nil", "nil", "nil")
	if owner != "" {
		f = append(f, owner)
	} else {
		f = append(f, "ST")
	}
	ingest(s, second, f...)
}

func TestShieldDamageAndFullyAbsorbedSupportConserveGroup(t *testing.T) {
	s := ready()
	start(s, 1)
	spec(s, 1, aug, 1468)
	absorbedRecord(s, 2, "SPELL_DAMAGE", other, 100, 20, 30, "") // 80 health + 30 shield
	f := actor("SPELL_MISSED", other, "Creature-1", 1)
	f[7] = "0xa48"
	ingest(s, 3, append(f, "ABSORB", "nil", "40", "40", "nil", "ST")...)
	damageRecord(s, 3, "SPELL_DAMAGE", aug, "Creature-1", 50, 1, "")
	absorbedRecord(s, 4, "SPELL_DAMAGE_SUPPORT", other, 10, -1, 5, aug)
	absorbedRecord(s, 4, "SPELL_DAMAGE_SUPPORT", other, 0, -1, 8, aug)
	end(s, 10)
	v := s.DamageView("boss")
	if v == nil || !v.Valid || v.RawTotal != 200 || v.AdjustedTotal != 200 || v.DamageBasis != "health+absorbs" || findRow(t, v, aug).Adjusted != 73 || findRow(t, v, other).Adjusted != 127 {
		t.Fatal(v)
	}
}

func TestEveryFullAbsorbPayloadAndNonDamageMiss(t *testing.T) {
	for _, event := range []string{"SWING_MISSED", "SPELL_MISSED", "SPELL_PERIODIC_MISSED", "SPELL_BUILDING_MISSED", "RANGE_MISSED", "DAMAGE_SHIELD_MISSED"} {
		s := ready()
		start(s, 1)
		f := actor(event, aug, "Creature-1", 1)
		f[3] = "0x511"
		f[7] = "0xa48"
		if event == "SWING_MISSED" {
			f = f[:9]
		}
		ingest(s, 2, append(append([]string(nil), f...), "ABSORB", "nil", "40", "40", "nil", "ST")...)
		ingest(s, 3, append(f, "IMMUNE", "nil", "999", "999", "nil", "ST")...)
		end(s, 10)
		v := s.DamageView("boss")
		if v == nil || !v.Valid || v.RawTotal != 40 || v.RejectedHooks != 0 {
			t.Fatalf("%s: %+v", event, v)
		}
	}
}

func TestOrdinaryShortMeleeMissDoesNotFlagDamageAttributionPartial(t *testing.T) {
	s := ready()
	start(s, 1)
	damageRecord(s, 2, "SPELL_DAMAGE", aug, "Creature-1", 100, 1, "")
	f := actor("SWING_MISSED", other, "Creature-1", 1)[:9]
	f[7] = "0xa48"
	ingest(s, 3, append(f, "MISS", "nil")...)
	end(s, 10)
	v := s.DamageView("boss")
	if v == nil || v.Partial || v.RejectedHooks != 0 || v.RawTotal != 100 {
		t.Fatal("normal miss mislabeled as lost damage", v)
	}
	if _, reason := recordedDamageAmount(append(f, "ABSORB", "nil"), 9); reason != "short_damage_payload" {
		t.Fatal("truncated absorb no longer rejected", reason)
	}
}

func TestCompletedNativeKeyUsesExactRowsWhenItsClockDiffers(t *testing.T) {
	s := ready()
	now := time.Now()
	s.damageMeters = []DamageMeter{{ID: "key/1", Kind: "key", Name: "Key", Owner: aug, AttributionMode: "all", DamageBasis: "health+absorbs", Valid: true, CollectionValid: true, RawTotal: 300, AdjustedTotal: 300, Seconds: 20,
		Rows: []DamageRow{{GUID: aug, Raw: 100, Added: 20, Adjusted: 120}, {GUID: other, Raw: 200, Removed: 20, Adjusted: 180}}}}
	n := nativeFrame(1, false, 100, 200)
	n.Frame = &lookup.NativeMeterFrame{Scope: "native:68"}
	s.ObserveNative(n, now, "")
	v := s.HybridView(now)
	if v == nil || v.Provisional || v.InputSource != "native+recorded" || v.Seconds != 10 || findRow(t, v, aug).DPS != 12 {
		t.Fatal(v)
	}
}
