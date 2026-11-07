package simming

import (
	"fmt"
	"strconv"
	"strings"
)

// Planning metadata is exported data, never passed through as simulator options.
// Roll history is observational: it does not claim to be Blizzard's authoritative
// remaining pool, and ownership is not duplicate protection.
type BonusRoll struct {
	Currency int `json:"currency"`
	Source   int `json:"source"`
	Context  int `json:"context"`
	KeyLevel int `json:"keyLevel"`
	ItemID   int `json:"itemID"`
	Spec     int `json:"spec"`
}
type Currency struct {
	ID       int `json:"id"`
	Quantity int `json:"quantity"`
}
type PlanningData struct {
	Rolls             []BonusRoll `json:"rolls"`
	Currencies        []Currency  `json:"currencies"`
	HistoryPresent    bool        `json:"historyPresent"`
	CurrenciesPresent bool        `json:"currenciesPresent"`
}

func metadataNumbers(value string, width, limit int) ([][]int, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > 32768 {
		return nil, fmt.Errorf("planning metadata is too large")
	}
	rows := strings.Split(value, "/")
	if len(rows) > limit {
		return nil, fmt.Errorf("too many planning records")
	}
	out := make([][]int, 0, len(rows))
	for _, row := range rows {
		parts := strings.Split(row, ":")
		if len(parts) != width {
			return nil, fmt.Errorf("invalid planning record")
		}
		ns := make([]int, width)
		for i, part := range parts {
			if part == "" || strings.Trim(part, "0123456789") != "" {
				return nil, fmt.Errorf("invalid planning value")
			}
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 || n > 100000000 {
				return nil, fmt.Errorf("invalid planning value")
			}
			ns[i] = n
		}
		out = append(out, ns)
	}
	return out, nil
}
func (p *PlanningData) parse(key, value string) (bool, error) {
	switch key {
	case "bonus_roll_items":
		if p.HistoryPresent {
			return true, fmt.Errorf("repeated bonus roll history")
		}
		rows, err := metadataNumbers(value, 6, 1024)
		if err != nil {
			return true, err
		}
		p.HistoryPresent = true
		for _, n := range rows {
			if n[4] == 0 {
				return true, fmt.Errorf("bonus roll item is missing")
			}
			p.Rolls = append(p.Rolls, BonusRoll{n[0], n[1], n[2], n[3], n[4], n[5]})
		}
		return true, nil
	case "bonus_roll_currencies":
		if p.CurrenciesPresent {
			return true, fmt.Errorf("repeated bonus roll currencies")
		}
		rows, err := metadataNumbers(value, 2, 64)
		if err != nil {
			return true, err
		}
		p.CurrenciesPresent = true
		seen := map[int]bool{}
		for _, n := range rows {
			if n[0] == 0 || seen[n[0]] {
				return true, fmt.Errorf("invalid bonus roll currency")
			}
			seen[n[0]] = true
			p.Currencies = append(p.Currencies, Currency{n[0], n[1]})
		}
		return true, nil
	}
	return false, nil
}
