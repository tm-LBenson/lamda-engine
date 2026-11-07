package engine

// Start moves from the configured endpoint to the normal appearance. Main
// repeats toward that endpoint after Start completes. The layout box is fixed.
type OverlayMotion struct {
	Start  *OverlayMotionEffect `json:"start,omitempty"`
	Main   *OverlayMotionEffect `json:"main,omitempty"`
	Finish *OverlayMotionEffect `json:"finish,omitempty"`
}

type OverlayMotionEffect struct {
	Duration float64  `json:"duration"`
	Easing   string   `json:"easing,omitempty"`
	Cycle    string   `json:"cycle,omitempty"`
	Alpha    *float64 `json:"alpha,omitempty"`
	X        float64  `json:"x,omitempty"`
	Y        float64  `json:"y,omitempty"`
	ScaleX   *float64 `json:"scaleX,omitempty"`
	ScaleY   *float64 `json:"scaleY,omitempty"`
	Rotation float64  `json:"rotation,omitempty"`
	Color    string   `json:"color,omitempty"`
}
