package addonstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var moduleID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
var hashValue = regexp.MustCompile(`^[0-9a-f]{64}$`)

type orderEntry struct {
	Path   string `json:"path"`
	Module string `json:"module"`
}

type moduleSummary struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Summary      string   `json:"summary"`
	Version      string   `json:"version"`
	Client       string   `json:"client"`
	Experimental bool     `json:"experimental,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

type manifest struct {
	moduleSummary
	Schema       int               `json:"schema"`
	Files        map[string]string `json:"files"`
	Entries      []string          `json:"entries"`
	MutableFiles []string          `json:"mutableFiles,omitempty"`
}

type catalog struct {
	Product      string              `json:"product"`
	Schema       int                 `json:"schema"`
	Client       string              `json:"client"`
	AddonVersion string              `json:"addonVersion"`
	Headers      []string            `json:"headers"`
	Order        []orderEntry        `json:"order"`
	Metadata     map[string][]string `json:"metadata"`
	Modules      []moduleSummary     `json:"modules"`
}

type inventory struct {
	Schema    int                        `json:"schema"`
	Installed map[string]installedModule `json:"installed"`
	UpdatedAt int64                      `json:"updatedAt"`
}

type installedModule struct {
	Version  string   `json:"version"`
	Files    []string `json:"files"`
	Entries  []string `json:"entries"`
	Metadata []string `json:"metadata,omitempty"`
}

type Module struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Summary          string   `json:"summary"`
	Version          string   `json:"version"`
	Installed        bool     `json:"installed"`
	InstalledVersion string   `json:"installedVersion,omitempty"`
	Status           string   `json:"status"`
	Experimental     bool     `json:"experimental,omitempty"`
	Dependencies     []string `json:"dependencies,omitempty"`
}

type State struct {
	Client       string   `json:"client"`
	AddonVersion string   `json:"addonVersion"`
	Modules      []Module `json:"modules"`
}

type Result struct {
	State  State  `json:"state"`
	Action string `json:"action"`
	Module string `json:"module"`
	Reload bool   `json:"reload"`
	Backup string `json:"backup,omitempty"`
}

type Store struct {
	mu         sync.Mutex
	client     string
	stateDir   string
	addonsDir  string
	catalogDir string
	catalog    catalog
	manifests  map[string]manifest
	inventory  inventory
}

func Open(stateDir, wow, client string) (*Store, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return OpenPaths(stateDir, filepath.Join(wow, "Interface", "AddOns"), filepath.Join(filepath.Dir(exe), "modules"), client)
}

func OpenPaths(stateDir, addonsDir, catalogDir, client string) (*Store, error) {
	if client != "retail" && client != "forever" {
		return nil, fmt.Errorf("unsupported game client")
	}
	for _, path := range []string{stateDir, addonsDir, catalogDir} {
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("module store paths must be absolute")
		}
	}
	s := &Store{client: client, stateDir: stateDir, addonsDir: addonsDir, catalogDir: catalogDir, manifests: map[string]manifest{}}
	if err := readJSON(filepath.Join(catalogDir, "catalog.json"), &s.catalog); err != nil {
		return nil, fmt.Errorf("read module catalog: %w", err)
	}
	if s.catalog.Product != "LamdaUI" || s.catalog.Schema != 1 || s.catalog.Client != client || s.catalog.AddonVersion == "" {
		return nil, fmt.Errorf("invalid module catalog")
	}
	for _, line := range s.catalog.Headers {
		if !strings.HasPrefix(line, "##") || strings.ContainsAny(line, "\r\n") || strings.HasPrefix(line, "## X-Lamda-Modules:") {
			return nil, fmt.Errorf("invalid module catalog header")
		}
	}
	seen := map[string]bool{}
	fileOwners := map[string]string{}
	for _, summary := range s.catalog.Modules {
		if !moduleID.MatchString(summary.ID) || seen[summary.ID] || summary.Client != client || summary.Version == "" || summary.Name == "" || summary.Summary == "" {
			return nil, fmt.Errorf("invalid module catalog entry")
		}
		seen[summary.ID] = true
		var item manifest
		if err := readJSON(filepath.Join(catalogDir, summary.ID, "manifest.json"), &item); err != nil {
			return nil, fmt.Errorf("read %s manifest: %w", summary.ID, err)
		}
		if item.Schema != 1 || item.ID != summary.ID || item.Name != summary.Name || item.Version != summary.Version || item.Client != client || len(item.Files) == 0 || len(item.Entries) == 0 {
			return nil, fmt.Errorf("invalid %s module package", summary.ID)
		}
		entryFiles := map[string]bool{}
		for name, expected := range item.Files {
			if _, err := safeRelative(name); err != nil || !hashValue.MatchString(expected) {
				return nil, fmt.Errorf("invalid %s module file", summary.ID)
			}
			key := strings.ToLower(name)
			if owner := fileOwners[key]; owner != "" {
				return nil, fmt.Errorf("module file is owned by both %s and %s", owner, summary.ID)
			}
			fileOwners[key] = summary.ID
		}
		for _, entry := range item.Entries {
			name, err := safeRelative(filepath.ToSlash(filepath.Join("lamdaUI", entry)))
			if err != nil || entryFiles[strings.ToLower(entry)] || item.Files[name] == "" {
				return nil, fmt.Errorf("invalid %s module entry", summary.ID)
			}
			entryFiles[strings.ToLower(entry)] = true
		}
		mutable := map[string]bool{}
		for _, name := range item.MutableFiles {
			if _, err := safeRelative(name); err != nil || item.Files[name] == "" || mutable[strings.ToLower(name)] {
				return nil, fmt.Errorf("invalid %s mutable file", summary.ID)
			}
			mutable[strings.ToLower(name)] = true
		}
		for _, dependency := range item.Dependencies {
			if !moduleID.MatchString(dependency) || dependency == item.ID {
				return nil, fmt.Errorf("invalid %s dependency", summary.ID)
			}
		}
		s.manifests[item.ID] = item
	}
	for id, item := range s.manifests {
		for _, dependency := range item.Dependencies {
			if _, ok := s.manifests[dependency]; !ok {
				return nil, fmt.Errorf("invalid %s dependency", id)
			}
		}
	}
	for id, lines := range s.catalog.Metadata {
		if !seen[id] {
			return nil, fmt.Errorf("invalid module metadata")
		}
		for _, line := range lines {
			if !strings.HasPrefix(line, "##") || strings.ContainsAny(line, "\r\n") || strings.HasPrefix(line, "## X-Lamda-Modules:") {
				return nil, fmt.Errorf("invalid module metadata")
			}
		}
	}
	orderedEntries := map[string]map[string]bool{}
	for id := range s.manifests {
		orderedEntries[id] = map[string]bool{}
	}
	for _, item := range s.catalog.Order {
		if _, err := safeRelative(filepath.ToSlash(filepath.Join("lamdaUI", item.Path))); err != nil || item.Module != "core" && !seen[item.Module] {
			return nil, fmt.Errorf("invalid module load order")
		}
		name := filepath.ToSlash(filepath.Join("lamdaUI", item.Path))
		if item.Module == "core" {
			if owner := fileOwners[strings.ToLower(name)]; owner != "" {
				return nil, fmt.Errorf("%s module file collides with LamdaUI core", owner)
			}
		} else {
			key := strings.ToLower(item.Path)
			if orderedEntries[item.Module][key] || s.manifests[item.Module].Files[name] == "" {
				return nil, fmt.Errorf("invalid module load order")
			}
			orderedEntries[item.Module][key] = true
		}
	}
	for id, item := range s.manifests {
		if len(item.Entries) != len(orderedEntries[id]) {
			return nil, fmt.Errorf("invalid %s module load order", id)
		}
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, err
	}
	if err := s.loadInventory(); err != nil {
		return nil, err
	}
	return s, nil
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func safeRelative(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("invalid package path")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean != name || clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(name)) {
		return "", fmt.Errorf("invalid package path")
	}
	first := strings.Split(clean, "/")[0]
	if first != "lamdaUI" && first != "LamdaFrames" && first != "LamdaMover" {
		return "", fmt.Errorf("package path leaves LamdaUI")
	}
	return clean, nil
}

func hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Store) inventoryPath() string { return filepath.Join(s.stateDir, "installed.json") }

func (s *Store) loadInventory() error {
	path := s.inventoryPath()
	if err := readJSON(path, &s.inventory); err == nil {
		if s.inventory.Schema != 1 || s.inventory.Installed == nil {
			return fmt.Errorf("invalid installed module record")
		}
		for id, record := range s.inventory.Installed {
			item, ok := s.manifests[id]
			if !ok {
				delete(s.inventory.Installed, id)
				continue
			}
			if record.Version == "" || len(record.Files) == 0 || len(record.Entries) == 0 {
				return fmt.Errorf("invalid installed module record")
			}
			for _, name := range record.Files {
				if _, err := safeRelative(name); err != nil {
					return fmt.Errorf("invalid installed module path")
				}
			}
			for _, entry := range record.Entries {
				if _, err := safeRelative(filepath.ToSlash(filepath.Join("lamdaUI", entry))); err != nil {
					return fmt.Errorf("invalid installed module entry")
				}
			}
			if len(record.Metadata) == 0 && len(s.catalog.Metadata[id]) > 0 {
				record.Metadata = s.installedRecord(item).Metadata
				s.inventory.Installed[id] = record
			}
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read installed modules: %w", err)
	}
	s.inventory = inventory{Schema: 1, Installed: map[string]installedModule{}}
	toc, _ := os.ReadFile(filepath.Join(s.addonsDir, "lamdaUI", "lamdaUI.toc"))
	selected, marked := parseMarker(string(toc))
	if marked {
		for _, id := range selected {
			if item, ok := s.manifests[id]; ok {
				s.inventory.Installed[id] = s.installedRecord(item)
			}
		}
	} else {
		// The pre-store LamdaUI package shipped every module. Presence-based
		// migration keeps those users' exact feature set and SavedVariables.
		for id, item := range s.manifests {
			present := true
			for name := range item.Files {
				if _, err := os.Stat(filepath.Join(s.addonsDir, filepath.FromSlash(name))); err != nil {
					present = false
					break
				}
			}
			if present {
				s.inventory.Installed[id] = s.installedRecord(item)
			}
		}
	}
	return s.saveInventory()
}

func (s *Store) installedRecord(item manifest) installedModule {
	files := make([]string, 0, len(item.Files))
	for name := range item.Files {
		files = append(files, name)
	}
	sort.Strings(files)
	metadata := append([]string(nil), s.catalog.Metadata[item.ID]...)
	for i := range metadata {
		metadata[i] = strings.ReplaceAll(metadata[i], "{client}", s.client)
	}
	return installedModule{Version: detectedVersion(s.addonsDir, item), Files: files,
		Entries: append([]string(nil), item.Entries...), Metadata: metadata}
}

func parseMarker(toc string) ([]string, bool) {
	for _, line := range strings.Split(strings.ReplaceAll(toc, "\r", ""), "\n") {
		if strings.HasPrefix(line, "## X-Lamda-Modules:") {
			var result []string
			for _, id := range strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "## X-Lamda-Modules:")), ",") {
				if id = strings.TrimSpace(id); id != "" {
					result = append(result, id)
				}
			}
			return result, true
		}
	}
	return nil, false
}

func detectedVersion(addons string, item manifest) string {
	mutable := map[string]bool{}
	for _, name := range item.MutableFiles {
		mutable[name] = true
	}
	for name, expected := range item.Files {
		if mutable[name] {
			continue
		}
		if hashFile(filepath.Join(addons, filepath.FromSlash(name))) != expected {
			return "legacy"
		}
	}
	return item.Version
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".lamda-module-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err = temp.Chmod(mode); err == nil {
		_, err = temp.Write(data)
	}
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (s *Store) saveInventory() error {
	s.inventory.UpdatedAt = time.Now().Unix()
	data, err := json.MarshalIndent(s.inventory, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.inventoryPath(), append(data, '\n'), 0600)
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked()
}

// Installed reports the user's selected module set. It intentionally follows
// inventory rather than package hashes: a changed module is still installed
// until the user updates or uninstalls it.
func (s *Store) Installed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.inventory.Installed[id]
	return ok
}

func (s *Store) stateLocked() State {
	result := State{Client: s.client, AddonVersion: s.catalog.AddonVersion}
	for _, summary := range s.catalog.Modules {
		record, installed := s.inventory.Installed[summary.ID]
		status := "available"
		if installed {
			actual := detectedVersion(s.addonsDir, s.manifests[summary.ID])
			if record.Version != summary.Version || actual != summary.Version {
				status = "update"
			} else {
				status = "installed"
			}
		}
		result.Modules = append(result.Modules, Module{ID: summary.ID, Name: summary.Name, Summary: summary.Summary,
			Version: summary.Version, Installed: installed, InstalledVersion: record.Version, Status: status,
			Experimental: summary.Experimental, Dependencies: append([]string(nil), summary.Dependencies...)})
	}
	return result
}

func (s *Store) Apply(id, action string) (Result, error) {
	releaseControl, err := acquireControl()
	if err != nil {
		return Result{}, err
	}
	defer releaseControl()
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.manifests[id]
	if !ok {
		return Result{}, fmt.Errorf("choose an addon from Browse Addons")
	}
	if action != "install" && action != "update" && action != "uninstall" {
		return Result{}, fmt.Errorf("unsupported addon action")
	}
	_, installed := s.inventory.Installed[id]
	previousRecord := s.inventory.Installed[id]
	if action == "uninstall" && !installed {
		return Result{State: s.stateLocked(), Action: action, Module: id}, nil
	}
	if action != "uninstall" {
		for _, dependency := range item.Dependencies {
			if _, ok := s.inventory.Installed[dependency]; !ok {
				return Result{}, fmt.Errorf("install %s first", s.manifests[dependency].Name)
			}
		}
		for name, expected := range item.Files {
			source := filepath.Join(s.catalogDir, id, "package", filepath.FromSlash(name))
			if hashFile(source) != expected {
				return Result{}, fmt.Errorf("%s package did not pass verification", item.Name)
			}
		}
	} else {
		for other := range s.inventory.Installed {
			if other == id {
				continue
			}
			for _, dependency := range s.manifests[other].Dependencies {
				if dependency == id {
					return Result{}, fmt.Errorf("uninstall %s first", s.manifests[other].Name)
				}
			}
		}
	}

	stamp := time.Now().Format("20060102-150405.000000000") + "-" + id
	backup := filepath.Join(s.stateDir, "backups", stamp)
	if err := os.MkdirAll(filepath.Join(backup, "files"), 0700); err != nil {
		return Result{}, err
	}
	previous := make(map[string][]byte)
	previousMode := make(map[string]os.FileMode)
	pathSet := make(map[string]bool, len(item.Files)+len(previousRecord.Files)+1)
	for name := range item.Files {
		pathSet[name] = true
	}
	for _, name := range previousRecord.Files {
		pathSet[name] = true
	}
	pathSet["lamdaUI/lamdaUI.toc"] = true
	paths := make([]string, 0, len(pathSet))
	for name := range pathSet {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		target, err := safeTarget(s.addonsDir, name)
		if err != nil {
			return Result{}, err
		}
		if info, err := os.Lstat(target); err == nil {
			if !info.Mode().IsRegular() {
				return Result{}, fmt.Errorf("refusing non-file addon path %s", name)
			}
			data, err := os.ReadFile(target)
			if err != nil {
				return Result{}, err
			}
			previous[name], previousMode[name] = data, info.Mode().Perm()
			if err := atomicWrite(filepath.Join(backup, "files", filepath.FromSlash(name)), data, 0600); err != nil {
				return Result{}, err
			}
		}
	}
	oldInventory, _ := os.ReadFile(s.inventoryPath())
	if len(oldInventory) > 0 {
		if err := atomicWrite(filepath.Join(backup, "installed-before.json"), oldInventory, 0600); err != nil {
			return Result{}, err
		}
	}
	rollback := func() {
		for _, name := range paths {
			target, targetErr := safeTarget(s.addonsDir, name)
			if targetErr != nil {
				continue
			}
			if data, ok := previous[name]; ok {
				_ = atomicWrite(target, data, previousMode[name])
			} else {
				_ = os.Remove(target)
			}
		}
		if len(oldInventory) > 0 {
			_ = atomicWrite(s.inventoryPath(), oldInventory, 0600)
		}
	}

	if action == "uninstall" {
		for _, name := range previousRecord.Files {
			target, targetErr := safeTarget(s.addonsDir, name)
			if targetErr != nil {
				rollback()
				return Result{}, targetErr
			}
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollback()
				return Result{}, err
			}
			removeEmptyParents(s.addonsDir, target)
		}
		delete(s.inventory.Installed, id)
	} else {
		mutable := map[string]bool{}
		for _, name := range item.MutableFiles {
			mutable[name] = true
		}
		for _, name := range previousRecord.Files {
			if _, retained := item.Files[name]; retained {
				continue
			}
			target, targetErr := safeTarget(s.addonsDir, name)
			if targetErr != nil {
				rollback()
				return Result{}, targetErr
			}
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollback()
				return Result{}, err
			}
			removeEmptyParents(s.addonsDir, target)
		}
		for name, expected := range item.Files {
			source := filepath.Join(s.catalogDir, id, "package", filepath.FromSlash(name))
			target, targetErr := safeTarget(s.addonsDir, name)
			if targetErr != nil {
				rollback()
				return Result{}, targetErr
			}
			data, err := os.ReadFile(source)
			if err != nil || hashBytes(data) != expected {
				rollback()
				return Result{}, fmt.Errorf("%s package changed during installation", item.Name)
			}
			if mutable[name] && hashFile(target) != "" {
				continue
			}
			if err := atomicWrite(target, data, 0644); err != nil {
				rollback()
				return Result{}, err
			}
		}
		record := s.installedRecord(item)
		record.Version = item.Version
		s.inventory.Installed[id] = record
	}
	toc := s.renderTOC()
	tocTarget, err := safeTarget(s.addonsDir, "lamdaUI/lamdaUI.toc")
	if err != nil {
		rollback()
		return Result{}, err
	}
	if err := atomicWrite(tocTarget, []byte(toc), 0644); err != nil {
		rollback()
		return Result{}, err
	}
	if err := s.saveInventory(); err != nil {
		rollback()
		return Result{}, err
	}
	return Result{State: s.stateLocked(), Action: action, Module: id, Reload: true, Backup: backup}, nil
}

func removeEmptyParents(root, file string) {
	root, err := filepath.Abs(root)
	if err != nil {
		return
	}
	for current := filepath.Dir(file); current != root; current = filepath.Dir(current) {
		if !strings.HasPrefix(strings.ToLower(current), strings.ToLower(root)+string(os.PathSeparator)) {
			return
		}
		if err := os.Remove(current); err != nil {
			return
		}
	}
}

func safeTarget(root, name string) (string, error) {
	name, err := safeRelative(name)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	for current := filepath.Dir(target); ; current = filepath.Dir(current) {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("refusing linked addon folder")
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		if current == root {
			break
		}
		parent := filepath.Dir(current)
		if parent == current || !strings.HasPrefix(strings.ToLower(current), strings.ToLower(root)+string(os.PathSeparator)) {
			return "", fmt.Errorf("addon path leaves LamdaUI")
		}
	}
	return target, nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Store) renderTOC() string {
	selected := make([]string, 0, len(s.inventory.Installed))
	for id := range s.inventory.Installed {
		selected = append(selected, id)
	}
	sort.Strings(selected)
	lines := append([]string(nil), s.catalog.Headers...)
	lines = append(lines, "## X-Lamda-Modules: "+strings.Join(selected, ","))
	for _, summary := range s.catalog.Modules {
		record, ok := s.inventory.Installed[summary.ID]
		if !ok {
			continue
		}
		for _, line := range record.Metadata {
			lines = append(lines, line)
		}
	}
	lines = append(lines, "")
	entries := map[string]map[string]bool{}
	emitted := map[string]map[string]bool{}
	for id, record := range s.inventory.Installed {
		entries[id] = map[string]bool{}
		emitted[id] = map[string]bool{}
		for _, entry := range record.Entries {
			entries[id][entry] = true
		}
	}
	for _, item := range s.catalog.Order {
		if item.Module == "core" {
			lines = append(lines, strings.ReplaceAll(item.Path, "/", "\\"))
		} else if _, ok := s.inventory.Installed[item.Module]; ok && entries[item.Module][item.Path] {
			lines = append(lines, strings.ReplaceAll(item.Path, "/", "\\"))
			emitted[item.Module][item.Path] = true
		}
	}
	for _, summary := range s.catalog.Modules {
		if record, ok := s.inventory.Installed[summary.ID]; ok {
			for _, entry := range record.Entries {
				if !emitted[summary.ID][entry] {
					lines = append(lines, strings.ReplaceAll(entry, "/", "\\"))
				}
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
