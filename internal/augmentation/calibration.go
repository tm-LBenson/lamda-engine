package augmentation

import (
	"bufio"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Calibration conditions observed support on the actual buff combination,
// rather than applying a percentage to all damage, including unbuffed time.
type Calibration struct {
	Owner   string `json:"owner"`
	GUID    string `json:"guid"`
	Mask    int    `json:"mask"`
	Raw     int64  `json:"raw"`
	Support int64  `json:"support"`
}

var augmentationBuffs = map[int]int{395152: 1, 410089: 2, 360827: 4, 413984: 8}

func calibrationKey(owner, guid string, mask int) string {
	return fmt.Sprintf("%s|%s|%d", owner, guid, mask)
}
func (s *Store) hybridAura(f []string) {
	if len(f) < 12 || !player(f[1]) || !player(f[5]) {
		return
	}
	id, _ := strconv.Atoi(f[9])
	bit := augmentationBuffs[id]
	if bit == 0 {
		return
	}
	if !strings.HasPrefix(f[0], "SPELL_AURA_") {
		return
	}
	h := s.hybridState()
	key := f[1] + "|" + f[5]
	switch f[0] {
	case "SPELL_AURA_APPLIED", "SPELL_AURA_REFRESH", "SPELL_AURA_APPLIED_DOSE":
		h.buffs[key] |= bit
	case "SPELL_AURA_REMOVED":
		h.buffs[key] &= ^bit
	}
}
func (s *Store) learnCalibration(guid, owner string, mask int, amount int64, support bool) {
	if !player(guid) || !player(owner) || guid == owner || mask < 1 || mask > 15 {
		return
	}
	h := s.hybridState()
	key := calibrationKey(owner, guid, mask)
	c := h.calibrations[key]
	if c == nil {
		if len(h.calibrations) >= 4096 {
			return
		}
		c = &Calibration{Owner: owner, GUID: guid, Mask: mask}
		h.calibrations[key] = c
	}
	if support {
		c.Support += amount
	} else {
		c.Raw += amount
	}
}
func (s *Store) calibrationRate(owner, guid string, class, mask int) (float64, bool) {
	h := s.hybridState()
	if c := h.calibrations[calibrationKey(owner, guid, mask)]; c != nil && c.Raw > 0 && c.Support <= c.Raw {
		return float64(c.Support) / float64(c.Raw), true
	}
	var raw, support int64
	for _, c := range h.calibrations {
		if c.Owner == owner && c.Mask == mask && class > 0 && h.classes[c.GUID] == class && c.Support <= c.Raw {
			raw += c.Raw
			support += c.Support
		}
	}
	if raw > 0 {
		return float64(support) / float64(raw), true
	}
	return 0, false
}
func (s *Store) calibrationSnapshot() []Calibration {
	if s.hybrid == nil {
		return nil
	}
	out := []Calibration{}
	for _, c := range s.hybrid.calibrations {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		return calibrationKey(out[i].Owner, out[i].GUID, out[i].Mask) < calibrationKey(out[j].Owner, out[j].GUID, out[j].Mask)
	})
	return out
}

// SeedCalibrationLog is an offline, read-only training pass. A replay supplies
// its start as the cutoff so future support cannot leak into live estimates.
func (s *Store) SeedCalibrationLog(path string, before time.Time, classes map[string]int) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	t := New("")
	t.SetEnabled(true)
	for guid, class := range classes {
		t.hybridState().classes[guid] = class
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		at, _, err := engine.RecordFields(line)
		if err != nil {
			continue
		}
		if !before.IsZero() && !at.Before(before) {
			break
		}
		t.Ingest(line, at)
	}
	if e := scanner.Err(); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.hybridState()
	h.calibrations = map[string]*Calibration{}
	h.buffs = map[string]int{}
	for key, c := range t.hybridState().calibrations {
		value := *c
		h.calibrations[key] = &value
	}
	for key, mask := range t.hybridState().buffs {
		h.buffs[key] = mask
	}
	for guid, class := range classes {
		h.classes[guid] = class
	}
	return nil
}
