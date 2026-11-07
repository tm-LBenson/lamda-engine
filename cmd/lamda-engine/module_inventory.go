package main

import "github.com/tm-LBenson/lamda-engine/internal/engine"

type moduleInventory interface {
	Installed(string) bool
}

// applyModuleInventory prevents retained SavedVariables from running engine
// work for code the user removed in Browse Addons. A missing store preserves
// the legacy configuration behavior so a catalog problem cannot disable an
// existing pre-store installation.
func applyModuleInventory(c engine.Config, modules moduleInventory) engine.Config {
	if modules == nil {
		return c
	}
	if !modules.Installed("cd") {
		c.Enabled = false
		c.DefensiveTracking = false
		c.FrameEstimates = false
		c.EstimatePanel = false
	}
	if !modules.Installed("kick") {
		c.KickEnabled = false
	}
	if !modules.Installed("gladius") {
		c.ArenaTracking = false
	}
	if !modules.Installed("overlays") {
		c.OverlaysEnabled = false
	}
	if !modules.Installed("kbb") {
		c.KBBEnabled = false
	}
	if !modules.Installed("augmentation") {
		c.AugEnabled = false
	}
	if !modules.Installed("sim") {
		c.SimEnabled = false
	}
	c.TrackingEnabled = c.DefensiveTracking || c.KickEnabled
	c.LookupBridge = c.TrackingEnabled
	c.PublicLookups = c.PublicLookups && c.DefensiveTracking
	return c
}
