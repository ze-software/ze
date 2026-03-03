// Design: docs/architecture/chaos-web-dashboard.md — chaos test orchestrator
// Detail: orchestrator.go — config types, established state, event processor
// Detail: orchestrator_run.go — orchestrator run loop and reporting setup
// Detail: scheduler.go — chaos and route dynamics schedulers
// Detail: subcommand.go — replay, shrink, diff subcommands and network utilities
//
// Command ze-chaos is a chaos monkey tool for testing Ze BGP route server
// (route server) route propagation behavior.
//
// It simulates multiple BGP peers, generates deterministic route announcements
// from a seed, validates that the route server correctly propagates routes,
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
	"flag"
	"fmt"
	"math"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // pprof server only starts when --pprof flag is set
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/inprocess"
	"codeberg.org/thomas-mangin/ze/internal/chaos/scenario"
	"codeberg.org/thomas-mangin/ze/internal/chaos/validation"
	"codeberg.org/thomas-mangin/ze/internal/chaos/web"
	_ "codeberg.org/thomas-mangin/ze/internal/plugin/all"
)

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
	pprofAddr := fs.String("pprof", "", "pprof HTTP server for ze-chaos (addr:port, e.g. :6060)")
	debugAddr := fs.String("ze-pprof", "", "pprof HTTP server for ze (injected into generated config, e.g. :6061)")
	quiet := fs.Bool("quiet", false, "Only errors and summary")
	verbose := fs.Bool("verbose", false, "Extra debug output")
	var debugLog bool
	fs.BoolVar(&debugLog, "d", false, "Enable debug logging (sets ze.log=debug, implies --verbose)")
	fs.BoolVar(&debugLog, "debug", false, "Enable debug logging (sets ze.log=debug, implies --verbose)")

	// Replay/diff/shrink flags
	replayFile := fs.String("replay", "", "Replay an event log through validation model")
	diffFile1 := fs.String("diff", "", "First event log for comparison (requires --diff2)")
	diffFile2 := fs.String("diff2", "", "Second event log for comparison")
	shrinkFile := fs.String("shrink", "", "Shrink a failing event log to minimal reproduction")

	// Property flags
	properties := fs.String("properties", "", "Comma-sep property names, or 'all' (default: disabled), 'list' to show available")
	convergenceDeadline := fs.Duration("convergence-deadline", 30*time.Second, "Convergence deadline for property checks")

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
  --pprof <addr:port>        pprof HTTP server for ze-chaos (e.g. :6060)
  --ze-pprof <addr:port>     pprof HTTP server for ze (injected into config, e.g. :6061)
  -d, --debug                Enable debug logging (sets ze.log=debug, implies --verbose)
  --quiet                    Only errors and summary
  --verbose                  Extra debug output

Replay:
  --replay <path>            Replay event log through validation model
  --diff <path>              Compare two event logs (first log)
  --diff2 <path>             Compare two event logs (second log)
  --shrink <path>            Shrink failing event log to minimal reproduction

Properties:
  --properties <names>       Comma-sep property names, 'all', or 'list'
  --convergence-deadline <d> Deadline for convergence property (default: 30s)

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

	// Apply debug logging early, before any loggers are created.
	if debugLog {
		_ = os.Setenv("ze.log", "debug")
		_ = os.Setenv("ze.log.relay", "debug")
		*verbose = true
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

	// Start pprof HTTP server if --pprof was set.
	// Uses DefaultServeMux which net/http/pprof registers handlers on.
	if *pprofAddr != "" {
		fmt.Fprintf(os.Stderr, "pprof server listening on %s\n", *pprofAddr) //nolint:gosec // stderr, not HTTP response
		go func() {
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil { //nolint:gosec // pprof is intentionally bound to user-specified address
				fmt.Fprintf(os.Stderr, "error: pprof server: %v\n", err)
			}
		}()
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
		PprofAddr: *debugAddr,
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
				Addr:               *webAddr,
				PeerCount:          len(profiles),
				Seed:               *seed,
				InitialSpeedFactor: 1,
				PeerFamilyTargets:  peerFamilyTargets(profiles),
			})
			if webErr != nil {
				fmt.Fprintf(os.Stderr, "error: starting web dashboard: %v\n", webErr)
				return 1
			}
			fmt.Fprintf(os.Stderr, "ze-chaos | web dashboard: %s\n", dashboardURL(*webAddr))
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
			ipCfg.StepDelay = 1 * time.Second  // Real-time pacing for web dashboard.
			ipCfg.StepDelayFunc = wd.StepDelay // Dynamic speed control from dashboard.
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
			fmt.Fprintf(os.Stderr, "ze-chaos | simulation done — dashboard at %s (Ctrl-C to exit)\n", dashboardURL(*webAddr))
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

	// Print the dashboard URL early so the user can open their browser
	// while Ze is still initializing. The HTTP server starts once
	// setupReporting runs inside runOrchestrator.
	if *webAddr != "" {
		fmt.Fprintf(os.Stderr, "ze-chaos | web dashboard: %s\n", dashboardURL(*webAddr))
	}

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
