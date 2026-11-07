package simming

import (
	"strings"
	"testing"
)

func TestPlanningMetadataIsDataNotWorkerInput(t *testing.T) {
	p, err := ParseProfile(profileFixture + "# bonus_roll_currencies=3418:0/3511:2\n# bonus_roll_items=3511:279618:16:10:123:1468/3511:279618:16:10:124:1467\n")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Planning.CurrenciesPresent || len(p.Planning.Currencies) != 2 || p.Planning.Currencies[0].Quantity != 0 || len(p.Planning.Rolls) != 2 || p.Planning.Rolls[1].Spec != 1467 {
		t.Fatalf("metadata lost: %+v", p.Planning)
	}
	input, err := p.Input(DefaultScenario(), nil)
	if err != nil || strings.Contains(input, "bonus_roll") {
		t.Fatalf("metadata reached worker: %v", err)
	}
}
func TestPlanningMetadataRejectsCorruptionAndDirectiveInjection(t *testing.T) {
	for _, suffix := range []string{
		"# bonus_roll_items=1:2:3:4:5\n", "# bonus_roll_items=1:2:3:4:5:6:7\n",
		"# bonus_roll_items=1:2:3:4:0:6\n", "# bonus_roll_items=1:2:3:4:5:include=x\n",
		"# bonus_roll_currencies=1:-2\n", "# bonus_roll_currencies=1:2/1:3\n",
		"# bonus_roll_currencies=1:2\nbonus_roll_currencies=1:3\n",
	} {
		if _, err := ParseProfile(profileFixture + suffix); err == nil {
			t.Fatalf("accepted %q", suffix)
		}
	}
}
func TestPlanningEmptyAndUnknownHistoryRemainDistinct(t *testing.T) {
	p, _ := ParseProfile(profileFixture)
	q, err := ParseProfile(profileFixture + "# bonus_roll_currencies=\n# bonus_roll_items=\n")
	if err != nil || p.Planning.HistoryPresent || !q.Planning.HistoryPresent || !q.Planning.CurrenciesPresent {
		t.Fatal("unknown and empty conflated", err)
	}
}
