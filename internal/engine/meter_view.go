package engine

// MeterBody is presentation data shared by attached and detached meters. The
// native controller owns the selected metric/session/profile; rendering never
// changes attribution, supplies missing counters, or sends chat messages.
type MeterBody struct {
	Navigation   *MeterNavigation `json:"navigation,omitempty"`
	WindowID     int              `json:"windowID,omitempty"`
	HeaderHeight int              `json:"headerHeight,omitempty"`
	FooterHeight int              `json:"footerHeight,omitempty"`
	Owner        string           `json:"owner,omitempty"`
	Segment      string           `json:"segment,omitempty"`
	Actor        string           `json:"actor,omitempty"`
	Spell        string           `json:"spell,omitempty"`
	Inspectable  bool             `json:"inspectable,omitempty"`
	View         string           `json:"view"`
	Metric       string           `json:"metric"`
	Session      string           `json:"session"`
	Status       string           `json:"status"`
	RowHeight    int              `json:"rowHeight"`
	FontSize     int              `json:"fontSize"`
	Opacity      float64          `json:"opacity"`
	Offset       int              `json:"offset"`
	TotalRows    int              `json:"totalRows"`
	Rows         []MeterBodyRow   `json:"rows"`
	Empty        string           `json:"empty,omitempty"`
	Provisional  bool             `json:"provisional,omitempty"`
}
type MeterNavigation struct {
	WindowID int    `json:"windowID"`
	Serial   int    `json:"serial"`
	Epoch    string `json:"epoch"`
	Native   bool   `json:"native"`
}

type MeterBodyRow struct {
	IconPath  string  `json:"iconPath,omitempty"`
	NameImage string  `json:"nameImage,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Value     string  `json:"value"`
	Fraction  float64 `json:"fraction"`
	Color     string  `json:"color"`
	SpellID   int     `json:"spellID,omitempty"`
	Tooltip   string  `json:"tooltip,omitempty"`
}
