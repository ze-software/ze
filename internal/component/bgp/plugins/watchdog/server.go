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

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
)

// watchdogServer manages watchdog route pools and command dispatch.
// Separated from SDK wiring for testability — tests inject a sendRoute
// function instead of a real SDK plugin.
type watchdogServer struct {
	// peerPools holds per-peer route pools, keyed by peer address.
	// Populated from config during OnConfigure.
	peerPools map[string]*PoolSet

	// peerUp tracks which peers currently have an established session.
	peerUp map[string]bool

	// sendRoute sends a text command to a peer via the engine.
	// In production this calls plugin.UpdateRoute; in tests it's a hook.
	sendRoute func(peer, cmd string)

	mu sync.RWMutex
}

// newWatchdogServer creates a watchdog server with the given route sender.
func newWatchdogServer(sendRoute func(peer, cmd string)) *watchdogServer {
	return &watchdogServer{
		peerPools: make(map[string]*PoolSet),
		peerUp:    make(map[string]bool),
		sendRoute: sendRoute,
	}
}

// Command response status constants.
const (
	statusDone  = "done"
	statusError = "error"
)

// ErrWatchdogNotFound is returned when a watchdog group doesn't exist for a peer.
var ErrWatchdogNotFound = errors.New("watchdog group not found")

// handleCommand dispatches watchdog commands.
// Called from OnExecuteCommand with the command name, arguments, and peer selector.
func (s *watchdogServer) handleCommand(command string, args []string, peer string) (string, string, error) {
	if command == "watchdog announce" {
		return s.handlePoolAction(args, peer, true)
	}
	if command == "watchdog withdraw" {
		return s.handlePoolAction(args, peer, false)
	}
	return statusError, "", fmt.Errorf("unknown watchdog command: %s", command)
}

// handlePoolAction handles announce (announce=true) or withdraw (announce=false)
// for a named watchdog pool. Flips state even if the peer is down so
// reconnect resend picks up the correct state.
//
// Announce syntax: watchdog announce <name> [med <N>] [peer]
// When peer is "*", the action applies to all peers that have the named pool.
func (s *watchdogServer) handlePoolAction(args []string, peer string, announce bool) (string, string, error) {
	if len(args) < 1 {
		return statusError, "", errors.New("missing watchdog name")
	}
	name := args[0]

	// Reject "med" as a group name -- ambiguous with the med keyword.
	if name == "med" {
		return statusError, "", errors.New("'med' is not allowed as a watchdog group name (ambiguous with med <N> argument)")
	}

	// Parse optional "med <N>" for announce commands.
	var medOverride *uint32
	remaining := args[1:]
	if announce && len(remaining) >= 1 && remaining[0] == "med" {
		if len(remaining) < 2 {
			return statusError, "", errors.New("missing MED value after 'med' keyword")
		}
		v, err := strconv.ParseUint(remaining[1], 10, 32)
		if err != nil {
			return statusError, "", fmt.Errorf("invalid MED value %q: %w", remaining[1], err)
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
func (s *watchdogServer) handlePoolActionAll(name string, announce bool, medOverride *uint32) (string, string, error) {
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
	return statusDone, fmt.Sprintf(`{"watchdog":%q,"peers":%d,"action":%q}`, name, total, action), nil
}

// handlePoolActionSingle handles announce/withdraw for a single peer.
// When medOverride is non-nil, the announce path clones each entry's Route,
// sets the overridden MED, and calls FormatAnnounceCommand for a one-off
// command. The MED path bypasses the announced boolean dedup because the
// pool tracks announced/withdrawn state, not command content.
func (s *watchdogServer) handlePoolActionSingle(name, peer string, announce bool, medOverride *uint32) (string, string, error) {
	s.mu.RLock()
	pools := s.peerPools[peer]
	isUp := s.peerUp[peer]
	s.mu.RUnlock()

	if pools == nil {
		return statusError, "", fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	pool := pools.GetPool(name)
	if pool == nil {
		return statusError, "", fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
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
		entries = pools.WithdrawPool(name, peer)
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
	}

	action := "announced"
	if !announce {
		action = "withdrawn"
	}
	logger().Debug("watchdog "+action, "peer", peer, "pool", name, "count", len(entries), "up", isUp)
	return statusDone, fmt.Sprintf(`{"peer":%q,"watchdog":%q,"count":%d}`, peer, name, len(entries)), nil
}

// handleMEDOverride dispatches announce commands with an overridden MED value.
// For each entry in the pool: clone Route, set MED, format a one-off command.
// Bypasses the announced boolean dedup -- always dispatches when peer is up.
// Sets announced[peer]=true so subsequent non-MED announces are deduped.
func (s *watchdogServer) handleMEDOverride(pool *RoutePool, peer string, isUp bool, name string, med *uint32) (string, string, error) {
	// Clone routes and build commands under lock (#17: Route read must be under lock).
	pool.mu.Lock()
	var cmds []string
	for _, e := range pool.routes {
		if isUp {
			e.announced[peer] = true
			clone := e.Route
			clone.MED = med
			cmds = append(cmds, bgp.FormatAnnounceCommand(&clone))
		}
	}
	count := len(pool.routes)
	pool.mu.Unlock()

	for _, cmd := range cmds {
		s.sendRoute(peer, cmd)
	}

	if med != nil {
		logger().Debug("watchdog announced (med override)", "peer", peer, "pool", name, "med", *med, "count", count, "up", isUp)
	}
	return statusDone, fmt.Sprintf(`{"peer":%q,"watchdog":%q,"count":%d}`, peer, name, count), nil
}

// handleStateUp handles a peer coming up (session established).
// Sends all announced routes for the peer, and initializes state for
// initially-announced routes that haven't been seen before.
func (s *watchdogServer) handleStateUp(peerAddr string) {
	s.mu.Lock()
	s.peerUp[peerAddr] = true
	pools := s.peerPools[peerAddr]
	s.mu.Unlock()

	if pools == nil {
		return
	}

	// For each pool, initialize state for initially-announced routes
	// and send all routes that are in announced state.
	for _, poolName := range pools.PoolNames() {
		pool := pools.GetPool(poolName)
		if pool == nil {
			continue
		}

		// First pass: mark only initiallyAnnounced routes as announced for this peer.
		// Routes with initiallyAnnounced=false (withdraw=true in config) are left
		// untouched — they require an explicit "watchdog announce" command.
		pools.AnnounceInitial(poolName, peerAddr)

		// Second pass: send all announced routes
		announced := pools.AnnouncedForPeer(poolName, peerAddr)
		for _, entry := range announced {
			s.sendRoute(peerAddr, entry.AnnounceCmd)
		}

		if len(announced) > 0 {
			logger().Debug("watchdog resent on reconnect", "peer", peerAddr, "pool", poolName, "count", len(announced))
		}
	}
}

// handleStateDown handles a peer going down.
func (s *watchdogServer) handleStateDown(peerAddr string) {
	s.mu.Lock()
	s.peerUp[peerAddr] = false
	s.mu.Unlock()
}
