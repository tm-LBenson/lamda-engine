package augmentation

import (
	"fmt"
	"math"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

// These names and number formats follow the normal LamdaMeters window. This
// layer consumes a meter result; it cannot turn raw damage into Aug credit.
var meterMetricNames = [...]string{"Damage", "DPS", "Healing", "HPS", "Absorbs", "Interrupts", "Dispels", "Damage taken", "Avoidable damage", "Deaths", "Enemy damage taken"}
var meterClassColors = map[int]string{1: "#C79C6E", 2: "#F58CBA", 3: "#ABD473", 4: "#FFF569", 5: "#FFFFFF", 6: "#C41F3B", 7: "#0070DE", 8: "#69CCF0", 9: "#9482C9", 10: "#00FF96", 11: "#FF7D0A", 12: "#A330C9", 13: "#33937F"}

type meterAppearance struct {
	HeaderHeight, FooterHeight  int
	PixelScaled                 bool
	RowHeight, FontSize, Offset int
	Opacity                     float64
	Values                      string
	ClassColors                 bool
}

func defaultMeterAppearance() meterAppearance {
	return meterAppearance{RowHeight: 21, FontSize: 11, Opacity: .88, Values: "both", ClassColors: true}
}

func meterNumber(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return "—"
	}
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", n/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", n/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.0fK", n/1e3)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

func meterValue(row DamageRow, total int64, metric int, mode string, valid bool) string {
	if !valid {
		return "—"
	}
	if metric == 5 || metric == 6 || metric == 9 {
		return meterNumber(float64(row.Adjusted))
	}
	switch mode {
	case "percent":
		if total <= 0 {
			return "—"
		}
		return fmt.Sprintf("%.1f%%", 100*float64(row.Adjusted)/float64(total))
	case "total":
		return meterNumber(float64(row.Adjusted))
	case "rate":
		return meterNumber(row.DPS) + "/s"
	}
	if metric == 1 || metric == 3 {
		return meterNumber(row.DPS) + "/s"
	}
	return meterNumber(float64(row.Adjusted)) + "  |  " + meterNumber(row.DPS) + "/s"
}

func presentMeter(m DamageMeter, appearance meterAppearance, classes map[string]int, height int, session, status string) *engine.MeterBody {
	metric := "Damage"
	if m.Metric >= 0 && m.Metric < len(meterMetricNames) {
		metric = meterMetricNames[m.Metric]
	}
	if appearance.PixelScaled {
		appearance.RowHeight = max(8, min(72, appearance.RowHeight))
		appearance.FontSize = max(6, min(40, appearance.FontSize))
	} else {
		appearance.RowHeight = max(16, min(36, appearance.RowHeight))
		appearance.FontSize = max(9, min(20, appearance.FontSize))
	}
	if appearance.HeaderHeight <= 0 {
		appearance.HeaderHeight = 53
	}
	if appearance.FooterHeight <= 0 {
		appearance.FooterHeight = 22
	}
	if math.IsNaN(appearance.Opacity) || math.IsInf(appearance.Opacity, 0) {
		appearance.Opacity = .88
	}
	body := &engine.MeterBody{View: "actors", Metric: metric, Session: session, Status: status,
		RowHeight: appearance.RowHeight, FontSize: appearance.FontSize, Opacity: max(.2, min(1, appearance.Opacity)), TotalRows: len(m.Rows),
		Provisional: m.Provisional, Rows: []engine.MeterBodyRow{}}
	body.Owner, body.Segment = m.Owner, m.ID
	body.HeaderHeight, body.FooterHeight = appearance.HeaderHeight, appearance.FooterHeight
	body.Inspectable = (m.Metric == 0 || m.Metric == 1) && m.Owner != "" && m.ID != ""
	visible := max(1, (height-body.HeaderHeight-body.FooterHeight)/appearance.RowHeight)
	body.Offset = max(0, min(appearance.Offset, max(0, len(m.Rows)-visible)))
	maximum := int64(1)
	for _, row := range m.Rows {
		maximum = max(maximum, row.Adjusted)
	}
	for i := body.Offset; i < min(len(m.Rows), body.Offset+visible); i++ {
		row := m.Rows[i]
		color := "#4786A8"
		if appearance.ClassColors && meterClassColors[classes[row.GUID]] != "" {
			color = meterClassColors[classes[row.GUID]]
		}
		name := strings.Split(row.Name, "-")[0]
		if name == "" {
			name = "Player"
		}
		fraction := float64(max(int64(0), row.Adjusted)) / float64(maximum)
		if !m.Valid {
			fraction = 0
		}
		body.Rows = append(body.Rows, engine.MeterBodyRow{ID: row.GUID, Name: fmt.Sprintf("%d. %s", i+1, name), Value: meterValue(row, m.AdjustedTotal, m.Metric, appearance.Values, m.Valid), Fraction: fraction, Color: color})
	}
	if len(body.Rows) == 0 {
		body.Empty = "No combat data"
	}
	return body
}
