package lookup

type OverlayLoadoutChange struct {
	Kind string `json:"kind"`
	Seq  int    `json:"seq"`
	Age  *int   `json:"age,omitempty"`
}

func ValidLoadoutChange(kind string) bool { return kind == "talents" || kind == "equipment" }
func sanitizeLoadoutChanges(p *OverlayInput) {
	if p.LoadoutChangesVersion != 1 || len(p.LoadoutChanges) > 2 {
		p.LoadoutChanges = nil
		return
	}
	seen := map[string]bool{}
	for _, c := range p.LoadoutChanges {
		if !ValidLoadoutChange(c.Kind) || seen[c.Kind] || c.Seq < 0 || c.Seq > 2147483647 || c.Age != nil && (*c.Age < 0 || *c.Age > 3000 || c.Seq == 0) {
			p.LoadoutChanges = nil
			return
		}
		seen[c.Kind] = true
	}
}
