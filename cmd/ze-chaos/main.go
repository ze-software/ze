// Command ze-chaos is a chaos monkey tool for testing Ze BGP route server
// (route reflector) route propagation behavior.
//
// It simulates multiple BGP peers, generates deterministic route announcements
// from a seed, validates that the route reflector correctly propagates routes,
// and injects chaos events (disconnects, hold-timer expiry, etc.).
//
// Usage:
//
//	ze-chaos [options]
//	ze-chaos --seed 42 --peers 4 --duration 30s --config-out chaos.conf
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/inprocess"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/replay"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/report"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/route"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/shrink"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/validation"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/web"
	"golang.org/x/term"
)

// reconnectBackoff is the delay before a peer reconnects after a chaos disconnect.
const reconnectBackoff = 2 * time.Second

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("ze-chaos", flag.ContinueOnError)

	// Scenario flags
	seed := fs.Uint64("seed", 0, "Deterministic seed (default: random, always printed)")
	peers := fs.Int("peers", 4, "Number of simulated peers (1-50)")
	ibgpRatio := fs.Float64("ibgp-ratio", 0.3, "Fraction of peers that are iBGP (0.0-1.0)")

	// Route flags
	routes := fs.Int("routes", 1000, "Base routes per peer")
	heavyPeersPct := fs.Float64("heavy-peers", 10, "Percentage of peers sending many routes (0-100)")
	heavyRoutes := fs.Int("heavy-routes", 1_000_000, "Routes for heavy peers (default: full table)")
	churnRate := fs.Float64("churn-rate", 0.01, "Percentage of routes churning per second per peer")

	// Family flags
	families := fs.String("families", "", "Only these families (comma-sep)")
	excludeFamilies := fs.String("exclude-families", "", "Exclude these families (comma-sep)")

	// Chaos flags
	chaosRate := fs.Float64("chaos-rate", 0.1, "Per-peer probability of chaos per interval (0.0-1.0)")
	chaosInterval := fs.Duration("chaos-interval", 1*time.Second, "Time between chaos checks")

	// Route dynamics flags
	routeRate := fs.Float64("route-rate", 0.0, "Per-peer probability of route action per interval (0.0-1.0, 0=disabled)")
	routeInterval := fs.Duration("route-interval", 5*time.Second, "Time between route dynamics checks")

	// Network flags
	port := fs.Int("port", 1850, "Base BGP port for Ze to listen on")
	listenBase := fs.Int("listen-base", 1950, "Base port for tool to listen on")
	localAddr := fs.String("local-addr", "127.0.0.1", "Local address")

	// Output flags
	configOut := fs.String("config-out", "", "Write Ze config to file instead of stdout")
	eventLog := fs.String("event-log", "", "NDJSON event log file")
	metricsAddr := fs.String("metrics", "", "Prometheus metrics endpoint (addr:port)")
	webAddr := fs.String("web", "", "Live web dashboard (addr:port, e.g. :8080)")
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
	zeBinary := fs.String("ze", "", "Path to ze binary for plugin run directives (default: ze from PATH)")
	inProcess := fs.Bool("in-process", false, "Run reactor in-process with mock network and virtual clock")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ze-chaos - Chaos monkey for Ze BGP route server testing

Usage:
  ze-chaos [options] | ze -         Pipe config to Ze, diagnostics on stderr
  ze-chaos --config-out chaos.conf  Write config to file (start Ze separately)

Scenario:
  --seed <uint64>            Deterministic seed (default: random, always printed)
  --peers <N>                Number of simulated peers (default: 4, max: 50)
  --ibgp-ratio <float>       Fraction of peers that are iBGP (default: 0.3)

Routes:
  --routes <N>               Base routes per peer (default: 1000)
  --heavy-peers <pct>        Percentage of peers sending many routes (default: 10%%)
  --heavy-routes <N>         Routes for heavy peers (default: 1000000, full table)
  --churn-rate <pct>         Percentage of routes churning/s/peer (default: 0.01%%)

Families:
  --families <list>          Only these families (comma-sep, default: all)
  --exclude-families <list>  Exclude these families (comma-sep)

Chaos:
  --chaos-rate <float>       Per-peer probability of chaos per interval (default: 0.1)
  --chaos-interval <dur>     Time between chaos checks (default: 1s)

Route Dynamics:
  --route-rate <float>       Per-peer probability of route action per interval (default: 0, disabled)
  --route-interval <dur>     Time between route dynamics checks (default: 5s)

Network:
  --port <N>                 Base BGP port for Ze to listen on (default: 1850)
  --listen-base <N>          Base port for tool to listen on (default: 1950)
  --local-addr <addr>        Local address (default: 127.0.0.1)

Output:
  --config-out <path>        Write Ze config to file instead of stdout
  --event-log <path>         NDJSON event log file (replayable)
  --metrics <addr:port>      Prometheus metrics endpoint
  --web <addr:port>          Live web dashboard (e.g. :8080)
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
  --ze <path>                Path to ze binary for plugin run directives (default: ze from PATH)
  --in-process               Run reactor in-process (mock network, virtual clock)
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

	// Validate mutually exclusive modes.
	modeCount := 0
	if *shrinkFile != "" {
		modeCount++
	}
	if *replayFile != "" {
		modeCount++
	}
	if *diffFile1 != "" {
		modeCount++
	}
	if modeCount > 1 {
		fmt.Fprintf(os.Stderr, "error: --shrink, --replay, and --diff are mutually exclusive\n")
		return 1
	}

	// Warn if --properties is set in non-live modes (it has no effect there).
	if *properties != "" && modeCount > 0 {
		fmt.Fprintf(os.Stderr, "warning: --properties is ignored in shrink/replay/diff modes\n")
	}

	// Shrink mode: minimize a failing event log.
	if *shrinkFile != "" {
		return runShrink(*shrinkFile, *convergenceDeadline, *verbose)
	}

	// Replay mode: feed recorded event log through validation model.
	if *replayFile != "" {
		return runReplay(*replayFile)
	}

	// Validate --diff2 requires --diff.
	if *diffFile2 != "" && *diffFile1 == "" {
		fmt.Fprintf(os.Stderr, "error: --diff2 requires --diff\n")
		return 1
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

	// Validate route-rate.
	if *routeRate < 0 || *routeRate > 1.0 {
		fmt.Fprintf(os.Stderr, "error: --route-rate must be 0.0-1.0, got %f\n", *routeRate)
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

	// Compute heavy peer count from percentage.
	heavyPeers := int(math.Round(float64(*peers) * *heavyPeersPct / 100))
	if *heavyPeersPct > 0 && heavyPeers < 1 {
		heavyPeers = 1 // At least 1 heavy peer when percentage is non-zero.
	}

	fmt.Fprintf(os.Stderr, "\n══════════════════════════════════════════\n")
	fmt.Fprintf(os.Stderr, "  ze-chaos | seed: %d\n", *seed)
	fmt.Fprintf(os.Stderr, "  peers: %d | routes: %d | heavy: %d×%d\n", *peers, *routes, heavyPeers, *heavyRoutes)
	fmt.Fprintf(os.Stderr, "══════════════════════════════════════════\n\n")

	// When --churn-rate is specified but --route-rate is not, derive route-rate
	// from churn-rate (percentage to probability conversion).
	if *routeRate == 0 && *churnRate > 0 {
		*routeRate = *churnRate / 100 // churn-rate is a percentage, route-rate is 0.0-1.0
	}

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
		HeavyPeers:      heavyPeers,
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

	// Auto-discover ze binary: if --ze is not set, look for "ze" next to
	// the running binary (e.g., ./bin/ze-chaos → ./bin/ze). This avoids
	// requiring ze in PATH when both binaries are built to the same directory.
	if *zeBinary == "" {
		if exe, exeErr := os.Executable(); exeErr == nil {
			candidate := filepath.Join(filepath.Dir(exe), "ze")
			if _, statErr := os.Stat(candidate); statErr == nil {
				*zeBinary = candidate
			}
		}
	}

	// Generate and output Ze config.
	configParams := scenario.ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: *localAddr,
		BasePort:  *port,
		ZeBinary:  *zeBinary,
		Profiles:  profiles,
	}
	zeConfig := scenario.GenerateConfig(configParams)

	// Per-peer ports eliminate the need for loopback aliases.
	// Each peer gets a unique Ze listen port on 127.0.0.1.

	// Pre-flight: verify the base port is free before starting Ze.
	// Catches conflicts early instead of producing confusing BGP errors later.
	zeAddr := fmt.Sprintf("%s:%d", *localAddr, *port)
	if !*inProcess {
		if err := checkPortFree(zeAddr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	if writeErr := writeConfig(zeConfig, configParams, *configOut, *quiet); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: writing config: %v\n", writeErr)
		return 1
	}

	// In-process mode: run reactor and peers in the same process with
	// mock network and virtual clock. Requires --duration.
	if *inProcess {
		if *duration == 0 {
			fmt.Fprintf(os.Stderr, "error: --in-process requires --duration\n")
			return 1
		}

		// When --web is set, start the web dashboard before the simulation
		// so HTTP endpoints are accessible during the run.
		var wd *web.Dashboard
		if *webAddr != "" {
			var webErr error
			wd, webErr = web.New(web.Config{
				Addr:      *webAddr,
				PeerCount: len(profiles),
				Seed:      *seed,
			})
			if webErr != nil {
				fmt.Fprintf(os.Stderr, "error: starting web dashboard: %v\n", webErr)
				return 1
			}
			defer func() { _ = wd.Close() }()
		}

		ipCtx, ipCancel := context.WithTimeout(context.Background(), *duration+30*time.Second)
		defer ipCancel()
		ipCfg := inprocess.RunConfig{
			Profiles:  profiles,
			Seed:      *seed,
			Duration:  *duration,
			LocalAS:   65000,
			RouterID:  netip.MustParseAddr("10.0.0.1"),
			LocalAddr: *localAddr,
		}
		if wd != nil {
			ipCfg.Consumer = wd
			ipCfg.StepDelay = 1 * time.Second // Real-time pacing for web dashboard.
		}
		result, ipErr := inprocess.Run(ipCtx, ipCfg)
		if ipErr != nil {
			fmt.Fprintf(os.Stderr, "error: in-process run: %v\n", ipErr)
			return 1
		}
		fmt.Fprintf(os.Stderr, "ze-chaos | in-process complete | events: %d\n", len(result.Events))

		// When web dashboard is active, keep serving until Ctrl-C
		// so the user can explore the final state.
		if wd != nil {
			fmt.Fprintf(os.Stderr, "ze-chaos | simulation done — dashboard at %s (Ctrl-C to exit)\n", *webAddr)
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
		}
		return 0
	}

	// Set up parent context for signal handling (process lifetime).
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		if !*quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | shutting down...\n")
		}
		parentCancel()
	}()

	// Wait for Ze to start listening. In pipeline mode, Ze is reading
	// piped config and needs time to initialize — retry with backoff.
	pipeline := *configOut == ""
	if err := waitForZe(parentCtx, zeAddr, pipeline); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Restart channel: web dashboard sends new seeds here.
	restartCh := make(chan uint64, 1)

	chaosCfg := ChaosConfig{
		Rate:     *chaosRate,
		Interval: *chaosInterval,
		Warmup:   *warmup,
	}

	routeCfg := RouteConfig{
		Rate:       *routeRate,
		Interval:   *routeInterval,
		Warmup:     *warmup,
		BaseRoutes: *routes,
	}

	// Restart loop: each iteration runs the full orchestrator with a (possibly new) seed.
	// On restart, only the seed and profiles change — all other config stays the same.
	for {
		if parentCtx.Err() != nil {
			return 0
		}

		// Child context for this run. The web dashboard cancels it via onStop.
		runCtx, runCancel := context.WithCancel(parentCtx)
		if *duration > 0 {
			runCtx, runCancel = context.WithTimeout(parentCtx, *duration)
		}

		start := time.Now()
		orchCfg := orchestratorConfig{
			profiles:            profiles,
			seed:                *seed,
			localAddr:           *localAddr,
			zePort:              *port,
			verbose:             *verbose,
			quiet:               *quiet,
			start:               start,
			chaosCfg:            chaosCfg,
			routeCfg:            routeCfg,
			zePID:               *zePID,
			eventLog:            *eventLog,
			metricsAddr:         *metricsAddr,
			webAddr:             *webAddr,
			properties:          *properties,
			convergenceDeadline: *convergenceDeadline,
			restartCh:           restartCh,
			onStop:              runCancel,
		}

		exitCode := runOrchestrator(runCtx, orchCfg)
		runCancel()

		// Check for pending restart.
		select {
		case newSeed := <-restartCh:
			fmt.Fprintf(os.Stderr, "ze-chaos | restarting with seed: %d\n", newSeed)
			*seed = newSeed

			// Regenerate scenario with new seed.
			newProfiles, genErr := scenario.Generate(scenario.GeneratorParams{
				Seed:            newSeed,
				Peers:           len(profiles),
				IBGPRatio:       *ibgpRatio,
				LocalAS:         65000,
				Routes:          *routes,
				HeavyPeers:      heavyPeers,
				HeavyRoutes:     *heavyRoutes,
				BasePort:        *port,
				ListenBase:      *listenBase,
				Families:        familyList,
				ExcludeFamilies: excludeList,
			})
			if genErr != nil {
				fmt.Fprintf(os.Stderr, "error: regenerating scenario: %v\n", genErr)
				return 1
			}
			profiles = newProfiles
			continue
		default:
			return exitCode
		}
	}
}

// writeConfig writes the full Ze config to stdout (for piping to `ze -`)
// or to a file (--config-out), then prints a compact peer summary to stderr.
// When writing to stdout, a NUL byte sentinel follows the config so Ze can
// start parsing immediately. Stdout stays open — when this process exits,
// the pipe closes and Ze treats the EOF as a shutdown signal.
func writeConfig(config string, params scenario.ConfigParams, path string, quiet bool) error {
	if path != "" {
		// Explicit file: write config there, stdout is unused.
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			return err
		}
	} else {
		// Default: write config to stdout for piping (ze-chaos | ze -).
		if _, err := fmt.Fprint(os.Stdout, config); err != nil {
			return err
		}
		// Write NUL sentinel so Ze can stop reading config without EOF.
		// Stdout stays open — Ze monitors it for EOF as a shutdown signal.
		if _, err := os.Stdout.Write([]byte{0}); err != nil {
			return fmt.Errorf("writing config sentinel: %w", err)
		}
	}
	if !quiet {
		_, err := fmt.Fprint(os.Stderr, scenario.PeerSummary(params))
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

	// Per-peer chaos channels (only allocated when chaos is enabled).
	var chaosChannels []chan chaos.ChaosAction
	if chaosEnabled {
		chaosChannels = make([]chan chaos.ChaosAction, n)
		for i := range n {
			chaosChannels[i] = make(chan chaos.ChaosAction, 1)
		}
	}

	// Per-peer route dynamics channels (only allocated when route dynamics is enabled).
	routeEnabled := cfg.routeCfg.Rate > 0
	var routeChannels []chan route.Action
	if routeEnabled {
		routeChannels = make([]chan route.Action, n)
		for i := range n {
			routeChannels[i] = make(chan route.Action, 1)
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
		}(p)
	}

	// Launch scheduler goroutine (only when chaos is enabled).
	if chaosEnabled {
		go runScheduler(ctx, cfg.chaosCfg, cfg.seed, n, established, chaosChannels, rr.controlCh, cfg.quiet)
	}

	// Launch route dynamics scheduler goroutine (only when route dynamics is enabled).
	if routeEnabled {
		go runRouteScheduler(ctx, cfg.routeCfg, cfg.seed, n, established, routeChannels, rr.routeControlCh, cfg.quiet)
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
		// Update established state for scheduler.
		switch ev.Type {
		case peer.EventEstablished:
			established.Set(ev.PeerIndex, true)
		case peer.EventDisconnected:
			established.Set(ev.PeerIndex, false)
		case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
			peer.EventEORSent, peer.EventError,
			peer.EventChaosExecuted, peer.EventReconnecting, peer.EventWithdrawalSent,
			peer.EventRouteAction:
			// Other events don't affect established state.
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

	// Close reporter (flush files, clear dashboard).
	if closeErr := rr.reporter.Close(); closeErr != nil {
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
	if cfg.webAddr != "" && cfg.chaosCfg.Rate > 0 {
		controlCh = make(chan web.ControlCommand, 16)
	}

	// Route control channel for web dashboard → route scheduler communication.
	var routeControlCh chan web.ControlCommand
	if cfg.webAddr != "" && cfg.routeCfg.Rate > 0 {
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
// When controlCh is non-nil, it also processes dashboard control commands
// (pause, resume, rate change, manual trigger, stop).
func runScheduler(ctx context.Context, cfg ChaosConfig, seed uint64, peerCount int, es *establishedState, channels []chan chaos.ChaosAction, controlCh <-chan web.ControlCommand, quiet bool) {
	sched := chaos.NewScheduler(chaos.SchedulerConfig{
		Seed:      seed,
		PeerCount: peerCount,
		Rate:      cfg.Rate,
		Interval:  cfg.Interval,
		Warmup:    cfg.Warmup,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	paused := false

	// dispatchAction sends a chaos action to the peer's channel (non-blocking).
	dispatchAction := func(a chaos.ScheduledAction) {
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | %s -> peer %d\n",
				a.Action.Type, a.PeerIndex)
		}
		select {
		case channels[a.PeerIndex] <- a.Action:
		default:
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | dropped %s for peer %d (busy)\n",
					a.Action.Type, a.PeerIndex)
			}
		}
	}

	// handleControl processes a single control command. Returns true if stop requested.
	handleControl := func(cmd web.ControlCommand) bool {
		switch cmd.Type {
		case "pause":
			paused = true
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | paused\n")
			}
		case "resume":
			paused = false
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | resumed\n")
			}
		case "rate":
			sched.SetRate(cmd.Rate)
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | rate -> %.2f\n", cmd.Rate)
			}
		case "trigger":
			if cmd.Trigger != nil {
				handleManualTrigger(cmd.Trigger, peerCount, es, channels, quiet)
			}
		case "stop":
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | stopped by dashboard\n")
			}
			return true
		}
		return false
	}

	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-controlCh:
			if !ok {
				return
			}
			if handleControl(cmd) {
				return
			}
		case now := <-ticker.C:
			if paused {
				continue
			}
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				dispatchAction(a)
			}
		}
	}
}

// handleManualTrigger dispatches a manually-triggered chaos action.
func handleManualTrigger(t *web.ManualTrigger, peerCount int, es *establishedState, channels []chan chaos.ChaosAction, quiet bool) {
	actionType, ok := chaos.ActionTypeFromString(t.ActionType)
	if !ok {
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | unknown trigger action: %s\n", t.ActionType)
		}
		return
	}

	// Determine target peers.
	targets := t.Peers
	if len(targets) == 0 {
		// Pick the first established peer.
		snap := es.Snapshot()
		for i, est := range snap {
			if est {
				targets = []int{i}
				break
			}
		}
	}

	for _, idx := range targets {
		if idx < 0 || idx >= peerCount {
			continue
		}
		action := chaos.ChaosAction{Type: actionType}
		if !quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | manual %s -> peer %d\n",
				actionType, idx)
		}
		select {
		case channels[idx] <- action:
		default:
			if !quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | scheduler | dropped manual %s for peer %d (busy)\n",
					actionType, idx)
			}
		}
	}
}

// runRouteScheduler runs the route dynamics scheduler in a goroutine.
// It mirrors runScheduler but dispatches route.Action instead of chaos.ChaosAction.
// When controlCh is non-nil, it processes dashboard control commands
// (pause, resume, rate change, stop).
func runRouteScheduler(ctx context.Context, cfg RouteConfig, seed uint64, peerCount int, es *establishedState, channels []chan route.Action, controlCh <-chan web.ControlCommand, quiet bool) {
	sched := route.NewScheduler(route.SchedulerConfig{
		Seed:       seed + 1, // Different seed from chaos to avoid correlated scheduling.
		PeerCount:  peerCount,
		Rate:       cfg.Rate,
		Interval:   cfg.Interval,
		Warmup:     cfg.Warmup,
		BaseRoutes: cfg.BaseRoutes,
	})

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	paused := false

	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-controlCh:
			if !ok {
				return
			}
			switch cmd.Type {
			case "pause":
				paused = true
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | paused\n")
				}
			case "resume":
				paused = false
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | resumed\n")
				}
			case "rate":
				sched.SetRate(cmd.Rate)
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | rate -> %.2f\n", cmd.Rate)
				}
			case "stop":
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | stopped by dashboard\n")
				}
				return
			}
		case now := <-ticker.C:
			if paused {
				continue
			}
			actions := sched.Tick(now, es.Snapshot())
			for _, a := range actions {
				if !quiet {
					fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | %s -> peer %d\n",
						a.Action.Type, a.PeerIndex)
				}
				select {
				case channels[a.PeerIndex] <- a.Action:
				default:
					if !quiet {
						fmt.Fprintf(os.Stderr, "ze-chaos | route-sched | dropped %s for peer %d (busy)\n",
							a.Action.Type, a.PeerIndex)
					}
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

// checkPortFree verifies that nothing is listening on addr.
// Called before starting Ze to fail fast on port conflicts.
func checkPortFree(addr string) error {
	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		// Port is in use — connection succeeded.
		if closeErr := conn.Close(); closeErr != nil {
			return fmt.Errorf("port %s in use (close: %w)", addr, closeErr)
		}
		return fmt.Errorf("port %s is already in use — stop the existing process first", addr)
	}

	// Connection refused or timeout means nothing is listening — port is free.
	// Other errors (permission denied, network unreachable) should propagate.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return nil
		}
		if opErr.Timeout() {
			return nil
		}
	}

	return fmt.Errorf("checking port %s: %w", addr, err)
}

// waitForZe waits for Ze to start listening on addr.
// In pipeline mode, Ze is reading piped config and needs time to initialize.
// Uses TCP connect only — no BGP OPEN, to avoid corrupting the peer session
// on the probed port.
func waitForZe(ctx context.Context, addr string, pipeline bool) error {
	maxAttempts := 1
	if pipeline {
		maxAttempts = 15
	}

	var lastErr error
	for attempt := range maxAttempts {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		dialer := net.Dialer{Timeout: time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				return fmt.Errorf("probe close: %w", closeErr)
			}
			// Brief delay to let ze process the probe's EOF.
			// The probe hits a per-peer BGP port, creating a session that
			// immediately gets EOF. This sleep gives ze's FSM time to
			// reset to Idle before the real peer connects on this port.
			time.Sleep(200 * time.Millisecond)
			return nil
		}
		lastErr = err

		if attempt < maxAttempts-1 {
			select {
			case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("ze did not start within timeout on %s: %w", addr, lastErr)
}
