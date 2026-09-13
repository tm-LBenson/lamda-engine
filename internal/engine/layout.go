package engine

import "math"

type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type Layout struct {
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	RowWidth   int     `json:"rowWidth"`
	RowHeight  int     `json:"rowHeight"`
	FontSize   int     `json:"fontSize"`
	HeaderStep int     `json:"headerStep"`
	AnchorX    float64 `json:"anchorX"`
	AnchorY    float64 `json:"anchorY"`
	Cells      []Cell  `json:"cells"`
}

func LayoutFor(c Config, count, headers int) Layout {
	scale := func(v int) int { return int(math.Round(float64(v) * c.Scale)) }
	cols := c.Columns
	if cols > count {
		cols = count
	}
	if cols < 1 {
		cols = 1
	}
	gap := scale(c.Gap)
	h := scale(c.Height)
	w := scale(c.Width)
	step := scale(24)
	l := Layout{RowWidth: w, RowHeight: h, FontSize: scale(c.FontSize), HeaderStep: step, AnchorX: float64((c.Anchor-1)%3) / 2, AnchorY: float64((c.Anchor-1)/3) / 2, Cells: []Cell{}}
	if l.FontSize > h-4 {
		l.FontSize = h - 4
	}
	rows := (count + cols - 1) / cols
	if count > 0 {
		l.Width = cols*w + (cols-1)*gap
		l.Height = rows*h + (rows-1)*gap
	}
	l.Height += headers * step
	if headers > 0 && l.Width == 0 {
		l.Width = w
	}
	for i := 0; i < count; i++ {
		r := i / cols
		if c.Grow == 2 {
			r = rows - 1 - r
		}
		l.Cells = append(l.Cells, Cell{(i % cols) * (w + gap), headers*step + r*(h+gap)})
	}
	return l
}
