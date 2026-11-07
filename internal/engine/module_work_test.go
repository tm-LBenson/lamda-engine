package engine

import (
	"fmt"
	"testing"
)

func TestModuleWorkConfiguration(t *testing.T) {
	base := `LamdaEngineDB={["schema"]=1,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,`
	for mask := 0; mask < 64; mask++ {
		cd, display, tracking, bridge, kick, public := mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0, mask&32 != 0
		fields := fmt.Sprintf(`["cdEnabled"]=%t,["engineFrameEstimates"]=%t,["engineTracking"]=%t,["lookupBridge"]=%t,["kickEnabled"]=%t,["publicLookups"]=%t}`, cd, display, tracking, bridge, kick, public)
		c, err := ReadConfig(writeConfigFixture(t, base+fields))
		if err != nil {
			t.Fatal(err)
		}
		demand := cd && display && tracking && bridge
		if c.DefensiveTracking != demand || c.FrameEstimates != demand || c.PublicLookups != (demand && public) || c.TrackingEnabled != (demand || kick) || c.LookupBridge != (demand || kick) {
			t.Fatal("unexpected worker demand", mask, c)
		}
	}
	for _, name := range []string{"kbbEnabled", "simEnabled", "overlaysEnabled"} {
		c, e := ReadConfig(writeConfigFixture(t, base+fmt.Sprintf(`["cdEnabled"]=false,["%s"]=true}`, name)))
		if e != nil || c.KBBEnabled != (name == "kbbEnabled") || c.SimEnabled != (name == "simEnabled") || c.OverlaysEnabled != (name == "overlaysEnabled") || c.TrackingEnabled || c.PublicLookups {
			t.Fatal(name, c, e)
		}
		if _, e := ReadConfig(writeConfigFixture(t, base+fmt.Sprintf(`["cdEnabled"]=false,["%s"]="true"}`, name))); e == nil {
			t.Fatal("invalid flag", name)
		}
	}
	c := DefaultClientConfig()
	if c.TrackingEnabled || c.FrameEstimates || c.KickEnabled || c.ArenaTracking || c.OverlaysEnabled || c.SimEnabled || c.KBBEnabled || c.PublicLookups || c.LookupBridge {
		t.Fatal("workers active before config", c)
	}
}
