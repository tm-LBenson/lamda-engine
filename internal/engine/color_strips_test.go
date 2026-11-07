package engine

import (
	"fmt"
	"strings"
	"testing"
)

func TestIndependentStripSettingsValidateAndPreservePositions(t *testing.T) {
	base := `LamdaEngineDB={["schema"]=1,["cdEnabled"]=false,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,`
	settings := `["colorStripVersion"]=2,["lookupBridgeX"]=21,["lookupBridgeY"]=22,["kickBridgeX"]=31,["kickBridgeY"]=32,["gladiusBridgeX"]=41,["gladiusBridgeY"]=42,["kbbBridgeX"]=51,["kbbBridgeY"]=52,["simBridgeX"]=61,["simBridgeY"]=62}`
	c, err := ReadConfig(writeConfigFixture(t, base+settings))
	if err != nil || c.ColorStripVersion != 2 || c.LookupBridgeX != 21 || c.KickBridgeY != 32 || c.GladiusBridgeX != 41 || c.KBBBridgeY != 52 || c.SimBridgeX != 61 {
		t.Fatal(c, err)
	}
	for _, key := range []string{"kickBridgeX", "kickBridgeY", "gladiusBridgeX", "gladiusBridgeY", "kbbBridgeX", "kbbBridgeY", "simBridgeX", "simBridgeY"} {
		for _, value := range []string{"-1", "16001", "1.5", `"23"`, `true`} {
			if _, err := ReadConfig(writeConfigFixture(t, base+`["`+key+`"]=`+value+`}`)); err == nil {
				t.Fatal("invalid position", key, value)
			}
		}
	}
	if _, err := ReadConfig(writeConfigFixture(t, base+strings.Replace(settings, `["colorStripVersion"]=2`, `["colorStripVersion"]=3`, 1))); err == nil {
		t.Fatal("unknown layout accepted")
	}
}

func TestOverlayWidthSettings(t *testing.T) {
	base := `LamdaEngineDB={["schema"]=1,["cdEnabled"]=false,["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,["overlayX"]=0,["overlayY"]=0,["overlayScale"]=1,`
	for _, width := range []int{16, 24, 64, 128, 144, 256, 512, 1152} {
		c, err := ReadConfig(writeConfigFixture(t, base+fmt.Sprintf(`["overlaysBridgeWidth"]=%d}`, width)))
		if err != nil || c.OverlaysBridgeWidth != width {
			t.Fatal(width, c, err)
		}
	}
	for _, value := range []string{"0", "20", "255", "1024", "16000", "64.5", `"64"`} {
		if _, err := ReadConfig(writeConfigFixture(t, base+`["overlaysBridgeWidth"]=`+value+`}`)); err == nil {
			t.Fatal("invalid width accepted", value)
		}
	}
	c, err := ReadConfig(writeConfigFixture(t, base+`}`))
	if err != nil || c.OverlaysBridgeWidth != 0 {
		t.Fatal("legacy capture must remain until settings save", c, err)
	}
}
