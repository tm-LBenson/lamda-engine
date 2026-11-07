package lookup

import (
	"testing"
	"time"
)

func TestIdleCaptureBackoffNeverSlowsValidTracking(t *testing.T) {
	for _, idle := range []time.Duration{0, time.Second, 10 * time.Second} {
		c := captureCadence{idle: idle}
		for i := 0; i < 100; i++ {
			if c.next(true) != 200*time.Millisecond {
				t.Fatal("active tracking slowed")
			}
		}
		for i := 0; i < 4; i++ {
			if c.next(false) != 200*time.Millisecond {
				t.Fatal("transient miss slowed tracking")
			}
		}
		want := activeCaptureInterval
		if idle > 0 {
			want = time.Second
		}
		for i := 0; i < 100; i++ {
			if c.next(false) != want {
				t.Fatal("idle did not back off", idle)
			}
		}
		if c.next(true) != activeCaptureInterval || c.next(false) != activeCaptureInterval {
			t.Fatal("valid reading did not restore cadence")
		}
	}
}
