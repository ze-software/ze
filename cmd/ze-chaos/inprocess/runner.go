// Design: docs/architecture/chaos-web-dashboard.md — in-process chaos runner

package inprocess

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/report"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/scenario"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/sim"
)

// RunConfig holds parameters for an in-process chaos run.
type RunConfig struct {
	// Profiles describes each simulated BGP peer.
	Profiles []scenario.PeerProfile

	// Seed is the scenario seed for deterministic route generation.
	Seed uint64

	// Duration is the virtual time duration to simulate.
	Duration time.Duration

	// LocalAS is the reactor's local ASN.
	LocalAS uint32

	// RouterID is the reactor's BGP router identifier.
	RouterID netip.Addr

	// LocalAddr is the reactor's local address for listeners.
	LocalAddr string

	// StopKeepalivesAt, when non-zero, instructs simulators to stop
	// sending keepalives after this virtual time offset. This tests
	// hold-timer expiry detection.
	StopKeepalivesAt time.Duration

	// DisconnectAt, when non-zero, closes peer 0's connection at this
	// virtual time offset. Tests session tear-down detection.
	DisconnectAt time.Duration

	// ReconnectDelay is the virtual time to wait after DisconnectAt
	// before reconnecting with a fresh mock connection. A short delay
	// may hit BGP collision detection (RFC 4271 §6.8) if the reactor
	// hasn't finished tearing down the old session. A long delay
	// (> DefaultReconnectMin = 5s) gives the reactor time to complete
	// the reconnect cycle and accept the new connection cleanly.
	ReconnectDelay time.Duration

	// Consumer receives events in real-time during the simulation.
	// When non-nil, events are forwarded as they arrive (before Run returns).
	// Used by --web to feed the dashboard during in-process mode.
	Consumer report.Consumer

	// StepDelay is the real-time pause between virtual clock advances.
	// Default (0) uses 10ms for fast simulation. Set to 1s for real-time
	// pacing when the web dashboard is active.
	StepDelay time.Duration

	// StepDelayFunc, when non-nil, is called each iteration to get the
	// current step delay. This enables dynamic speed control from the web
	// dashboard. If it returns 0, the static StepDelay is used instead.
	StepDelayFunc func() time.Duration
}

// RunResult holds the output from an in-process chaos run.
type RunResult struct {
	// Events is every lifecycle event from all peer simulators.
	Events []peer.Event
}

// Run executes an in-process chaos scenario. It creates a reactor with mock
// network and virtual clock, connects peer simulators via net.Pipe(), and
// advances virtual time to drive the simulation to completion.
//
// The function blocks until Duration virtual time has elapsed or ctx is canceled.
func Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	// Assign unique per-peer addresses (127.0.0.{2+i}) to avoid reactor map collision.
	// Set all peers to passive so the reactor only accepts incoming connections
	// (it won't try to dial out, which would fail with MockDialer).
	// Clear ZePort: mock connections are queued directly on the MockListener,
	// so per-peer TCP ports are meaningless. Without this, the reactor creates
	// per-port listeners (tcp:127.0.0.1:1850, etc.) that the runner can't find.
	for i := range cfg.Profiles {
		cfg.Profiles[i].Address = netip.MustParseAddr(fmt.Sprintf("127.0.0.%d", 2+i))
		cfg.Profiles[i].Mode = scenario.ModePassive
		cfg.Profiles[i].ZePort = 0
	}

	// Generate Ze config from profiles.
	zeConfig := scenario.GenerateConfig(scenario.ConfigParams{
		LocalAS:   cfg.LocalAS,
		RouterID:  cfg.RouterID,
		LocalAddr: cfg.LocalAddr,
		Profiles:  cfg.Profiles,
		NoPlugin:  true, // In-process mode: plugins added via CLI args to LoadReactorWithPlugins.
	})

	// Create temp directory for API socket to avoid conflicts.
	tmpDir, err := os.MkdirTemp("", "ze-chaos-inprocess-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Set API socket path via environment variable for reactor config loading.
	// Scoped narrowly: set before load, unset immediately after.
	socketPath := filepath.Join(tmpDir, "ze.socket")
	_ = os.Setenv("ze.bgp.api.socketpath", socketPath) //nolint:errcheck // best-effort env setup
	reactor, err := bgpconfig.LoadReactorWithPlugins(zeConfig, "-", []string{"ze.bgp-rs"})
	_ = os.Unsetenv("ze.bgp.api.socketpath") //nolint:errcheck // best-effort cleanup
	if err != nil {
		return nil, fmt.Errorf("create reactor: %w", err)
	}

	// Create virtual clock starting at a fixed epoch for determinism.
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := sim.NewVirtualClock(epoch)

	// Create mock network components.
	dialer := NewMockDialer()
	listenerFactory := NewMockListenerFactory()

	// Inject mock components into reactor.
	reactor.SetClock(vc)
	reactor.SetDialer(dialer)
	reactor.SetListenerFactory(listenerFactory)

	// Start reactor — creates listeners, starts API, starts peers.
	reactorCtx, reactorCancel := context.WithCancel(ctx)
	defer reactorCancel()

	if err := reactor.StartWithContext(reactorCtx); err != nil {
		return nil, fmt.Errorf("start reactor: %w", err)
	}

	// Find the MockListener for our local address.
	// The reactor's Listener calls listenerFactory.Listen("tcp", "127.0.0.1:0"),
	// so the key in MockListenerFactory is "tcp:127.0.0.1:0".
	listenAddr := cfg.LocalAddr + ":0"
	ml := listenerFactory.GetListener("tcp", listenAddr)
	if ml == nil {
		reactorCancel()
		return nil, fmt.Errorf("no mock listener found for tcp:%s", listenAddr)
	}

	// Create connection pairs and wire them up.
	cpm := NewConnPairManager()

	// Size the events channel proportionally to expected route volume.
	// Each route generates a send event; reflected routes generate receive events.
	// The channel must be large enough to absorb bursts without blocking the
	// readLoop (which would cause TCP backpressure deadlocks).
	evBuf := 0
	for i := range cfg.Profiles {
		evBuf += cfg.Profiles[i].RouteCount * max(len(cfg.Profiles[i].Families), 1)
	}
	evBuf = min(max(evBuf, 65536), 5_000_000)
	events := make(chan peer.Event, evBuf)

	// Track simulator goroutines.
	var simWg sync.WaitGroup
	simCtx, simCancel := context.WithCancel(ctx)
	defer simCancel()

	localTCPAddr := &net.TCPAddr{IP: net.ParseIP(cfg.LocalAddr), Port: 179}
	peerConns := make([]net.Conn, len(cfg.Profiles))

	for i := range cfg.Profiles {
		profile := cfg.Profiles[i]
		peerEnd, reactorEnd, pairErr := cpm.NewPair()
		if pairErr != nil {
			return nil, fmt.Errorf("create connection pair %d: %w", i, pairErr)
		}
		peerConns[i] = peerEnd

		// Wrap reactor end with TCP addresses so handleConnection can do
		// its *net.TCPAddr type assertion and peer lookup.
		peerIP := net.ParseIP(fmt.Sprintf("127.0.0.%d", 2+i))
		remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
		wrappedReactorEnd := NewConnWithAddr(reactorEnd, localTCPAddr, remoteTCPAddr)

		// Queue the reactor end on the listener — this unblocks Accept().
		ml.QueueConn(wrappedReactorEnd)

		// Start the peer simulator with the peer end of the connection.
		simWg.Add(1)
		go func(p scenario.PeerProfile, conn net.Conn) {
			defer simWg.Done()
			peer.RunSimulator(simCtx, peer.SimulatorConfig{
				Profile: peer.SimProfile{
					Index:      p.Index,
					ASN:        p.ASN,
					RouterID:   p.RouterID,
					IsIBGP:     p.IsIBGP,
					HoldTime:   p.HoldTime,
					RouteCount: p.RouteCount,
					TotalPeers: len(cfg.Profiles),
					Families:   p.Families,
				},
				Seed:   cfg.Seed,
				Addr:   "", // Not used — Conn is set.
				Events: events,
				Conn:   conn,
				Clock:  vc,
			})
		}(profile, peerEnd)
	}

	// Drain events in real-time so the channel buffer doesn't fill up.
	// When a Consumer is set (e.g., web dashboard), forward events as they arrive.
	var collectedEvents []peer.Event
	var eventsMu sync.Mutex
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		for ev := range events {
			if cfg.Consumer != nil {
				cfg.Consumer.ProcessEvent(ev)
			}
			eventsMu.Lock()
			collectedEvents = append(collectedEvents, ev)
			eventsMu.Unlock()
		}
	}()

	// Give real time for reactor startup, BGP handshake, and route exchange.
	// The base 3s covers reactor + plugin initialization (which is slower
	// under the race detector). The per-peer 500ms covers each OPEN exchange
	// competing for goroutine scheduling — the race detector adds 5-10x
	// overhead per goroutine operation, and the reactor accepts connections
	// sequentially.
	handshakeWait := 3*time.Second + time.Duration(len(cfg.Profiles))*500*time.Millisecond
	time.Sleep(handshakeWait)

	// Advance virtual time in 1-second steps with real-time pauses
	// to let goroutines process timer-fired callbacks.
	step := 1 * time.Second
	stepDelay := cfg.StepDelay
	if stepDelay == 0 {
		stepDelay = 10 * time.Millisecond
	}
	simulated := time.Duration(0)
	disconnected := false
	reconnected := false

	for simulated < cfg.Duration {
		if ctx.Err() != nil {
			break
		}

		// If StopKeepalivesAt is set and we've reached it, cancel simulators
		// so they stop sending keepalives. The reactor should then expire
		// the hold timer and tear down the session.
		if cfg.StopKeepalivesAt > 0 && simulated >= cfg.StopKeepalivesAt {
			simCancel()
		}

		// Collision test (ReconnectDelay == 0): queue new connection BEFORE
		// closing the old one so the session is still ESTABLISHED when the
		// reactor's accept loop delivers the new connection. This triggers
		// RFC 4271 §6.8 collision detection deterministically.
		//
		// ConnWithAddr.SetReadDeadline is a no-op, so the reactor's read
		// goroutine blocks on ReadFull until data or close — there is no
		// polling interval. Closing the old connection delivers EOF
		// instantly, tearing down the session in microseconds. If we
		// closed first and reconnected later (even 10ms later), the session
		// would already be gone and no collision would occur.
		if cfg.DisconnectAt > 0 && cfg.ReconnectDelay == 0 && simulated >= cfg.DisconnectAt && !disconnected {
			disconnected = true
			reconnected = true
			oldConn := peerConns[0]

			// First: queue new connection while old session is still ESTABLISHED.
			newPeerEnd, newReactorEnd, collisionErr := cpm.NewPair()
			if collisionErr != nil {
				fmt.Fprintf(os.Stderr, "collision reconnect pair: %v\n", collisionErr)
			} else {
				peerIP := net.ParseIP(fmt.Sprintf("127.0.0.%d", 2))
				remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
				wrappedEnd := NewConnWithAddr(newReactorEnd, localTCPAddr, remoteTCPAddr)
				ml.QueueConn(wrappedEnd)
				peerConns[0] = newPeerEnd

				simWg.Add(1)
				go func(p scenario.PeerProfile, conn net.Conn) {
					defer simWg.Done()
					peer.RunSimulator(simCtx, peer.SimulatorConfig{
						Profile: peer.SimProfile{
							Index:      p.Index,
							ASN:        p.ASN,
							RouterID:   p.RouterID,
							IsIBGP:     p.IsIBGP,
							HoldTime:   p.HoldTime,
							RouteCount: p.RouteCount,
							TotalPeers: len(cfg.Profiles),
							Families:   p.Families,
						},
						Seed:   cfg.Seed,
						Addr:   "",
						Events: events,
						Conn:   conn,
						Clock:  vc,
					})
				}(cfg.Profiles[0], newPeerEnd)
			}

			// Then close old connection — reactor now has two connections
			// for the same peer, triggering collision detection.
			if err := oldConn.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "collision close old conn: %v\n", err)
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Normal disconnect (with delayed reconnect).
		// The 500ms real-time wait gives the reactor time to process the
		// EOF and tear down the session before the reconnect phase.
		if cfg.DisconnectAt > 0 && cfg.ReconnectDelay > 0 && simulated >= cfg.DisconnectAt && !disconnected {
			disconnected = true
			if err := peerConns[0].Close(); err != nil {
				fmt.Fprintf(os.Stderr, "disconnect close: %v\n", err)
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Delayed reconnect: queue a fresh connection after the delay.
		// Whether this succeeds depends on ReconnectDelay relative to the
		// reactor's reconnect backoff (DefaultReconnectMin = 5s virtual):
		// - Short delay (< backoff): reactor may reject (session cycling).
		// - Long delay (> backoff): peer has recycled, accepts cleanly.
		if disconnected && !reconnected && cfg.ReconnectDelay > 0 && simulated >= cfg.DisconnectAt+cfg.ReconnectDelay {
			reconnected = true

			newPeerEnd, newReactorEnd, reconnErr := cpm.NewPair()
			if reconnErr != nil {
				fmt.Fprintf(os.Stderr, "delayed reconnect pair: %v\n", reconnErr)
			} else {
				peerIP := net.ParseIP(fmt.Sprintf("127.0.0.%d", 2))
				remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
				wrappedEnd := NewConnWithAddr(newReactorEnd, localTCPAddr, remoteTCPAddr)
				ml.QueueConn(wrappedEnd)
				peerConns[0] = newPeerEnd

				simWg.Add(1)
				go func(p scenario.PeerProfile, conn net.Conn) {
					defer simWg.Done()
					peer.RunSimulator(simCtx, peer.SimulatorConfig{
						Profile: peer.SimProfile{
							Index:      p.Index,
							ASN:        p.ASN,
							RouterID:   p.RouterID,
							IsIBGP:     p.IsIBGP,
							HoldTime:   p.HoldTime,
							RouteCount: p.RouteCount,
							TotalPeers: len(cfg.Profiles),
							Families:   p.Families,
						},
						Seed:   cfg.Seed,
						Addr:   "",
						Events: events,
						Conn:   conn,
						Clock:  vc,
					})
				}(cfg.Profiles[0], newPeerEnd)

				// Wait for the new BGP handshake to complete.
				time.Sleep(500 * time.Millisecond)
			}
		}

		vc.Advance(step)
		simulated += step

		// Dynamic speed: poll StepDelayFunc each iteration for dashboard control.
		delay := stepDelay
		if cfg.StepDelayFunc != nil {
			if d := cfg.StepDelayFunc(); d > 0 {
				delay = d
			}
		}
		time.Sleep(delay)
	}

	// Stop simulators and reactor.
	simCancel()
	simWg.Wait()

	reactorCancel()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = reactor.Wait(waitCtx)

	// Close events channel and wait for the drain goroutine to finish.
	close(events)
	<-eventsDone

	eventsMu.Lock()
	result := RunResult{Events: collectedEvents}
	eventsMu.Unlock()

	return &result, nil
}
