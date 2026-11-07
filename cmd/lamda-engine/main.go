package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/arena"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

var version = "0.7.3"

type install struct {
	Retail string `json:"retail"`
	Config string `json:"config"`
}
type state struct {
	Version      string                    `json:"version"`
	Layout       engine.Layout             `json:"layout"`
	PID          int                       `json:"pid"`
	Client       string                    `json:"client"`
	Config       engine.Config             `json:"config"`
	Rows         []engine.Row              `json:"rows"`
	Update       string                    `json:"update,omitempty"`
	Inference    *engine.InferenceSnapshot `json:"inference,omitempty"`
	InputStatus  string                    `json:"inputStatus,omitempty"`
	LastInputAt  *time.Time                `json:"lastInputAt,omitempty"`
	Lookups      lookup.Status             `json:"lookups"`
	LookupBridge string                    `json:"lookupBridge"`
	Estimates    engine.EstimatePanel      `json:"estimates"`
	FrameDisplay engine.FrameDisplay       `json:"frameDisplay"`
	Arena        arena.Status              `json:"arena"`
}

func main() {
	client := flag.String("client", "retail", "Game client: retail or forever")
	wowPath := flag.String("wow", "", "Selected WoW client directory")
	retail := flag.String("retail", "", "WoW _retail_ directory")
	saved := flag.String("config", "", "lamdaUI.lua SavedVariables path")
	replay := flag.String("replay", "", "Replay a combat log to stdout without starting engine")
	audit := flag.String("audit", "", "Replay a combat log into an inference and data-gap JSON report")
	replayDelay := flag.Duration("replay-delay", 0, "Simulated delivery delay for --audit (e.g. 15s)")
	flag.Bool("no-overlay", true, "Compatibility option; detached cooldown panels remain disabled")
	flag.Bool("solo-mvp", false, "Compatibility option for frame-only MVP collection")
	demo := flag.Bool("demo", false, "Run a headless fictional simulation without reading WoW or querying profiles")
	stateDir := flag.String("state-dir", "", "Store this run's state and diagnostics in a separate directory")
	simWorkspace := flag.Bool("sim-workspace", false, "Run only the local sim workspace, without WoW capture or tracking")
	ver := flag.Bool("version", false, "Show version")
	flag.Parse()
	if err := engine.ConfigureClient(*client); err != nil {
		log.Fatal(err)
	}
	if *wowPath != "" {
		*retail = *wowPath
	}
	if *ver {
		fmt.Println(version)
		return
	}
	if *replay != "" {
		if e := runReplay(*replay); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *audit != "" {
		if *replayDelay < 0 || *replayDelay > 24*time.Hour {
			log.Fatal("replay delay must be between 0 and 24h")
		}
		if e := runAudit(*audit, *replayDelay, os.Stdout); e != nil {
			log.Fatal(e)
		}
		return
	}
	home, e := os.UserConfigDir()
	if e != nil {
		log.Fatal(e)
	}
	dir := filepath.Join(home, "LamdaUI")
	if engine.Forever {
		dir = filepath.Join(dir, "Forever")
	}
	if *stateDir != "" {
		dir = *stateDir
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		log.Fatal(e)
	}
	logfile, e := os.OpenFile(filepath.Join(dir, "engine.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		log.Fatal(e)
	}
	defer logfile.Close()
	log.SetOutput(logfile)
	if *demo {
		if err := runDemo(dir); err != nil {
			log.Print(err)
		}
		return
	}
	if *simWorkspace {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		service, err := startSimWorkspace(ctx, dir, *client, "")
		if err != nil {
			log.Fatal(err)
		}
		defer service.Close()
		fmt.Println(service.URL)
		<-ctx.Done()
		return
	}
	// A loopback bind is used only as a singleton lock. No service or commands exposed.
	port := "127.0.0.1:47639"
	if engine.Forever {
		port = "127.0.0.1:47640"
	}
	lock, e := net.Listen("tcp4", port)
	if e != nil {
		log.Print("Engine already running or singleton port unavailable")
		return
	}
	defer lock.Close()
	var ins install
	if b, e := os.ReadFile(filepath.Join(dir, "install.json")); e == nil {
		_ = json.Unmarshal([]byte(strings.TrimPrefix(string(b), "\ufeff")), &ins)
	}
	if *retail == "" {
		*retail = ins.Retail
	}
	if *saved == "" {
		*saved = ins.Config
	}
	if *retail == "" {
		log.Fatal("WoW path missing; rerun installer with -WowPath")
	}
	if engine.Forever {
		executable := "WowB.exe"
		if strings.EqualFold(filepath.Base(filepath.Clean(*retail)), "_forever_") {
			executable = "Wow.exe"
		}
		if _, err := os.Stat(filepath.Join(*retail, executable)); err != nil {
			log.Fatalf("Forever client requires %s in --wow directory", executable)
		}
		if *saved != "" {
			rel, err := filepath.Rel(filepath.Join(*retail, "WTF", "Account"), *saved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
				log.Fatal("SavedVariables must belong to the selected Forever client")
			}
		}
	}
	statePath := filepath.Join(dir, "state.json")
	defer os.Remove(statePath)
	frameStatePath := filepath.Join(dir, "frame-display.json")
	defer os.Remove(frameStatePath)
	cfg := engine.DefaultClientConfig()
	model := engine.NewModel()
	history := newTrackingHistory(dir)
	history.Record(map[string]any{"kind": "engine_start", "version": version, "client": *client})
	model.TrackingAudit = func(e engine.TrackingEvent) { history.Record(e) }
	defer func() {
		history.Record(map[string]any{"kind": "engine_stop", "diagnostics": model.Diagnostics})
		if err := history.Flush(); err != nil {
			log.Printf("Tracking history: %v", err)
		}
	}()
	defer func() {
		if err := engine.WriteJSON(filepath.Join(dir, "diagnostics.json"), model.Diagnostics); err != nil {
			log.Printf("Final diagnostics write: %v", err)
		}
	}()
	tail := &combatInput{}
	overlayLogActive := false
	augLogActive := false
	augWindow := &lookup.WindowTracker{}
	session := &engine.LogSession{}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go lookup.RunQueueAccept(ctx, *retail, *client, log.Printf)
	simService, simError := startSimWorkspace(ctx, dir, *client, *retail, *saved)
	if simError != nil {
		log.Printf("Sim workspace unavailable: %v", simError)
	} else {
		defer simService.Close()
		if simService.Addons != nil {
			cfg = applyModuleInventory(cfg, simService.Addons)
		}
	}
	var meterShares <-chan struct{}
	var meterCommands <-chan lookup.MeterCommand
	if simService != nil {
		meterShares = simService.MeterShares
		meterCommands = simService.MeterCommands
	}
	meterSamples := map[int]lookup.BridgeSample{}
	meterSampleSeen := map[int]time.Time{}
	var summarySample lookup.BridgeSample
	var summarySeen time.Time
	summaryTokens := map[string]bool{}
	summaryBusy := false
	type summaryDelivery struct {
		token, text string
		err         error
	}
	summaryDone := make(chan summaryDelivery, 1)
	summaryAttempts := map[string]int{}
	summaryTextCache := map[string]string{}
	var summaryRetryAt time.Time
	lastSummaryOpen := time.Time{}
	lookups := lookup.NewQueue(ctx, lookup.NewRaiderIO())
	ownGUID := ""
	lookups.RosterGrace = 15 * time.Second
	lookups.LocalBuild = func(guid string, now time.Time) bool { return guid == ownGUID || model.HasInspectedBuild(guid, now) }
	lookups.SetEnabled(cfg.PublicLookups)
	lookups.SetScanPolicy(cfg.ScanTiming, cfg.ScanMinutes)
	defer lookups.Clear()
	defer func() {
		if err := engine.WriteJSON(filepath.Join(dir, "lookup-diagnostics.json"), lookups.Status()); err != nil {
			log.Printf("Final lookup diagnostics write: %v", err)
		}
	}()
	var bridge <-chan lookup.BridgeSample
	var roster map[string]bool
	var castRosterEvent *lookup.Event
	var observations observationInput
	panels := map[string]capturePanel{}
	var displays bridgeDisplayCache
	var screenPanel *lookup.ScreenCasts
	var screenPanelSeen time.Time
	var capture moduleCapture
	defer capture.Close()
	bridgeStatus := "disabled"
	var bridgeSeen time.Time
	var lastBridgeID string
	bridgeExpired := false
	var focus displayFocus
	var opponents arena.Model
	var stopDisplay context.CancelFunc
	var displayDone <-chan error
	var displayRetry time.Time
	defer func() {
		if stopDisplay != nil {
			stopDisplay()
		}
	}()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	tick := 0
	nextUpdate := time.Time{}
	update := ""
	notifiedUpdate := ""
	updateResults := make(chan string, 1)
	checking := false
	var lastInput time.Time
	inputStatus := ""
	configStatus := ""
	log.Printf("Lamda Engine %s started", version)
	for {
		availableMeterCommands := meterCommands
		if summaryBusy {
			availableMeterCommands = nil
		}
		select {
		case <-ctx.Done():
			return
		case result := <-summaryDone:
			summaryBusy = false
			if simService != nil {
				phase := "acknowledged"
				if result.err != nil {
					phase = "failed"
				}
				simService.Augmentation.NoteShare(phase, result.token, result.err)
			}
			if result.text != "" {
				summaryTextCache[result.token] = result.text
			}
			if result.err == nil && result.token != "" {
				summaryTokens[result.token] = true
			}
			if result.err != nil {
				summaryRetryAt = time.Now().Add(time.Second)
				log.Printf("Meter summary: %v", result.err)
			}
		case <-meterShares:
			if summaryBusy || time.Since(lastSummaryOpen) < 3*time.Second || time.Since(summarySeen) > 2*time.Second {
				continue
			}
			summaryBusy = true
			lastSummaryOpen = time.Now()
			sample := summarySample
			go func() {
				summaryDone <- summaryDelivery{err: lookup.DeliverMeterSummary(ctx, sample, *retail, *client, "")}
			}()
		case command := <-availableMeterCommands:
			if summaryBusy || time.Since(meterSampleSeen[command.WindowID]) > 2*time.Second {
				continue
			}
			sample, ok := meterSamples[command.WindowID]
			if !ok {
				continue
			}
			summaryBusy = true
			debugNavigation := cfg.AugDebug
			go func() {
				started := time.Now()
				commandCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
				defer cancel()
				err := lookup.DeliverMeterNavigation(commandCtx, sample, *retail, *client, command)
				if debugNavigation {
					log.Printf("Meter navigation: window=%d action=%s elapsed=%s result=%v", command.WindowID, command.Action, time.Since(started).Round(time.Millisecond), err)
				}
				summaryDone <- summaryDelivery{err: err}
			}()
		case u := <-updateResults:
			checking = false
			update = u
			if cfg.Updates && cfg.Notify && u != "" && u != notifiedUpdate {
				notifiedUpdate = u
				go func(tag string) {
					if err := notifyUpdate(ctx, tag); err != nil {
						log.Printf("Update notification: %v", err)
					}
				}(u)
			}
		case sample, ok := <-bridge:
			if len(sample.Sim) != 0 {
				continue
			}
			if !ok {
				capture.Close()
				bridge = nil
				focus.pause("bridge_closed", time.Now())
				displays.clear()
				castRosterEvent = nil
				opponents.Pause()
				continue
			}
			if sample.Source == "augmentation" {
				if simService != nil && cfg.AugEnabled && !engine.Forever && !sample.Forever {
					simService.Augmentation.ObserveNativeSlot(sample.NativeMeter, sample.MeterSlot, time.Now(), sample.Error)
					if n := sample.NativeMeter; n != nil && n.Captured && n.Frame != nil && n.Frame.WindowID > 0 && sample.Error == "" {
						meterSamples[n.Frame.WindowID] = sample
						meterSampleSeen[n.Frame.WindowID] = time.Now()
					}
					if sample.MeterSlot == 0 {
						summarySample = sample
						summarySeen = time.Now()
					}
					if n := sample.NativeMeter; n != nil && n.ShareRequest != nil && n.ShareRequest.FocusBlocked {
						simService.Augmentation.NoteShare("waiting_for_focus", n.ShareRequest.Token, nil)
					}
					if n := sample.NativeMeter; n != nil && n.ShareRequest != nil && n.ShareRequest.TransferVersion <= 4 && !n.ShareRequest.FocusBlocked && n.ShareRequest.ReceivedBytes == 0 && sample.Error == "" && !summaryBusy && !summaryTokens[n.ShareRequest.Token] && summaryAttempts[n.ShareRequest.Token] < 3 && !time.Now().Before(summaryRetryAt) {
						if len(summaryAttempts) > 128 {
							summaryAttempts = map[string]int{}
							summaryTokens = map[string]bool{}
							summaryTextCache = map[string]string{}
						}
						summaryAttempts[n.ShareRequest.Token]++
						simService.Augmentation.NoteShare("loading", n.ShareRequest.Token, nil)
						cachedText := summaryTextCache[n.ShareRequest.Token]
						summaryBusy = true
						go func(sample lookup.BridgeSample) {
							r := sample.NativeMeter.ShareRequest
							text := cachedText
							started := time.Now()
							readyAt := started
							segment := ""
							if text == "" {
								reportCtx, cancelReport := context.WithTimeout(ctx, 15*time.Second)
								expectedEnd := time.Time{}
								if r.EndedAt > 0 {
									expectedEnd = time.Unix(r.EndedAt, 0)
								}
								summary, err := simService.Augmentation.SummaryForBoundaryWhenReady(reportCtx, r.Scope, sample.NativeMeter.Player, expectedEnd, r.EncounterID, r.Options)
								readyAt = time.Now()
								segment = summary.Segment
								cancelReport()
								text = summary.Text
								if err != nil {
									text = "[LM?] " + err.Error()
									if len(text) > 255 {
										text = "[LM?] The summary is not ready. Try again after the logs finish."
									}
								}
							}
							deliveryErr := lookup.DeliverMeterSummary(ctx, sample, *retail, *client, text)
							simService.Augmentation.NoteShareTiming(r.Token, segment, started, readyAt, time.Now(), r.EndedAt, len(text), r.Options, deliveryErr)
							summaryDone <- summaryDelivery{token: r.Token, text: text, err: deliveryErr}
						}(sample)
					}
					if sample.NativeMeter != nil {
						key := sample.Source
						if sample.MeterSlot > 0 {
							key += strconv.Itoa(sample.MeterSlot)
						}
						panels[key] = capturePanel{position: sample.Position, native: sample.NativeMeter, seen: time.Now()}
					}
				}
				continue
			}
			displayStatus := displays.observe(sample, engine.Forever, time.Now())
			if displayStatus == displaySampleTransientFailure {
				// Retain the last valid layout and native-band capture hole until
				// their normal FrameLifetime leases expire. Observation onset
				// confirmation remains paused independently for safety.
				switch sample.Source {
				case "cd":
					observations.pauseSource("cd")
				case "kick":
					observations.pauseSource("kick")
				case "gladius":
					opponents.Pause()
				}
				continue
			}
			if sample.Error != "" {
				panels = map[string]capturePanel{}
				wasHolding := focus.holding
				observations.pauseSource("")
				if !focus.pause(sample.Error, time.Now()) {
					opponents.Pause()
					screenPanel = nil
				} else if !wasHolding && !opponents.CanHold(time.Now()) {
					opponents.Pause()
				}
				castRosterEvent = nil
				if sample.Error != bridgeStatus {
					lookups.Fail(sample.Error)
				}
				bridgeStatus = sample.Error
				continue
			}
			if sample.Forever != engine.Forever {
				opponents.Pause()
				focus.pause("wrong_client_packet", time.Now())
				screenPanel = nil
				observations.pauseSource("")
				bridgeStatus = "wrong_client_packet"
				castRosterEvent = nil
				continue
			}
			if o := sample.Observations; o != nil && (o.Screen != nil || o.Auras != nil) {
				panels[sample.Source] = capturePanel{position: sample.Position, screen: o.Screen, auras: o.Auras, seen: time.Now()}
			}
			if sample.Arena != nil {
				focus.observe(sample)
				if cfg.ArenaTracking && !engine.Forever {
					opponents.Handle(sample, time.Now())
				} else {
					opponents.Reset()
				}
				if sample.Observations != nil && sample.Observations.Screen != nil {
					screenPanel = sample.Observations.Screen
					screenPanelSeen = time.Now()
				}
				bridgeStatus = "receiving"
				bridgeSeen = sample.Event.At
				bridgeExpired = false
				continue
			}
			castRosterEvent = &sample.Event
			focus.observe(sample)
			screenPanel = nil
			if sample.Observations != nil {
				screenPanel = sample.Observations.Screen
				screenPanelSeen = time.Now()
			}
			// Connection status must recover on a valid retained packet after
			// alt-tab, independently of event deduplication for talent requests.
			age := time.Since(sample.Event.At)
			if age >= -5*time.Second && age <= 30*time.Second {
				bridgeStatus = "receiving"
				bridgeSeen = sample.Event.At
				bridgeExpired = false
			} else {
				bridgeStatus = "stale"
			}
			if cfg.TrackingEnabled {
				model.DisabledDefensives, model.DisabledInterrupts = !cfg.DefensiveTracking, !cfg.KickEnabled
				observations.handle(model, sample, time.Now())
			}
			if sample.Frames != nil {
				ownGUID = sample.Frames.Player
			}
			if sample.Kick != nil {
				ownGUID = sample.Kick.Player
			}
			if sample.Event.ID == lastBridgeID {
				continue
			}
			lastBridgeID = sample.Event.ID
			if cfg.PublicLookups {
				for _, p := range lookups.Handle(sample.Event, time.Now()) {
					model.ObservePublicBuild(p)
				}
				roster = lookups.Roster()
			}
		case now := <-ticker.C:
			tick++
			if tick%8 == 1 {
				p := savedConfigPath(*saved, *retail)
				status := "missing_config"
				if p != "" {
					if c, e := engine.ReadCachedConfig(p); e == nil {
						status = "ok"
						c = engine.LimitForeverConfig(c)
						if simService != nil && simService.Addons != nil {
							c = applyModuleInventory(c, simService.Addons)
						}
						if cfg.TrackingEnabled != c.TrackingEnabled || cfg.DefensiveTracking != c.DefensiveTracking || cfg.KickEnabled != c.KickEnabled || cfg.Companions != c.Companions {
							model.Reset()
						}
						if cfg.TrackingEnabled != c.TrackingEnabled || cfg.PublicLookups != c.PublicLookups || cfg.LookupBridge != c.LookupBridge {
							lookups.Clear()
							model.PublicBuilds = map[string]lookup.Profile{}
							roster = nil
							lastBridgeID = ""
						}

						cfg = c
						lookups.SetEnabled(cfg.PublicLookups)
						lookups.SetScanPolicy(cfg.ScanTiming, cfg.ScanMinutes)
						if simService != nil {
							simService.Overlays.SetCaptureEnabled(cfg.OverlaysEnabled)
						}
					} else {
						status = "unreadable_config"
					}
				}
				if status != configStatus && status != "ok" {
					model.RecordGap(status, now, engine.Cast{})
				}
				configStatus = status
			}
			// Retired profile keys cannot re-enable a detached cooldown display.
			cfg.EstimatePanel = false
			targets := trackingStrips(cfg)
			bridgeEnabled := len(targets) > 0
			if capture.Configure(ctx, targets, *retail, *client) {
				bridge = capture.samples
				panels = map[string]capturePanel{}
				displays.clear()
				castRosterEvent = nil
				screenPanel = nil
				observations.pauseSource("")
				opponents.Reset()
				bridgeExpired = false
				lastBridgeID = ""
				if bridgeEnabled {
					focus.pause("bridge_start", now)
					bridgeStatus = "waiting"
					bridgeSeen = now
				} else {
					focus.pause("bridge_disabled", now)
					bridgeStatus = "disabled"
					lookups.Clear()
					roster = nil
				}
			}
			if bridgeEnabled && now.Sub(bridgeSeen) > 30*time.Second && !bridgeExpired {
				lookups.Fail("bridge_stale")
				bridgeStatus = "stale"
				bridgeExpired = true
				lookups.Clear()
				roster = nil
				castRosterEvent = nil
			}
			lookups.SetInterval(time.Duration(cfg.LookupInterval) * time.Second)
			for _, p := range lookups.Advance(now) {
				model.ObservePublicBuild(p)
			}
			if cfg.Updates && !checking && now.After(nextUpdate) {
				checking = true
				nextUpdate = now.Add(time.Duration(cfg.Days) * 24 * time.Hour)
				go func() { updateResults <- checkUpdate(ctx) }()
			}
			overlayLogNeeded := cfg.OverlaysEnabled && simService != nil && !engine.Forever && simService.Overlays.NeedsCombatLog(now)
			augLogNeeded := cfg.AugEnabled && !engine.Forever && simService != nil
			if simService != nil {
				simService.Augmentation.SetEnabled(augLogNeeded)
				simService.Augmentation.SetDebug(augLogNeeded && cfg.AugDebug)
			}
			logNeeded := cfg.TrackingEnabled || cfg.ArenaTracking || overlayLogNeeded || augLogNeeded
			generation := tail.Generation
			lines, e := tail.Poll(filepath.Join(*retail, "Logs"), logNeeded)
			logReceivedAt := time.Now()
			if augLogNeeded && tail.Info != nil {
				simService.Augmentation.ObserveLogRead(tail.Info.Size(), tail.Offset, len(lines), logReceivedAt)
			}
			if !logNeeded {
				lastInput = time.Time{}
			}
			if overlayLogNeeded && (!overlayLogActive || generation != tail.Generation) {
				offset := tail.StartOffset
				if generation == tail.Generation {
					offset = tail.Offset
				}
				if err := simService.Overlays.SeedCombatLog(tail.Path, offset, now); err != nil {
					log.Printf("Overlay log metadata: %v", err)
				}
			}
			overlayLogActive = overlayLogNeeded
			if e != nil && tick%40 == 1 {
				log.Printf("Log read: %v", e)
			}
			if augLogNeeded && (!augLogActive || generation != tail.Generation || e != nil) {
				simService.Augmentation.ResetInput(tail.Path)
			}
			augLogActive = augLogNeeded
			if logNeeded && generation != tail.Generation {
				history.Record(map[string]any{"kind": "log_generation", "path": tail.Path, "startOffset": tail.StartOffset})
				model.ResetLog(time.Time{})
				model.LogVersion, model.GameBuild = 0, ""
				lastInput = time.Time{}
				session = &engine.LogSession{}
				if err := session.Seed(tail.Path, tail.StartOffset); err != nil {
					log.Printf("Log session metadata: %v", err)
					model.RecordGap("session_metadata_read_failed", now, engine.Cast{})
				}
				if tail.StartOffset > 0 {
					model.RecordGap("history_skipped", now, engine.Cast{})
				}
			}
			if len(lines) > 0 {
				lastInput = now
			}
			nextInputStatus := "receiving"
			switch {
			case !cfg.TrackingEnabled:
				nextInputStatus = "disabled"
			case e != nil:
				nextInputStatus = "read_error"
			case tail.Path == "":
				nextInputStatus = "waiting_for_log"
			case lastInput.IsZero() || now.Sub(lastInput) > 30*time.Second:
				nextInputStatus = "idle"
			}
			if nextInputStatus != inputStatus && nextInputStatus != "receiving" && nextInputStatus != "disabled" {
				model.RecordGap("input_"+nextInputStatus, now, engine.Cast{})
			}
			inputStatus = nextInputStatus
			castRoster := rosterForCasts(castRosterEvent, now)
			for _, line := range lines {
				if augLogNeeded {
					simService.Augmentation.Ingest(line, logReceivedAt)
				}
				if overlayLogNeeded {
					simService.Overlays.IngestCombatLog(line, now)
				}
				if cfg.ArenaTracking && !engine.Forever {
					opponents.IngestLog(line, now)
				}
				if session.ResetFor(line) {
					at, event, err := engine.RecordHeader(line)
					if err == nil {
						before := model.Snapshot(now)
						history.Record(map[string]any{"kind": "dungeon_boundary", "event": event, "eventAt": at,
							"receivedAt": now, "roster": castRoster, "landed": before.InterruptCounts, "attempts": before.InterruptAttempts})
						// Count boundaries remain valid even when the log is too
						// delayed to establish initial cooldown readiness for a key.
						if event == "ZONE_CHANGE" || event == "CHALLENGE_MODE_START" {
							model.ResetInterruptCounts(at)
						}
						if event == "CHALLENGE_MODE_START" {
							model.BeginFreshKey(at, now, castRoster)
						} else {
							if event == "CHALLENGE_MODE_END" {
								model.EndFreshKey()
							}
							model.ResetLog(at)
						}
					}
				}
				if !cfg.TrackingEnabled {
					continue
				}
				model.DisabledDefensives, model.DisabledInterrupts = !cfg.DefensiveTracking, !cfg.KickEnabled
				model.IngestTeam(line, now, cfg.Companions, castRoster, displays.kick, displays.kickSeen)
			}
			shownUpdate := ""
			if cfg.Updates && cfg.Notify {
				shownUpdate = update
			}
			current := buildState(os.Getpid(), cfg, displayRows(model, cfg, now), shownUpdate)
			current.Client = *client
			inference := model.Snapshot(now)
			current.Inference, current.InputStatus = &inference, inputStatus
			current.Lookups, current.LookupBridge = lookups.Status(), bridgeStatus
			current.Estimates = engine.PanelFor(inference, cfg, inputStatus, lastInput, roster, now)
			current.FrameDisplay = focus.view(inference, cfg, displays.geometry, displays.geometrySeen, displays.geometryWindow, displays.kick, displays.kickSeen, displays.kickWindow, now)
			if cfg.ArenaTracking && !engine.Forever {
				current.FrameDisplay = arena.Merge(current.FrameDisplay, opponents.View(now, focus.holding))
				if current.FrameDisplay.Enabled && !focus.holding {
					focus.lastVisible = now
				}
			} else {
				opponents.Reset()
			}
			current.Arena = opponents.Status()
			if cfg.ColorStripVersion != 2 && screenPanel != nil && (focus.holding || now.Sub(screenPanelSeen) <= engine.FrameLifetime) {
				d := &current.FrameDisplay
				x := max(0, min(cfg.LookupBridgeX, d.Width-128))
				y := max(0, min(cfg.LookupBridgeY, d.Height-16))
				engine.ReserveCaptureArea(d, x, y+screenPanel.Offset, 128, screenPanel.Height())
			}
			if simService != nil && cfg.OverlaysEnabled {
				current.FrameDisplay = simService.Overlays.Merge(current.FrameDisplay, now)
			}
			if augLogNeeded {
				if window, ok := augWindow.Context(*retail, *client); ok {
					current.FrameDisplay = simService.Augmentation.Merge(current.FrameDisplay, cfg, window, now)
				}
			}
			reserveModuleStrips(&current.FrameDisplay, cfg, panels, now, focus.holding)
			reserveQueueStrip(&current.FrameDisplay)
			current.FrameDisplay.LowResourceRendering = !cfg.OptionalWork(cfg.ResourceSmooth)
			if !lastInput.IsZero() {
				at := lastInput
				current.LastInputAt = &at
			}
			if tick%4 == 1 {
				if augLogNeeded {
					if err := simService.Augmentation.Flush(filepath.Join(*retail, "Interface", "AddOns")); err != nil && tick%40 == 1 {
						log.Printf("Augmentation review: %v", err)
					}
				}
				if e := engine.WriteJSON(statePath, current); e != nil && tick%40 == 1 {
					log.Printf("State write: %v", e)
				}
			}
			if e := engine.WriteJSON(frameStatePath, struct {
				PID          int                 `json:"pid"`
				FrameDisplay engine.FrameDisplay `json:"frameDisplay"`
			}{os.Getpid(), current.FrameDisplay}); e != nil && tick%40 == 1 {
				log.Printf("Frame state write: %v", e)
			}
			select {
			case err := <-displayDone:
				if err != nil {
					log.Printf("Frame display stopped: %v", err)
				}
				if stopDisplay != nil {
					stopDisplay()
				}
				stopDisplay = nil
				displayDone = nil
				displayRetry = now.Add(5 * time.Second)
			default:
			}
			if stopDisplay != nil && !current.FrameDisplay.OverlaysActive && len(current.FrameDisplay.Overlays) == 0 && len(current.FrameDisplay.OverlaySounds) == 0 && ((!cfg.FrameEstimates && !cfg.KickEnabled && !opponents.Active()) || !bridgeEnabled) {
				stopDisplay()
			}
			if stopDisplay == nil && current.FrameDisplay.Enabled && current.FrameDisplay.Window != "0" && !now.Before(displayRetry) {
				displayCtx, cancel := context.WithCancel(ctx)
				done, err := startFrameDisplay(displayCtx, frameStatePath)
				if err != nil {
					cancel()
					displayRetry = now.Add(5 * time.Second)
					log.Printf("Frame display: %v", err)
				} else {
					stopDisplay = cancel
					displayDone = done
				}
			}
			// Keep a diagnostic summary after exit; state.json is intentionally ephemeral.
			if tick%40 == 1 {
				history.Record(map[string]any{"kind": "tracking_health", "input": inputStatus, "bridge": bridgeStatus,
					"trackingEnabled": cfg.TrackingEnabled, "kickEnabled": cfg.KickEnabled, "defensiveTracking": cfg.DefensiveTracking,
					"roster": castRoster, "diagnostics": inference.Diagnostics, "landed": inference.InterruptCounts,
					"attempts": inference.InterruptAttempts, "kickBars": current.FrameDisplay.KickBars,
					"defensives": current.FrameDisplay.Icons})
				if err := history.Flush(); err != nil {
					log.Printf("Tracking history: %v", err)
				}
				if e := engine.WriteJSON(filepath.Join(dir, "diagnostics.json"), inference.Diagnostics); e != nil {
					log.Printf("Diagnostics write: %v", e)
				}
				if err := engine.WriteJSON(filepath.Join(dir, "lookup-diagnostics.json"), lookups.Status()); err != nil {
					log.Printf("Lookup diagnostics write: %v", err)
				}
			}
		}
	}
}
func checkUpdate(ctx context.Context) string { return checkRelease(ctx) }
func newestTag(output, current string) string {
	best := current
	found := ""
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		tag := strings.TrimPrefix(f[1], "refs/tags/")
		if newer(tag, best) {
			best = tag
			found = tag
		}
	}
	return found
}
func checkRelease(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/tm-LBenson/lamda-engine/releases/latest", nil)
	req.Header.Set("User-Agent", "LamdaEngine/"+version)
	r, e := http.DefaultClient.Do(req)
	if e != nil {
		return ""
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return ""
	}
	var release struct {
		Tag   string `json:"tag_name"`
		Draft bool   `json:"draft"`
		Pre   bool   `json:"prerelease"`
	}
	if json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&release) != nil {
		return ""
	}
	if !release.Draft && !release.Pre && newer(release.Tag, productReleaseVersion) {
		return release.Tag
	}
	return ""
}
func runReplay(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 1<<20)
	enc := json.NewEncoder(os.Stdout)
	count := 0
	for scan.Scan() {
		c, e := engine.Parse(scan.Text())
		if e != nil {
			continue
		}
		s, ok := engine.Catalog[c.Spell]
		if c.Companion {
			name, known := engine.CompanionSpell(c.GUID, c.Spell)
			s = engine.Spell{Name: name}
			ok = known
		}
		if !ok {
			continue
		}
		count++
		record := map[string]any{"at": c.At, "player": c.Name, "spell": c.Spell, "name": s.Name, "companion": c.Companion}
		if !c.Companion {
			record["estimatedCooldown"] = s.Cooldown
		}
		enc.Encode(record)
	}
	fmt.Fprintf(os.Stderr, "Matched %d supported casts\n", count)
	return scan.Err()
}

func runAudit(path string, delay time.Duration, output io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	snapshot, err := engine.ReplayAudit(f, delay)
	if err != nil {
		return err
	}
	// Recording order is real; receipt times in this mode are explicitly synthetic.
	report := struct {
		Mode                     string  `json:"mode"`
		SimulatedDeliverySeconds float64 `json:"simulatedDeliverySeconds"`
		engine.InferenceSnapshot
	}{"replay", delay.Seconds(), snapshot}
	enc := json.NewEncoder(output)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func displayRows(m *engine.Model, c engine.Config, now time.Time) []engine.Row {
	rows := m.Active(now)
	if !c.Enabled {
		return []engine.Row{}
	}
	if !c.Companions {
		filtered := make([]engine.Row, 0, len(rows))
		for _, row := range rows {
			if !row.Companion {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	if c.Preview && c.Enabled {
		rows = engine.PreviewRows(now)
	}
	if len(rows) > c.MaxRows {
		rows = rows[:c.MaxRows]
	}
	return rows
}

func buildState(pid int, c engine.Config, rows []engine.Row, update string) state {
	headers := 0
	if c.Preview && c.Enabled {
		headers++
	}
	if update != "" {
		headers++
	}
	return state{Version: version, PID: pid, Config: c, Rows: rows, Update: update, Layout: engine.LayoutFor(c, len(rows), headers)}
}
