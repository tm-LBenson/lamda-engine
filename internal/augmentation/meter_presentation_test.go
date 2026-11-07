package augmentation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeterPresentationUsesRecordedKeyResultsAndSeparateReadableColumns(t *testing.T) {
	m := DamageMeter{Valid: true, Metric: 0, AdjustedTotal: 333753630, Rows: []DamageRow{
		{GUID: "mage", Name: "Mage-Realm", Raw: 118763997, Adjusted: 98182206, DPS: 112982.9758},
		{GUID: "shaman", Name: "Shaman-Realm", Raw: 108522155, Adjusted: 89729634, DPS: 103256.1956},
		{GUID: "aug", Name: "Aug-Realm", Raw: 32694839, Added: 47606286, Adjusted: 80301125, DPS: 92406.3579},
		{GUID: "tank", Name: "Tank-Realm", Raw: 52517156, Adjusted: 44411383, DPS: 51106.3095},
		{GUID: "healer", Name: "Healer-Realm", Raw: 21255483, Adjusted: 21129282, DPS: 24314.4787},
	}}
	body := presentMeter(m, defaultMeterAppearance(), map[string]int{"mage": 8, "shaman": 7, "aug": 13, "tank": 11, "healer": 10}, 236, "Overall", "Recorded")
	if len(body.Rows) != 5 || body.Rows[2].Name != "3. Aug" || body.Rows[2].Value != "80.3M  |  92K/s" || body.Rows[2].Color != "#33937F" || body.RowHeight != 21 || body.FontSize != 11 {
		t.Fatal(body)
	}
	if body.Rows[2].Fraction <= .81 || body.Rows[2].Fraction >= .83 || m.Rows[2].Raw != 32694839 {
		t.Fatal("presentation changed the accounting", body.Rows[2])
	}
	if dir := os.Getenv("LAMDA_METER_PREVIEW_DIR"); dir != "" {
		data, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meter-body.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMeterPresentationHonorsValueModesCountMetricsAndUnknownInputs(t *testing.T) {
	r := DamageRow{Adjusted: 2500000, DPS: 12500}
	for _, test := range []struct {
		metric     int
		mode, want string
	}{{0, "both", "2.5M  |  12K/s"}, {0, "total", "2.5M"}, {0, "rate", "12K/s"}, {0, "percent", "25.0%"}, {1, "both", "12K/s"}, {3, "both", "12K/s"}, {5, "rate", "2.5M"}, {6, "percent", "2.5M"}, {9, "both", "2.5M"}} {
		if got := meterValue(r, 10000000, test.metric, test.mode, true); got != test.want {
			t.Fatal(test, got)
		}
	}
	if meterValue(r, 0, 0, "percent", true) != "—" || meterValue(r, 10, 0, "total", false) != "—" || meterNumber(math.NaN()) != "—" {
		t.Fatal("missing data became a numeric zero")
	}
}

func TestMeterPresentationBoundsScrollAndStylesWithoutTruncatingNumbers(t *testing.T) {
	m := DamageMeter{Valid: true, AdjustedTotal: 100}
	for i := 0; i < 40; i++ {
		m.Rows = append(m.Rows, DamageRow{GUID: "player", Name: strings.Repeat("Name", 20) + "-Realm", Adjusted: 100, DPS: 10})
	}
	a := defaultMeterAppearance()
	a.RowHeight, a.FontSize, a.Opacity, a.Offset = 0, 99, math.NaN(), 999
	body := presentMeter(m, a, nil, 120, "Current", "Live")
	if body.RowHeight != 16 || body.FontSize != 20 || body.Opacity != .88 || body.Offset != 38 || len(body.Rows) != 2 || body.Rows[1].Value != "100  |  10/s" {
		t.Fatal(body)
	}
	m.Valid = false
	body = presentMeter(m, defaultMeterAppearance(), nil, 236, "Current", "Waiting for live feed")
	for _, row := range body.Rows {
		if row.Value != "—" || row.Fraction != 0 {
			t.Fatal(row)
		}
	}
}
