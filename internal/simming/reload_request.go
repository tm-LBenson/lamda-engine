package simming

import (
	"fmt"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

type ReloadRequest = lookup.ReloadRequest

func ParseReloadRequest(body []byte, client string, now time.Time) (*ReloadRequest, error) {
	return lookup.ParseSimReloadRequest(body, client, now)
}

type ReloadGate struct {
	Client      string
	pending     string
	firstSeen   time.Time
	lastAttempt time.Time
	consumed    map[string]bool
}

func (g *ReloadGate) Accept(body []byte, state WorkspaceState, now time.Time) (*ReloadRequest, error) {
	request, err := ParseReloadRequest(body, g.Client, now)
	if err != nil {
		g.pending = ""
		return nil, err
	}
	validSnapshot := false
	task := ""
	if state.Client == g.Client {
		for _, summary := range state.Snapshots {
			s := summary.Snapshot
			if s.ID == request.RequestID && s.Client == g.Client && s.Character.GUID == request.GUID && s.Character.Spec == request.Spec && s.GameRequest != nil && s.GameRequest.ID == request.RequestID {
				if s.GameRequest.Validate(s.ID, s.Client) == nil {
					validSnapshot = true
					task = s.GameRequest.Task
				}
			}
		}
	}
	accepted := false
	if validSnapshot {
		for _, job := range state.Jobs {
			if job.RequestID == request.RequestID && job.SnapshotID == request.RequestID && job.GameTask == task {
				switch job.State {
				case "running", "cancelling", "complete", "failed", "cancelled", "interrupted":
					accepted = true
				}
			}
		}
	}
	if !accepted {
		g.pending = ""
		return nil, fmt.Errorf("reload request has no matching accepted character job")
	}
	if g.consumed[request.Nonce] {
		g.pending = ""
		return nil, nil
	}
	if len(g.consumed) >= 4096 {
		g.pending = ""
		return nil, fmt.Errorf("reload attempt cache is full")
	}
	if !g.lastAttempt.IsZero() && now.Sub(g.lastAttempt) < 5*time.Second {
		g.pending = ""
		return nil, nil
	}
	text := string(body)
	if g.pending != text || now.Before(g.firstSeen) || now.Sub(g.firstSeen) > time.Second {
		g.pending = text
		g.firstSeen = now
		return nil, nil
	}
	if now.Sub(g.firstSeen) < 150*time.Millisecond {
		return nil, nil
	}
	if g.consumed == nil {
		g.consumed = map[string]bool{}
	}
	// Consume before publishing or sending input, including failed attempts.
	g.consumed[request.Nonce] = true
	g.lastAttempt = now
	g.pending = ""
	return request, nil
}
