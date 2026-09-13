package engine

import (
	"io"
	"os"
	"strconv"
	"strings"
)

// LogSession distinguishes instance boundaries from ordinary room/subzone
// changes. It deliberately does not infer live game context from an old log.
type LogSession struct {
	zoneID, difficulty int
	known              bool
}

// Seed reads only recent session metadata preceding a live tail's starting EOF.
// It never creates cast observations. If no marker exists in the bounded scan,
// context stays unknown and the next valid zone event conservatively resets.
func (s *LogSession) Seed(path string, offset int64) error {
	*s = LogSession{}
	if offset <= 0 {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	start := max(int64(0), offset-(8<<20))
	b := make([]byte, offset-start)
	n, err := f.ReadAt(b, start)
	if err != nil && err != io.EOF {
		return err
	}
	lines := strings.Split(string(b[:n]), "\n")
	if start > 0 {
		lines = lines[1:] // The bounded read may start halfway through a record.
	}
	if len(lines) > 0 {
		lines = lines[:len(lines)-1] // Only complete records preceding EOF.
	}
	for i := len(lines) - 1; i >= 0; i-- {
		match := stamp.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		event, _, _ := strings.Cut(match[3], ",")
		if event == "ZONE_CHANGE" || event == "COMBAT_LOG_VERSION" {
			if s.ResetFor(lines[i]) {
				return nil
			}
		}
	}
	return nil
}

func (s *LogSession) ResetFor(line string) bool {
	match := stamp.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	event, _, _ := strings.Cut(match[3], ",")
	switch event {
	case "COMBAT_LOG_VERSION", "CHALLENGE_MODE_START", "CHALLENGE_MODE_END", "ZONE_CHANGE":
	default:
		return false
	}
	_, fields, err := parseRecord(line)
	if err != nil {
		return false
	}
	switch event {
	case "COMBAT_LOG_VERSION":
		if len(fields) < 2 {
			return false
		}
		*s = LogSession{}
		return true
	case "CHALLENGE_MODE_START", "CHALLENGE_MODE_END":
		return len(fields) > 1
	case "ZONE_CHANGE":
		if len(fields) < 4 {
			return false
		}
		zone, zoneErr := strconv.Atoi(fields[1])
		difficulty, diffErr := strconv.Atoi(fields[3])
		if zoneErr != nil || diffErr != nil || zone < 0 || difficulty < 0 {
			return false
		}
		changed := !s.known || s.zoneID != zone || s.difficulty != difficulty
		s.zoneID, s.difficulty, s.known = zone, difficulty, true
		return changed
	}
	return false
}
