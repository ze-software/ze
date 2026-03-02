// Design: docs/architecture/chaos-web-dashboard.md — orchestrator run loop and reporting setup
// Overview: main.go — CLI entry and flag parsing
// Related: orchestrator.go — config types, established state, event processor
// Related: scheduler.go — chaos and route dynamics schedulers
// Related: guard.go — peerGuard for action compatibility checks

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/report"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/route"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/validation"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/web"
)

// runOrchestrator launches N peer simulators and validates route propagation.
// When chaos is enabled (chaosCfg.Rate > 0), it also starts the chaos scheduler
// and wraps each peer in a reconnection loop.
func runOrchestrator(ctx context.Context, cfg orchestratorConfig) int {
	// Save terminal state before launching subprocesses.
	// After Ctrl+C, a dying subprocess can leave ONLCR disabled,
	// causing \n to line-feed without carriage-return (staircase output).
	var savedTermState *term.State
	if fd := int(os.Stderr.Fd()); term.IsTerminal(fd) {
		savedTermState, _ = term.GetState(fd)
	}

	profiles := cfg.profiles
	n := len(profiles)
	// Chaos is enabled when rate > 0 OR the web dashboard is active
	// (so the user can increase the rate from the UI later).
	chaosEnabled := cfg.chaosCfg.Rate > 0 || cfg.webAddr != ""

	// Create validation components.
	model := validation.NewModel(n)
	tracker := validation.NewTracker(n)
	convergence := validation.NewConvergence(n, cfg.convergenceDeadline)
	ep := &EventProcessor{
		Model:       model,
		Tracker:     tracker,
		Convergence: convergence,
	}

	// Create property engine (nil when --properties is not set).
	var propEngine *validation.PropertyEngine
	if cfg.properties != "" {
		all := validation.AllProperties(n, cfg.convergenceDeadline)
		if cfg.properties == "all" {
			propEngine = validation.NewPropertyEngine(all)
		} else {
			var names []string
			for n := range strings.SplitSeq(cfg.properties, ",") {
				n = strings.TrimSpace(n)
				if n != "" {
					names = append(names, n)
				}
			}
			if len(names) == 0 {
				fmt.Fprintf(os.Stderr, "error: --properties requires at least one property name\n")
				return 1
			}
			selected, selErr := validation.SelectProperties(all, names)
			if selErr != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", selErr)
				return 1
			}
			propEngine = validation.NewPropertyEngine(selected)
		}
	}

	// Set up reporting consumers based on CLI flags.
	rr, err := setupReporting(cfg, n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: setting up reporting: %v\n", err)
		return 1
	}
	defer rr.cleanup()

	// Established state tracking (shared with scheduler goroutine).
	established := newEstablishedState(n)
	guard := newPeerGuard(n)

	// Per-peer chaos channels (only allocated when chaos is enabled).
	var chaosChannels []chan chaos.ChaosAction
	if chaosEnabled {
		chaosChannels = make([]chan chaos.ChaosAction, n)
		for i := range n {
			chaosChannels[i] = make(chan chaos.ChaosAction, 1)
		}
	}

	// Per-peer route dynamics channels (only allocated when route dynamics is enabled).
	routeEnabled := cfg.routeCfg.Rate > 0 || cfg.webAddr != ""
	var routeChannels []chan route.Action
	if routeEnabled {
		routeChannels = make([]chan route.Action, n)
		for i := range n {
			routeChannels[i] = make(chan route.Action, 1)
		}
	}

	// Size the events channel proportionally to expected route volume.
	// Each route generates a send event; reflected routes generate receive events.
	// Must be large enough to absorb bursts without blocking readLoop
	// (which would cause TCP backpressure deadlocks).
	evBuf := 0
	for i := range profiles {
		evBuf += profiles[i].RouteCount * max(len(profiles[i].Families), 1)
	}
	evBuf = min(max(evBuf, 65536), 5_000_000)
	events := make(chan peer.Event, evBuf)

	// Record sync start time just before launching peer goroutines.
	syncStart := time.Now()
	if !cfg.quiet {
		fmt.Fprintf(os.Stderr, "sync-start: %s\n", syncStart.Format("15:04:05.000"))
	}

	// Track initial EOR per peer to measure initial route synchronization.
	eorSeen := make([]bool, n)
	eorCount := 0
	var syncDuration time.Duration

	// Launch per-peer goroutines.
	var wg sync.WaitGroup
	for i := range profiles {
		wg.Add(1)
		go func(prof scenario.PeerProfile) {
			defer wg.Done()

			// Chaos channel for this peer (nil when chaos is disabled).
			var chaosCh <-chan chaos.ChaosAction
			if chaosEnabled {
				chaosCh = chaosChannels[prof.Index]
			}

			// Route dynamics channel for this peer (nil when route dynamics is disabled).
			var routeCh <-chan route.Action
			if routeEnabled {
				routeCh = routeChannels[prof.Index]
			}

			// Each peer dials its unique Ze-facing port on localhost.
			peerAddr := fmt.Sprintf("%s:%d", cfg.localAddr, prof.ZePort)

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
				},
				Seed:    cfg.seed,
				Addr:    peerAddr,
				Events:  events,
				Chaos:   chaosCh,
				Routes:  routeCh,
				ZePID:   cfg.zePID,
				Verbose: cfg.verbose,
				Quiet:   cfg.quiet,
			}

			if !chaosEnabled {
				// Single-shot mode (Phase 1/2 behavior).
				peer.RunSimulator(ctx, simCfg)
				return
			}

			// Reconnection loop: restart simulator after chaos disconnects.
			runPeerLoop(ctx, simCfg, prof.Index, events)
		}(profiles[i])
	}

	// Launch scheduler goroutine (only when chaos is enabled).
	if chaosEnabled {
		go runScheduler(ctx, cfg.chaosCfg, cfg.seed, n, established, guard, chaosChannels, rr.controlCh, cfg.quiet)
	}

	// Launch route dynamics scheduler goroutine (only when route dynamics is enabled).
	if routeEnabled {
		go runRouteScheduler(ctx, cfg.routeCfg, cfg.seed, n, established, guard, routeChannels, rr.routeControlCh, cfg.quiet)
	}

	// Close events channel when all peer goroutines finish.
	go func() {
		wg.Wait()
		close(events)
	}()

	// Property badge update counter (push every ~50 events to avoid overhead).
	propUpdateCounter := 0

	// Process events from all peers.
	for ev := range events {
		// Restore terminal before any lifecycle event that produces output.
		// The ze subprocess may corrupt ONLCR at any point during shutdown,
		// so a single restore is not enough — we must restore before each
		// visible line. Lifecycle events are rare (O(peers)), so the ioctl
		// cost is negligible.
		if savedTermState != nil && isLifecycleEvent(ev.Type) {
			_ = term.Restore(int(os.Stderr.Fd()), savedTermState)
		}

		// Update established state and peer guard for schedulers.
		switch ev.Type {
		case peer.EventEstablished:
			established.Set(ev.PeerIndex, true)
			guard.OnEstablished(ev.PeerIndex)
		case peer.EventDisconnected:
			established.Set(ev.PeerIndex, false)
			guard.OnDisconnected(ev.PeerIndex)
		case peer.EventChaosExecuted:
			if ev.ChaosAction == chaos.ActionHoldTimerExpiry.String() {
				guard.OnHoldTimerExpiry(ev.PeerIndex)
			}
		case peer.EventRouteAction:
			switch ev.RouteAction {
			case route.ActionFullWithdraw.String():
				guard.OnFullWithdraw(ev.PeerIndex)
			case route.ActionChurn.String():
				// Churn re-announces routes, so they're live again.
				guard.OnRoutesRestored(ev.PeerIndex)
			}
		case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
			peer.EventEORSent, peer.EventError,
			peer.EventReconnecting, peer.EventWithdrawalSent, peer.EventDroppedEvents:
			// Other events don't affect guard or established state.
		}

		// Track initial EOR to measure sync time. Only the first EOR per peer
		// counts — chaos reconnects can produce additional EORs.
		if ev.Type == peer.EventEORSent && ev.PeerIndex < len(eorSeen) && !eorSeen[ev.PeerIndex] {
			eorSeen[ev.PeerIndex] = true
			eorCount++
			if eorCount == n {
				syncDuration = time.Since(syncStart)
				if !cfg.quiet {
					fmt.Fprintf(os.Stderr, "sync-done:  %s (duration: %s)\n",
						time.Now().Format("15:04:05.000"),
						syncDuration.Truncate(time.Millisecond))
				}
			}
		}

		ep.Process(ev)
		if propEngine != nil {
			propEngine.ProcessEvent(ev)
		}
		rr.reporter.Process(ev)

		// Push property results to web dashboard periodically.
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

		if cfg.verbose && ev.Type == peer.EventError {
			fmt.Fprintf(os.Stderr, "ze-chaos | peer %d | error: %v\n", ev.PeerIndex, ev.Err)
		}
	}

	// Restore terminal state saved at startup. After Ctrl+C, a dying
	// subprocess can leave ONLCR disabled (staircase output).
	if savedTermState != nil {
		_ = term.Restore(int(os.Stderr.Fd()), savedTermState)
	}

	// Close reporter (flush files, clear dashboard).
	if closeErr := rr.reporter.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "error: closing reporter: %v\n", closeErr)
	}

	// Count iBGP/eBGP peers for summary.
	var ibgpCount, ebgpCount int
	for i := range profiles {
		if profiles[i].IsIBGP {
			ibgpCount++
		} else {
			ebgpCount++
		}
	}

	// Final validation.
	result := validation.Check(model, tracker)
	convStats := convergence.Stats()
	slow := convergence.CheckDeadline(time.Now())

	// Build per-peer failure details from check result.
	var peerFailures []report.PeerFailure
	var missingCount, extraCount int
	for i, pr := range result.Peers {
		if pr.Missing.Len() == 0 && pr.Extra.Len() == 0 {
			continue
		}
		pf := report.PeerFailure{
			PeerIndex:     i,
			ExpectedCount: pr.ExpectedCount,
			ActualCount:   pr.ActualCount,
		}
		pf.Missing = pr.Missing.SortedStrings()
		pf.Extra = pr.Extra.SortedStrings()
		missingCount += len(pf.Missing)
		extraCount += len(pf.Extra)
		peerFailures = append(peerFailures, pf)
	}

	// Collect property results (nil-safe: empty slice when engine not active).
	var propResults []report.PropertyLine
	if propEngine != nil {
		for _, r := range propEngine.Results() {
			pl := report.PropertyLine{
				Name: r.Name,
				Pass: r.Pass,
			}
			for _, v := range r.Violations {
				pl.Violations = append(pl.Violations, v.Message)
			}
			propResults = append(propResults, pl)
		}
	}

	// Build and print summary.
	summary := report.Summary{
		Seed:          cfg.seed,
		Duration:      time.Since(cfg.start).Truncate(time.Millisecond),
		SyncDuration:  syncDuration.Truncate(time.Millisecond),
		PeerCount:     n,
		IBGPCount:     ibgpCount,
		EBGPCount:     ebgpCount,
		Announced:     ep.Announced,
		Received:      ep.Received,
		Missing:       missingCount,
		Extra:         extraCount,
		MinLatency:    convStats.Min,
		AvgLatency:    convStats.Avg,
		MaxLatency:    convStats.Max,
		P99Latency:    convStats.P99,
		SlowRoutes:    len(slow),
		ChaosEvents:   ep.ChaosEvents,
		Reconnections: ep.Reconnections,
		Withdrawn:     ep.Withdrawn,
		DroppedEvents: ep.DroppedEvents,
		PeerFailures:  peerFailures,
		Properties:    propResults,
	}

	return summary.Write(os.Stderr)
}

// reportingResult holds everything returned from setupReporting.
type reportingResult struct {
	reporter       *report.Reporter
	cleanup        func()
	controlCh      chan web.ControlCommand
	routeControlCh chan web.ControlCommand
	webDash        *web.Dashboard
}

// setupReporting creates the Reporter with optional consumers based on CLI flags.
func setupReporting(cfg orchestratorConfig, peerCount int) (*reportingResult, error) {
	var consumers []report.Consumer
	var cleanups []func()
	var controlCh chan web.ControlCommand
	var webDashRef *web.Dashboard
	var controlLogger web.ControlLogger

	// Dashboard: enabled when TTY and not --quiet.
	if !cfg.quiet {
		isTTY := term.IsTerminal(int(os.Stderr.Fd()))
		dash := report.NewDashboard(os.Stderr, report.DashboardConfig{
			IsTTY:     isTTY,
			PeerCount: peerCount,
		})
		consumers = append(consumers, dash)
	}

	// NDJSON event log: enabled when --event-log is set.
	if cfg.eventLog != "" {
		f, err := os.Create(cfg.eventLog)
		if err != nil {
			return nil, fmt.Errorf("opening event file %s: %w", cfg.eventLog, err)
		}
		jlog := report.NewJSONLog(f, report.JSONLogConfig{
			Start:     cfg.start,
			Seed:      cfg.seed,
			Peers:     peerCount,
			ChaosRate: cfg.chaosCfg.Rate,
		})
		consumers = append(consumers, jlog)
		controlLogger = jlog
		cleanups = append(cleanups, func() {
			if err := f.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error: closing event file: %v\n", err)
			}
		})
	}

	// Control channel for web dashboard → chaos scheduler communication.
	// Created whenever the web dashboard is active, even at rate 0,
	// so the user can increase the rate from the UI later.
	if cfg.webAddr != "" {
		controlCh = make(chan web.ControlCommand, 16)
	}

	// Route control channel for web dashboard → route scheduler communication.
	// Same logic: always created when web is active.
	var routeControlCh chan web.ControlCommand
	if cfg.webAddr != "" {
		routeControlCh = make(chan web.ControlCommand, 16)
	}

	// Helper to create a web dashboard with common config.
	newWebDash := func(mux *http.ServeMux) (*web.Dashboard, error) {
		return web.New(web.Config{
			Addr:                cfg.webAddr,
			PeerCount:           peerCount,
			Seed:                cfg.seed,
			Mux:                 mux,
			Control:             controlCh,
			RouteControl:        routeControlCh,
			ChaosRate:           cfg.chaosCfg.Rate,
			RouteRate:           cfg.routeCfg.Rate,
			WarmupDuration:      cfg.chaosCfg.Warmup,
			ConvergenceDeadline: cfg.convergenceDeadline,
			ControlLogger:       controlLogger,
			RestartCh:           cfg.restartCh,
			OnStop:              cfg.onStop,
			PeerFamilyTargets:   peerFamilyTargets(cfg.profiles),
		})
	}

	// Shared mux: when both --metrics and --web are set, share a single HTTP server.
	if cfg.metricsAddr != "" && cfg.webAddr != "" {
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

		// Single server for both metrics and dashboard.
		srv := &http.Server{Addr: cfg.webAddr, Handler: sharedMux, ReadHeaderTimeout: 10 * time.Second}
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
		// Prometheus metrics: standalone server when --web is not set.
		if cfg.metricsAddr != "" {
			m := report.NewMetrics()
			consumers = append(consumers, m)

			mux := http.NewServeMux()
			mux.Handle("/metrics", m.Handler())
			srv := &http.Server{Addr: cfg.metricsAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

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

		// Web dashboard: standalone server when --metrics is not set.
		if cfg.webAddr != "" {
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

	r := report.NewReporter(consumers...)
	return &reportingResult{
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
