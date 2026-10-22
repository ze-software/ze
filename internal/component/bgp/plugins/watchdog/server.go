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
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// watchdogServer manages watchdog route pools and command dispatch.
// Separated from SDK wiring for testability — tests inject a sendRoute
// function instead of a real SDK plugin.
type watchdogServer struct {
	// peerPools holds per-peer route pools, keyed by peer address.
	// Installed at Stage 2 and reconciled by each reload (config_tx.go).
	peerPools map[string]*poolSet

	// peerUp tracks which peers currently have an established session.
	peerUp map[string]bool

	// send sends a text command to a peer via the engine, and answers the
	// engine's refusal. In production this calls plugin.UpdateRoute; in tests
	// it's a hook. The configured pools send through sendRoute, which logs a
	// refusal; a given route's command answers it to the process (given.go).
	send func(peer, cmd string) error

	// sessionReady tells the engine this plugin has finished its initial
	// contribution for one peer. In production it dispatches
	// `request peer <addr> plugin session ready session <token>`; in tests it is a hook.
	// Callers MUST pass the token captured from the peer-up event. The engine
	// credits completion only to the session whose initial routes were replayed.
	sessionReady func(peer string, initialReplay uint64)

	// given holds the routes processes gave each watchdog at runtime
	// (`request bgp watchdog give`), keyed by watchdog name then givenKey.
	// givenCount is their total, bounded by givenRoutesMax (given.go).
	given      map[string]map[string]*givenRoute
	givenCount int

	mu sync.RWMutex
}

// newWatchdogServer creates a watchdog server with the given route sender and
// readiness signal. Either hook may be nil in a test that asserts neither.
func newWatchdogServer(send func(peer, cmd string) error, sessionReady func(peer string, initialReplay uint64)) *watchdogServer {
	return &watchdogServer{
		peerPools:    make(map[string]*poolSet),
		peerUp:       make(map[string]bool),
		send:         send,
		sessionReady: sessionReady,
		given:        make(map[string]map[string]*givenRoute),
	}
}

// sendRoute sends one command of a configured pool, and logs the engine's
// refusal: the pools move on peer events no caller waits on, so the log is
// where an operator reads it.
func (s *watchdogServer) sendRoute(peer, cmd string) {
	if err := s.send(peer, cmd); err != nil {
		logger().Warn("update-route failed", "peer", peer, "error", err)
	}
}

const (
	commandRequestWatchdogAnnounce = "request bgp watchdog announce"
	commandRequestWatchdogWithdraw = "request bgp watchdog withdraw"

	statusDone  = "done"
	statusError = "error"

	// medKeyword is the announce argument that overrides the MED, so it is
	// no watchdog's name.
	medKeyword = "med"
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
	if command == commandRequestWatchdogGive {
		return s.handleGive(args)
	}
	if command == commandRequestWatchdogForget {
		return s.handleForget(args)
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
	if name == medKeyword {
		return statusError, nil, errors.New("'med' is not allowed as a watchdog group name (ambiguous with med <N> argument)")
	}

	// Parse optional "med <N>" for announce commands.
	var medOverride *uint32
	remaining := args[1:]
	if announce && len(remaining) >= 1 && remaining[0] == medKeyword {
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

	// The routes a process gave this watchdog move with the configured ones
	// (given.go). Their update text is the process's own, so a MED override
	// cannot be written into it: refused rather than announced without it.
	if medOverride != nil && s.givenHolds(name) {
		return statusError, nil, fmt.Errorf("watchdog %s holds routes a process gave it, whose MED the med argument cannot change; "+
			"write request bgp watchdog announce %s without med", name, name)
	}
	// A given route the engine refused to send is the answer, after the
	// configured pools have moved: the command did not do all it said.
	given, givenErr := s.givenAction(name, peer, announce)

	// Wildcard: dispatch to all peers with this pool.
	if peer == "*" {
		status, data, err := s.handlePoolActionAll(name, announce, medOverride)
		if givenErr != nil {
			return statusError, nil, givenErr
		}
		return status, data, err
	}

	status, data, err := s.handlePoolActionSingle(name, peer, announce, medOverride)
	if givenErr != nil {
		return statusError, nil, givenErr
	}
	if given > 0 && errors.Is(err, ErrWatchdogNotFound) {
		return statusDone, watchdogPeerResult{Peer: peer, Watchdog: name, Count: given}, nil
	}
	return status, data, err
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

// handlePeerUp routes a peer-up event. An up carrying reason
// rpc.ReasonAttached is a reload attaching this plugin to a session already
// established: the peer's initial routing update was sent without this plugin,
// so it is not this plugin's to resend, and such an up carries no initial-replay
// receipt to report ready against. Every other up resends the announced routes.
func (s *watchdogServer) handlePeerUp(peerAddr, reason string, initialReplay uint64) {
	if reason == rpc.ReasonAttached {
		s.markPeerUp(peerAddr)
		return
	}
	s.handleStateUp(peerAddr, initialReplay)
}

// markPeerUp records the peer as up and returns its pools, nil when the
// configuration gives it none.
func (s *watchdogServer) markPeerUp(peerAddr string) *poolSet {
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
	return pools
}

// handleStateUp handles a peer coming up (session established).
// Sends all announced routes for the peer, and initializes state for
// initially-announced routes that haven't been seen before.
// Callers MUST pass the token captured from the peer-up event.
func (s *watchdogServer) handleStateUp(peerAddr string, initialReplay uint64) {
	pools := s.markPeerUp(peerAddr)
	if pools == nil {
		s.signalReady(peerAddr, initialReplay)
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

	s.signalReady(peerAddr, initialReplay)
}

// signalReady reports completion of the initial routing contribution.
// Callers MUST pass their peer-up event's captured token.
// Every peer-up with a token reports, including peers with no announced routes.
func (s *watchdogServer) signalReady(peerAddr string, initialReplay uint64) {
	if initialReplay == 0 {
		return
	}
	if s.sessionReady != nil {
		s.sessionReady(peerAddr, initialReplay)
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
