package lookup

// Verified placement context only; this does not capture pixels or send input.
type WindowContext struct {
	Window        uintptr
	PID           uint32
	Width, Height int
	Foreground    bool
}
