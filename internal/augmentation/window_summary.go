package augmentation

import (
	"fmt"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

// Reuse the clicked window's presentation and attribution. No other window,
// encounter, metric or prepared boss/key report is a fallback for this action.
func (s *Store) selectedWindowSummary(w windowMeter, owner string) (ChatSummary, error) {
	m, f := w.meter, w.frame
	a := defaultMeterAppearance()
	a.Values = f.Values
	if a.Values == "" {
		a.Values = "both"
	}
	var body *engine.MeterBody
	nativeView := false
	if n := w.native; n != nil && n.View != nil && !n.View.Primary {
		nativeView = true
		if n.Player != owner || !n.Captured || n.Combat || n.Error != "" {
			return ChatSummary{}, fmt.Errorf("This window's capture is not ready")
		}
		// Protected name images can be drawn but cannot become report text.
		for _, r := range n.View.Rows {
			if strings.TrimSpace(r.Name) == "" {
				return ChatSummary{}, fmt.Errorf("This window's row names are unavailable for sharing")
			}
		}
		body = s.presentNativeView(n, m, a)
	} else {
		if m.Owner != owner || !m.Valid || m.Provisional || m.Live {
			return ChatSummary{}, fmt.Errorf("This window's counters or Aug attribution are still pending")
		}
		if m.Metric < 0 || m.Metric >= len(meterMetricNames) {
			return ChatSummary{}, fmt.Errorf("This window's metric is unavailable")
		}
		if (m.Metric == 0 || m.Metric == 1) && m.Seconds <= 0 {
			return ChatSummary{}, fmt.Errorf("This window's duration is unavailable")
		}
		if m.Metric == 0 || m.Metric == 1 {
			nav := s.meterNavigation(m, f.WindowID)
			if nav.Actor == "" {
				// Keep corrected player DPS and its existing completeness rules.
				m.Name = f.Label
				m.Metric = 0
				return summaryWithOptions(makeChatSummary(m, summaryCounts{}), 4), nil
			}
			body = s.presentMeterView(m, a, w.classes, 53+22+5*a.RowHeight, f.Label, "", f.WindowID)
		} else {
			body = presentMeter(m, a, w.classes, 53+22+5*a.RowHeight, f.Label, "")
		}
	}
	if body == nil || len(body.Rows) == 0 || body.Provisional {
		return ChatSummary{}, fmt.Errorf("This window's selected view is unavailable")
	}
	title := summaryText(f.Label, 60) + " - " + summaryText(body.Metric, 40)
	if body.View != "actors" {
		title += " - " + summaryText(body.View, 20)
		if body.Actor != "" {
			name := ""
			for _, r := range m.Rows {
				if r.GUID == body.Actor {
					name = r.Name
				}
			}
			if name == "" {
				s.mu.Lock()
				name = s.party[body.Actor]
				if name == "" {
					name = s.names[body.Actor]
				}
				s.mu.Unlock()
			}
			if name != "" {
				title += " (" + summaryText(strings.Split(name, "-")[0], 36) + ")"
			}
		}
	}
	if body.Offset > 0 || len(body.Rows) < min(5, body.TotalRows) {
		title += " · shown rows"
	} else if body.TotalRows > 5 {
		title += " · top 5"
	}
	lines := []string{"[LamdaMeters] " + title}
	for i, row := range body.Rows {
		if i >= 5 && row.ID != "support" {
			continue
		}
		if row.Value == "" || strings.Contains(row.Value, "—") {
			return ChatSummary{}, fmt.Errorf("This window's selected values are unavailable")
		}
		name := summaryText(row.Name, 100)
		if body.View != "actors" && !nativeView && row.ID != "support" && body.Offset == 0 {
			name = fmt.Sprintf("%d. %s", i+1, name)
		}
		value := strings.ReplaceAll(row.Value, "|", "/")
		lines = append(lines, "[LamdaMeters] "+name+" "+summaryText(value, 80))
	}
	return ChatSummary{Segment: m.ID, Name: title, EndedAt: m.EndedAt, Lines: lines, Text: strings.Join(lines, "~")}, nil
}
