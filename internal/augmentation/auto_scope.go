package augmentation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// The native addon emits one token at the actual combat/key boundary. Persist
// it with the desktop selection so a manual choice survives later frames and
// Engine restarts. Called under Store.mu, only for validated native captures.
func (s *Store) syncAutoScope(n *lookup.NativeMeter) {
	a := n.Auto
	if a == nil || a.Token == "" || s.path == "" {
		return
	}
	token := n.Player + "/" + a.Token
	if token == s.autoScopeToken {
		return
	}
	s.autoScopeToken = token
	if !(a.Scope == "boss" && a.OnCombat || a.Scope == "overall" && a.AfterKey) {
		return
	}
	err := s.writeAutoScope(token, a.Scope)
	if s.debug != nil {
		s.debug.write(map[string]any{"kind": "meter_auto_scope", "scope": a.Scope, "token": token, "error": fmt.Sprint(err)})
	}
}
func (s *Store) writeAutoScope(token, scope string) error {
	path := filepath.Join(filepath.Dir(s.path), "display.json")
	doc := map[string]any{"schema": 1, "scopeOnly": true, "scope": "boss"}
	bytes, err := os.ReadFile(path)
	if err == nil {
		if len(bytes) > 4096 || json.Unmarshal(bytes, &doc) != nil || doc == nil {
			return fmt.Errorf("invalid meter placement; preserved original")
		}
		if doc["autoToken"] == token {
			return nil
		}
		// Preserve malformed files for recovery rather than replacing user layout.
		if s.placement() == nil {
			return fmt.Errorf("invalid meter placement; preserved original")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	doc["scope"] = scope
	doc["autoToken"] = token
	return engine.WriteJSON(path, doc)
}
