package lookup

import "time"

const activeCaptureInterval = 200 * time.Millisecond

// Short decode gaps retain the normal cadence. Only repeatedly missing strips
// back off, and the first valid sample immediately restores active tracking.
type captureCadence struct {
	idle   time.Duration
	missed int
}

func (c *captureCadence) next(valid bool) time.Duration {
	if valid {
		c.missed = 0
	} else if c.missed < 5 {
		c.missed++
	}
	if c.missed >= 5 && c.idle > activeCaptureInterval {
		return min(c.idle, time.Second)
	}
	return activeCaptureInterval
}
