package peer

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"syscall"
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
	routes := scenario.GenerateIPv4Routes(cfg.Seed, p.Index, p.RouteCount)
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
				emit(Event{Type: EventRouteSent, Prefix: prefix, Family: family})
				totalSent++
			}
		case familyIPv6Unicast:
			ipv6Routes := scenario.GenerateIPv6Routes(cfg.Seed, p.Index, p.RouteCount)
			for _, prefix := range ipv6Routes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.BuildRoute(prefix)
				if _, writeErr = conn.Write(data); writeErr != nil {
					break
				}
				emit(Event{Type: EventRouteSent, Prefix: prefix, Family: family})
				totalSent++
			}
		case "ipv4/vpn":
			vpnRoutes := scenario.GenerateVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, false)
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
				emit(Event{Type: EventRouteSent, Family: family})
				totalSent++
			}
		case "ipv6/vpn":
			vpnRoutes := scenario.GenerateVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, true)
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
				emit(Event{Type: EventRouteSent, Family: family})
				totalSent++
			}
		case "l2vpn/evpn":
			evpnRoutes := scenario.GenerateEVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4)
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
				emit(Event{Type: EventRouteSent, Family: family})
				totalSent++
			}
		case "ipv4/flow":
			flowRoutes := scenario.GenerateFlowSpecRoutes(cfg.Seed, p.Index, p.RouteCount/4, false)
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
				emit(Event{Type: EventRouteSent, Family: family})
				totalSent++
			}
		case "ipv6/flow":
			flowRoutes := scenario.GenerateFlowSpecRoutes(cfg.Seed, p.Index, p.RouteCount/4, true)
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
				emit(Event{Type: EventRouteSent, Family: family})
				totalSent++
			}
		}
		if writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending %s UPDATE: %w", family, writeErr)})
			return
		}
	}

	// Send End-of-RIB for each negotiated family.
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
	}
	emit(Event{Type: EventEORSent, Count: totalSent, Families: families})

	// Start reader goroutine for incoming messages from RR.
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

// executeChaos handles a single chaos action on the simulator's live connection.
// stopKeepalive stops the keepalive timer/ticker (works with both real and virtual time).
func executeChaos(ctx context.Context, action chaos.ChaosAction, conn net.Conn,
	stopKeepalive func(), p SimProfile, cfg SimulatorConfig, emit func(Event),
) ChaosResult {
	switch action.Type {
	case chaos.ActionTCPDisconnect:
		// Abrupt disconnect — no NOTIFICATION.
		return ChaosResult{Disconnected: true}

	case chaos.ActionNotificationCease:
		// Clean disconnect with NOTIFICATION.
		sendCease(conn, p.Index, cfg.Quiet)
		return ChaosResult{Disconnected: true}

	case chaos.ActionHoldTimerExpiry:
		// Stop sending KEEPALIVEs — Ze will detect hold-timer expiry.
		stopKeepalive()
		return ChaosResult{Disconnected: false}

	case chaos.ActionDisconnectDuringBurst:
		// During steady-state this acts like a TCP disconnect.
		// The "during burst" aspect is handled by orchestrator scheduling
		// the action before EOR is sent.
		return ChaosResult{Disconnected: true}

	case chaos.ActionReconnectStorm:
		// Rapid reconnect storm: close this connection, then rapidly
		// open/close mini-sessions to stress Ze's session handling.
		// The final reconnection is handled by runPeerLoop.
		conn.Close() //nolint:errcheck,gosec // intentional close to start storm
		executeReconnectStorm(ctx, cfg.Addr, p, emit)
		return ChaosResult{Disconnected: true}

	case chaos.ActionConnectionCollision:
		// Open a second TCP connection with the same RouterID while
		// the first is active. Tests RFC 4271 Section 6.8 collision handling.
		executeConnectionCollision(ctx, cfg.Addr, p, emit)
		return ChaosResult{Disconnected: false}

	case chaos.ActionMalformedUpdate:
		// Send an UPDATE with invalid ORIGIN value (0xFF).
		// Tests RFC 7606 revised error handling (treat-as-withdraw).
		data := BuildMalformedUpdate()
		if _, writeErr := conn.Write(data); writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending malformed UPDATE: %w", writeErr)})
		}
		return ChaosResult{Disconnected: false}

	case chaos.ActionConfigReload:
		// Send SIGHUP to the Ze process to trigger config reload.
		// No-op if ZePID is not configured.
		if cfg.ZePID > 0 {
			proc, err := os.FindProcess(cfg.ZePID)
			if err == nil {
				if sigErr := proc.Signal(syscall.SIGHUP); sigErr != nil {
					emit(Event{Type: EventError, Err: fmt.Errorf("SIGHUP to Ze (pid %d): %w", cfg.ZePID, sigErr)})
				}
			}
		}
		return ChaosResult{Disconnected: false}

	default:
		return ChaosResult{}
	}
}

// executeRoute handles a single route dynamics action on the simulator's live connection.
// Route actions never disconnect the session — they only modify the route table.
func executeRoute(action route.Action, conn net.Conn, routes []netip.Prefix,
	sender *Sender, p SimProfile, cfg SimulatorConfig, emit func(Event),
) {
	switch action.Type {
	case route.ActionChurn:
		// Churn: withdraw N routes, then immediately re-announce them.
		// Simulates normal internet route instability.
		selected := pickRandomRoutes(routes, action.ChurnCount, cfg.Seed, p.Index)
		if err := sendWithdrawal(conn, selected); err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending churn withdrawal: %w", err)})
			return
		}
		emit(Event{Type: EventWithdrawalSent, Count: len(selected)})
		// Re-announce the churned routes.
		for _, prefix := range selected {
			data := sender.BuildRoute(prefix)
			if _, writeErr := conn.Write(data); writeErr != nil {
				emit(Event{Type: EventError, Err: fmt.Errorf("sending churn re-announce: %w", writeErr)})
				return
			}
			emit(Event{Type: EventRouteSent, Prefix: prefix})
		}

	case route.ActionPartialWithdraw:
		withdrawn := withdrawFraction(conn, routes, action.WithdrawFraction, cfg.Seed, p.Index, emit)
		emit(Event{Type: EventWithdrawalSent, Count: len(withdrawn)})

	case route.ActionFullWithdraw:
		if err := sendWithdrawal(conn, routes); err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending full withdrawal: %w", err)})
		}
		emit(Event{Type: EventWithdrawalSent, Count: len(routes)})
	}
}

// pickRandomRoutes selects n random routes using a deterministic PRNG.
func pickRandomRoutes(routes []netip.Prefix, n int, seed uint64, peerIndex int) []netip.Prefix {
	if len(routes) == 0 || n <= 0 {
		return nil
	}
	if n > len(routes) {
		n = len(routes)
	}
	//nolint:gosec // Deterministic PRNG intentional for reproducibility.
	rng := rand.New(rand.NewSource(int64(seed) + int64(peerIndex) + time.Now().UnixNano()))
	indices := rng.Perm(len(routes))
	selected := make([]netip.Prefix, n)
	for i := range n {
		selected[i] = routes[indices[i]]
	}
	return selected
}

// executeReconnectStorm performs rapid connect/disconnect cycles to stress
// Ze's session handling. Each cycle does a minimal OPEN/KEEPALIVE handshake.
func executeReconnectStorm(ctx context.Context, addr string, p SimProfile, emit func(Event)) {
	d := net.Dialer{Timeout: 5 * time.Second}
	for range stormCycles {
		time.Sleep(stormDelay)

		stormConn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			break
		}

		// Minimal OPEN/KEEPALIVE handshake.
		open := BuildOpen(SessionConfig{ASN: p.ASN, RouterID: p.RouterID, HoldTime: p.HoldTime, Families: p.Families})
		if writeErr := writeMsg(stormConn, open); writeErr != nil {
			stormConn.Close() //nolint:errcheck,gosec // closing failed connection
			break
		}
		if readErr := readMsg(stormConn); readErr != nil {
			stormConn.Close() //nolint:errcheck,gosec // closing failed connection
			break
		}
		if writeErr := writeMsg(stormConn, message.NewKeepalive()); writeErr != nil {
			stormConn.Close() //nolint:errcheck,gosec // closing failed connection
			break
		}
		if readErr := readMsg(stormConn); readErr != nil {
			stormConn.Close() //nolint:errcheck,gosec // closing failed connection
			break
		}

		emit(Event{Type: EventEstablished})

		time.Sleep(stormDelay)
		stormConn.Close() //nolint:errcheck,gosec // intentional close for storm cycle
		emit(Event{Type: EventDisconnected})
	}
}

// executeConnectionCollision opens a second TCP connection with the same
// RouterID to trigger RFC 4271 Section 6.8 connection collision handling.
func executeConnectionCollision(ctx context.Context, addr string, p SimProfile, emit func(Event)) {
	d := net.Dialer{Timeout: 5 * time.Second}
	collisionConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("collision connection: %w", err)})
		return
	}

	// Send OPEN with the same RouterID to trigger collision detection.
	open := BuildOpen(SessionConfig{ASN: p.ASN, RouterID: p.RouterID, HoldTime: p.HoldTime, Families: p.Families})
	if writeErr := writeMsg(collisionConn, open); writeErr != nil {
		collisionConn.Close() //nolint:errcheck,gosec // closing failed connection
		return
	}

	// Brief pause for Ze to detect the collision.
	time.Sleep(500 * time.Millisecond)
	collisionConn.Close() //nolint:errcheck,gosec // intentional close after collision test
}

// sendWithdrawal sends a withdrawal UPDATE for the given prefixes.
func sendWithdrawal(conn net.Conn, prefixes []netip.Prefix) error {
	data := BuildWithdrawal(prefixes)
	if data == nil {
		return nil
	}
	_, err := conn.Write(data)
	return err
}

// withdrawFraction withdraws a random subset of routes and returns the
// withdrawn prefixes. Uses a deterministic PRNG derived from the seed.
func withdrawFraction(conn net.Conn, routes []netip.Prefix, fraction float64,
	seed uint64, peerIndex int, emit func(Event),
) []netip.Prefix {
	if len(routes) == 0 || fraction <= 0 {
		return nil
	}

	count := min(max(int(float64(len(routes))*fraction), 1), len(routes))

	// Shuffle and pick first 'count' routes using deterministic PRNG.
	//nolint:gosec // Deterministic PRNG intentional for reproducibility.
	rng := rand.New(rand.NewSource(int64(seed) + int64(peerIndex)))
	indices := rng.Perm(len(routes))

	selected := make([]netip.Prefix, count)
	for i := range count {
		selected[i] = routes[indices[i]]
	}

	if err := sendWithdrawal(conn, selected); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending partial withdrawal: %w", err)})
	}

	return selected
}

// readLoop reads BGP messages from conn and emits route events.
// It runs until the connection closes or ctx is canceled.
func readLoop(ctx context.Context, conn net.Conn, peerIndex int, events chan<- Event) {
	for {
		if ctx.Err() != nil {
			return
		}

		header := make([]byte, message.HeaderLen)
		if _, err := io.ReadFull(conn, header); err != nil {
			return // Connection closed.
		}

		msgLen := int(binary.BigEndian.Uint16(header[16:18]))
		if msgLen < message.HeaderLen {
			return
		}

		var body []byte
		if msgLen > message.HeaderLen {
			body = make([]byte, msgLen-message.HeaderLen)
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
		}

		if len(header) < 19 {
			return
		}
		msgType := header[18]
		if msgType != 2 { // Not UPDATE — skip (KEEPALIVE, etc.)
			continue
		}

		// Parse IPv4/unicast UPDATE for announced and withdrawn prefixes.
		parseUpdatePrefixes(body, peerIndex, events)
	}
}

// parseUpdatePrefixes extracts announced and withdrawn prefixes from an
// UPDATE message body (after the 19-byte header). Handles both IPv4/unicast
// NLRI (trailing field) and MP_REACH_NLRI / MP_UNREACH_NLRI attributes for
// IPv6/unicast.
func parseUpdatePrefixes(body []byte, peerIndex int, events chan<- Event) {
	if len(body) < 4 {
		return
	}

	// Withdrawn routes length (2 bytes).
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2

	// Parse IPv4/unicast withdrawn prefixes.
	end := off + withdrawnLen
	if end > len(body) {
		return
	}
	for off < end {
		prefix, n := parseIPv4Prefix(body[off:end])
		if n <= 0 {
			break
		}
		off += n
		// Non-blocking send: readLoop must never block on event emission,
		// otherwise TCP reads stall and backpressure deadlocks the engine.
		select {
		case events <- Event{Type: EventRouteWithdrawn, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix, Family: familyIPv4Unicast}:
		default:
		}
	}

	// Total path attribute length (2 bytes).
	if off+2 > len(body) {
		return
	}
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2

	// Walk attributes looking for MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15).
	attrEnd := min(off+attrLen, len(body))
	for off < attrEnd {
		if off+3 > attrEnd {
			break
		}
		flags := body[off]
		code := body[off+1]
		off += 2

		// Attribute length: 1 byte normally, 2 bytes if extended-length flag set.
		var aLen int
		if flags&0x10 != 0 { // Extended length.
			if off+2 > attrEnd {
				break
			}
			aLen = int(binary.BigEndian.Uint16(body[off : off+2]))
			off += 2
		} else {
			aLen = int(body[off])
			off++
		}
		if off+aLen > attrEnd {
			break
		}

		switch code {
		case 14: // MP_REACH_NLRI
			parseMPReachNLRI(body[off:off+aLen], peerIndex, events)
		case 15: // MP_UNREACH_NLRI
			parseMPUnreachNLRI(body[off:off+aLen], peerIndex, events)
		}
		off += aLen
	}
	off = attrEnd

	// Parse trailing IPv4/unicast NLRI (announced prefixes).
	for off < len(body) {
		prefix, n := parseIPv4Prefix(body[off:])
		if n <= 0 {
			break
		}
		off += n
		// Non-blocking send: readLoop must never block on event emission.
		select {
		case events <- Event{Type: EventRouteReceived, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix, Family: familyIPv4Unicast}:
		default:
		}
	}
}

// afiSafiFamily maps AFI/SAFI to the family string used throughout chaos.
// Returns empty string for unrecognized combinations.
func afiSafiFamily(afi uint16, safi uint8) string {
	switch {
	case afi == 1 && safi == 1:
		return familyIPv4Unicast
	case afi == 2 && safi == 1:
		return familyIPv6Unicast
	case afi == 1 && safi == 128:
		return "ipv4/vpn"
	case afi == 2 && safi == 128:
		return "ipv6/vpn"
	case afi == 25 && safi == 70:
		return "l2vpn/evpn"
	case afi == 1 && safi == 133:
		return "ipv4/flow"
	case afi == 2 && safi == 133:
		return "ipv6/flow"
	default:
		return ""
	}
}

// parseMPReachNLRI parses MP_REACH_NLRI (type 14) and emits EventRouteReceived.
// Format: AFI(2) + SAFI(1) + NH-len(1) + NH(variable) + reserved(1) + NLRI...
//
// For IPv4/IPv6 unicast: parses individual prefixes from the NLRI field.
// For other families (VPN, EVPN, FlowSpec): emits one event per UPDATE
// with the family tag. In the chaos simulator each UPDATE carries exactly
// one non-unicast NLRI, so the count stays accurate.
func parseMPReachNLRI(data []byte, peerIndex int, events chan<- Event) {
	if len(data) < 5 { // AFI(2) + SAFI(1) + NH-len(1) + reserved(1) minimum
		return
	}
	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	family := afiSafiFamily(afi, safi)
	if family == "" {
		return
	}
	nhLen := int(data[3])
	off := 4 + nhLen + 1 // Skip next-hop + reserved byte.
	if off > len(data) {
		return
	}

	emitNLRIEvents(data[off:], family, EventRouteReceived, peerIndex, events)
}

// parseMPUnreachNLRI parses MP_UNREACH_NLRI (type 15) and emits EventRouteWithdrawn.
// Format: AFI(2) + SAFI(1) + withdrawn-NLRI...
//
// For IPv4/IPv6 unicast: parses individual prefixes.
// For other families: emits one event per UPDATE with the family tag.
func parseMPUnreachNLRI(data []byte, peerIndex int, events chan<- Event) {
	if len(data) < 3 { // AFI(2) + SAFI(1) minimum
		return
	}
	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	family := afiSafiFamily(afi, safi)
	if family == "" {
		return
	}
	emitNLRIEvents(data[3:], family, EventRouteWithdrawn, peerIndex, events)
}

// emitNLRIEvents dispatches NLRI parsing by family and sends events.
// For unicast families, individual prefixes are parsed. For others (VPN,
// EVPN, FlowSpec), one event per UPDATE is emitted since the chaos
// simulator sends exactly one NLRI per UPDATE for non-unicast families.
func emitNLRIEvents(data []byte, family string, evType EventType, peerIndex int, events chan<- Event) {
	switch family {
	case familyIPv4Unicast:
		emitPrefixEvents(data, parseIPv4Prefix, family, evType, peerIndex, events)
	case familyIPv6Unicast:
		emitPrefixEvents(data, parseIPv6Prefix, family, evType, peerIndex, events)
	default:
		// VPN, EVPN, FlowSpec: one NLRI per UPDATE in chaos simulator.
		// Non-blocking send: readLoop must never block on event emission.
		if len(data) > 0 {
			select {
			case events <- Event{Type: evType, PeerIndex: peerIndex, Time: time.Now(), Family: family}:
			default:
			}
		}
	}
}

// emitPrefixEvents parses consecutive unicast prefixes and emits an event for each.
func emitPrefixEvents(data []byte, parse func([]byte) (netip.Prefix, int), family string, evType EventType, peerIndex int, events chan<- Event) {
	off := 0
	for off < len(data) {
		prefix, n := parse(data[off:])
		if n <= 0 {
			break
		}
		off += n
		// Non-blocking send: readLoop must never block on event emission,
		// otherwise TCP reads stall and backpressure deadlocks the engine.
		select {
		case events <- Event{Type: evType, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix, Family: family}:
		default:
		}
	}
}

// parseIPv4Prefix parses a single IPv4 prefix from wire format.
// Returns the prefix and the number of bytes consumed, or 0 on error.
func parseIPv4Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 32 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [4]byte
	copy(addr[:], data[1:1+byteLen])
	prefix := netip.PrefixFrom(netip.AddrFrom4(addr), prefixLen)

	return prefix, 1 + byteLen
}

// parseIPv6Prefix parses a single IPv6 prefix from wire format.
// Returns the prefix and the number of bytes consumed, or 0 on error.
func parseIPv6Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 128 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [16]byte
	copy(addr[:], data[1:1+byteLen])
	prefix := netip.PrefixFrom(netip.AddrFrom16(addr), prefixLen)

	return prefix, 1 + byteLen
}

// writeMsg serializes and sends a BGP message on a connection.
func writeMsg(conn net.Conn, msg message.Message) error {
	data := SerializeMessage(msg)
	_, err := conn.Write(data)
	return err
}

// readMsg reads and discards a single BGP message from the connection.
func readMsg(conn net.Conn) error {
	header := make([]byte, message.HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	msgLen := int(binary.BigEndian.Uint16(header[16:18]))
	if msgLen < message.HeaderLen {
		return fmt.Errorf("invalid message length: %d", msgLen)
	}

	if msgLen > message.HeaderLen {
		body := make([]byte, msgLen-message.HeaderLen)
		if _, err := io.ReadFull(conn, body); err != nil {
			return fmt.Errorf("reading body: %w", err)
		}
	}

	return nil
}

// sendCease sends a NOTIFICATION Cease (best-effort on shutdown).
func sendCease(conn net.Conn, peerIndex int, quiet bool) {
	notif := BuildCeaseNotification()
	_ = writeMsg(conn, notif)

	if !quiet {
		fmt.Fprintf(os.Stderr, "ze-chaos | peer %d | sent NOTIFICATION cease\n", peerIndex)
	}
}
