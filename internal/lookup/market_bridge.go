package lookup

import "image"

// Probe the bulk board at its own clamped origin. A legacy module still has its
// original origin/geometry, so callers fall back to that reader when absent.
func captureMarketBoard(position BridgePosition, width, height int, capture func(BridgePosition, int, int) (Pixels, bool)) (BridgeSample, bool) {
	p, fits := position.clampSize(width, height, MarketBridgeWidth, MarketBridgeHeight)
	if !fits {
		return BridgeSample{}, false
	}
	pixels, ok := capture(p, MarketBridgeWidth, MarketBridgeHeight)
	if !ok {
		return BridgeSample{}, false
	}
	data := pixels.Cells(image.Rect(0, 0, MarketBridgeWidth, MarketBridgeHeight))
	if len(data) < 4 || string(data[:4]) != "LMK2" {
		return BridgeSample{}, false
	}
	return DecodeSample(data), true
}
