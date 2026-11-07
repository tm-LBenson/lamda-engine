package kbb

import (
	"fmt"
	"hash/crc32"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const TransferTTL = 3 * time.Minute

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)

type transfer struct {
	header      string
	at          time.Time
	size, count int
	checksum    uint32
	chunks      map[int]string
}

// One active user request per client. Recently completed IDs survive packet
// repetition; the workspace also persists IDs to cover an engine restart.
type Assembler struct {
	Client string
	active *transfer
	done   map[string]time.Time
}

func (a *Assembler) Accept(body []byte, now time.Time) (*Snapshot, error) {
	header, part, ok := strings.Cut(string(body), "\n")
	f := strings.Split(header, "|")
	if !ok || len(f) != 8 || f[0] != "2" || !requestID.MatchString(f[1]) || f[2] != a.Client {
		return nil, fmt.Errorf("invalid planner transfer header")
	}
	chunkSize := 5600
	at, e1 := strconv.ParseInt(f[3], 10, 64)
	size, e2 := strconv.Atoi(f[4])
	index, e3 := strconv.Atoi(f[5])
	count, e4 := strconv.Atoi(f[6])
	checksum, e5 := strconv.ParseUint(f[7], 10, 32)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || size < 1 || size > MaxTransfer || count != (size+chunkSize-1)/chunkSize || index < 1 || index > count || len(part) != min(chunkSize, size-(index-1)*chunkSize) {
		return nil, fmt.Errorf("invalid planner chunk")
	}
	stamp := time.Unix(at, 0)
	if now.Sub(stamp) > TransferTTL || stamp.Sub(now) > 30*time.Second {
		return nil, fmt.Errorf("expired planner transfer; click Run Sim again")
	}
	if a.done == nil {
		a.done = map[string]time.Time{}
	}
	for id, t := range a.done {
		if now.Sub(t) > TransferTTL {
			delete(a.done, id)
		}
	}
	if _, exists := a.done[f[1]]; exists {
		return nil, nil
	}
	identity := strings.Join([]string{f[1], f[0], f[2], f[3], f[4], f[6], f[7]}, "|")
	if a.active != nil && a.active.header != identity {
		oldID := strings.Split(a.active.header, "|")[0]
		if oldID == f[1] {
			return nil, fmt.Errorf("conflicting planner transfer metadata")
		}
		if stamp.Before(a.active.at) {
			return nil, fmt.Errorf("superseded planner transfer")
		}
		a.done[oldID] = now
		a.active = nil
	}
	if a.active == nil {
		a.active = &transfer{identity, stamp, size, count, uint32(checksum), map[int]string{}}
	}
	t := a.active
	if previous, exists := t.chunks[index]; exists && previous != part {
		return nil, fmt.Errorf("conflicting planner chunk")
	}
	t.chunks[index] = part
	if len(t.chunks) != count {
		return nil, nil
	}
	var data strings.Builder
	for i := 1; i <= count; i++ {
		data.WriteString(t.chunks[i])
	}
	if data.Len() != size || crc32.ChecksumIEEE([]byte(data.String())) != t.checksum {
		a.active = nil
		return nil, fmt.Errorf("planner snapshot checksum failed")
	}
	var value Snapshot
	err := Decode([]byte(data.String()), &value)
	if err == nil {
		err = value.Validate(a.Client, now)
	}
	s := &value
	if err != nil {
		a.active = nil
		return nil, err
	}
	a.done[f[1]] = now
	a.active = nil
	return s, nil
}
