package augmentation

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func (s *Store) meterImage(img *image.NRGBA) string {
	if img == nil || s.path == "" {
		return ""
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(img.Pix))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.meterImages == nil {
		s.meterImages = map[string]string{}
	}
	if path := s.meterImages[hash]; path != "" {
		return path
	}
	dir := filepath.Join(filepath.Dir(s.path), "native-meter-images")
	if os.MkdirAll(dir, 0700) != nil {
		return ""
	}
	path := filepath.Join(dir, hash+".png")
	if _, e := os.Stat(path); e != nil {
		f, e := os.CreateTemp(dir, "image-*.tmp")
		if e != nil {
			return ""
		}
		tmp := f.Name()
		defer os.Remove(tmp)
		e = png.Encode(f, img)
		closeErr := f.Close()
		if e != nil || closeErr != nil {
			return ""
		}
		if e = os.Rename(tmp, path); e != nil {
			return ""
		}
	}
	if len(s.meterImages) < 8192 {
		s.meterImages[hash] = path
	}
	return path
}
func (s *Store) decorateNativeBody(body *engine.MeterBody, n *lookup.NativeMeter) {
	if n.View == nil {
		return
	}
	images := map[string]lookup.NativeViewRow{}
	for _, r := range n.View.Rows {
		images[r.Key] = r
	}
	for i := range body.Rows {
		r, ok := images[body.Rows[i].ID]
		if !ok {
			continue
		}
		body.Rows[i].IconPath = s.meterImage(r.Icon)
		body.Rows[i].NameImage = s.meterImage(r.NameImage)
	}
}
func (s *Store) presentNativeView(n *lookup.NativeMeter, parent DamageMeter, a meterAppearance) *engine.MeterBody {
	v, f := n.View, n.Frame
	metric := meterMetricNames[n.Metric]
	body := &engine.MeterBody{Owner: n.Player, Segment: fmt.Sprintf("native/%s/%d", n.ID, n.SessionID), View: v.View, Actor: v.Actor, Metric: metric, Session: f.Label, Status: "WoW meter", Inspectable: v.View != "targets", HeaderHeight: a.HeaderHeight, FooterHeight: a.FooterHeight, RowHeight: a.RowHeight, FontSize: a.FontSize, Opacity: a.Opacity, Offset: v.Offset, TotalRows: v.TotalRows, Rows: []engine.MeterBodyRow{}}
	if v.Actor == "_" {
		body.Actor = ""
	}
	maximum := max(int64(1), v.Maximum)
	if v.View == "targets" {
		maximum = max(int64(1), v.Total)
	}
	for _, r := range v.Rows {
		color := "#4786A8"
		if a.ClassColors && meterClassColors[r.Class] != "" {
			color = meterClassColors[r.Class]
		}
		row := DamageRow{Adjusted: r.Total, DPS: float64(r.Rate)}
		name := r.Name
		if v.View == "actors" {
			name = strings.Split(name, "-")[0]
		}
		value := meterValue(row, v.Total, n.Metric, a.Values, true)
		if n.Metric == 9 && v.View == "actors" {
			value = fmt.Sprintf("%d:%02d", r.Rate/60, r.Rate%60)
			maximum = 1
		}
		body.Rows = append(body.Rows, engine.MeterBodyRow{ID: r.Key, Name: fmt.Sprintf("%d. %s", r.NativeIndex, name), Value: value, Fraction: min(1, float64(r.Total)/float64(maximum)), Color: color, SpellID: r.SpellID})
	}
	// The native spell API reports personal spell damage. Keep the exact native
	// spell counters and reconcile the support total in one explicit signed row
	// when a fully captured actor agrees with the parent native session.
	if v.View == "spells" && (n.Metric == 0 || n.Metric == 1) && v.TotalRows == len(v.Rows) && parent.Valid {
		for _, actor := range parent.Rows {
			if actor.GUID != v.Actor || actor.Raw != v.Total {
				continue
			}
			delta := actor.Adjusted - actor.Raw
			if delta != 0 {
				label := "Augmentation contributions"
				if delta < 0 {
					label = "Contributions credited to Augmentation"
				}
				value := fmt.Sprintf("%s%s", map[bool]string{true: "-", false: "+"}[delta < 0], meterNumber(float64(absMeter(delta))))
				body.Rows = append(body.Rows, engine.MeterBodyRow{ID: "support", Name: label, Value: value, Fraction: max(0, min(1, float64(delta)/float64(maximum))), Color: meterClassColors[13], Tooltip: "Combined support adjustment; individual spell attribution remains unavailable in this native view."})
				body.TotalRows++
			}
			body.Provisional = parent.Provisional
			break
		}
	}
	if len(body.Rows) == 0 {
		body.Empty = "No breakdown data"
	}
	return body
}
func absMeter(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
