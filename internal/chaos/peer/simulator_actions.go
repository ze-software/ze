// Design: docs/architecture/chaos-web-dashboard.md — chaos and route action execution
// Overview: simulator.go — main simulation loop and types
// Related: simulator_reader.go — message reading and parsing

package peer

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/engine"
	"codeberg.org/thomas-mangin/ze/internal/chaos/route"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// executeChaos handles a single chaos action on the simulator's live connection.
// stopKeepalive stops the keepalive timer/ticker (works with both real and virtual time).
func executeChaos(ctx context.Context, action engine.ChaosAction, conn net.Conn,
	stopKeepalive func(), p SimProfile, cfg SimulatorConfig, emit func(Event),
) ChaosResult {
	switch action.Type {
	case engine.ActionTCPDisconnect:
		// Abrupt disconnect — no NOTIFICATION.
		return ChaosResult{Disconnected: true}

	case engine.ActionNotificationCease:
		// Clean disconnect with NOTIFICATION.
		sendCease(conn, p.Index, cfg.Quiet)
		return ChaosResult{Disconnected: true}

	case engine.ActionHoldTimerExpiry:
		// Stop sending KEEPALIVEs — Ze will detect hold-timer expiry.
		stopKeepalive()
		return ChaosResult{Disconnected: false}

	case engine.ActionDisconnectDuringBurst:
		// During steady-state this acts like a TCP disconnect.
		// The "during burst" aspect is handled by orchestrator scheduling
		// the action before EOR is sent.
		return ChaosResult{Disconnected: true}

	case engine.ActionReconnectStorm:
		// Rapid reconnect storm: close this connection, then rapidly
		// open/close mini-sessions to stress Ze's session handling.
		// The final reconnection is handled by runPeerLoop.
		conn.Close() //nolint:errcheck,gosec // intentional close to start storm
		executeReconnectStorm(ctx, cfg.Addr, p, emit)
		return ChaosResult{Disconnected: true}

	case engine.ActionConnectionCollision:
		// Open a second TCP connection with the same RouterID while
		// the first is active. Tests RFC 4271 Section 6.8 collision handling.
		executeConnectionCollision(ctx, cfg.Addr, p, emit)
		return ChaosResult{Disconnected: false}

	case engine.ActionMalformedUpdate:
		// Send an UPDATE with invalid ORIGIN value (0xFF).
		// Tests RFC 7606 revised error handling (treat-as-withdraw).
		data := BuildMalformedUpdate()
		if _, writeErr := conn.Write(data); writeErr != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending malformed UPDATE: %w", writeErr)})
		}
		return ChaosResult{Disconnected: false}

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
		wdBytes, err := sendWithdrawal(conn, selected)
		if err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending churn withdrawal: %w", err)})
			return
		}
		emit(Event{Type: EventWithdrawalSent, Count: len(selected), BytesSent: int64(wdBytes)})
		// Re-announce the churned routes.
		for _, prefix := range selected {
			data := sender.BuildRoute(prefix)
			if _, writeErr := conn.Write(data); writeErr != nil {
				emit(Event{Type: EventError, Err: fmt.Errorf("sending churn re-announce: %w", writeErr)})
				return
			}
			emit(Event{Type: EventRouteSent, Prefix: prefix, BytesSent: int64(len(data))})
		}

	case route.ActionPartialWithdraw:
		withdrawn, wdBytes := withdrawFraction(conn, routes, action.WithdrawFraction, cfg.Seed, p.Index, emit)
		emit(Event{Type: EventWithdrawalSent, Count: len(withdrawn), BytesSent: int64(wdBytes)})

	case route.ActionFullWithdraw:
		wdBytes, err := sendWithdrawal(conn, routes)
		if err != nil {
			emit(Event{Type: EventError, Err: fmt.Errorf("sending full withdrawal: %w", err)})
		}
		emit(Event{Type: EventWithdrawalSent, Count: len(routes), BytesSent: int64(wdBytes)})
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
// Returns the number of bytes written and any error.
func sendWithdrawal(conn net.Conn, prefixes []netip.Prefix) (int, error) {
	data := BuildWithdrawal(prefixes)
	if data == nil {
		return 0, nil
	}
	_, err := conn.Write(data)
	return len(data), err
}

// withdrawFraction withdraws a random subset of routes and returns the
// withdrawn prefixes and number of bytes written. Uses a deterministic PRNG derived from the seed.
func withdrawFraction(conn net.Conn, routes []netip.Prefix, fraction float64,
	seed uint64, peerIndex int, emit func(Event),
) ([]netip.Prefix, int) {
	if len(routes) == 0 || fraction <= 0 {
		return nil, 0
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

	n, err := sendWithdrawal(conn, selected)
	if err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("sending partial withdrawal: %w", err)})
	}

	return selected, n
}
