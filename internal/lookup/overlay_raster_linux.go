//go:build linux

package lookup

import "image"

// Proton's compact ScrollFrame clips can paint the adjacent glyph into the
// right column of a 2x2 cell. The left column remains correct in both rows.
// Accept that raster only after all eight calibration colors agree vertically;
// keep the independent duplicate sinks and every shared marker/range check.
// Never apply this adjustment to metadata, the Windows reader, or legacy cells.
func decodeLinuxOverlayShape(p Pixels) BridgeSample {
	result := DecodeOverlayShape(p)
	if result.Error != "overlays_calibration" {
		return result
	}
	canonical := Pixels{Width: 256, Height: 72, BGRA: make([]byte, 18432*4)}
	for y := 0; y < 72; y++ {
		for x := 0; x < 256; x++ {
			tile := y/4*64 + x/4
			sx, sy := tile%(p.Width/4)*4+x%4, tile/(p.Width/4)*4+y%4
			copy(canonical.BGRA[(y*256+x)*4:(y*256+x+1)*4], p.BGRA[(sy*p.Width+sx)*4:(sy*p.Width+sx+1)*4])
		}
	}
	metadata := DecodeSample(canonical.Cells(image.Rect(0, 0, 256, 32)))
	if metadata.Error != "" || metadata.Overlays == nil || metadata.Overlays.NativeVersion < 2 {
		return result
	}
	for i, want := range OverlayCalibration {
		x, y := overlayCalibrationPoint(i, metadata.Overlays.NativeVersion)
		for dy := 0; dy < 2; dy++ {
			r, g, b := canonical.rgb(x, 32+y+dy)
			if int(r)<<16|int(g)<<8|int(b) != want {
				return result
			}
		}
	}
	// Retain vertical masks, including the marked lower-row duration glyph.
	for y := 32; y < 72; y++ {
		for x := 0; x < 256; x += 2 {
			i := (y*256 + x) * 4
			copy(canonical.BGRA[i+4:i+8], canonical.BGRA[i:i+4])
		}
	}
	adjusted := DecodeOverlayPixels(canonical, 0, 32)
	if adjusted.Error == "" && adjusted.Overlays.StripWidth != p.Width {
		return BridgeSample{Error: "overlays_mapping_changed"}
	}
	return adjusted
}
