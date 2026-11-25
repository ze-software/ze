// Design: docs/architecture/chaos-web-dashboard.md — chaos and route action execution
// Overview: simulator.go — main simulation loop and types
// Related: simulator_reader.go — message reading and parsing

package peer

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/route"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// defaultSlowReadDelay is the read delay applied when ActionSlowRead toggles
// on a peer that wasn't configured with --slow-read-delay.
const defaultSlowReadDelay = 1 * time.Second

// executeChaos handles a single chaos action on the simulator's live connection.
// stopKeepalive stops the keepalive timer/ticker (works with both real and virtual time).
// readDelayNs is the atomic read delay (nanoseconds) shared with readLoop.
func executeChaos(ctx context.Context, action engine.ChaosAction, conn net.Conn,
	stopKeepalive func(), p SimProfile, cfg SimulatorConfig, emit func(Event),
	readDelayNs *atomic.Int64,
) chaosResult {
	switch action.Type {
	case engine.ActionTCPDisconnect:
		// Abrupt disconnect — no NOTIFICATION.
		return chaosResult{Disconnected: true}

	case engine.ActionNotificationCease:
		// Clean disconnect with NOTIFICATION.
		sendCease(ctx, conn, p.Index, cfg.Quiet)
		return chaosResult{Disconnected: true}

	case engine.ActionHoldTimerExpiry:
		// Stop sending KEEPALIVEs — Ze will detect hold-timer expiry.
		stopKeepalive()
		return chaosResult{Disconnected: false}

	case engine.ActionDisconnectDuringBurst:
		// During steady-state this acts like a TCP disconnect.
		// The "during burst" aspect is handled by orchestrator scheduling
		// the action before EOR is sent.
		return chaosResult{Disconnected: true}

	case engine.ActionReconnectStorm:
		// Rapid reconnect storm: close this connection, then rapidly
		// open/close mini-sessions to stress Ze's session handling.
		// The final reconnection is handled by runPeerLoop.
		conn.Close() //nolint:errcheck,gosec // intentional close to start storm
		executeReconnectStorm(ctx, cfg.Addr, p, cfg.Dialer, emit)
		return chaosResult{Disconnected: true}

	case engine.ActionConnectionCollision:
		// Open a second TCP connection with the same RouterID while
		// the first is active. Tests RFC 4271 Section 6.8 collision handling.
		executeConnectionCollision(ctx, cfg.Addr, p, cfg.Dialer, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionMalformedUpdate:
		// Send an UPDATE with invalid ORIGIN value (0xFF).
		// Tests RFC 7606 revised error handling (treat-as-withdraw).
		data := buildMalformedUpdate()
		if _, writeErr := conn.Write(data); writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending malformed UPDATE: %w", writeErr)})
		}
		return chaosResult{Disconnected: false}

	case engine.ActionConfigReload:
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
		return chaosResult{Disconnected: false}

	case engine.ActionSlowRead:
		// Toggle slow reading: if currently reading normally, enable delay;
		// if already slow, restore normal speed.
		if readDelayNs.Load() == 0 {
			delay := defaultSlowReadDelay
			if p.SlowRead > 0 {
				delay = p.SlowRead
			}
			readDelayNs.Store(int64(delay))
		} else {
			readDelayNs.Store(0)
		}
		return chaosResult{Disconnected: false}

	case engine.ActionClockDrift:
		executeClockDrift(ctx, action, conn, p, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionRouteBurst:
		executeRouteBurst(ctx, action, conn, p, cfg, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionWithdrawalBurst:
		executeWithdrawalBurst(action, conn, p, cfg, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionRouteFlap:
		executeRouteFlap(ctx, action, conn, p, cfg, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionSlowPeer:
		go executeSlowPeer(ctx, action, conn, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionZeroWindow:
		go executeZeroWindow(ctx, action, conn, emit)
		return chaosResult{Disconnected: false}

	case engine.ActionIfaceLinkFlap:
		// netns-scoped: brings the configured interface down then up. On a
		// non-netns loopback run (no iface param) this is a graceful no-op.
		return executeIfaceLinkFlap(action, emit)

	case engine.ActionIfaceAddrRemove:
		// netns-scoped: removes then restores an address on the interface.
		return executeIfaceAddrRemove(action, emit)
	default:
		panic("BUG: invalid executed chaos action")
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
		if err := sendWithdrawal(conn, selected, emit); err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending churn withdrawal: %w", err)})
			return
		}
		// Re-announce the churned routes.
		for _, prefix := range selected {
			data := sender.BuildRoute(prefix)
			if writeErr := sendRouteUpdate(conn, prefix, familyIPv4Unicast, data, emit); writeErr != nil {
				emit(Event{Type: EventError, Err: fmt.Errorf("sending churn re-announce: %w", writeErr)})
				return
			}
		}

	case route.ActionPartialWithdraw:
		if err := withdrawFraction(conn, routes, action.WithdrawFraction, cfg.Seed, p.Index, emit); err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending partial withdrawal: %w", err)})
		}

	case route.ActionFullWithdraw:
		if err := sendWithdrawal(conn, routes, emit); err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending full withdrawal: %w", err)})
		}
	default:
		panic("BUG: invalid executed route action")
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
// When dialer is non-nil, it is used instead of net.Dialer (for in-process mode).
func executeReconnectStorm(ctx context.Context, addr string, p SimProfile, dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}, emit func(Event)) {
	var dial func(ctx context.Context, network, address string) (net.Conn, error)
	if dialer != nil {
		dial = dialer.DialContext
	} else {
		d := net.Dialer{Timeout: 5 * time.Second}
		dial = d.DialContext
	}
	for range stormCycles {
		if !waitWithContext(ctx, stormDelay) {
			return
		}
		if err := runStormCycle(ctx, addr, p, dial, emit); err != nil {
			return
		}
	}
}

func runStormCycle(ctx context.Context, addr string, p SimProfile,
	dial func(ctx context.Context, network, address string) (net.Conn, error), emit func(Event),
) error {
	stormConn, err := dial(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	// Close the storm socket on context cancellation so handshake reads cannot
	// block shutdown indefinitely if the remote peer stops replying.
	stop := context.AfterFunc(ctx, func() {
		stormConn.Close() //nolint:errcheck,gosec // best-effort close to unblock storm handshake
	})
	defer stop()
	defer func() { _ = stormConn.Close() }()

	if err := stormConn.SetReadDeadline(time.Now().Add(handshakeDeadline)); err != nil {
		return err
	}

	// Minimal OPEN/KEEPALIVE handshake.
	open, buildErr := BuildOpen(SessionConfig{ASN: p.ASN, RouterID: p.RouterID, HoldTime: p.HoldTime, Families: p.Families})
	if buildErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("storm OPEN: %w", buildErr)})
		return buildErr
	}
	if writeErr := writeMsg(stormConn, open); writeErr != nil {
		return writeErr
	}
	if readErr := readMsg(stormConn, msgtype.TypeOPEN); readErr != nil {
		return readErr
	}
	if writeErr := writeMsg(stormConn, message.NewKeepalive()); writeErr != nil {
		return writeErr
	}
	if readErr := readMsg(stormConn, msgtype.TypeKEEPALIVE); readErr != nil {
		return readErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	emit(Event{Type: EventEstablished})

	planned := !waitWithContext(ctx, stormDelay)
	stormConn.Close() //nolint:errcheck,gosec // intentional close for storm cycle
	emit(Event{Type: EventDisconnected, PlannedShutdown: planned})
	if planned {
		return ctx.Err()
	}
	return nil
}

// executeConnectionCollision opens a second TCP connection with the same
// RouterID to trigger RFC 4271 Section 6.8 connection collision handling.
// When dialer is non-nil, it is used instead of net.Dialer (for in-process mode).
func executeConnectionCollision(ctx context.Context, addr string, p SimProfile, dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}, emit func(Event)) {
	var dial func(ctx context.Context, network, address string) (net.Conn, error)
	if dialer != nil {
		dial = dialer.DialContext
	} else {
		d := net.Dialer{Timeout: 5 * time.Second}
		dial = d.DialContext
	}
	collisionConn, err := dial(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() == nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("collision connection: %w", err)})
		}
		return
	}
	defer func() { _ = collisionConn.Close() }()

	// Send OPEN with the same RouterID to trigger collision detection.
	open, buildErr := BuildOpen(SessionConfig{ASN: p.ASN, RouterID: p.RouterID, HoldTime: p.HoldTime, Families: p.Families})
	if buildErr != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("collision OPEN: %w", buildErr)})
		return
	}
	if writeErr := writeMsg(collisionConn, open); writeErr != nil {
		return
	}

	// Brief pause for Ze to detect the collision.
	waitWithContext(ctx, 500*time.Millisecond)
}

func waitWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// sendWithdrawal writes standard IPv4 UPDATEs and emits each completed batch,
// even if a subsequent write fails. The event owns the serialized UPDATE for MRT.
func sendWithdrawal(conn io.Writer, prefixes []netip.Prefix, emit func(Event)) error {
	for len(prefixes) > 0 {
		// RFC 4271 Section 4: split at the size limit before serializing.
		count := withdrawalBatchSize(prefixes)
		batch := prefixes[:count]
		data := buildWithdrawal(batch)
		written, err := conn.Write(data)
		if err != nil {
			return err
		}
		if written != len(data) {
			return io.ErrShortWrite
		}
		emit(Event{
			Type:              EventWithdrawalSent,
			WithdrawnPrefixes: batch,
			Family:            familyIPv4Unicast,
			Count:             count,
			BytesSent:         int64(written),
			BGPMessage:        data,
		})
		prefixes = prefixes[count:]
	}
	return nil
}

// withdrawFraction withdraws a subset chosen using the peer's deterministic seed.
func withdrawFraction(conn net.Conn, routes []netip.Prefix, fraction float64,
	seed uint64, peerIndex int, emit func(Event),
) error {
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

	return sendWithdrawal(conn, selected, emit)
}
