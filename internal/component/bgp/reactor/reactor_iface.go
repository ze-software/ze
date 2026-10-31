// Design: docs/architecture/bgp/interface-event-reactions.md -- BGP reactions to interface events
// RFC: rfc/short/rfc2545.md — the Section 3 link-local condition is re-settled on an address event
// Overview: reactor.go — Reactor struct and lifecycle

package reactor

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/network"
)

// interfaceAddrPayload is the JSON payload emitted by the interface monitor
// for (interface, addr-added) and (interface, addr-removed). The reactor
// parses it to discover the local address that needs a listener.
type interfaceAddrPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Index        int    `json:"index"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
}

// bgpListenerReadyPayload is the JSON payload emitted by the reactor on
// (bgp, listener-ready). Iface migration consumers wait for this signal
// before tearing down the old address.
type bgpListenerReadyPayload struct {
	Address string `json:"address"`
}

// subscribeInterfaceEvents registers EventBus handlers for the interface
// events the reactor cares about. Replaces the legacy OnBusEvent prefix
// subscription that lived in reactor_bus.go. Must be called after
// SetEventBus and before StartWithContext.
//
// The reactor acts on addr-added and addr-removed in the (interface, *)
// namespace. Other events (created, up, down, dhcp-*, rollback, router-*) have
// no BGP-side reaction. Neighbor table changes do not come from the bus: they
// come from the neighbor table watch (startNeighborWatch), which runs whether
// or not an interface monitor does.
func (r *Reactor) subscribeInterfaceEvents() {
	if r.eventBus == nil {
		return
	}
	unsubAdded := r.eventBus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, events.AsString(r.onInterfaceAddrAdded))
	unsubRemoved := r.eventBus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, events.AsString(r.onInterfaceAddrRemoved))
	r.eventBusUnsubs = append(r.eventBusUnsubs, unsubAdded, unsubRemoved)
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

// neighborWatchRetryFirst and neighborWatchRetryMax bound how soon a watch
// that could not start, or that ended under a live reactor, is started again.
// The delay doubles from the first to the max, so a source that keeps failing
// costs one start, one stopped state and one warning per delay. It resets to
// the first only after a watch ran neighborWatchRetryMax: a watch that starts
// and then ends at once is the failing source the doubling exists for.
const (
	neighborWatchRetryFirst = time.Second
	neighborWatchRetryMax   = time.Minute
)

// waitNeighborWatch is the wait between two watch starts. It is
// waitNeighborWatchRetry; a test replaces it to record the delays without
// sleeping through them, and restores it in t.Cleanup.
var waitNeighborWatch = waitNeighborWatchRetry

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
// The worker owns each watch it starts: it ranges over the watch's channel
// until the watch closes it, so no watch goroutine outlives the worker. A
// replaced interface backend never closes the channel, because the watch
// moves to the backend that replaced it. A watch that cannot start, or that
// closes while ctx is live, is published as stopped and started again after a
// delay that doubles up to neighborWatchRetryMax; the loop has no count bound
// because the reactor needs the watch for as long as it runs, and it ends only
// with ctx, the reactor's run context. Every state it reaches is published
// (noteNeighborWatch), so a next hop sent without its router's Link-Local
// says whether the table it was answered from is watched.
//
// RFC 2545 Section 3 binds the form of each advertisement made after the
// change, as for an address event; it states no obligation to re-advertise a
// route already sent, so none is triggered here.
func (r *Reactor) neighborWatchWorker(ctx context.Context) {
	// published is the last state this worker stored. A later StartWithContext
	// starts a new worker: leave no state of this run behind for views built
	// before that worker publishes its own, and never erase the state the new
	// worker already published.
	var published *neighborWatchState
	defer func() { r.neighborWatch.CompareAndSwap(published, nil) }()
	retry := neighborWatchRetryFirst
	for {
		changes, err := watchNeighborTable(ctx)
		if err == nil {
			started := r.clock.Now()
			// The table may have moved while no watch ran, so read it again
			// before waiting for the first signal.
			published, _ = r.noteNeighborWatch(neighborWatchState{status: neighborWatchRunning})
			r.refreshPeerLinkScopes()
			for range changes {
				if ctx.Err() != nil {
					continue // draining until the watch closes the channel
				}
				r.refreshPeerLinkScopes()
			}
			if ctx.Err() != nil {
				return
			}
			// A watch that ran a long time failed once, and starts again
			// from the shortest delay.
			if r.clock.Now().Sub(started) >= neighborWatchRetryMax {
				retry = neighborWatchRetryFirst
			}
			err = errNeighborWatchEnded
		}
		var changed bool
		published, changed = r.noteNeighborWatch(neighborWatchState{status: neighborWatchStopped, err: err})
		if changed {
			r.refreshPeerLinkScopes()
		}
		if !waitNeighborWatch(ctx, r.clock, retry) {
			return
		}
		retry = min(2*retry, neighborWatchRetryMax)
	}
}

// waitNeighborWatchRetry waits delay on clk, and reports false when ctx ended
// first.
func waitNeighborWatchRetry(ctx context.Context, clk clock.Clock, delay time.Duration) bool {
	timer := clk.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C():
		return true
	}
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

// onInterfaceAddrAdded is the EventBus handler for (interface, addr-added).
// Runs synchronously inside the EventBus delivery path; MUST NOT hold
// reactor.mu (deadlock risk with the listener startup path).
func (r *Reactor) onInterfaceAddrAdded(payload string) {
	var p interfaceAddrPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		reactorLogger().Debug("iface: unmarshal addr-added", "error", err)
		return
	}
	r.handleAddrAddedPayload(p)
}

// onInterfaceAddrRemoved is the EventBus handler for (interface, addr-removed).
func (r *Reactor) onInterfaceAddrRemoved(payload string) {
	var p interfaceAddrPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		reactorLogger().Debug("iface: unmarshal addr-removed", "error", err)
		return
	}
	r.handleAddrRemovedPayload(p)
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
// This binds the FORM of every advertisement made after the event. Section 3
// constrains what a speaker "shall advertise ... in the Network Address of Next
// Hop field", which is the act of advertising; it states no obligation to
// re-advertise a route already sent, so no re-advertisement is triggered here.
//
// It runs on the EventBus delivery goroutine for an address event and on
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

// handleAddrAddedPayload starts a listener when an address matching a
// peer's LocalAddress appears. On success it emits (bgp, listener-ready)
// so iface migration consumers can complete their make-before-break.
func (r *Reactor) handleAddrAddedPayload(p interfaceAddrPayload) {
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		reactorLogger().Debug("iface: parse address", "address", p.Address, "error", err)
		return
	}
	addr = addr.Unmap()

	// RFC 2545 Section 3: the new address changes which subnets this host is
	// attached to, for every peer and not only the ones bound to it.
	r.refreshPeerLinkScopes()

	// Find peers whose LocalAddress matches.
	r.mu.RLock()
	var matchingPeers []*Peer
	for _, peer := range r.peers {
		if peer.settings.LocalAddress == addr {
			matchingPeers = append(matchingPeers, peer)
		}
	}
	r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return
	}

	reactorLogger().Info("iface: address added, starting listener",
		"address", p.Address, "unit", p.Unit, "peers", len(matchingPeers))

	// Start listener for this address. startListenerForAddressPort is
	// idempotent (returns nil if already listening).
	r.mu.Lock()
	port := r.config.Port
	startErr := r.startListenerForAddressPort(addr, port)
	r.mu.Unlock()

	if startErr != nil {
		// A stopping reactor refuses the listener by design (reactor.go, the
		// r.stopping read in startListenerForAddressPort), and this handler runs
		// on the EventBus goroutine that cleanup unsubscribes only after the
		// cancel -- so any address event during a shutdown lands here. That is
		// the seal working, not an operator's problem, and Error would put it in
		// front of them on every stop that meets a netlink event.
		if errors.Is(startErr, errReactorStopping) {
			reactorLogger().Debug("iface: listener not started, reactor is stopping",
				"address", p.Address, "port", port)
			return
		}
		reactorLogger().Error("iface: start listener failed",
			"address", p.Address, "port", port, "error", startErr)
		return
	}
	if r.eventBus != nil {
		readyPayload, _ := json.Marshal(bgpListenerReadyPayload{Address: p.Address})
		if _, emitErr := r.eventBus.Emit(bgpevents.Namespace, bgpevents.EventListenerReady, string(readyPayload)); emitErr != nil {
			reactorLogger().Debug("iface: emit listener-ready", "address", p.Address, "error", emitErr)
		}
	}
}

// handleAddrRemovedPayload stops the listener for an address that was removed.
func (r *Reactor) handleAddrRemovedPayload(p interfaceAddrPayload) {
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		return
	}
	addr = addr.Unmap()

	// RFC 2545 Section 3: a removed address can end a shared subnet, for every
	// peer and not only the one bound to this address.
	r.refreshPeerLinkScopes()

	r.mu.Lock()
	port := r.config.Port
	// Use net.JoinHostPort for consistent key format with
	// startListenerForAddressPort, which also uses net.JoinHostPort. This
	// is critical for IPv6 addresses where JoinHostPort wraps the address
	// in brackets: "[::1]:179".
	lkey := net.JoinHostPort(addr.String(), strconv.Itoa(port))
	listener, exists := r.listeners[lkey]
	if exists {
		reactorLogger().Info("iface: address removed, stopping listener",
			"address", p.Address, "unit", p.Unit)
		listener.Stop()
		delete(r.listeners, lkey)
	}
	r.mu.Unlock()
}
