package overlays

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

// The tail owns ordering. This reader never replays history or deduplicates
// equal hits within a millisecond; both can be legitimate combat events.
type combatFeed struct {
	format  combatFormat
	logger  string
	started time.Time
	lagging bool
}

func (f *combatFeed) observe(line string) {
	_, fields, err := engine.RecordFields(line)
	if err != nil {
		return
	}
	f.observeFields(fields)
}
func (f *combatFeed) observeFields(fields []string) {
	if f.format.Observe(fields) {
		f.logger = ""
		f.lagging = false
		return
	}
	if len(fields) < 9 {
		return
	}
	logger := ""
	for _, i := range []int{1, 5} {
		flags, err := strconv.ParseUint(fields[i+2], 0, 32)
		if err != nil || flags&0xf != 1 || flags&0x400 == 0 || !strings.HasPrefix(fields[i], "Player-") || !knownCombatGUID(fields[i]) {
			continue
		}
		if logger != "" && logger != fields[i] {
			f.logger = ""
			return
		}
		logger = fields[i]
	}
	if logger != "" {
		f.logger = logger
	}
}
func (f *combatFeed) seed(path string, offset int64, now time.Time) error {
	*f = combatFeed{started: now}
	if offset <= 0 {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	read := func(start, end int64) error {
		b := make([]byte, end-start)
		n, e := file.ReadAt(b, start)
		if e != nil && e != io.EOF {
			return e
		}
		lines := strings.Split(string(b[:n]), "\n")
		if start > 0 && len(lines) > 0 {
			lines = lines[1:]
		}
		if len(lines) > 0 {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			if len(line) <= 65536 {
				f.observe(line)
			}
		}
		return nil
	}
	// The first header supplies format when the recent tail has no header. A
	// later header replaces it, including an unknown/new format.
	head := min(offset, int64(128<<10))
	if err = read(0, head); err != nil {
		return err
	}
	if offset > head {
		return read(max(head, offset-(1<<20)), offset)
	}
	return nil
}
func (s *Store) SeedCombatLog(path string, offset int64, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.captureDisabled {
		return nil
	}
	err := s.combat.seed(path, offset, now)
	s.updateCombatStatus()
	return err
}
func (s *Store) IngestCombatLog(line string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.captureDisabled {
		return
	}
	f, r := &s.combat, s.runtime
	at, fields, err := engine.RecordFields(line)
	if err != nil {
		return
	}
	previousLogger := f.logger
	f.observeFields(fields)
	if previousLogger != f.logger {
		s.updateCombatStatus()
	}
	p := r.last
	if p == nil || !r.initialized || now.Before(r.seen) || now.Sub(r.seen) > engine.FrameLifetime || p.CombatVersion != 1 || p.Logging == nil || !*p.Logging {
		return
	}
	player := identityFor(p, "player")
	if player == "" || f.logger != player {
		return
	}
	e, ok := parseCombatFields(at, fields, f.format)
	if !ok {
		return
	}
	lagging := now.Sub(e.At) > 3*time.Second
	if f.lagging != lagging {
		f.lagging = lagging
		s.updateCombatStatus()
	}
	if e.At.Before(f.started) || e.At.Before(r.connected) || lagging || e.At.After(now.Add(250*time.Millisecond)) {
		return
	}
	r.evaluateCombat(s.rulesForExecution(), p, &e, now)
	r.advance(now)
}

func (s *Store) updateCombatStatus() {
	r, p := s.runtime, s.runtime.last
	r.combatProblem = ""
	if p == nil {
		return
	}
	rules := []Rule{}
	for _, rule := range s.rulesForExecution() {
		if rule.Enabled && loaded(rule.Load, p) {
			rules = append(rules, rule)
		}
	}
	if len(CombatWatches(rules)) == 0 {
		return
	}
	if p.CombatVersion != 1 {
		return
	}
	if p.Logging == nil {
		r.combatProblem = "Combat logging unavailable"
		return
	}
	if !*p.Logging {
		r.combatProblem = "Combat logging is off"
		return
	}
	if id := identityFor(p, "player"); id == "" {
		r.combatProblem = "Player identity unavailable"
	} else if s.combat.logger == "" {
		r.combatProblem = "Waiting for combat log"
	} else if s.combat.logger != id {
		r.combatProblem = "Combat log belongs to another character"
	} else if s.combat.lagging {
		r.combatProblem = "Combat log delayed"
	}
}
