// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation
// Related: simulator_actions.go — chaos and route action execution
// Related: simulator_reader.go — message reading and parsing

package peer

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/chaos"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/route"
	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/scenario"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/sim"
)

// Family string constants used in event tagging and NLRI dispatch.
const (
	familyIPv4Unicast = "ipv4/unicast"
	familyIPv6Unicast = "ipv6/unicast"
)

// SimProfile holds the peer identity and route parameters for a simulator.
// It mirrors the fields from scenario.PeerProfile needed at runtime.
type SimProfile struct {
	Index      int
	ASN        uint32
	RouterID   netip.Addr
	IsIBGP     bool
	HoldTime   uint16
	RouteCount int
	TotalPeers int
	Families   []string
}

// SimulatorConfig holds all parameters for running a single peer simulator.
type SimulatorConfig struct {
	// Profile is the peer's identity and route parameters.
	Profile SimProfile

	// Seed is the scenario seed for deterministic route generation.
	Seed uint64

	// Addr is the TCP address to connect to (host:port).
	Addr string

	// LocalAddr is the local address to bind when dialing (e.g. "127.0.0.2").
	// BGP identifies peers by source address, so each simulated peer needs
	// a distinct local address. Empty means use OS default.
	LocalAddr string

	// Events is the channel to send lifecycle and route events on.
	Events chan<- Event

	// Chaos receives chaos actions from the scheduler. Nil means no chaos.
	Chaos <-chan chaos.ChaosAction

	// Routes receives route dynamics actions from the route scheduler. Nil means no route dynamics.
	Routes <-chan route.Action

	// ZePID is the Ze process ID for config-reload chaos events.
	// Zero means config-reload actions are skipped.
	ZePID int

	// Verbose enables extra debug output.
	Verbose bool

	// Quiet suppresses non-error output.
	Quiet bool

	// Conn is an optional pre-connected connection for in-process mode.
	// When non-nil, RunSimulator uses this connection instead of dialing
	// cfg.Addr via TCP. The connection must be ready for BGP message exchange.
	Conn net.Conn

	// Clock is an optional virtual clock for in-process mode.
	// When non-nil, the keepalive loop uses this clock instead of real time.
	// This allows VirtualClock.Advance() to drive keepalive timing deterministically.
	Clock sim.Clock
}

// ChaosResult describes the outcome of a chaos action on this simulator.
type ChaosResult struct {
	// Disconnected is true if the action caused a session teardown.
	Disconnected bool

	// WithdrawnPrefixes lists prefixes explicitly withdrawn by the action.
	WithdrawnPrefixes []netip.Prefix
}

// stormCycles is the number of rapid reconnect cycles in a reconnect storm.
const stormCycles = 2

// stormDelay is the pause between storm reconnect cycles.
const stormDelay = 200 * time.Millisecond

// RunSimulator runs a single BGP peer simulator. It connects to Ze, performs
// the OPEN/KEEPALIVE handshake, sends routes, reads incoming messages, and
// maintains the KEEPALIVE loop. All lifecycle events are reported via cfg.Events.
//
// RunSimulator blocks until ctx is canceled or a fatal error occurs.
func RunSimulator(ctx context.Context, cfg SimulatorConfig) {
	p := cfg.Profile

	emit := func(ev Event) {
		ev.PeerIndex = p.Index
		if ev.Time.IsZero() {
			ev.Time = time.Now()
		}
		// Try non-blocking send first (succeeds when channel has buffer space).
		// Fall back to blocking select with cancellation.
		select {
		case cfg.Events <- ev:
			return
		default:
		}
		select {
		case cfg.Events <- ev:
		case <-ctx.Done():
		}
	}

	// Connect to Ze: use pre-connected Conn if provided (in-process mode),
	// otherwise dial via TCP (external mode).
	var conn net.Conn
	if cfg.Conn != nil {
		conn = cfg.Conn
	} else {
		d := net.Dialer{}
		if cfg.LocalAddr != "" {
			d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(cfg.LocalAddr)}
		}
		var err error
		conn, err = d.DialContext(ctx, "tcp", cfg.Addr)
		if err != nil {
			if ctx.Err() != nil {
				emit(Event{Type: EventDisconnected})
				return
			}
			emit(Event{Type: EventError, Err: fmt.Errorf("connecting to %s: %w", cfg.Addr, err)})
			return
		}
	}
	defer func() { _ = conn.Close() }()

	// Context watcher: close connection when ctx is canceled, regardless
	// of which phase the simulator is in. Without this, readMsg() calls
	// during the handshake block forever on io.ReadFull with no way to
	// unblock them (the main select loop that handles ctx.Done is only
	// reachable after the handshake completes).
	connClosed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close() //nolint:errcheck,gosec // best-effort close to unblock handshake reads
		case <-connClosed:
		}
	}()
	defer close(connClosed)

	// OPEN exchange — apply a read deadline so we don't hang forever
	// if the remote side accepts TCP but never sends a BGP OPEN.
	// In in-process mode (Clock != nil), skip the deadline: the context
	// watcher goroutine above already closes the connection on cancellation,
	// and the real-time deadline can expire during runner.Run()'s initial
	// sleep, causing spurious handshake failures at scale.
	if cfg.Clock == nil {
		handshakeDeadline := 10 * time.Second
		if err := conn.SetReadDeadline(time.Now().Add(handshakeDeadline)); err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("setting handshake deadline: %w", err)})
			return
		}
	}

	open := BuildOpen(SessionConfig{
		ASN:      p.ASN,
		RouterID: p.RouterID,
		HoldTime: p.HoldTime,
		Families: p.Families,
	})
	if writeErr := writeMsg(conn, open); writeErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending OPEN: %w", writeErr)})
		return
	}

	if readErr := readMsg(conn); readErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("reading OPEN: %w", readErr)})
		return
	}

	// KEEPALIVE exchange.
	if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending KEEPALIVE: %w", writeErr)})
		return
	}

	if readErr := readMsg(conn); readErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("reading KEEPALIVE: %w", readErr)})
		return
	}

	// Clear read deadline for the session phase (keepalive loop handles its own).
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("clearing read deadline: %w", err)})
		return
	}

	// Send routes for all negotiated families.
	// IPv4 next-hop for IPv4 families, IPv6 next-hop for IPv6 families.
	// RFC 4760: MP_REACH_NLRI next-hop length must match the AFI.
	sender := NewSender(SenderConfig{
		ASN:     p.ASN,
		IsIBGP:  p.IsIBGP,
		NextHop: p.RouterID,
	})
	// Derive an IPv6 next-hop from the RouterID for IPv6 families.
	// Maps 10.255.P.Q → 2001:db8::P:Q (deterministic, unique per peer).
	rid4 := p.RouterID.As4()
	ipv6NextHop := netip.AddrFrom16([16]byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, rid4[2], rid4[3],
	})
	senderV6 := NewSender(SenderConfig{
		ASN:     p.ASN,
		IsIBGP:  p.IsIBGP,
		NextHop: ipv6NextHop,
	})

	families := p.Families
	if len(families) == 0 {
		families = []string{familyIPv4Unicast}
	}

	emit(Event{Type: EventEstablished, Families: families})

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "ze-chaos | peer %d | session established\n", p.Index)
	}

	// IPv4 unicast routes are used for chaos withdrawals.
	routes := scenario.GenerateIPv4Routes(cfg.Seed, p.Index, p.RouteCount, p.TotalPeers)
	totalSent := 0

	// Route count per family: unicast families get the full RouteCount,
	// non-unicast families (VPN, EVPN, FlowSpec) get RouteCount/4 to keep
	// total route volume manageable while still exercising all code paths.
	for _, family := range families {
		if ctx.Err() != nil {
			sendCease(conn, p.Index, cfg.Quiet)
			emit(Event{Type: EventDisconnected})
			return
		}

		var writeErr error
		switch family {
		case familyIPv4Unicast:
			for _, prefix := range routes {
				if ctx.Err() != nil {
					break
				}
				data := sender.BuildRoute(prefix)
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Prefix: prefix, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case familyIPv6Unicast:
			ipv6Routes := scenario.GenerateIPv6Routes(cfg.Seed, p.Index, p.RouteCount, p.TotalPeers)
			for _, prefix := range ipv6Routes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.BuildRoute(prefix)
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Prefix: prefix, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "ipv4/vpn":
			vpnRoutes := scenario.GenerateVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, false)
			for _, r := range vpnRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.BuildVPNRoute(r)
				if data == nil {
					continue
				}
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "ipv6/vpn":
			vpnRoutes := scenario.GenerateVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, true)
			for _, r := range vpnRoutes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.BuildVPNRoute(r)
				if data == nil {
					continue
				}
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "l2vpn/evpn":
			evpnRoutes := scenario.GenerateEVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers)
			for _, r := range evpnRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.BuildEVPNRoute(r)
				if data == nil {
					continue
				}
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "ipv4/flow":
			flowRoutes := scenario.GenerateFlowSpecRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, false)
			for _, r := range flowRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.BuildFlowSpecRoute(r)
				if data == nil {
					continue
				}
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "ipv6/flow":
			flowRoutes := scenario.GenerateFlowSpecRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, true)
			for _, r := range flowRoutes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.BuildFlowSpecRoute(r)
				if data == nil {
					continue
				}
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "ipv4/multicast":
			mcastRoutes := scenario.GenerateIPv4MulticastRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers)
			for _, prefix := range mcastRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.BuildMulticastRoute(prefix)
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Prefix: prefix, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		case "ipv6/multicast":
			mcastRoutes := scenario.GenerateIPv6MulticastRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers)
			for _, prefix := range mcastRoutes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.BuildMulticastRoute(prefix)
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Prefix: prefix, Family: family, BytesSent: int64(len(data))})
				totalSent++
			}
		}
		if writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending %s UPDATE: %w", family, writeErr)})
			return
		}
	}

	// Send End-of-RIB for each negotiated family.
	var eorBytes int64
	for _, family := range families {
		eor := BuildEOR(family)
		if eor == nil {
			fmt.Fprintf(os.Stderr, "ze-chaos | peer %d | skipping EOR for unknown family %s\n", p.Index, family)
			continue
		}
		if _, writeErr := conn.Write(eor); writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending %s EOR: %w", family, writeErr)})
			return
		}
		eorBytes += int64(len(eor))
	}
	emit(Event{Type: EventEORSent, Count: totalSent, Families: families, BytesSent: eorBytes})

	// Start reader goroutine for incoming messages from RR.
	// Uses an unbounded EventBuffer so readLoop never blocks on event emission
	// and no route events are dropped. A drain goroutine feeds the output channel.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		readLoop(ctx, conn, p.Index, cfg.Events)
	}()

	// KEEPALIVE loop with optional chaos handling.
	// When cfg.Clock is set (in-process mode), use a virtual timer that fires
	// when VirtualClock.Advance() reaches the deadline. Otherwise use real time.
	keepaliveInterval := time.Duration(p.HoldTime/3) * time.Second

	var keepaliveCh <-chan time.Time
	var keepaliveStop func()
	var keepaliveReset func()

	if cfg.Clock != nil {
		// Virtual time: one-shot timer, manually reset after each fire.
		vt := cfg.Clock.NewTimer(keepaliveInterval)
		keepaliveCh = vt.C()
		keepaliveStop = func() { vt.Stop() }
		keepaliveReset = func() { vt.Reset(keepaliveInterval) }
	} else {
		// Real time: auto-repeating ticker.
		ticker := time.NewTicker(keepaliveInterval)
		keepaliveCh = ticker.C
		keepaliveStop = ticker.Stop
		keepaliveReset = func() {} // Ticker auto-repeats.
	}
	defer keepaliveStop()

	// Nil-safe chaos channel: if nil, create a never-firing channel.
	chaosCh := cfg.Chaos
	if chaosCh == nil {
		chaosCh = make(<-chan chaos.ChaosAction)
	}

	// Nil-safe route dynamics channel: if nil, create a never-firing channel.
	routeCh := cfg.Routes
	if routeCh == nil {
		routeCh = make(<-chan route.Action)
	}

	for {
		select {
		case <-ctx.Done():
			sendCease(conn, p.Index, cfg.Quiet)
			conn.Close() //nolint:errcheck,gosec // best-effort close to unblock readLoop
			<-readerDone
			emit(Event{Type: EventDisconnected})
			return
		case <-readerDone:
			// Reader closed — connection lost.
			emit(Event{Type: EventDisconnected})
			return
		case <-keepaliveCh:
			keepaliveReset()
			if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
				conn.Close() //nolint:errcheck,gosec // best-effort close to unblock readLoop
				<-readerDone
				if ctx.Err() != nil {
					emit(Event{Type: EventDisconnected})
					return
				}
				emit(Event{Type: EventError, Err: fmt.Errorf("sending KEEPALIVE: %w", writeErr)})
				return
			}
		case action := <-chaosCh:
			result := executeChaos(ctx, action, conn, keepaliveStop, p, cfg, emit)
			emit(Event{Type: EventChaosExecuted, ChaosAction: action.Type.String()})
			if result.Disconnected {
				conn.Close() //nolint:errcheck,gosec // best-effort close to unblock readLoop
				<-readerDone
				emit(Event{Type: EventDisconnected})
				return
			}
		case action := <-routeCh:
			executeRoute(action, conn, routes, sender, p, cfg, emit)
			emit(Event{Type: EventRouteAction, RouteAction: action.Type.String()})
		}
	}
}
