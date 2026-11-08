// Design: docs/features/interfaces.md -- Router Advertisement content and the CE Router Lifetime
// Related: sender_linux.go -- the send loop that owns the socket and calls this
//
// RFC: rfc/short/rfc7084.md, rfc/short/rfc9915.md

package ifacera

import (
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/routewatch"
)

// mainRouteTable is the kernel's main routing table (RT_TABLE_MAIN), where the
// WAN default route lives, whether Router Discovery or the operator put it.
const mainRouteTable = 254

// advertiser decides what one sender's next advertisement carries. It holds no
// socket and no timer: the send loop owns the I/O and calls it, and a test
// drives every decision with a chosen clock. Not safe for concurrent use; the
// send loop is its only caller.
type advertiser struct {
	spec iface.RASenderSpec
	// retiring holds the delegated /64s an update dropped. The next
	// advertisement carries each once with zero lifetimes, then forgets it.
	retiring []iface.DelegatedAdvert
	// lastLifetime is the Router Lifetime of the last advertisement sent,
	// and sentAny says one was sent at all.
	lastLifetime uint16
	sentAny      bool
}

// update replaces the spec. A delegated /64 the new spec no longer carries is
// retired: the lease ended, or the unit no longer numbers from it.
func (a *advertiser) update(spec iface.RASenderSpec) {
	for _, old := range a.spec.Delegated {
		kept := slices.ContainsFunc(spec.Delegated, func(d iface.DelegatedAdvert) bool {
			return d.Address.Masked() == old.Address.Masked()
		})
		if !kept {
			a.retiring = append(a.retiring, old)
		}
	}
	a.spec = spec
}

// build returns the advertisement to send at now. wanDefault says whether a
// WAN the unit names holds an IPv6 default route.
func (a *advertiser) build(now time.Time, wanDefault bool) ndp.RAConfig {
	cfg := a.spec.Advertisement
	cfg.Prefixes = slices.Clone(cfg.Prefixes)
	cfg.Routes = slices.Clone(cfg.Routes)
	for _, d := range a.spec.Delegated {
		// RFC 9915 Section 6.3
		valid, preferred := d.Lifetimes(now)
		cfg.Prefixes = append(cfg.Prefixes, delegatedPrefixInfo(d.Address.Masked(), valid, preferred))
		// RFC 7084 Section 4.3 L-3
		cfg.Routes = withRoute(cfg.Routes, d.Delegated.Masked(), valid)
	}
	for _, d := range a.retiring {
		cfg.Prefixes = append(cfg.Prefixes, delegatedPrefixInfo(d.Address.Masked(), 0, 0))
		cfg.Routes = withRoute(cfg.Routes, d.Delegated.Masked(), 0)
	}
	cfg.RouterLifetime = a.routerLifetime(now, wanDefault)
	return cfg
}

// delegatedPrefixInfo is the Prefix Information option of a delegated /64,
// with the lifetimes that remain of its delegation.
//
// RFC 9915 Section 6.3: "In particular, if the delegated prefix or a prefix
// derived from it is advertised for stateless address autoconfiguration
// [RFC4862], the advertised preferred and valid lifetimes MUST NOT exceed the
// corresponding remaining lifetimes of the delegated prefix." The L and A
// flags are set: the /64 is the link's prefix, and hosts form their
// addresses from it (RFC 7084 Section 4.3 L-6).
func delegatedPrefixInfo(subnet netip.Prefix, valid, preferred uint32) ndp.PrefixInfo {
	return ndp.PrefixInfo{
		Prefix:            subnet,
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     valid,
		PreferredLifetime: preferred,
	}
}

// withRoute adds a Route Information option for prefix, or raises the
// lifetime of the one already there. One option per prefix is sent, so a
// host never reads two lifetimes for one route; it carries the longest
// lifetime any delegation gives that prefix.
//
// RFC 7084 Section 4.3 L-3: "An IPv6 CE router MUST advertise itself as a
// router for the delegated prefix(es) (and ULA prefix if configured to
// provide ULA addressing) using the "Route Information Option" specified in
// Section 2.3 of [RFC4191]."
func withRoute(routes []ndp.RouteInfo, prefix netip.Prefix, lifetime uint32) []ndp.RouteInfo {
	for i := range routes {
		if routes[i].Prefix.Masked() == prefix {
			routes[i].Lifetime = max(routes[i].Lifetime, lifetime)
			return routes
		}
	}
	return append(routes, ndp.RouteInfo{Prefix: prefix, Lifetime: lifetime})
}

// hasPrefix reports whether the unit advertises a prefix at now: one the
// operator configured, or a delegated /64 whose delegation is still valid.
func (a *advertiser) hasPrefix(now time.Time) bool {
	if len(a.spec.Advertisement.Prefixes) > 0 {
		return true
	}
	for _, d := range a.spec.Delegated {
		if d.Valid.Remaining(now) > 0 {
			return true
		}
	}
	return false
}

// routerLifetime is the Router Lifetime of the condition matrix: the
// configured value for a unit in no CE role, and in the CE role the configured
// value only while a WAN holds a default route and the unit has a prefix.
//
// RFC 7084 Section 4.1 G-4: "By default, an IPv6 CE router that has no
// default router(s) on its WAN interface MUST NOT advertise itself as an IPv6
// default router on its LAN interfaces."
// RFC 7084 Section 4.3 L-4: "An IPv6 CE router MUST NOT advertise itself as a
// default router with a Router Lifetime [RFC4861] greater than zero if it has
// no prefixes configured or delegated to it."
func (a *advertiser) routerLifetime(now time.Time, wanDefault bool) uint16 {
	configured := a.spec.Advertisement.RouterLifetime
	if len(a.spec.WANs) == 0 {
		// RFC 4861 Section 6.2.3: the configured AdvDefaultLifetime.
		return configured
	}
	if !wanDefault {
		return 0
	}
	if !a.hasPrefix(now) {
		return 0
	}
	return configured
}

// urgent reports whether the advertisement at now must go before the
// periodic schedule: a retired prefix to withdraw, or a Router Lifetime that
// crossed zero since the last one sent.
//
// RFC 7084 Section 4.1 G-5: "By default, if the IPv6 CE router is an
// advertising router and loses its IPv6 default router(s) and/or detects loss
// of connectivity on the WAN interface, it MUST explicitly invalidate itself
// as an IPv6 default router on each of its advertising interfaces by
// immediately transmitting one or more Router Advertisement messages with the
// "Router Lifetime" field set to zero [RFC4861]." The change the other way,
// and a withdrawn prefix, use the configuration change of RFC 4861 Section
// 6.2.4, under which the router "MAY transmit up to
// MAX_INITIAL_RTR_ADVERTISEMENTS unsolicited advertisements".
func (a *advertiser) urgent(now time.Time, wanDefault bool) bool {
	if len(a.retiring) > 0 {
		return true
	}
	if !a.sentAny {
		// The initial burst has not started; it carries the change.
		return false
	}
	next := a.routerLifetime(now, wanDefault)
	return (a.lastLifetime > 0) != (next > 0)
}

// sent records the advertisement that left, which ends every retirement.
func (a *advertiser) sent(cfg ndp.RAConfig) {
	a.lastLifetime = cfg.RouterLifetime
	a.sentAny = true
	a.retiring = nil
}

// defaultRouteKey identifies one IPv6 default route in the main table.
type defaultRouteKey struct {
	ifindex int
	nextHop netip.Addr
	metric  uint32
}

// wanDefaults is the set of IPv6 default routes in the main table, kept from
// the shared route watcher. The watcher's goroutine writes it and the send
// loop reads it. Bounded by the default routes the kernel holds. Safe for
// concurrent use.
type wanDefaults struct {
	mu     sync.Mutex
	routes map[defaultRouteKey]struct{}
	// touched holds the routes an event named before the seed landed, so a
	// seed read before a removal cannot put the removed route back. Nil once
	// seeded.
	touched map[defaultRouteKey]struct{}
}

func newWANDefaults() *wanDefaults {
	return &wanDefaults{
		routes:  make(map[defaultRouteKey]struct{}),
		touched: make(map[defaultRouteKey]struct{}),
	}
}

// isMainDefault reports whether ev names an IPv6 default route in the main table.
func isMainDefault(ev routewatch.RouteEvent) bool {
	if ev.TableID != mainRouteTable {
		return false
	}
	if !ev.Prefix.Addr().Is6() {
		return false
	}
	return ev.Prefix.Bits() == 0
}

// observe applies one route event and reports whether the set changed. It
// does no I/O, so it is safe on the watcher's goroutine.
func (w *wanDefaults) observe(ev routewatch.RouteEvent) bool {
	if !isMainDefault(ev) {
		return false
	}
	key := defaultRouteKey{ifindex: ev.OutIfindex, nextHop: ev.NextHop, metric: ev.Metric}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.touched != nil {
		w.touched[key] = struct{}{}
	}
	_, had := w.routes[key]
	switch ev.Action {
	case routewatch.ActionAdd:
		w.routes[key] = struct{}{}
		return !had
	case routewatch.ActionRemove:
		delete(w.routes, key)
		return had
	}
	// The action set is open on the wire side: an unknown action changes nothing.
	return false
}

// seed adds the default routes the kernel held when the sender started. The
// watcher reports existing routes only when it subscribes, so a sender that
// starts later reads them once. A route an event already named keeps the
// event's verdict.
func (w *wanDefaults) seed(existing []routewatch.RouteEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, ev := range existing {
		if !isMainDefault(ev) {
			continue
		}
		key := defaultRouteKey{ifindex: ev.OutIfindex, nextHop: ev.NextHop, metric: ev.Metric}
		if _, named := w.touched[key]; named {
			continue
		}
		w.routes[key] = struct{}{}
	}
	w.touched = nil
}

// holds reports whether a default route leaves through ifindex.
func (w *wanDefaults) holds(ifindex int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for key := range w.routes {
		if key.ifindex == ifindex {
			return true
		}
	}
	return false
}
