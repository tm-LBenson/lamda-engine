package engine

// Keep our own output from obscuring capture panels. Custom overlays retain
// their placement; the renderer clips only reserved pixels, including shadows.
func ReserveCaptureArea(d *FrameDisplay, x, y, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	overlaps := func(left, top, w, h int) bool { return left < x+width && left+w > x && top < y+height && top+h > y }
	left, top := max(0, x), max(0, y)
	right, bottom := min(d.Width, x+width), min(d.Height, y+height)
	if right > left && bottom > top {
		area := CaptureArea{left, top, right - left, bottom - top}
		duplicate := false
		for _, existing := range d.CaptureAreas {
			if existing == area {
				duplicate = true
				break
			}
		}
		if !duplicate {
			d.CaptureAreas = append(d.CaptureAreas, area)
		}
	}
	icons := d.Icons[:0]
	for _, icon := range d.Icons {
		if overlaps(icon.X, icon.Y, icon.Size, icon.Size) {
			d.Omitted++
		} else {
			icons = append(icons, icon)
		}
	}
	d.Icons = icons
	notices := d.Notices[:0]
	for _, notice := range d.Notices {
		if overlaps(notice.X, notice.Y, notice.Width, 18) {
			d.Omitted++
		} else {
			notices = append(notices, notice)
		}
	}
	d.Notices = notices
	if panel := d.KickPanel; panel != nil && overlaps(panel.X, panel.Y, panel.Width, 22) {
		d.Omitted += len(d.KickBars)
		d.KickBars = nil
		d.KickPanel = nil
		return
	}
	// Bars have a shared background; hiding just an intersecting row would still
	// cover the panel and make capture flicker as the overlay appears/disappears.
	for _, bar := range d.KickBars {
		if overlaps(bar.X, bar.Y, bar.Width, bar.Height) {
			d.Omitted += len(d.KickBars)
			d.KickBars = nil
			d.KickPanel = nil
			break
		}
	}
}
