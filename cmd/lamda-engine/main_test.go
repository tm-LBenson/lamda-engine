package main

import (
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"testing"
	"time"
)

func TestReleaseSelection(t *testing.T) {
	for _, tag := range []string{"v0.0.9", "v0.1.0", "v0.2.0-beta.1", "v0.2.0^{}", "v0.2.0junk"} {
		if newer(tag, "0.1.0") {
			t.Fatal(tag)
		}
	}
	if got := newestTag("abc refs/tags/v0.2.0\ndef refs/tags/v0.3.0-beta.1\nghi refs/tags/v0.10.0\njkl refs/tags/v0.9.0", "0.1.0"); got != "v0.10.0" {
		t.Fatal(got)
	}
}

func TestPreviewDoesNotCreateObservations(t *testing.T) {
	m := engine.NewModel()
	c := engine.DefaultConfig()
	c.Preview = true
	rows := displayRows(m, c, time.Now())
	if len(rows) != 3 || len(m.Rows) != 0 || len(m.Seen) != 0 {
		t.Fatal("preview polluted observations")
	}
	c.Preview = false
	if len(displayRows(m, c, time.Now())) != 0 {
		t.Fatal("preview survived disable")
	}
}
