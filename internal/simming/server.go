package simming

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/addonstore"
	"github.com/tm-LBenson/lamda-engine/internal/augmentation"
	"github.com/tm-LBenson/lamda-engine/internal/kbb"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"github.com/tm-LBenson/lamda-engine/internal/overlays"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFiles embed.FS

type Service struct {
	MeterCommands chan lookup.MeterCommand
	MeterShares   chan struct{}
	summaryDone   chan struct{}
	auditMu       sync.Mutex
	auditCancel   context.CancelFunc
	Context       context.Context
	cancel        context.CancelFunc
	Augmentation  *augmentation.Store
	Workspace     *Workspace
	Addons        *addonstore.Store
	Overlays      *overlays.Store
	Planner       *kbb.Store
	URL           string
	server        *http.Server
	listener      net.Listener
}

func Listen(ctx context.Context, dir, client string, p Calculator) (*Service, error) {
	address := "127.0.0.1:47641"
	if client == "forever" {
		address = "127.0.0.1:47642"
	}
	l, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	w, err := NewWorkspace(ctx, dir, client, p)
	if err != nil {
		cancel()
		l.Close()
		return nil, err
	}
	planner, err := kbb.Open(filepath.Join(filepath.Dir(dir), "kbb"), client)
	if err != nil {
		cancel()
		l.Close()
		w.Close()
		return nil, err
	}
	overlayStore, err := overlays.Open(filepath.Join(filepath.Dir(dir), "overlays"))
	if err != nil {
		cancel()
		l.Close()
		w.Close()
		return nil, err
	}
	s := &Service{MeterCommands: make(chan lookup.MeterCommand, 4), MeterShares: make(chan struct{}, 1), Augmentation: augmentation.New(filepath.Join(filepath.Dir(dir), "augmentation", "reports.json")), Overlays: overlayStore, Planner: planner, Workspace: w, URL: "http://" + address, listener: l, Context: ctx, cancel: cancel}
	s.summaryDone = make(chan struct{})
	go func() { defer close(s.summaryDone); s.Augmentation.RunSummaryPreparation(ctx) }()
	s.server = &http.Server{Handler: s.Handler(address), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	go s.server.Serve(l)
	go overlayStore.RunMemory(ctx)
	return s, nil
}

// Automatic source monitoring follows the Sim module. Explicit audit requests
// in the browser remain available independently.
func (s *Service) SetAutomaticAudit(enabled bool) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	if !enabled {
		if s.auditCancel != nil {
			s.auditCancel()
			s.auditCancel = nil
		}
		return
	}
	if s.auditCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(s.Context)
	s.auditCancel = cancel
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			s.Workspace.auditor.Check(ctx, false)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) Close() {
	s.cancel()
	if s.summaryDone != nil {
		<-s.summaryDone
	}
	if s.Augmentation != nil {
		s.Augmentation.Close()
	}
	s.SetAutomaticAudit(false)
	s.server.Close()
	s.Workspace.Close()
	if s.Overlays != nil {
		_ = s.Overlays.FlushMemory()
	}
}

func jsonResponse(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	jsonResponse(w, map[string]string{"error": err.Error()})
}
func decodeRequest(w http.ResponseWriter, r *http.Request, value any) error {
	limit := int64(MaxSnapshot + 2048)
	if r.URL.Path == "/api/overlays" {
		limit = overlays.MaxRulesBytes
	}
	if r.URL.Path == "/api/overlays/media/import" {
		limit = 24 * 1024 * 1024
	}
	if r.URL.Path == "/api/kbb/edit" {
		limit = kbb.MaxFile
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("request must contain one JSON value")
	}
	return nil
}

func (s *Service) Handler(host string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		// Fixed Host defeats DNS rebinding; no CORS, no cookies or localhost
		// secrets in URLs. Mutations require a custom header and an allowed type.
		if r.Host != host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+host) {
			http.Error(w, "Local workspace only", http.StatusForbidden)
			return
		}
		contentType := r.Header.Get("Content-Type")
		mediaType := r.URL.Path == "/api/overlays/media" && (contentType == "image/png" || contentType == "image/jpeg" || contentType == "audio/wav")
		allowedType := mediaType || contentType == "application/json" || (r.URL.Path == "/api/overlays/backgrounds" && (contentType == "image/png" || contentType == "image/jpeg"))
		if r.Method != "GET" && (r.Method != "POST" || r.Header.Get("X-Lamda-Sim") != "1" || !allowedType) {
			http.Error(w, "Use the local workspace controls", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/overlays") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; media-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		}
		switch {
		case r.URL.Path == "/api/overlays/pack-history" || r.URL.Path == "/api/overlays/pack-history/delete" || r.URL.Path == "/api/overlays/pack-history/recover":
			s.overlayPackHistory(w, r)
		case r.URL.Path == "/api/addons":
			if s.Addons == nil {
				failure(w, 503, fmt.Errorf("Browse Addons is unavailable"))
				return
			}
			if r.Method == "POST" {
				var request struct {
					ID     string `json:"id"`
					Action string `json:"action"`
				}
				if err := decodeRequest(w, r, &request); err != nil {
					failure(w, 400, err)
					return
				}
				result, err := s.Addons.Apply(request.ID, request.Action)
				if err != nil {
					failure(w, 400, err)
					return
				}
				jsonResponse(w, result)
				return
			}
			if r.Method != "GET" {
				http.NotFound(w, r)
				return
			}
			jsonResponse(w, s.Addons.State())
		case r.URL.Path == "/api/overlays/memory" || r.URL.Path == "/api/overlays/memory/reset" || r.URL.Path == "/api/overlays/memory/recover":
			s.overlayMemory(w, r)
		case r.URL.Path == "/api/overlays/media" || strings.HasPrefix(r.URL.Path, "/api/overlays/media/"):
			s.overlayMedia(w, r)
		case r.URL.Path == "/api/overlays/icon" && r.Method == "GET":
			s.overlayIcon(w, r)
		case (r.URL.Path == "/api/overlays/spells" || r.URL.Path == "/api/overlays/spells/license") && r.Method == "GET":
			s.overlaySpells(w, r)
		case r.URL.Path == "/api/overlays/characters" && r.Method == "GET":
			s.overlayCharacters(w, r)
		case r.URL.Path == "/api/overlays/backgrounds" || r.URL.Path == "/api/overlays/backgrounds/remove" || r.URL.Path == "/api/overlays/background":
			s.overlayBackgrounds(w, r)
		case r.URL.Path == "/api/overlays/sound/sonar" && r.Method == "GET":
			if s.Overlays == nil {
				http.NotFound(w, r)
				return
			}
			p := s.Overlays.SoundPath("sonar")
			if p == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "audio/wav")
			http.ServeFile(w, r, p)
		case r.URL.Path == "/api/overlays":
			if s.Overlays == nil {
				failure(w, 503, fmt.Errorf("Overlays unavailable"))
				return
			}
			if r.Method == "POST" {
				var edit struct {
					Revision    int64           `json:"revision"`
					Rules       []overlays.Rule `json:"rules"`
					PackUpdates []string        `json:"packUpdates,omitempty"`
				}
				if err := decodeRequest(w, r, &edit); err != nil {
					failure(w, 400, err)
					return
				}
				if err := s.Overlays.SavePackUpdates(edit.Revision, edit.Rules, edit.PackUpdates); err != nil {
					code := 400
					if errors.Is(err, overlays.ErrConflict) {
						code = 409
					}
					failure(w, code, err)
					return
				}
			}
			jsonResponse(w, s.Overlays.State())
		case r.URL.Path == "/overlays" && r.Method == "GET":
			http.Redirect(w, r, "/overlays/", http.StatusTemporaryRedirect)
		case strings.HasPrefix(r.URL.Path, "/overlays/") && r.Method == "GET":
			http.StripPrefix("/overlays/", overlays.Files()).ServeHTTP(w, r)

		case r.URL.Path == "/api/kbb" && r.Method == "GET":
			if s.Planner == nil {
				failure(w, 503, fmt.Errorf("planner unavailable"))
				return
			}
			jsonResponse(w, s.Planner.State())
		case r.URL.Path == "/api/kbb/edit" && r.Method == "POST":
			if s.Planner == nil {
				failure(w, 503, fmt.Errorf("planner unavailable"))
				return
			}
			var edit kbb.Edit
			if err := decodeRequest(w, r, &edit); err != nil {
				failure(w, 400, err)
				return
			}
			if err := s.Planner.Edit(edit, time.Now()); err != nil {
				code := 400
				if errors.Is(err, kbb.ErrConflict) {
					code = 409
				}
				failure(w, code, err)
				return
			}
			jsonResponse(w, s.Planner.State())
		case r.Method == "POST" && r.URL.Path == "/api/plan":
			var request struct {
				ID      string      `json:"id"`
				Options PlanOptions `json:"options"`
			}
			if err := decodeRequest(w, r, &request); err != nil {
				failure(w, 400, err)
				return
			}
			plan, err := s.Workspace.Plan(request.ID, request.Options)
			if err != nil {
				failure(w, 400, err)
				return
			}
			jsonResponse(w, plan)
		case r.Method == "GET" && r.URL.Path == "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "POST" && r.URL.Path == "/api/augmentation/share":
			if s.MeterShares == nil {
				failure(w, 503, fmt.Errorf("native summary unavailable"))
				return
			}
			select {
			case s.MeterShares <- struct{}{}:
			default:
			}
			jsonResponse(w, map[string]bool{"requested": true})
		case r.Method == "POST" && r.URL.Path == "/api/augmentation/navigation":
			if s.MeterCommands == nil {
				failure(w, 503, fmt.Errorf("native meter navigation unavailable"))
				return
			}
			var command lookup.MeterCommand
			if err := decodeRequest(w, r, &command); err != nil || !command.Valid() {
				failure(w, 400, fmt.Errorf("invalid meter navigation"))
				return
			}
			select {
			case s.MeterCommands <- command:
				jsonResponse(w, map[string]bool{"requested": true})
			default:
				failure(w, 409, fmt.Errorf("meter navigation is busy"))
			}
		case r.Method == "GET" && r.URL.Path == "/api/augmentation":
			if s.Augmentation == nil {
				failure(w, 503, fmt.Errorf("Augmentation review unavailable"))
				return
			}
			jsonResponse(w, s.Augmentation.Snapshot())
		case r.Method == "GET" && r.URL.Path == "/api/augmentation/breakdown":
			if s.Augmentation == nil {
				failure(w, 503, fmt.Errorf("meter breakdown unavailable"))
				return
			}
			q := r.URL.Query()
			breakdown, err := s.Augmentation.DamageBreakdown(q.Get("owner"), q.Get("segment"), q.Get("actor"), q.Get("spell"))
			if err != nil {
				failure(w, 404, err)
				return
			}
			jsonResponse(w, breakdown)
		case r.Method == "GET" && r.URL.Path == "/api/state":
			jsonResponse(w, s.Workspace.State())
		case r.URL.Path == "/api/sim/workflow" || r.URL.Path == "/api/sim/workflow/run" || r.URL.Path == "/api/sim/decisions":
			s.workflowHTTP(w, r)
		case r.Method == "GET" && r.URL.Path == "/api/snapshot":
			snapshot, err := s.Workspace.Snapshot(r.URL.Query().Get("id"))
			if err != nil {
				failure(w, 404, err)
				return
			}
			jsonResponse(w, snapshot)
		case r.Method == "GET" && r.URL.Path == "/api/result":
			id := r.URL.Query().Get("id")
			if !requestID.MatchString(id) {
				http.NotFound(w, r)
				return
			}
			data, err := readBounded(filepath.Join(s.Workspace.dir, "jobs", id, "job.json"), 2*1024*1024)
			if err != nil {
				failure(w, 404, fmt.Errorf("result unavailable"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", `attachment; filename="lamda-sim-`+id+`.json"`)
			w.Write(data)
		case r.Method == "GET" && r.URL.Path == "/api/log":
			id := r.URL.Query().Get("id")
			if !requestID.MatchString(id) {
				http.NotFound(w, r)
				return
			}
			data, err := readBounded(filepath.Join(s.Workspace.dir, "jobs", id, "worker.log"), 2*1024*1024)
			if err != nil {
				failure(w, 404, fmt.Errorf("worker log unavailable"))
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write(data)
		case r.Method == "POST" && r.URL.Path == "/api/import":
			var snapshot Snapshot
			if err := decodeRequest(w, r, &snapshot); err != nil {
				failure(w, 400, err)
				return
			}
			if time.Since(time.Unix(snapshot.CapturedAt, 0)) > 24*time.Hour || time.Until(time.Unix(snapshot.CapturedAt, 0)) > 30*time.Second {
				failure(w, 400, fmt.Errorf("export a fresh character snapshot before importing"))
				return
			}
			var err error
			if snapshot.GameRequest != nil {
				_, err = s.Workspace.ReceiveGame(&snapshot)
			} else {
				_, err = s.Workspace.Receive(&snapshot)
			}
			if err != nil {
				failure(w, 400, err)
				return
			}
			jsonResponse(w, map[string]string{"id": snapshot.ID})
		case r.Method == "POST" && r.URL.Path == "/api/run":
			var request struct {
				ID       string   `json:"id"`
				Scenario Scenario `json:"scenario"`
				Selected []string `json:"selected"`
			}
			if err := decodeRequest(w, r, &request); err != nil {
				failure(w, 400, err)
				return
			}
			job, err := s.Workspace.Start(request.ID, request.Scenario, request.Selected)
			if err != nil {
				failure(w, 400, err)
				return
			}
			jsonResponse(w, job)
		case r.Method == "POST" && r.URL.Path == "/api/cancel":
			var request struct {
				ID string `json:"id"`
			}
			if err := decodeRequest(w, r, &request); err != nil {
				failure(w, 400, err)
				return
			}
			if err := s.Workspace.Cancel(request.ID); err != nil {
				failure(w, 400, err)
				return
			}
			jsonResponse(w, map[string]bool{"ok": true})
		case r.Method == "POST" && r.URL.Path == "/api/review":
			var request struct {
				ID    string `json:"id"`
				Notes string `json:"notes"`
			}
			if err := decodeRequest(w, r, &request); err != nil {
				failure(w, 400, err)
				return
			}
			review, err := s.Workspace.RecordReview(request.ID, request.Notes)
			if err != nil {
				failure(w, 400, err)
				return
			}
			jsonResponse(w, review)
		case r.Method == "POST" && r.URL.Path == "/api/audit":
			go s.Workspace.auditor.Check(s.Workspace.ctx, true)
			jsonResponse(w, map[string]bool{"requested": true})
		case r.Method == "GET" && (r.URL.Path == "/" || r.URL.Path == "/app.js" || r.URL.Path == "/style.css" || r.URL.Path == "/kbb" || r.URL.Path == "/kbb.js" || r.URL.Path == "/kbb.css" || r.URL.Path == "/addons" || r.URL.Path == "/addons.js" || r.URL.Path == "/addons.css" || r.URL.Path == "/augmentation" || r.URL.Path == "/augmentation.js" || r.URL.Path == "/augmentation.css"):
			name := r.URL.Path[1:]
			mime := "text/html; charset=utf-8"
			if name == "" {
				name = "index.html"
			}
			if name == "kbb" {
				name = "kbb.html"
			}
			if name == "augmentation" {
				name = "augmentation.html"
			}
			if name == "addons" {
				name = "addons.html"
			}
			if name == "app.js" || name == "kbb.js" || name == "addons.js" || name == "augmentation.js" {
				mime = "text/javascript; charset=utf-8"
			}
			if name == "style.css" || name == "kbb.css" || name == "addons.css" || name == "augmentation.css" {
				mime = "text/css; charset=utf-8"
			}
			data, err := webFiles.ReadFile("web/" + name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", mime)
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	})
}
