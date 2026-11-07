package lookup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/framewire"
)

// 512 RGB cells, each 2x2 physical pixels, at a configurable client position.
// The CRC rejects partial rendering, occlusion, and altered display colors.
const BridgeWidth = 128
const BridgeHeight = 16
const BridgeBytes = 1536

// Simulation v2 alone uses a wider independent strip. Tracking keeps its
// original geometry and packet size.
const SimBridgeWidth = 256
const SimBridgeHeight = 32
const SimBridgeBytes = 6144

// BridgePosition is measured in physical pixels from the client area's top left.
// Keep a profile's preferred origin when a smaller window temporarily clamps it.
type BridgePosition struct{ X, Y int }

func (p BridgePosition) Clamp(width, height int) (BridgePosition, bool) {
	return p.clampSize(width, height, BridgeWidth, BridgeHeight)
}

func (p BridgePosition) clampSize(width, height, stripWidth, stripHeight int) (BridgePosition, bool) {
	if width < stripWidth || height < stripHeight {
		return BridgePosition{}, false
	}
	return BridgePosition{max(0, min(p.X, 16000, width-stripWidth)), max(0, min(p.Y, 16000, height-stripHeight))}, true
}

func DecodePacket(data []byte) (Event, error) {
	sample := DecodeSample(data)
	if sample.Error != "" {
		return Event{}, fmt.Errorf("%s", sample.Error)
	}
	return sample.Event, nil
}

func DecodeSample(data []byte) BridgeSample {
	if len(data) < 10 {
		return BridgeSample{Error: "bridge_not_visible"}
	}
	magic := string(data[:4])
	expected := BridgeBytes
	if magic == "LUS2" || magic == "LUR2" || magic == "LUK2" || magic == "LUO1" || magic == "LUM1" {
		expected = SimBridgeBytes
	} else if magic != "LUQ1" && magic != "LUI1" && magic != "LUI2" && magic != "LUI3" && magic != "LUI4" && magic != "LUF4" && magic != "LUS1" && magic != "LUA1" && magic != "LCD1" && magic != "LKI1" && magic != "FCD1" && magic != "FKI1" {
		return BridgeSample{Error: "bridge_not_visible"}
	}
	if len(data) != expected {
		return BridgeSample{Error: "bridge_not_visible"}
	}
	size := int(binary.BigEndian.Uint16(data[4:6]))
	if size < 1 || size > expected-10 {
		return BridgeSample{Error: "bridge_invalid_size"}
	}
	body := data[10 : 10+size]
	if binary.BigEndian.Uint32(data[6:10]) != crc32.ChecksumIEEE(body) {
		return BridgeSample{Error: "bridge_checksum"}
	}
	if magic == "LUQ1" {
		if len(body) > 512 || !bytes.HasPrefix(body, []byte("1|")) {
			return BridgeSample{Error: "queue_bridge_invalid"}
		}
		return BridgeSample{Source: "queue", Queue: append([]byte(nil), body...)}
	}
	if magic == "LUM1" {
		return parseNativeMeter(body)
	}
	if magic == "LUO1" {
		return parseOverlays(body)
	}
	if magic == "LUA1" {
		sample := parseArena(string(body))
		sample.Source = "gladius"
		return sample
	}
	if magic == "LUK2" {
		if !bytes.HasPrefix(body, []byte("2|")) {
			return BridgeSample{Error: "kbb_bridge_version"}
		}
		return BridgeSample{Source: "kbb", KBB: append([]byte(nil), body...)}
	}
	if magic == "LUR2" {
		if len(body) > 512 || !bytes.HasPrefix(body, []byte("2|")) {
			return BridgeSample{Error: "reload_bridge_invalid"}
		}
		return BridgeSample{Source: "sim", Reload: append([]byte(nil), body...)}
	}
	if magic == "LUS1" || magic == "LUS2" {
		// Keep the outer geometry/version bound to its transfer header. This
		// also prevents a narrow read from accepting the first row of v2.
		if len(body) < 2 || body[0] != magic[3] || body[1] != '|' {
			return BridgeSample{Error: "sim_bridge_version"}
		}
		return BridgeSample{Source: "sim", Sim: append([]byte(nil), body...)}
	}
	parts := strings.Split(string(body), "\n\n")
	e, err := ParseEvent(parts[0])
	if err != nil {
		return BridgeSample{Error: err.Error()}
	}
	sample := BridgeSample{Event: e, Forever: magic == "LUF4" || magic == "FCD1" || magic == "FKI1"}
	switch magic {
	case "LCD1", "FCD1":
		sample.Source = "cd"
		magic = "LUI4"
	case "LKI1", "FKI1":
		sample.Source = "kick"
		magic = "LUI4"
	}
	if magic == "LUI3" || (magic == "LUI4" || magic == "LUF4") {
		count := 3
		if magic == "LUI4" || magic == "LUF4" {
			count = 4
		}
		if len(parts) != count {
			return BridgeSample{Error: "observation_bridge_missing"}
		}
		if count == 4 && parts[3] != "" {
			sample.Kick, err = framewire.ParseKick(parts[3])
			if err != nil {
				return BridgeSample{Error: err.Error()}
			}
			for i := range sample.Kick.Members {
				m := &sample.Kick.Members[i]
				for _, c := range e.Members {
					if c.GUID == m.GUID {
						m.Name = c.Name
						break
					}
				}
				if m.Name == "" && m.GUID == sample.Kick.Player {
					m.Name = "You"
				}
				if m.Name == "" {
					return BridgeSample{Error: "kick_member_not_in_roster"}
				}
			}
		}
		sample.Observations, err = parseObservations(parts[2])
		if err != nil {
			return BridgeSample{Error: err.Error()}
		}
		if a := sample.Observations.Auras; a != nil {
			for i := range a.Members {
				m := &a.Members[i]
				if m.Member > len(e.Members) {
					return BridgeSample{Error: "aura_member_not_in_roster"}
				}
				m.GUID = e.Members[m.Member-1].GUID
			}
		}
		if s := sample.Observations.Screen; s != nil {
			for i := range s.Members {
				m := &s.Members[i]
				if m.Member > len(e.Members) {
					return BridgeSample{Error: "screen_member_not_in_roster"}
				}
				m.GUID = e.Members[m.Member-1].GUID
			}
		}
		if parts[1] != "" {
			sample.Frames, err = framewire.Parse(parts[1])
			if err != nil {
				return BridgeSample{Error: err.Error()}
			}
		}
	} else if magic == "LUI2" {
		if len(parts) != 2 {
			return BridgeSample{Error: "frame_bridge_missing"}
		}
		sample.Frames, err = framewire.Parse(parts[1])
		if err != nil {
			return BridgeSample{Error: err.Error()}
		}
	} else if len(parts) != 1 {
		return BridgeSample{Error: "bridge_invalid_event"}
	}
	if sample.Source == "cd" && (sample.Kick != nil || sample.Observations.Screen != nil) ||
		sample.Source == "kick" && (sample.Frames != nil || sample.Observations.Auras != nil) {
		return BridgeSample{Error: "bridge_module_payload"}
	}
	return sample
}
func ParseEvent(body string) (Event, error) {
	if len(body) > BridgeBytes-10 {
		return Event{}, fmt.Errorf("bridge_invalid_size")
	}
	lines := strings.Split(body, "\n")
	head := strings.Split(lines[0], "|")
	if (len(head) != 4 && len(head) != 5) || len(lines) > 6 {
		return Event{}, fmt.Errorf("bridge_invalid_event")
	}
	stamp, err := strconv.ParseInt(head[1], 10, 64)
	if err != nil {
		return Event{}, fmt.Errorf("bridge_invalid_time")
	}
	deadline, err := strconv.ParseInt(head[3], 10, 64)
	if err != nil {
		return Event{}, fmt.Errorf("bridge_invalid_time")
	}
	e := Event{ID: head[0], At: time.Unix(stamp, 0), Kind: head[2], Members: []Character{}}
	if len(head) == 5 {
		if head[4] != "0" && head[4] != "1" {
			return Event{}, fmt.Errorf("bridge_invalid_visibility")
		}
		e.KeepUIWhenUnfocused = head[4] == "1"
	}
	if deadline > 0 {
		e.Deadline = time.Unix(deadline, 0)
	}
	for _, line := range lines[1:] {
		parts := strings.Split(line, "|")
		if len(parts) != 4 {
			return Event{}, fmt.Errorf("bridge_invalid_member")
		}
		c := Character{GUID: parts[0], Region: parts[1], Realm: parts[2], Name: parts[3]}
		// Older addons emitted NPC party members. Aura/cast indices already
		// count only Player rows; removing NPCs preserves those bindings.
		if framewire.IsNPCGUID(c.GUID) {
			continue
		}
		if !c.Valid() {
			return Event{}, fmt.Errorf("bridge_invalid_member")
		}
		e.Members = append(e.Members, c)
	}
	return e, nil
}

type BridgeSample struct {
	MeterCaptureSlot int            `json:"-"` // physical source slot + 1 for a shared view
	MeterSlot        int            `json:"-"`
	LocatorBase      BridgePosition `json:"-"`
	NativeMeter      *NativeMeter
	Overlays         *OverlayInput
	Source           string
	Position         BridgePosition
	KBB              []byte
	Arena            *ArenaSnapshot
	Queue            []byte
	Reload           []byte
	Sim              []byte
	Forever          bool
	Kick             *framewire.KickSnapshot
	Observations     *Observations
	Event            Event
	Error            string
	Frames           *framewire.Snapshot
	Window           uintptr
	WindowPID        uint32
}
