package lookup

import "testing"

func TestBridgePositionClampsToClient(t *testing.T) {
	for _, test := range []struct {
		name          string
		preferred     BridgePosition
		width, height int
		want          BridgePosition
		fits          bool
	}{
		{"origin", BridgePosition{}, 1600, 1000, BridgePosition{}, true},
		{"moved", BridgePosition{271, 140}, 1600, 1000, BridgePosition{271, 140}, true},
		{"edge", BridgePosition{1500, 990}, 1600, 1000, BridgePosition{1472, 984}, true},
		{"resized", BridgePosition{1500, 990}, 1000, 600, BridgePosition{872, 584}, true},
		{"negative", BridgePosition{-2, -8}, 1600, 1000, BridgePosition{}, true},
		{"exact", BridgePosition{20, 20}, 128, 16, BridgePosition{}, true},
		{"too narrow", BridgePosition{}, 127, 1000, BridgePosition{}, false},
		{"too short", BridgePosition{}, 1600, 15, BridgePosition{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, fits := test.preferred.Clamp(test.width, test.height)
			if got != test.want || fits != test.fits {
				t.Fatal(got, fits)
			}
		})
	}
}
