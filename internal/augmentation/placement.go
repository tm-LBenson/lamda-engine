package augmentation

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
)

// The renderer and native auto-selection events share this placement file. Neither writes
// WoW SavedVariables. Coordinates are fractions of the verified client area.
type placement struct {
	ScopeOnly bool    `json:"scopeOnly,omitempty"`
	Schema    int     `json:"schema"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Locked    bool    `json:"locked"`
	Scope     string  `json:"scope"`
}

func (s *Store) placement() *placement {
	if s.path == "" {
		return nil
	}
	f, err := os.Open(filepath.Join(filepath.Dir(s.path), "display.json"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var p placement
	if json.NewDecoder(io.LimitReader(f, 2048)).Decode(&p) != nil || p.Schema != 1 || !validMeterScope(p.Scope) {
		return nil
	}
	if p.ScopeOnly {
		return &p
	}
	if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) ||
		p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 || p.Width < 300 || p.Width > 1000 ||
		p.Height < 120 || p.Height > 1000 || !validMeterScope(p.Scope) {
		return nil
	}
	return &p
}

func (s *Store) desktopMeter() bool {
	if s.path == "" {
		return false
	}
	f, err := os.Open(filepath.Join(filepath.Dir(filepath.Dir(s.path)), "overlay-windows.json"))
	if err != nil {
		return false
	}
	defer f.Close()
	var doc struct {
		Schema  int `json:"schema"`
		Windows map[string]struct {
			Detached bool `json:"detached"`
		} `json:"windows"`
	}
	if json.NewDecoder(io.LimitReader(f, 131072)).Decode(&doc) != nil || doc.Schema != 1 {
		return false
	}
	for id, w := range doc.Windows {
		if w.Detached && (id == "meters" || len(id) == 7 && id[:6] == "meters" && id[6] >= '2' && id[6] <= '8') {
			return true
		}
	}
	return false
}
