package main

import (
	"crypto/sha256"
	"fmt"
)

// The capture loop sees one lease every 200 ms. Keep only its bounded body and
// latest rejection so diagnostics do not repeat once per frame. The short tag
// correlates lines without recording character identity or the full packet.
type simReloadDiagnostics struct {
	body, tag, problem string
	submitted          bool
	write              func(string, ...any)
}

func (d *simReloadDiagnostics) observed(body []byte) {
	if string(body) == d.body {
		return
	}
	d.body = string(body)
	sum := sha256.Sum256(body)
	d.tag = fmt.Sprintf("%x", sum[:6])
	d.problem = ""
	d.submitted = false
	d.write("Sim reload request observed: lease=%s bytes=%d", d.tag, len(body))
}

func (d *simReloadDiagnostics) rejected(err error) {
	if err == nil || err.Error() == d.problem {
		return
	}
	d.problem = err.Error()
	d.write("Sim reload request rejected: lease=%s reason=%s", d.tag, d.problem)
}

func (d *simReloadDiagnostics) keySubmitted() {
	if d.submitted {
		return
	}
	d.submitted = true
	d.write("Sim reload Ctrl+Shift+F12 submitted to Windows: lease=%s; WoW reload is not confirmed", d.tag)
}
