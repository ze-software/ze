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

	chaossim "github.com/ze-software/ze/internal/chaos"
	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/guard"
	"github.com/ze-software/ze/internal/chaos/mocknet"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/chaos/route"
	"github.com/ze-software/ze/internal/chaos/scenario"
	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	bgpreactor "github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/core/textbuf"
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

	// DisconnectAt, when non-zero, closes peer 0's connection at the first
	// virtual time offset at or after this one where the reactor reports the
	// peer ESTABLISHED. Tests session tear-down detection. It is an earliest
	// bound, not an instant: disconnecting a session that has not come up yet
	// exercises a scenario nobody wrote, and on a slow host a fixed instant
	// lands there.
	DisconnectAt time.Duration

	// ReconnectDelay is the virtual time to wait after the disconnect ACTUALLY
	// happened before reconnecting with a fresh mock connection.
	//
	// Zero selects collision mode: the fresh connection is delivered while the
	// session is ESTABLISHED and the old one is closed only after the reactor
	// has refused it, so RFC 4271 Section 6.8 rejection is structural rather
	// than a race the accept path has to win.
	//
	// Non-zero reconnects after the reactor has left Established. Below
	// DefaultReconnectMin (5s virtual) the peer may still be cycling and refuse;
	// above it the peer has recycled and accepts cleanly.
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

	// ChaosRate is the per-peer probability of a chaos event per interval (0.0-1.0).
	// Zero disables chaos scheduling (existing behavior).
	ChaosRate float64

	// ChaosInterval is the time between chaos scheduling checks.
	// Defaults to 1s if ChaosRate > 0.
	ChaosInterval time.Duration

	// RouteRate is the per-peer probability of a route dynamics event per interval (0.0-1.0).
	// Zero disables route dynamics (existing behavior).
	RouteRate float64

	// RouteInterval is the time between route dynamics checks.
	// Defaults to 5s if RouteRate > 0.
	RouteInterval time.Duration

	// Warmup is the virtual time delay before chaos/route scheduling begins.
	// Defaults to 5s.
	Warmup time.Duration

	// BaseRoutes is the base route count per peer for churn calculations.
	// Defaults to the first profile's RouteCount if zero.
	BaseRoutes int
}

// peerEstablished reports whether the reactor considers the peer at addr to be
// in PeerStateEstablished.
//
// This reads the SAME producer the accept path's collision check reads
// (Reactor.acceptOrReject: "if peer.State() == PeerStateEstablished { reject }",
// RFC 4271 Section 6.8). Gating a disconnect on it is therefore not an
// approximation: while the peer's current connection is still open nothing else
// moves it out of Established, so the accept path is guaranteed to observe the
// same value the runner just observed.
func peerEstablished(r *bgpreactor.Reactor, addr netip.Addr) bool {
	for _, p := range r.Peers() {
		if p.Settings().Address == addr {
			return p.State() == bgpreactor.PeerStateEstablished
		}
	}
	return false
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
		cfg.Profiles[i].Address = netip.AddrFrom4([4]byte{127, 0, 0, byte(2 + i)})
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
	// Standalone: the in-process sim owns the reactor lifecycle (virtual clock,
	// mock net) and self-hosts the plugin server; there is no hub to borrow from.
	reactor, err := bgpconfig.LoadReactorWithPluginsStandalone(storage.NewFilesystem(), zeConfig, "-", []string{"ze.bgp-rs"})
	_ = os.Unsetenv("ze.bgp.api.socketpath") //nolint:errcheck // best-effort cleanup
	if err != nil {
		return nil, fmt.Errorf("create reactor: %w", err)
	}

	// Create virtual clock starting at a fixed epoch for determinism.
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vc := chaossim.NewVirtualClock(epoch)

	// Create mock network components.
	dialer := mocknet.NewMockDialer()
	listenerFactory := mocknet.NewMockListenerFactory()

	// Inject mock components into reactor.
	reactor.SetClock(vc)
	reactor.SetDialer(dialer)
	reactor.SetListenerFactory(listenerFactory)

	// Set ProcessSpawner so plugin startup can spawn internal plugins.
	// Without this, Server.runPluginPhase fails and WaitForAPIReady
	// blocks forever (virtual clock timeout never fires).
	pm := pluginmgr.NewManager()
	if err := pm.StartAll(ctx, nil, nil); err != nil {
		return nil, fmt.Errorf("start plugin manager: %w", err)
	}
	reactor.SetProcessSpawner(pm)

	// Start reactor — creates listeners, starts API, starts peers.
	reactorCtx, reactorCancel := context.WithCancel(ctx)
	defer reactorCancel()

	if err := reactor.StartWithContext(reactorCtx); err != nil {
		return nil, fmt.Errorf("start reactor: %w", err)
	}

	// Find the MockListener for our local address.
	// The reactor creates a passive-peer listener at the default BGP port (179)
	// via peerListenPort(), so the MockListenerFactory key is "tcp:127.0.0.1:179".
	var tb textbuf.Buffer
	listenAddr := tb.Str(cfg.LocalAddr).Str(":179").String()
	ml := listenerFactory.GetListener("tcp", listenAddr)
	if ml == nil {
		reactorCancel()
		return nil, fmt.Errorf("no mock listener found for tcp:%s", listenAddr)
	}

	// Create connection pairs and wire them up.
	cpm := mocknet.NewConnPairManager()

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
	peerCount := len(cfg.Profiles)

	// Set up chaos/route scheduling infrastructure when rates > 0.
	chaosEnabled := cfg.ChaosRate > 0
	routeEnabled := cfg.RouteRate > 0

	var chaosChannels []chan engine.ChaosAction
	var routeChannels []chan route.Action
	var peerGuard *guard.Guard
	var es *establishedState

	if chaosEnabled || routeEnabled {
		es = newEstablishedState(peerCount)
		peerGuard = guard.New(peerCount)

		if chaosEnabled {
			chaosChannels = make([]chan engine.ChaosAction, peerCount)
			for i := range chaosChannels {
				chaosChannels[i] = make(chan engine.ChaosAction, 1)
			}
		}
		if routeEnabled {
			routeChannels = make([]chan route.Action, peerCount)
			for i := range routeChannels {
				routeChannels[i] = make(chan route.Action, 1)
			}
		}
	}

	for i := range cfg.Profiles {
		profile := cfg.Profiles[i]
		peerEnd, reactorEnd, pairErr := cpm.NewPair()
		if pairErr != nil {
			return nil, fmt.Errorf("create connection pair %d: %w", i, pairErr)
		}
		peerConns[i] = peerEnd

		// Wrap reactor end with TCP addresses so handleConnection can do
		// its *net.TCPAddr type assertion and peer lookup.
		peerIP := net.IPv4(127, 0, 0, byte(2+i))
		remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
		wrappedReactorEnd := mocknet.NewConnWithAddr(reactorEnd, localTCPAddr, remoteTCPAddr)

		// Queue the reactor end on the listener — this unblocks Accept().
		ml.QueueConn(wrappedReactorEnd)

		// Build SimulatorConfig with optional chaos/route channels and dialer.
		simCfg := peer.SimulatorConfig{
			Profile: peer.SimProfile{
				Index:      profile.Index,
				ASN:        profile.ASN,
				RouterID:   profile.RouterID,
				IsIBGP:     profile.IsIBGP,
				HoldTime:   profile.HoldTime,
				RouteCount: profile.RouteCount,
				TotalPeers: peerCount,
				Families:   profile.Families,
				SlowRead:   profile.SlowRead,
			},
			Seed:   cfg.Seed,
			Addr:   "",
			Events: events,
			Conn:   peerEnd,
			Clock:  vc,
		}
		if chaosEnabled {
			simCfg.Chaos = chaosChannels[i]
			simCfg.Dialer = &reconnectDialer{
				cpm:       cpm,
				ml:        ml,
				localAddr: localTCPAddr,
				peerIP:    peerIP,
			}
		}
		if routeEnabled {
			simCfg.Routes = routeChannels[i]
		}

		// When chaos is enabled, wrap the simulator in a reconnection loop
		// so it restarts after chaos-induced disconnects.
		if chaosEnabled {
			simWg.Add(1)
			go func(sc peer.SimulatorConfig, idx int) {
				defer simWg.Done()
				for {
					peer.RunSimulator(simCtx, sc)
					if simCtx.Err() != nil {
						return
					}
					// Emit reconnecting event (matches external orchestrator's runPeerLoop).
					select {
					case events <- peer.Event{Type: peer.EventReconnecting, PeerIndex: idx, Time: time.Now()}:
					case <-simCtx.Done():
						return
					}
					// Create new connection pair for reconnection.
					newPeer, newReactor, err := cpm.NewPair()
					if err != nil {
						return
					}
					peerIP := net.IPv4(127, 0, 0, byte(2+idx))
					remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
					wrapped := mocknet.NewConnWithAddr(newReactor, localTCPAddr, remoteTCPAddr)
					ml.QueueConn(wrapped)
					sc.Conn = newPeer
				}
			}(simCfg, i)
		} else {
			simWg.Add(1)
			go func(sc peer.SimulatorConfig) {
				defer simWg.Done()
				peer.RunSimulator(simCtx, sc)
			}(simCfg)
		}
	}

	// Drain events in real-time so the channel buffer doesn't fill up.
	// When a Consumer is set (e.g., web dashboard), forward events as they arrive.
	// When chaos/route is enabled, also update established state and guard.
	var collectedEvents []peer.Event
	var eventsMu sync.Mutex
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		for ev := range events {
			if cfg.Consumer != nil {
				cfg.Consumer.ProcessEvent(ev)
			}
			if es != nil {
				switch ev.Type { //nolint:exhaustive // only tracking lifecycle transitions
				case peer.EventEstablished:
					es.Set(ev.PeerIndex, true)
					peerGuard.OnEstablished(ev.PeerIndex)
				case peer.EventDisconnected:
					es.Set(ev.PeerIndex, false)
					peerGuard.OnDisconnected(ev.PeerIndex)
				}
			}
			eventsMu.Lock()
			collectedEvents = append(collectedEvents, ev)
			eventsMu.Unlock()
		}
	}()

	// Give real time for reactor startup and plugin initialization.
	// The base 2s covers reactor + plugin init under the race detector.
	// The per-peer 200ms covers each connection being queued and accepted
	// sequentially. Kept short because in-process mode uses DirectBridge
	// (no external processes) and connections are pre-queued on mock listener.
	//
	// NOTE: session.Run() uses s.clock.Sleep(10ms) — a virtual clock sleep —
	// when waiting for a connection. The handshake cannot complete until the
	// virtual clock advances (via vc.Advance in the loop below). This wait
	// is only for reactor/plugin startup; the handshake happens during the
	// first few virtual time steps.
	handshakeWait := 2*time.Second + time.Duration(len(cfg.Profiles))*200*time.Millisecond
	time.Sleep(handshakeWait)

	// Start chaos/route scheduler goroutines. They read from a tick channel
	// fed by the advance loop below (virtual time drives scheduling).
	var chaosTick, routeTick chan time.Time
	if chaosEnabled {
		chaosInterval := cfg.ChaosInterval
		if chaosInterval == 0 {
			chaosInterval = 1 * time.Second
		}
		warmup := cfg.Warmup
		if warmup == 0 {
			warmup = 5 * time.Second
		}
		chaosTick = make(chan time.Time, 1)
		chaosSched := engine.NewScheduler(engine.SchedulerConfig{
			Seed:      cfg.Seed,
			PeerCount: peerCount,
			Rate:      cfg.ChaosRate,
			Interval:  chaosInterval,
			Warmup:    warmup,
		})
		go chaosSchedulerLoop(simCtx, chaosSched, peerGuard, es, chaosChannels, chaosTick)
	}
	if routeEnabled {
		routeInterval := cfg.RouteInterval
		if routeInterval == 0 {
			routeInterval = 5 * time.Second
		}
		warmup := cfg.Warmup
		if warmup == 0 {
			warmup = 5 * time.Second
		}
		baseRoutes := cfg.BaseRoutes
		if baseRoutes == 0 && len(cfg.Profiles) > 0 {
			baseRoutes = cfg.Profiles[0].RouteCount
		}
		routeTick = make(chan time.Time, 1)
		routeSched := route.NewScheduler(route.SchedulerConfig{
			Seed:       cfg.Seed + 1,
			PeerCount:  peerCount,
			Rate:       cfg.RouteRate,
			Interval:   routeInterval,
			Warmup:     warmup,
			BaseRoutes: baseRoutes,
		})
		go routeSchedulerLoop(simCtx, routeSched, peerGuard, es, routeChannels, routeTick)
	}

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
	// disconnectedAt is the virtual instant the disconnect ACTUALLY fired.
	// The reconnect gap is measured from here, not from cfg.DisconnectAt: the
	// disconnect waits for the session to reach Established, so on a slow host
	// it fires later, and measuring from the fixed offset would silently shrink
	// the gap the scenario is named for.
	disconnectedAt := time.Duration(0)
	// peer0Addr is the peer whose connection the disconnect/collision branches
	// below manipulate (Run assigns 127.0.0.{2+i} above).
	peer0Addr := cfg.Profiles[0].Address

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

		// Collision test (ReconnectDelay == 0): RFC 4271 Section 6.8 closes an
		// incoming connection while the existing one is ESTABLISHED. Both halves
		// of that premise are bound to STATE here, never to elapsed time:
		//
		//  1. Fire only once the REACTOR reports PeerStateEstablished. The
		//     simulator's own "established" event is not a substitute: it is
		//     emitted after the simulator reads ze's KEEPALIVE, while ze may
		//     still be in OpenConfirm, where acceptOrReject takes the BGP-ID
		//     comparison rail instead of the outright reject, and the remote ID
		//     wins, producing a SECOND established session.
		//  2. Close the old connection only AFTER the reactor has closed the new
		//     one. Queue-then-close orders nothing: the accept path costs a
		//     mocknet channel handoff plus a `go safeHandle` spawn before it
		//     reads peer.State(), and that races the netpoller wake of the old
		//     connection's read goroutine. Whichever won decided whether a
		//     collision happened at all, so "at most 1 established" was an
		//     assertion about the Go scheduler. Waiting on the rejection removes
		//     the race instead of widening it.
		//
		// The rejection path (accept -> handleConnection -> acceptOrReject ->
		// rejectConnectionCollision) touches no clock, so blocking the advance
		// loop on it cannot stall the virtual time the handshake needs.
		if cfg.DisconnectAt > 0 && cfg.ReconnectDelay == 0 && !chaosEnabled && simulated >= cfg.DisconnectAt && !disconnected &&
			peerEstablished(reactor, peer0Addr) {
			disconnected = true
			reconnected = true
			disconnectedAt = simulated
			oldConn := peerConns[0]

			// First: queue new connection while old session is still ESTABLISHED.
			newPeerEnd, newReactorEnd, collisionErr := cpm.NewPair()
			if collisionErr != nil {
				fmt.Fprintf(os.Stderr, "collision reconnect pair: %v\n", collisionErr)
			} else {
				peerIP := net.IPv4(127, 0, 0, 2)
				remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
				// Wrapped so the reactor's close of the refused connection
				// (its Section 6.8 verdict) is observable.
				refused := newCloseNotifier(newReactorEnd)
				wrappedEnd := mocknet.NewConnWithAddr(refused, localTCPAddr, remoteTCPAddr)
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
							SlowRead:   p.SlowRead,
						},
						Seed:   cfg.Seed,
						Addr:   "",
						Events: events,
						Conn:   conn,
						Clock:  vc,
					})
				}(cfg.Profiles[0], newPeerEnd)

				// Wait for the reactor's verdict on the colliding connection.
				// The peer is ESTABLISHED and its connection is still open, so
				// nothing can move it out of that state: acceptOrReject is
				// guaranteed to take the reject rail and close this connection.
				// ctx bounds a genuine product regression rather than pacing
				// the normal case.
				select {
				case <-refused.Closed():
				case <-ctx.Done():
				}
			}

			// Only now close the old connection. The collision has already been
			// resolved against an ESTABLISHED session, so the teardown below
			// cannot pre-empt it.
			if err := oldConn.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "collision close old conn: %v\n", err)
			}
		}

		// Normal disconnect (with delayed reconnect). Like the collision branch
		// it fires on the session's state, not on elapsed time: disconnecting a
		// session that never came up produces a scenario nobody wrote.
		//
		// No pause afterwards. The reconnect gate below waits for the reactor to
		// have actually LEFT Established, which is what a fixed post-close sleep
		// was approximating, and it does so without freezing the virtual clock.
		if cfg.DisconnectAt > 0 && cfg.ReconnectDelay > 0 && !chaosEnabled && simulated >= cfg.DisconnectAt && !disconnected &&
			peerEstablished(reactor, peer0Addr) {
			disconnected = true
			disconnectedAt = simulated
			if err := peerConns[0].Close(); err != nil {
				fmt.Fprintf(os.Stderr, "disconnect close: %v\n", err)
			}
		}

		// Delayed reconnect: queue a fresh connection after the delay.
		// Whether this succeeds depends on ReconnectDelay relative to the
		// reactor's reconnect backoff (DefaultReconnectMin = 5s virtual):
		// - Short delay (< backoff): reactor may reject (session cycling).
		// - Long delay (> backoff): peer has recycled, accepts cleanly.
		//
		// Gated on the reactor having LEFT Established, so the gap is measured
		// from the teardown the test is about rather than from our Close() call,
		// and on disconnectedAt so a late disconnect does not eat the gap.
		if disconnected && !reconnected && cfg.ReconnectDelay > 0 && simulated >= disconnectedAt+cfg.ReconnectDelay &&
			!peerEstablished(reactor, peer0Addr) {
			reconnected = true

			newPeerEnd, newReactorEnd, reconnErr := cpm.NewPair()
			if reconnErr != nil {
				fmt.Fprintf(os.Stderr, "delayed reconnect pair: %v\n", reconnErr)
			} else {
				peerIP := net.IPv4(127, 0, 0, 2)
				remoteTCPAddr := &net.TCPAddr{IP: peerIP, Port: 0}
				wrappedEnd := mocknet.NewConnWithAddr(newReactorEnd, localTCPAddr, remoteTCPAddr)
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
							SlowRead:   p.SlowRead,
						},
						Seed:   cfg.Seed,
						Addr:   "",
						Events: events,
						Conn:   conn,
						Clock:  vc,
					})
				}(cfg.Profiles[0], newPeerEnd)

				// Deliberate real-time pause, NOT a deadline to widen: this
				// scenario's whole point is whether the reconnect handshake
				// succeeds, so "wait until Established" would be waiting for the
				// answer, and on the borderline gap the answer is legitimately
				// "no", so the wait would run to ctx. It cannot become a
				// condition wait in this loop either: the handshake advances
				// only while this goroutine advances the virtual clock
				// (session.Run polls clock.Sleep), so blocking here on any
				// session state deadlocks. What it buys is real time for the
				// TCP exchange; the virtual steps that follow do the rest.
				time.Sleep(500 * time.Millisecond)
			}
		}

		vc.Advance(step)
		simulated += step

		// Feed virtual time to scheduler goroutines (non-blocking).
		now := vc.Now()
		if chaosTick != nil {
			select {
			case chaosTick <- now:
			default: // scheduler still processing previous tick
			}
		}
		if routeTick != nil {
			select {
			case routeTick <- now:
			default: // scheduler still processing previous tick
			}
		}

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

	// Virtual time MUST keep moving while everything shuts down.
	//
	// session.Run() polls for its connection with s.clock.Sleep(10ms)
	// (session.go:767), and VirtualClock.Sleep is a bare channel receive
	// (virtualclock.go:49) -- clock.Clock.Sleep takes no ctx, so a goroutine
	// parked there is unreachable by simCancel(): only Advance can release it.
	// The advance loop above burns cfg.Duration of virtual time in
	// (Duration/step)*stepDelay of REAL time -- 60s of virtual time in ~0.6s -- so
	// a chaos action that fires late in the window is still mid-handshake when the
	// loop exits. Stopping the clock here stranded ze's session mid-sleep forever:
	// it never finished the handshake, the simulator blocked forever on the reply
	// that therefore never came, and simWg.Wait() hung until the caller's context
	// tore the sockets down. Real time does not stop while a system shuts down,
	// and neither may virtual time.
	stopAdvance := make(chan struct{})
	advanceDone := make(chan struct{})
	go func() {
		defer close(advanceDone)
		ticker := time.NewTicker(stepDelay)
		defer ticker.Stop()
		for {
			select {
			case <-stopAdvance:
				return
			case <-ticker.C:
				vc.Advance(step)
			}
		}
	}()

	simWg.Wait()

	reactorCancel()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	_ = reactor.Wait(waitCtx)

	// Both are down, so nothing can still be waiting on the clock.
	close(stopAdvance)
	<-advanceDone

	// Close events channel and wait for the drain goroutine to finish.
	close(events)
	<-eventsDone

	eventsMu.Lock()
	result := RunResult{Events: collectedEvents}
	eventsMu.Unlock()

	return &result, nil
}
