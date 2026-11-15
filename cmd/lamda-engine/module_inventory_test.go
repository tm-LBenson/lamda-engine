package main

import (
	"reflect"
	"testing"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

type testModuleInventory map[string]bool

func (m testModuleInventory) Installed(id string) bool { return m[id] }

func TestModuleInventoryStopsRemovedModuleWork(t *testing.T) {
	c := engine.DefaultConfig()
	c.DefensiveTracking, c.FrameEstimates, c.EstimatePanel = true, true, true
	c.KickEnabled, c.ArenaTracking = true, true
	c.OverlaysEnabled, c.KBBEnabled, c.SimEnabled, c.AugEnabled = true, true, true, true
	c.TrackingEnabled, c.LookupBridge, c.PublicLookups = true, true, true

	got := applyModuleInventory(c, testModuleInventory{"kick": true, "kbb": true})
	if got.Enabled || got.DefensiveTracking || got.FrameEstimates || got.EstimatePanel || got.PublicLookups {
		t.Fatal("removed LamdaCD kept engine work enabled")
	}
	if !got.KickEnabled || !got.KBBEnabled || !got.TrackingEnabled || !got.LookupBridge {
		t.Fatal("installed modules were disabled")
	}
	if got.ArenaTracking || got.OverlaysEnabled || got.SimEnabled || got.AugEnabled {
		t.Fatal("removed modules kept engine work enabled")
	}
}

func TestModuleInventoryKeepsLegacyConfigWhenStoreUnavailable(t *testing.T) {
	c := engine.DefaultConfig()
	c.KickEnabled, c.OverlaysEnabled = true, true
	if got := applyModuleInventory(c, nil); !reflect.DeepEqual(got, c) {
		t.Fatal("missing module store changed legacy configuration")
	}
}

func TestNewMarketInstallStartsBeforeFirstSettingsWrite(t *testing.T) {
	modules := testModuleInventory{"market": true}
	c := engine.DefaultClientConfig()
	c.MarketConfigured = false
	c.MarketEnabled = false
	if !applyModuleInventory(c, modules).MarketEnabled {
		t.Fatal("newly installed market requires a second settings write")
	}
	c.MarketConfigured = true
	if applyModuleInventory(c, modules).MarketEnabled {
		t.Fatal("explicit market opt-out was ignored")
	}
	c.MarketEnabled = true
	if applyModuleInventory(c, testModuleInventory{}).MarketEnabled {
		t.Fatal("removed module kept capturing")
	}
}
