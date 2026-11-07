package engine

import (
	"os"
	"sync"
)

// Both capture loops read the same SavedVariables file. Cache successful parses
// only, retaining file identity so atomic same-size replacements invalidate it.
var savedConfigCache struct {
	sync.Mutex
	path   string
	info   os.FileInfo
	config Config
}

func ReadCachedConfig(path string) (Config, error) {
	savedConfigCache.Lock()
	defer savedConfigCache.Unlock()
	info, err := os.Stat(path)
	if err != nil {
		return DefaultConfig(), err
	}
	old := savedConfigCache.info
	if savedConfigCache.path == path && old != nil && os.SameFile(old, info) && old.Size() == info.Size() && old.ModTime().Equal(info.ModTime()) {
		return cloneConfig(savedConfigCache.config), nil
	}
	c, err := ReadConfig(path)
	if err != nil {
		return c, err
	}
	after, err := os.Stat(path)
	if err == nil && os.SameFile(info, after) && info.Size() == after.Size() && info.ModTime().Equal(after.ModTime()) {
		savedConfigCache.path, savedConfigCache.info, savedConfigCache.config = path, info, cloneConfig(c)
	}
	return c, nil
}

func cloneConfig(c Config) Config {
	c.FrameSpells = append([]int(nil), c.FrameSpells...)
	c.KickSpells = append([]int(nil), c.KickSpells...)
	return c
}
