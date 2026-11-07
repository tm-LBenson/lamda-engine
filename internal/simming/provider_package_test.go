package simming

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writePackagedProvider(t *testing.T, dir string, worker []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "simc.exe")
	if err := os.WriteFile(path, worker, 0700); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(worker)
	networking := false
	manifest := packagedProviderManifest{Provider: "simc", Client: "retail", Version: "pinned-test",
		Source: "https://github.com/simulationcraft/simc/commit/test", Networking: &networking,
		Files: map[string]string{"simc.exe": hex.EncodeToString(sum[:])}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPackagedProviderIsCleanInstallFallback(t *testing.T) {
	state, packaged := t.TempDir(), t.TempDir()
	path := writePackagedProvider(t, packaged, []byte("fixture worker"))
	p := LoadProvider(state, packaged)
	if p.Config.Path != path || p.Config.Version != "pinned-test" || len(p.Config.Builds) != 0 || len(p.Config.Specs) != 0 {
		t.Fatalf("bundled worker not loaded without granting review: %+v", p.Config)
	}
	if capability := p.Capability(nil, time.Now()); !capability.Available || capability.Validation != "unreviewed" {
		t.Fatalf("bundled worker unavailable: %+v", capability)
	}
}

func TestRegisteredProviderOverridesPackage(t *testing.T) {
	state, packaged := t.TempDir(), t.TempDir()
	writePackagedProvider(t, packaged, []byte("bundled"))
	registered := filepath.Join(state, "custom.exe")
	if err := os.WriteFile(registered, []byte("custom"), 0700); err != nil {
		t.Fatal(err)
	}
	config := ProviderConfig{Path: registered, Version: "custom", SHA256: string(make([]byte, 64)), Builds: []string{"reviewed"}, Specs: []int{64}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(state, "simc-provider.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	p := LoadProvider(state, packaged)
	if p.Config.Path != registered || p.Config.Version != "custom" {
		t.Fatalf("explicit registration was replaced: %+v", p.Config)
	}
}

func TestDamagedPackagedProviderIsRejected(t *testing.T) {
	for name, damage := range map[string]func(string){
		"missing worker": func(dir string) { _ = os.Remove(filepath.Join(dir, "simc.exe")) },
		"bad hash": func(dir string) {
			var manifest packagedProviderManifest
			data, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
			_ = json.Unmarshal(data, &manifest)
			manifest.Files["simc.exe"] = "bad"
			data, _ = json.Marshal(manifest)
			_ = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600)
		},
		"changed worker": func(dir string) { _ = os.WriteFile(filepath.Join(dir, "simc.exe"), []byte("changed"), 0700) },
		"networking build": func(dir string) {
			var manifest packagedProviderManifest
			data, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
			_ = json.Unmarshal(data, &manifest)
			networking := true
			manifest.Networking = &networking
			data, _ = json.Marshal(manifest)
			_ = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writePackagedProvider(t, dir, []byte("fixture"))
			damage(dir)
			if p := LoadProvider(t.TempDir(), dir); p.Config.Path != "" {
				t.Fatalf("damaged package loaded: %+v", p.Config)
			}
		})
	}
}

func TestRealPackagedProviderArtifact(t *testing.T) {
	dir := os.Getenv("LAMDA_PACKAGED_SIMC_TEST")
	if dir == "" {
		t.Skip("set LAMDA_PACKAGED_SIMC_TEST to verify a built Retail provider package")
	}
	p := LoadProvider(t.TempDir(), dir)
	if p.Config.Path == "" || p.Config.Version == "" || p.Config.SHA256 == "" {
		t.Fatalf("built provider package did not load: %+v", p.Config)
	}
	if capability := p.Capability(nil, time.Now()); !capability.Available || capability.Validation != "unreviewed" {
		t.Fatalf("built provider package unavailable: %+v", capability)
	}
}
