// Design: docs/architecture/bgp/interface-event-reactions.md -- BGP reactions to interface events
// RFC: rfc/short/rfc2545.md — the Section 3 link-local condition is re-settled on an address or neighbor change
// Overview: reactor.go — Reactor struct and lifecycle

package reactor

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"syscall"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/network"
)

// bgpListenerReadyPayload is the JSON payload emitted by the reactor on
// (bgp, listener-ready). Iface migration consumers wait for this signal
// before tearing down the old address.
type bgpListenerReadyPayload struct {
	Address string `json:"address"`
}

// refreshPeerLinkScopesMu serializes refreshPeerLinkScopes across its two
// callers. It is package-level because the Reactor struct belongs to
// reactor.go; serializing refreshes of distinct reactors costs nothing a
// test can see.
var refreshPeerLinkScopesMu sync.Mutex

// watchNeighborTable is the watch neighborWatchWorker starts. It is
// network.WatchNeighbors; a test replaces it and restores it in t.Cleanup,
// because the table registry is a one-shot slot filled by the interface
// component's init().
var watchNeighborTable = network.WatchNeighbors

// waitNeighborWatch is the wait between two neighbor watch starts. It is
// waitTableWatchRetry; a test replaces it to record the delays without
// sleeping through them, and restores it in t.Cleanup.
var waitNeighborWatch = waitTableWatchRetry

// errNeighborWatchEnded is the state of a watch that closed its channel while
// the reactor ran. The interface component's watch follows a replaced backend
// itself (watchIPv6Neighbors), so a close is a watch that failed: its kernel
// subscription ended, or the backend was closed with nothing in its place.
// The worker starts a new one after a delay.
var errNeighborWatchEnded = errors.New("the neighbor table watch ended; ze starts it again")

// startNeighborWatch starts neighborWatchWorker on the reactor's wait group.
// MUST be called after r.ctx is set, from StartWithContext.
func (r *Reactor) startNeighborWatch() {
	ctx := r.ctx
	r.wg.Go(func() { r.neighborWatchWorker(ctx) })
}

// neighborWatchWorker is the long-lived worker that keeps every live peer's
// neighbor view current. The Link-Local of a third-party next hop is derived
// from two entries (its global and the Link-Local with the same link-layer
// address), so each watch signal re-reads the table whole, once for every
// peer (refreshPeerLinkScopes). The refresh skip compares whole tables, so a
// change anywhere in the neighbor table rebuilds every live peer's snapshot
// once.
//
// superviseTableWatch keeps the watch open and starts it again after a
// failure. A replaced interface backend never closes the watch's channel,
// because the watch moves to the backend that replaced it. Every state the
// worker reaches is published (noteNeighborWatch), so a next hop sent without
// its router's Link-Local says whether the table it was answered from is
// watched.
//
// RFC 2545 Section 3 binds the form of each advertisement made after the
// change; it states no obligation to re-advertise a route already sent, so
// none is triggered here.
func (r *Reactor) neighborWatchWorker(ctx context.Context) {
	// published is the last state this worker stored. A later StartWithContext
	// starts a new worker: leave no state of this run behind for views built
	// before that worker publishes its own, and never erase the state the new
	// worker already published.
	var published *neighborWatchState
	defer func() { r.neighborWatch.CompareAndSwap(published, nil) }()
	changes, err := watchNeighborTable(ctx)
	r.superviseTableWatch(ctx, tableWatch{
		start: watchNeighborTable,
		wait:  waitNeighborWatch,
		ended: errNeighborWatchEnded,
		running: func() {
			published, _ = r.noteNeighborWatch(neighborWatchState{status: neighborWatchRunning})
			r.refreshPeerLinkScopes()
		},
		changed: r.refreshPeerLinkScopes,
		stopped: func(err error) {
			var changed bool
			published, changed = r.noteNeighborWatch(neighborWatchState{status: neighborWatchStopped, err: err})
			if changed {
				r.refreshPeerLinkScopes()
			}
		},
	}, changes, err)
}

// noteNeighborWatch publishes state, logs it when it differs from the state
// before, and answers the stored pointer and whether the state changed. The
// caller then refreshes every live peer so each view carries the new state: a
// view built while the table was watched would otherwise go on claiming it
// is. The worker keeps the pointer to clear only its own state when it ends.
func (r *Reactor) noteNeighborWatch(state neighborWatchState) (*neighborWatchState, bool) {
	previous := r.neighborWatchState()
	stored := &state
	r.neighborWatch.Store(stored)
	if previous.equal(state) {
		return stored, false
	}
	switch state.status {
	case neighborWatchRunning:
		reactorLogger().Info("watching the kernel neighbor table for RFC 2545 third-party link-local next hops")
	case neighborWatchStopped:
		reactorLogger().Warn("the kernel neighbor table is not watched: a change to a router's neighbor entry is not seen until the watch runs again",
			"error", state.err, "fix", neighborWatchFix(state.err))
	case neighborWatchNotStarted:
		panic("BUG: the neighbor watch worker published the not-started state")
	default:
		panic("BUG: invalid neighbor watch status")
	}
	return stored, true
}

// neighborWatchState answers the state the worker last published, or
// not started before it published one.
func (r *Reactor) neighborWatchState() neighborWatchState {
	if state := r.neighborWatch.Load(); state != nil {
		return *state
	}
	return neighborWatchState{status: neighborWatchNotStarted}
}

// refreshPeerLinkScopes re-reads the host interface and neighbor tables once and
// re-settles RFC 2545 Section 3's inclusion condition, and the third-party
// Link-Local it names, for every peer that has a live forwarding snapshot.
//
// Section 3 decides the next-hop wire form against the subnets this host is
// attached to, and the snapshot that answers it is built at session
// establishment and at a config reload (link_scope.go). An address added to or
// removed from an interface OTHER than the one carrying the session moves that
// answer while the TCP connection survives, so without this the speaker keeps
// appending a link-local Section 3 now forbids, or keeps omitting one it now
// requires.
//
// This binds the FORM of every advertisement made after the change. Section 3
// constrains what a speaker "shall advertise ... in the Network Address of Next
// Hop field", which is the act of advertising; it states no obligation to
// re-advertise a route already sent, so no re-advertisement is triggered here.
//
// It runs on addressWatchWorker for an address change and on
// neighborWatchWorker for a neighbor change; refreshPeerLinkScopesMu serializes
// the two so an older read never stores after a newer one. It MUST NOT hold
// reactor.mu while refreshing, so the peer list is copied under the read lock and
// the lock is released before any peer is touched. The kernel is read once for
// the whole fan-out rather than once per peer, and a peer whose scope already
// holds that exact table is left alone (see the loop).
func (r *Reactor) refreshPeerLinkScopes() {
	refreshPeerLinkScopesMu.Lock()
	defer refreshPeerLinkScopesMu.Unlock()
	r.mu.RLock()
	peers := make([]*Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer)
	}
	r.mu.RUnlock()

	if len(peers) == 0 {
		return
	}

	addresses, err := network.InterfacePrefixes()
	if err != nil {
		reactorLogger().Error("refresh receive NEXT_HOP addresses", "error", err)
	}
	connected := make([]netip.Prefix, len(addresses))
	for i, address := range addresses {
		connected[i] = address.Masked()
	}
	// RFC 2545 Section 3: another router's Link-Local comes from the neighbor
	// table, read once for the fan-out like the interface table.
	neighbors := readNeighborView(r.neighborWatchState())
	for _, peer := range peers {
		peer.mu.RLock()
		session := peer.session
		peer.mu.RUnlock()
		if session != nil {
			session.mu.RLock()
			conn := session.conn
			session.mu.RUnlock()
			if conn != nil {
				if err != nil {
					session.nextHopScope.Store(nil)
				} else {
					session.nextHopScope.Store(newReceiveNextHopScope(addresses, conn, session.settings))
				}
			}
		}
		// An interface burst delivers one event per address, and the kernel already
		// holds every address of the burst by the time the first event is delivered.
		// Events 2..N therefore read a table identical to the one this peer's scope
		// was built from, and rebuilding on an identical table allocates a linkScope
		// and a forwarding-facts snapshot per peer per event to reach the same
		// answer. Section 3's condition is decided by the two tables alone, so
		// equal tables decide it equally.
		//
		// A peer that has never read a table has a nil scope, so the comparison is
		// false and it always rebuilds. A reordered table is not equal and rebuilds
		// too, which costs the saving rather than the answer.
		//
		// This guard belongs here and NOT in refreshForwardFactsIfLiveFrom
		// (peer_forward_facts.go). That function is also the settings-apply path
		// (peer_settings_apply.go), which changes inputs the interface table says
		// nothing about, and must rebuild every time.
		if scope := peer.llScope.Load(); scope != nil && slices.Equal(scope.connected, connected) && scope.neighbors.equal(neighbors) {
			continue
		}
		peer.refreshForwardFactsIfLiveFrom(connected, neighbors)
	}
}

// listenPortsAt returns the ports the configuration wants a listener on at
// addr, by the rule the start uses (listenerSpecs): a passive peer or dynamic
// group bound to addr, each at its own listen port. It is empty when nothing
// accepts connections on addr.
// Caller MUST hold r.mu (read or write).
func (r *Reactor) listenPortsAt(addr netip.Addr) []int {
	var ports []int
	for _, spec := range r.listenerSpecs() {
		if spec.addr == addr {
			ports = append(ports, spec.port)
		}
	}
	return ports
}

// addressBound reports whether any peer or dynamic group names addr as its
// local address, whether it listens or only dials out.
// Caller MUST hold r.mu (read or write).
func (r *Reactor) addressBound(addr netip.Addr) bool {
	for _, peer := range r.peers {
		if peer.Settings().LocalAddress == addr {
			return true
		}
	}
	for _, dg := range r.dynamicGroups {
		if dg.Settings != nil && dg.Settings.LocalAddress == addr {
			return true
		}
	}
	return false
}

// handleAddressAdded starts the listeners the configuration wants on an
// address that appeared (listenPortsAt): one per listen port of the passive
// peers and dynamic groups bound to it, and none for a peer that only dials
// out. When every one of them is up it emits (bgp, listener-ready) so iface
// migration consumers can complete their make-before-break. An address bound
// only by peers that dial out owes no listener, so it is ready as soon as it
// appears and the event goes out at once: MigrateInterface waits for it before
// it removes the old address, and would otherwise time out and roll back. It
// runs on addressWatchWorker and MUST be called without reactor.mu held.
//
// It answers false only when the address cannot be bound YET: an IPv6
// address still in duplicate address detection is in the table, and the
// kernel refuses to bind it (EADDRNOTAVAIL) until DAD clears its tentative
// flag. That clearing is an address update of its own, so the worker leaves
// the address unrecorded and tries again on the next signal. The retry
// re-opens nothing already up, because startListenerForAddressPort returns
// nil for a key it already holds.
func (r *Reactor) handleAddressAdded(addr netip.Addr) bool {
	r.mu.Lock()
	bound := r.addressBound(addr)
	ports := r.listenPortsAt(addr)
	startErrs := make([]error, len(ports))
	for i, port := range ports {
		startErrs[i] = r.startListenerForAddressPort(addr, port)
	}
	r.mu.Unlock()

	if !bound {
		return true
	}
	if len(ports) > 0 {
		reactorLogger().Info("iface: address added, starting listeners",
			"address", addr, "ports", ports)
	}

	usable := true
	ready := true
	for i, startErr := range startErrs {
		if startErr == nil {
			continue
		}
		ready = false
		port := ports[i]
		// A stopping reactor refuses the listener by design (reactor.go, the
		// r.stopping read in startListenerForAddressPort), and this handler runs
		// on the address watch worker, which ends only with the cancel -- so any
		// address change during a shutdown lands here. That is the seal working,
		// not an operator's problem, and Error would put it in front of them on
		// every stop that meets an address change.
		if errors.Is(startErr, errReactorStopping) {
			reactorLogger().Debug("iface: listener not started, reactor is stopping",
				"address", addr, "port", port)
			continue
		}
		if errors.Is(startErr, syscall.EADDRNOTAVAIL) {
			reactorLogger().Info("iface: address not usable yet, the listener starts when the kernel reports it usable",
				"address", addr, "port", port, "error", startErr)
			usable = false
			continue
		}
		reactorLogger().Error("iface: start listener failed",
			"address", addr, "port", port, "error", startErr,
			"fix", "the error names the cause: for address already in use, find the other listener with: ss -ltnp; for permission denied, run ze with CAP_NET_BIND_SERVICE")
	}
	if !ready {
		return usable
	}
	if r.eventBus != nil {
		readyPayload, _ := json.Marshal(bgpListenerReadyPayload{Address: addr.String()})
		if _, emitErr := r.eventBus.Emit(bgpevents.Namespace, bgpevents.EventListenerReady, string(readyPayload)); emitErr != nil {
			reactorLogger().Debug("iface: emit listener-ready", "address", addr, "error", emitErr)
		}
	}
	return true
}

// handleAddressRemoved stops every listener on an address that was removed,
// at any port. Nothing can accept on an address the host no longer holds, so
// the stop does not ask the configuration which ports it wants: a peer on a
// port of its own loses its listener, and so does a listener a reload left
// with no claimant (installDynamicGroupsLocked leaves a removed group's socket
// open, because it may be shared with a peer and closing it is the
// peer-removal path's decision). It runs on addressWatchWorker and MUST be
// called without reactor.mu held.
func (r *Reactor) handleAddressRemoved(addr netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for lkey, listener := range r.listeners {
		// startListenerForAddressPort builds every key with net.JoinHostPort,
		// so each one parses back to the address and port it was bound to.
		bound, err := netip.ParseAddrPort(lkey)
		if err != nil {
			reactorLogger().Error("iface: listener key is not an address and port, its listener is left running",
				"key", lkey, "error", err,
				"fix", "none: this is a ze defect, report it with this log line")
			continue
		}
		if bound.Addr() != addr {
			continue
		}
		reactorLogger().Info("iface: address removed, stopping listener",
			"address", addr, "port", bound.Port())
		listener.Stop()
		delete(r.listeners, lkey)
	}
}
