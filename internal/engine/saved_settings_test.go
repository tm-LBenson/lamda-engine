package engine

import (
	"os"
	"strings"
	"testing"
)

func TestConfigSkipsNestedAddonState(t *testing.T) {
	// Reproduces the shared LamdaEngineDB after a sim reload attempt. Nested
	// diagnostics previously made the flat-table regexp reject the entire file.
	base := `LamdaEngineDB = {
["schema"]=1,["cdEnabled"]=true,["nativeFrames"]=true,
["checkUpdates"]=false,["notifyUpdates"]=false,["checkDays"]=1,
["overlayX"]=60,["overlayY"]=240,["overlayScale"]=1,
["simReloadDiagnostic"]={
 ["stage"]="engine-timeout",["reason"]="Quoted \" text { } and [\"schema\"]=9",
 ["nested"]={ ["engineFrameEstimates"]=false,["kickEnabled"]=false },
},
["simReloadResume"]={ ["poll"]={ ["chunks"]={1,2,3},["note"]=[=[} { -- ignored]=] } },
["engineFrameEstimates"]=true,["engineFrameSpells"]="642,363916",
["kickEnabled"]=true,["lookupBridgeX"]=370,["lookupBridgeY"]=40,
}`
	c, err := ReadConfig(writeConfigFixture(t, base))
	if err != nil {
		t.Fatal(err)
	}
	if !c.FrameEstimates || !c.KickEnabled || !c.DefensiveTracking || c.LookupBridgeX != 370 || c.LookupBridgeY != 40 || len(c.FrameSpells) != 2 {
		t.Fatalf("top-level settings lost or replaced by nested values: %+v", c)
	}
	for index, bad := range []string{
		strings.TrimSuffix(base, "}"),
		strings.Replace(base, `["engineFrameEstimates"]=true`, `["engineFrameEstimates"]={ ["value"]=true }`, 1),
		strings.Replace(base, `["kickEnabled"]=true`, `["kickEnabled"]=true,["kickEnabled"]=false`, 1),
		strings.Replace(base, `["schema"]=1`, `["schema"]={ ["schema"]=1 }`, 1),
		`LamdaEngineDB = {["simReloadDiagnostic"]={["stage"]="unterminated`,
	} {
		if _, err := ReadConfig(writeConfigFixture(t, bad)); err == nil {
			t.Fatalf("accepted malformed or wrongly typed settings case %d", index)
		}
	}
}

func TestSettingsStringsAndCommentsDoNotSupplyFields(t *testing.T) {
	fields, err := savedEngineFields([]byte(`LamdaEngineDB = {
["label"]="hello { } [\"schema\"]=9",
-- ["schema"]=8, {
--[=[ ["schema"]=7, } ]=]
["schema"]=1,
["nested"]={ ["text"]='} {', -- } ignored
 ["deeper"]={ ["schema"]=6 } },
["enabled"]=true, -- trailing comment
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 || fields["schema"] != "1" || fields["enabled"] != "true" || fields["nested"] != "{}" {
		t.Fatal(fields)
	}
}

func TestSettingsBoundedAndNeverExecuted(t *testing.T) {
	for _, data := range []string{
		`LamdaEngineDB = {["x"]={` + strings.Repeat("{", 64) + strings.Repeat("}", 65) + `}`,
		`LamdaEngineDB = {["x"]={ [=[unterminated } }`,
		`LamdaEngineDB = {["x"]={ --[[unterminated } }`,
		`LamdaEngineDB = {["x"]=true ["y"]=false}`,
	} {
		if _, err := savedEngineFields([]byte(data)); err == nil {
			t.Fatal("accepted malformed or unbounded data")
		}
	}
}

func TestLocalSavedConfigRegression(t *testing.T) {
	// Optional read-only verification against a local SavedVariables file.
	path := os.Getenv("LAMDA_SETTINGS_CHECK")
	if path == "" {
		t.Skip("local file check not requested")
	}
	c, err := ReadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.FrameEstimates || !c.KickEnabled || len(c.FrameSpells) == 0 {
		t.Fatal("local tracking settings were not recovered")
	}
}
