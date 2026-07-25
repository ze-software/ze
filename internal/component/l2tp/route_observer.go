// Design: docs/guide/l2tp.md -- subscriber route lifecycle
// Related: redistribute.go -- source registration
// Related: events/events.go -- typed EventBus handle for route-change
// Related: reactor.go -- session FSM calls OnSessionIPUp / OnSessionDown

package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/ze"
)

// RouteObserver is the callback contract invoked by the reactor when a
// session's IP assignment or teardown requires a corresponding redistribute
// action. Implementations are expected to be cheap; the reactor calls them
// while holding only their own state (no reactor locks).
//
// Callers MUST NOT assume OnSessionIPUp fires exactly once per session:
// IPv4 (IPCP) and IPv6 (IPv6CP) can each fire once per session, so a
// dual-stack subscriber generates two events.
//
// OnSessionDown fires at most once per session, paired with whichever
// OnSessionIPUp events preceded it.
type RouteObserver interface {
	// OnSessionIPUp fires when one NCP (IPCP or IPv6CP) successfully
	// negotiates a peer IP. Called once per family per session.
	// tunnelID is needed to look up RADIUS metadata (framed routes).
	OnSessionIPUp(tunnelID, sessionID uint16, username string, addr netip.Addr)

	// OnSessionDown fires when the session's per-session goroutine
	// exits (peer CDN, local teardown, auth failure, NCP timeout).
	// tunnelID is needed to look up RADIUS metadata (framed routes).
	OnSessionDown(tunnelID, sessionID uint16)
}

// routeRecord is the live state the observer tracks per session.
type routeRecord struct {
	tunnelID      uint16
	sessionID     uint16
	username      string
	v4            netip.Addr
	v6            netip.Addr
	framedRoutes  []FramedRoute
	framedEmitted bool
}

// subscriberRouteObserver is the concrete RouteObserver the Subsystem
// installs into each reactor. It maintains a sessionID -> routeRecord
// map so the CLI and future RIB injection path can read the live set.
//
// Thread safety: the internal map is protected by mu; all public
// methods are safe for concurrent use.
type subscriberRouteObserver struct {
	logger *slog.Logger
	bus    ze.EventBus

	mu      sync.Mutex
	records map[uint16]*routeRecord

	// injectedTotal and withdrawnTotal are monotonic counters the CLI
	// `show l2tp statistics` handler reads to surface redistribute
	// activity without having to walk the map.
	injectedTotal  uint64
	withdrawnTotal uint64
}

// newSubscriberRouteObserver returns an observer that logs every IP-up
// and session-down, retains the last-known state per session, and emits
// route-change events on the EventBus when bus is non-nil. When bus is
// nil (tests, partial subsystem init), state tracking and counters
// still work but no events are emitted.
func newSubscriberRouteObserver(logger *slog.Logger, bus ze.EventBus) *subscriberRouteObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &subscriberRouteObserver{
		logger:  logger.With("component", "l2tp-redistribute"),
		bus:     bus,
		records: make(map[uint16]*routeRecord),
	}
}

// OnSessionIPUp records the new NCP-assigned address and logs the
// event. IPv4 and IPv6 are tracked side-by-side under the same
// session record. Also loads RADIUS framed routes from metadata
// on the first call per session.
func (o *subscriberRouteObserver) OnSessionIPUp(tunnelID, sessionID uint16, username string, addr netip.Addr) {
	if !addr.IsValid() {
		return
	}
	var prev netip.Addr
	o.mu.Lock()
	r := o.records[sessionID]
	if r == nil {
		r = &routeRecord{tunnelID: tunnelID, sessionID: sessionID, username: username}
		o.records[sessionID] = r
		if meta := LoadSessionMetadata(tunnelID, sessionID); meta != nil && len(meta.FramedRoutes) > 0 {
			r.framedRoutes = make([]FramedRoute, len(meta.FramedRoutes))
			copy(r.framedRoutes, meta.FramedRoutes)
		}
	}
	if username != "" && r.username == "" {
		r.username = username
	}
	if addr.Is4() {
		prev = r.v4
		r.v4 = addr
	} else {
		prev = r.v6
		r.v6 = addr
	}
	var emitFramed []FramedRoute
	if !r.framedEmitted && len(r.framedRoutes) > 0 {
		emitFramed = r.framedRoutes
		r.framedEmitted = true
	}
	o.injectedTotal++
	o.mu.Unlock()
	o.logger.Info("l2tp: subscriber route inject",
		"session-id", sessionID,
		"username", r.username,
		"address", addr.String(),
		"family", familyOf(addr))
	if prev.IsValid() && prev != addr {
		o.emitRemove(prev)
	}
	o.emitAdd(addr)
	if len(emitFramed) > 0 {
		o.emitFramedRoutes(redistevents.ActionAdd, emitFramed)
	}
}

// OnSessionDown clears the session's record and bumps the
// withdrawn counter once for each family that had been reported.
// Also withdraws any RADIUS framed routes for this session.
func (o *subscriberRouteObserver) OnSessionDown(tunnelID, sessionID uint16) {
	o.mu.Lock()
	r, ok := o.records[sessionID]
	if !ok {
		o.mu.Unlock()
		return
	}
	delete(o.records, sessionID)
	withdrawn := 0
	if r.v4.IsValid() {
		withdrawn++
		o.withdrawnTotal++
	}
	if r.v6.IsValid() {
		withdrawn++
		o.withdrawnTotal++
	}
	framedRoutes := r.framedRoutes
	o.mu.Unlock()
	if r.v4.IsValid() {
		o.emitRemove(r.v4)
	}
	if r.v6.IsValid() {
		o.emitRemove(r.v6)
	}
	if len(framedRoutes) > 0 {
		o.emitFramedRoutes(redistevents.ActionRemove, framedRoutes)
	}
	o.logger.Info("l2tp: subscriber routes withdrawn",
		"session-id", sessionID,
		"username", r.username,
		"withdrawn", withdrawn)
}

// Stats returns a snapshot of the observer's cumulative counters plus
// the number of sessions currently tracked. Used by the CLI
// `show l2tp statistics` handler in spec-l2tp-10.
func (o *subscriberRouteObserver) Stats() (injected, withdrawn uint64, active int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.injectedTotal, o.withdrawnTotal, len(o.records)
}

// emitAdd builds and emits a single-entry add batch for the given address.
// Nil bus is tolerated (no emission, state still tracked).
func (o *subscriberRouteObserver) emitAdd(addr netip.Addr) {
	o.emitEntries(0, familyForAddr(addr), redistevents.RouteChangeEntry{
		Action: redistevents.ActionAdd,
		Prefix: prefixForAddr(addr),
	})
}

// emitRemove builds and emits a single-entry remove batch for the given address.
func (o *subscriberRouteObserver) emitRemove(addr netip.Addr) {
	o.emitEntries(0, familyForAddr(addr), redistevents.RouteChangeEntry{
		Action: redistevents.ActionRemove,
		Prefix: prefixForAddr(addr),
	})
}

// emitEntries builds and emits one batch for fam with the given entries, tagged
// with replayID (0 for the normal incremental path; nonzero echoes a
// redistribute ReplayRequest so the orchestrator can replay to a peer that
// established after injection). Central emit path for the observer. Nil bus or
// no entries is a no-op.
func (o *subscriberRouteObserver) emitEntries(replayID uint64, fam family.Family, entries ...redistevents.RouteChangeEntry) {
	if o.bus == nil || len(entries) == 0 {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = l2tpevents.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.ReplayID = replayID
	b.Entries = append(b.Entries, entries...)
	if _, err := l2tpevents.RouteChange.Emit(o.bus, b); err != nil {
		o.logger.Warn("l2tp: route-change emit failed", "error", err)
	}
}

// reemitAll re-emits every currently-tracked subscriber route (each session's
// live v4/v6 address and any already-emitted RADIUS framed routes) as adds
// tagged with replayID, so the redistribute orchestrator can replay them to a
// peer that establishes after the original emit. Reflects the CURRENT live set;
// a session torn down before the peer joined is absent. A zero replayID is a
// no-op (the orchestrator only allocates nonzero tokens).
func (o *subscriberRouteObserver) reemitAll(replayID uint64) {
	if o.bus == nil || replayID == 0 {
		return
	}
	var v4addrs, v6addrs []netip.Addr
	var v4framed, v6framed []redistevents.RouteChangeEntry
	o.mu.Lock()
	for _, r := range o.records {
		if r.v4.IsValid() {
			v4addrs = append(v4addrs, r.v4)
		}
		if r.v6.IsValid() {
			v6addrs = append(v6addrs, r.v6)
		}
		if r.framedEmitted {
			for _, fr := range r.framedRoutes {
				e := redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: fr.Prefix, Metric: fr.Metric}
				if fr.Prefix.Addr().Is4() {
					v4framed = append(v4framed, e)
				} else {
					v6framed = append(v6framed, e)
				}
			}
		}
	}
	o.mu.Unlock()

	for _, a := range v4addrs {
		o.emitEntries(replayID, family.IPv4Unicast, redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: prefixForAddr(a)})
	}
	for _, a := range v6addrs {
		o.emitEntries(replayID, family.IPv6Unicast, redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: prefixForAddr(a)})
	}
	if len(v4framed) > 0 {
		o.emitEntries(replayID, family.IPv4Unicast, v4framed...)
	}
	if len(v6framed) > 0 {
		o.emitEntries(replayID, family.IPv6Unicast, v6framed...)
	}
}

// emitFramedRoutes emits a batch per address family for the given
// framed routes. Groups routes by AFI so each batch has a consistent
// family header.
func (o *subscriberRouteObserver) emitFramedRoutes(action redistevents.RouteAction, routes []FramedRoute) {
	if o.bus == nil || len(routes) == 0 {
		return
	}
	var v4, v6 []redistevents.RouteChangeEntry
	for _, fr := range routes {
		entry := redistevents.RouteChangeEntry{
			Action: action,
			Prefix: fr.Prefix,
			Metric: fr.Metric,
		}
		if fr.Prefix.Addr().Is4() {
			v4 = append(v4, entry)
		} else {
			v6 = append(v6, entry)
		}
	}
	if len(v4) > 0 {
		o.emitBatch(family.IPv4Unicast, v4)
	}
	if len(v6) > 0 {
		o.emitBatch(family.IPv6Unicast, v6)
	}
}

func (o *subscriberRouteObserver) emitBatch(fam family.Family, entries []redistevents.RouteChangeEntry) {
	o.emitEntries(0, fam, entries...)
}

// prefixForAddr returns /32 for IPv4, /128 for IPv6.
func prefixForAddr(addr netip.Addr) netip.Prefix {
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32)
	}
	return netip.PrefixFrom(addr, 128)
}

// familyForAddr returns ipv4/unicast for IPv4, ipv6/unicast for IPv6.
func familyForAddr(addr netip.Addr) family.Family {
	if addr.Is4() {
		return family.IPv4Unicast
	}
	return family.IPv6Unicast
}

// familyOf returns "ipv4" or "ipv6" for the given address.
func familyOf(a netip.Addr) string {
	if a.Is4() {
		return "ipv4"
	}
	return "ipv6"
}
