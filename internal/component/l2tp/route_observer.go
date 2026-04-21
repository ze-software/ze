// Design: docs/architecture/l2tp.md -- subscriber route lifecycle
// Related: redistribute.go -- source registration
// Related: events/events.go -- typed EventBus handle for route-change
// Related: reactor.go -- session FSM calls OnSessionIPUp / OnSessionDown

package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
	OnSessionIPUp(sessionID uint16, username string, addr netip.Addr)

	// OnSessionDown fires when the session's per-session goroutine
	// exits (peer CDN, local teardown, auth failure, NCP timeout).
	OnSessionDown(sessionID uint16)
}

// routeRecord is the live state the observer tracks per session.
type routeRecord struct {
	sessionID uint16
	username  string
	v4        netip.Addr
	v6        netip.Addr
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
// session record.
func (o *subscriberRouteObserver) OnSessionIPUp(sessionID uint16, username string, addr netip.Addr) {
	if !addr.IsValid() {
		return
	}
	var prev netip.Addr
	o.mu.Lock()
	r := o.records[sessionID]
	if r == nil {
		r = &routeRecord{sessionID: sessionID, username: username}
		o.records[sessionID] = r
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
}

// OnSessionDown clears the session's record and bumps the
// withdrawn counter once for each family that had been reported.
func (o *subscriberRouteObserver) OnSessionDown(sessionID uint16) {
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
	o.mu.Unlock()
	if r.v4.IsValid() {
		o.emitRemove(r.v4)
	}
	if r.v6.IsValid() {
		o.emitRemove(r.v6)
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
	if o.bus == nil {
		return
	}
	fam := familyForAddr(addr)
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = l2tpevents.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action: redistevents.ActionAdd,
		Prefix: prefixForAddr(addr),
	})
	if _, err := l2tpevents.RouteChange.Emit(o.bus, b); err != nil {
		o.logger.Warn("l2tp: route-change emit failed", "error", err)
	}
}

// emitRemove builds and emits a single-entry remove batch for the given address.
func (o *subscriberRouteObserver) emitRemove(addr netip.Addr) {
	if o.bus == nil {
		return
	}
	fam := familyForAddr(addr)
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = l2tpevents.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action: redistevents.ActionRemove,
		Prefix: prefixForAddr(addr),
	})
	if _, err := l2tpevents.RouteChange.Emit(o.bus, b); err != nil {
		o.logger.Warn("l2tp: route-change emit failed", "error", err)
	}
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
