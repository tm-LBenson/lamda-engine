package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"strconv"
)

func logicData(props map[string]any, field string, p *lookup.OverlayInput, actor string, c *triggerContext) logicValue {
	if p == nil {
		return logicValue{}
	}
	source := boundSource(explicitSource(props), p, actor)
	if source.Kind == "value" {
		return logicValue{}
	}
	data := resolvedDisplayValues(source, p, c, actor)
	if n, ok := data.Raw[field]; ok {
		return logicNumber(n)
	}
	text := data.Text[field]
	if one(field, "name", "character", "realm", "unit", "unitName", "role", "sourceName", "targetName", "extraName", "event", "miss", "encounterName", "outcome", "provider") {
		if text != "" && text != "—" {
			return logicLiteral(text)
		}
		return logicValue{}
	}
	// Integer identifiers/ranks are already exact strings. A numeric timer is
	// available only through Raw, never by parsing a rounded display label.
	if one(field, "spellID", "extraSpellID", "encounterID", "difficulty", "groupSize", "timerID", "rank") {
		if n, err := strconv.ParseFloat(text, 64); err == nil {
			return logicNumber(n)
		}
	}
	return logicValue{}
}

func calculatedDisplayValues(source displaySource, p *lookup.OverlayInput, actor string, c *triggerContext) displayValues {
	out := displayValues{Text: map[string]string{}, Raw: map[string]float64{}}
	e := newValueEvaluation(logicFrom(c), p, actor, c)
	v := e.named(source.Value)
	if definition := e.logic.named(source.Value); definition != nil {
		out.Text["name"] = definition.Name
	}
	if v.kind != "" {
		out.Text["current"] = v.display()
	}
	maxValue := logicNumber(source.Maximum)
	if source.MaxValue != "" {
		maxValue = e.named(source.MaxValue)
	}
	if maxValue.kind == "number" {
		out.Raw["max"] = maxValue.number
		out.Text["max"] = maxValue.display()
	}
	if v.kind == "number" {
		out.Raw["current"] = v.number
		if maxValue.kind == "number" && maxValue.number > 0 {
			fraction := max(0, min(1, v.number/maxValue.number))
			out.Fraction = &fraction
			out.Raw["percent"] = fraction * 100
			out.Text["percent"] = logicNumber(fraction * 100).display()
		}
	}
	return out
}
