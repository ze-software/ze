// Command ze-bgp-chaos is a chaos monkey tool for testing Ze BGP route server
// (route reflector) route propagation behavior.
//
// It simulates multiple BGP peers, generates deterministic route announcements
// from a seed, validates that the route reflector correctly propagates routes,
// and injects chaos events (disconnects, hold-timer expiry, etc.).
//
// Usage:
//
//	ze-bgp-chaos [options]
//	ze-bgp-chaos --seed 42 --peers 4 --duration 30s --config-out chaos.conf
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/replay"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/report"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/shrink"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/validation"
	"golang.org/x/term"
)

// reconnectBackoff is the delay before a peer reconnects after a chaos disconnect.
const reconnectBackoff = 2 * time.Second

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("ze-bgp-chaos", flag.ContinueOnError)

	// Scenario flags
	seed := fs.Uint64("seed", 0, "Deterministic seed (default: random, always printed)")
	peers := fs.Int("peers", 4, "Number of simulated peers (1-50)")
	ibgpRatio := fs.Float64("ibgp-ratio", 0.3, "Fraction of peers that are iBGP (0.0-1.0)")

	// Route flags
	routes := fs.Int("routes", 100, "Base routes per peer")
	heavyPeers := fs.Int("heavy-peers", 1, "Peers sending many routes")
	heavyRoutes := fs.Int("heavy-routes", 2000, "Routes for heavy peers")
	churnRate := fs.Float64("churn-rate", 5, "Route changes per second per peer in steady state")

	// Family flags
	families := fs.String("families", "", "Only these families (comma-sep)")
	excludeFamilies := fs.String("exclude-families", "", "Exclude these families (comma-sep)")

	// Chaos flags
	chaosRate := fs.Float64("chaos-rate", 0.1, "Probability of chaos per interval (0.0-1.0)")
	chaosInterval := fs.Duration("chaos-interval", 10*time.Second, "Time between chaos checks")

	// Network flags
	port := fs.Int("port", 1790, "Base BGP port for Ze to listen on")
	listenBase := fs.Int("listen-base", 1890, "Base port for tool to listen on")
	localAddr := fs.String("local-addr", "127.0.0.1", "Local address")

	// Output flags
	configOut := fs.String("config-out", "", "Write Ze config here (default: stdout before start)")
	eventLog := fs.String("event-log", "", "NDJSON event log file")
	metricsAddr := fs.String("metrics", "", "Prometheus metrics endpoint (addr:port)")
	quiet := fs.Bool("quiet", false, "Only errors and summary")
	verbose := fs.Bool("verbose", false, "Extra debug output")

	// Replay/diff/shrink flags
	replayFile := fs.String("replay", "", "Replay an event log through validation model")
	diffFile1 := fs.String("diff", "", "First event log for comparison (requires --diff2)")
	diffFile2 := fs.String("diff2", "", "Second event log for comparison")
	shrinkFile := fs.String("shrink", "", "Shrink a failing event log to minimal reproduction")

	// Property flags
	properties := fs.String("properties", "", "Comma-sep property names, or 'all' (default: disabled), 'list' to show available")
	convergenceDeadline := fs.Duration("convergence-deadline", 5*time.Second, "Convergence deadline for property checks")

	// Control flags
	duration := fs.Duration("duration", 0, "Max runtime (0 = run forever until Ctrl-C)")
	warmup := fs.Duration("warmup", 5*time.Second, "Time before chaos starts")
	zePID := fs.Int("ze-pid", 0, "Ze process PID (for config-reload chaos events)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ze-bgp-chaos - Chaos monkey for Ze BGP route server testing

Usage:
  ze-bgp-chaos [options]

Scenario:
  --seed <uint64>            Deterministic seed (default: random, always printed)
  --peers <N>                Number of simulated peers (default: 4, max: 50)
  --ibgp-ratio <float>       Fraction of peers that are iBGP (default: 0.3)

Routes:
  --routes <N>               Base routes per peer (default: 100)
  --heavy-peers <N>          Peers sending many routes (default: 1)
  --heavy-routes <N>         Routes for heavy peers (default: 2000)
  --churn-rate <N/s>         Route changes per second per peer (default: 5)

Families:
  --families <list>          Only these families (comma-sep, default: all)
  --exclude-families <list>  Exclude these families (comma-sep)

Chaos:
  --chaos-rate <float>       Probability of chaos per interval (default: 0.1)
  --chaos-interval <dur>     Time between chaos checks (default: 10s)

Network:
  --port <N>                 Base BGP port for Ze to listen on (default: 1790)
  --listen-base <N>          Base port for tool to listen on (default: 1890)
  --local-addr <addr>        Local address (default: 127.0.0.1)

Output:
  --config-out <path>        Write Ze config here (default: stdout before start)
  --event-log <path>         NDJSON event log file (replayable)
  --metrics <addr:port>      Prometheus metrics endpoint
  --quiet                    Only errors and summary
  --verbose                  Extra debug output

Replay:
  --replay <path>            Replay event log through validation model
  --diff <path>              Compare two event logs (first log)
  --diff2 <path>             Compare two event logs (second log)
  --shrink <path>            Shrink failing event log to minimal reproduction

Properties:
  --properties <names>       Comma-sep property names, 'all', or 'list'
  --convergence-deadline <d> Deadline for convergence property (default: 5s)

Control:
  --duration <dur>           Max runtime (default: 0 = run forever until Ctrl-C)
  --warmup <dur>             Time before chaos starts (default: 5s)
  --ze-pid <N>               Ze process PID (for config-reload chaos events)
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// List properties mode: show available properties and exit.
	if *properties == "list" {
		all := validation.AllProperties(2, *convergenceDeadline)
		for _, line := range validation.ListProperties(all) {
			fmt.Println(line)
		}
		return 0
	}

	// Shrink mode: minimize a failing event log.
	if *shrinkFile != "" {
		return runShrink(*shrinkFile, *convergenceDeadline, *verbose)
	}

	// Replay mode: feed recorded event log through validation model.
	if *replayFile != "" {
		return runReplay(*replayFile)
	}

	// Diff mode: compare two event logs.
	if *diffFile1 != "" {
		if *diffFile2 == "" {
			fmt.Fprintf(os.Stderr, "error: --diff requires --diff2\n")
			return 1
		}
		return runDiff(*diffFile1, *diffFile2)
	}

	// Validate peer count.
	if *peers < 1 || *peers > 50 {
		fmt.Fprintf(os.Stderr, "error: --peers must be 1-50, got %d\n", *peers)
		return 1
	}

	// Validate routes.
	if *routes < 1 {
		fmt.Fprintf(os.Stderr, "error: --routes must be >= 1, got %d\n", *routes)
		return 1
	}

	// Validate chaos-rate.
	if *chaosRate < 0 || *chaosRate > 1.0 {
		fmt.Fprintf(os.Stderr, "error: --chaos-rate must be 0.0-1.0, got %f\n", *chaosRate)
		return 1
	}

	// Validate ibgp-ratio (clamp silently).
	if *ibgpRatio < 0 {
		*ibgpRatio = 0
	}
	if *ibgpRatio > 1 {
		*ibgpRatio = 1
	}

	// Validate port.
	if *port < 1024 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "error: --port must be 1024-65535, got %d\n", *port)
		return 1
	}

	// Generate random seed if not provided.
	if *seed == 0 {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: generating random seed: %v\n", err)
			return 1
		}
		*seed = binary.BigEndian.Uint64(buf[:])
	}

	fmt.Fprintf(os.Stderr, "ze-bgp-chaos | seed: %d | peers: %d\n", *seed, *peers)

	// churnRate: steady-state route churn is parsed but not wired to peer simulators.
	_ = churnRate

	// Parse family filters.
	var familyList, excludeList []string
	if *families != "" {
		familyList = strings.Split(*families, ",")
	}
	if *excludeFamilies != "" {
		excludeList = strings.Split(*excludeFamilies, ",")
	}

	// Generate scenario from seed.
	profiles, err := scenario.Generate(scenario.GeneratorParams{
		Seed:            *seed,
		Peers:           *peers,
		IBGPRatio:       *ibgpRatio,
		LocalAS:         65000,
		Routes:          *routes,
		HeavyPeers:      *heavyPeers,
		HeavyRoutes:     *heavyRoutes,
		BasePort:        *port,
		ListenBase:      *listenBase,
		Families:        familyList,
		ExcludeFamilies: excludeList,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generating scenario: %v\n", err)
		return 1
	}

	// Generate and output Ze config.
	zeConfig := scenario.GenerateConfig(scenario.ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: *localAddr,
		BasePort:  *port,
		Profiles:  profiles,
	})

	if writeErr := writeConfig(zeConfig, *configOut, *quiet); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: writing config: %v\n", writeErr)
		return 1
	}

	// Set up context with cancellation for clean shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		if !*quiet {
			fmt.Fprintf(os.Stderr, "ze-bgp-chaos | shutting down...\n")
		}
		cancel()
	}()

	if *duration > 0 {
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	// Launch multi-peer orchestrator.
	start := time.Now()
	chaosCfg := ChaosConfig{
		Rate:     *chaosRate,
		Interval: *chaosInterval,
		Warmup:   *warmup,
	}
	orchCfg := orchestratorConfig{
		profiles:            profiles,
		seed:                *seed,
		localAddr:           *localAddr,
		zePort:              *port,
		verbose:             *verbose,
		quiet:               *quiet,
		start:               start,
		chaosCfg:            chaosCfg,
		zePID:               *zePID,
		eventLog:            *eventLog,
		metricsAddr:         *metricsAddr,
		properties:          *properties,
		convergenceDeadline: *convergenceDeadline,
	}
	return runOrchestrator(ctx, orchCfg)
}

// writeConfig writes the Ze config to the specified file or stderr.
func writeConfig(config, path string, quiet bool) error {
	if path != "" {
		return os.WriteFile(path, []byte(config), 0o600)
	}
	if !quiet {
		_, err := fmt.Fprint(os.Stderr, config)
		return err
	}
	return nil
}

// runOrchestrator launches N peer simulators and validates route propagation.
// When chaos is enabled (chaosCfg.Rate > 0), it also starts the chaos scheduler
// and wraps each peer in a reconnection loop.
func runOrchestrator(ctx context.Context, cfg orchestratorConfig) int {
	profiles := cfg.profiles
	n := len(profiles)
	addr := fmt.Sprintf("%s:%d", cfg.localAddr, cfg.zePort)
	chaosEnabled := cfg.chaosCfg.Rate > 0

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
			names := strings.Split(cfg.properties, ",")
			selected, selErr := validation.SelectProperties(all, names)
			if selErr != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", selErr)
				return 1
			}
			propEngine = validation.NewPropertyEngine(selected)
		}
	}

	// Set up reporting consumers based on CLI flags.
	reporter, cleanup, err := setupReporting(cfg, n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: setting up reporting: %v\n", err)
		return 1
	}
	defer cleanup()

	// Established state tracking (shared with scheduler goroutine).
	established := newEstablishedState(n)

	// Per-peer chaos channels (only allocated when chaos is enabled).
	var chaosChannels []chan chaos.ChaosAction
	if chaosEnabled {
		chaosChannels = make([]chan chaos.ChaosAction, n)
		for i := range n {
			chaosChannels[i] = make(chan chaos.ChaosAction, 1)
		}
	}

	// Shared event channel with generous buffer.
	events := make(chan peer.Event, n*1000)

	// Launch per-peer goroutines.
	var wg sync.WaitGroup
	for _, p := range profiles {
		wg.Add(1)
		go func(prof scenario.PeerProfile) {
			defer wg.Done()

			// Chaos channel for this peer (nil when chaos is disabled).
			var chaosCh <-chan chaos.ChaosAction
			if chaosEnabled {
				chaosCh = chaosChannels[prof.Index]
			}

			simCfg := peer.SimulatorConfig{
				Profile: peer.SimProfile{
					Index:      prof.Index,
					ASN:        prof.ASN,
					RouterID:   prof.RouterID,
					IsIBGP:     prof.IsIBGP,
					HoldTime:   prof.HoldTime,
					RouteCount: prof.RouteCount,
					Families:   prof.Families,
				},
				Seed:    cfg.seed,
				Addr:    addr,
				Events:  events,
				Chaos:   chaosCh,
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
		}(p)
	}

	// Launch scheduler goroutine (only when chaos is enabled).
	if chaosEnabled {
		go runScheduler(ctx, cfg.chaosCfg, cfg.seed, n, established, chaosChannels, cfg.quiet)
	}

	// Close events channel when all peer goroutines finish.
	go func() {
		wg.Wait()
		close(events)
	}()

	// Process events from all peers.
	for ev := range events {
		// Update established state for scheduler.
		switch ev.Type {
		case peer.EventEstablished:
			established.Set(ev.PeerIndex, true)
		case peer.EventDisconnected:
			established.Set(ev.PeerIndex, false)
		case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
			peer.EventEORSent, peer.EventError,
			peer.EventChaosExecuted, peer.EventReconnecting, peer.EventWithdrawalSent:
			// Other events don't affect established state.
		}

		ep.Process(ev)
		if propEngine != nil {
			propEngine.ProcessEvent(ev)
		}
		reporter.Process(ev)

		if cfg.verbose && ev.Type == peer.EventError {
			fmt.Fprintf(os.Stderr, "ze-bgp-chaos | peer %d | error: %v\n", ev.PeerIndex, ev.Err)
		}
	}

	// Close reporter (flush files, clear dashboard).
	if closeErr := reporter.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "error: closing reporter: %v\n", closeErr)
	}

	// Count iBGP/eBGP peers for summary.
	var ibgpCount, ebgpCount int
	for _, p := range profiles {
		if p.IsIBGP {
			ibgpCount++
		} else {
			ebgpCount++
		}
	}

	// Final validation.
	result := validation.Check(model, tracker)
	convStats := convergence.Stats()
	slow := convergence.CheckDeadline(time.Now())

	// Collect property results (nil-safe: empty slice when engine not active).
	var propResults []report.PropertyLine
	if propEngine != nil {
		for _, r := range propEngine.Results() {
			propResults = append(propResults, report.PropertyLine{
				Name: r.Name,
				Pass: r.Pass,
			})
		}
	}

	// Build and print summary.
	summary := report.Summary{
		Seed:          cfg.seed,
		Duration:      time.Since(cfg.start).Truncate(time.Millisecond),
		PeerCount:     n,
		IBGPCount:     ibgpCount,
		EBGPCount:     ebgpCount,
		Announced:     ep.Announced,
		Received:      ep.Received,
		Missing:       result.TotalMissing,
		Extra:         result.TotalExtra,
		MinLatency:    convStats.Min,
		AvgLatency:    convStats.Avg,
		MaxLatency:    convStats.Max,
		P99Latency:    convStats.P99,
		SlowRoutes:    len(slow),
		ChaosEvents:   ep.ChaosEvents,
		Reconnections: ep.Reconnections,
		Withdrawn:     ep.Withdrawn,
		Properties:    propResults,
	}

	return summary.Write(os.Stderr)
}

// setupReporting creates the Reporter with optional consumers based on CLI flags.
// Returns the reporter, a cleanup function, and any error.
func setupReporting(cfg orchestratorConfig, peerCount int) (*report.Reporter, func(), error) {
	var consumers []report.Consumer
	var cleanups []func()

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
			return nil, nil, fmt.Errorf("opening event file %s: %w", cfg.eventLog, err)
		}
		jlog := report.NewJSONLog(f, report.JSONLogConfig{
			Start:     cfg.start,
			Seed:      cfg.seed,
			Peers:     peerCount,
			ChaosRate: cfg.chaosCfg.Rate,
		})
		consumers = append(consumers, jlog)
		cleanups = append(cleanups, func() {
			if err := f.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error: closing event file: %v\n", err)
			}
		})
	}

	// Prometheus metrics: enabled when --metrics is set.
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

	r := report.NewReporter(consumers...)
	cleanup := func() {
		for _, fn := range cleanups {
			fn()
		}
	}

	return r, cleanup, nil
}

// runPeerLoop runs a peer simulator with reconnection after chaos disconnects.
// It loops until the context is cancelled, restarting the simulator each time.
func runPeerLoop(ctx context.Context, cfg peer.SimulatorConfig, peerIndex int, events chan<- peer.Event) {
	for {
		peer.RunSimulator(ctx, cfg)

		// Exit if context is cancelled (clean shutdown).
		if ctx.Err() != nil {
			return
		}

		// Emit reconnecting event.
		select {
		case events <- peer.Event{Type: peer.EventReconnecting, PeerIndex: peerIndex, Time: time.Now()}:
		case <-ctx.Done():
			return
		}

		// Brief backoff before reconnecting.
		select {
		case <-time.After(reconnectBackoff):
		case <-ctx.Done():
			return
		}
	}
}

// runScheduler runs the chaos scheduler goroutine. It ticks at the configured
// interval and dispatches chaos actions to per-peer channels.
func runScheduler(ctx context.Context, cfg ChaosConfig, seed uint64, peerCount int, es *establishedState, channels []chan chaos.ChaosAction, quiet bool) {
	sched := chaos.NewScheduler(chaos.SchedulerConfig{
		Seed:      seed,
		PeerCount: peerCount,
		Rate:      cfg.Rate,
		Interval:  cfg.Interval,
		Warmup:    cfg.Warmup,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-bgp-chaos | scheduler | %s -> peer %d\n",
						a.Action.Type, a.PeerIndex)
				}
				// Non-blocking send: if the peer is busy with a previous
				// action, skip this one rather than blocking the scheduler.
				select {
				case channels[a.PeerIndex] <- a.Action:
				default:
				}
			}
		}
	}
}

// runReplay opens an event log file and replays it through the validation model.
func runReplay(path string) int {
	f, err := os.Open(path) // #nosec G304 - path is from CLI flag, not user-controlled web input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening replay file: %v\n", err)
		return 2
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing replay file: %v\n", err)
		}
	}()
	return replay.Run(f, os.Stderr)
}

// runShrink reads a failing event log and minimizes it to the smallest
// subsequence that still triggers the same property violation.
func runShrink(path string, deadline time.Duration, verbose bool) int {
	f, err := os.Open(path) // #nosec G304 - path is from CLI flag
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening shrink file: %v\n", err)
		return 2
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing shrink file: %v\n", err)
		}
	}()

	meta, events, parseErr := shrink.ParseLog(f)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "error: parsing event log: %v\n", parseErr)
		return 2
	}

	cfg := shrink.Config{
		PeerCount: meta.Peers,
		Deadline:  deadline,
	}
	if verbose {
		cfg.Verbose = os.Stderr
	}

	result, shrinkErr := shrink.Run(events, cfg)
	if shrinkErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", shrinkErr)
		return 1
	}

	// Print human-readable summary.
	fmt.Fprintf(os.Stderr, "shrink: %d → %d events (%d iterations), property: %s\n",
		result.Original, len(result.Events), result.Iterations, result.Property)
	fmt.Fprintf(os.Stderr, "\nMinimal reproduction (%d steps):\n", len(result.Events))
	for i, ev := range result.Events {
		line := fmt.Sprintf("  %d. [peer %d] %s", i+1, ev.PeerIndex, ev.Type)
		if ev.Prefix.IsValid() {
			line += " " + ev.Prefix.String()
		}
		fmt.Fprintln(os.Stderr, line)
	}

	return 0
}

// runDiff opens two event log files and reports the first divergence.
func runDiff(path1, path2 string) int {
	f1, err := os.Open(path1) // #nosec G304 - path is from CLI flag, not user-controlled web input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening diff file 1: %v\n", err)
		return 2
	}
	defer func() {
		if err := f1.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing diff file 1: %v\n", err)
		}
	}()

	f2, err := os.Open(path2) // #nosec G304 - path is from CLI flag, not user-controlled web input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening diff file 2: %v\n", err)
		return 2
	}
	defer func() {
		if err := f2.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing diff file 2: %v\n", err)
		}
	}()

	return replay.Diff(f1, f2, os.Stderr)
}
