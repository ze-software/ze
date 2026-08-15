// Design: docs/architecture/rib-transition.md — watchdog plugin extraction
// Overview: watchdog.go — plugin main and SDK lifecycle
// Related: pool.go — route pool management
// Related: config.go — config parsing and route building

package watchdog

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	bgp "github.com/ze-software/ze/internal/component/bgp"
)

// watchdogServer manages watchdog route pools and command dispatch.
// Separated from SDK wiring for testability — tests inject a sendRoute
// function instead of a real SDK plugin.
type watchdogServer struct {
	// peerPools holds per-peer route pools, keyed by peer address.
	// Populated from config during OnConfigure.
	peerPools map[string]*poolSet

	// peerUp tracks which peers currently have an established session.
	peerUp map[string]bool

	// sendRoute sends a text command to a peer via the engine.
	// In production this calls plugin.UpdateRoute; in tests it's a hook.
	sendRoute func(peer, cmd string)

	// sessionReady tells the engine this plugin has finished its initial
	// contribution for one peer. In production it dispatches
	// `request peer <addr> plugin session ready`; in tests it is a hook.
	//
	// It is OWED, not optional. A peer that grants a process
	// `send [ update ]` holds its End-of-RIB until that process says it is
	// done, so that the marker means "the initial routing update is complete"
	// (RFC 4724 Section 2). A process that never says it holds the marker for
	// the whole sync timeout, and an unrelated event-driven announce raised
	// inside that window is queued AHEAD of the marker -- which makes the
	// marker claim a route that was never part of the initial update. Measured
	// on the healthcheck fixtures, whose probe rises inside the window.
	sessionReady func(peer string)

	mu sync.RWMutex
}

// newWatchdogServer creates a watchdog server with the given route sender and
// readiness signal. Either hook may be nil in a test that asserts neither.
func newWatchdogServer(sendRoute func(peer, cmd string), sessionReady func(peer string)) *watchdogServer {
	return &watchdogServer{
		peerPools:    make(map[string]*poolSet),
		peerUp:       make(map[string]bool),
		sendRoute:    sendRoute,
		sessionReady: sessionReady,
	}
}

const (
	commandRequestWatchdogAnnounce = "request bgp watchdog announce"
	commandRequestWatchdogWithdraw = "request bgp watchdog withdraw"

	statusDone  = "done"
	statusError = "error"
)

// ErrWatchdogNotFound is returned when a watchdog group doesn't exist for a peer.
var ErrWatchdogNotFound = errors.New("watchdog group not found")

// watchdogAllResult is the command response for a wildcard (all-peers) action.
type watchdogAllResult struct {
	Watchdog string `json:"watchdog"`
	Peers    int    `json:"peers"`
	Action   string `json:"action"`
}

// watchdogPeerResult is the command response for a single-peer action.
type watchdogPeerResult struct {
	Peer     string `json:"peer"`
	Watchdog string `json:"watchdog"`
	Count    int    `json:"count"`
}

// handleCommand dispatches watchdog commands. The data return is an arbitrary
// Go value the SDK marshals once into the response (see OnExecuteCommand);
// handlers must return a struct/map, never a pre-marshaled JSON string, or the
// value is double-encoded on the wire.
// Called from OnExecuteCommand with the command name, arguments, and peer selector.
func (s *watchdogServer) handleCommand(command string, args []string, peer string) (string, any, error) {
	if command == commandRequestWatchdogAnnounce {
		return s.handlePoolAction(args, peer, true)
	}
	if command == commandRequestWatchdogWithdraw {
		return s.handlePoolAction(args, peer, false)
	}
	return statusError, nil, fmt.Errorf("unknown watchdog command: %s", command)
}

// handlePoolAction handles announce (announce=true) or withdraw (announce=false)
// for a named watchdog pool. Flips state even if the peer is down so
// reconnect resend picks up the correct state.
//
// Announce syntax: request bgp watchdog announce <name> [med <N>] [peer]
// When peer is "*", the action applies to all peers that have the named pool.
func (s *watchdogServer) handlePoolAction(args []string, peer string, announce bool) (string, any, error) {
	if len(args) < 1 {
		return statusError, nil, errors.New("missing watchdog name")
	}
	name := args[0]

	// Reject "med" as a group name -- ambiguous with the med keyword.
	if name == "med" {
		return statusError, nil, errors.New("'med' is not allowed as a watchdog group name (ambiguous with med <N> argument)")
	}

	// Parse optional "med <N>" for announce commands.
	var medOverride *uint32
	remaining := args[1:]
	if announce && len(remaining) >= 1 && remaining[0] == "med" {
		if len(remaining) < 2 {
			return statusError, nil, errors.New("missing MED value after 'med' keyword")
		}
		v, err := strconv.ParseUint(remaining[1], 10, 32)
		if err != nil {
			return statusError, nil, fmt.Errorf("invalid MED value %q: %w", remaining[1], err)
		}
		med32 := uint32(v)
		medOverride = &med32
		remaining = remaining[2:]
	}

	// Remaining token (if any) overrides the peer selector.
	if len(remaining) > 0 {
		peer = remaining[0]
	}

	// Wildcard: dispatch to all peers with this pool.
	if peer == "*" {
		return s.handlePoolActionAll(name, announce, medOverride)
	}

	return s.handlePoolActionSingle(name, peer, announce, medOverride)
}

// handlePoolActionAll dispatches a watchdog action to all peers that have the named pool.
func (s *watchdogServer) handlePoolActionAll(name string, announce bool, medOverride *uint32) (string, any, error) {
	s.mu.RLock()
	peers := make([]string, 0, len(s.peerPools))
	for addr := range s.peerPools {
		peers = append(peers, addr)
	}
	s.mu.RUnlock()

	total := 0
	for _, addr := range peers {
		_, _, err := s.handlePoolActionSingle(name, addr, announce, medOverride)
		if err != nil {
			if !errors.Is(err, ErrWatchdogNotFound) {
				logger().Warn("watchdog wildcard peer error", "peer", addr, "pool", name, "error", err)
			}
			continue
		}
		total++
	}

	action := "announced"
	if !announce {
		action = "withdrawn"
	}
	return statusDone, watchdogAllResult{Watchdog: name, Peers: total, Action: action}, nil
}

// handlePoolActionSingle handles announce/withdraw for a single peer.
// When medOverride is non-nil, the announce path clones each entry's Route,
// sets the overridden MED, and calls FormatAnnounceCommand for a one-off
// command. The MED path bypasses the announced boolean dedup because the
// pool tracks announced/withdrawn state, not command content.
func (s *watchdogServer) handlePoolActionSingle(name, peer string, announce bool, medOverride *uint32) (string, any, error) {
	s.mu.RLock()
	pools := s.peerPools[peer]
	isUp := s.peerUp[peer]
	s.mu.RUnlock()

	if pools == nil {
		return statusError, nil, fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	pool := pools.getPool(name)
	if pool == nil {
		return statusError, nil, fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	if announce && medOverride != nil {
		// MED override path: bypass dedup, always dispatch.
		return s.handleMEDOverride(pool, peer, isUp, name, medOverride)
	}

	// Standard path: use pool announce/withdraw with dedup.
	// Pool existence already verified above. A nil/empty result means
	// all routes are already in the target state (dedup), not "not found."
	var entries []*PoolEntry
	if announce {
		entries = pools.AnnouncePool(name, peer)
	} else {
		entries = pools.withdrawPool(name, peer)
	}

	// Only send if peer is up
	if isUp {
		for _, entry := range entries {
			if announce {
				s.sendRoute(peer, entry.AnnounceCmd)
			} else {
				s.sendRoute(peer, entry.WithdrawCmd)
			}
		}
		if m := watchdogMetricsPtr.Load(); m != nil {
			if announce {
				m.routeAnnouncements.Add(float64(len(entries)))
			} else {
				m.routeWithdrawals.Add(float64(len(entries)))
			}
		}
	}

	action := "announced"
	if !announce {
		action = "withdrawn"
	}
	logger().Debug("watchdog "+action, "peer", peer, "pool", name, "count", len(entries), "up", isUp)
	return statusDone, watchdogPeerResult{Peer: peer, Watchdog: name, Count: len(entries)}, nil
}

// handleMEDOverride dispatches announce commands with an overridden MED value.
// For each entry in the pool: clone Route, set MED, format a one-off command.
// Bypasses the announced boolean dedup -- always dispatches when peer is up.
// Sets announced[peer]=true so subsequent non-MED announces are deduped.
//
// When the peer session is NOT yet up, the MED is stashed in
// PoolEntry.pendingMED and announced[peer] is still flipped to true.
// handleStateUp consumes pendingMED on session establishment so the
// first UPDATE carries the override instead of the config default.
// This covers the healthcheck race where probe success fires before
// BGP finishes its OPEN handshake (regression surfaced by
// test/plugin/healthcheck-cycle.ci with debounce=true).
func (s *watchdogServer) handleMEDOverride(pool *routePool, peer string, isUp bool, name string, med *uint32) (string, any, error) {
	// Clone routes and build commands under lock (#17: Route read must be under lock).
	pool.mu.Lock()
	var cmds []string
	for _, e := range pool.routes {
		e.announced[peer] = true
		if isUp {
			delete(e.pendingMED, peer)
			clone := e.Route
			clone.MED = med
			cmds = append(cmds, bgp.FormatAnnounceCommand(&clone))
			continue
		}
		// Peer not yet up — defer the dispatch by stashing the override.
		e.pendingMED[peer] = med
	}
	count := len(pool.routes)
	pool.mu.Unlock()

	for _, cmd := range cmds {
		s.sendRoute(peer, cmd)
	}
	if len(cmds) > 0 {
		if m := watchdogMetricsPtr.Load(); m != nil {
			m.routeAnnouncements.Add(float64(len(cmds)))
		}
	}

	if med != nil {
		logger().Debug("watchdog announced (med override)", "peer", peer, "pool", name, "med", *med, "count", count, "up", isUp)
	}
	return statusDone, watchdogPeerResult{Peer: peer, Watchdog: name, Count: count}, nil
}

// handleStateUp handles a peer coming up (session established).
// Sends all announced routes for the peer, and initializes state for
// initially-announced routes that haven't been seen before.
func (s *watchdogServer) handleStateUp(peerAddr string) {
	s.mu.Lock()
	wasUp := s.peerUp[peerAddr]
	s.peerUp[peerAddr] = true
	pools := s.peerPools[peerAddr]
	s.mu.Unlock()

	if !wasUp {
		if m := watchdogMetricsPtr.Load(); m != nil {
			m.peersUp.Inc()
		}
	}

	if pools == nil {
		s.signalReady(peerAddr)
		return
	}

	// For each pool, initialize state for initially-announced routes
	// and send all routes that are in announced state.
	for _, poolName := range pools.poolNames() {
		pool := pools.getPool(poolName)
		if pool == nil {
			continue
		}

		// First pass: mark only initiallyAnnounced routes as announced for this peer.
		// Routes with initiallyAnnounced=false (withdraw=true in config) are left
		// untouched — they require an explicit "request bgp watchdog announce" command.
		pools.AnnounceInitial(poolName, peerAddr)

		// Second pass: send all announced routes. When an earlier
		// handleMEDOverride deferred its dispatch because the peer was
		// not yet up, the override MED sits in pendingMED[peerAddr];
		// consume it here so the replayed UPDATE carries the same MED
		// the healthcheck asked for.
		announced := pools.AnnouncedForPeer(poolName, peerAddr)
		for _, entry := range announced {
			pool.mu.Lock()
			med, hasPending := entry.pendingMED[peerAddr]
			if hasPending {
				delete(entry.pendingMED, peerAddr)
			}
			pool.mu.Unlock()
			if hasPending {
				clone := entry.Route
				clone.MED = med
				s.sendRoute(peerAddr, bgp.FormatAnnounceCommand(&clone))
				continue
			}
			s.sendRoute(peerAddr, entry.AnnounceCmd)
		}

		if len(announced) > 0 {
			if m := watchdogMetricsPtr.Load(); m != nil {
				m.routeAnnouncements.Add(float64(len(announced)))
			}
			logger().Debug("watchdog resent on reconnect", "peer", peerAddr, "pool", poolName, "count", len(announced))
		}
	}

	s.signalReady(peerAddr)
}

// signalReady tells the engine this plugin has put everything it owes this peer
// on the wire, so the End-of-RIB may follow. See watchdogServer.sessionReady.
//
// Sent on EVERY peer-up, including the peer that has no pool and the pool whose
// routes are all withdrawn: "I have nothing for this peer" completes the initial
// update exactly as a route does, and staying silent would hold the marker for
// the sync timeout on precisely the peers that need it least.
func (s *watchdogServer) signalReady(peerAddr string) {
	if s.sessionReady != nil {
		s.sessionReady(peerAddr)
	}
}

// handleStateDown handles a peer going down.
func (s *watchdogServer) handleStateDown(peerAddr string) {
	s.mu.Lock()
	wasUp := s.peerUp[peerAddr]
	s.peerUp[peerAddr] = false
	s.mu.Unlock()

	if wasUp {
		if m := watchdogMetricsPtr.Load(); m != nil {
			m.peersUp.Dec()
		}
	}
}
