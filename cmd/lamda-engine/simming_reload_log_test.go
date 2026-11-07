package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSimReloadDiagnosticsDeduplicateSamplesAndDescribeOnlyOSSubmission(t *testing.T) {
	lines := []string{}
	d := simReloadDiagnostics{write: func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }}
	body := []byte("2|retail|request_1|Player-1|1473|nonce_1|1800000000|1800000015")
	for i := 0; i < 50; i++ {
		d.observed(body)
		d.rejected(errors.New("reload request expired or invalid"))
	}
	if len(lines) != 2 || !strings.Contains(lines[0], "request observed") || !strings.Contains(lines[1], "request rejected") {
		t.Fatalf("per-frame log spam: %#v", lines)
	}
	d.rejected(nil)
	d.rejected(errors.New("reload request has no matching accepted character job"))
	d.rejected(errors.New("reload request has no matching accepted character job"))
	if len(lines) != 3 {
		t.Fatalf("changed rejection lost or duplicated: %#v", lines)
	}
	next := []byte(strings.Replace(string(body), "nonce_1", "nonce_2", 1))
	d.observed(next)
	d.keySubmitted()
	for i := 0; i < 50; i++ {
		d.observed(next)
		d.keySubmitted()
	}
	if len(lines) != 5 || !strings.Contains(lines[4], "submitted to Windows") || !strings.Contains(lines[4], "WoW reload is not confirmed") {
		t.Fatalf("submission missing, duplicated, or overstated: %#v", lines)
	}
	for _, line := range lines {
		if strings.Contains(line, "Player-1") || strings.Contains(line, "nonce_") {
			t.Fatal("raw character or lease leaked into diagnostics", line)
		}
	}
}
