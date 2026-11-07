package engine

import (
	"fmt"
	"testing"
)

func TestModuleDefaultsAndSavedOptInsForBothClients(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureClient("retail") })
	for _, client := range []string{"retail", "forever"} {
		t.Run(client, func(t *testing.T) {
			if err := ConfigureClient(client); err != nil {
				t.Fatal(err)
			}
			fresh := DefaultClientConfig()
			if fresh.Enabled || fresh.DefensiveTracking || fresh.TrackingEnabled || fresh.KickEnabled || fresh.FrameEstimates || fresh.ArenaTracking || fresh.AugEnabled {
				t.Fatal("modules active before first saved settings", fresh)
			}
			for _, enabled := range []bool{false, true} {
				// Omit foreverEngineV1 to exercise the legacy compatibility path.
				data := fmt.Sprintf(`LamdaEngineDB = {
				["schema"]=1,["cdEnabled"]=%t,["kickEnabled"]=%t,["gladiusEnabled"]=%t,["augEnabled"]=%t,
				["engineTracking"]=true,["engineFrameEstimates"]=true,["gladiusEngine"]=true,
				["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,
				["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1}`, enabled, enabled, enabled, enabled)
				config, err := ReadConfig(writeConfigFixture(t, data))
				if err != nil {
					t.Fatal(err)
				}
				if config.Enabled != enabled || config.DefensiveTracking != enabled || config.KickEnabled != enabled || config.FrameEstimates != enabled || config.TrackingEnabled != enabled || config.ArenaTracking != (enabled && client == "retail") || config.AugEnabled != (enabled && client == "retail") {
					t.Fatal("saved module choices changed", config)
				}
			}
		})
	}
}
