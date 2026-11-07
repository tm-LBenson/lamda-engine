package main

import (
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type screenCandidate struct {
	cast            lookup.LiveCast
	first, last, at time.Time
	rejected        bool
}
type screenInput struct {
	id      string
	pending map[string]screenCandidate
}

// A single frame cannot create a cooldown. Require two consistent readings,
// an advancing public clock and a current roster. Conflicting readings poison
// this delivery ID instead of selecting whichever digit looks plausible later.
func (in *screenInput) casts(m *engine.Model, s *lookup.ScreenCasts, now time.Time) []lookup.LiveCast {
	if s != nil {
		m.Diagnostics.ScreenCaptureError = s.Error
		m.Diagnostics.ScreenFormat = s.Version
	}
	if engine.Forever || s == nil || !s.Captured {
		*in = screenInput{}
		if s != nil {
			m.Diagnostics.ScreenCaptureFailures++
		}
		return nil
	}
	m.Diagnostics.ScreenSamples++
	m.Diagnostics.ScreenRejected += uint64(s.Rejected)
	if in.id != s.ID {
		*in = screenInput{id: s.ID, pending: map[string]screenCandidate{}}
	}
	current := map[string]bool{}
	var result []lookup.LiveCast
	for _, c := range s.Casts {
		if !engine.IsInterrupt(c.Spell) {
			continue
		}
		key := c.GUID + "/" + c.ID
		current[key] = true
		p, exists := in.pending[key]
		at := now.Add(-time.Duration(c.AgeMillis) * time.Millisecond)
		if !exists {
			in.pending[key] = screenCandidate{cast: c, first: now, last: now, at: at}
			continue
		}
		if c.Spell != p.cast.Spell || at.Sub(p.at) > 350*time.Millisecond || p.at.Sub(at) > 350*time.Millisecond {
			p.rejected = true
		}
		if now.Sub(p.last) > 750*time.Millisecond || now.Before(p.last) {
			p.first = now
			p.at = at
		}
		p.last = now
		in.pending[key] = p
		if !p.rejected && now.Sub(p.first) >= 150*time.Millisecond && c.AgeMillis > p.cast.AgeMillis {
			c.AgeMillis = int(now.Sub(p.at) / time.Millisecond)
			if c.AgeMillis >= 0 && c.AgeMillis <= 30000 {
				result = append(result, c)
			}
		}
	}
	for key := range in.pending {
		if !current[key] {
			delete(in.pending, key)
		}
	}
	return result
}
