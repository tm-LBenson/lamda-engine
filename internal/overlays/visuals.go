package overlays

import "github.com/tm-LBenson/lamda-engine/internal/engine"

type VisualStyle = engine.OverlayVisual

func validVisual(v *VisualStyle) bool {
	if v == nil {
		return true
	}
	return validMotion(v.Motion) && validSprite(v.Sprite) && (v.Width == 0 || bounded(v.Width, 8, 2048)) &&
		one(v.FontFamily, "", "ui", "arial", "georgia", "consolas") && one(v.TextEffect, "", "shadow", "outline", "none") &&
		(v.Image == "" || backgroundHash(v.Image)) && one(v.Fit, "", "contain", "cover", "stretch") &&
		(v.Height == 0 || bounded(v.Height, 8, 2048)) &&
		bounded(v.Rotation, -360, 360) && v.Layer >= -100 && v.Layer <= 100 &&
		one(v.Shape, "", "rectangle", "circle", "diamond", "triangle", "arrow", "cross", "star") &&
		one(v.Direction, "", "right", "left", "up", "down") &&
		(v.TrackColor == "" || colorValue(v.TrackColor) != "") &&
		(v.TrackOpacity == nil || bounded(*v.TrackOpacity, 0, 100)) &&
		(v.FillOpacity == nil || bounded(*v.FillOpacity, 0, 100)) &&
		one(v.TextAlign, "", "left", "center", "right") &&
		one(v.BarIcon, "", "none", "left", "right", "top", "bottom")
}

func validSprite(v *engine.OverlaySprite) bool {
	return v == nil || v.Columns >= 1 && v.Columns <= 64 && v.Rows >= 1 && v.Rows <= 64 &&
		v.Frames >= 1 && v.Frames <= 256 && v.Start >= 0 && v.Start+v.Frames <= v.Columns*v.Rows &&
		bounded(v.FPS, 1, 60) && one(v.Mode, "loop", "once", "bounce", "progress")
}

func spriteFits(v *engine.OverlaySprite, width, height int) bool {
	return v == nil || validSprite(v) && v.Columns <= width && v.Rows <= height
}

func resolveVisual(v *VisualStyle) *engine.OverlayVisual {
	if v == nil {
		return nil
	}
	copy := *v
	copy.Motion = resolveMotion(v.Motion)
	if copy.TrackColor != "" {
		copy.TrackColor = colorValue(copy.TrackColor)
	}
	return &copy
}

func visualLayer(v engine.OverlayDisplay) int {
	if v.Visual != nil {
		return v.Visual.Layer
	}
	return 0
}
