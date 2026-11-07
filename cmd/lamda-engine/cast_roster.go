package main

import (
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Public roster/layout timestamps use WoW's integer server clock. Allow the
// same measured clock difference as the display; cast times remain anchored
// to local receipt minus monotonic age, with native two-read confirmation.
const addonClockTolerance = engine.FrameTimestampSkew

// Public build lookups are optional. Cast membership instead comes from the
// addon's explicit player/party1..4 roster, independently of HTTP/cache results.
// A retained pixel packet expires by its original timestamp, never read time.
func rosterForCasts(event *lookup.Event, now time.Time) map[string]bool {
	if event == nil || event.ID == "" || len(event.ID) > 128 || len(event.Members) > 5 ||
		event.At.After(now.Add(addonClockTolerance)) || now.Sub(event.At) > 30*time.Second {
		return nil
	}
	if event.Kind != "roster" && event.Kind != "key_start" && event.Kind != "arena_ready" && event.Kind != "challenge_start" && event.Kind != "challenge_end" {
		return nil
	}
	if (event.Kind == "key_start" || event.Kind == "arena_ready") && (event.Deadline.IsZero() || !event.Deadline.After(event.At) || event.Deadline.Sub(event.At) > 15*time.Second) {
		return nil
	}
	if event.Kind == "challenge_start" && (event.Deadline.IsZero() || event.Deadline.After(event.At) || event.At.Sub(event.Deadline) > 30*time.Second) {
		return nil
	}
	roster := make(map[string]bool, len(event.Members))
	for _, member := range event.Members {
		if !member.Valid() || roster[member.GUID] {
			return nil
		}
		roster[member.GUID] = true
	}
	return roster
}
