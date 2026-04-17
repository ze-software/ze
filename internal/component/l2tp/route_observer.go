// Design: docs/architecture/l2tp.md -- subscriber route lifecycle
// Related: redistribute.go -- source registration
// Related: reactor.go -- session FSM calls OnSessionIPUp / OnSessionDown

package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"
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

	mu      sync.Mutex
	records map[uint16]*routeRecord

	// injectedTotal and withdrawnTotal are monotonic counters the CLI
	// `show l2tp statistics` handler reads to surface redistribute
	// activity without having to walk the map.
	injectedTotal  uint64
	withdrawnTotal uint64
}

// newSubscriberRouteObserver returns an observer that logs every IP-up
// and session-down, and retains the last-known state per session.
// spec-l2tp-7 scope stops at state retention + counters; emitting a
// real BGP UPDATE for each route requires the programmatic inject path
// (currently only the `bgp rib inject` CLI entry exists) and is
// deferred to a follow-up.
func newSubscriberRouteObserver(logger *slog.Logger) *subscriberRouteObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &subscriberRouteObserver{
		logger:  logger.With("component", "l2tp-redistribute"),
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
	o.mu.Lock()
	defer o.mu.Unlock()
	r := o.records[sessionID]
	if r == nil {
		r = &routeRecord{sessionID: sessionID, username: username}
		o.records[sessionID] = r
	}
	if username != "" && r.username == "" {
		r.username = username
	}
	if addr.Is4() {
		r.v4 = addr
	} else {
		r.v6 = addr
	}
	o.injectedTotal++
	o.logger.Info("l2tp: subscriber route inject",
		"session-id", sessionID,
		"username", r.username,
		"address", addr.String(),
		"family", familyOf(addr))
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

// familyOf returns "ipv4" or "ipv6" for the given address.
func familyOf(a netip.Addr) string {
	if a.Is4() {
		return "ipv4"
	}
	return "ipv6"
}
