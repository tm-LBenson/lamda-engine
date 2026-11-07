package addonstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func testStore(t *testing.T, legacy bool) (*Store, string, string) {
	t.Helper()
	root := t.TempDir()
	addons := filepath.Join(root, "AddOns")
	catalogDir := filepath.Join(root, "catalog")
	stateDir := filepath.Join(root, "state")
	moduleFile := "lamdaUI/Modules/Test.lua"
	moduleFileAfter := "lamdaUI/Modules/TestAfterUI.lua"
	supportFile := "LamdaFrames/LamdaFrames.toc"
	mutableFile := "lamdaUI/TestData.lua"
	payload := []byte("local _,LUI=...\nLUI.Test=true\n")
	payloadAfter := []byte("local _,LUI=...\nLUI.TestAfterUI=true\n")
	supportPayload := []byte("## Interface: 120100\n## Title: Test support\n")
	mutablePayload := []byte("LamdaTestData={}\n")
	packageFile := filepath.Join(catalogDir, "test", "package", filepath.FromSlash(moduleFile))
	if err := os.MkdirAll(filepath.Dir(packageFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packageFile, payload, 0644); err != nil {
		t.Fatal(err)
	}
	packageFileAfter := filepath.Join(catalogDir, "test", "package", filepath.FromSlash(moduleFileAfter))
	if err := os.WriteFile(packageFileAfter, payloadAfter, 0644); err != nil {
		t.Fatal(err)
	}
	supportPackage := filepath.Join(catalogDir, "test", "package", filepath.FromSlash(supportFile))
	if err := os.MkdirAll(filepath.Dir(supportPackage), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(supportPackage, supportPayload, 0644); err != nil {
		t.Fatal(err)
	}
	mutablePackage := filepath.Join(catalogDir, "test", "package", filepath.FromSlash(mutableFile))
	if err := os.WriteFile(mutablePackage, mutablePayload, 0644); err != nil {
		t.Fatal(err)
	}
	summary := moduleSummary{ID: "test", Name: "LamdaTest", Summary: "Test module.", Version: "1.0.0-retail.1", Client: "retail"}
	writeTestJSON(t, filepath.Join(catalogDir, "catalog.json"), catalog{Product: "LamdaUI", Schema: 1, Client: "retail", AddonVersion: "1.0.0-retail.1",
		Headers: []string{"## Interface: 120100", "## Title: LamdaUI"},
		Order: []orderEntry{{Path: "Core.lua", Module: "core"}, {Path: "Modules/Test.lua", Module: "test"},
			{Path: "UI.lua", Module: "core"}, {Path: "Modules/TestAfterUI.lua", Module: "test"}},
		Modules: []moduleSummary{summary}})
	writeTestJSON(t, filepath.Join(catalogDir, "test", "manifest.json"), manifest{moduleSummary: summary, Schema: 1,
		Files: map[string]string{moduleFile: hashBytes(payload), moduleFileAfter: hashBytes(payloadAfter),
			supportFile: hashBytes(supportPayload), mutableFile: hashBytes(mutablePayload)},
		Entries: []string{"Modules/Test.lua", "Modules/TestAfterUI.lua"}, MutableFiles: []string{mutableFile}})
	if err := os.MkdirAll(filepath.Join(addons, "lamdaUI", "Modules"), 0755); err != nil {
		t.Fatal(err)
	}
	toc := "## Interface: 120100\n## Title: LamdaUI\n\nCore.lua\nUI.lua\n"
	if legacy {
		toc = "## Interface: 120100\n## Title: LamdaUI\n\nCore.lua\nModules\\Test.lua\nUI.lua\n"
		if err := os.WriteFile(filepath.Join(addons, filepath.FromSlash(moduleFile)), payload, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(addons, filepath.FromSlash(moduleFileAfter)), payloadAfter, 0644); err != nil {
			t.Fatal(err)
		}
		supportTarget := filepath.Join(addons, filepath.FromSlash(supportFile))
		if err := os.MkdirAll(filepath.Dir(supportTarget), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(supportTarget, supportPayload, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(addons, filepath.FromSlash(mutableFile)), []byte("LamdaTestData={saved=true}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	} else {
		toc = "## Interface: 120100\n## Title: LamdaUI\n## X-Lamda-Modules: \n\nCore.lua\nUI.lua\n"
	}
	if err := os.WriteFile(filepath.Join(addons, "lamdaUI", "lamdaUI.toc"), []byte(toc), 0644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenPaths(stateDir, addons, catalogDir, "retail")
	if err != nil {
		t.Fatal(err)
	}
	return store, addons, moduleFile
}

func TestLegacyModulesMigrateAndUninstallReinstall(t *testing.T) {
	store, addons, moduleFile := testStore(t, true)
	if !store.Installed("test") || store.Installed("missing") {
		t.Fatal("installed module lookup did not follow inventory")
	}
	if state := store.State(); len(state.Modules) != 1 || !state.Modules[0].Installed || state.Modules[0].Status != "installed" {
		t.Fatalf("legacy module not migrated: %+v", state)
	}
	result, err := store.Apply("test", "uninstall")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reload || result.Backup == "" {
		t.Fatalf("missing operation receipt: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(addons, filepath.FromSlash(moduleFile))); !os.IsNotExist(err) {
		t.Fatal("module file was not removed")
	}
	if _, err := os.Stat(filepath.Join(addons, "LamdaFrames")); !os.IsNotExist(err) {
		t.Fatal("empty support addon folder was not removed")
	}
	toc, _ := os.ReadFile(filepath.Join(addons, "lamdaUI", "lamdaUI.toc"))
	if !strings.Contains(string(toc), "## X-Lamda-Modules: \n") || strings.Contains(string(toc), "Modules\\Test.lua") {
		t.Fatalf("uninstall toc: %s", toc)
	}
	if _, err = store.Apply("test", "install"); err != nil {
		t.Fatal(err)
	}
	if !store.Installed("test") {
		t.Fatal("reinstalled module was not reported installed")
	}
	toc, _ = os.ReadFile(filepath.Join(addons, "lamdaUI", "lamdaUI.toc"))
	if !strings.Contains(string(toc), "## X-Lamda-Modules: test") || !strings.Contains(string(toc), "Modules\\Test.lua") {
		t.Fatalf("install toc: %s", toc)
	}
	if !(strings.Index(string(toc), "Modules\\Test.lua") < strings.Index(string(toc), "UI.lua") &&
		strings.Index(string(toc), "UI.lua") < strings.Index(string(toc), "Modules\\TestAfterUI.lua")) {
		t.Fatalf("module entries lost their catalog load positions: %s", toc)
	}
}

func TestChangedInstalledModuleOffersAndAppliesUpdate(t *testing.T) {
	store, addons, moduleFile := testStore(t, false)
	if _, err := store.Apply("test", "install"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(addons, filepath.FromSlash(moduleFile))
	if err := os.WriteFile(path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if state := store.State(); state.Modules[0].Status != "update" {
		t.Fatalf("changed module not detected: %+v", state)
	}
	if _, err := store.Apply("test", "update"); err != nil {
		t.Fatal(err)
	}
	if state := store.State(); state.Modules[0].Status != "installed" {
		t.Fatalf("module not repaired: %+v", state)
	}
}

func TestMutableModuleDataDoesNotOfferUpdateOrGetOverwritten(t *testing.T) {
	store, addons, _ := testStore(t, false)
	if _, err := store.Apply("test", "install"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(addons, "lamdaUI", "TestData.lua")
	custom := []byte("LamdaTestData={saved=true}\n")
	if err := os.WriteFile(path, custom, 0644); err != nil {
		t.Fatal(err)
	}
	if state := store.State(); state.Modules[0].Status != "installed" {
		t.Fatalf("mutable data offered a code update: %+v", state)
	}
	if _, err := store.Apply("test", "update"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(custom) {
		t.Fatalf("mutable module data was replaced: %q %v", data, err)
	}
}

func TestUpdateAndUninstallRemoveFilesOwnedByPreviousVersion(t *testing.T) {
	store, addons, moduleFile := testStore(t, false)
	if _, err := store.Apply("test", "install"); err != nil {
		t.Fatal(err)
	}
	oldFile := "lamdaUI/Modules/OldTest.lua"
	oldPath := filepath.Join(addons, filepath.FromSlash(oldFile))
	if err := os.WriteFile(oldPath, []byte("old version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	record := store.inventory.Installed["test"]
	record.Files = append(record.Files, oldFile)
	record.Version = "0.9.0-retail.1"
	store.inventory.Installed["test"] = record
	if _, err := store.Apply("test", "update"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("file removed from the new module version was left installed")
	}
	if _, err := os.Stat(filepath.Join(addons, filepath.FromSlash(moduleFile))); err != nil {
		t.Fatal("current module file missing after update")
	}

	oldFile = "lamdaUI/Modules/RemovedBeforeUninstall.lua"
	oldPath = filepath.Join(addons, filepath.FromSlash(oldFile))
	if err := os.WriteFile(oldPath, []byte("previous version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	record = store.inventory.Installed["test"]
	record.Files = append(record.Files, oldFile)
	store.inventory.Installed["test"] = record
	if _, err := store.Apply("test", "uninstall"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("previously owned file was left behind on uninstall")
	}
}

func TestRejectsEscapingModulePackage(t *testing.T) {
	store, _, _ := testStore(t, false)
	item := store.manifests["test"]
	item.Files = map[string]string{"../outside.lua": strings.Repeat("0", 64)}
	writeTestJSON(t, filepath.Join(store.catalogDir, "test", "manifest.json"), item)
	if _, err := OpenPaths(t.TempDir(), store.addonsDir, store.catalogDir, "retail"); err == nil {
		t.Fatal("escaping module package accepted")
	}
}

func TestRefusesLinkedAddonFolder(t *testing.T) {
	store, addons, _ := testStore(t, false)
	modules := filepath.Join(addons, "lamdaUI", "Modules")
	if err := os.RemoveAll(modules); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), modules); err != nil {
		t.Skip(err)
	}
	if _, err := store.Apply("test", "install"); err == nil || !strings.Contains(err.Error(), "linked") {
		t.Fatalf("linked addon folder accepted: %v", err)
	}
}
