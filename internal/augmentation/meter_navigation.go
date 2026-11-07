package augmentation

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

var meterSpellSelection = regexp.MustCompile(`^(raw/[0-9]{1,8}|credit/[0-9]{1,8}|support/Player-[A-Za-z0-9-]{1,150}/[0-9]{1,8})$`)

// This is only the overlay's navigation, never WoW settings or input. Bind
// every action to the exact displayed owner/session and actor GUID. Reordering
// Aug rankings therefore cannot open a different player's native raw row.
type meterNavigation struct {
	Schema  int    `json:"schema"`
	Owner   string `json:"owner"`
	Segment string `json:"segment"`
	Actor   string `json:"actor"`
	Spell   string `json:"spell"`
	Offset  int    `json:"offset"`
}

func (s *Store) meterNavigation(m DamageMeter, windowID ...int) meterNavigation {
	if s.path == "" {
		return meterNavigation{}
	}
	file := "view.json"
	if len(windowID) > 0 && windowID[0] > 0 && windowID[0] <= 8 {
		file = fmt.Sprintf("view-%d.json", windowID[0])
	}
	f, err := os.Open(filepath.Join(filepath.Dir(s.path), file))
	if err != nil {
		return meterNavigation{}
	}
	defer f.Close()
	var v meterNavigation
	d := json.NewDecoder(io.LimitReader(f, 2048))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || v.Schema != 1 || v.Owner != m.Owner || v.Segment != m.ID || v.Offset < 0 || v.Offset > maxDetailEntries || (v.Actor != "" && (!player(v.Actor) || len(v.Actor) > 160)) || (v.Spell != "" && (v.Actor == "" || !meterSpellSelection.MatchString(v.Spell))) {
		return meterNavigation{}
	}
	return v
}

func signedMeterNumber(n float64) string {
	if n < 0 {
		return "-" + meterNumber(-n)
	}
	return meterNumber(n)
}

func (s *Store) presentMeterView(m DamageMeter, appearance meterAppearance, classes map[string]int, height int, session, status string, windowID ...int) *engine.MeterBody {
	nav := s.meterNavigation(m, windowID...)
	if nav.Schema == 1 {
		appearance.Offset = nav.Offset
	}
	body := presentMeter(m, appearance, classes, height, session, status)
	if nav.Actor == "" {
		return body
	}
	body.Actor, body.Spell = nav.Actor, nav.Spell
	body.View = "spells"
	if nav.Spell != "" {
		body.View = "targets"
	}
	body.Rows = []engine.MeterBodyRow{}
	body.TotalRows = 0
	body.Offset = 0
	name := "Player"
	known := false
	for _, row := range m.Rows {
		if row.GUID == nav.Actor {
			known = true
			name = strings.Split(row.Name, "-")[0]
			break
		}
	}
	if !known || (m.Metric != 0 && m.Metric != 1) {
		body.Empty = "Breakdown unavailable"
		return body
	}
	if !m.Valid || m.Provisional {
		body.Empty = "Spell details pending"
		return body
	}
	breakdown, err := s.DamageBreakdown(m.Owner, m.ID, nav.Actor, nav.Spell)
	if err != nil {
		body.Empty = "Spell details unavailable for this session"
		return body
	}
	if !breakdown.Complete {
		body.Status = "Partial spell attribution"
	} else {
		body.Status = "Spells · " + name
		if nav.Spell != "" {
			body.Status = "Targets · " + name
		}
	}
	body.TotalRows = len(breakdown.Rows)
	visible := max(1, (height-body.HeaderHeight-body.FooterHeight)/body.RowHeight)
	body.Offset = max(0, min(nav.Offset, max(0, len(breakdown.Rows)-visible)))
	maximum := int64(1)
	for _, row := range breakdown.Rows {
		maximum = max(maximum, row.Amount)
	}
	for i := body.Offset; i < min(len(breakdown.Rows), body.Offset+visible); i++ {
		row := breakdown.Rows[i]
		value := signedMeterNumber(float64(row.Amount))
		rate := float64(row.Amount) / max(1, m.Seconds)
		switch appearance.Values {
		case "rate":
			value = signedMeterNumber(rate) + "/s"
		case "percent":
			if breakdown.Expected > 0 {
				value = fmt.Sprintf("%.1f%%", 100*float64(row.Amount)/float64(breakdown.Expected))
			} else {
				value = "—"
			}
		case "both":
			value += "  |  " + signedMeterNumber(rate) + "/s"
		}
		color := "#4786A8"
		if appearance.ClassColors && meterClassColors[classes[nav.Actor]] != "" {
			color = meterClassColors[classes[nav.Actor]]
		}
		body.Rows = append(body.Rows, engine.MeterBodyRow{ID: row.ID, Name: row.Name, Value: value, Fraction: math.Max(0, float64(row.Amount)/float64(maximum)), Color: color, SpellID: row.SpellID})
	}
	if len(body.Rows) == 0 {
		body.Empty = "No breakdown data"
	}
	return body
}
