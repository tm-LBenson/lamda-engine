package lookup

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
)

// OverlayInput binds public character context and native visual readings to one
// capture. Unknown values stay nil; a failed tile never means aura absent.
type OverlayInput struct {
	NativeVersion         int                        `json:"nativeVersion,omitempty"`
	PetVersion            int                        `json:"petVersion,omitempty"`
	Pet                   *OverlayPet                `json:"pet,omitempty"`
	LoadoutChangesVersion int                        `json:"loadoutChangesVersion,omitempty"`
	LoadoutChanges        []OverlayLoadoutChange     `json:"loadoutChanges,omitempty"`
	EquipmentVersion      int                        `json:"equipmentVersion,omitempty"`
	Equipment             *OverlayEquipment          `json:"equipment,omitempty"`
	HeroSpec              *int                       `json:"heroSpec,omitempty"`
	TalentsVersion        int                        `json:"talentsVersion,omitempty"`
	TalentHash            string                     `json:"talentHash,omitempty"`
	Talents               []OverlayTalent            `json:"talents,omitempty"`
	BossesVersion         int                        `json:"bossesVersion,omitempty"`
	Bosses                *OverlayBosses             `json:"bosses,omitempty"`
	EncountersVersion     int                        `json:"encountersVersion,omitempty"`
	Encounter             *OverlayEncounter          `json:"encounter,omitempty"`
	CombatVersion         int                        `json:"combatVersion,omitempty"`
	IdentityHash          string                     `json:"identityHash,omitempty"`
	Identities            []OverlayIdentity          `json:"identities,omitempty"`
	Logging               *bool                      `json:"logging,omitempty"`
	CastsVersion          int                        `json:"castsVersion,omitempty"`
	CastHash              string                     `json:"castHash,omitempty"`
	Casts                 []OverlayCast              `json:"casts,omitempty"`
	MeasurementsVersion   int                        `json:"measurementsVersion,omitempty"`
	MeasurementHash       string                     `json:"measurementHash,omitempty"`
	Measurements          []OverlayMeasurement       `json:"measurements,omitempty"`
	UnitsVersion          int                        `json:"unitsVersion,omitempty"`
	UnitHash              string                     `json:"unitHash,omitempty"`
	UnitTokens            []string                   `json:"unitTokens,omitempty"`
	Units                 []OverlayUnit              `json:"units,omitempty"`
	AnchorsVersion        int                        `json:"anchorsVersion,omitempty"`
	Anchors               []OverlayAnchor            `json:"anchors,omitempty"`
	AuraDurationsVersion  int                        `json:"auraDurationsVersion,omitempty"`
	CooldownsVersion      int                        `json:"cooldownsVersion,omitempty"`
	CooldownHash          string                     `json:"cooldownHash,omitempty"`
	Cooldowns             []OverlayCooldown          `json:"cooldowns,omitempty"`
	CaptureError          string                     `json:"captureError,omitempty"`
	StripWidth            int                        `json:"stripWidth,omitempty"`
	Epoch                 string                     `json:"epoch"`
	Seq                   int                        `json:"seq"`
	At                    int64                      `json:"at"`
	Hash                  string                     `json:"hash"`
	Width                 int                        `json:"width"`
	Height                int                        `json:"height"`
	Offset                int                        `json:"offset"`
	Native                bool                       `json:"native"`
	Character             string                     `json:"character"`
	Realm                 string                     `json:"realm"`
	Class                 string                     `json:"class"`
	Spec                  int                        `json:"spec"`
	Race                  string                     `json:"race"`
	Faction               string                     `json:"faction"`
	Role                  string                     `json:"role"`
	Location              string                     `json:"location"`
	Zone                  string                     `json:"zone"`
	Group                 string                     `json:"group"`
	Level                 int                        `json:"level"`
	Combat                bool                       `json:"combat"`
	Mounted               bool                       `json:"mounted"`
	Alive                 bool                       `json:"alive"`
	Key                   bool                       `json:"key"`
	Cast                  int                        `json:"cast"`
	CastSeq               int                        `json:"castSeq"`
	Watches               []OverlayAura              `json:"watches"`
	Health                *float64                   `json:"health,omitempty"`
	ResourceMods          map[string]float64         `json:"resourceMods,omitempty"`
	ResourcesVersion      int                        `json:"resourcesVersion"`
	PrimaryPower          int                        `json:"primaryPower"`
	Resources             map[string]OverlayResource `json:"resources,omitempty"`
}
type OverlayResource struct {
	Current float64 `json:"current"`
	Max     float64 `json:"max"`
}

type OverlayAura struct {
	Generation        int      `json:"g,omitempty"`
	DurationAvailable bool     `json:"durationAvailable,omitempty"`
	Duration          *float64 `json:"duration,omitempty"`
	Spell             int      `json:"spell"`
	Kind              string   `json:"kind"`
	Unit              string   `json:"unit"`
	Available         bool     `json:"available"`
	Active            *bool    `json:"active,omitempty"`
	Stacks            *float64 `json:"stacks,omitempty"`
	Remaining         *float64 `json:"remaining,omitempty"`
	Permanent         bool     `json:"permanent,omitempty"`
}

func parseOverlays(b []byte) BridgeSample {
	var err error
	b, err = expandOverlayMetadata(b)
	if err != nil {
		return BridgeSample{Error: "overlays_compression_invalid"}
	}
	var p OverlayInput
	if json.Unmarshal(b, &p) != nil || len(p.CaptureError) > 240 || p.Epoch == "" || len(p.Epoch) > 80 || len(p.Hash) != 16 || p.Seq < 1 || p.Width < 256 || p.Width > 16000 || p.Height < 100 || p.Height > 16000 || len(p.Watches) > overlaySourceLimit(p.NativeVersion) || (p.Offset != 32 && p.Offset != -40) {
		return BridgeSample{Error: "overlays_metadata_invalid"}
	}
	if p.NativeVersion != 0 && p.NativeVersion != 2 && p.NativeVersion != 3 {
		return BridgeSample{Error: "overlays_native_version_invalid"}
	}
	if p.StripWidth != 0 && !ValidOverlayWidth(p.StripWidth) {
		return BridgeSample{Error: "overlays_shape_invalid"}
	}
	if len(p.Measurements) > overlaySourceLimit(p.NativeVersion) || (p.MeasurementsVersion == 1 && len(p.MeasurementHash) != 16) {
		return BridgeSample{Error: "overlays_measurements_invalid"}
	}
	for i := range p.Measurements {
		p.Measurements[i].Current = nil
		p.Measurements[i].Max = nil
	}
	sanitizeAnchors(&p)
	sanitizeOverlayUnits(&p)
	sanitizeOverlayIdentities(&p)
	sanitizeOverlayEncounter(&p)
	sanitizeOverlayPet(&p)
	sanitizeOverlayBosses(&p)
	sanitizeOverlayTalents(&p)
	sanitizeLoadoutChanges(&p)
	sanitizeOverlayEquipment(&p)
	if p.HeroSpec != nil && (*p.HeroSpec < 1 || *p.HeroSpec >= 10000000) {
		p.HeroSpec = nil
	}
	// Never trust native measurements from the JSON metadata itself.
	p.Health = nil
	p.Resources = nil
	if len(p.Cooldowns) > overlayTimerLimit(p.NativeVersion) || (p.CooldownsVersion == 1 && len(p.CooldownHash) != 16) {
		return BridgeSample{Error: "overlays_cooldowns_invalid"}
	}
	if len(p.Cooldowns)+len(p.Casts)*2 > overlayTimerLimit(p.NativeVersion) || (p.CastsVersion == 1 && len(p.CastHash) != 16) {
		return BridgeSample{Error: "overlays_casts_invalid"}
	}
	for i := range p.Casts {
		c := &p.Casts[i]
		c.Mode = nil
		c.Spell = nil
		c.Remaining = nil
		c.Duration = nil
		c.Interruptible = nil
		for j := range c.Events {
			c.Events[j].Spell = nil
		}
	}
	for i, c := range p.Cooldowns {
		if !ValidCooldownWatch(c.Kind, c.ID) {
			return BridgeSample{Error: "overlays_cooldown_invalid"}
		}
		p.Cooldowns[i] = OverlayCooldown{Kind: c.Kind, ID: c.ID, Available: c.Available}
	}
	for i := range p.Watches {
		a := &p.Watches[i]
		if a.Generation < 0 || a.Generation > 2147483647 || !OverlayUnitToken(a.Unit) || a.Spell < 1 || a.Spell > 10000000 || (a.Kind != "buff" && a.Kind != "debuff") {
			return BridgeSample{Error: "overlays_watch_invalid"}
		}
		a.Active = nil
		a.Stacks = nil
		a.Remaining = nil
		a.Duration = nil
		a.Permanent = false
	}
	return BridgeSample{Source: "overlays", Overlays: &p}
}
func overlayColor(p Pixels, x, y int) (int, bool) {
	r, g, b := p.rgb(x+1, y+1)
	for _, v := range [][2]int{{2, 1}, {1, 2}, {2, 2}} {
		a, c, d := p.rgb(x+v[0], y+v[1])
		if a != r || c != g || d != b {
			return 0, false
		}
	}
	return int(r)<<16 | int(g)<<8 | int(b), true
}

var OverlayCalibration = []int{0, 255, 65280, 16711680, 16777215, 0x123456, 0xabcdef, 0x808080}

func DecodeOverlayPixels(p Pixels, packetY, panelY int) BridgeSample {
	s := DecodeSample(p.Cells(image.Rect(0, packetY, 256, packetY+32)))
	if s.Error != "" || s.Overlays == nil {
		return BridgeSample{Error: "overlays_capture_invalid"}
	}
	a := s.Overlays
	if a.Offset != panelY-packetY {
		return BridgeSample{Error: "overlays_mapping_changed"}
	}
	if !a.Native {
		return s
	}
	if panelY < 0 || panelY+40 > p.Height {
		return BridgeSample{Error: "overlays_panel_outside_window"}
	}
	for i, v := range OverlayCalibration {
		x, y := overlayCalibrationPoint(i, a.NativeVersion)
		c, ok := overlayNativeColor(p, x, panelY+y, a.NativeVersion)
		if !ok || v != c {
			return BridgeSample{Error: "overlays_calibration"}
		}
	}
	for i := range a.Watches {
		w := &a.Watches[i]
		if a.CaptureError != "" || !w.Available {
			continue
		}
		x, y := overlayAuraPoint(i, 0, 0, a.NativeVersion)
		x2, y2 := overlayAuraPoint(i, 0, 1, a.NativeVersion)
		// Two independently bound native slots must agree on presence and measurements.
		present, ok := overlayNativeColor(p, x, panelY+y, a.NativeVersion)
		copy, ok2 := overlayNativeColor(p, x2, panelY+y2, a.NativeVersion)
		if !ok || !ok2 || present != copy || (present != 0x19354b && present != 0x37d6a0) {
			continue
		}
		active := present == 0x37d6a0
		w.Active = &active
		if !active {
			continue
		}
		for field := 0; field < 3; field++ {
			x, y := overlayAuraPoint(i, field+1, 0, a.NativeVersion)
			x2, y2 := overlayAuraPoint(i, field+1, 1, a.NativeVersion)
			v, ok := overlayNativeColor(p, x, panelY+y, a.NativeVersion)
			v2, ok2 := overlayNativeColor(p, x2, panelY+y2, a.NativeVersion)
			if !ok || !ok2 || v != v2 {
				continue
			}
			if field == 0 && v <= 999 {
				n := float64(v)
				w.Stacks = &n
			}
			if field == 1 {
				if v == 0xffffff {
					w.Permanent = true
				} else if v <= 864000 {
					n := float64(v) / 10
					w.Remaining = &n
				}
			}
			if field == 2 && w.DurationAvailable && v >= 0x100000 && v < 0x100000+864001 {
				n := float64(v-0x100000) / 10
				w.Duration = &n
			}
		}
	}
	hx, hy := overlayHealthPoint(0, a.NativeVersion)
	hx2, hy2 := overlayHealthPoint(1, a.NativeVersion)
	h, ok := overlayNativeColor(p, hx, panelY+hy, a.NativeVersion)
	h2, ok2 := overlayNativeColor(p, hx2, panelY+hy2, a.NativeVersion)
	if ok && ok2 && h == h2 && h <= 100 {
		n := float64(h)
		if !math.IsNaN(n) {
			a.Health = &n
		}
	}
	// Interleaved free rows carry paired current/capacity tiles for all powers.
	// A zero capacity, failed pair, or older addon means unknown, never zero.
	if a.ResourcesVersion == 1 {
		a.Resources = map[string]OverlayResource{}
		for power := 0; power < 32; power++ {
			read := func(field int) (int, bool) {
				x, y := overlayResourcePoint(power, field, a.NativeVersion)
				return overlayNativeColor(p, x, panelY+y, a.NativeVersion)
			}
			current, ok := read(0)
			current2, ok2 := read(1)
			capacity, ok3 := read(2)
			capacity2, ok4 := read(3)
			if ok && ok2 && ok3 && ok4 && current == current2 && capacity == capacity2 && capacity > 0 && capacity < 0xffffff && current <= capacity {
				key := fmt.Sprint(power)
				mod := a.ResourceMods[key]
				if mod > 0 && mod <= 1000 && !math.IsNaN(mod) && !math.IsInf(mod, 0) {
					a.Resources[key] = OverlayResource{float64(current) / mod, float64(capacity) / mod}
				}
			}
		}
	}
	decodeOverlayCooldowns(p, panelY, a)
	decodeOverlayCasts(p, panelY, a)
	decodeOverlayMeasurements(p, panelY, a)
	return s
}
func (a OverlayAura) Key() string { return fmt.Sprintf("%s:%s:%d", a.Unit, a.Kind, a.Spell) }

// Shapes redistribute 1152 whole 4x4 tiles; area and calibration stay identical.
func ValidOverlayWidth(width int) bool {
	return width >= 16 && width <= 1152 && width%4 == 0 && 18432%width == 0 && (18432/width)%4 == 0
}

// Rebuild the canonical 256x72 image from one atomic rectangular capture.
func DecodeOverlayShape(p Pixels) BridgeSample {
	if !ValidOverlayWidth(p.Width) || p.Height != 18432/p.Width || len(p.BGRA) != 18432*4 {
		return BridgeSample{Error: "overlays_shape_invalid"}
	}
	canonical := Pixels{Width: 256, Height: 72, BGRA: make([]byte, 18432*4)}
	for y := 0; y < 72; y++ {
		for x := 0; x < 256; x++ {
			index := (y/4)*64 + x/4
			sx, sy := (index%(p.Width/4))*4+x%4, (index/(p.Width/4))*4+y%4
			copy(canonical.BGRA[(y*256+x)*4:(y*256+x+1)*4], p.BGRA[(sy*p.Width+sx)*4:(sy*p.Width+sx+1)*4])
		}
	}
	s := DecodeOverlayPixels(canonical, 0, 32)
	if s.Error == "" && s.Overlays.StripWidth != p.Width {
		return BridgeSample{Error: "overlays_mapping_changed"}
	}
	return s
}
