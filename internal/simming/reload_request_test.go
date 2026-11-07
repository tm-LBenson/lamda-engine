package simming

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func reloadTestState(now time.Time) WorkspaceState {
	s := Snapshot{Schema: 1, Client: "retail", ID: "request_1", CapturedAt: now.Unix(),
		Character:   Character{GUID: "Player-1136-TEST", Spec: 1473},
		GameRequest: &GameRequest{Schema: 1, ID: "request_1", Task: "baseline", Scenario: DefaultScenario()}}
	return WorkspaceState{Client: "retail", Snapshots: []Summary{{Snapshot: s}},
		Jobs: []Job{{ID: "job_1", RequestID: s.ID, SnapshotID: s.ID, GameTask: "baseline", State: "running"}}}
}

func reloadTestBody(now time.Time, nonce string) []byte {
	return []byte(fmt.Sprintf("2|retail|request_1|Player-1136-TEST|1473|%s|%d|%d", nonce, now.Unix(), now.Unix()+15))
}

func TestReloadRequestStrictIdentityAndLeaseParsing(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	valid := reloadTestBody(now, "nonce_1")
	request, err := ParseReloadRequest(valid, "retail", now)
	if err != nil || request == nil || request.Client != "retail" || request.RequestID != "request_1" || request.GUID != "Player-1136-TEST" || request.Spec != 1473 || request.Nonce != "nonce_1" || request.At != now.Unix() || request.Expires != now.Unix()+15 {
		t.Fatalf("valid lease lost identity: %#v %v", request, err)
	}
	for name, change := range map[string]func([]string){
		"version":        func(f []string) { f[0] = "1" },
		"foreign client": func(f []string) { f[1] = "forever" },
		"request path":   func(f []string) { f[2] = "../request" },
		"request size":   func(f []string) { f[2] = strings.Repeat("a", 97) },
		"guid code":      func(f []string) { f[3] = "Player;ReloadUI()" },
		"guid size":      func(f []string) { f[3] = strings.Repeat("a", 129) },
		"negative spec":  func(f []string) { f[4] = "-1" },
		"spec size":      func(f []string) { f[4] = "10001" },
		"spec fraction":  func(f []string) { f[4] = "1473.5" },
		"nonce newline":  func(f []string) { f[5] = "nonce\nrequest" },
		"nonce size":     func(f []string) { f[5] = strings.Repeat("a", 129) },
		"nonce empty":    func(f []string) { f[5] = "" },
		"future":         func(f []string) { f[6] = fmt.Sprint(now.Unix() + 1) },
		"too old":        func(f []string) { f[6] = fmt.Sprint(now.Unix() - 6) },
		"empty lease":    func(f []string) { f[7] = f[6] },
		"long lease":     func(f []string) { f[7] = fmt.Sprint(now.Unix() + 16) },
		"overflow":       func(f []string) { f[7] = "9223372036854775808" },
	} {
		t.Run(name, func(t *testing.T) {
			fields := strings.Split(string(valid), "|")
			change(fields)
			if r, err := ParseReloadRequest([]byte(strings.Join(fields, "|")), "retail", now); err == nil || r != nil {
				t.Fatalf("invalid lease accepted: %#v %v", r, err)
			}
		})
	}
	for _, bad := range [][]byte{nil, []byte(strings.Repeat("a", 513)), append(append([]byte{}, valid...), '|'), []byte("2|retail")} {
		if r, err := ParseReloadRequest(bad, "retail", now); err == nil || r != nil {
			t.Fatal("invalid packet size or field count accepted")
		}
	}
	if _, err := ParseReloadRequest(valid, "retail", now.Add(5*time.Second)); err != nil {
		t.Fatal("five-second-old sample should remain valid:", err)
	}
	if _, err := ParseReloadRequest(valid, "retail", now.Add(5*time.Second+time.Millisecond)); err == nil {
		t.Fatal("sample older than five seconds accepted")
	}
	short := []byte(fmt.Sprintf("2|retail|request_1|Player-1136-TEST|1473|nonce|%d|%d", now.Unix(), now.Unix()+1))
	if _, err := ParseReloadRequest(short, "retail", now.Add(time.Second)); err == nil {
		t.Fatal("expired lease accepted at exact expiry")
	}
	forever := []byte(strings.Replace(strings.Replace(string(valid), "|retail|", "|forever|", 1), "|1473|", "|0|", 1))
	if r, err := ParseReloadRequest(forever, "forever", now); err != nil || r == nil || r.Spec != 0 {
		t.Fatalf("Forever's zero specialization rejected: %#v %v", r, err)
	}
}

func TestReloadGateNeedsTwoStableSamplesAndConsumesNonceBeforeReturn(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	state := reloadTestState(now)
	body := reloadTestBody(now, "nonce_1")
	g := ReloadGate{Client: "retail"}
	for _, delay := range []time.Duration{0, 149 * time.Millisecond} {
		if r, err := g.Accept(body, state, now.Add(delay)); err != nil || r != nil {
			t.Fatalf("unconfirmed request became ready: %#v %v", r, err)
		}
	}
	if r, err := g.Accept(body, state, now.Add(150*time.Millisecond)); err != nil || r == nil {
		t.Fatalf("stable request not ready: %#v %v", r, err)
	}
	// No publication or input callback runs here. The attempt is already spent.
	for _, delay := range []time.Duration{200 * time.Millisecond, time.Second, 4 * time.Second} {
		if r, err := g.Accept(body, state, now.Add(delay)); err != nil || r != nil {
			t.Fatalf("consumed nonce became ready again: %#v %v", r, err)
		}
	}
	// A fresh lease cannot recycle a previously attempted nonce, even after the
	// original lease expires and the five-second rate limit has passed.
	later := now.Add(20 * time.Second)
	for _, delay := range []time.Duration{0, 200 * time.Millisecond} {
		if r, err := g.Accept(reloadTestBody(later, "nonce_1"), state, later.Add(delay)); err != nil || r != nil {
			t.Fatalf("renewing timestamps recycled a consumed nonce: %#v %v", r, err)
		}
	}
}

func TestReloadGateChecksAcceptedSnapshotJobAndCanonicalState(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	body := reloadTestBody(now, "nonce_1")
	for name, change := range map[string]func(*WorkspaceState){
		"workspace client": func(s *WorkspaceState) { s.Client = "forever" },
		"no snapshot":      func(s *WorkspaceState) { s.Snapshots = nil },
		"snapshot client":  func(s *WorkspaceState) { s.Snapshots[0].Snapshot.Client = "forever" },
		"snapshot ID":      func(s *WorkspaceState) { s.Snapshots[0].Snapshot.ID = "another" },
		"no request":       func(s *WorkspaceState) { s.Snapshots[0].Snapshot.GameRequest = nil },
		"request ID":       func(s *WorkspaceState) { s.Snapshots[0].Snapshot.GameRequest.ID = "another" },
		"invalid request":  func(s *WorkspaceState) { s.Snapshots[0].Snapshot.GameRequest.Task = "execute" },
		"character":        func(s *WorkspaceState) { s.Snapshots[0].Snapshot.Character.GUID = "Player-OTHER" },
		"specialization":   func(s *WorkspaceState) { s.Snapshots[0].Snapshot.Character.Spec = 1468 },
		"no job":           func(s *WorkspaceState) { s.Jobs = nil },
		"job request":      func(s *WorkspaceState) { s.Jobs[0].RequestID = "another" },
		"job snapshot":     func(s *WorkspaceState) { s.Jobs[0].SnapshotID = "another" },
		"job task":         func(s *WorkspaceState) { s.Jobs[0].GameTask = "upgrades" },
		"unknown state":    func(s *WorkspaceState) { s.Jobs[0].State = "approved" },
	} {
		t.Run(name, func(t *testing.T) {
			state := reloadTestState(now)
			change(&state)
			g := ReloadGate{Client: "retail"}
			if r, err := g.Accept(body, state, now); err == nil || r != nil {
				t.Fatalf("unrelated or unknown report accepted: %#v %v", r, err)
			}
		})
	}
	for _, status := range []string{"running", "cancelling", "complete", "failed", "cancelled", "interrupted"} {
		t.Run(status, func(t *testing.T) {
			state := reloadTestState(now)
			state.Jobs[0].State = status
			g := ReloadGate{Client: "retail"}
			if r, err := g.Accept(body, state, now); err != nil || r != nil {
				t.Fatalf("first valid report sample: %#v %v", r, err)
			}
			if r, err := g.Accept(body, state, now.Add(200*time.Millisecond)); err != nil || r == nil {
				t.Fatalf("canonical report cannot be imported: %#v %v", r, err)
			}
		})
	}
}

func TestReloadGateChangedInvalidAndDelayedSamplesResetConfirmation(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	state := reloadTestState(now)
	a, b := reloadTestBody(now, "nonce_a"), reloadTestBody(now, "nonce_b")
	g := ReloadGate{Client: "retail"}
	for i, body := range [][]byte{a, b, a} {
		if r, err := g.Accept(body, state, now.Add(time.Duration(i)*200*time.Millisecond)); err != nil || r != nil {
			t.Fatalf("alternating request samples combined: %#v %v", r, err)
		}
	}
	if r, err := g.Accept(a, state, now.Add(600*time.Millisecond)); err != nil || r == nil {
		t.Fatalf("stable pair after alternation not recognized: %#v %v", r, err)
	}
	for _, interruption := range []string{"delay", "malformed", "foreign job"} {
		g := ReloadGate{Client: "retail"}
		if _, err := g.Accept(a, state, now); err != nil {
			t.Fatal(err)
		}
		second := now.Add(1100 * time.Millisecond)
		if interruption != "delay" {
			badBody, badState := []byte("malformed"), reloadTestState(now)
			if interruption == "foreign job" {
				badBody = a
				badState.Jobs[0].SnapshotID = "another"
			}
			if r, err := g.Accept(badBody, badState, now.Add(100*time.Millisecond)); err == nil || r != nil {
				t.Fatal("invalid interruption accepted:", interruption)
			}
			second = now.Add(200 * time.Millisecond)
		}
		if r, err := g.Accept(a, state, second); err != nil || r != nil {
			t.Fatalf("stale or invalid pair was retained: %#v %v", r, err)
		}
		if r, err := g.Accept(a, state, second.Add(200*time.Millisecond)); err != nil || r == nil {
			t.Fatalf("new stable pair failed: %#v %v", r, err)
		}
	}
}

func TestReloadGateRateLimitSurvivesChangingNonce(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	state := reloadTestState(now)
	g := ReloadGate{Client: "retail"}
	a := reloadTestBody(now, "first")
	_, _ = g.Accept(a, state, now)
	if r, err := g.Accept(a, state, now.Add(200*time.Millisecond)); err != nil || r == nil {
		t.Fatalf("first attempt not ready: %#v %v", r, err)
	}
	for n := 1; n < 20; n++ {
		at := now.Add(time.Duration(n+1) * 200 * time.Millisecond)
		b := reloadTestBody(now, fmt.Sprintf("fresh_%d", n))
		for _, delay := range []time.Duration{0, 150 * time.Millisecond} {
			if r, err := g.Accept(b, state, at.Add(delay)); err != nil || r != nil {
				t.Fatalf("new nonce bypassed attempt rate limit: %#v %v", r, err)
			}
		}
	}
	later := now.Add(5200 * time.Millisecond)
	b := reloadTestBody(later, "after_cooldown")
	if r, err := g.Accept(b, state, later); err != nil || r != nil {
		t.Fatalf("new attempt skipped confirmation: %#v %v", r, err)
	}
	if r, err := g.Accept(b, state, later.Add(200*time.Millisecond)); err != nil || r == nil {
		t.Fatalf("rate limit failed to release: %#v %v", r, err)
	}
}
