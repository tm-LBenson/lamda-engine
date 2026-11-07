package simming

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type auditTransport func(*http.Request) (*http.Response, error)

func (f auditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAutomaticAuditDisabledAndCancellationDoesNotMarkDayComplete(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	started := make(chan struct{}, 8)
	finished := make(chan struct{}, 8)
	http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-r.Context().Done()
		finished <- struct{}{}
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &Workspace{auditor: NewAuditor(t.TempDir())}
	s := &Service{Context: ctx, Workspace: w}
	s.SetAutomaticAudit(false)
	if s.auditCancel != nil {
		t.Fatal("disabled audit started")
	}
	s.SetAutomaticAudit(true)
	s.SetAutomaticAudit(true)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("enabled audit did not start")
	}
	s.SetAutomaticAudit(false)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("disabled audit did not cancel HTTP request")
	}
	deadline := time.Now().Add(time.Second)
	for {
		w.auditor.mu.Lock()
		busy := w.auditor.checking
		w.auditor.mu.Unlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("audit did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if !w.auditor.State().CheckedAt.IsZero() || len(started) != 0 {
		t.Fatal("cancelled audit scheduled more requests or marked day complete")
	}
}
