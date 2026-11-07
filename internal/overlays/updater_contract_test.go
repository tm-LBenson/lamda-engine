package overlays

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise the actual publisher against the updater's independent data grammar.
// A missed field here otherwise causes an upgrade to replace live capture data.
func TestPublishedCaptureDataSurvivesUpdaterValidation(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	validatorDir := ""
	for {
		for _, name := range []string{"Tools", "tools"} {
			candidate := filepath.Join(root, name)
			if info, err := os.Stat(filepath.Join(candidate, "sim_results_runtime.py")); err == nil && info.Mode().IsRegular() {
				validatorDir = candidate
				break
			}
		}
		if validatorDir != "" {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not locate Tools/sim_results_runtime.py in source tree")
		}
		root = parent
	}
	roster := rule()
	roster.Roots[0].Children = nil
	for _, scope := range []string{"party", "group", "raid", "boss", "arena", "nameplates"} {
		roster.Roots[0].Children = append(roster.Roots[0].Children, Node{ID: "each-" + scope, Type: "each", Props: map[string]any{"scope": scope}})
	}
	group := scopedRule("group")
	group.Roots[0].Children[0].Children[0].Props = map[string]any{"condition": "casting", "unit": "current", "check": "active"}
	group.Style.Anchor = &AnchorStyle{Kind: "unit", Target: "current", Point: "center", SelfPoint: "center"}
	cases := map[string][]Rule{"empty": {}, "roster": {roster}, "group-casts": {group}, "auras": captureRules(64, "aura"), "timers": captureRules(90, "timer"), "measurements": captureRules(64, "measurement"), "anchors": captureRules(64, "anchor")}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			s, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.dir, "OverlaysData.lua")
			if err = os.WriteFile(path, []byte(Marker+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			s.SetGameFile(path)
			if err = s.Save(0, rules); err != nil {
				t.Fatal(err)
			}
			if s.publishError != "" {
				t.Fatal(s.publishError)
			}
			cmd := exec.Command(python, "-c", "import sys;sys.path.insert(0,sys.argv[1]);from pathlib import Path;from sim_results_runtime import validate_overlays;validate_overlays(Path(sys.argv[2]).read_bytes(),'retail')", validatorDir, path)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("updater rejected published data: %v\n%s", err, out)
			}
		})
	}
}
