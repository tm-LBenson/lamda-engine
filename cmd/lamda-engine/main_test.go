package main

import (
	"bytes"
	"encoding/json"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseSelection(t *testing.T) {
	for _, tag := range []string{"v0.0.9", "v0.1.0", "v0.2.0-beta.1", "v0.2.0^{}", "v0.2.0junk"} {
		if newer(tag, "0.1.0") {
			t.Fatal(tag)
		}
	}
	if got := newestTag("abc refs/tags/v0.2.0\ndef refs/tags/v0.3.0-beta.1\nghi refs/tags/v0.10.0\njkl refs/tags/v0.9.0", "0.1.0"); got != "v0.10.0" {
		t.Fatal(got)
	}
}

func TestPreviewDoesNotCreateObservations(t *testing.T) {
	m := engine.NewModel()
	c := engine.DefaultConfig()
	c.Preview = true
	rows := displayRows(m, c, time.Now())
	if len(rows) != 3 || len(m.Rows) != 0 || len(m.Seen) != 0 || len(m.Snapshot(time.Now()).Cooldowns) != 0 {
		t.Fatal("preview polluted observations")
	}
	c.Preview = false
	if len(displayRows(m, c, time.Now())) != 0 {
		t.Fatal("preview survived disable")
	}
}

func TestNativeDisplayKeepsSelfInferenceAndAuditReportsSimulatedTime(t *testing.T) {
	line := `9/13/2026 01:35:24.767-4  SPELL_CAST_SUCCESS,Player-1-TEST,"Test-Realm",0x511,0x0,0000000000000000,nil,0x80000000,0x0,55342,"Mirror Image",0x40`
	c, _ := engine.Parse(line)
	m := engine.NewModel()
	cfg := engine.DefaultConfig()
	cfg.Enabled = false
	if cfg.TrackingEnabled {
		m.Ingest(line, c.At, cfg.Companions)
	}
	if len(displayRows(m, cfg, c.At)) != 0 || len(m.Snapshot(c.At).Cooldowns) != 1 || !m.Snapshot(c.At).Cooldowns[0].Self {
		t.Fatal("hidden display stopped inference")
	}
	path := filepath.Join(t.TempDir(), "replay.txt")
	if err := os.WriteFile(path, []byte(line+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runAudit(path, 15*time.Second, &output); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Mode  string  `json:"mode"`
		Delay float64 `json:"simulatedDeliverySeconds"`
		engine.InferenceSnapshot
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Mode != "replay" || report.Delay != 15 || report.Diagnostics.Gaps["missing_build"].Count != 1 || report.Cooldowns[0].DeliverySeconds != 15 {
		t.Fatal(report)
	}
}

func TestDisplayDisableAndCompanionFilter(t *testing.T) {
	m := engine.NewModel()
	now := time.Now()
	m.Rows["player"] = engine.Row{GUID: "player", Player: "Teammate", Ends: float64(now.Unix()) + 30}
	m.Rows["companion"] = engine.Row{GUID: "companion", Companion: true, Ends: float64(now.Unix()) + 30}
	c := engine.DefaultConfig()
	c.Companions = false
	if rows := displayRows(m, c, now); len(rows) != 1 || rows[0].Companion {
		t.Fatal("disabled companion still shown", rows)
	}
	c.Enabled = false
	c.Preview = true
	if rows := displayRows(m, c, now); len(rows) != 0 {
		t.Fatal("disabled module still shows observations or preview", rows)
	}
	if len(m.Rows) != 2 {
		t.Fatal("display filters changed observations")
	}
}
