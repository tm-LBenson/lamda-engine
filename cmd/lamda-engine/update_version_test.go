package main

import "testing"

func TestClientReleaseSelection(t *testing.T) {
	for _, test := range []struct {
		candidate, current string
		want               bool
	}{
		{"v0.7.13-retail.2", "0.7.13-retail.1", true},
		{"0.7.13-forever.10", "0.7.13-forever.2", true},
		{"0.8.0-forever.1", "0.7.13-forever.99", true},
		{"0.7.13-retail.1", "0.7.13-retail.2", false},
		{"0.7.13-retail.2", "0.7.13-retail.2", false},
		{"9.0.0-retail.1", "0.7.13-forever.1", false},
		{"9.0.0", "0.7.13-retail.1", false},
		{"9.0.0-forever.1", "0.7.13", false},
		{"0.7.13-retail.2^{}", "0.7.13-retail.1", false},
		{"0.7.13-retail.2junk", "0.7.13-retail.1", false},
		{"0.7.13-retail.99999999999999999999999", "0.7.13-retail.1", false},
	} {
		if got := newer(test.candidate, test.current); got != test.want {
			t.Errorf("newer(%q, %q) = %v", test.candidate, test.current, got)
		}
	}
	tags := "abc refs/tags/v99.0.0\ndef refs/tags/v99.0.0-forever.1\nghi refs/tags/v0.7.13-retail.3\njkl refs/tags/v0.7.13-retail.2"
	if got := newestTag(tags, "0.7.13-retail.1"); got != "v0.7.13-retail.3" {
		t.Fatal(got)
	}
}
