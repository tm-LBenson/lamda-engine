package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

func writeConfigFixture(t *testing.T, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "lamdaUI.lua")
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPublicBuildFallbackDoesNotOverrideFreshLogs(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	old := c.At.Add(-48 * time.Hour)
	p := lookup.Profile{Character: lookup.Character{GUID: c.GUID, Name: "Friend", Realm: "Realm", Region: "us"}, FetchedAt: c.At, SourceUpdatedAt: &old, Source: "raider_io", SpecID: 1468, Revision: "public", Talents: []lookup.Talent{{NodeID: 1, EntryID: 2, SpellID: 3, Rank: 1}}}
	m.ObservePublicBuild(p)
	m.Observe(c, c.At)
	e := m.Snapshot(c.At).Cooldowns[0]
	if e.BuildSource != "raider_io" || e.BuildRevision != "public" || m.Diagnostics.Gaps["public_build_stale"].Count != 1 || m.Diagnostics.Gaps["missing_build"].Count != 0 {
		t.Fatal(e, m.Diagnostics)
	}
	m.Reset()
	if len(m.PublicBuilds) != 1 {
		t.Fatal("session reset dropped prewarmed public build")
	}
	m.Ingest(versionLine(c.At), c.At, true)
	m.Ingest(buildLine(c.At, "[(1,2,1)]"), c.At, true)
	c.At = c.At.Add(time.Second)
	m.Observe(c, c.At)
	e = m.Snapshot(c.At).Cooldowns[0]
	if e.BuildSource != "combat_log" || e.BuildRevision == "public" {
		t.Fatal("lookup overrode log evidence", e)
	}
	regressed := old.Add(-time.Hour)
	p.SourceUpdatedAt = &regressed
	p.FetchedAt = c.At
	m.ObservePublicBuild(p)
	if !m.PublicBuilds[c.GUID].SourceUpdatedAt.Equal(old) || m.Diagnostics.Gaps["public_build_regressed"].Count != 1 {
		t.Fatal("older public snapshot replaced newer one")
	}
}
func TestLookupConfigurationAndModuleMaster(t *testing.T) {
	base := `LamdaEngineDB = {
["schema"] = 1,
["cdEnabled"] = true,
["engineFrameEstimates"] = true,
["nativeFrames"] = true,
["checkUpdates"] = false,
["notifyUpdates"] = false,
["checkDays"] = 1,
["overlayX"] = 60,
["overlayY"] = 240,
["overlayScale"] = 1,
["publicLookups"] = false,
["lookupBridge"] = true,
["lookupInterval"] = 1,
}`
	// Uses the data-only parser; fractional, absent and invalid values differ.
	for _, value := range []string{"1", "5", "0", "1.5", "nil"} {
		data := strings.Replace(base, `["lookupInterval"] = 1`, `["lookupInterval"] = `+value, 1)
		path := writeConfigFixture(t, data)
		cfg, err := ReadConfig(path)
		if value == "1" || value == "5" {
			if err != nil || cfg.PublicLookups || !cfg.LookupBridge || !cfg.TrackingEnabled || cfg.Enabled {
				t.Fatal(cfg, err)
			}
		} else if err == nil {
			t.Fatal("invalid interval accepted", value)
		}
	}
	for _, value := range []string{"0", "271", "16000", "-1", "16001", "1.5", "nil"} {
		data := strings.Replace(base, `["lookupInterval"] = 1`, `["lookupBridgeX"] = `+value+`, ["lookupBridgeY"] = `+value+`, ["lookupInterval"] = 1`, 1)
		cfg, err := ReadConfig(writeConfigFixture(t, data))
		if value == "0" || value == "271" || value == "16000" {
			if err != nil || cfg.LookupBridgeX != cfg.LookupBridgeY || (value == "271" && cfg.LookupBridgeX != 271) {
				t.Fatal(cfg, err)
			}
		} else if err == nil {
			t.Fatal("invalid position accepted", value)
		}
	}
	cfg, err := ReadConfig(writeConfigFixture(t, base))
	if err != nil || cfg.LookupBridgeX != 0 || cfg.LookupBridgeY != 0 {
		t.Fatal("legacy configuration must retain top-left capture", cfg, err)
	}
}
