package augmentation

import (
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func TestSelectedWindowSharesEveryNativeMetricWithoutSwitchingToDamage(t *testing.T) {
	for metric := 2; metric <= 10; metric++ {
		s := ready()
		n := windowFrame(2, metric, 1, 8, 12)
		n.Spec = 1467
		n.SessionID = 77
		n.Frame.Scope, n.Frame.Label = "native:77", "Selected boss"
		if metric == 5 || metric == 6 || metric == 9 {
			n.DurationMillis = 0 // counts do not require a rate clock
		}
		s.ObserveNativeSlot(n, 1, time.Now(), "")
		damage := windowFrame(1, 0, 1, 10000, 20000)
		damage.Spec = 1467
		s.ObserveNativeSlot(damage, 0, time.Now(), "")
		v, err := s.PreparedSummaryForPlayer("window:2", aug, 4)
		if err != nil || !strings.Contains(v.Text, "Selected boss - "+meterMetricNames[metric]) || strings.Contains(v.Text, "DPS") || strings.Contains(v.Text, "MVP") {
			t.Fatal(metric, err, v)
		}
		if !strings.Contains(v.Text, "12") || strings.Contains(v.Text, "20000") {
			t.Fatal("wrong window's amount", metric, v)
		}
		if metric == 5 || metric == 6 || metric == 9 {
			if strings.Contains(v.Text, "/s") {
				t.Fatal("utility counts became rates", v)
			}
		}
		if _, err := s.PreparedSummaryForPlayer("window:3", aug, 4); err == nil {
			t.Fatal("missing window fell back to damage")
		}
	}
}

func TestSelectedWindowSharesCapturedSpellAndTargetViewsWithoutParentCapture(t *testing.T) {
	for _, view := range []string{"spells", "targets"} {
		s := ready()
		s.own = aug
		n := windowFrame(1, 5, 1, 0, 0)
		n.Spec, n.SessionID = 1467, 77
		n.Frame.Scope, n.Frame.Label = "native:77", "Earlier boss"
		n.View = &lookup.NativeMeterView{View: view, Actor: aug, Primary: false, TotalRows: 8, Offset: 3, Total: 8, Maximum: 8,
			Rows: []lookup.NativeViewRow{{Key: "spell:57994", Name: "Wind Shear", NativeIndex: 4, SpellID: 57994, Total: 8}}}
		s.ObserveNativeSlot(n, 0, time.Now(), "")
		v, err := s.PreparedSummaryForPlayer("window:1", aug, 4)
		if err != nil || !strings.Contains(v.Text, "Earlier boss - Interrupts - "+view) || !strings.Contains(v.Text, "shown rows") || !strings.Contains(v.Text, "4. Wind Shear 8") || strings.Contains(v.Text, "1. 4.") {
			t.Fatal(view, err, v)
		}
		if _, err := s.PreparedSummaryForPlayer("window:1", other, 4); err == nil {
			t.Fatal("another character read a captured view")
		}
		// A protected name image is not transferable as a plain text name.
		n.View.Rows[0].Name = ""
		s.ObserveNativeSlot(n, 0, time.Now(), "")
		if _, err := s.PreparedSummaryForPlayer("window:1", aug, 4); err == nil {
			t.Fatal("unavailable row name was invented")
		}
	}
}
