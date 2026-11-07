package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/addonstore"
	"github.com/tm-LBenson/lamda-engine/internal/engine"
	"github.com/tm-LBenson/lamda-engine/internal/kbb"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"github.com/tm-LBenson/lamda-engine/internal/simming"
)

func startSimWorkspace(ctx context.Context, dir, client, wow string, configs ...string) (*simming.Service, error) {
	dir = filepath.Join(dir, "sim")
	var damagePackages []string
	if client == "retail" {
		if executable, err := os.Executable(); err == nil {
			damagePackages = append(damagePackages, filepath.Join(filepath.Dir(executable), "providers", "simc"))
		}
	}
	provider := simming.LoadProviders(dir, hideWindow, damagePackages...)
	service, err := simming.Listen(ctx, dir, client, provider)
	if err != nil {
		return nil, err
	}
	log.Printf("LamdaSim workspace: %s", service.URL)
	if wow == "" {
		return service, nil
	}
	moduleStore, storeErr := addonstore.Open(filepath.Join(filepath.Dir(dir), "addons"), wow, client)
	if storeErr != nil {
		log.Printf("Browse Addons unavailable: %v", storeErr)
	} else {
		service.Addons = moduleStore
	}
	if client == "retail" {
		service.Workspace.SetHealingLogs(filepath.Join(wow, "Logs"))
	}
	service.Overlays.SetGameFile(filepath.Join(wow, "Interface", "AddOns", "lamdaUI", "OverlaysData.lua"))
	moduleEnabled := func(id string) func() bool {
		if moduleStore == nil {
			return nil
		}
		return func() bool { return moduleStore.Installed(id) }
	}
	go service.Planner.RunPublisher(service.Context, filepath.Join(wow, "Interface", "AddOns"), moduleEnabled("kbb"))
	go simming.PublishGameResults(service.Context, service.Workspace, filepath.Join(wow, "Interface", "AddOns"), moduleEnabled("sim"))
	go func() {
		assembler := &simming.Assembler{Client: client}
		plannerAssembler := &kbb.Assembler{Client: client}
		plannerProblem := ""
		reloadGate := &simming.ReloadGate{Client: client}
		reloadDiagnostics := simReloadDiagnostics{write: log.Printf}
		transferDiagnostics := simTransferDiagnostics{write: log.Printf}
		// Independent channel: CD/Kick opt-outs never disable a user-requested sim.
		saved := ""
		if len(configs) > 0 {
			saved = configs[0]
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		var samples <-chan lookup.BridgeSample
		var capture moduleCapture
		defer capture.Close()
		cfg := engine.DefaultClientConfig()
		refresh := func() {
			cfg = engine.DefaultClientConfig()
			if path := savedConfigPath(saved, wow); path != "" {
				if c, err := engine.ReadCachedConfig(path); err == nil {
					cfg = c
				} else if samples != nil {
					return
				}
			}
			if service.Addons != nil {
				cfg = applyModuleInventory(cfg, service.Addons)
			}
			service.Overlays.SetCaptureEnabled(cfg.OverlaysEnabled)
			service.SetAutomaticAudit(cfg.SimEnabled)
			capture.Configure(service.Context, transferStrips(cfg), wow, client)
			samples = capture.samples
		}
		refresh()
		for {
			var sample lookup.BridgeSample
			select {
			case <-service.Context.Done():
				return
			case <-ticker.C:
				refresh()
				continue
			case received, ok := <-samples:
				if !ok {
					capture.Close()
					samples = nil
					continue
				}
				sample = received
			}
			if sample.Error != "" {
				continue
			}
			if sample.Overlays != nil && cfg.OverlaysEnabled {
				service.Overlays.Accept(sample, time.Now())
				continue
			}
			if len(sample.KBB) > 0 && cfg.KBBEnabled {
				snapshot, err := plannerAssembler.Accept(sample.KBB, time.Now())
				if err == nil && snapshot != nil {
					err = service.Planner.Merge(*snapshot, time.Now())
				}
				if err != nil && err.Error() != plannerProblem {
					plannerProblem = err.Error()
					log.Printf("KBB sync: %s", plannerProblem)
				}
				if err == nil && snapshot != nil {
					plannerProblem = ""
				}
				continue
			}
			if len(sample.Reload) > 0 && cfg.SimEnabled {
				transferDiagnostics.summary()
				reloadDiagnostics.observed(sample.Reload)
				state := service.Workspace.State()
				request, err := reloadGate.Accept(sample.Reload, state, time.Now())
				if err != nil {
					reloadDiagnostics.rejected(err)
					continue
				}
				if request == nil {
					continue
				}
				// Serialize publication and read the newest state before input.
				if _, err := service.Workspace.PublishResults(filepath.Join(wow, "Interface", "AddOns"), request.RequestID); err != nil {
					log.Printf("Sim reload publication failed: %v", err)
					continue
				}
				if err := lookup.SendSimReload(service.Context, sample.Window, wow, client, sample.Reload, sample.Position); err != nil {
					log.Printf("Sim reload key not delivered: %v", err)
				} else {
					reloadDiagnostics.keySubmitted()
				}
				continue
			}
			if len(sample.Sim) == 0 || !cfg.SimEnabled {
				continue
			}
			transferDiagnostics.observed(sample.Sim)
			snapshot, err := assembler.Accept(sample.Sim, time.Now())
			transferDiagnostics.result(snapshot != nil, err)
			if err != nil || snapshot == nil {
				continue
			}
			if snapshot.GameRequest != nil {
				if _, err := service.Workspace.ReceiveGame(snapshot); err != nil {
					log.Printf("In-game sim request: %v", err)
				}
				continue
			}
			fresh, err := service.Workspace.Receive(snapshot)
			if err != nil {
				log.Printf("Sim capture: %v", err)
				continue
			}
			if !fresh {
				continue
			}
			if _, err = service.Workspace.Start(snapshot.ID, simming.DefaultScenario(), nil); err != nil {
				log.Printf("Sim calculation waiting for user/provider: %v", err)
			}
			// Only a completely validated and persisted snapshot can launch a tab.
			if err = openSimBrowser(ctx, service.URL+"/?snapshot="+url.QueryEscape(snapshot.ID)); err != nil {
				log.Printf("Open sim workspace: %v", err)
			}
		}
	}()
	return service, nil
}
