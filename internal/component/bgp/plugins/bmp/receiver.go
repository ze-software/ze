// Design: docs/guide/bmp.md -- BMP receiver, the `environment bmp` listeners
//
// Overview: bmp.go -- plugin lifecycle, the config callbacks that call in here
// Related: bmp_events.go -- the SENDER half, which shares none of this state
//
// The BMP receiver: the TCP listeners that accept feeds from BMP-enabled
// routers, and the reconcile that keeps them equal to the configuration after
// a reload.

package bmp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
)

// parseReceiverConfig extracts BMP receiver config from the environment section JSON.
// The JSON is {"environment": {"bmp": {...}}} (wrapped by ExtractConfigSubtree).
func parseReceiverConfig(data string) (*receiverConfig, error) {
	var sec environmentSection
	if err := json.Unmarshal([]byte(data), &sec); err != nil {
		return nil, fmt.Errorf("bmp receiver config: %w", err)
	}
	if sec.Environment == nil || sec.Environment.BMP == nil {
		return &receiverConfig{}, nil
	}
	return sec.Environment.BMP, nil
}

// wantedListeners is the set of listen addresses a receiver configuration asks
// for, keyed by the address string that also keys bp.listeners.
//
// A receiver that is not enabled wants none, so `enabled false` and a removed
// `server` reach applyReceiverConfig by the one route and close the same way.
func wantedListeners(cfg *receiverConfig) map[string]struct{} {
	if cfg == nil || cfg.Enabled != yangTrue {
		return nil
	}
	wanted := make(map[string]struct{}, len(cfg.Servers))
	for _, srv := range cfg.Servers {
		wanted[net.JoinHostPort(srv.IP, srv.Port)] = struct{}{}
	}
	return wanted
}

// applyReceiverConfig makes the bound listeners equal to what cfg asks for.
//
// This is the whole receiver rail: Stage-2 configure, a reload's config-apply
// and a rollback all call it, and each one is the same question -- which
// addresses should be bound now. An address already bound is LEFT alone, so a
// commit that adds one listener does not interrupt the routers feeding the
// others. An address no longer wanted is closed, which is how `enabled false`,
// a deleted `server` and a moved port all take effect.
//
// A BMP session already accepted on a closed listener is not torn down here.
// RFC 7854 Section 4.5 gives the receiver no way to refuse a session it has
// accepted other than closing the socket, and the monitored router owns the
// reconnect; what the operator asked for is that ze stop accepting on that
// address, which closing the listener does.
//
// max-sessions is stored rather than passed down, so a reload changes the cap
// on a listener it did not have to rebind (acceptLoop reads it per accept).
func (bp *BMPPlugin) applyReceiverConfig(cfg *receiverConfig) {
	if cfg != nil && cfg.RouteAction == "redistribute" {
		logger().Warn("bmp: route-action redistribute is not yet implemented, using monitor")
	}
	if cfg != nil {
		bp.maxSessions.Store(uint32(parseUint16(cfg.MaxSessions, 100)))
	}

	wanted := wantedListeners(cfg)

	bp.mu.Lock()
	defer bp.mu.Unlock()

	for addr, ln := range bp.listeners {
		if _, keep := wanted[addr]; keep {
			delete(wanted, addr)
			continue
		}
		// Removed from the map BEFORE the close, because that is what tells
		// acceptLoop the close was deliberate (listenerIsCurrent).
		delete(bp.listeners, addr)
		closeLog(ln, "listener")
		logger().Info("bmp: receiver stopped listening", "address", addr)
	}

	for addr := range wanted {
		bp.startListenerLocked(addr)
	}
}

// startListenerLocked binds one receiver listener and starts its accept loop.
//
// Caller MUST hold bp.mu.
func (bp *BMPPlugin) startListenerLocked(addr string) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		logger().Error("bmp: listener bind failed", "address", addr, "error", err)
		return
	}

	if bp.listeners == nil {
		bp.listeners = make(map[string]net.Listener)
	}
	bp.listeners[addr] = ln
	logger().Info("bmp: receiver listening", "address", addr)

	bp.sessions.Go(func() {
		bp.acceptLoop(addr, ln)
	})
}

// stopListeners closes all receiver listeners.
func (bp *BMPPlugin) stopListeners() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for addr, ln := range bp.listeners {
		if err := ln.Close(); err != nil {
			logger().Debug("bmp: listener close", "address", addr, "error", err)
		}
	}
	bp.listeners = nil
}

// listenerIsCurrent reports whether ln is still the listener bound at addr.
//
// It is how acceptLoop tells a deliberate close from a failure: a reload and a
// shutdown both drop the entry before closing the socket, so a false answer
// means somebody meant it. Comparing the LISTENER and not just the address
// matters, because a rebind of the same address puts a new listener in the map
// while the old goroutine is still waking up.
func (bp *BMPPlugin) listenerIsCurrent(addr string, ln net.Listener) bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.listeners[addr] == ln
}

// acceptLoop accepts BMP connections on the listener until it is closed.
//
// The session cap is read from bp.maxSessions on each accept rather than taken
// as a parameter, so a reload that changes `max-sessions` binds no socket.
func (bp *BMPPlugin) acceptLoop(addr string, ln net.Listener) {
	var active atomic.Int32

	for {
		conn, err := ln.Accept()
		if err != nil {
			if bp.isStopping() {
				return
			}
			if !bp.listenerIsCurrent(addr, ln) {
				// A reload closed this listener on purpose. Reporting it as a
				// failure would write one false warning per listener on every
				// commit that moves a port.
				return
			}
			logger().Warn("bmp: accept failed", "address", addr, "error", err)
			return
		}

		// Increment before goroutine spawn to avoid TOCTOU race at the limit.
		if active.Add(1) > int32(bp.maxSessions.Load()) { //nolint:gosec // max-sessions is a YANG range of 1..1000
			active.Add(-1)
			logger().Warn("bmp: max sessions reached, rejecting", "remote", conn.RemoteAddr())
			closeLog(conn, "rejected-conn")
			continue
		}

		bp.sessions.Go(func() {
			defer active.Add(-1)
			bp.handleSession(conn)
		})
	}
}
