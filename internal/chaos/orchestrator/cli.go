// Design: docs/architecture/chaos-web-dashboard.md -- chaos CLI entry point

package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/inprocess"
	chaosmcp "github.com/ze-software/ze/internal/chaos/mcp"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/chaos/validation"
	"github.com/ze-software/ze/internal/chaos/watchdog"
	"github.com/ze-software/ze/internal/chaos/web"
	zemcp "github.com/ze-software/ze/internal/component/mcp"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Env var registrations for ze-chaos port flags.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.bgp.port", Type: "int", Default: "1850", Description: "Base BGP port for Ze to listen on"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.listen.base", Type: "int", Default: "1950", Description: "Base port for ze-chaos to listen on"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.ssh.port", Type: "int", Default: "0", Description: "Ze SSH server port (0 = disabled)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.web.ui.port", Type: "int", Default: "0", Description: "Ze web UI port (0 = disabled)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.lg.port", Type: "int", Default: "0", Description: "Ze looking glass port (0 = disabled)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.web", Type: "string", Default: "", Description: "ze-chaos live web dashboard (addr:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.metrics", Type: "string", Default: "", Description: "ze-chaos Prometheus metrics endpoint (addr:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.pprof", Type: "string", Default: "", Description: "ze-chaos pprof HTTP server (addr:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.mcp", Type: "string", Default: "", Description: "ze-chaos MCP server (addr:port)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.chaos.ze.mcp.port", Type: "int", Default: "0", Description: "Ze MCP server port injected into generated config (0 = disabled)"})
)

// cLIRun is the ze-chaos root handler body. Production entry is the registry
// closure in register.go (root command "chaos", blank-imported by
// cmd/ze/ze_chaos_run.go); exported because the ze_chaos-tagged cmd/ze tests
// drive the full CLI through it directly (same convention as env.Run).
func cLIRun(args []string) int {
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

	// Backpressure flags
	slowPeers := fs.Int("slow-peers", 0, "Number of peers that read slowly (backpressure testing)")
	slowReadDelay := fs.Duration("slow-read-delay", 1*time.Second, "Read delay for slow peers")

	// Family flags
	families := fs.String("families", "", "Only these families (comma-sep)")
	excludeFamilies := fs.String("exclude-families", "", "Exclude these families (comma-sep)")

	// Chaos flags
	chaosRate := fs.Float64("chaos-rate", 0.1, "Per-peer probability of chaos per interval (0.0-1.0)")
	chaosInterval := fs.Duration("chaos-interval", 1*time.Second, "Time between chaos checks")
	// Derived from the engine action tables so help never drifts from the real set.
	var v2tb textbuf.Buffer
	v2List := v2tb.Join(engine.V2ActionNames(), ",").String()
	chaosActions := fs.String("chaos-actions", "", v2tb.Reset().Str("Enable v2 chaos actions (comma-sep: ").Str(v2List).Byte(')').String())

	// Route dynamics flags
	routeRate := fs.Float64("route-rate", 0.0, "Per-peer probability of route action per interval (0.0-1.0, 0=disabled)")
	routeInterval := fs.Duration("route-interval", 5*time.Second, "Time between route dynamics checks")

	// Network flags (env-aware: override via ze.chaos.* env vars)
	portDefault, portDesc := env.PortDefault("ze.chaos.bgp.port", 1850, "Base BGP port for Ze to listen on")
	port := fs.Int("port", portDefault, portDesc)
	listenDefault, listenDesc := env.PortDefault("ze.chaos.listen.base", 1950, "Base port for tool to listen on")
	listenBase := fs.Int("listen-base", listenDefault, listenDesc)
	localAddr := fs.String("local-addr", "127.0.0.1", "Local address")

	// SSH flags (env-aware)
	sshDefault, sshDesc := env.PortDefault("ze.chaos.ssh.port", 0, "Enable SSH server on port (user: test, password: test)")
	sshPort := fs.Int("ssh", sshDefault, sshDesc)

	// Ze service flags (injected into generated config, env-aware)
	webUIDefault, webUIDesc := env.PortDefault("ze.chaos.web.ui.port", 0, "Enable Ze web UI on port (insecure, 127.0.0.1)")
	webUIPort := fs.Int("web-ui", webUIDefault, webUIDesc)
	lgDefault, lgDesc := env.PortDefault("ze.chaos.lg.port", 0, "Enable Ze looking glass on port (127.0.0.1)")
	lgPort := fs.Int("lg", lgDefault, lgDesc)

	// Output flags (addr:port flags are env-aware)
	configOut := fs.String("config-out", "", "Write Ze config to file instead of stdout")
	eventLog := fs.String("event-log", "", "NDJSON event log file")
	mrtFile := fs.String("mrt-file", "", "MRT file (BGP4MP records, strftime patterns supported; \"-\" writes to stdout, no rotation)")
	metricsDefault, metricsDesc := env.AddrPortDefault("ze.chaos.metrics", "", "Prometheus metrics endpoint (addr:port)")
	metricsAddr := fs.String("metrics", metricsDefault, metricsDesc)
	webDefault, webDesc := env.AddrPortDefault("ze.chaos.web", "", "Live web dashboard (addr:port, e.g. :8000)")
	webAddr := fs.String("web", webDefault, webDesc)
	pprofDefault, pprofDesc := env.AddrPortDefault("ze.chaos.pprof", "", "pprof HTTP server for ze-chaos (addr:port, e.g. :6060)")
	pprofAddr := fs.String("pprof", pprofDefault, pprofDesc)
	debugAddr := fs.String("ze-pprof", "", "pprof HTTP server for ze (injected into generated config, e.g. :6061)")
	mcpDefault, mcpDesc := env.AddrPortDefault("ze.chaos.mcp", "", "MCP server for AI queries (addr:port, e.g. :8001)")
	mcpAddr := fs.String("mcp", mcpDefault, mcpDesc)
	zeMCPDefault, zeMCPDesc := env.PortDefault("ze.chaos.ze.mcp.port", 0, "Ze MCP port injected into generated config (0 = disabled)")
	zeMCPPort := fs.Int("ze-mcp", zeMCPDefault, zeMCPDesc)
	aiHelp := fs.Bool("ai-help", false, "Print chaos MCP tool definitions and exit")
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
	application := fs.String("application", "ze", "Target BGP daemon: ze, frr, bird")
	daemonBinary := fs.String("binary", "", "Path to daemon binary (default: auto-discover based on --application)")
	inProcess := fs.Bool("in-process", false, "Run reactor in-process with mock network and virtual clock")
	configOnly := fs.Bool("config-only", false, "Generate config and exit (no orchestrator)")
	pipe := fs.Bool("pipe", false, "Write config to stdout for piping (ze-chaos --pipe | ze -)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ze-chaos - Chaos monkey for Ze BGP route server testing

Usage:
  ze-chaos [options]                        Run with forked ze (default)
  ze-chaos --application frr [options]      Run with forked FRR bgpd
  ze-chaos --application bird [options]     Run with forked BIRD
  ze-chaos --pipe [options] | ze -          Pipe config to ze (old behavior)
  ze-chaos --config-only                    Generate config to stdout and exit
  ze-chaos --config-only --application frr  Generate FRR config to stdout

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

Backpressure:
  --slow-peers <N>           Number of peers that read slowly (default: 0, disabled)
  --slow-read-delay <dur>    Read delay for slow peers (default: 1s)

Chaos:
  --chaos-rate <float>       Per-peer probability of chaos per interval (default: 0.1)
  --chaos-interval <dur>     Time between chaos checks (default: 1s)
  --chaos-actions <list>     Enable v2 actions (comma-sep: %s)

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
  --mrt-file <path>          MRT file (BGP4MP records, strftime patterns supported; "-" writes stdout, no rotation)
  --metrics <addr:port>      Prometheus metrics endpoint
  --web <addr:port>          Live web dashboard (e.g. :8000)
  --pprof <addr:port>        pprof HTTP server for ze-chaos (e.g. :6060)
  --ze-pprof <addr:port>     pprof HTTP server for ze (injected into config, e.g. :6061)
  --mcp <addr:port>          MCP server for AI queries (e.g. :8001)
  --ai-help                  Print chaos MCP tool definitions and exit
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

SSH:
  --ssh <port>               Enable SSH server on port (user: test, password: test)

Ze Services (injected into generated config):
  --web-ui <port>            Enable Ze web UI on port (insecure, 127.0.0.1)
  --lg <port>                Enable Ze looking glass on port (127.0.0.1)
  --ze-mcp <port>            Enable Ze MCP server on port (127.0.0.1)

Target:
  --application <name>       Target daemon: ze (default), frr, bird
  --binary <path>            Path to daemon binary (default: auto-discover)

Control:
  --duration <dur>           Max runtime (default: 0 = run forever until Ctrl-C)
  --warmup <dur>             Time before chaos starts (default: 5s)
  --ze-pid <N>               Ze process PID (for config-reload chaos events)
  --in-process               Run reactor in-process (mock network, virtual clock)
  --config-only              Generate config and exit (no orchestrator)
  --pipe                     Write config to stdout for piping (ze-chaos --pipe | ze -)
`, v2List)
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

	// AI help mode: print chaos MCP tool definitions and exit.
	if *aiHelp {
		printAIHelp()
		return 0
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

	// Validate --config-only / --pipe exclusivity.
	if *configOnly && *pipe {
		fmt.Fprintf(os.Stderr, "error: --config-only and --pipe are mutually exclusive\n")
		return 1
	}
	if *pipe && *configOut != "" {
		fmt.Fprintf(os.Stderr, "error: --pipe and --config-out are mutually exclusive\n")
		return 1
	}
	if (*configOnly || *pipe) && modeCount > 0 {
		fmt.Fprintf(os.Stderr, "error: --config-only/--pipe cannot be used with --shrink/--replay/--diff\n")
		return 1
	}
	if *configOnly && *inProcess {
		fmt.Fprintf(os.Stderr, "error: --config-only and --in-process are mutually exclusive\n")
		return 1
	}

	// Parse --application into a Target.
	target, targetErr := scenario.ParseTarget(*application)
	if targetErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", targetErr)
		return 1
	}

	if target != scenario.TargetZe && *pipe {
		fmt.Fprintf(os.Stderr, "error: --pipe is only supported with --application ze\n")
		return 1
	}
	if target != scenario.TargetZe && *inProcess {
		fmt.Fprintf(os.Stderr, "error: --in-process is only supported with --application ze\n")
		return 1
	}

	if target != scenario.TargetZe {
		if *sshPort > 0 {
			fmt.Fprintf(os.Stderr, "warning: --ssh is ignored with --application %s\n", *application)
		}
		if *webUIPort > 0 {
			fmt.Fprintf(os.Stderr, "warning: --web-ui is ignored with --application %s\n", *application)
		}
		if *lgPort > 0 {
			fmt.Fprintf(os.Stderr, "warning: --lg is ignored with --application %s\n", *application)
		}
		if *zeMCPPort > 0 {
			fmt.Fprintf(os.Stderr, "warning: --ze-mcp is ignored with --application %s\n", *application)
		}
		if *debugAddr != "" {
			fmt.Fprintf(os.Stderr, "warning: --ze-pprof is ignored with --application %s\n", *application)
		}
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

	// Validate slow-peers and slow-read-delay.
	if *slowPeers < 0 || *slowPeers > *peers {
		fmt.Fprintf(os.Stderr, "error: --slow-peers must be 0-%d, got %d\n", *peers, *slowPeers)
		return 1
	}
	if *slowPeers > 0 && *slowReadDelay <= 0 {
		fmt.Fprintf(os.Stderr, "error: --slow-read-delay must be positive, got %s\n", *slowReadDelay)
		return 1
	}

	// Validate port (0 = auto-allocate, otherwise 1024-65535).
	if *port != 0 && (*port < 1024 || *port > 65535) {
		fmt.Fprintf(os.Stderr, "error: --port must be 0 (auto) or 1024-65535, got %d\n", *port)
		return 1
	}

	// Check for listener port conflicts among single-port flags.
	if err := validateChaosListenerConflicts(*sshPort, *webUIPort, *lgPort, *zeMCPPort, *webAddr, *pprofAddr, *metricsAddr, *debugAddr, *mcpAddr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Check for single-port listeners falling inside allocated port ranges.
	// When port=0 (auto-allocate), bgp range [0, peers*2] won't conflict with
	// any real service (all > 1024), but listenBase range is still checked.
	if err := ValidateRangeConflicts(*port, *listenBase, *peers, *sshPort, *webUIPort, *lgPort, *zeMCPPort, *webAddr, *pprofAddr, *metricsAddr, *debugAddr, *mcpAddr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
	if *slowPeers > 0 {
		fmt.Fprintf(os.Stderr, "  slow peers: %d (delay: %s)\n", *slowPeers, *slowReadDelay)
	}
	fmt.Fprintf(os.Stderr, "══════════════════════════════════════════\n\n")

	// When --churn-rate is specified but --route-rate is not, derive route-rate
	// from churn-rate (percentage to probability conversion).
	if *routeRate == 0 && *churnRate > 0 {
		*routeRate = *churnRate / 100 // churn-rate is a percentage, route-rate is 0.0-1.0
	}

	// Parse family filters.
	var familyList, excludeList []string
	if *families != "" {
		familyList, _ = stringsx.SplitCount(*families, ",")
	}
	if *excludeFamilies != "" {
		excludeList, _ = stringsx.SplitCount(*excludeFamilies, ",")
	}

	// Port auto-allocation: --port 0 asks the kernel for a free port.
	if *port == 0 {
		allocated, allocErr := allocatePort(context.Background(), *localAddr)
		if allocErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", allocErr)
			return 1
		}
		*port = allocated
		if !*quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | allocated port: %d\n", *port)
		}
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
		SlowPeers:       *slowPeers,
		SlowReadDelay:   *slowReadDelay,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generating scenario: %v\n", err)
		return 1
	}

	// Auto-discover daemon binary: if --binary is not set, look for the
	// target's default binary next to the running binary first, then PATH.
	if *daemonBinary == "" && target == scenario.TargetZe {
		if exe, exeErr := os.Executable(); exeErr == nil {
			candidate := filepath.Join(filepath.Dir(exe), target.DefaultBinary())
			if _, statErr := os.Stat(candidate); statErr == nil {
				*daemonBinary = candidate
			}
		}
	}

	// Generate config for the target daemon.
	configParams := scenario.ConfigParams{
		LocalAS:   65000,
		RouterID:  netip.MustParseAddr("10.0.0.1"),
		LocalAddr: *localAddr,
		BasePort:  *port,
		ZeBinary:  *daemonBinary,
		Profiles:  profiles,
		PprofAddr: *debugAddr,
		SSHPort:   *sshPort,
		WebUIPort: *webUIPort,
		LGPort:    *lgPort,
		MCPPort:   *zeMCPPort,
	}

	var daemonConfig string
	switch target {
	case scenario.TargetFRR:
		daemonConfig = scenario.GenerateFRRConfig(configParams)
	case scenario.TargetBIRD:
		daemonConfig = scenario.GenerateBIRDConfig(configParams)
	default:
		daemonConfig = scenario.GenerateConfig(configParams)
	}

	// Config-only mode: output config and exit (no orchestrator).
	if *configOnly {
		if *configOut != "" {
			// "-" writes to stdout, same as omitting --config-out.
			if err := cliio.WriteFile(*configOut, []byte(daemonConfig), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "error: writing config: %v\n", err)
				return 1
			}
		} else {
			fmt.Print(daemonConfig)
		}
		if !*quiet {
			fmt.Fprint(os.Stderr, scenario.PeerSummary(configParams))
		}
		return 0
	}

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

	// Write config for pipe, config-out, and in-process modes.
	// Fork mode (default) pipes config to the child process instead.
	if *pipe || *configOut != "" || *inProcess {
		if writeErr := writeConfig(daemonConfig, configParams, *configOut, *quiet); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: writing config: %v\n", writeErr)
			return 1
		}
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
				PeerFamilyTargets:  PeerFamilyTargets(profiles),
			})
			if webErr != nil {
				fmt.Fprintf(os.Stderr, "error: starting web dashboard: %v\n", webErr)
				return 1
			}
			fmt.Fprintf(os.Stderr, "ze-chaos | web dashboard: %s\n", dashboardURL(*webAddr))
			defer func() { _ = wd.Close() }()
		}

		// Start chaos MCP server in in-process mode.
		var ipConsumer report.Consumer
		if *mcpAddr != "" && wd == nil {
			fmt.Fprintf(os.Stderr, "error: --mcp requires --web (MCP reads dashboard state)\n")
			return 1
		}
		if *mcpAddr != "" && wd != nil {
			wdCfg := watchdog.DefaultConfig()
			wdCfg.Warmup = *warmup
			ipWatchdog := watchdog.New(os.Stderr, wdCfg)
			mcpProvider := &chaosmcp.Provider{
				State:       wd.State(),
				Watchdog:    ipWatchdog,
				Convergence: validation.NewConvergence(len(profiles), *convergenceDeadline),
				Seed:        *seed,
				StartTime:   time.Now(),
				PeerCount:   len(profiles),
			}
			mcpHandler, mcpErr := zemcp.NewStreamable(zemcp.StreamableConfig{Provider: mcpProvider})
			if mcpErr != nil {
				var tb textbuf.Buffer
				os.Stderr.WriteString(tb.Str("error: chaos MCP server: ").Err(mcpErr).Byte('\n').String()) //nolint:errcheck // CLI error output
				return 1
			}
			defer mcpHandler.Close()
			mcpMux := http.NewServeMux()
			mcpMux.Handle(zemcp.Endpoint, mcpHandler)
			mcpSrv := &http.Server{Addr: *mcpAddr, Handler: mcpMux, ReadHeaderTimeout: 10 * time.Second}
			go func() {
				if err := mcpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					fmt.Fprintf(os.Stderr, "error: chaos MCP server: %v\n", err)
				}
			}()
			var mcpURLBuf textbuf.Buffer
			os.Stderr.WriteString(mcpURLBuf.Str("ze-chaos | MCP server: http://").Str(*mcpAddr).Str(zemcp.Endpoint).Byte('\n').String()) //nolint:errcheck // CLI status output
			defer func() {
				shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer shutCancel()
				if err := mcpSrv.Shutdown(shutCtx); err != nil {
					var tb textbuf.Buffer
					os.Stderr.WriteString(tb.Str("error: shutting down MCP server: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI error output
				}
			}()

			// Fan out events to both the web dashboard and watchdog.
			ipConsumer = &reporterConsumer{r: report.NewReporter(wd, ipWatchdog)}
		}

		ipCtx, ipCancel := context.WithTimeout(context.Background(), *duration+30*time.Second)
		defer ipCancel()
		ipCfg := inprocess.RunConfig{
			Profiles:      profiles,
			Seed:          *seed,
			Duration:      *duration,
			LocalAS:       65000,
			RouterID:      netip.MustParseAddr("10.0.0.1"),
			LocalAddr:     *localAddr,
			ChaosRate:     *chaosRate,
			ChaosInterval: *chaosInterval,
			RouteRate:     *routeRate,
			RouteInterval: *routeInterval,
			Warmup:        *warmup,
			BaseRoutes:    *routes,
		}
		if wd != nil {
			if ipConsumer != nil {
				ipCfg.Consumer = ipConsumer
			} else {
				ipCfg.Consumer = wd
			}
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

	// Fork mode (default): start the daemon as a child process.
	var child *zeChild
	if !*pipe && *configOut == "" {
		var forkErr error
		if target == scenario.TargetZe {
			child, forkErr = forkZe(context.Background(), daemonConfig, *daemonBinary)
		} else {
			child, forkErr = forkDaemon(context.Background(), daemonConfig, *daemonBinary, target, *port, *localAddr)
		}
		if forkErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", forkErr)
			return 1
		}
		defer child.Shutdown()
		if *zePID != 0 {
			fmt.Fprintf(os.Stderr, "warning: --ze-pid ignored in fork mode (using child pid %d)\n", child.PID())
		}
		*zePID = child.PID()
		if !*quiet {
			fmt.Fprintf(os.Stderr, "ze-chaos | forked ze (pid %d)\n", child.PID())
			fmt.Fprint(os.Stderr, scenario.PeerSummary(configParams))
		}
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
		if child != nil {
			child.Signal(syscall.SIGTERM)
		}
	}()

	// Monitor child process: detect unexpected Ze exit (crash).
	var zeCrashed atomic.Bool
	if child != nil {
		go func() {
			<-child.Done()
			if parentCtx.Err() != nil {
				return
			}
			zeCrashed.Store(true)
			if !*quiet {
				fmt.Fprintf(os.Stderr, "ze-chaos | ze crashed: %v\n", child.WaitErr())
			}
			parentCancel()
		}()
	}

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
		Rate:           *chaosRate,
		Interval:       *chaosInterval,
		Warmup:         *warmup,
		EnabledActions: parseChaosActions(*chaosActions),
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
			Profiles:            profiles,
			Target:              target,
			Seed:                *seed,
			LocalAddr:           *localAddr,
			ZePort:              *port,
			Verbose:             *verbose,
			Quiet:               *quiet,
			Start:               start,
			ChaosCfg:            chaosCfg,
			RouteCfg:            routeCfg,
			ZePID:               *zePID,
			EventLog:            *eventLog,
			MRTFile:             *mrtFile,
			MetricsAddr:         *metricsAddr,
			WebAddr:             *webAddr,
			McpAddr:             *mcpAddr,
			Properties:          *properties,
			ConvergenceDeadline: *convergenceDeadline,
			RestartCh:           restartCh,
			OnStop:              runCancel,
		}

		exitCode := RunOrchestrator(runCtx, &orchCfg)
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
				SlowPeers:       *slowPeers,
				SlowReadDelay:   *slowReadDelay,
			})
			if genErr != nil {
				fmt.Fprintf(os.Stderr, "error: regenerating scenario: %v\n", genErr)
				return 1
			}
			profiles = newProfiles
			continue
		default:
			if zeCrashed.Load() {
				return 2
			}
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
		// Explicit destination: a real file, or stdout when "-".
		if err := cliio.WriteFile(path, []byte(config), 0o600); err != nil {
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

type reporterConsumer struct{ r *report.Reporter }

func (rc *reporterConsumer) ProcessEvent(ev peer.Event) { rc.r.Process(ev) }
func (rc *reporterConsumer) Close() error               { return rc.r.Close() }

// parseChaosActions parses a comma-separated list of v2 action names into ActionTypes.
// Unknown names are silently skipped.
func parseChaosActions(s string) []engine.ActionType {
	if s == "" {
		return nil
	}
	seen := make(map[engine.ActionType]bool)
	var actions []engine.ActionType
	for name := range strings.SplitSeq(s, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		at, ok := engine.ActionTypeFromString(name)
		if ok && engine.IsV2Action(at) && !seen[at] {
			seen[at] = true
			actions = append(actions, at)
		}
	}
	return actions
}

func printAIHelp() {
	tools := (&chaosmcp.Provider{}).Tools()
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding tools: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
