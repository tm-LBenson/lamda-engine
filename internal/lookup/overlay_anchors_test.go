package lookup

import "testing"

func TestAnchorMetadataInvalidityOnlyDiscardsGeometry(t *testing.T) {
	p := OverlayInput{Width: 1920, Height: 1080, AnchorsVersion: 1, Anchors: []OverlayAnchor{{Kind: "frame", Target: "Minimap", Available: true, X: 100, Y: 100, Width: 200, Height: 200}}}
	sanitizeAnchors(&p)
	if !p.Anchors[0].Available {
		t.Fatal("valid rectangle rejected")
	}
	p.Anchors[0].X = 1900
	sanitizeAnchors(&p)
	if p.Anchors[0].Available || p.Anchors[0].Width != 0 {
		t.Fatal("offscreen rectangle retained")
	}
	p.Anchors = append(p.Anchors, p.Anchors[0])
	sanitizeAnchors(&p)
	if len(p.Anchors) != 0 {
		t.Fatal("duplicate identity accepted")
	}
	for _, w := range []OverlayAnchor{{Kind: "frame", Target: "PlayerFrame"}, {Kind: "unit", Target: "raid40"}, {Kind: "nameplate", Target: "nameplate40"}} {
		if !ValidAnchor(w.Kind, w.Target) {
			t.Fatal(w)
		}
	}
	for _, w := range []OverlayAnchor{{Kind: "frame", Target: "foo()"}, {Kind: "unit", Target: "raid41"}, {Kind: "nameplate", Target: "nameplate0"}} {
		if ValidAnchor(w.Kind, w.Target) {
			t.Fatal(w)
		}
	}
}
