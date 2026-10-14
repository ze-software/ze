// Design: docs/architecture/core-design.md -- watchdog route pools
// Related: server.go -- watchdogServer, whose peerPools a reload replaces
// Related: pool.go -- PoolEntry, the per-peer announce state a reload carries

package watchdog

import (
	"sync"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// watchdogConfigTx is the plugin's side of the config transaction. The pools
// are config and announce state at once: an entry remembers, per peer,
// whether it is announced. So a reload cannot install the parsed pools whole
// the way a filter installs its definitions. Apply reconciles instead
// (watchdogServer.reconcile): an entry the candidate keeps carries its
// announce state over, an announced entry the candidate removes is withdrawn,
// and rollback reconciles back to the pools apply replaced.
//
// Safe for concurrent use.
type watchdogConfigTx struct {
	srv *watchdogServer

	mu sync.Mutex
	// pending is the candidate of a verify that accepted it, nil when no
	// verified transaction is open.
	pending *watchdogCandidate
	// previous is what the last accepted apply replaced, nil when that apply
	// changed nothing or a rollback already put it back.
	previous *map[string]*poolSet
}

// watchdogCandidate is one verified candidate: its pools, and whether the
// verify carried the bgp root at all.
type watchdogCandidate struct {
	pools   map[string]*poolSet
	present bool
}

// register binds Stage 2 and the reload callbacks. It MUST be called before
// Run.
func (tx *watchdogConfigTx) register(p *sdk.Plugin) {
	p.OnConfigure(tx.configure)
	p.OnConfigVerify(tx.verify)
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return tx.apply() })
	p.OnConfigRollback(func(_ string) error { return tx.rollback() })
}

// parseWatchdogSections reads the pools from the bgp section of one delivery.
// present is false when the delivery carries no bgp section.
func parseWatchdogSections(sections []sdk.ConfigSection) (pools map[string]*poolSet, present bool, err error) {
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		pools, err := parseConfig(section.Data)
		if err != nil {
			return nil, false, err
		}
		return pools, true, nil
	}
	return nil, false, nil
}

// configure is Stage 2: no session is up yet and no entry is announced, so
// the parsed pools are installed whole.
func (tx *watchdogConfigTx) configure(sections []sdk.ConfigSection) error {
	pools, present, err := parseWatchdogSections(sections)
	if err != nil {
		logger().Error("config parse failed", "error", err)
		return err
	}
	if !present {
		return nil
	}
	tx.srv.mu.Lock()
	tx.srv.peerPools = pools
	tx.srv.mu.Unlock()
	logger().Info("config loaded", "peers", len(pools))
	return nil
}

// verify parses a reload candidate and holds it for apply. It changes no
// pool: a refused verify and a CLI validate run it too.
func (tx *watchdogConfigTx) verify(sections []sdk.ConfigSection) error {
	pools, present, err := parseWatchdogSections(sections)
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.pending = nil
	if err != nil {
		return err
	}
	tx.pending = &watchdogCandidate{pools: pools, present: present}
	return nil
}

// apply reconciles the running pools to the verified candidate and keeps the
// replaced pools for rollback. A candidate with no bgp section changes
// nothing and drops the undo set of an earlier, committed apply.
func (tx *watchdogConfigTx) apply() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	candidate := tx.pending
	tx.pending, tx.previous = nil, nil
	if candidate == nil {
		return sdk.ErrConfigApplyUnverified
	}
	if !candidate.present {
		return nil
	}
	replaced := tx.srv.reconcile(candidate.pools)
	tx.previous = &replaced
	return nil
}

// rollback reconciles back to the pools the last accepted apply replaced.
func (tx *watchdogConfigTx) rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.pending = nil
	if tx.previous == nil {
		return nil
	}
	tx.srv.reconcile(*tx.previous)
	tx.previous = nil
	return nil
}

// peerCommand is one route command reconcile owes a peer, sent after
// watchdogServer.mu is released.
type peerCommand struct {
	peer    string
	command string
}

// reconcile makes target the running pools and returns the pools it
// replaced. Per peer and per entry, keyed by pool name and route key:
//
//   - kept: the entry carries the running entry's announce state and pending
//     MED over; when its announce command changed and it is announced to an
//     up peer, the new command is sent, so the route on the wire follows the
//     config;
//   - removed: when it is announced to an up peer, its withdraw is sent;
//   - added: to an up peer it is announced when it already holds that peer's
//     state as announced (an entry a rollback brings back, as apply removed
//     it) or, holding none, when its config does not say withdraw; to a peer
//     that is down it is left for handleStateUp, as Stage 2 leaves it.
//
// A rollback's target is the pools an apply replaced, which a command that
// loaded them before that apply can still hold, so every entry's state is
// read and written under its pool's lock.
func (s *watchdogServer) reconcile(target map[string]*poolSet) map[string]*poolSet {
	s.mu.Lock()
	replaced := s.peerPools
	var sends []peerCommand
	for peer, pools := range target {
		sends = carryPeerState(replaced[peer], pools, peer, s.peerUp[peer], sends)
	}
	for peer, pools := range replaced {
		if _, kept := target[peer]; kept {
			continue
		}
		sends = withdrawRemoved(pools, nil, peer, s.peerUp[peer], sends)
	}
	s.peerPools = target
	s.mu.Unlock()

	for _, send := range sends {
		s.sendRoute(send.peer, send.command)
	}
	logger().Info("config reloaded", "peers", len(target), "commands", len(sends))
	return replaced
}

// carryPeerState moves one peer's announce state from running into target and
// appends the commands the change owes the peer.
func carryPeerState(running, target *poolSet, peer string, up bool, sends []peerCommand) []peerCommand {
	for _, name := range target.poolNames() {
		var old *routePool
		if running != nil {
			old = running.getPool(name)
		}
		for _, entry := range target.getPool(name).Routes() {
			sends = carryEntryState(old.route(entry.Key), entry, peer, up, sends)
		}
	}
	return withdrawRemoved(running, target, peer, up, sends)
}

// carryEntryState gives entry the announce state previous holds for peer, or
// the Stage 2 state when previous is nil.
func carryEntryState(previous, entry *PoolEntry, peer string, up bool, sends []peerCommand) []peerCommand {
	if previous == nil {
		if !up {
			return sends
		}
		entry.pool.mu.Lock()
		announced, known := entry.announced[peer]
		if !known {
			announced = entry.initiallyAnnounced
		}
		entry.announced[peer] = announced
		entry.pool.mu.Unlock()
		if announced {
			sends = append(sends, peerCommand{peer: peer, command: entry.AnnounceCmd})
		}
		return sends
	}
	previous.pool.mu.RLock()
	announced := previous.announced[peer]
	med, hasMED := previous.pendingMED[peer]
	previous.pool.mu.RUnlock()

	entry.pool.mu.Lock()
	entry.announced[peer] = announced
	delete(entry.pendingMED, peer)
	if hasMED {
		entry.pendingMED[peer] = med
	}
	entry.pool.mu.Unlock()
	if up && announced && previous.AnnounceCmd != entry.AnnounceCmd {
		sends = append(sends, peerCommand{peer: peer, command: entry.AnnounceCmd})
	}
	return sends
}

// withdrawRemoved appends the withdraw of every entry of running that target
// does not hold and that is announced to the up peer. A nil target removes
// every entry.
func withdrawRemoved(running, target *poolSet, peer string, up bool, sends []peerCommand) []peerCommand {
	if running == nil || !up {
		return sends
	}
	for _, name := range running.poolNames() {
		var kept *routePool
		if target != nil {
			kept = target.getPool(name)
		}
		for _, entry := range running.AnnouncedForPeer(name, peer) {
			if kept.route(entry.Key) != nil {
				continue
			}
			sends = append(sends, peerCommand{peer: peer, command: entry.WithdrawCmd})
		}
	}
	return sends
}

// route answers the pool's entry for key, nil when the pool is nil or holds
// none.
func (p *routePool) route(key string) *PoolEntry {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.routes[key]
}
