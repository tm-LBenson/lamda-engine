package engine

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConfigCacheReplacementFailureAndIndependentSlices(t *testing.T) {
	text := `LamdaEngineDB = {["schema"]=1,["cdEnabled"]=true,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=60,["overlayY"]=240,["overlayScale"]=1,["engineFrameSpells"]="642,363916"}`
	path := writeConfigFixture(t, text)
	first, err := ReadCachedConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.FrameSpells) == 0 {
		t.Fatal("missing spells")
	}
	first.FrameSpells[0] = -1
	second, err := ReadCachedConfig(path)
	if err != nil || second.FrameSpells[0] == -1 {
		t.Fatal("shared mutable config", err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if _, err := ReadCachedConfig(path); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	original, _ := os.Stat(path)
	replacement := filepath.Join(filepath.Dir(path), "replacement.lua")
	bad := make([]byte, original.Size())
	for i := range bad {
		bad[i] = 'x'
	}
	if err := os.WriteFile(replacement, bad, 0600); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(replacement, original.ModTime(), original.ModTime())
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCachedConfig(path); err == nil {
		t.Fatal("atomic same-size/mtime replacement used old cache")
	}
	os.WriteFile(path, []byte(text), 0600)
	if _, err := ReadCachedConfig(path); err != nil {
		t.Fatal("failed parse poisoned cache", err)
	}
	os.Remove(path)
	if _, err := ReadCachedConfig(path); err == nil {
		t.Fatal("deleted config remained valid")
	}
}
