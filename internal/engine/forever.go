package engine

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// Selected once at process startup, before any workers. Retail stays the default.
var Forever bool
var foreverAliases = map[int]int{}
var foreverBuild string
var retailCatalog, retailInterrupts, retailCandidates = Catalog, InterruptCatalog, specializationDefensives

// Identity candidates, not evidence of a learned ability or a cooldown duration.
//
//go:embed forever_families.json
var foreverFamiliesJSON []byte

func ConfigureClient(client string) error {
	if client == "retail" {
		Forever = false
		Catalog, InterruptCatalog, specializationDefensives = retailCatalog, retailInterrupts, retailCandidates
		return nil
	}
	if client != "forever" {
		return fmt.Errorf("unsupported client %q", client)
	}
	var families struct{ Interrupts, Defensives map[int]string }
	if err := json.Unmarshal(foreverFamiliesJSON, &families); err != nil {
		return err
	}
	Forever = true
	Catalog, InterruptCatalog = map[int]Spell{}, map[int]Spell{}
	for id, name := range families.Defensives {
		Catalog[id] = Spell{Name: name}
	}
	for id, name := range families.Interrupts {
		InterruptCatalog[id] = Spell{Name: name}
	}
	foreverAliases = map[int]int{}
	foreverBuild = ""
	specializationDefensives = map[int][]int{}
	return nil
}

// Only metadata emitted by the Forever addon is accepted. The addon probes
// public static spell information; no live cooldown value is exported.
func (m *Model) ObserveForeverSpell(meta lookup.SpellMetadata) bool {
	if !Forever || !strings.HasPrefix(meta.Build, "1.60.") {
		return false
	}
	if foreverBuild != "" && foreverBuild != meta.Build {
		_ = ConfigureClient("forever")
		m.Reset()
	}
	foreverBuild = meta.Build
	target := Catalog
	if _, ok := InterruptCatalog[meta.Root]; ok {
		target = InterruptCatalog
	}
	old, ok := target[meta.Root]
	if !ok {
		return false
	}
	if old.Cooldown != float64(meta.Millis)/1000 {
		// A new baseline cannot retroactively turn old casts into ready timers.
		for key, e := range m.Evidence {
			if e.Spell == meta.Root {
				delete(m.Evidence, key)
				delete(m.Rows, key)
				delete(m.Seen, key)
			}
		}
	}
	target[meta.Root] = Spell{Name: meta.Name, Cooldown: float64(meta.Millis) / 1000}
	foreverAliases[meta.Spell] = meta.Root
	return true
}

func LimitForeverConfig(c Config) Config {
	if Forever {
		c.PublicLookups, c.Companions, c.Updates, c.Notify = false, false, false, false
	}
	return c
}

// The first beta login can precede the first modern SavedVariables write.
// Match the addon's shipped defaults until a valid saved profile is available.
func DefaultClientConfig() Config {
	c := DefaultConfig()
	if Forever {
		c = activateForeverConfig(c)
	}
	// No module runs before the user opts in and saves the addon settings.
	c.Enabled, c.DefensiveTracking, c.TrackingEnabled = false, false, false
	c.FrameEstimates, c.KickEnabled, c.ArenaTracking = false, false, false
	c.PublicLookups, c.LookupBridge = false, false
	return LimitForeverConfig(c)
}

// Mirror the addon's one-time activation in memory: /reload writes the old
// settings before the new Lua runs. Do not edit SavedVariables on disk.
func activateForeverConfig(c Config) Config {
	c.TrackingEnabled, c.LookupBridge, c.FrameEstimates = true, true, true
	c.FrameSpells, c.KickSpells = []int{}, []int{}
	for id := range Catalog {
		c.FrameSpells = append(c.FrameSpells, id)
	}
	for id := range InterruptCatalog {
		c.KickSpells = append(c.KickSpells, id)
	}
	sort.Ints(c.FrameSpells)
	sort.Ints(c.KickSpells)
	return c
}
