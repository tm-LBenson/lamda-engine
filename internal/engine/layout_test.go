package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutAnchorsGridAndGrowth(t *testing.T) {
	c := DefaultConfig()
	c.Columns = 2
	for anchor := 1; anchor <= 9; anchor++ {
		c.Anchor = anchor
		l := LayoutFor(c, 3, 1)
		if l.Width != 683 || l.Height != 95 || l.Cells[2].Y != 61 {
			t.Fatal(l)
		}
		if l.AnchorX != float64((anchor-1)%3)/2 || l.AnchorY != float64((anchor-1)/3)/2 {
			t.Fatal(l)
		}
	}
	c.Grow = 2
	l := LayoutFor(c, 3, 0)
	if l.Cells[0].Y != 37 || l.Cells[2].Y != 0 {
		t.Fatal(l)
	}
	c.Scale = 2
	l = LayoutFor(c, 3, 0)
	if l.Width != 1366 || l.RowHeight != 68 || l.Cells[0].Y != 74 {
		t.Fatal(l)
	}
	c.FontSize = 32
	c.Height = 20
	l = LayoutFor(c, 1, 0)
	if l.FontSize > l.RowHeight-4 {
		t.Fatal("text exceeds row", l)
	}
}
func TestProgressDurationIsKnownOnly(t *testing.T) {
	c, _ := Parse(mirror)
	m := NewModel()
	m.Observe(c, c.At)
	if m.Active(c.At)[0].Duration != 120 {
		t.Fatal("missing player duration")
	}
	npc, _ := Parse(barrier)
	m.Observe(npc, npc.At)
	for _, r := range m.Active(npc.At) {
		if r.Companion && r.Duration != 0 {
			t.Fatal("invented NPC progress")
		}
	}
}

func TestCustomizationConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.lua")
	data := `LamdaEngineDB = {
 ["schema"]=1,
 ["cdEnabled"]=true,
 ["checkUpdates"]=false,
 ["notifyUpdates"]=false,
 ["checkDays"]=1,
 ["overlayX"]=-20,
 ["overlayY"]=-40,
 ["overlayScale"]=1.5,
 ["anchor"]=9,
 ["grow"]=2,
 ["rowWidth"]=420,
 ["rowHeight"]=38,
 ["fontSize"]=16,
 ["opacity"]=65,
 ["showNames"]=false,
 ["showSpells"]=true,
 ["bars"]=false,
 }`
	os.WriteFile(path, []byte(data), 0600)
	c, e := ReadConfig(path)
	if e != nil || c.X != -20 || c.Anchor != 9 || c.Width != 420 || c.FontSize != 16 || c.ShowNames || c.Bars {
		t.Fatal(c, e)
	}
	for _, bad := range []string{strings.Replace(data, `["anchor"]=9`, `["anchor"]=10`, 1), strings.Replace(data, `["opacity"]=65`, `["opacity"]=0`, 1)} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, e := ReadConfig(path); e == nil {
			t.Fatal("invalid layout accepted")
		}
	}
}
