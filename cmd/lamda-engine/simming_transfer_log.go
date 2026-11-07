package main

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
)

// One bounded request summary. Never log character/profile/packet contents and
// never emit a line for every 200ms sample or repeated transmission pass.
type simTransferDiagnostics struct {
	identity, tag, problem, reported string
	count                            int
	seen                             map[int]bool
	decoded                          bool
	write                            func(string, ...any)
}

func (d *simTransferDiagnostics) observed(body []byte) {
	header, _, _ := strings.Cut(string(body), "\n")
	f := strings.Split(header, "|")
	if len(f) != 8 {
		return
	}
	index, e1 := strconv.Atoi(f[5])
	count, e2 := strconv.Atoi(f[6])
	if e1 != nil || e2 != nil || count < 1 || count > 110 || index < 1 || index > count {
		return
	}
	identity := strings.Join(append(append([]string{}, f[:5]...), f[6:]...), "|")
	if identity != d.identity {
		d.summary()
		sum := sha256.Sum256([]byte(identity))
		d.identity, d.tag, d.count = identity, fmt.Sprintf("%x", sum[:6]), count
		d.seen = map[int]bool{}
		d.problem, d.reported, d.decoded = "", "", false
		d.write("Sim data transfer observed: transfer=%s chunks=%d", d.tag, count)
	}
	d.seen[index] = true
}

func (d *simTransferDiagnostics) result(decoded bool, err error) {
	if err != nil {
		problem := strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return ' '
			}
			return r
		}, err.Error())
		if len(problem) > 240 {
			problem = problem[:240]
		}
		if problem != d.problem {
			d.problem = problem
			d.write("Sim data transfer rejected: transfer=%s reason=%s", d.tag, problem)
		}
	}
	if decoded && !d.decoded {
		d.decoded = true
		d.write("Sim data transfer decoded: transfer=%s chunks=%d", d.tag, d.count)
	}
}

func (d *simTransferDiagnostics) summary() {
	if d.identity == "" || d.decoded {
		return
	}
	status := fmt.Sprintf("%d/%d:%s", len(d.seen), d.count, d.problem)
	if status != d.reported {
		d.reported = status
		d.write("Sim data transfer incomplete: transfer=%s received=%d/%d", d.tag, len(d.seen), d.count)
	}
}
