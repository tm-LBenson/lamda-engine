package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type pendingBuild struct {
	first    time.Time
	complete bool
	chunk    lookup.BuildChunk
	parts    map[int]string
}
type observationInput struct {
	screen screenInput
	meter  meterInput
	auras  auraInput
	seen   map[string]time.Time
	builds map[string]*pendingBuild
}

// Native samples arrive independently. A missing CD sample must not discard
// the two-read confirmation underway on Kick, and vice versa.
func (in *observationInput) pauseSource(source string) {
	if source == "" || source == "cd" {
		in.auras = auraInput{}
	}
	if source == "" || source == "kick" {
		in.screen = screenInput{}
		in.meter = meterInput{}
	}
}

func (in *observationInput) handle(m *engine.Model, sample lookup.BridgeSample, now time.Time) {
	if sample.Observations == nil || sample.Error != "" || now.Sub(sample.Event.At) > 3*time.Second || sample.Event.At.After(now.Add(addonClockTolerance)) {
		in.pauseSource(sample.Source)
		return
	}
	if engine.Forever != sample.Forever {
		return
	}
	if engine.Forever {
		for _, meta := range sample.Observations.Metadata {
			m.ObserveForeverSpell(meta)
		}
	}
	roster := rosterForCasts(&sample.Event, now)
	if roster == nil {
		return
	}
	if c := sample.Observations.Context; c != nil && !m.DisabledDefensives {
		m.ObservePartyContext(*c, now)
	}
	if sample.Event.Kind == "challenge_start" {
		m.BeginFreshKey(sample.Event.Deadline, now, roster)
	} else if sample.Event.Kind == "challenge_end" {
		m.EndFreshKey()
	}
	m.KeepKeyRoster(roster)
	m.TrackPartyRoster(roster, sample.Event.At)
	names := map[string]string{}
	for _, c := range sample.Event.Members {
		names[c.GUID] = c.Name
	}
	own := ""
	if sample.Frames != nil && now.Sub(sample.Frames.At) <= 2*time.Second && !sample.Frames.At.After(now.Add(addonClockTolerance)) {
		own = sample.Frames.Player
		roster[own] = true
		if names[own] == "" {
			names[own] = "You"
		}
	}
	if sample.Kick != nil && now.Sub(sample.Kick.At) <= 2*time.Second && !sample.Kick.At.After(now.Add(addonClockTolerance)) {
		own = sample.Kick.Player
		roster[own] = true
		if names[own] == "" {
			names[own] = "You"
		}
	}
	if in.seen == nil {
		in.seen = map[string]time.Time{}
		in.builds = map[string]*pendingBuild{}
	}
	for id, at := range in.seen {
		if now.Sub(at) > 20*time.Minute {
			delete(in.seen, id)
		}
	}
	for guid, b := range in.builds {
		if now.Sub(b.first) > 30*time.Second || !roster[guid] {
			delete(in.builds, guid)
		}
	}
	if d := sample.Observations.Diagnostics; d != nil {
		m.Diagnostics.AddonObservations = append([]uint64(nil), d...)
	}
	if status := sample.Observations.ScreenStatus; status != nil {
		copy := *status
		copy.At = now.Format(time.RFC3339Nano)
		m.Diagnostics.ScreenStatus = &copy
	}
	// Assemble builds before casts when both arrive in this sample.
	for _, part := range sample.Observations.Builds {
		if !roster[part.GUID] || part.At.After(now.Add(2*time.Second)) || (now.Sub(part.At) > 5*time.Minute && !strings.HasPrefix(part.ID, "Held:")) || now.Sub(part.At) > 7*24*time.Hour {
			continue
		}
		b := in.builds[part.GUID]
		if b == nil || b.chunk.ID != part.ID {
			if b != nil && part.At.Before(b.chunk.At) {
				continue
			}
			if b == nil && len(in.builds) >= 6 {
				continue
			}
			b = &pendingBuild{first: now, chunk: part, parts: map[int]string{}}
			in.builds[part.GUID] = b
		}
		if b.chunk.Total != part.Total || b.chunk.Spec != part.Spec || b.chunk.Game != part.Game || !b.chunk.At.Equal(part.At) {
			delete(in.builds, part.GUID)
			m.RecordGap("inspect_chunk_conflict", now, engine.Cast{GUID: part.GUID})
			continue
		}
		if data, exists := b.parts[part.Part]; exists && data != part.Data {
			delete(in.builds, part.GUID)
			continue
		}
		if b.complete {
			continue
		}
		b.parts[part.Part] = part.Data
		if len(b.parts) != part.Total {
			continue
		}
		build, err := assembleBuild(b, names[part.GUID])
		if err != nil {
			m.RecordGap("inspect_build_invalid", now, engine.Cast{GUID: part.GUID})
			continue
		}
		if strings.HasPrefix(part.ID, "Held:") {
			m.ObserveRetainedPartyBuild(build, now)
		} else {
			m.ObserveInspectedBuild(build, now)
		}
		b.complete = true
	}
	casts := append([]lookup.LiveCast{}, sample.Observations.Casts...)
	if sample.Source != "cd" {
		casts = append(casts, in.screen.casts(m, sample.Observations.Screen, now)...)
	}
	for _, cast := range casts {
		if !roster[cast.GUID] {
			continue
		}
		key := cast.GUID + "/" + cast.ID
		if _, exists := in.seen[key]; exists {
			continue
		}
		if len(in.seen) >= 4096 {
			m.RecordGap("live_cast_dedup_full", now, engine.Cast{})
			continue
		}
		in.seen[key] = now
		path := "public_packet"
		if strings.HasPrefix(cast.ID, "Screen:") {
			path = "screen_capture"
			m.Diagnostics.ScreenConfirmed++
		}
		// The packet carries monotonic age. First receipt anchors the event once;
		// subsequent rendered copies cannot slide the countdown forward.
		at := now.Add(-time.Duration(cast.AgeMillis) * time.Millisecond)
		m.Observe(engine.Cast{At: at, GUID: cast.GUID, Name: names[cast.GUID], Spell: cast.Spell, Self: cast.GUID == own, Source: "unit_spellcast", LivePath: path, RosterMatched: true}, now)
	}
	if sample.Source != "kick" {
		in.auras.handle(m, sample, now, names, own)
	}
	if sample.Source != "cd" {
		in.meter.handle(m, sample.Observations.Screen, sample.Kick, now, roster, names, own)
	}
}

func assembleBuild(p *pendingBuild, name string) (engine.BuildSnapshot, error) {
	b := engine.BuildSnapshot{GUID: p.chunk.GUID, Player: name, At: p.chunk.At, Source: "inspect", SpecID: p.chunk.Spec, GameBuild: p.chunk.Game, SpellRanks: map[int]int{}, Missing: []string{"equipment", "pvp_talents"}}
	seenNodes := map[int]bool{}
	for i := 1; i <= p.chunk.Total; i++ {
		data, ok := p.parts[i]
		if !ok {
			return b, fmt.Errorf("missing chunk")
		}
		if data == "" {
			if p.chunk.Total != 1 {
				return b, fmt.Errorf("empty chunk")
			}
			continue
		}
		for _, entry := range strings.Split(data, ";") {
			cells := strings.Split(entry, ",")
			if len(cells) != 4 {
				return b, fmt.Errorf("invalid talent")
			}
			var v [4]int
			for j, s := range cells {
				n, err := strconv.Atoi(s)
				if err != nil || n <= 0 || n > 10000000 || strconv.Itoa(n) != s {
					return b, fmt.Errorf("invalid talent")
				}
				v[j] = n
			}
			if v[2] > 10 || seenNodes[v[0]] || len(seenNodes) >= 128 {
				return b, fmt.Errorf("invalid talent rank or duplicate")
			}
			seenNodes[v[0]] = true
			b.Talents = append(b.Talents, [3]int{v[0], v[1], v[2]})
			b.SpellRanks[v[3]] = max(b.SpellRanks[v[3]], v[2])
		}
	}
	return b, nil
}
