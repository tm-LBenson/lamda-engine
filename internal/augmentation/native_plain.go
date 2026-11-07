package augmentation

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"sort"
	"time"
)

// Non-damage metrics use Blizzard's live native totals directly. Damage support
// hooks cannot be reused to invent a healing redistribution.
func nativePlainMeter(n *lookup.NativeMeter, start, seen, now time.Time) *DamageMeter {
	v := &DamageMeter{ID: fmt.Sprintf("native/%s/%d/metric%d", n.Player, n.SessionID, n.Metric), Name: "Current combat", Kind: "combat", Owner: n.Player, Metric: n.Metric,
		StartedAt: start, Seconds: float64(n.DurationMillis) / 1000, Live: n.Combat && !nativeHistorical(n), Valid: now.Sub(seen) <= 1500*time.Millisecond, InputSource: "native", NativeSessionID: n.SessionID, NativeSeenAt: seen, Rows: []DamageRow{}}
	if !v.Live {
		v.EndedAt = start.Add(time.Duration(n.DurationMillis) * time.Millisecond)
	}
	if nativeHistorical(n) {
		v.Historical = true
		v.Name = fmt.Sprintf("Combat %d", n.SessionID)
		v.StartedAt, v.EndedAt = time.Time{}, time.Time{}
	}
	if !v.Valid {
		v.NativeError = "native_feed_stale"
	}
	for _, m := range n.Members {
		v.Rows = append(v.Rows, DamageRow{GUID: m.GUID, Name: m.Name, Raw: m.Total, Adjusted: m.Total, DPS: float64(m.Total) / max(1, v.Seconds)})
		v.RawTotal += m.Total
		v.AdjustedTotal += m.Total
	}
	sort.Slice(v.Rows, func(i, j int) bool {
		if v.Rows[i].Adjusted == v.Rows[j].Adjusted {
			return v.Rows[i].GUID < v.Rows[j].GUID
		}
		return v.Rows[i].Adjusted > v.Rows[j].Adjusted
	})
	return v
}
