package market

import "time"

type CaptureStatus struct {
	Enabled    *bool  `json:"enabled"`
	LastPacket int64  `json:"lastPacket"`
	LastEvent  int64  `json:"lastEvent"`
	Events     int64  `json:"events"`
	Error      string `json:"error"`
}

func (s *Store) SetCaptureEnabled(enabled bool) {
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	s.capture.Enabled = &enabled
}
func (s *Store) Capture() CaptureStatus {
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	return s.capture
}
func (s *Store) Accept(body []byte, now time.Time) error {
	data, err := s.transfer.Accept(body, now)
	if err == nil && data != nil {
		err = s.Ingest(data, now)
		if err != nil {
			s.transfer.Retry(body)
		}
	}
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	s.capture.LastPacket = now.Unix()
	if err != nil {
		s.capture.Error = err.Error()
	} else {
		s.capture.Error = ""
		if data != nil {
			s.capture.LastEvent = now.Unix()
			s.capture.Events++
		}
	}
	return err
}
