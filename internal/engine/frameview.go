package engine

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

const FrameLifetime = 1200 * time.Millisecond

// Addon layout timestamps use WoW's integer server clock, which can differ
// from the Windows wall clock. Fresh physical capture still owns the shorter
// FrameLifetime lease; this tolerance never refreshes it or creates evidence.
const FrameTimestampSkew = 5 * time.Second

type FrameIcon struct {
	Module          string   `json:"module,omitempty"`
	Kick            bool     `json:"kick,omitempty"`
	GUID            string   `json:"guid"`
	Spell           int      `json:"spell"`
	Name            string   `json:"name"`
	Label           string   `json:"label"`
	X               int      `json:"x"`
	Y               int      `json:"y"`
	Size            int      `json:"size"`
	Opacity         int      `json:"opacity"`
	Status          string   `json:"status"`
	Ends            float64  `json:"ends"`
	Duration        float64  `json:"duration"`
	Source          string   `json:"source"`
	DeliverySeconds float64  `json:"deliverySeconds"`
	ChargesUnknown  bool     `json:"chargesUnknown"`
	Reasons         []string `json:"reasons"`
}

type OverlayDisplay struct {
	Finishing     float64        `json:"finishing,omitempty"`
	Started       float64        `json:"started,omitempty"`
	Instance      string         `json:"instance,omitempty"`
	ImagePath     string         `json:"imagePath,omitempty"`
	Visual        *OverlayVisual `json:"visual,omitempty"`
	Actor         string         `json:"actor,omitempty"`
	PixelX        *float64       `json:"pixelX,omitempty"`
	PixelY        *float64       `json:"pixelY,omitempty"`
	SortRemaining *float64       `json:"-"`
	SortStacks    *float64       `json:"-"`
	IconRequested bool           `json:"iconRequested,omitempty"`
	Spell         int            `json:"spell,omitempty"`
	IconPath      string         `json:"iconPath,omitempty"`
	Fraction      *float64       `json:"fraction,omitempty"`
	Arc           *OverlayArc    `json:"arc,omitempty"`
	Persistent    bool           `json:"persistent,omitempty"`
	ID            string         `json:"id"`
	Rule          string         `json:"rule"`
	Kind          string         `json:"kind"`
	Text          string         `json:"text"`
	X             float64        `json:"x"`
	Y             float64        `json:"y"`
	Size          float64        `json:"size"`
	Font          float64        `json:"font"`
	Opacity       float64        `json:"opacity"`
	Color         string         `json:"color"`
	Transparent   bool           `json:"transparent"`
	Glow          bool           `json:"glow"`
	Duration      float64        `json:"duration"`
	Ends          float64        `json:"ends"`
}

// Resolved arch data. Timed fill is animated by the renderer between captures.
type OverlayArc struct {
	Pair      string  `json:"pair"`
	Layout    string  `json:"layout"`
	Start     float64 `json:"start"`
	Sweep     float64 `json:"sweep"`
	Thickness float64 `json:"thickness"`
	Gap       float64 `json:"gap"`
	Reverse   bool    `json:"reverse"`
	Count     int     `json:"count"`
	Capacity  int     `json:"capacity"`
	Fraction  float64 `json:"fraction"`
	Timed     bool    `json:"timed"`
}
type OverlaySound struct {
	Path string  `json:"path,omitempty"`
	ID   string  `json:"id"`
	Name string  `json:"name"`
	At   float64 `json:"at"`
}
type CaptureArea struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type FrameDisplay struct {
	Meters               []*MeterWindow   `json:"meters,omitempty"`
	Meter                *MeterWindow     `json:"meter,omitempty"`
	LowResourceRendering bool             `json:"lowResourceRendering,omitempty"`
	CaptureAreas         []CaptureArea    `json:"captureAreas,omitempty"`
	OverlaysActive       bool             `json:"overlaysActive,omitempty"`
	Overlays             []OverlayDisplay `json:"overlays,omitempty"`
	OverlaySounds        []OverlaySound   `json:"overlaySounds,omitempty"`
	Notices              []FrameNotice    `json:"notices,omitempty"`
	KeepUIWhenUnfocused  bool             `json:"keepUIWhenUnfocused,omitempty"`
	Background           bool             `json:"background,omitempty"`
	WindowPID            uint32           `json:"windowPID,omitempty"`
	KickBars             []KickBar        `json:"kickBars,omitempty"`
	KickPanel            *KickPanel       `json:"kickPanel,omitempty"`
	Enabled              bool             `json:"enabled"`
	Status               string           `json:"status"`
	Window               string           `json:"window"`
	Width                int              `json:"width"`
	Height               int              `json:"height"`
	At                   float64          `json:"at"`
	Expires              float64          `json:"expires"`
	GeometryAge          float64          `json:"geometryAge"`
	Omitted              int              `json:"omitted"`
	NativeHidden         int              `json:"nativeHidden"`
	Icons                []FrameIcon      `json:"icons"`
}

// MeterWindow bounds identify the one interactive meter. All other overlay
// windows retain their existing click-through behavior.
type MeterWindow struct {
	ID           int              `json:"id,omitempty"`
	HeaderHeight int              `json:"headerHeight,omitempty"`
	Body         *MeterBody       `json:"body,omitempty"`
	Metric       string           `json:"metric,omitempty"`
	Menu         *MeterNativeMenu `json:"menu,omitempty"`
	NativeHost   bool             `json:"nativeHost,omitempty"`
	Choices      []MeterChoice    `json:"choices,omitempty"`
	Scope        string           `json:"scope"`
	X            float64          `json:"x"`
	Y            float64          `json:"y"`
	Width        float64          `json:"width"`
	Height       float64          `json:"height"`
	Locked       bool             `json:"locked"`
}
type MeterNativeMenu struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type MeterChoice struct {
	Scope string `json:"scope"`
	Label string `json:"label"`
}
type FrameNotice struct {
	Text      string  `json:"text"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Width     int     `json:"width"`
	Opacity   int     `json:"opacity"`
	Ends      float64 `json:"ends"`
	Countdown bool    `json:"countdown"`
}

// FrameView joins observations to current public GUID bindings, never slot order
// or character names. Rendering and timer expiry cannot create observations.
func FrameView(s InferenceSnapshot, c Config, geometry *framewire.Snapshot, received, now time.Time, window uintptr) FrameDisplay {
	d := FrameDisplay{Status: "disabled", Icons: []FrameIcon{}, At: float64(now.UnixMilli()) / 1000}
	if !c.TrackingEnabled || !c.FrameEstimates || !c.LookupBridge {
		return d
	}
	d.Status = "waiting_for_frames"
	if geometry == nil || received.IsZero() {
		return d
	}
	d.GeometryAge = now.Sub(received).Seconds()
	if now.Before(received) || now.Sub(received) > FrameLifetime || now.Sub(geometry.At) > FrameTimestampSkew || geometry.At.Sub(now) > FrameTimestampSkew {
		d.Status = "frame_data_stale"
		return d
	}
	d.Enabled = true
	d.Status = "receiving"
	d.Window = strconv.FormatUint(uint64(window), 10)
	d.Width, d.Height = geometry.Width, geometry.Height
	d.Expires = float64(received.Add(FrameLifetime).UnixMilli()) / 1000
	d.Omitted = geometry.Omitted
	selected := map[int]bool{}
	for _, id := range c.FrameSpells {
		selected[CooldownSpell(id)] = true
	}
	ownSelected := map[int]bool{}
	for id := range selected {
		ownSelected[id] = true
	}
	for _, frame := range geometry.Frames {
		if frame.GUID != geometry.Player {
			continue
		}
		for _, slot := range frame.SelfSlots {
			for _, id := range slot.Spells {
				if selected[id] {
					for _, alias := range slot.Spells {
						ownSelected[alias] = true
					}
					break
				}
			}
		}
	}
	byGUID := map[string][]CooldownEvidence{}
	evidence := s.Cooldowns
	if geometry.Persistent && geometry.Unknown {
		evidence = append(append([]CooldownEvidence{}, s.Cooldowns...), knownCooldowns(s, now)...)
		evidence = append(evidence, candidateCooldowns(s, now, geometry.Player)...)
		evidence = append(evidence, ownSlotCooldowns(evidence, geometry)...)
	}
	for _, e := range evidence {
		if IsInterrupt(e.Spell) {
			continue
		}
		self := e.GUID == geometry.Player
		if c.FrameSpells != nil && ((!self && !selected[e.Spell]) || (self && !ownSelected[e.Spell])) {
			continue
		}
		if self && !geometry.Self || !self && !geometry.Party {
			continue
		}
		if e.Status == "estimated_ready" && !geometry.Ready && !geometry.Persistent || e.Status == "unknown" && !geometry.Unknown {
			continue
		}
		if e.Status != "estimated_cooldown" && e.Status != "estimated_recharge" && e.Status != "estimated_ready" && e.Status != "unknown" {
			continue
		}
		byGUID[e.GUID] = append(byGUID[e.GUID], e)
	}
	for guid := range byGUID {
		sort.Slice(byGUID[guid], func(i, j int) bool {
			a, b := byGUID[guid][i], byGUID[guid][j]
			active := func(e CooldownEvidence) bool {
				return e.Status == "estimated_cooldown" || e.Status == "estimated_recharge"
			}
			if geometry.Persistent && active(a) != active(b) {
				return active(a)
			}
			if (a.Source == "spec_candidate") != (b.Source == "spec_candidate") {
				return a.Source != "spec_candidate"
			}
			return a.Spell < b.Spell
		})
	}
	for _, frame := range geometry.Frames {
		ownSlots := frame.GUID == geometry.Player && frame.SelfSlots != nil
		layout := framewire.Layout{Size: geometry.Size, Spacing: geometry.Spacing, PerRow: geometry.PerRow, Maximum: geometry.Maximum, Opacity: geometry.Opacity, Anchor: geometry.Anchor, Gap: geometry.Gap, OffsetX: geometry.OffsetX, OffsetY: geometry.OffsetY, Growth: geometry.Growth}
		matchNative := frame.GUID != geometry.Player && geometry.PartyLayout != nil
		if matchNative {
			layout = *geometry.PartyLayout
		}
		entries := make([]CooldownEvidence, 0, len(byGUID[frame.GUID]))
		for _, e := range byGUID[frame.GUID] {
			if ownSlots && ownSlotIndex(frame.SelfSlots, e.Spell) < 0 {
				continue
			}
			// Prefer a working native widget for this spell on this exact frame.
			// Retain evidence so fallback needs no new cast if native data fails.
			if frame.GUID == geometry.Player && slices.Contains(frame.NativeSpells, e.Spell) {
				d.NativeHidden++
				continue
			}
			entries = append(entries, e)
		}
		if !ownSlots && len(entries) > layout.Maximum {
			d.Omitted += len(entries) - layout.Maximum
			entries = entries[:layout.Maximum]
		}
		if len(entries) == 0 {
			continue
		}
		capacity := len(entries)
		if matchNative {
			capacity = layout.Maximum
		}
		columns := min(capacity, layout.PerRow)
		rows := (capacity + columns - 1) / columns
		width := columns*layout.Size + (columns-1)*layout.Spacing
		height := rows*layout.Size + (rows-1)*layout.Spacing
		x, y := frame.X, frame.Y+frame.Height+layout.Gap
		switch layout.Anchor {
		case 2:
			y = frame.Y - height - layout.Gap
		case 3:
			x = frame.X - width - layout.Gap
			y = frame.Y
		case 4:
			x = frame.X + frame.Width + layout.Gap
			y = frame.Y
		}
		x += layout.OffsetX
		y -= layout.OffsetY
		reverse := layout.Growth == 3 || (layout.Growth == 1 && layout.Anchor == 3)
		usedSlots := map[int]bool{}
		for i, e := range entries {
			column := i % columns
			if reverse {
				column = columns - 1 - column
			}
			ix := x + column*(layout.Size+layout.Spacing)
			iy := y + (i/columns)*(layout.Size+layout.Spacing)
			size, opacity := layout.Size, layout.Opacity
			if ownSlots {
				index := ownSlotIndex(frame.SelfSlots, e.Spell)
				if usedSlots[index] {
					continue
				}
				usedSlots[index] = true
				slot := frame.SelfSlots[index]
				ix, iy, size, opacity = slot.X, slot.Y, slot.Size, slot.Opacity
			}
			if ix < 0 || iy < 0 || ix+size > d.Width || iy+size > d.Height {
				d.Omitted++
				continue
			}
			stripX := max(0, min(c.LookupBridgeX, d.Width-128))
			stripY := max(0, min(c.LookupBridgeY, d.Height-16))
			stripTop := stripY
			if stripY+24 > geometry.Height {
				stripTop -= 8
			}
			if ix < stripX+128 && ix+size > stripX && iy < stripTop+24 && iy+size > stripTop {
				d.Omitted++
				continue
			}
			label := ""
			for _, word := range strings.Fields(e.Name) {
				for _, r := range word {
					label += string(r)
					break
				}
			}
			if label == "" {
				label = fmt.Sprint(e.Spell)
			} else if len(strings.Fields(e.Name)) == 1 {
				runes := []rune(e.Name)
				label = strings.ToUpper(string(runes[:min(3, len(runes))]))
			}
			displaySpell := e.Spell
			if e.DisplaySpell > 0 {
				displaySpell = e.DisplaySpell
			}
			d.Icons = append(d.Icons, FrameIcon{GUID: e.GUID, Spell: displaySpell, Name: e.Name, Label: label, X: ix, Y: iy, Size: size, Opacity: opacity,
				Status: e.Status, Ends: e.EstimatedEnd, Duration: evidenceDuration(e), Source: e.Source, DeliverySeconds: e.DeliverySeconds, ChargesUnknown: e.MayHaveCharges,
				Reasons: append([]string{}, e.Reasons...)})
		}
	}
	if len(geometry.Frames) == 0 {
		d.Status = "no_visible_frames"
	} else if len(d.Icons) == 0 {
		if d.NativeHidden > 0 {
			d.Status = "native_cooldowns"
		} else {
			d.Status = "waiting_for_observations"
		}
	}
	return d
}

func evidenceDuration(e CooldownEvidence) float64 {
	if e.CooldownSeconds > 0 {
		return e.CooldownSeconds
	}
	return e.BaselineSeconds
}
