package main

import (
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type bridgeDisplayStatus uint8

const (
	displaySampleAccepted bridgeDisplayStatus = iota
	displaySampleTransientFailure
	displaySampleGlobalFailure
	displaySampleWrongClient
)

// bridgeDisplayCache keeps the most recent independently decoded CD and Kick
// layouts. FrameView and KickView still enforce engine.FrameLifetime, so a
// retained layout cannot outlive its normal safety lease. A single checksum,
// visibility, or repaint miss must not turn a 200 ms reader gap into a visible
// empty frame.
type bridgeDisplayCache struct {
	geometry       *framewire.Snapshot
	geometrySeen   time.Time
	geometryID     string
	geometryWindow uintptr
	kick           *framewire.KickSnapshot
	kickSeen       time.Time
	kickID         string
	kickWindow     uintptr
}

func (d *bridgeDisplayCache) clear() {
	*d = bridgeDisplayCache{}
}

func (d *bridgeDisplayCache) observe(sample lookup.BridgeSample, forever bool, now time.Time) bridgeDisplayStatus {
	if sample.Error != "" {
		if sample.Source != "" && !strings.HasPrefix(sample.Error, "wow_") {
			return displaySampleTransientFailure
		}
		d.clear()
		return displaySampleGlobalFailure
	}
	if sample.Forever != forever {
		d.clear()
		return displaySampleWrongClient
	}
	// An absent source section can be a one-frame addon/API miss. Deliberate
	// context changes publish a real replacement snapshot (for example Kick
	// mode zero or a frame snapshot with no rows), which is accepted below.
	if (sample.Source == "" || sample.Source == "kick") && sample.Kick != nil &&
		(d.kick == nil || sample.Kick.ID != d.kickID || sample.Window != d.kickWindow) {
		d.kick, d.kickSeen, d.kickID, d.kickWindow = sample.Kick, now, sample.Kick.ID, sample.Window
	}
	if (sample.Source == "" || sample.Source == "cd") && sample.Frames != nil &&
		(d.geometry == nil || sample.Frames.ID != d.geometryID || sample.Window != d.geometryWindow) {
		d.geometry, d.geometrySeen, d.geometryID, d.geometryWindow = sample.Frames, now, sample.Frames.ID, sample.Window
	}
	return displaySampleAccepted
}
