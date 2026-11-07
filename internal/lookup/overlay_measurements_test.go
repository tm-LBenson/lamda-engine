package lookup

import (
	"encoding/json"
	"testing"
)

func TestUnitMeasurementSlotsPreserveExistingNativeData(t *testing.T) {
	input := &OverlayInput{MeasurementsVersion: 1}
	p := overlayPixels()
	occupied := map[[2]int]bool{}
	for i := 0; i < 16; i++ {
		input.Measurements = append(input.Measurements, OverlayMeasurement{Unit: "target", Kind: "resource", Resource: "0", Power: 0, Mod: 10, Available: true})
		for field, value := range []int{0x28c6a1, 35000, 50000} {
			for copy := 0; copy < 2; copy++ {
				x, y := OverlayMeasurementPoint(i, field, copy)
				key := [2]int{x, y}
				if occupied[key] || x < 0 || x > 252 || !(y == 36 || y == 0 && x >= 48) {
					t.Fatal("overlap or invalid position", key)
				}
				occupied[key] = true
				paintOverlay(&p, x, y+32, value)
			}
		}
	}
	decodeOverlayMeasurements(p, 32, input)
	for _, m := range input.Measurements {
		if m.Current == nil || *m.Current != 3500 || m.Max == nil || *m.Max != 5000 {
			t.Fatal("bad resource", m)
		}
	}
	if got, ok := overlayColor(p, 40, 32); !ok || got != 90 {
		t.Fatal("existing health overwritten")
	}
	for _, v := range []struct{ field, copy, value int }{{0, 0, 0}, {1, 1, 123}, {2, 0, 0xffffff}} {
		bad := Pixels{Width: p.Width, Height: p.Height, BGRA: append([]byte(nil), p.BGRA...)}
		x, y := OverlayMeasurementPoint(0, v.field, v.copy)
		paintOverlay(&bad, x, y+32, v.value)
		decodeOverlayMeasurements(bad, 32, input)
		if input.Measurements[0].Current != nil {
			t.Fatal("occluded/mismatched measurement trusted", v)
		}
	}
	input.Measurements[1].Available = false
	decodeOverlayMeasurements(p, 32, input)
	if input.Measurements[1].Current != nil {
		t.Fatal("unavailable value reused")
	}
}
func TestUnitMeasurementMetadataCannotSupplyReadings(t *testing.T) {
	var m OverlayMeasurement
	for _, data := range []string{`["target","resource","0",0,10,true]`, `["boss1","health","",-1,1,true]`} {
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatal(err)
		}
		if m.Current != nil || m.Max != nil {
			t.Fatal("metadata supplied readings")
		}
	}
	for _, data := range []string{`["target","resource","0",0,10,true,1,9000]`, `["target","resource","-1",0,10,true]`, `["raid41","health","",-1,1,true]`, `["player","health","",-1,1,true]`} {
		if json.Unmarshal([]byte(data), &m) == nil {
			t.Fatal("malformed metadata accepted", data)
		}
	}
}
func TestUnitMeasurementHealthZeroAndUnknown(t *testing.T) {
	p := overlayPixels()
	input := &OverlayInput{MeasurementsVersion: 1, Measurements: []OverlayMeasurement{{Unit: "focus", Kind: "health", Available: true, Mod: 1}}}
	for field, value := range []int{0x28c6a1, 0, 100} {
		for copy := 0; copy < 2; copy++ {
			x, y := OverlayMeasurementPoint(0, field, copy)
			paintOverlay(&p, x, y+32, value)
		}
	}
	decodeOverlayMeasurements(p, 32, input)
	if input.Measurements[0].Current == nil || *input.Measurements[0].Current != 0 {
		t.Fatal("real zero discarded")
	}
	for copy := 0; copy < 2; copy++ {
		x, y := OverlayMeasurementPoint(0, 2, copy)
		paintOverlay(&p, x, y+32, 0)
	}
	decodeOverlayMeasurements(p, 32, input)
	if input.Measurements[0].Current != nil {
		t.Fatal("unknown became zero")
	}
}

func TestUnitMeasurementExpandedMetadataReadingsAreErased(t *testing.T) {
	current, maximum := 20.0, 100.0
	input := OverlayInput{Epoch: "test", Seq: 1, Hash: "1234567812345678", Width: 1920, Height: 1080, Offset: 32, MeasurementsVersion: 1, MeasurementHash: "1234567812345678", Measurements: []OverlayMeasurement{{Unit: "target", Kind: "health", Mod: 1, Available: true, Current: &current, Max: &maximum}}}
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	sample := parseOverlays(b)
	if sample.Error != "" {
		t.Fatal(sample.Error)
	}
	if sample.Overlays.Measurements[0].Current != nil || sample.Overlays.Measurements[0].Max != nil {
		t.Fatal("JSON supplied native readings")
	}
	var roundtrip OverlayInput
	if err = json.Unmarshal(b, &roundtrip); err != nil || roundtrip.Measurements[0].Current == nil {
		t.Fatal("diagnostic roundtrip failed", err)
	}
}
