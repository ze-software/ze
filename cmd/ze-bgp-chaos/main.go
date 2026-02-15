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
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/report"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/validation"
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
	eventFile := fs.String("event-file", "", "JSON event file")
	metricsAddr := fs.String("metrics", "", "Prometheus metrics endpoint (addr:port)")
	quiet := fs.Bool("quiet", false, "Only errors and summary")
	verbose := fs.Bool("verbose", false, "Extra debug output")

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
  --event-file <path>        JSON event file
  --metrics <addr:port>      Prometheus metrics endpoint
  --quiet                    Only errors and summary
  --verbose                  Extra debug output

Control:
  --duration <dur>           Max runtime (default: 0 = run forever until Ctrl-C)
  --warmup <dur>             Time before chaos starts (default: 5s)
  --ze-pid <N>               Ze process PID (for config-reload chaos events)
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
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

	// Suppress unused-variable warnings for flags not yet wired (later phases).
	_ = churnRate
	_ = eventFile
	_ = metricsAddr

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
	return runOrchestrator(ctx, profiles, *seed, *localAddr, *port, *verbose, *quiet, start, chaosCfg, *zePID)
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
func runOrchestrator(ctx context.Context, profiles []scenario.PeerProfile, seed uint64, localAddr string, zePort int, verbose, quiet bool, start time.Time, chaosCfg ChaosConfig, zePID int) int {
	n := len(profiles)
	addr := fmt.Sprintf("%s:%d", localAddr, zePort)
	chaosEnabled := chaosCfg.Rate > 0

	// Create validation components.
	model := validation.NewModel(n)
	tracker := validation.NewTracker(n)
	convergence := validation.NewConvergence(n, 5*time.Second)
	ep := &EventProcessor{
		Model:       model,
		Tracker:     tracker,
		Convergence: convergence,
	}

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
				Seed:    seed,
				Addr:    addr,
				Events:  events,
				Chaos:   chaosCh,
				ZePID:   zePID,
				Verbose: verbose,
				Quiet:   quiet,
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
		go runScheduler(ctx, chaosCfg, seed, n, established, chaosChannels, quiet)
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

		if verbose && ev.Type == peer.EventError {
			fmt.Fprintf(os.Stderr, "ze-bgp-chaos | peer %d | error: %v\n", ev.PeerIndex, ev.Err)
		}
	}

	// Final validation.
	result := validation.Check(model, tracker)
	convStats := convergence.Stats()
	slow := convergence.CheckDeadline(time.Now())

	// Build and print summary.
	summary := report.Summary{
		Seed:          seed,
		Duration:      time.Since(start).Truncate(time.Millisecond),
		PeerCount:     n,
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
	}

	return summary.Write(os.Stderr)
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
