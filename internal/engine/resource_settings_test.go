package engine

import "testing"

func TestResourceConfigPreservesRequiredTrackingAndOldSettings(t *testing.T) {
	base := `LamdaEngineDB={ ["schema"]=1,["cdEnabled"]=true,["engineFrameEstimates"]=true,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,["kickEnabled"]=true,`
	cases := []struct {
		settings              string
		lookups, idle, smooth bool
	}{
		{``, true, true, true},
		{`["resourceLow"]=true,`, false, false, false},
		{`["resourceLow"]=true,["resourceAdvanced"]=true,`, true, true, true},
		{`["resourceAdvanced"]=true,["resourceLookups"]=false,["resourceFastIdle"]=false,["resourceSmooth"]=false,`, false, false, false},
		{`["resourceAdvanced"]=false,["resourceLookups"]=false,["resourceFastIdle"]=false,`, true, true, true},
		{`["publicLookups"]=false,["resourceAdvanced"]=true,["resourceLookups"]=true,`, false, true, true},
	}
	for _, tc := range cases {
		c, err := ReadConfig(writeConfigFixture(t, base+tc.settings+`}`))
		if err != nil || c.PublicLookups != tc.lookups || c.OptionalWork(c.ResourceFastIdle) != tc.idle || c.OptionalWork(c.ResourceSmooth) != tc.smooth {
			t.Fatal(tc, c, err)
		}
		if !c.TrackingEnabled || !c.LookupBridge || !c.KickEnabled || !c.DefensiveTracking {
			t.Fatal("required work disabled", c)
		}
	}
	if _, err := ReadConfig(writeConfigFixture(t, base+`["resourceLow"]="yes"}`)); err == nil {
		t.Fatal("malformed policy accepted")
	}
}
