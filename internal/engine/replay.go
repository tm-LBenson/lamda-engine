package engine

import (
	"bufio"
	"io"
	"time"
)

// ReplayAudit uses event time plus a supplied delivery delay, never wall-clock
// "now" for historical logs. Delay is simulated, not measured log latency.
func ReplayAudit(r io.Reader, delivery time.Duration) (InferenceSnapshot, error) {
	m := NewModel()
	session := &LogSession{}
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 65536), 1<<20)
	var now time.Time
	for scan.Scan() {
		line := scan.Text()
		at, _, err := RecordHeader(line)
		if err == nil && at.Add(delivery).After(now) {
			now = at.Add(delivery)
		}
		if session.ResetFor(line) {
			m.Reset()
		}
		m.Ingest(line, now, true)
	}
	return m.Snapshot(now), scan.Err()
}
