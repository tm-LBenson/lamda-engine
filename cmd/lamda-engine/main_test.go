package main

import "testing"

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
