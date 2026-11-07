package simming

import (
	"net/http"
	"sort"
	"strings"
)

// Only identities leave the local stores; the picker does not need gear or cards.
func (s *Service) overlayCharacters(w http.ResponseWriter, r *http.Request) {
	type identity struct {
		Character string `json:"character"`
		Realm     string `json:"realm"`
	}
	characters := map[string]identity{}
	add := func(name, realm string) {
		name, realm = strings.TrimSpace(name), strings.TrimSpace(realm)
		if name == "" || realm == "" {
			return
		}
		key := strings.ToLower(name) + "\x00" + strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(realm, " ", ""), "-", ""))
		characters[key] = identity{name, realm}
	}
	if s.Workspace != nil {
		s.Workspace.mu.Lock()
		for _, snapshot := range s.Workspace.snapshots {
			add(snapshot.Character.Name, snapshot.Character.Realm)
		}
		s.Workspace.mu.Unlock()
	}
	if s.Planner != nil {
		for _, character := range s.Planner.CharacterNames() {
			add(character[0], character[1])
		}
	}
	if s.Overlays != nil {
		for _, character := range s.Overlays.MemoryCharacters() {
			add(character.Character, character.Realm)
		}
	}
	keys := make([]string, 0, len(characters))
	for key := range characters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]identity, 0, len(keys))
	for _, key := range keys {
		out = append(out, characters[key])
	}
	jsonResponse(w, map[string]any{"characters": out})
}
