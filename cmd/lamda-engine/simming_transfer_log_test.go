package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSimTransferDiagnosticsExposeRejectedAndIncompleteTransfersWithoutSpam(t *testing.T) {
	var lines []string
	d := simTransferDiagnostics{write: func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }}
	packet := func(index int) []byte {
		return []byte(fmt.Sprintf("2|Player-1_request|retail|1800000000|12000|%d|3|1234\nprivate-profile", index))
	}
	for repeat := 0; repeat < 20; repeat++ {
		d.observed(packet(1))
		d.observed(packet(3))
		d.result(false, nil)
		d.summary()
	}
	if len(lines) != 2 || !strings.Contains(lines[1], "received=2/3") {
		t.Fatal("missing-part summary lost or repeated", lines)
	}
	d.observed(packet(2))
	for repeat := 0; repeat < 20; repeat++ {
		d.result(false, errors.New("invalid item upgrade metadata"))
		d.summary()
	}
	if len(lines) != 4 || !strings.Contains(lines[2], "invalid item upgrade metadata") || !strings.Contains(lines[3], "received=3/3") {
		t.Fatal("decode rejection swallowed or repeated", lines)
	}
	for repeat := 0; repeat < 20; repeat++ {
		d.result(true, nil)
		d.summary()
	}
	if len(lines) != 5 || !strings.Contains(lines[4], "decoded") {
		t.Fatal("completion lost or repeated", lines)
	}
	d.observed([]byte("2|next|retail|1800000000|1|1|1|2\nx"))
	d.result(false, errors.New("new rejection\n"+strings.Repeat("x", 500)))
	if len(lines) != 7 || strings.Contains(lines[6], "\n") || len(lines[6]) > 320 {
		t.Fatal("new request or bounded error", lines)
	}
	for _, line := range lines {
		if strings.Contains(line, "Player-1") || strings.Contains(line, "private-profile") {
			t.Fatal("raw character/profile logged", line)
		}
	}
}
