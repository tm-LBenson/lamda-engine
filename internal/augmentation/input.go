package augmentation

import "time"

// LogProgress distinguishes queued bytes in the reader from events already
// delayed in the file. These are measured byte positions, not an inferred lag.
type LogProgress struct {
	PolledAt     time.Time `json:"polledAt"`
	FileSize     int64     `json:"fileSize"`
	ReadOffset   int64     `json:"readOffset"`
	BacklogBytes int64     `json:"backlogBytes"`
	Lines        int       `json:"lines"`
}

func (s *Store) ObserveLogRead(size, offset int64, lines int, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logInput = &LogProgress{PolledAt: at, FileSize: size, ReadOffset: offset, BacklogBytes: max(int64(0), size-offset), Lines: lines}
	if s.debug != nil && s.debug.Enabled {
		if previous := s.debug.lastOutput["log_poll"]; at.Sub(previous) >= time.Second {
			s.debug.lastOutput["log_poll"] = at
			s.debug.write(map[string]any{"kind": "log_poll", "input": s.logInput})
		}
	}
}

func (s *Store) NoteShareTiming(token, segment string, started, ready, finished time.Time, endedAt int64, characters int, options uint8, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debug == nil {
		return
	}
	event := map[string]any{"kind": "share_timing", "token": token, "segment": segment, "requestedAt": started, "readyAt": ready, "finishedAt": finished, "waitSeconds": ready.Sub(started).Seconds(), "transferSeconds": finished.Sub(ready).Seconds(), "totalSeconds": finished.Sub(started).Seconds(), "characters": characters, "options": options, "acknowledged": err == nil}
	if endedAt > 0 {
		event["nativeEndedAt"] = time.Unix(endedAt, 0)
		event["secondsAfterBoundary"] = finished.Sub(time.Unix(endedAt, 0)).Seconds()
	}
	if err != nil {
		event["error"] = err.Error()
	}
	s.debug.write(event)
}
