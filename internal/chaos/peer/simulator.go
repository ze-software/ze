// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation
// Detail: simulator_actions.go — chaos and route action execution
// Detail: simulator_connect.go — connection, handshake and the connection retry
// Detail: simulator_reader.go — message reading and parsing

package peer

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/route"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var logger = slogutil.Logger("chaos.peer")

// The family names used in event tagging and NLRI dispatch, read back from the
// registry that composed them (internal/core/family/registry.go) rather than
// spelled a second time here. Resolved once: the dispatch switch below compares
// against them for every family a simulated peer sends.
var (
	familyIPv4Unicast   = family.IPv4Unicast.String()
	familyIPv6Unicast   = family.IPv6Unicast.String()
	familyIPv4Multicast = family.IPv4Multicast.String()
	familyIPv6Multicast = family.IPv6Multicast.String()
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
	SlowRead   time.Duration

	// RouteSet is the exact IPv4 unicast routes the peer announces, each with
	// its own next hop and AS path, in place of RouteCount routes generated
	// from the seed. Nil means seed-generated routes. A profile with a route
	// set MUST name no family but IPv4 unicast: RunSimulator refuses it
	// otherwise, because the other families would still be seed-generated.
	//
	// Withdrawals and re-announcements a chaos or route action asks for stay
	// inside the set. A route-burst action still announces extra generated
	// routes, which is the disturbance that action exists for.
	RouteSet *scenario.RouteSet
}

// newIPv4Sender builds the sender of the profile's IPv4 unicast routes: the
// routes of its route set when it has one, and otherwise routes whose next hop
// is the router id and whose AS path the session kind implies.
func newIPv4Sender(p SimProfile) *Sender {
	return NewSender(SenderConfig{
		ASN:      p.ASN,
		IsIBGP:   p.IsIBGP,
		NextHop:  p.RouterID,
		RouteSet: p.RouteSet,
	})
}

// ipv4Routes answers the IPv4 unicast prefixes the profile announces: those of
// its route set when it has one, and otherwise the ones generated from seed.
func ipv4Routes(seed uint64, p SimProfile) []netip.Prefix {
	if p.RouteSet != nil {
		return p.RouteSet.Prefixes()
	}
	return scenario.GenerateIPv4Routes(seed, p.Index, p.RouteCount, p.TotalPeers)
}

// routeSetProblem answers why the profile's route set cannot be announced as
// given, or nil. A profile with no route set has none.
func routeSetProblem(p SimProfile) error {
	if p.RouteSet == nil {
		return nil
	}
	for _, name := range p.Families {
		if name != familyIPv4Unicast {
			return fmt.Errorf("peer %d has a route set and the family %s: a route set holds IPv4 unicast routes only, so remove the family",
				p.Index, name)
		}
	}
	if err := newIPv4Sender(p).RouteSetProblem(); err != nil {
		return fmt.Errorf("peer %d cannot announce its route set: %w", p.Index, err)
	}
	return nil
}

// SimulatorConfig holds all parameters for running a single peer simulator.
type SimulatorConfig struct {
	// Profile is the peer's identity and route parameters.
	Profile SimProfile

	// Seed is the scenario seed for deterministic route generation.
	Seed uint64

	// Addr is the TCP address to connect to (host:port). With Conn set it is
	// the address a connection retry dials: leave it empty and a connection
	// the peer closes before its OPEN ends the run, with no retry.
	Addr string

	// LocalAddr is the local address to bind when dialing (e.g. "127.0.0.2").
	// BGP identifies peers by source address, so each simulated peer needs
	// a distinct local address. Empty means use OS default.
	LocalAddr string

	// Events is the channel to send lifecycle and route events on.
	Events chan<- Event

	// Chaos receives chaos actions from the scheduler. Nil means no chaos.
	Chaos <-chan engine.ChaosAction

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
	// When non-nil, RunSimulator uses this connection for its first attempt
	// instead of dialing cfg.Addr via TCP. The connection must be ready for
	// BGP message exchange.
	Conn net.Conn

	// Clock is an optional virtual clock for in-process mode.
	// When non-nil, the keepalive loop uses this clock instead of real time.
	// This allows VirtualClock.Advance() to drive keepalive timing deterministically.
	Clock clock.Clock

	// Dialer is an optional network dialer for in-process mode.
	// When non-nil, executeReconnectStorm and executeConnectionCollision use
	// this instead of net.Dialer{}. Allows mock connections in in-process mode.
	Dialer interface {
		DialContext(ctx context.Context, network, address string) (net.Conn, error)
	}

	// OnSessionEnd, when non-nil, is called the moment the simulator decides the
	// current session is over. It runs before the reader drain and before
	// EventDisconnected. A watcher therefore learns that the session is dying
	// from this call, and not from that event.
	//
	// EventDisconnected cannot carry this. It is emitted only after
	// <-readerDone, and readLoop itself waits for its own drain goroutine
	// (simulator_reader.go). The event therefore trails the decision by several
	// goroutine wakeups. A caller that stops the run on peer state reads the peer
	// as healthy for that whole window, and it cuts the reconnect that follows.
	//
	// It must not block. It runs on the simulator goroutine.
	OnSessionEnd func()
}

// chaosResult describes the outcome of a chaos action on this simulator.
type chaosResult struct {
	// Disconnected is true if the action caused a session teardown.
	Disconnected bool
}

// stormCycles is the number of rapid reconnect cycles in a reconnect storm.
const stormCycles = 2

// stormDelay is the pause between storm reconnect cycles.
const stormDelay = 200 * time.Millisecond

// RunSimulator runs a single BGP peer simulator. It connects to Ze, performs
// the OPEN/KEEPALIVE handshake, sends routes, reads incoming messages, and
// maintains the KEEPALIVE loop. All lifecycle events are reported via cfg.Events.
//
// A connection Ze closes or refuses before its OPEN is retried a bounded number
// of times, each retry reported as EventReconnecting (establishSession).
//
// RunSimulator blocks until ctx is canceled or a fatal error occurs.
func RunSimulator(ctx context.Context, cfg SimulatorConfig) {
	p := cfg.Profile

	// endSession publishes the decision that this session is over, before the
	// reader drain that delays EventDisconnected. Safe to call more than once.
	var sessionEnded bool
	endSession := func() {
		if cfg.OnSessionEnd == nil || sessionEnded {
			return
		}
		sessionEnded = true
		cfg.OnSessionEnd()
	}

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

	// A route set that cannot go out as given ends the run before any session:
	// a session announcing other routes than the ones asked for would look
	// healthy and be wrong.
	if err := routeSetProblem(p); err != nil {
		emit(Event{Type: EventError, Err: err})
		return
	}

	// Connect to Ze and exchange OPEN and KEEPALIVE: on the pre-connected Conn
	// if provided (in-process mode), otherwise on a connection dialed to
	// cfg.Addr (external mode). A connection Ze closes before its OPEN is
	// tried again there, within a bound. RFC 4271 Section 8.2.2
	conn, established := establishSession(ctx, cfg, emit)
	if !established {
		return
	}
	defer func() { _ = conn.Close() }()

	// Context watcher: close connection when ctx is canceled, so that a write
	// of the session phase that blocks is unblocked before the main select
	// loop is reached. The handshake has its own, inside establishSession.
	connClosed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close() //nolint:errcheck,gosec // best-effort close to unblock session I/O
		case <-connClosed:
		}
	}()
	defer close(connClosed)

	// Send routes for all negotiated families.
	// IPv4 next-hop for IPv4 families, IPv6 next-hop for IPv6 families.
	// RFC 4760: MP_REACH_NLRI next-hop length must match the AFI.
	// The IPv4 sender announces the profile's route set when it has one.
	sender := newIPv4Sender(p)
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
		logger.Debug("session established", "peer", p.Index)
	}

	// Read while announcing. The remote peer can send UPDATEs as soon as the
	// handshake completes, even while our own initial write is blocked.
	var readDelayNs atomic.Int64
	readDelayNs.Store(int64(p.SlowRead))
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		readLoop(ctx, conn, p.Index, cfg.Events, &readDelayNs)
	}()
	// A disconnect event MUST follow the reader's final event for this session.
	closeReader := func() {
		conn.Close() //nolint:errcheck,gosec // best-effort close to unblock readLoop
		<-readerDone
	}
	finishInitial := func(err error) {
		endSession()
		planned := ctx.Err() != nil
		if planned {
			sendCease(ctx, conn, p.Index, cfg.Quiet)
		}
		closeReader()
		if err != nil && !planned {
			emit(Event{Type: EventError, Err: err})
		}
		emit(Event{Type: EventDisconnected, PlannedShutdown: planned})
	}

	// IPv4 unicast routes are used for chaos withdrawals.
	routes := ipv4Routes(cfg.Seed, p)
	totalSent := 0

	// Route count per family: unicast families get the full RouteCount,
	// non-unicast families (VPN, EVPN, FlowSpec) get RouteCount/4 to keep
	// total route volume manageable while still exercising all code paths.
	for _, fam := range families {
		if ctx.Err() != nil {
			finishInitial(nil)
			return
		}

		var writeErr error
		switch fam {
		case familyIPv4Unicast:
			for _, prefix := range routes {
				if ctx.Err() != nil {
					break
				}
				data := sender.BuildRoute(prefix)
				if writeErr = sendRouteUpdate(conn, prefix, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case familyIPv6Unicast:
			ipv6Routes := scenario.GenerateIPv6Routes(cfg.Seed, p.Index, p.RouteCount, p.TotalPeers)
			for _, prefix := range ipv6Routes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.BuildRoute(prefix)
				if writeErr = sendRouteUpdate(conn, prefix, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case "ipv4/mpls-vpn":
			vpnRoutes := scenario.GenerateVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, false)
			for _, r := range vpnRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.buildVPNRoute(r)
				if data == nil {
					continue
				}
				if writeErr = sendRouteUpdate(conn, netip.Prefix{}, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case "ipv6/mpls-vpn":
			vpnRoutes := scenario.GenerateVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, true)
			for _, r := range vpnRoutes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.buildVPNRoute(r)
				if data == nil {
					continue
				}
				if writeErr = sendRouteUpdate(conn, netip.Prefix{}, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case "l2vpn/evpn":
			evpnRoutes := scenario.GenerateEVPNRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers)
			for _, r := range evpnRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.buildEVPNRoute(r)
				if data == nil {
					continue
				}
				if writeErr = sendRouteUpdate(conn, netip.Prefix{}, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case "ipv4/flow":
			flowRoutes := scenario.GenerateFlowSpecRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, false)
			for _, r := range flowRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.buildFlowSpecRoute(r)
				if data == nil {
					continue
				}
				if writeErr = sendRouteUpdate(conn, netip.Prefix{}, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case "ipv6/flow":
			flowRoutes := scenario.GenerateFlowSpecRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers, true)
			for _, r := range flowRoutes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.buildFlowSpecRoute(r)
				if data == nil {
					continue
				}
				if writeErr = sendRouteUpdate(conn, netip.Prefix{}, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case familyIPv4Multicast:
			mcastRoutes := scenario.GenerateIPv4MulticastRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers)
			for _, prefix := range mcastRoutes {
				if ctx.Err() != nil {
					break
				}
				data := sender.buildMulticastRoute(prefix)
				if writeErr = sendRouteUpdate(conn, prefix, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		case familyIPv6Multicast:
			mcastRoutes := scenario.GenerateIPv6MulticastRoutes(cfg.Seed, p.Index, p.RouteCount/4, p.TotalPeers)
			for _, prefix := range mcastRoutes {
				if ctx.Err() != nil {
					break
				}
				data := senderV6.buildMulticastRoute(prefix)
				if writeErr = sendRouteUpdate(conn, prefix, fam, data, emit); writeErr != nil {
					break
				}
				totalSent++
			}
		}
		if ctx.Err() != nil {
			finishInitial(nil)
			return
		}
		if writeErr != nil {
			finishInitial(fmt.Errorf("sending %s UPDATE: %w", fam, writeErr))
			return
		}
	}

	// Send End-of-RIB for each negotiated family.
	var eorBytes int64
	for _, fam := range families {
		eor := BuildEOR(fam)
		if eor == nil {
			logger.Warn("skipping EOR for unknown family", "peer", p.Index, "family", fam)
			continue
		}
		if _, writeErr := conn.Write(eor); writeErr != nil {
			finishInitial(fmt.Errorf("sending %s EOR: %w", fam, writeErr))
			return
		}
		eorBytes += int64(len(eor))
	}
	emit(Event{Type: EventEORSent, Count: totalSent, Families: families, BytesSent: eorBytes})

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
		chaosCh = make(<-chan engine.ChaosAction)
	}

	// Nil-safe route dynamics channel: if nil, create a never-firing channel.
	routeCh := cfg.Routes
	if routeCh == nil {
		routeCh = make(<-chan route.Action)
	}

	for {
		select {
		case <-ctx.Done():
			endSession()
			sendCease(ctx, conn, p.Index, cfg.Quiet)
			closeReader()
			emit(Event{Type: EventDisconnected, PlannedShutdown: true})
			return
		case <-readerDone:
			// The cancellation watcher can close the socket before this select
			// receives ctx.Done(). Preserve the reason for that disconnect.
			endSession()
			emit(Event{Type: EventDisconnected, PlannedShutdown: ctx.Err() != nil})
			return
		case <-keepaliveCh:
			keepaliveReset()
			if writeErr := writeMsg(conn, message.NewKeepalive()); writeErr != nil {
				endSession()
				closeReader()
				if ctx.Err() != nil {
					emit(Event{Type: EventDisconnected, PlannedShutdown: true})
					return
				}
				emit(Event{Type: EventError, Err: fmt.Errorf("sending KEEPALIVE: %w", writeErr)})
				emit(Event{Type: EventDisconnected})
				return
			}
		case action := <-chaosCh:
			result := executeChaos(ctx, action, conn, keepaliveStop, p, cfg, emit, &readDelayNs)
			if result.Disconnected {
				// Before the event, not after. A watcher that counts executed
				// actions must never see this action complete while the peer
				// still reads as up.
				endSession()
			}
			emit(Event{Type: EventChaosExecuted, ChaosAction: action.Type.String(), ChaosParams: action.Params})
			if result.Disconnected {
				closeReader()
				emit(Event{Type: EventDisconnected})
				return
			}
		case action := <-routeCh:
			executeRoute(action, conn, routes, sender, p, cfg, emit)
			emit(Event{Type: EventRouteAction, RouteAction: action.Type.String()})
		}
	}
}

// sendRouteUpdate measures convergence from the start of a successful write.
// The remote route event can reach the observer before this write returns.
func sendRouteUpdate(conn net.Conn, prefix netip.Prefix, family string, data []byte, emit func(Event)) error {
	at := time.Now()
	if _, err := conn.Write(data); err != nil {
		return err
	}
	emit(Event{
		Type: EventRouteSent, Time: at, Prefix: prefix, Family: family,
		BytesSent: int64(len(data)), BGPMessage: data,
	})
	return nil
}
