// Design: docs/architecture/chaos-web-dashboard.md -- orchestrator run loop and reporting setup

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"net/netip"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	chaosmcp "github.com/ze-software/ze/internal/chaos/mcp"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/chaos/route"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/chaos/validation"
	"github.com/ze-software/ze/internal/chaos/watchdog"
	"github.com/ze-software/ze/internal/chaos/web"
	zemcp "github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrMcpRequiresWeb is returned when --mcp is set without --web.
var ErrMcpRequiresWeb = errors.New("--mcp requires --web (MCP reads dashboard state)")

// RunOrchestrator launches N peer simulators and validates route propagation.
func RunOrchestrator(ctx context.Context, cfg *orchestratorConfig) int {
	var savedTermState *term.State
	if fd := int(os.Stderr.Fd()); term.IsTerminal(fd) {
		savedTermState, _ = term.GetState(fd)
	}

	profiles := cfg.Profiles
	n := len(profiles)

	// Reject a config whose single-port service listeners (web, metrics, mcp)
	// fall inside the BGP or listen port ranges before any setup runs. The flag
	// path checks this at cli.go and prints there; doing it here (identical
	// error text from ValidateRangeConflicts) protects every caller that builds
	// an OrchestratorConfig directly (AC-10).
	if err := ValidateConfigRangeConflicts(cfg); err != nil {
		slog.Error("chaos orchestrator config rejected", "error", err)
		return 1
	}

	chaosEnabled := cfg.ChaosCfg.Rate > 0 || cfg.WebAddr != ""

	ibgp, families := replayPeerProfiles(cfg)
	reachable := validation.ScenarioReachability(ibgp, families)
	ep := validation.NewEventProcessor(n, cfg.ConvergenceDeadline, reachable)
	var propEngine *validation.PropertyEngine
	if cfg.Properties != "" {
		var err error
		propEngine, err = validation.ScenarioProperties(cfg.Properties, n, cfg.ConvergenceDeadline, reachable)
		if err != nil {
			var tb textbuf.Buffer
			tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI diagnostic
			return 1
		}
	}

	schedDone := make(chan struct{})
	var schedDoneOnce sync.Once
	finishSched := func() { schedDoneOnce.Do(func() { close(schedDone) }) }
	defer finishSched()

	rr, err := setupReporting(ctx, cfg, ep.Convergence, ibgp, families, schedDone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: setting up reporting: %v\n", err)
		return 1
	}
	defer rr.cleanup()

	established := newEstablishedState(n)
	guard := guard.New(n)

	var chaosChannels []chan engine.ChaosAction
	if chaosEnabled {
		chaosChannels = make([]chan engine.ChaosAction, n)
		for i := range n {
			chaosChannels[i] = make(chan engine.ChaosAction, 1)
		}
	}

	routeEnabled := cfg.RouteCfg.Rate > 0 || cfg.WebAddr != ""
	var routeChannels []chan route.Action
	if routeEnabled {
		routeChannels = make([]chan route.Action, n)
		for i := range n {
			routeChannels[i] = make(chan route.Action, 1)
		}
	}

	evBuf := 0
	for i := range profiles {
		evBuf += profiles[i].RouteCount * max(len(profiles[i].Families), 1)
	}
	evBuf = min(max(evBuf, 65536), 5_000_000)
	events := make(chan peer.Event, evBuf)

	syncStart := time.Now()
	if !cfg.Quiet {
		fmt.Fprintf(os.Stderr, "sync-start: %s\n", syncStart.Format("15:04:05.000"))
	}

	eorSeen := make([]bool, n)
	eorCount := 0
	var syncDuration time.Duration

	var wg sync.WaitGroup
	for i := range profiles {
		wg.Add(1)
		go func(prof scenario.PeerProfile) {
			defer wg.Done()

			var chaosCh <-chan engine.ChaosAction
			if chaosEnabled {
				chaosCh = chaosChannels[prof.Index]
			}

			var routeCh <-chan route.Action
			if routeEnabled {
				routeCh = routeChannels[prof.Index]
			}

			var peerAddr, srcAddr string
			var tb textbuf.Buffer
			if cfg.Target.SinglePort() {
				peerAddr = tb.Reset().Str(cfg.LocalAddr).Byte(':').Int(int64(cfg.ZePort)).String()
				srcAddr = prof.Address.String()
			} else {
				peerAddr = tb.Reset().Str(cfg.LocalAddr).Byte(':').Int(int64(prof.ZePort)).String()
				srcAddr = cfg.PeerLocalAddr
			}

			simCfg := peer.SimulatorConfig{
				Profile: peer.SimProfile{
					Index:      prof.Index,
					ASN:        prof.ASN,
					RouterID:   prof.RouterID,
					IsIBGP:     prof.IsIBGP,
					HoldTime:   prof.HoldTime,
					RouteCount: prof.RouteCount,
					TotalPeers: n,
					Families:   prof.Families,
					SlowRead:   prof.SlowRead,
					RouteSet:   prof.RouteSet,
				},
				Seed:      cfg.Seed,
				Addr:      peerAddr,
				LocalAddr: srcAddr,
				Events:    events,
				Chaos:     chaosCh,
				Routes:    routeCh,
				ZePID:     cfg.ZePID,
				Verbose:   cfg.Verbose,
				Quiet:     cfg.Quiet,
			}

			if !chaosEnabled {
				peer.RunSimulator(ctx, simCfg)
				return
			}

			runPeerLoop(ctx, simCfg, prof.Index, events)
		}(profiles[i])
	}

	observationEnd, hasDeadline := ctx.Deadline()
	chaosStopAt := schedulerStopTime(observationEnd, hasDeadline, syncStart, cfg.ChaosCfg.Warmup)
	routeStopAt := schedulerStopTime(observationEnd, hasDeadline, syncStart, cfg.RouteCfg.Warmup)

	if chaosEnabled {
		go runScheduler(ctx, cfg.ChaosCfg, cfg.Seed, n, established, guard, chaosChannels, rr.controlCh, chaosStopAt, cfg.Quiet, finishSched)
	}

	if routeEnabled {
		go runRouteScheduler(ctx, cfg.RouteCfg, cfg.Seed, n, established, guard, routeChannels, rr.routeControlCh, routeStopAt, cfg.Quiet)
	}

	go func() {
		wg.Wait()
		close(events)
	}()

	propUpdateCounter := 0

	for ev := range events {
		if savedTermState != nil && isLifecycleEvent(ev.Type) {
			_ = term.Restore(int(os.Stderr.Fd()), savedTermState)
		}

		switch ev.Type {
		case peer.EventEstablished:
			established.Set(ev.PeerIndex, true)
			guard.OnEstablished(ev.PeerIndex)
		case peer.EventDisconnected:
			established.Set(ev.PeerIndex, false)
			guard.OnDisconnected(ev.PeerIndex)
		case peer.EventChaosExecuted:
			if ev.ChaosAction == engine.ActionHoldTimerExpiry.String() {
				guard.OnHoldTimerExpiry(ev.PeerIndex)
			}
		case peer.EventRouteAction:
			switch ev.RouteAction {
			case route.ActionFullWithdraw.String():
				guard.OnFullWithdraw(ev.PeerIndex)
			case route.ActionChurn.String():
				guard.OnRoutesRestored(ev.PeerIndex)
			}
		case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
			peer.EventEORSent, peer.EventError,
			peer.EventReconnecting, peer.EventWithdrawalSent, peer.EventDroppedEvents:
		default:
			panic("BUG: invalid orchestrator event type")
		}

		if ev.Type == peer.EventEORSent && ev.PeerIndex < len(eorSeen) && !eorSeen[ev.PeerIndex] {
			eorSeen[ev.PeerIndex] = true
			eorCount++
			if eorCount == n {
				syncDuration = time.Since(syncStart)
				if !cfg.Quiet {
					fmt.Fprintf(os.Stderr, "sync-done:  %s (duration: %s)\n",
						time.Now().Format("15:04:05.000"),
						syncDuration.Truncate(time.Millisecond))
				}
			}
		}

		if ev.PlannedShutdown && (observationEnd.IsZero() || ev.Time.Before(observationEnd)) {
			observationEnd = ev.Time
		}
		if !ev.PlannedShutdown && (observationEnd.IsZero() || ev.Time.Before(observationEnd)) {
			ep.Process(ev)
			if propEngine != nil {
				propEngine.ProcessEvent(ev)
			}
			if rr.eventLog != nil {
				rr.eventLog.ProcessEvent(ev)
			}
		}
		rr.reporter.Process(ev)

		if propEngine != nil && rr.webDash != nil {
			propUpdateCounter++
			if propUpdateCounter%50 == 0 {
				results := propEngine.Results()
				badges := make([]web.PropertyBadge, len(results))
				for i, r := range results {
					var violations []string
					for _, v := range r.Violations {
						violations = append(violations, v.Message)
					}
					badges[i] = web.PropertyBadge{
						Name:       r.Name,
						Pass:       r.Pass,
						Violations: violations,
					}
				}
				rr.webDash.SetPropertyResults(badges)
			}
		}

		if cfg.Verbose && ev.Type == peer.EventError {
			fmt.Fprintf(os.Stderr, "le chaos run | peer %d | error: %v\n", ev.PeerIndex, ev.Err)
		}
	}

	if savedTermState != nil {
		_ = term.Restore(int(os.Stderr.Fd()), savedTermState)
	}

	if now := time.Now(); observationEnd.IsZero() || now.Before(observationEnd) {
		observationEnd = now
	}
	if rr.eventLog != nil {
		rr.eventLog.EndObservation(observationEnd)
		if closeErr := rr.eventLog.Close(); closeErr != nil {
			var tb textbuf.Buffer
			tb.Str("error: closing event log: ").Err(closeErr).Byte('\n').StdErr() //nolint:errcheck // CLI diagnostic
			return 1
		}
	}
	if closeErr := rr.reporter.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "error: closing reporter: %v\n", closeErr)
		return 1
	}

	var ibgpCount, ebgpCount int
	for i := range profiles {
		if profiles[i].IsIBGP {
			ibgpCount++
		} else {
			ebgpCount++
		}
	}

	summary := report.Summary{
		Seed: cfg.Seed, Duration: observationEnd.Sub(cfg.Start), SyncDuration: syncDuration,
		PeerCount: n, IBGPCount: ibgpCount, EBGPCount: ebgpCount,
	}
	summary.Observe(ep, propEngine, observationEnd)

	return summary.Write(os.Stderr)
}

type reportingResult struct {
	eventLog       *report.JSONLog
	reporter       *report.Reporter
	cleanup        func()
	controlCh      chan web.ControlCommand
	routeControlCh chan web.ControlCommand
	webDash        *web.Dashboard
}

func replayPeerProfiles(cfg *orchestratorConfig) ([]bool, [][]string) {
	families := make([][]string, len(cfg.Profiles))
	var ibgp []bool
	if cfg.Target == scenario.TargetZe {
		ibgp = make([]bool, len(cfg.Profiles))
	}
	for _, profile := range cfg.Profiles {
		families[profile.Index] = profile.Families
		if ibgp != nil {
			ibgp[profile.Index] = profile.IsIBGP
		}
	}
	return ibgp, families
}

func setupReporting(ctx context.Context, cfg *orchestratorConfig, convergence *validation.Convergence, ibgp []bool, families [][]string, schedDone <-chan struct{}) (*reportingResult, error) {
	peerCount := len(cfg.Profiles)
	var eventLog *report.JSONLog
	var consumers []report.Consumer
	var cleanups []func()
	var controlCh chan web.ControlCommand
	var webDashRef *web.Dashboard
	var controlLogger web.ControlLogger

	if !cfg.Quiet {
		isTTY := term.IsTerminal(int(os.Stderr.Fd()))
		dash := report.NewDashboard(os.Stderr, report.DashboardConfig{
			IsTTY:     isTTY,
			PeerCount: peerCount,
		})
		consumers = append(consumers, dash)
	}

	if cfg.EventLog != "" {
		f, err := cliio.Create(cfg.EventLog) // "-" writes stdout
		if err != nil {
			return nil, fmt.Errorf("opening event file %s: %w", cfg.EventLog, err)
		}
		eventLog = report.NewJSONLog(f, report.JSONLogConfig{
			Start:               cfg.Start,
			Seed:                cfg.Seed,
			Peers:               peerCount,
			ChaosRate:           cfg.ChaosCfg.Rate,
			IBGPPeers:           ibgp,
			PeerFamilies:        families,
			ConvergenceDeadline: cfg.ConvergenceDeadline,
			Properties:          cfg.Properties,
		})
		controlLogger = eventLog
		cleanups = append(cleanups, func() {
			if err := f.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error: closing event file: %v\n", err)
			}
		})
	}

	if cfg.MRTFile != "" {
		peers := make([]report.MRTPeer, len(cfg.Profiles))
		for i, p := range cfg.Profiles {
			peers[i] = report.MRTPeer{ASN: p.ASN, Addr: p.RouterID}
		}
		localAddr := netip.MustParseAddr(cfg.LocalAddr)
		mrtlog := report.NewMRTLog(cfg.MRTFile, report.MRTLogConfig{
			LocalAS:   65000,
			LocalAddr: localAddr,
			Peers:     peers,
		})
		consumers = append(consumers, mrtlog)
	}

	if cfg.WebAddr != "" {
		controlCh = make(chan web.ControlCommand, 16)
	}

	var routeControlCh chan web.ControlCommand
	if cfg.WebAddr != "" {
		routeControlCh = make(chan web.ControlCommand, 16)
	}

	newWebDash := func(mux *http.ServeMux) (*web.Dashboard, error) {
		return web.New(web.Config{
			Addr:                cfg.WebAddr,
			PeerCount:           peerCount,
			Seed:                cfg.Seed,
			Mux:                 mux,
			Control:             controlCh,
			RouteControl:        routeControlCh,
			ChaosRate:           cfg.ChaosCfg.Rate,
			RouteRate:           cfg.RouteCfg.Rate,
			WarmupDuration:      cfg.ChaosCfg.Warmup,
			ConvergenceDeadline: cfg.ConvergenceDeadline,
			ControlLogger:       controlLogger,
			RestartCh:           cfg.RestartCh,
			OnStop:              cfg.OnStop,
			PeerFamilyTargets:   PeerFamilyTargets(cfg.Profiles),
		})
	}

	if cfg.MetricsAddr != "" && cfg.WebAddr != "" {
		sharedMux := http.NewServeMux()

		m := report.NewMetrics()
		consumers = append(consumers, m)
		sharedMux.Handle("/metrics", m.Handler())

		wd, webErr := newWebDash(sharedMux)
		if webErr != nil {
			return nil, fmt.Errorf("starting web dashboard: %w", webErr)
		}
		webDashRef = wd
		consumers = append(consumers, wd)

		srv := &http.Server{Addr: cfg.WebAddr, Handler: sharedMux, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintf(os.Stderr, "error: shared server: %v\n", err)
			}
		}()

		cleanups = append(cleanups, func() {
			if err := wd.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error: closing web dashboard: %v\n", err)
			}
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutCancel()
			if err := srv.Shutdown(shutCtx); err != nil {
				fmt.Fprintf(os.Stderr, "error: shutting down shared server: %v\n", err)
			}
		})
	} else {
		if cfg.MetricsAddr != "" {
			m := report.NewMetrics()
			consumers = append(consumers, m)

			mux := http.NewServeMux()
			mux.Handle("/metrics", m.Handler())
			srv := &http.Server{Addr: cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

			go func() {
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					fmt.Fprintf(os.Stderr, "error: metrics server: %v\n", err)
				}
			}()

			cleanups = append(cleanups, func() {
				shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer shutCancel()
				if err := srv.Shutdown(shutCtx); err != nil {
					fmt.Fprintf(os.Stderr, "error: shutting down metrics server: %v\n", err)
				}
			})
		}

		if cfg.WebAddr != "" {
			wd, webErr := newWebDash(nil)
			if webErr != nil {
				return nil, fmt.Errorf("starting web dashboard: %w", webErr)
			}
			webDashRef = wd
			consumers = append(consumers, wd)
			cleanups = append(cleanups, func() {
				if err := wd.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "error: closing web dashboard: %v\n", err)
				}
			})
		}
	}

	wdCfg := watchdog.DefaultConfig()
	wdCfg.Warmup = cfg.ChaosCfg.Warmup
	wd := watchdog.New(os.Stderr, wdCfg)
	consumers = append(consumers, wd)

	if cfg.McpAddr != "" && webDashRef == nil {
		return nil, ErrMcpRequiresWeb
	}
	if cfg.McpAddr != "" && webDashRef != nil {
		provider := &chaosmcp.Provider{
			State:       webDashRef.State(),
			Watchdog:    wd,
			Convergence: convergence,
			Control:     newControlDispatcher(ctx, controlCh, schedDone, webDashRef),
			Seed:        cfg.Seed,
			StartTime:   cfg.Start,
			PeerCount:   peerCount,
		}
		mcpHandler, mcpErr := zemcp.NewStreamable(zemcp.StreamableConfig{Provider: provider})
		if mcpErr != nil {
			return nil, fmt.Errorf("chaos MCP server: %w", mcpErr)
		}
		cleanups = append(cleanups, mcpHandler.Close)
		mcpMux := http.NewServeMux()
		mcpMux.Handle(zemcp.Endpoint, mcpHandler)
		mcpSrv := &http.Server{Addr: cfg.McpAddr, Handler: mcpMux, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			if err := mcpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintf(os.Stderr, "error: chaos MCP server: %v\n", err)
			}
		}()
		var mcpURLBuf textbuf.Buffer
		mcpURLBuf.Str("le chaos run | MCP server: http://").Str(cfg.McpAddr).Str(zemcp.Endpoint).Byte('\n').StdErr() //nolint:errcheck // CLI status output
		cleanups = append(cleanups, func() {
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutCancel()
			if err := mcpSrv.Shutdown(shutCtx); err != nil {
				fmt.Fprintf(os.Stderr, "error: shutting down MCP server: %v\n", err)
			}
		})
	}

	r := report.NewReporter(consumers...)
	return &reportingResult{
		eventLog:       eventLog,
		reporter:       r,
		controlCh:      controlCh,
		routeControlCh: routeControlCh,
		webDash:        webDashRef,
		cleanup: func() {
			for _, fn := range cleanups {
				fn()
			}
		},
	}, nil
}
