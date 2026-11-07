package overlays

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"time"
)

// Pausing in-game work never edits authoring data or saved per-character memory.
func (s *Store) SetCaptureEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.captureDisabled == !enabled {
		return
	}
	s.captureDisabled = !enabled
	s.runtime.Reset()
	s.runtime.last = nil
	s.runtime.seen = time.Time{}
	s.runtime.problem = ""
	s.combat = combatFeed{}
	if !enabled {
		s.runtime.problem = "Disabled"
	}
}

func (s *Store) NeedsCombatLog(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.captureDisabled {
		return false
	}
	r := s.runtime
	if r.last == nil || !r.initialized || now.Before(r.seen) || now.Sub(r.seen) > engine.FrameLifetime || r.last.CombatVersion != 1 {
		return false
	}
	s.rulesForExecution()
	return s.combatRequested
}
