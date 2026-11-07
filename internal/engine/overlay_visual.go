package engine

// Optional visual settings. Zero dimensions retain the original automatic size.
// Transforms act around the centre without changing the layout/attachment box.
type OverlayVisual struct {
	Motion       *OverlayMotion `json:"motion,omitempty"`
	Sprite       *OverlaySprite `json:"sprite,omitempty"`
	FontFamily   string         `json:"fontFamily,omitempty"`
	Bold         *bool          `json:"bold,omitempty"`
	Italic       bool           `json:"italic,omitempty"`
	TextEffect   string         `json:"textEffect,omitempty"`
	Image        string         `json:"image,omitempty"`
	Fit          string         `json:"fit,omitempty"`
	Tint         bool           `json:"tint,omitempty"`
	Width        float64        `json:"width,omitempty"`
	Height       float64        `json:"height,omitempty"`
	Rotation     float64        `json:"rotation,omitempty"`
	FlipX        bool           `json:"flipX,omitempty"`
	FlipY        bool           `json:"flipY,omitempty"`
	Layer        int            `json:"layer,omitempty"`
	Shape        string         `json:"shape,omitempty"`
	Direction    string         `json:"direction,omitempty"`
	TrackColor   string         `json:"trackColor,omitempty"`
	TrackOpacity *float64       `json:"trackOpacity,omitempty"`
	FillOpacity  *float64       `json:"fillOpacity,omitempty"`
	TextAlign    string         `json:"textAlign,omitempty"`
	BarIcon      string         `json:"barIcon,omitempty"`
}

// Frames run left-to-right, then top-to-bottom in an uploaded image.
type OverlaySprite struct {
	Columns int     `json:"columns"`
	Rows    int     `json:"rows"`
	Frames  int     `json:"frames"`
	Start   int     `json:"start"`
	FPS     float64 `json:"fps"`
	Mode    string  `json:"mode"`
	Reverse bool    `json:"reverse,omitempty"`
}
